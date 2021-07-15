// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"context"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/storage"
)

// DeleteRepositoryArchives deletes all repositories' archives.
func DeleteRepositoryArchives(ctx context.Context) error {
	if err := models.DeleteAllRepoArchives(); err != nil {
		return err
	}
	return storage.Clean(storage.RepoArchives)
}
