// Copyright 2020 The Gitea Authors. All rights reserved.
// Copyright 2015 Kenneth Shaw
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package emoji

import (
	"io"
	"sort"
	"strings"
	"sync"
)

// Gemoji is a set of emoji data.
type Gemoji []Emoji

// Emoji represents a single emoji and associated data.
type Emoji struct {
	Emoji          string
	Description    string
	Aliases        []string
	UnicodeVersion string
	SkinTones      bool
}

var (
	// codeMap provides a map of the emoji unicode code to its emoji data.
	codeMap map[string]int

	// aliasMap provides a map of the alias to its emoji data.
	aliasMap map[string]int

	// emptyReplacer is the string replacer for emoji codes.
	emptyReplacer *strings.Replacer

	// codeReplacer is the string replacer for emoji codes.
	codeReplacer *strings.Replacer

	// aliasReplacer is the string replacer for emoji aliases.
	aliasReplacer *strings.Replacer

	once sync.Once
)

func loadMap() {

	once.Do(func() {

		// initialize
		codeMap = make(map[string]int, len(GemojiData))
		aliasMap = make(map[string]int, len(GemojiData))

		// process emoji codes and aliases
		codePairs := make([]string, 0)
		emptyPairs := make([]string, 0)
		aliasPairs := make([]string, 0)

		// sort from largest to small so we match combined emoji first
		sort.Slice(GemojiData, func(i, j int) bool {
			return len(GemojiData[i].Emoji) > len(GemojiData[j].Emoji)
		})

		for i, e := range GemojiData {
			if e.Emoji == "" || len(e.Aliases) == 0 {
				continue
			}

			// setup codes
			codeMap[e.Emoji] = i
			codePairs = append(codePairs, e.Emoji, ":"+e.Aliases[0]+":")
			emptyPairs = append(emptyPairs, e.Emoji, e.Emoji)

			// setup aliases
			for _, a := range e.Aliases {
				if a == "" {
					continue
				}

				aliasMap[a] = i
				aliasPairs = append(aliasPairs, ":"+a+":", e.Emoji)
			}
		}

		// create replacers
		emptyReplacer = strings.NewReplacer(emptyPairs...)
		codeReplacer = strings.NewReplacer(codePairs...)
		aliasReplacer = strings.NewReplacer(aliasPairs...)
	})

}

// FromCode retrieves the emoji data based on the provided unicode code (ie,
// "\u2618" will return the Gemoji data for "shamrock").
func FromCode(code string) *Emoji {
	loadMap()
	i, ok := codeMap[code]
	if !ok {
		return nil
	}

	return &GemojiData[i]
}

// FromAlias retrieves the emoji data based on the provided alias in the form
// "alias" or ":alias:" (ie, "shamrock" or ":shamrock:" will return the Gemoji
// data for "shamrock").
func FromAlias(alias string) *Emoji {
	loadMap()
	if strings.HasPrefix(alias, ":") && strings.HasSuffix(alias, ":") {
		alias = alias[1 : len(alias)-1]
	}

	i, ok := aliasMap[alias]
	if !ok {
		return nil
	}

	return &GemojiData[i]
}

// ReplaceCodes replaces all emoji codes with the first corresponding emoji
// alias (in the form of ":alias:") (ie, "\u2618" will be converted to
// ":shamrock:").
func ReplaceCodes(s string) string {
	loadMap()
	return codeReplacer.Replace(s)
}

// ReplaceAliases replaces all aliases of the form ":alias:" with its
// corresponding unicode value.
func ReplaceAliases(s string) string {
	loadMap()
	return aliasReplacer.Replace(s)
}

type rememberSecondWriteWriter struct {
	pos        int
	idx        int
	end        int
	writecount int
}

func (n *rememberSecondWriteWriter) Write(p []byte) (int, error) {
	n.writecount++
	if n.writecount == 2 {
		n.idx = n.pos
		n.end = n.pos + len(p)
		n.pos += len(p)
		return len(p), io.EOF
	}
	n.pos += len(p)
	return len(p), nil
}

func (n *rememberSecondWriteWriter) WriteString(s string) (int, error) {
	n.writecount++
	if n.writecount == 2 {
		n.idx = n.pos
		n.end = n.pos + len(s)
		n.pos += len(s)
		return len(s), io.EOF
	}
	n.pos += len(s)
	return len(s), nil
}

// FindEmojiSubmatchIndex returns index pair of longest emoji in a string
func FindEmojiSubmatchIndex(s string) []int {
	loadMap()
	secondWriteWriter := rememberSecondWriteWriter{}

	// A faster and clean implementation would copy the trie tree formation in strings.NewReplacer but
	// we can be lazy here.
	//
	// The implementation of strings.Replacer.WriteString is such that the first index of the emoji
	// submatch is simply the second thing that is written to WriteString in the writer.
	//
	// Therefore we can simply take the index of the second write as our first emoji
	//
	// FIXME: just copy the trie implementation from strings.NewReplacer
	_, _ = emptyReplacer.WriteString(&secondWriteWriter, s)

	// if we wrote less than twice then we never "replaced"
	if secondWriteWriter.writecount < 2 {
		return nil
	}

	return []int{secondWriteWriter.idx, secondWriteWriter.end}
}
