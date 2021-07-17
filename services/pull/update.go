// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pull

import (
	"fmt"
	"os"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
)

// Update updates pull request with base branch.
func Update(pull *models.PullRequest, doer *models.User, message string, rebase bool) error {
	//use merge functions but switch repo's and branch's
	pr := &models.PullRequest{
		HeadRepoID: pull.BaseRepoID,
		BaseRepoID: pull.HeadRepoID,
		HeadBranch: pull.BaseBranch,
		BaseBranch: pull.HeadBranch,
	}

	if err := pr.LoadHeadRepo(); err != nil {
		log.Error("LoadHeadRepo: %v", err)
		return fmt.Errorf("LoadHeadRepo: %v", err)
	} else if err = pr.LoadBaseRepo(); err != nil {
		log.Error("LoadBaseRepo: %v", err)
		return fmt.Errorf("LoadBaseRepo: %v", err)
	}

	diffCount, err := GetDiverging(pull)
	if err != nil {
		return err
	} else if diffCount.Behind == 0 {
		return fmt.Errorf("HeadBranch of PR %d is up to date", pull.Index)
	}

	if rebase {
		err = doRebase(pr, doer)
	} else {
		_, err = rawMerge(pr, doer, models.MergeStyleMerge, message)
	}

	defer func() {
		go AddTestPullRequestTask(doer, pr.HeadRepo.ID, pr.HeadBranch, false, "", "")
	}()

	return err
}

// IsUserAllowedToUpdate check if user is allowed to update PR with given permissions and branch protections
func IsUserAllowedToUpdate(pull *models.PullRequest, user *models.User, rebase bool) (bool, error) {
	if user == nil {
		return false, nil
	}
	headRepoPerm, err := models.GetUserRepoPermission(pull.HeadRepo, user)
	if err != nil {
		return false, err
	}

	pr := &models.PullRequest{
		HeadRepoID: pull.BaseRepoID,
		BaseRepoID: pull.HeadRepoID,
		HeadBranch: pull.BaseBranch,
		BaseBranch: pull.HeadBranch,
	}

	err = pr.LoadProtectedBranch()
	if err != nil {
		return false, err
	}

	// can't do rebase on protected branch because need force push
	if rebase && pr.ProtectedBranch != nil {
		return false, err
	}

	// Update function need push permission
	if pr.ProtectedBranch != nil && !pr.ProtectedBranch.CanUserPush(user.ID) {
		return false, nil
	}

	return IsUserAllowedToMerge(pr, headRepoPerm, user)
}

// GetDiverging determines how many commits a PR is ahead or behind the PR base branch
func GetDiverging(pr *models.PullRequest) (*git.DivergeObject, error) {
	log.Trace("GetDiverging[%d]: compare commits", pr.ID)
	if err := pr.LoadBaseRepo(); err != nil {
		return nil, err
	}
	if err := pr.LoadHeadRepo(); err != nil {
		return nil, err
	}

	tmpRepo, err := createTemporaryRepo(pr)
	if err != nil {
		if !models.IsErrBranchDoesNotExist(err) {
			log.Error("CreateTemporaryRepo: %v", err)
		}
		return nil, err
	}
	defer func() {
		if err := models.RemoveTemporaryPath(tmpRepo); err != nil {
			log.Error("Merge: RemoveTemporaryPath: %s", err)
		}
	}()

	diff, err := git.GetDivergingCommits(tmpRepo, "base", "tracking")
	return &diff, err
}

func doRebase(pr *models.PullRequest, doer *models.User) error {
	// 1. Clone base repo.
	tmpBasePath, err := createTemporaryRepo(pr)
	if err != nil {
		log.Error("CreateTemporaryPath: %v", err)
		return err
	}
	defer func() {
		if err := models.RemoveTemporaryPath(tmpBasePath); err != nil {
			log.Error("Update-By-Rebase: RemoveTemporaryPath: %s", err)
		}
	}()

	baseBranch := "base"
	trackingBranch := "tracking"

	// 2. do rebase to ttacking branch
	sig := doer.NewGitSig()
	committer := sig

	// Determine if we should sign
	signArg := ""
	if git.CheckGitVersionAtLeast("1.7.9") == nil {
		sign, keyID, signer, _ := pr.SignMerge(doer, tmpBasePath, "HEAD", trackingBranch)
		if sign {
			signArg = "-S" + keyID
			if pr.BaseRepo.GetTrustModel() == models.CommitterTrustModel || pr.BaseRepo.GetTrustModel() == models.CollaboratorCommitterTrustModel {
				committer = signer
			}
		} else if git.CheckGitVersionAtLeast("2.0.0") == nil {
			signArg = "--no-gpg-sign"
		}
	}

	commitTimeStr := time.Now().Format(time.RFC3339)

	// Because this may call hooks we should pass in the environment
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME="+sig.Name,
		"GIT_AUTHOR_EMAIL="+sig.Email,
		"GIT_AUTHOR_DATE="+commitTimeStr,
		"GIT_COMMITTER_NAME="+committer.Name,
		"GIT_COMMITTER_EMAIL="+committer.Email,
		"GIT_COMMITTER_DATE="+commitTimeStr,
	)

	var outbuf, errbuf strings.Builder
	err = git.NewCommand("rebase", trackingBranch, signArg).RunInDirTimeoutEnvFullPipeline(env, -1, tmpBasePath, &outbuf, &errbuf, nil)
	if err != nil {
		log.Error("git rebase [%s:%s -> %s:%s]: %v\n%s\n%s", pr.BaseRepo.FullName(), pr.BaseBranch, pr.HeadRepo.FullName(), pr.HeadBranch, err, outbuf.String(), errbuf.String())
		return fmt.Errorf("git rebase [%s:%s -> %s:%s]: %v\n%s\n%s", pr.BaseRepo.FullName(), pr.BaseBranch, pr.HeadRepo.FullName(), pr.HeadBranch, err, outbuf.String(), errbuf.String())
	}

	// 3. force push to base branch
	env = models.FullPushingEnvironment(doer, doer, pr.BaseRepo, pr.BaseRepo.Name, pr.ID)

	outbuf.Reset()
	errbuf.Reset()
	if err := git.NewCommand("push", "-f", "origin", baseBranch+":refs/heads/"+pr.BaseBranch).RunInDirTimeoutEnvPipeline(env, -1, tmpBasePath, &outbuf, &errbuf); err != nil {
		if strings.Contains(errbuf.String(), "! [remote rejected]") {
			err := &git.ErrPushRejected{
				StdOut: outbuf.String(),
				StdErr: errbuf.String(),
				Err:    err,
			}
			err.GenerateMessage()
			return err
		}
		return fmt.Errorf("git force push: %s", errbuf.String())
	}

	return nil
}
