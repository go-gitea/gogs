// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repofiles

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	repo_module "code.gitea.io/gitea/modules/repository"
	api "code.gitea.io/gitea/modules/structs"
)

// DeleteRepoFileOptions holds the repository delete file options
type DeleteRepoFileOptions struct {
	LastCommitID string
	OldBranch    string
	NewBranch    string
	TreePath     string
	Message      string
	SHA          string
	Author       *IdentityOptions
	Committer    *IdentityOptions
	Dates        *CommitDateOptions
	Signoff      bool
}

// DeleteRepoFile deletes a file in the given repository
func DeleteRepoFile(repo *models.Repository, doer *models.User, opts *DeleteRepoFileOptions) (*api.FileResponse, error) {
	// If no branch name is set, assume the repo's default branch
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

	// A NewBranch can be specified for the file to be created/updated in a new branch.
	// Check to make sure the branch does not already exist, otherwise we can't proceed.
	// If we aren't branching to a new branch, make sure user can commit to the given branch
	if opts.NewBranch != opts.OldBranch {
		newBranch, err := repo_module.GetBranch(repo, opts.NewBranch)
		if err != nil && !git.IsErrBranchNotExist(err) {
			return nil, err
		}
		if newBranch != nil {
			return nil, models.ErrBranchAlreadyExists{
				BranchName: opts.NewBranch,
			}
		}
	} else if err := VerifyBranchProtection(repo, doer, opts.OldBranch, opts.TreePath); err != nil {
		return nil, err
	}

	// Check that the path given in opts.treeName is valid (not a git path)
	treePath := CleanUploadFileName(opts.TreePath)
	if treePath == "" {
		return nil, models.ErrFilenameInvalid{
			Path: opts.TreePath,
		}
	}

	message := strings.TrimSpace(opts.Message)

	author, committer := GetAuthorAndCommitterUsers(opts.Author, opts.Committer, doer)

	t, err := NewTemporaryUploadRepository(repo)
	if err != nil {
		return nil, err
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
			return nil, fmt.Errorf("DeleteRepoFile: Invalid last commit ID: %v", err)
		}
		opts.LastCommitID = lastCommitID.String()
	}

	// Get the files in the index
	filesInIndex, err := t.LsFiles(opts.TreePath)
	if err != nil {
		return nil, fmt.Errorf("DeleteRepoFile: %v", err)
	}

	// Find the file we want to delete in the index
	inFilelist := false
	for _, file := range filesInIndex {
		if file == opts.TreePath {
			inFilelist = true
			break
		}
	}
	if !inFilelist {
		return nil, models.ErrRepoFileDoesNotExist{
			Path: opts.TreePath,
		}
	}

	// Get the entry of treePath and check if the SHA given is the same as the file
	entry, err := commit.GetTreeEntryByPath(treePath)
	if err != nil {
		return nil, err
	}
	if opts.SHA != "" {
		// If a SHA was given and the SHA given doesn't match the SHA of the fromTreePath, throw error
		if opts.SHA != entry.ID.String() {
			return nil, models.ErrSHADoesNotMatch{
				Path:       treePath,
				GivenSHA:   opts.SHA,
				CurrentSHA: entry.ID.String(),
			}
		}
	} else if opts.LastCommitID != "" {
		// If a lastCommitID was given and it doesn't match the commitID of the head of the branch throw
		// an error, but only if we aren't creating a new branch.
		if commit.ID.String() != opts.LastCommitID && opts.OldBranch == opts.NewBranch {
			// CommitIDs don't match, but we don't want to throw a ErrCommitIDDoesNotMatch unless
			// this specific file has been edited since opts.LastCommitID
			if changed, err := commit.FileChangedSinceCommit(treePath, opts.LastCommitID); err != nil {
				return nil, err
			} else if changed {
				return nil, models.ErrCommitIDDoesNotMatch{
					GivenCommitID:   opts.LastCommitID,
					CurrentCommitID: opts.LastCommitID,
				}
			}
			// The file wasn't modified, so we are good to delete it
		}
	} else {
		// When deleting a file, a lastCommitID or SHA needs to be given to make sure other commits haven't been
		// made. We throw an error if one wasn't provided.
		return nil, models.ErrSHAOrCommitIDNotProvided{}
	}

	// Remove the file from the index
	if err := t.RemoveFilesFromIndex(opts.TreePath); err != nil {
		return nil, err
	}

	// Now write the tree
	treeHash, err := t.WriteTree()
	if err != nil {
		return nil, err
	}

	// Now commit the tree
	var commitHash string
	if opts.Dates != nil {
		commitHash, err = t.CommitTreeWithDate(author, committer, treeHash, message, opts.Signoff, opts.Dates.Author, opts.Dates.Committer)
	} else {
		commitHash, err = t.CommitTree(author, committer, treeHash, message, opts.Signoff)
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

	file, err := GetFileResponseFromCommit(repo, commit, opts.NewBranch, treePath)
	if err != nil {
		return nil, err
	}
	return file, nil
}
