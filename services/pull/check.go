// Copyright 2019 The Gitea Authors.
// All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pull

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/services"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
)

// prQueue represents a queue to handle update pull request tests
var prQueue queue.UniqueQueue

// AddToTaskQueue adds itself to pull request test task queue.
func AddToTaskQueue(pr *models.PullRequest) {
	go func() {
		err := prQueue.PushFunc(strconv.FormatInt(pr.ID, 10), func() error {
			pr.Status = models.PullRequestStatusChecking
			err := pr.UpdateColsIfNotMerged("status")
			if err != nil {
				log.Error("AddToTaskQueue.UpdateCols[%d].(add to queue): %v", pr.ID, err)
			} else {
				log.Trace("Adding PR ID: %d to the test pull requests queue", pr.ID)
			}
			return err
		})
		if err != nil && err != queue.ErrAlreadyInQueue {
			log.Error("Error adding prID %d to the test pull requests queue: %v", pr.ID, err)
		}
	}()
}

// checkAndUpdateStatus checks if pull request is possible to leaving checking status,
// and set to be either conflict or mergeable.
func checkAndUpdateStatus(pr *models.PullRequest) {
	// Status is not changed to conflict means mergeable.
	if pr.Status == models.PullRequestStatusChecking {
		pr.Status = models.PullRequestStatusMergeable
	}

	// Make sure there is no waiting test to process before leaving the checking status.
	has, err := prQueue.Has(strconv.FormatInt(pr.ID, 10))
	if err != nil {
		log.Error("Unable to check if the queue is waiting to reprocess pr.ID %d. Error: %v", pr.ID, err)
	}

	if !has {
		if err := pr.UpdateColsIfNotMerged("merge_base", "status", "conflicted_files", "changed_protected_files"); err != nil {
			log.Error("Update[%d]: %v", pr.ID, err)
		}
	}
}

// getMergeCommit checks if a pull request got merged
// Returns the git.Commit of the pull request if merged
func getMergeCommit(pr *models.PullRequest) (*git.Commit, error) {
	if pr.BaseRepo == nil {
		var err error
		pr.BaseRepo, err = models.GetRepositoryByID(pr.BaseRepoID)
		if err != nil {
			return nil, fmt.Errorf("GetRepositoryByID: %v", err)
		}
	}

	indexTmpPath, err := ioutil.TempDir(os.TempDir(), "gitea-"+pr.BaseRepo.Name)
	if err != nil {
		return nil, fmt.Errorf("Failed to create temp dir for repository %s: %v", pr.BaseRepo.RepoPath(), err)
	}
	defer func() {
		if err := util.RemoveAll(indexTmpPath); err != nil {
			log.Warn("Unable to remove temporary index path: %s: Error: %v", indexTmpPath, err)
		}
	}()

	headFile := pr.GetGitRefName()

	// Check if a pull request is merged into BaseBranch
	_, err = git.NewCommand("merge-base", "--is-ancestor", headFile, pr.BaseBranch).
		RunInDirWithEnv(pr.BaseRepo.RepoPath(), []string{"GIT_INDEX_FILE=" + indexTmpPath, "GIT_DIR=" + pr.BaseRepo.RepoPath()})
	if err != nil {
		// Errors are signaled by a non-zero status that is not 1
		if strings.Contains(err.Error(), "exit status 1") {
			return nil, nil
		}
		return nil, fmt.Errorf("git merge-base --is-ancestor: %v", err)
	}

	commitIDBytes, err := ioutil.ReadFile(pr.BaseRepo.RepoPath() + "/" + headFile)
	if err != nil {
		return nil, fmt.Errorf("ReadFile(%s): %v", headFile, err)
	}
	commitID := string(commitIDBytes)
	if len(commitID) < 40 {
		return nil, fmt.Errorf(`ReadFile(%s): invalid commit-ID "%s"`, headFile, commitID)
	}
	cmd := commitID[:40] + ".." + pr.BaseBranch

	// Get the commit from BaseBranch where the pull request got merged
	mergeCommit, err := git.NewCommand("rev-list", "--ancestry-path", "--merges", "--reverse", cmd).
		RunInDirWithEnv("", []string{"GIT_INDEX_FILE=" + indexTmpPath, "GIT_DIR=" + pr.BaseRepo.RepoPath()})
	if err != nil {
		return nil, fmt.Errorf("git rev-list --ancestry-path --merges --reverse: %v", err)
	} else if len(mergeCommit) < 40 {
		// PR was maybe fast-forwarded, so just use last commit of PR
		mergeCommit = commitID[:40]
	}

	gitRepo, err := git.OpenRepository(pr.BaseRepo.RepoPath())
	if err != nil {
		return nil, fmt.Errorf("OpenRepository: %v", err)
	}
	defer gitRepo.Close()

	commit, err := gitRepo.GetCommit(mergeCommit[:40])
	if err != nil {
		return nil, fmt.Errorf("GetCommit: %v", err)
	}

	return commit, nil
}

// manuallyMerged checks if a pull request got manually merged
// When a pull request got manually merged mark the pull request as merged
func manuallyMerged(pr *models.PullRequest) bool {
	if err := pr.LoadBaseRepo(); err != nil {
		log.Error("PullRequest[%d].LoadBaseRepo: %v", pr.ID, err)
		return false
	}

	if unit, err := pr.BaseRepo.GetUnit(models.UnitTypePullRequests); err == nil {
		config := unit.PullRequestsConfig()
		if !config.AutodetectManualMerge {
			return false
		}
	} else {
		log.Error("PullRequest[%d].BaseRepo.GetUnit(models.UnitTypePullRequests): %v", pr.ID, err)
		return false
	}

	commit, err := getMergeCommit(pr)
	if err != nil {
		log.Error("PullRequest[%d].getMergeCommit: %v", pr.ID, err)
		return false
	}
	if commit != nil {
		pr.MergedCommitID = commit.ID.String()
		pr.MergedUnix = timeutil.TimeStamp(commit.Author.When.Unix())
		pr.Status = models.PullRequestStatusManuallyMerged
		merger, _ := models.GetUserByEmail(commit.Author.Email)

		// When the commit author is unknown set the BaseRepo owner as merger
		if merger == nil {
			if pr.BaseRepo.Owner == nil {
				if err = pr.BaseRepo.GetOwner(); err != nil {
					log.Error("BaseRepo.GetOwner[%d]: %v", pr.ID, err)
					return false
				}
			}
			merger = pr.BaseRepo.Owner
		}
		pr.Merger = merger
		pr.MergerID = merger.ID

		if merged, err := pr.SetMerged(); err != nil {
			log.Error("PullRequest[%d].setMerged : %v", pr.ID, err)
			return false
		} else if !merged {
			return false
		}

		notification.NotifyMergePullRequest(pr, merger)

		log.Info("manuallyMerged[%d]: Marked as manually merged into %s/%s by commit id: %s", pr.ID, pr.BaseRepo.Name, pr.BaseBranch, commit.ID.String())
		return true
	}
	return false
}

// InitializePullRequests checks and tests untested patches of pull requests.
func InitializePullRequests(ctx context.Context) {
	prs, err := models.GetPullRequestIDsByCheckStatus(models.PullRequestStatusChecking)
	if err != nil {
		log.Error("Find Checking PRs: %v", err)
		return
	}
	for _, prID := range prs {
		select {
		case <-ctx.Done():
			return
		default:
			if err := prQueue.PushFunc(strconv.FormatInt(prID, 10), func() error {
				log.Trace("Adding PR ID: %d to the pull requests patch checking queue", prID)
				return nil
			}); err != nil {
				log.Error("Error adding prID: %s to the pull requests patch checking queue %v", prID, err)
			}
		}
	}
}

// handle passed PR IDs and test the PRs
func handle(data ...queue.Data) {
	for _, datum := range data {
		id, _ := strconv.ParseInt(datum.(string), 10, 64)

		log.Trace("Testing PR ID %d from the pull requests patch checking queue", id)

		pr, err := models.GetPullRequestByID(id)
		if err != nil {
			log.Error("GetPullRequestByID[%s]: %v", datum, err)
			continue
		} else if pr.HasMerged {
			continue
		} else if manuallyMerged(pr) {
			continue
		} else if err = TestPatch(pr); err != nil {
			log.Error("testPatch[%d]: %v", pr.ID, err)
			pr.Status = models.PullRequestStatusError
			if err := pr.UpdateCols("status"); err != nil {
				log.Error("update pr [%d] status to PullRequestStatusError failed: %v", pr.ID, err)
			}
			continue
		}
		checkAndUpdateStatus(pr)
	}
}

// CheckPrsForBaseBranch check all pulls with bseBrannch
func CheckPrsForBaseBranch(baseRepo *models.Repository, baseBranchName string) error {
	prs, err := models.GetUnmergedPullRequestsByBaseInfo(baseRepo.ID, baseBranchName)
	if err != nil {
		return err
	}

	for _, pr := range prs {
		AddToTaskQueue(pr)
	}

	return nil
}

// Init runs the task queue to test all the checking status pull requests
func Init() error {
	prQueue = queue.CreateUniqueQueue("pr_patch_checker", handle, "").(queue.UniqueQueue)

	if prQueue == nil {
		return fmt.Errorf("Unable to create pr_patch_checker Queue")
	}

	go graceful.GetManager().RunWithShutdownFns(prQueue.Run)
	go graceful.GetManager().RunWithShutdownContext(InitializePullRequests)
	return nil
}

func init() {
	services.RegisterService("pull", Init, "setting")
}
