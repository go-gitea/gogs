// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"sort"
	"strings"
)

const (
	beginpgp = "\n-----BEGIN PGP SIGNATURE-----\n"
	endpgp   = "\n-----END PGP SIGNATURE-----"
)

// Tag represents a Git tag.
type Tag struct {
	Name      string
	ID        SHA1
	repo      *Repository
	Object    SHA1 // The id of this commit object
	Type      string
	Tagger    *Signature
	Message   string
	Signature *CommitGPGSignature
}

// Commit return the commit of the tag reference
func (tag *Tag) Commit() (*Commit, error) {
	return tag.repo.getCommit(tag.Object)
}

// Parse commit information from the (uncompressed) raw
// data from the commit object.
// \n\n separate headers from message
func parseTagData(data []byte) (*Tag, error) {
	tag := new(Tag)
	// we now have the contents of the commit object. Let's investigate...
	nextline := 0
l:
	for {
		eol := bytes.IndexByte(data[nextline:], '\n')
		switch {
		case eol > 0:
			line := data[nextline : nextline+eol]
			spacepos := bytes.IndexByte(line, ' ')
			reftype := line[:spacepos]
			switch string(reftype) {
			case "object":
				id, err := NewIDFromString(string(line[spacepos+1:]))
				if err != nil {
					return nil, err
				}
				tag.Object = id
			case "type":
				// A commit can have one or more parents
				tag.Type = string(line[spacepos+1:])
			case "tagger":
				sig, err := newSignatureFromCommitline(line[spacepos+1:])
				if err != nil {
					return nil, err
				}
				tag.Tagger = sig
			}
			nextline += eol + 1
		case eol == 0:
			tag.Message = string(data[nextline+1:])
			break l
		default:
			break l
		}
	}
	idx := strings.LastIndex(tag.Message, beginpgp)
	if idx > 0 {
		endSigIdx := strings.Index(tag.Message[idx:], endpgp)
		if endSigIdx > 0 {
			tag.Signature = &CommitGPGSignature{
				Signature: tag.Message[idx+1 : idx+endSigIdx+len(endpgp)],
				Payload:   string(data[:bytes.LastIndex(data, []byte(beginpgp))+1]),
			}
			tag.Message = tag.Message[:idx+1]
		}
	}
	return tag, nil
}

type tagSorter []*Tag

func (ts tagSorter) Len() int {
	return len([]*Tag(ts))
}

func (ts tagSorter) Less(i, j int) bool {
	return []*Tag(ts)[i].Tagger.When.After([]*Tag(ts)[j].Tagger.When)
}

func (ts tagSorter) Swap(i, j int) {
	[]*Tag(ts)[i], []*Tag(ts)[j] = []*Tag(ts)[j], []*Tag(ts)[i]
}

// sortTagsByTime
func sortTagsByTime(tags []*Tag) {
	sorter := tagSorter(tags)
	sort.Sort(sorter)
}
