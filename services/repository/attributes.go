// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"code.gitea.io/gitea/modules/git"
)

// SyncGitAttributes copies the content of the .gitattributes file from the default branch into repo.git/info/attributes.
func SyncGitAttributes(gitRepo *git.Repository, sourceBranch string) error {
	commit, err := gitRepo.GetBranchCommit(sourceBranch)
	if err != nil {
		return err
	}

	attributesBlob, err := commit.GetBlobByPath("/.gitattributes")
	if err != nil {
		if git.IsErrNotExist(err) {
			return nil
		}
		return err
	}

	infoPath := filepath.Join(gitRepo.Path, "info")
	if err := os.MkdirAll(infoPath, 0700); err != nil {
		return fmt.Errorf("Error creating directory [%s]: %v", infoPath, err)
	}
	attributesPath := filepath.Join(infoPath, "attributes")

	attributesFile, err := os.OpenFile(attributesPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("Error creating file [%s]: %v", attributesPath, err)
	}
	defer attributesFile.Close()

	blobReader, err := attributesBlob.DataAsync()
	if err != nil {
		return err
	}
	defer blobReader.Close()

	_, err = io.Copy(attributesFile, blobReader)
	return err
}
