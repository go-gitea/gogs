// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddDeletedBranch(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	firstBranch := AssertExistsAndLoadBean(t, &DeletedBranch{ID: 1}).(*DeletedBranch)

	assert.Error(t, repo.AddDeletedBranch(firstBranch.Name, firstBranch.Commit, firstBranch.DeletedByID))
	assert.NoError(t, repo.AddDeletedBranch("test", "5655464564554545466464656", int64(1)))
}

func TestGetDeletedBranches(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)

	branches, err := repo.GetDeletedBranches()
	assert.NoError(t, err)
	assert.Len(t, branches, 2)
}

func TestGetDeletedBranch(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	firstBranch := AssertExistsAndLoadBean(t, &DeletedBranch{ID: 1}).(*DeletedBranch)

	assert.NotNil(t, getDeletedBranch(t, firstBranch))
}

func TestDeletedBranchLoadUser(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	firstBranch := AssertExistsAndLoadBean(t, &DeletedBranch{ID: 1}).(*DeletedBranch)
	secondBranch := AssertExistsAndLoadBean(t, &DeletedBranch{ID: 2}).(*DeletedBranch)

	branch := getDeletedBranch(t, firstBranch)
	assert.Nil(t, branch.DeletedBy)
	branch.LoadUser()
	assert.NotNil(t, branch.DeletedBy)
	assert.Equal(t, "user1", branch.DeletedBy.Name)

	branch = getDeletedBranch(t, secondBranch)
	assert.Nil(t, branch.DeletedBy)
	branch.LoadUser()
	assert.NotNil(t, branch.DeletedBy)
	assert.Equal(t, "Ghost", branch.DeletedBy.Name)
}

func TestRemoveDeletedBranch(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)

	firstBranch := AssertExistsAndLoadBean(t, &DeletedBranch{ID: 1}).(*DeletedBranch)

	err := repo.RemoveDeletedBranch(1)
	assert.NoError(t, err)
	AssertNotExistsBean(t, firstBranch)
	AssertExistsAndLoadBean(t, &DeletedBranch{ID: 2})
}

func getDeletedBranch(t *testing.T, branch *DeletedBranch) *DeletedBranch {
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)

	deletedBranch, err := repo.GetDeletedBranchByID(branch.ID)
	assert.NoError(t, err)
	assert.Equal(t, branch.ID, deletedBranch.ID)
	assert.Equal(t, branch.Name, deletedBranch.Name)
	assert.Equal(t, branch.Commit, deletedBranch.Commit)
	assert.Equal(t, branch.DeletedByID, deletedBranch.DeletedByID)

	return deletedBranch
}

func TestFindRenamedBranch(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	branch, exist, err := FindRenamedBranch(1, "dev")
	assert.NoError(t, err)
	assert.Equal(t, true, exist)
	assert.Equal(t, "master", branch.To)

	_, exist, err = FindRenamedBranch(1, "unknow")
	assert.NoError(t, err)
	assert.Equal(t, false, exist)
}

func TestRenameBranch(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	repo1 := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	_isDefault := false

	err := UpdateProtectBranch(repo1, &ProtectedBranch{
		RepoID:     repo1.ID,
		BranchName: "master",
	}, WhitelistOptions{})
	assert.NoError(t, err)

	assert.NoError(t, repo1.RenameBranch("master", "main", func(isDefault bool) error {
		_isDefault = isDefault
		return nil
	}))

	assert.Equal(t, true, _isDefault)
	repo1 = AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	assert.Equal(t, "main", repo1.DefaultBranch)

	pull := AssertExistsAndLoadBean(t, &PullRequest{ID: 1}).(*PullRequest) // merged
	assert.Equal(t, "master", pull.BaseBranch)

	pull = AssertExistsAndLoadBean(t, &PullRequest{ID: 2}).(*PullRequest) // open
	assert.Equal(t, "main", pull.BaseBranch)

	renamedBranch := AssertExistsAndLoadBean(t, &RenamedBranch{ID: 2}).(*RenamedBranch)
	assert.Equal(t, "master", renamedBranch.From)
	assert.Equal(t, "main", renamedBranch.To)
	assert.Equal(t, int64(1), renamedBranch.RepoID)

	AssertExistsAndLoadBean(t, &ProtectedBranch{
		RepoID:     repo1.ID,
		BranchName: "main",
	})
}
