// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitdiff

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"html"
	"html/template"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/analyze"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/highlight"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"

	"github.com/sergi/go-diff/diffmatchpatch"
	stdcharset "golang.org/x/net/html/charset"
	"golang.org/x/text/transform"
)

// DiffLineType represents the type of a DiffLine.
type DiffLineType uint8

// DiffLineType possible values.
const (
	DiffLinePlain DiffLineType = iota + 1
	DiffLineAdd
	DiffLineDel
	DiffLineSection
)

// DiffFileType represents the type of a DiffFile.
type DiffFileType uint8

// DiffFileType possible values.
const (
	DiffFileAdd DiffFileType = iota + 1
	DiffFileChange
	DiffFileDel
	DiffFileRename
	DiffFileCopy
)

// DiffLineExpandDirection represents the DiffLineSection expand direction
type DiffLineExpandDirection uint8

// DiffLineExpandDirection possible values.
const (
	DiffLineExpandNone DiffLineExpandDirection = iota + 1
	DiffLineExpandSingle
	DiffLineExpandUpDown
	DiffLineExpandUp
	DiffLineExpandDown
)

// DiffLine represents a line difference in a DiffSection.
type DiffLine struct {
	LeftIdx     int
	RightIdx    int
	Type        DiffLineType
	Content     string
	Comments    []*models.Comment
	SectionInfo *DiffLineSectionInfo
}

// DiffLineSectionInfo represents diff line section meta data
type DiffLineSectionInfo struct {
	Path          string
	LastLeftIdx   int
	LastRightIdx  int
	LeftIdx       int
	RightIdx      int
	LeftHunkSize  int
	RightHunkSize int
}

// BlobExcerptChunkSize represent max lines of excerpt
const BlobExcerptChunkSize = 20

// GetType returns the type of a DiffLine.
func (d *DiffLine) GetType() int {
	return int(d.Type)
}

// CanComment returns whether or not a line can get commented
func (d *DiffLine) CanComment() bool {
	return len(d.Comments) == 0 && d.Type != DiffLineSection
}

// GetCommentSide returns the comment side of the first comment, if not set returns empty string
func (d *DiffLine) GetCommentSide() string {
	if len(d.Comments) == 0 {
		return ""
	}
	return d.Comments[0].DiffSide()
}

// GetLineTypeMarker returns the line type marker
func (d *DiffLine) GetLineTypeMarker() string {
	if strings.IndexByte(" +-", d.Content[0]) > -1 {
		return d.Content[0:1]
	}
	return ""
}

// GetBlobExcerptQuery builds query string to get blob excerpt
func (d *DiffLine) GetBlobExcerptQuery() string {
	query := fmt.Sprintf(
		"last_left=%d&last_right=%d&"+
			"left=%d&right=%d&"+
			"left_hunk_size=%d&right_hunk_size=%d&"+
			"path=%s",
		d.SectionInfo.LastLeftIdx, d.SectionInfo.LastRightIdx,
		d.SectionInfo.LeftIdx, d.SectionInfo.RightIdx,
		d.SectionInfo.LeftHunkSize, d.SectionInfo.RightHunkSize,
		url.QueryEscape(d.SectionInfo.Path))
	return query
}

// GetExpandDirection gets DiffLineExpandDirection
func (d *DiffLine) GetExpandDirection() DiffLineExpandDirection {
	if d.Type != DiffLineSection || d.SectionInfo == nil || d.SectionInfo.RightIdx-d.SectionInfo.LastRightIdx <= 1 {
		return DiffLineExpandNone
	}
	if d.SectionInfo.LastLeftIdx <= 0 && d.SectionInfo.LastRightIdx <= 0 {
		return DiffLineExpandUp
	} else if d.SectionInfo.RightIdx-d.SectionInfo.LastRightIdx > BlobExcerptChunkSize && d.SectionInfo.RightHunkSize > 0 {
		return DiffLineExpandUpDown
	} else if d.SectionInfo.LeftHunkSize <= 0 && d.SectionInfo.RightHunkSize <= 0 {
		return DiffLineExpandDown
	}
	return DiffLineExpandSingle
}

func getDiffLineSectionInfo(treePath, line string, lastLeftIdx, lastRightIdx int) *DiffLineSectionInfo {
	leftLine, leftHunk, rightLine, righHunk := git.ParseDiffHunkString(line)

	return &DiffLineSectionInfo{
		Path:          treePath,
		LastLeftIdx:   lastLeftIdx,
		LastRightIdx:  lastRightIdx,
		LeftIdx:       leftLine,
		RightIdx:      rightLine,
		LeftHunkSize:  leftHunk,
		RightHunkSize: righHunk,
	}
}

// escape a line's content or return <br> needed for copy/paste purposes
func getLineContent(content string) string {
	if len(content) > 0 {
		return html.EscapeString(content)
	}
	return "<br>"
}

// DiffSection represents a section of a DiffFile.
type DiffSection struct {
	FileName string
	Name     string
	Lines    []*DiffLine
}

var (
	addedCodePrefix   = []byte(`<span class="added-code">`)
	removedCodePrefix = []byte(`<span class="removed-code">`)
	codeTagSuffix     = []byte(`</span>`)
)
var trailingSpanRegex = regexp.MustCompile(`<span\s*[[:alpha:]="]*?[>]?$`)
var entityRegex = regexp.MustCompile(`&[#]*?[0-9[:alpha:]]*$`)

// shouldWriteInline represents combinations where we manually write inline changes
func shouldWriteInline(diff diffmatchpatch.Diff, lineType DiffLineType) bool {
	if true &&
		diff.Type == diffmatchpatch.DiffEqual ||
		diff.Type == diffmatchpatch.DiffInsert && lineType == DiffLineAdd ||
		diff.Type == diffmatchpatch.DiffDelete && lineType == DiffLineDel {
		return true
	}
	return false
}

func diffToHTML(fileName string, diffs []diffmatchpatch.Diff, lineType DiffLineType) template.HTML {
	buf := bytes.NewBuffer(nil)
	match := ""

	for _, diff := range diffs {
		if shouldWriteInline(diff, lineType) {
			if len(match) > 0 {
				diff.Text = match + diff.Text
				match = ""
			}
			// Chroma HTML syntax highlighting is done before diffing individual lines in order to maintain consistency.
			// Since inline changes might split in the middle of a chroma span tag or HTML entity, make we manually put it back together
			// before writing so we don't try insert added/removed code spans in the middle of one of those
			// and create broken HTML. This is done by moving incomplete HTML forward until it no longer matches our pattern of
			// a line ending with an incomplete HTML entity or partial/opening <span>.

			// EX:
			// diffs[{Type: dmp.DiffDelete, Text: "language</span><span "},
			// {Type: dmp.DiffEqual, Text: "c"},
			// {Type: dmp.DiffDelete, Text: "lass="p">}]

			// After first iteration
			// diffs[{Type: dmp.DiffDelete, Text: "language</span>"}, //write out
			// {Type: dmp.DiffEqual, Text: "<span c"},
			// {Type: dmp.DiffDelete, Text: "lass="p">,</span>}]

			// After second iteration
			// {Type: dmp.DiffEqual, Text: ""}, // write out
			// {Type: dmp.DiffDelete, Text: "<span class="p">,</span>}]

			// Final
			// {Type: dmp.DiffDelete, Text: "<span class="p">,</span>}]
			// end up writing <span class="removed-code"><span class="p">,</span></span>
			// Instead of <span class="removed-code">lass="p",</span></span>

			m := trailingSpanRegex.FindStringSubmatchIndex(diff.Text)
			if m != nil {
				match = diff.Text[m[0]:m[1]]
				diff.Text = strings.TrimSuffix(diff.Text, match)
			}
			m = entityRegex.FindStringSubmatchIndex(diff.Text)
			if m != nil {
				match = diff.Text[m[0]:m[1]]
				diff.Text = strings.TrimSuffix(diff.Text, match)
			}
			// Print an existing closing span first before opening added/remove-code span so it doesn't unintentionally close it
			if strings.HasPrefix(diff.Text, "</span>") {
				buf.WriteString("</span>")
				diff.Text = strings.TrimPrefix(diff.Text, "</span>")
			}
			// If we weren't able to fix it then this should avoid broken HTML by not inserting more spans below
			// The previous/next diff section will contain the rest of the tag that is missing here
			if strings.Count(diff.Text, "<") != strings.Count(diff.Text, ">") {
				buf.WriteString(diff.Text)
				continue
			}
		}
		switch {
		case diff.Type == diffmatchpatch.DiffEqual:
			buf.WriteString(diff.Text)
		case diff.Type == diffmatchpatch.DiffInsert && lineType == DiffLineAdd:
			buf.Write(addedCodePrefix)
			buf.WriteString(diff.Text)
			buf.Write(codeTagSuffix)
		case diff.Type == diffmatchpatch.DiffDelete && lineType == DiffLineDel:
			buf.Write(removedCodePrefix)
			buf.WriteString(diff.Text)
			buf.Write(codeTagSuffix)
		}
	}
	return template.HTML(buf.Bytes())
}

// GetLine gets a specific line by type (add or del) and file line number
func (diffSection *DiffSection) GetLine(lineType DiffLineType, idx int) *DiffLine {
	var (
		difference    = 0
		addCount      = 0
		delCount      = 0
		matchDiffLine *DiffLine
	)

LOOP:
	for _, diffLine := range diffSection.Lines {
		switch diffLine.Type {
		case DiffLineAdd:
			addCount++
		case DiffLineDel:
			delCount++
		default:
			if matchDiffLine != nil {
				break LOOP
			}
			difference = diffLine.RightIdx - diffLine.LeftIdx
			addCount = 0
			delCount = 0
		}

		switch lineType {
		case DiffLineDel:
			if diffLine.RightIdx == 0 && diffLine.LeftIdx == idx-difference {
				matchDiffLine = diffLine
			}
		case DiffLineAdd:
			if diffLine.LeftIdx == 0 && diffLine.RightIdx == idx+difference {
				matchDiffLine = diffLine
			}
		}
	}

	if addCount == delCount {
		return matchDiffLine
	}
	return nil
}

var diffMatchPatch = diffmatchpatch.New()

func init() {
	diffMatchPatch.DiffEditCost = 100
}

// GetComputedInlineDiffFor computes inline diff for the given line.
func (diffSection *DiffSection) GetComputedInlineDiffFor(diffLine *DiffLine) template.HTML {
	if setting.Git.DisableDiffHighlight {
		return template.HTML(getLineContent(diffLine.Content[1:]))
	}

	var (
		compareDiffLine *DiffLine
		diff1           string
		diff2           string
	)

	// try to find equivalent diff line. ignore, otherwise
	switch diffLine.Type {
	case DiffLineSection:
		return template.HTML(getLineContent(diffLine.Content[1:]))
	case DiffLineAdd:
		compareDiffLine = diffSection.GetLine(DiffLineDel, diffLine.RightIdx)
		if compareDiffLine == nil {
			return template.HTML(highlight.Code(diffSection.FileName, diffLine.Content[1:]))
		}
		diff1 = compareDiffLine.Content
		diff2 = diffLine.Content
	case DiffLineDel:
		compareDiffLine = diffSection.GetLine(DiffLineAdd, diffLine.LeftIdx)
		if compareDiffLine == nil {
			return template.HTML(highlight.Code(diffSection.FileName, diffLine.Content[1:]))
		}
		diff1 = diffLine.Content
		diff2 = compareDiffLine.Content
	default:
		if strings.IndexByte(" +-", diffLine.Content[0]) > -1 {
			return template.HTML(highlight.Code(diffSection.FileName, diffLine.Content[1:]))
		}
		return template.HTML(highlight.Code(diffSection.FileName, diffLine.Content))
	}

	diffRecord := diffMatchPatch.DiffMain(highlight.Code(diffSection.FileName, diff1[1:]), highlight.Code(diffSection.FileName, diff2[1:]), true)
	diffRecord = diffMatchPatch.DiffCleanupEfficiency(diffRecord)

	return diffToHTML(diffSection.FileName, diffRecord, diffLine.Type)
}

// DiffFile represents a file diff.
type DiffFile struct {
	Name               string
	OldName            string
	Index              int
	Addition, Deletion int
	Type               DiffFileType
	IsCreated          bool
	IsDeleted          bool
	IsBin              bool
	IsLFSFile          bool
	IsRenamed          bool
	IsSubmodule        bool
	Sections           []*DiffSection
	IsIncomplete       bool
	IsProtected        bool
	IsIgnored          bool
}

// GetType returns type of diff file.
func (diffFile *DiffFile) GetType() int {
	return int(diffFile.Type)
}

// GetTailSection creates a fake DiffLineSection if the last section is not the end of the file
func (diffFile *DiffFile) GetTailSection(gitRepo *git.Repository, leftCommitID, rightCommitID string) *DiffSection {
	if len(diffFile.Sections) == 0 || diffFile.Type != DiffFileChange || diffFile.IsBin || diffFile.IsLFSFile {
		return nil
	}
	leftCommit, err := gitRepo.GetCommit(leftCommitID)
	if err != nil {
		return nil
	}
	rightCommit, err := gitRepo.GetCommit(rightCommitID)
	if err != nil {
		return nil
	}
	lastSection := diffFile.Sections[len(diffFile.Sections)-1]
	lastLine := lastSection.Lines[len(lastSection.Lines)-1]
	leftLineCount := getCommitFileLineCount(leftCommit, diffFile.Name)
	rightLineCount := getCommitFileLineCount(rightCommit, diffFile.Name)
	if leftLineCount <= lastLine.LeftIdx || rightLineCount <= lastLine.RightIdx {
		return nil
	}
	tailDiffLine := &DiffLine{
		Type:    DiffLineSection,
		Content: " ",
		SectionInfo: &DiffLineSectionInfo{
			Path:         diffFile.Name,
			LastLeftIdx:  lastLine.LeftIdx,
			LastRightIdx: lastLine.RightIdx,
			LeftIdx:      leftLineCount,
			RightIdx:     rightLineCount,
		}}
	tailSection := &DiffSection{FileName: diffFile.Name, Lines: []*DiffLine{tailDiffLine}}
	return tailSection

}

func getCommitFileLineCount(commit *git.Commit, filePath string) int {
	blob, err := commit.GetBlobByPath(filePath)
	if err != nil {
		return 0
	}
	lineCount, err := blob.GetBlobLineCount()
	if err != nil {
		return 0
	}
	return lineCount
}

// Diff represents a difference between two git trees.
type Diff struct {
	NumFiles, TotalAddition, TotalDeletion int
	Files                                  []*DiffFile
	IsIncomplete                           bool
}

// LoadComments loads comments into each line
func (diff *Diff) LoadComments(issue *models.Issue, currentUser *models.User) error {
	allComments, err := models.FetchCodeComments(issue, currentUser)
	if err != nil {
		return err
	}
	for _, file := range diff.Files {
		if lineCommits, ok := allComments[file.Name]; ok {
			for _, section := range file.Sections {
				for _, line := range section.Lines {
					if comments, ok := lineCommits[int64(line.LeftIdx*-1)]; ok {
						line.Comments = append(line.Comments, comments...)
					}
					if comments, ok := lineCommits[int64(line.RightIdx)]; ok {
						line.Comments = append(line.Comments, comments...)
					}
					sort.SliceStable(line.Comments, func(i, j int) bool {
						return line.Comments[i].CreatedUnix < line.Comments[j].CreatedUnix
					})
				}
			}
		}
	}
	return nil
}

const cmdDiffHead = "diff --git "

// ParsePatch builds a Diff object from a io.Reader and some parameters.
func ParsePatch(maxLines, maxLineCharacters, maxFiles int, reader io.Reader) (*Diff, error) {
	var curFile *DiffFile

	diff := &Diff{Files: make([]*DiffFile, 0)}

	sb := strings.Builder{}

	// OK let's set a reasonable buffer size.
	// This should be let's say at least the size of maxLineCharacters or 4096 whichever is larger.
	readerSize := maxLineCharacters
	if readerSize < 4096 {
		readerSize = 4096
	}

	input := bufio.NewReaderSize(reader, readerSize)
	line, err := input.ReadString('\n')
	if err != nil {
		if err == io.EOF {
			return diff, nil
		}
		return diff, err
	}
parsingLoop:
	for {
		// 1. A patch file always begins with `diff --git ` + `a/path b/path` (possibly quoted)
		// if it does not we have bad input!
		if !strings.HasPrefix(line, cmdDiffHead) {
			return diff, fmt.Errorf("Invalid first file line: %s", line)
		}

		// TODO: Handle skipping first n files
		if len(diff.Files) >= maxFiles {
			diff.IsIncomplete = true
			_, err := io.Copy(ioutil.Discard, reader)
			if err != nil {
				// By the definition of io.Copy this never returns io.EOF
				return diff, fmt.Errorf("Copy: %v", err)
			}
			break parsingLoop
		}

		curFile = createDiffFile(diff, line)
		diff.Files = append(diff.Files, curFile)

		// 2. It is followed by one or more extended header lines:
		//
		//     old mode <mode>
		//     new mode <mode>
		//     deleted file mode <mode>
		//     new file mode <mode>
		//     copy from <path>
		//     copy to <path>
		//     rename from <path>
		//     rename to <path>
		//     similarity index <number>
		//     dissimilarity index <number>
		//     index <hash>..<hash> <mode>
		//
		// * <mode> 6-digit octal numbers including the file type and file permission bits.
		// * <path> does not include the a/ and b/ prefixes
		// * <number> percentage of unchanged lines for similarity, percentage of changed
		//   lines dissimilarity as integer rounded down with terminal %. 100% => equal files.
		// * The index line includes the blob object names before and after the change.
		//   The <mode> is included if the file mode does not change; otherwise, separate
		//   lines indicate the old and the new mode.
		// 3. Following this header the "standard unified" diff format header may be encountered: (but not for every case...)
		//
		//     --- a/<path>
		//     +++ b/<path>
		//
		// With multiple hunks
		//
		//     @@ <hunk descriptor> @@
		//     +added line
		//     -removed line
		//      unchanged line
		//
		// 4. Binary files get:
		//
		//     Binary files a/<path> and b/<path> differ
		//
		// but one of a/<path> and b/<path> could be /dev/null.
	curFileLoop:
		for {
			line, err = input.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					return diff, err
				}
				break parsingLoop
			}
			switch {
			case strings.HasPrefix(line, cmdDiffHead):
				break curFileLoop
			case strings.HasPrefix(line, "old mode ") ||
				strings.HasPrefix(line, "new mode "):
				if strings.HasSuffix(line, " 160000\n") {
					curFile.IsSubmodule = true
				}
			case strings.HasPrefix(line, "copy from "):
				curFile.IsRenamed = true
				curFile.Type = DiffFileCopy
			case strings.HasPrefix(line, "copy to "):
				curFile.IsRenamed = true
				curFile.Type = DiffFileCopy
			case strings.HasPrefix(line, "new file"):
				curFile.Type = DiffFileAdd
				curFile.IsCreated = true
				if strings.HasSuffix(line, " 160000\n") {
					curFile.IsSubmodule = true
				}
			case strings.HasPrefix(line, "deleted"):
				curFile.Type = DiffFileDel
				curFile.IsDeleted = true
				if strings.HasSuffix(line, " 160000\n") {
					curFile.IsSubmodule = true
				}
			case strings.HasPrefix(line, "index"):
				if strings.HasSuffix(line, " 160000\n") {
					curFile.IsSubmodule = true
				}
			case strings.HasPrefix(line, "similarity index 100%"):
				curFile.Type = DiffFileRename
			case strings.HasPrefix(line, "Binary"):
				curFile.IsBin = true
			case strings.HasPrefix(line, "--- "):
				// Do nothing with this line
			case strings.HasPrefix(line, "+++ "):
				// Do nothing with this line
				lineBytes, isFragment, err := parseHunks(curFile, maxLines, maxLineCharacters, input)
				diff.TotalAddition += curFile.Addition
				diff.TotalDeletion += curFile.Deletion
				if err != nil {
					if err != io.EOF {
						return diff, err
					}
					break parsingLoop
				}
				sb.Reset()
				_, _ = sb.Write(lineBytes)
				for isFragment {
					lineBytes, isFragment, err = input.ReadLine()
					if err != nil {
						// Now by the definition of ReadLine this cannot be io.EOF
						return diff, fmt.Errorf("Unable to ReadLine: %v", err)
					}
					_, _ = sb.Write(lineBytes)
				}
				line = sb.String()
				sb.Reset()

				break curFileLoop
			}
		}

	}

	// FIXME: There are numerous issues with this:
	// - we might want to consider detecting encoding while parsing but...
	// - we're likely to fail to get the correct encoding here anyway as we won't have enough information
	// - and this doesn't really account for changes in encoding
	var buf bytes.Buffer
	for _, f := range diff.Files {
		buf.Reset()
		for _, sec := range f.Sections {
			for _, l := range sec.Lines {
				if l.Type == DiffLineSection {
					continue
				}
				buf.WriteString(l.Content[1:])
				buf.WriteString("\n")
			}
		}
		charsetLabel, err := charset.DetectEncoding(buf.Bytes())
		if charsetLabel != "UTF-8" && err == nil {
			encoding, _ := stdcharset.Lookup(charsetLabel)
			if encoding != nil {
				d := encoding.NewDecoder()
				for _, sec := range f.Sections {
					for _, l := range sec.Lines {
						if l.Type == DiffLineSection {
							continue
						}
						if c, _, err := transform.String(d, l.Content[1:]); err == nil {
							l.Content = l.Content[0:1] + c
						}
					}
				}
			}
		}
	}

	diff.NumFiles = len(diff.Files)
	return diff, nil
}

func parseHunks(curFile *DiffFile, maxLines, maxLineCharacters int, input *bufio.Reader) (lineBytes []byte, isFragment bool, err error) {
	sb := strings.Builder{}

	var (
		curSection        *DiffSection
		curFileLinesCount int
		curFileLFSPrefix  bool
	)

	leftLine, rightLine := 1, 1

	for {
		for isFragment {
			curFile.IsIncomplete = true
			_, isFragment, err = input.ReadLine()
			if err != nil {
				// Now by the definition of ReadLine this cannot be io.EOF
				err = fmt.Errorf("Unable to ReadLine: %v", err)
				return
			}
		}
		sb.Reset()
		lineBytes, isFragment, err = input.ReadLine()
		if err != nil {
			if err == io.EOF {
				return
			}
			err = fmt.Errorf("Unable to ReadLine: %v", err)
			return
		}
		if lineBytes[0] == 'd' {
			// End of hunks
			return
		}

		switch lineBytes[0] {
		case '@':
			if curFileLinesCount >= maxLines {
				curFile.IsIncomplete = true
				continue
			}

			_, _ = sb.Write(lineBytes)
			for isFragment {
				// This is very odd indeed - we're in a section header and the line is too long
				// This really shouldn't happen...
				lineBytes, isFragment, err = input.ReadLine()
				if err != nil {
					// Now by the definition of ReadLine this cannot be io.EOF
					err = fmt.Errorf("Unable to ReadLine: %v", err)
					return
				}
				_, _ = sb.Write(lineBytes)
			}
			line := sb.String()

			// Create a new section to represent this hunk
			curSection = &DiffSection{}
			curFile.Sections = append(curFile.Sections, curSection)

			lineSectionInfo := getDiffLineSectionInfo(curFile.Name, line, leftLine-1, rightLine-1)
			diffLine := &DiffLine{
				Type:        DiffLineSection,
				Content:     line,
				SectionInfo: lineSectionInfo,
			}
			curSection.Lines = append(curSection.Lines, diffLine)
			curSection.FileName = curFile.Name
			// update line number.
			leftLine = lineSectionInfo.LeftIdx
			rightLine = lineSectionInfo.RightIdx
			continue
		case '\\':
			if curFileLinesCount >= maxLines {
				curFile.IsIncomplete = true
				continue
			}
			// This is used only to indicate that the current file does not have a terminal newline
			if !bytes.Equal(lineBytes, []byte("\\ No newline at end of file")) {
				err = fmt.Errorf("Unexpected line in hunk: %s", string(lineBytes))
				return
			}
			// Technically this should be the end the file!
			// FIXME: we should be putting a marker at the end of the file if there is no terminal new line
			continue
		case '+':
			curFileLinesCount++
			curFile.Addition++
			if curFileLinesCount >= maxLines {
				curFile.IsIncomplete = true
				continue
			}
			diffLine := &DiffLine{Type: DiffLineAdd, RightIdx: rightLine}
			rightLine++
			curSection.Lines = append(curSection.Lines, diffLine)
		case '-':
			curFileLinesCount++
			curFile.Deletion++
			if curFileLinesCount >= maxLines {
				curFile.IsIncomplete = true
				continue
			}
			diffLine := &DiffLine{Type: DiffLineDel, LeftIdx: leftLine}
			if leftLine > 0 {
				leftLine++
			}
			curSection.Lines = append(curSection.Lines, diffLine)
		case ' ':
			curFileLinesCount++
			if curFileLinesCount >= maxLines {
				curFile.IsIncomplete = true
				continue
			}
			diffLine := &DiffLine{Type: DiffLinePlain, LeftIdx: leftLine, RightIdx: rightLine}
			leftLine++
			rightLine++
			curSection.Lines = append(curSection.Lines, diffLine)
		default:
			// This is unexpected
			err = fmt.Errorf("Unexpected line in hunk: %s", string(lineBytes))
			return
		}

		line := string(lineBytes)
		if isFragment {
			curFile.IsIncomplete = true
			for isFragment {
				lineBytes, isFragment, err = input.ReadLine()
				if err != nil {
					// Now by the definition of ReadLine this cannot be io.EOF
					err = fmt.Errorf("Unable to ReadLine: %v", err)
					return
				}
			}
		}
		if len(line) > maxLineCharacters {
			curFile.IsIncomplete = true
			line = line[:maxLineCharacters]
		}
		curSection.Lines[len(curSection.Lines)-1].Content = line

		// handle LFS
		if line[1:] == models.LFSMetaFileIdentifier {
			curFileLFSPrefix = true
		} else if curFileLFSPrefix && strings.HasPrefix(line[1:], models.LFSMetaFileOidPrefix) {
			oid := strings.TrimPrefix(line[1:], models.LFSMetaFileOidPrefix)
			if len(oid) == 64 {
				m := &models.LFSMetaObject{Oid: oid}
				count, err := models.Count(m)

				if err == nil && count > 0 {
					curFile.IsBin = true
					curFile.IsLFSFile = true
					curSection.Lines = nil
				}
			}
		}
	}
}

func createDiffFile(diff *Diff, line string) *DiffFile {
	// The a/ and b/ filenames are the same unless rename/copy is involved.
	// Especially, even for a creation or a deletion, /dev/null is not used
	// in place of the a/ or b/ filenames.
	//
	// When rename/copy is involved, file1 and file2 show the name of the
	// source file of the rename/copy and the name of the file that rename/copy
	// produces, respectively.
	//
	// Path names are quoted if necessary.
	//
	// This means that you should always be able to determine the file name even when there
	// there is potential ambiguity...
	//
	// but we can be simpler with our heuristics by just forcing git to prefix things nicely
	curFile := &DiffFile{
		Index:    len(diff.Files) + 1,
		Type:     DiffFileChange,
		Sections: make([]*DiffSection, 0, 10),
	}

	rd := strings.NewReader(line[len(cmdDiffHead):] + " ")
	curFile.Type = DiffFileChange
	curFile.OldName = readFileName(rd)
	curFile.Name = readFileName(rd)
	curFile.IsRenamed = curFile.Name != curFile.OldName
	return curFile
}

func readFileName(rd *strings.Reader) string {
	var name string
	char, _ := rd.ReadByte()
	_ = rd.UnreadByte()
	if char == '"' {
		fmt.Fscanf(rd, "%q ", &name)
		if name[0] == '\\' {
			name = name[1:]
		}
	} else {
		fmt.Fscanf(rd, "%s ", &name)
	}
	return name[2:]
}

// GetDiffRange builds a Diff between two commits of a repository.
// passing the empty string as beforeCommitID returns a diff from the
// parent commit.
func GetDiffRange(repoPath, beforeCommitID, afterCommitID string, maxLines, maxLineCharacters, maxFiles int, files []string) (*Diff, error) {
	return GetDiffRangeWithWhitespaceBehavior(repoPath, beforeCommitID, afterCommitID, maxLines, maxLineCharacters, maxFiles, files, "")
}

// GetDiffRangeWithWhitespaceBehavior builds a Diff between two commits of a repository.
// Passing the empty string as beforeCommitID returns a diff from the parent commit.
// The whitespaceBehavior is either an empty string or a git flag
func GetDiffRangeWithWhitespaceBehavior(repoPath, beforeCommitID, afterCommitID string, maxLines, maxLineCharacters, maxFiles int, files []string, whitespaceBehavior string) (*Diff, error) {
	gitRepo, err := git.OpenRepository(repoPath)
	if err != nil {
		return nil, err
	}
	defer gitRepo.Close()

	commit, err := gitRepo.GetCommit(afterCommitID)
	if err != nil {
		return nil, err
	}

	var (
		diff          = &Diff{}
		vendor        = []*DiffFile{}
		filenames     []string
		filesChanged  = &git.DiffFiles{}
		numFiles      int
		totalAddition int
		totalDeletion int
	)

	// we have been given a specific list of file(s) to diff
	// this can be used in the future to process diff in batches since we will have the entire list
	// of files after the initial GetDiffAllStats run.
	if len(files) > 0 {
		for _, v := range files {
			fileName, err := strconv.Unquote(v)
			if err == nil {
				v = fileName
			}
			filenames = append(filenames, v)
		}
	} else {
		shortstatArgs := []string{beforeCommitID, afterCommitID}
		if len(beforeCommitID) == 0 || beforeCommitID == git.EmptySHA {
			shortstatArgs = []string{git.EmptyTreeSHA, afterCommitID}
		}

		numFiles, totalAddition, totalDeletion, filesChanged, err = git.GetDiffAllStats(repoPath, shortstatArgs...)
		if err != nil && strings.Contains(err.Error(), "no merge base") {
			// git >= 2.28 now returns an error if base and head have become unrelated.
			// previously it would return the results of git diff --shortstat base head so let's try that...
			shortstatArgs = []string{beforeCommitID, afterCommitID}
			numFiles, totalAddition, totalDeletion, filesChanged, err = git.GetDiffAllStats(repoPath, shortstatArgs...)
		}
		if err != nil {
			return nil, err
		}
	}

	for _, file := range filesChanged.Files {
		fileName, err := strconv.Unquote(file.Name)
		if err == nil {
			file.Name = fileName
		}
		// separate vendored and generated files and do not display them by default
		if analyze.DiffIgnore(file.Name) {
			curFile := &DiffFile{}
			curFile.IsIgnored = true
			curFile.Name = file.Name
			curFile.Type = DiffFileType(file.Type)
			curFile.IsIncomplete = false
			curFile.IsCreated = file.IsCreated
			curFile.IsDeleted = file.IsDeleted
			curFile.IsRenamed = file.IsRenamed
			if !file.IsBin {
				curFile.Addition = file.Addition
				curFile.Deletion = file.Deletion
			}

			vendor = append(vendor, curFile)
		} else {
			filenames = append(filenames, file.Name)
			if file.IsRenamed {
				oldName, err := strconv.Unquote(file.OldName)
				if err != nil {
					filenames = append(filenames, file.OldName)
				} else {
					filenames = append(filenames, oldName)
				}
			}
		}
	}

	// FIXME: graceful: These commands should likely have a timeout
	ctx, cancel := context.WithCancel(git.DefaultContext)
	defer cancel()
	var cmd *exec.Cmd
	if (len(beforeCommitID) == 0 || beforeCommitID == git.EmptySHA) && commit.ParentCount() == 0 {
		diffArgs := []string{"diff", "--src-prefix=\\a/", "--dst-prefix=\\b/", "-M"}
		if len(whitespaceBehavior) != 0 {
			diffArgs = append(diffArgs, whitespaceBehavior)
		}
		// append empty tree ref
		diffArgs = append(diffArgs, "4b825dc642cb6eb9a060e54bf8d69288fbee4904")
		diffArgs = append(diffArgs, afterCommitID)
		diffArgs = append(diffArgs, "--")
		diffArgs = append(diffArgs, filenames...)
		cmd = exec.CommandContext(ctx, git.GitExecutable, diffArgs...)
	} else {
		actualBeforeCommitID := beforeCommitID
		if len(actualBeforeCommitID) == 0 {
			parentCommit, _ := commit.Parent(0)
			actualBeforeCommitID = parentCommit.ID.String()
		}
		diffArgs := []string{"diff", "--src-prefix=\\a/", "--dst-prefix=\\b/", "-M"}
		if len(whitespaceBehavior) != 0 {
			diffArgs = append(diffArgs, whitespaceBehavior)
		}
		diffArgs = append(diffArgs, actualBeforeCommitID)
		diffArgs = append(diffArgs, afterCommitID)
		diffArgs = append(diffArgs, "--")
		diffArgs = append(diffArgs, filenames...)
		cmd = exec.CommandContext(ctx, git.GitExecutable, diffArgs...)
		beforeCommitID = actualBeforeCommitID
	}

	cmd.Dir = repoPath
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("StdoutPipe: %v", err)
	}

	if err = cmd.Start(); err != nil {
		return nil, fmt.Errorf("Start: %v", err)
	}

	pid := process.GetManager().Add(fmt.Sprintf("GetDiffRange [repo_path: %s]", repoPath), cancel)
	defer process.GetManager().Remove(pid)

	diff, err = ParsePatch(maxLines, maxLineCharacters, maxFiles, stdout)
	if err != nil {
		return nil, fmt.Errorf("ParsePatch: %v", err)
	}
	for _, diffFile := range diff.Files {
		tailSection := diffFile.GetTailSection(gitRepo, beforeCommitID, afterCommitID)
		if tailSection != nil {
			diffFile.Sections = append(diffFile.Sections, tailSection)
		}
	}

	diff.Files = append(diff.Files, vendor...)
	// re-set these here because they are still modified in parsePatch for now
	diff.NumFiles = numFiles
	diff.TotalAddition = totalAddition
	diff.TotalDeletion = totalDeletion

	if err = cmd.Wait(); err != nil {
		return nil, fmt.Errorf("Wait: %v", err)
	}

	return diff, nil
}

// GetDiffCommit builds a Diff representing the given commitID.
func GetDiffCommit(repoPath, parent, commitID string, maxLines, maxLineCharacters, maxFiles int) (*Diff, error) {
	return GetDiffRange(repoPath, parent, commitID, maxLines, maxLineCharacters, maxFiles, nil)
}

// CommentAsDiff returns c.Patch as *Diff
func CommentAsDiff(c *models.Comment) (*Diff, error) {
	diff, err := ParsePatch(setting.Git.MaxGitDiffLines,
		setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(c.Patch))
	if err != nil {
		return nil, err
	}
	if len(diff.Files) == 0 {
		return nil, fmt.Errorf("no file found for comment ID: %d", c.ID)
	}
	secs := diff.Files[0].Sections
	if len(secs) == 0 {
		return nil, fmt.Errorf("no sections found for comment ID: %d", c.ID)
	}
	return diff, nil
}

// CommentMustAsDiff executes AsDiff and logs the error instead of returning
func CommentMustAsDiff(c *models.Comment) *Diff {
	diff, err := CommentAsDiff(c)
	if err != nil {
		log.Warn("CommentMustAsDiff: %v", err)
	}
	return diff
}
