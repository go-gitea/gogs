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
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/highlight"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"

	"github.com/sergi/go-diff/diffmatchpatch"
	stdcharset "golang.org/x/net/html/charset"
	"golang.org/x/text/encoding"
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

var unfinishedtagRegex = regexp.MustCompile(`<[^>]*$`)
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

func fixupBrokenSpans(diffs []diffmatchpatch.Diff) []diffmatchpatch.Diff {

	// Create a new array to store our fixed up blocks
	fixedup := make([]diffmatchpatch.Diff, 0, len(diffs))

	// semantically label some numbers
	const insert, delete, equal = 0, 1, 2

	// record the positions of the last type of each block in the fixedup blocks
	last := []int{-1, -1, -1}
	operation := []diffmatchpatch.Operation{diffmatchpatch.DiffInsert, diffmatchpatch.DiffDelete, diffmatchpatch.DiffEqual}

	// create a writer for insert and deletes
	toWrite := []strings.Builder{
		{},
		{},
	}

	// make some flags for insert and delete
	unfinishedTag := []bool{false, false}
	unfinishedEnt := []bool{false, false}

	// store stores the provided text in the writer for the typ
	store := func(text string, typ int) {
		(&(toWrite[typ])).WriteString(text)
	}

	// hasStored returns true if there is stored content
	hasStored := func(typ int) bool {
		return (&toWrite[typ]).Len() > 0
	}

	// stored will return that content
	stored := func(typ int) string {
		return (&toWrite[typ]).String()
	}

	// empty will empty the stored content
	empty := func(typ int) {
		(&toWrite[typ]).Reset()
	}

	// pop will remove the stored content appending to a diff block for that typ
	pop := func(typ int, fixedup []diffmatchpatch.Diff) []diffmatchpatch.Diff {
		if hasStored(typ) {
			if last[typ] > last[equal] {
				fixedup[last[typ]].Text += stored(typ)
			} else {
				fixedup = append(fixedup, diffmatchpatch.Diff{
					Type: operation[typ],
					Text: stored(typ),
				})
			}
			empty(typ)
		}
		return fixedup
	}

	// Now we walk the provided diffs and check the type of each block in turn
	for _, diff := range diffs {

		typ := delete // flag for handling insert or delete typs
		switch diff.Type {
		case diffmatchpatch.DiffEqual:
			// First check if there is anything stored
			if hasStored(insert) || hasStored(delete) {
				// There are two reasons for storing content:
				// 1. Unfinished Entity <- Could be more efficient here by not doing this if we're looking for a tag
				if unfinishedEnt[insert] || unfinishedEnt[delete] {
					// we look for a ';' to finish an entity
					idx := strings.IndexRune(diff.Text, ';')
					if idx >= 0 {
						// if we find a ';' store the preceding content to both insert and delete
						store(diff.Text[:idx+1], insert)
						store(diff.Text[:idx+1], delete)

						// and remove it from this block
						diff.Text = diff.Text[idx+1:]

						// reset the ent flags
						unfinishedEnt[insert] = false
						unfinishedEnt[delete] = false
					} else {
						// otherwise store it all on insert and delete
						store(diff.Text, insert)
						store(diff.Text, delete)
						// and empty this block
						diff.Text = ""
					}
				}
				// 2. Unfinished Tag
				if unfinishedTag[insert] || unfinishedTag[delete] {
					// we look for a '>' to finish a tag
					idx := strings.IndexRune(diff.Text, '>')
					if idx >= 0 {
						store(diff.Text[:idx+1], insert)
						store(diff.Text[:idx+1], delete)
						diff.Text = diff.Text[idx+1:]
						unfinishedTag[insert] = false
						unfinishedTag[delete] = false
					} else {
						store(diff.Text, insert)
						store(diff.Text, delete)
						diff.Text = ""
					}
				}

				// If we've completed the required tag/entities
				if !(unfinishedTag[insert] || unfinishedTag[delete] || unfinishedEnt[insert] || unfinishedEnt[delete]) {
					// pop off the stack
					fixedup = pop(insert, fixedup)
					fixedup = pop(delete, fixedup)
				}

				// If that has left this diff block empty then shortcut
				if len(diff.Text) == 0 {
					continue
				}
			}

			// check if this block ends in an unfinished tag?
			idx := unfinishedtagRegex.FindStringIndex(diff.Text)
			if idx != nil {
				unfinishedTag[insert] = true
				unfinishedTag[delete] = true
			} else {
				// otherwise does it end in an unfinished entity?
				idx = entityRegex.FindStringIndex(diff.Text)
				if idx != nil {
					unfinishedEnt[insert] = true
					unfinishedEnt[delete] = true
				}
			}

			// If there is an unfinished component
			if idx != nil {
				// Store the fragment
				store(diff.Text[idx[0]:], insert)
				store(diff.Text[idx[0]:], delete)
				// and remove it from this block
				diff.Text = diff.Text[:idx[0]]
			}

			// If that hasn't left the block empty
			if len(diff.Text) > 0 {
				// store the position of the last equal block and store it in our diffs
				last[equal] = len(fixedup)
				fixedup = append(fixedup, diff)
			}
			continue
		case diffmatchpatch.DiffInsert:
			typ = insert
			fallthrough
		case diffmatchpatch.DiffDelete:
			// First check if there is anything stored for this type
			if hasStored(typ) {
				// if there is prepend it to this block, empty the storage and reset our flags
				diff.Text = stored(typ) + diff.Text
				empty(typ)
				unfinishedEnt[typ] = false
				unfinishedTag[typ] = false
			}

			// check if this block ends in an unfinished tag
			idx := unfinishedtagRegex.FindStringIndex(diff.Text)
			if idx != nil {
				unfinishedTag[typ] = true
			} else {
				// otherwise does it end in an unfinished entity
				idx = entityRegex.FindStringIndex(diff.Text)
				if idx != nil {
					unfinishedEnt[typ] = true
				}
			}

			// If there is an unfinished component
			if idx != nil {
				// Store the fragment
				store(diff.Text[idx[0]:], typ)
				// and remove it from this block
				diff.Text = diff.Text[:idx[0]]
			}

			// If that hasn't left the block empty
			if len(diff.Text) > 0 {
				// if the last block of this type was after the last equal block
				if last[typ] > last[equal] {
					// store this blocks content on that block
					fixedup[last[typ]].Text += diff.Text
				} else {
					// otherwise store the position of the last block of this type and store the block
					last[typ] = len(fixedup)
					fixedup = append(fixedup, diff)
				}
			}
			continue
		}
	}

	// pop off any remaining stored content
	fixedup = pop(insert, fixedup)
	fixedup = pop(delete, fixedup)

	return fixedup
}

func diffToHTML(fileName string, diffs []diffmatchpatch.Diff, lineType DiffLineType) template.HTML {
	buf := bytes.NewBuffer(nil)
	match := ""

	diffs = fixupBrokenSpans(diffs)

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
	Name                    string
	OldName                 string
	Index                   int
	Addition, Deletion      int
	Type                    DiffFileType
	IsCreated               bool
	IsDeleted               bool
	IsBin                   bool
	IsLFSFile               bool
	IsRenamed               bool
	IsAmbiguous             bool
	IsSubmodule             bool
	Sections                []*DiffSection
	IsIncomplete            bool
	IsIncompleteLineTooLong bool
	IsProtected             bool
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
			case strings.HasPrefix(line, "rename from "):
				curFile.IsRenamed = true
				curFile.Type = DiffFileRename
				if curFile.IsAmbiguous {
					curFile.OldName = line[len("rename from ") : len(line)-1]
				}
			case strings.HasPrefix(line, "rename to "):
				curFile.IsRenamed = true
				curFile.Type = DiffFileRename
				if curFile.IsAmbiguous {
					curFile.Name = line[len("rename to ") : len(line)-1]
					curFile.IsAmbiguous = false
				}
			case strings.HasPrefix(line, "copy from "):
				curFile.IsRenamed = true
				curFile.Type = DiffFileCopy
				if curFile.IsAmbiguous {
					curFile.OldName = line[len("copy from ") : len(line)-1]
				}
			case strings.HasPrefix(line, "copy to "):
				curFile.IsRenamed = true
				curFile.Type = DiffFileCopy
				if curFile.IsAmbiguous {
					curFile.Name = line[len("copy to ") : len(line)-1]
					curFile.IsAmbiguous = false
				}
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
				// Handle ambiguous filenames
				if curFile.IsAmbiguous {
					if len(line) > 6 && line[4] == 'a' {
						curFile.OldName = line[6 : len(line)-1]
						if line[len(line)-2] == '\t' {
							curFile.OldName = curFile.OldName[:len(curFile.OldName)-1]
						}
					} else {
						curFile.OldName = ""
					}
				}
				// Otherwise do nothing with this line
			case strings.HasPrefix(line, "+++ "):
				// Handle ambiguous filenames
				if curFile.IsAmbiguous {
					if len(line) > 6 && line[4] == 'b' {
						curFile.Name = line[6 : len(line)-1]
						if line[len(line)-2] == '\t' {
							curFile.Name = curFile.Name[:len(curFile.Name)-1]
						}
						if curFile.OldName == "" {
							curFile.OldName = curFile.Name
						}
					} else {
						curFile.Name = curFile.OldName
					}
					curFile.IsAmbiguous = false
				}
				// Otherwise do nothing with this line, but now switch to parsing hunks
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

	// TODO: There are numerous issues with this:
	// - we might want to consider detecting encoding while parsing but...
	// - we're likely to fail to get the correct encoding here anyway as we won't have enough information
	var diffLineTypeBuffers = make(map[DiffLineType]*bytes.Buffer, 3)
	var diffLineTypeDecoders = make(map[DiffLineType]*encoding.Decoder, 3)
	diffLineTypeBuffers[DiffLinePlain] = new(bytes.Buffer)
	diffLineTypeBuffers[DiffLineAdd] = new(bytes.Buffer)
	diffLineTypeBuffers[DiffLineDel] = new(bytes.Buffer)
	for _, f := range diff.Files {
		for _, buffer := range diffLineTypeBuffers {
			buffer.Reset()
		}
		for _, sec := range f.Sections {
			for _, l := range sec.Lines {
				if l.Type == DiffLineSection {
					continue
				}
				diffLineTypeBuffers[l.Type].WriteString(l.Content[1:])
				diffLineTypeBuffers[l.Type].WriteString("\n")
			}
		}
		for lineType, buffer := range diffLineTypeBuffers {
			diffLineTypeDecoders[lineType] = nil
			if buffer.Len() == 0 {
				continue
			}
			charsetLabel, err := charset.DetectEncoding(buffer.Bytes())
			if charsetLabel != "UTF-8" && err == nil {
				encoding, _ := stdcharset.Lookup(charsetLabel)
				if encoding != nil {
					diffLineTypeDecoders[lineType] = encoding.NewDecoder()
				}
			}
		}
		for _, sec := range f.Sections {
			for _, l := range sec.Lines {
				decoder := diffLineTypeDecoders[l.Type]
				if decoder != nil {
					if c, _, err := transform.String(decoder, l.Content[1:]); err == nil {
						l.Content = l.Content[0:1] + c
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
			curFile.IsIncompleteLineTooLong = true
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
			if curSection == nil {
				// Create a new section to represent this hunk
				curSection = &DiffSection{}
				curFile.Sections = append(curFile.Sections, curSection)
			}
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
			if curSection == nil {
				// Create a new section to represent this hunk
				curSection = &DiffSection{}
				curFile.Sections = append(curFile.Sections, curSection)
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
			if curSection == nil {
				// Create a new section to represent this hunk
				curSection = &DiffSection{}
				curFile.Sections = append(curFile.Sections, curSection)
			}
			curSection.Lines = append(curSection.Lines, diffLine)
		default:
			// This is unexpected
			err = fmt.Errorf("Unexpected line in hunk: %s", string(lineBytes))
			return
		}

		line := string(lineBytes)
		if isFragment {
			curFile.IsIncomplete = true
			curFile.IsIncompleteLineTooLong = true
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
			curFile.IsIncompleteLineTooLong = true
			line = line[:maxLineCharacters]
		}
		curSection.Lines[len(curSection.Lines)-1].Content = line

		// handle LFS
		if line[1:] == lfs.MetaFileIdentifier {
			curFileLFSPrefix = true
		} else if curFileLFSPrefix && strings.HasPrefix(line[1:], lfs.MetaFileOidPrefix) {
			oid := strings.TrimPrefix(line[1:], lfs.MetaFileOidPrefix)
			if len(oid) == 64 {
				m := &models.LFSMetaObject{Pointer: lfs.Pointer{Oid: oid}}
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
	oldNameAmbiguity := false
	newNameAmbiguity := false

	curFile.OldName, oldNameAmbiguity = readFileName(rd)
	curFile.Name, newNameAmbiguity = readFileName(rd)
	if oldNameAmbiguity && newNameAmbiguity {
		curFile.IsAmbiguous = true
		// OK we should bet that the oldName and the newName are the same if they can be made to be same
		// So we need to start again ...
		if (len(line)-len(cmdDiffHead)-1)%2 == 0 {
			// diff --git a/b b/b b/b b/b b/b b/b
			//
			midpoint := (len(line) + len(cmdDiffHead) - 1) / 2
			new, old := line[len(cmdDiffHead):midpoint], line[midpoint+1:]
			if len(new) > 2 && len(old) > 2 && new[2:] == old[2:] {
				curFile.OldName = old[2:]
				curFile.Name = old[2:]
			}
		}
	}

	curFile.IsRenamed = curFile.Name != curFile.OldName
	return curFile
}

func readFileName(rd *strings.Reader) (string, bool) {
	ambiguity := false
	var name string
	char, _ := rd.ReadByte()
	_ = rd.UnreadByte()
	if char == '"' {
		fmt.Fscanf(rd, "%q ", &name)
		if name[0] == '\\' {
			name = name[1:]
		}
	} else {
		// This technique is potentially ambiguous it may not be possible to uniquely identify the filenames from the diff line alone
		ambiguity = true
		fmt.Fscanf(rd, "%s ", &name)
		char, _ := rd.ReadByte()
		_ = rd.UnreadByte()
		for !(char == 0 || char == '"' || char == 'b') {
			var suffix string
			fmt.Fscanf(rd, "%s ", &suffix)
			name += " " + suffix
			char, _ = rd.ReadByte()
			_ = rd.UnreadByte()
		}
	}
	if len(name) < 2 {
		log.Error("Unable to determine name from reader: %v", rd)
		return "", true
	}
	return name[2:], ambiguity
}

// GetDiffRange builds a Diff between two commits of a repository.
// passing the empty string as beforeCommitID returns a diff from the
// parent commit.
func GetDiffRange(repoPath, beforeCommitID, afterCommitID string, maxLines, maxLineCharacters, maxFiles int) (*Diff, error) {
	return GetDiffRangeWithWhitespaceBehavior(repoPath, beforeCommitID, afterCommitID, maxLines, maxLineCharacters, maxFiles, "")
}

// GetDiffRangeWithWhitespaceBehavior builds a Diff between two commits of a repository.
// Passing the empty string as beforeCommitID returns a diff from the parent commit.
// The whitespaceBehavior is either an empty string or a git flag
func GetDiffRangeWithWhitespaceBehavior(repoPath, beforeCommitID, afterCommitID string, maxLines, maxLineCharacters, maxFiles int, whitespaceBehavior string) (*Diff, error) {
	gitRepo, err := git.OpenRepository(repoPath)
	if err != nil {
		return nil, err
	}
	defer gitRepo.Close()

	commit, err := gitRepo.GetCommit(afterCommitID)
	if err != nil {
		return nil, err
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

	diff, err := ParsePatch(maxLines, maxLineCharacters, maxFiles, stdout)
	if err != nil {
		return nil, fmt.Errorf("ParsePatch: %v", err)
	}
	for _, diffFile := range diff.Files {
		tailSection := diffFile.GetTailSection(gitRepo, beforeCommitID, afterCommitID)
		if tailSection != nil {
			diffFile.Sections = append(diffFile.Sections, tailSection)
		}
	}

	if err = cmd.Wait(); err != nil {
		return nil, fmt.Errorf("Wait: %v", err)
	}

	shortstatArgs := []string{beforeCommitID + "..." + afterCommitID}
	if len(beforeCommitID) == 0 || beforeCommitID == git.EmptySHA {
		shortstatArgs = []string{git.EmptyTreeSHA, afterCommitID}
	}
	diff.NumFiles, diff.TotalAddition, diff.TotalDeletion, err = git.GetDiffShortStat(repoPath, shortstatArgs...)
	if err != nil && strings.Contains(err.Error(), "no merge base") {
		// git >= 2.28 now returns an error if base and head have become unrelated.
		// previously it would return the results of git diff --shortstat base head so let's try that...
		shortstatArgs = []string{beforeCommitID, afterCommitID}
		diff.NumFiles, diff.TotalAddition, diff.TotalDeletion, err = git.GetDiffShortStat(repoPath, shortstatArgs...)
	}
	if err != nil {
		return nil, err
	}

	return diff, nil
}

// GetDiffCommit builds a Diff representing the given commitID.
func GetDiffCommit(repoPath, commitID string, maxLines, maxLineCharacters, maxFiles int) (*Diff, error) {
	return GetDiffRangeWithWhitespaceBehavior(repoPath, "", commitID, maxLines, maxLineCharacters, maxFiles, "")
}

// GetDiffCommitWithWhitespaceBehavior builds a Diff representing the given commitID.
// The whitespaceBehavior is either an empty string or a git flag
func GetDiffCommitWithWhitespaceBehavior(repoPath, commitID string, maxLines, maxLineCharacters, maxFiles int, whitespaceBehavior string) (*Diff, error) {
	return GetDiffRangeWithWhitespaceBehavior(repoPath, "", commitID, maxLines, maxLineCharacters, maxFiles, whitespaceBehavior)
}

// CommentAsDiff returns c.Patch as *Diff
func CommentAsDiff(c *models.Comment) (*Diff, error) {
	diff, err := ParsePatch(setting.Git.MaxGitDiffLines,
		setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, strings.NewReader(c.Patch))
	if err != nil {
		log.Error("Unable to parse patch: %v", err)
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
	if c == nil {
		return nil
	}
	defer func() {
		if err := recover(); err != nil {
			log.Error("PANIC whilst retrieving diff for comment[%d] Error: %v\nStack: %s", c.ID, err, log.Stack(2))
		}
	}()
	diff, err := CommentAsDiff(c)
	if err != nil {
		log.Warn("CommentMustAsDiff: %v", err)
	}
	return diff
}

// GetWhitespaceFlag returns git diff flag for treating whitespaces
func GetWhitespaceFlag(whiteSpaceBehavior string) string {
	whitespaceFlags := map[string]string{
		"ignore-all":    "-w",
		"ignore-change": "-b",
		"ignore-eol":    "--ignore-space-at-eol",
		"":              ""}

	return whitespaceFlags[whiteSpaceBehavior]
}
