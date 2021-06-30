// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// SignMerge determines if we should sign a PR merge commit to the base repository
func (pr *PullRequest) SignMerge(u *User, tmpBasePath, baseCommit, headCommit string) (bool, string, *git.Signature, error) {
	if err := pr.LoadBaseRepo(); err != nil {
		log.Error("Unable to get Base Repo for pull request")
		return false, "", nil, err
	}
	repo := pr.BaseRepo

	signingKey, signer := SigningKey(repo.RepoPath())
	if signingKey == "" {
		return false, "", nil, &ErrWontSign{noKey}
	}
	rules := signingModeFromStrings(setting.Repository.Signing.Merges)

	var gitRepo *git.Repository
	var err error

Loop:
	for _, rule := range rules {
		switch rule {
		case never:
			return false, "", nil, &ErrWontSign{never}
		case always:
			break Loop
		case pubkey:
			keys, err := ListGPGKeys(u.ID, ListOptions{})
			if err != nil {
				return false, "", nil, err
			}
			if len(keys) == 0 {
				return false, "", nil, &ErrWontSign{pubkey}
			}
		case twofa:
			twofaModel, err := GetTwoFactorByUID(u.ID)
			if err != nil && !IsErrTwoFactorNotEnrolled(err) {
				return false, "", nil, err
			}
			if twofaModel == nil {
				return false, "", nil, &ErrWontSign{twofa}
			}
		case approved:
			protectedBranch, err := GetProtectedBranchBy(repo.ID, pr.BaseBranch)
			if err != nil {
				return false, "", nil, err
			}
			if protectedBranch == nil {
				return false, "", nil, &ErrWontSign{approved}
			}
			if protectedBranch.GetGrantedApprovalsCount(pr) < 1 {
				return false, "", nil, &ErrWontSign{approved}
			}
		case baseSigned:
			if gitRepo == nil {
				gitRepo, err = git.OpenRepository(tmpBasePath)
				if err != nil {
					return false, "", nil, err
				}
				defer gitRepo.Close()
			}
			commit, err := gitRepo.GetCommit(baseCommit)
			if err != nil {
				return false, "", nil, err
			}
			verification := ParseCommitWithSignature(commit)
			if !verification.Verified {
				return false, "", nil, &ErrWontSign{baseSigned}
			}
		case headSigned:
			if gitRepo == nil {
				gitRepo, err = git.OpenRepository(tmpBasePath)
				if err != nil {
					return false, "", nil, err
				}
				defer gitRepo.Close()
			}
			commit, err := gitRepo.GetCommit(headCommit)
			if err != nil {
				return false, "", nil, err
			}
			verification := ParseCommitWithSignature(commit)
			if !verification.Verified {
				return false, "", nil, &ErrWontSign{headSigned}
			}
		case commitsSigned:
			if gitRepo == nil {
				gitRepo, err = git.OpenRepository(tmpBasePath)
				if err != nil {
					return false, "", nil, err
				}
				defer gitRepo.Close()
			}
			commit, err := gitRepo.GetCommit(headCommit)
			if err != nil {
				return false, "", nil, err
			}
			verification := ParseCommitWithSignature(commit)
			if !verification.Verified {
				return false, "", nil, &ErrWontSign{commitsSigned}
			}
			// need to work out merge-base
			mergeBaseCommit, _, err := gitRepo.GetMergeBase("", baseCommit, headCommit)
			if err != nil {
				return false, "", nil, err
			}
			commitList, err := commit.CommitsBeforeUntil(mergeBaseCommit)
			if err != nil {
				return false, "", nil, err
			}
			for _, commit := range commitList {
				verification := ParseCommitWithSignature(commit)
				if !verification.Verified {
					return false, "", nil, &ErrWontSign{commitsSigned}
				}
			}
		}
	}
	return true, signingKey, signer, nil
}
