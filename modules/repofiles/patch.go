// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repofiles

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/structs"
	api "code.gitea.io/gitea/modules/structs"
)

// ApplyDiffPatchOptions holds the repository diff patch update options
type ApplyDiffPatchOptions struct {
	LastCommitID string
	OldBranch    string
	NewBranch    string
	Message      string
	Content      string
	SHA          string
	Author       *IdentityOptions
	Committer    *IdentityOptions
	Dates        *CommitDateOptions
}

// ApplyDiffPatch applies a patch to the given repository
func ApplyDiffPatch(repo *models.Repository, doer *models.User, opts *ApplyDiffPatchOptions) (*structs.FileResponse, error) {
	// If no branch name is set, assume master
	if opts.OldBranch == "" {
		opts.OldBranch = repo.DefaultBranch
	}
	if opts.NewBranch == "" {
		opts.NewBranch = opts.OldBranch
	}

	// oldBranch must exist for this operation
	if _, err := repo_module.GetBranch(repo, opts.OldBranch); err != nil {
		return nil, err
	}

	// A NewBranch can be specified for the patch to be applied to.
	// Check to make sure the branch does not already exist, otherwise we can't proceed.
	// If we aren't branching to a new branch, make sure user can commit to the given branch
	if opts.NewBranch != opts.OldBranch {
		existingBranch, err := repo_module.GetBranch(repo, opts.NewBranch)
		if existingBranch != nil {
			return nil, models.ErrBranchAlreadyExists{
				BranchName: opts.NewBranch,
			}
		}
		if err != nil && !git.IsErrBranchNotExist(err) {
			return nil, err
		}
	} else {
		protectedBranch, err := repo.GetBranchProtection(opts.OldBranch)
		if err != nil {
			return nil, err
		}
		if protectedBranch != nil && !protectedBranch.CanUserPush(doer.ID) {
			return nil, models.ErrUserCannotCommit{
				UserName: doer.LowerName,
			}
		}
		if protectedBranch != nil && protectedBranch.RequireSignedCommits {
			_, _, err := repo.SignCRUDAction(doer, repo.RepoPath(), opts.OldBranch)
			if err != nil {
				if !models.IsErrWontSign(err) {
					return nil, err
				}
				return nil, models.ErrUserCannotCommit{
					UserName: doer.LowerName,
				}
			}
		}
	}

	message := strings.TrimSpace(opts.Message)

	author, committer := GetAuthorAndCommitterUsers(opts.Author, opts.Committer, doer)

	t, err := NewTemporaryUploadRepository(repo)
	if err != nil {
		log.Error("%v", err)
	}
	defer t.Close()
	if err := t.Clone(opts.OldBranch); err != nil {
		return nil, err
	}
	if err := t.SetDefaultIndex(); err != nil {
		return nil, err
	}

	// Get the commit of the original branch
	commit, err := t.GetBranchCommit(opts.OldBranch)
	if err != nil {
		return nil, err // Couldn't get a commit for the branch
	}

	// Assigned LastCommitID in opts if it hasn't been set
	if opts.LastCommitID == "" {
		opts.LastCommitID = commit.ID.String()
	} else {
		lastCommitID, err := t.gitRepo.ConvertToSHA1(opts.LastCommitID)
		if err != nil {
			return nil, fmt.Errorf("ApplyPatch: Invalid last commit ID: %v", err)
		}
		opts.LastCommitID = lastCommitID.String()
	}

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	err = git.NewCommand("apply", "--index", "--cached", "--ignore-whitespace", "--whitespace=fix").RunInDirFullPipeline(t.basePath, stdout, stderr, strings.NewReader(opts.Content))
	if err != nil {
		return nil, fmt.Errorf("Error: Stdout: %s\nStderr: %s\nErr: %v", stdout.String(), stderr.String(), err)
	}

	// Now write the tree
	treeHash, err := t.WriteTree()
	if err != nil {
		return nil, err
	}

	// Now commit the tree
	var commitHash string
	if opts.Dates != nil {
		commitHash, err = t.CommitTreeWithDate(author, committer, treeHash, message, opts.Dates.Author, opts.Dates.Committer)
	} else {
		commitHash, err = t.CommitTree(author, committer, treeHash, message)
	}
	if err != nil {
		return nil, err
	}

	// Then push this tree to NewBranch
	if err := t.Push(doer, commitHash, opts.NewBranch); err != nil {
		return nil, err
	}

	commit, err = t.GetCommit(commitHash)
	if err != nil {
		return nil, err
	}

	fileCommitResponse, _ := GetFileCommitResponse(repo, commit) // ok if fails, then will be nil
	verification := GetPayloadCommitVerification(commit)
	fileResponse := &api.FileResponse{
		Commit:       fileCommitResponse,
		Verification: verification,
	}

	return fileResponse, nil
}
