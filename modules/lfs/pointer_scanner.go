// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build gogit

package lfs

import (
	"io"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/git"

	"github.com/go-git/go-git/v5/plumbing/object"
)

const (
	blobSizeCutoff = 1024

	// TODO remove duplicate from models

	// LFSMetaFileIdentifier is the string appearing at the first line of LFS pointer files.
	// https://github.com/git-lfs/git-lfs/blob/master/docs/spec.md
	LFSMetaFileIdentifier = "version https://git-lfs.github.com/spec/v1"

	// LFSMetaFileOidPrefix appears in LFS pointer files on a line before the sha256 hash.
	LFSMetaFileOidPrefix = "oid sha256:"
)

// TryReadPointer tries to read LFS pointer data from the reader
func TryReadPointer(reader io.Reader) *Pointer {
	buf := make([]byte, blobSizeCutoff)
	n, _ := reader.Read(buf)
	buf = buf[:n]

	return TryReadPointerFromBuffer(buf)
}

// TryReadPointerFromBuffer will return a pointer if the provided byte slice is a pointer file or nil otherwise.
func TryReadPointerFromBuffer(buf []byte) *Pointer {
	headString := string(buf)
	if !strings.HasPrefix(headString, LFSMetaFileIdentifier) {
		return nil
	}

	splitLines := strings.Split(headString, "\n")
	if len(splitLines) < 3 {
		return nil
	}

	oid := strings.TrimPrefix(splitLines[1], LFSMetaFileOidPrefix)
	size, err := strconv.ParseInt(strings.TrimPrefix(splitLines[2], "size "), 10, 64)
	if len(oid) != 64 || err != nil {
		return nil
	}

	return &Pointer{Oid: oid, Size: size}
}

// SearchPointerFiles scans the whole repository for LFS pointer files
func SearchPointerFiles(repo *git.Repository) ([]*Pointer, error) {
	gitRepo := repo.GoGitRepo()

	blobs, err := gitRepo.BlobObjects()
	if err != nil {
		return nil, err
	}

	var pointers []*Pointer

	err = blobs.ForEach(func(blob *object.Blob) error {
		if blob.Size > blobSizeCutoff {
			return nil
		}

		reader, err := blob.Reader()
		if err != nil {
			return nil
		}
		defer reader.Close()

		pointer := TryReadPointer(reader)
		if pointer != nil {
			pointers = append(pointers, pointer)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return pointers, nil
}