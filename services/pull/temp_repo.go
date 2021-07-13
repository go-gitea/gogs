// Copyright 2019 The Gitea Authors.
// All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pull

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
)

// createTemporaryRepo creates a temporary repo with "base" for pr.BaseBranch and "tracking" for  pr.HeadBranch
// it also create a second base branch called "original_base"
func createTemporaryRepo(pr *models.PullRequest) (string, error) {
	if err := pr.LoadHeadRepo(); err != nil {
		log.Error("LoadHeadRepo: %v", err)
		return "", fmt.Errorf("LoadHeadRepo: %v", err)
	} else if pr.HeadRepo == nil {
		log.Error("Pr %d HeadRepo %d does not exist", pr.ID, pr.HeadRepoID)
		return "", &models.ErrRepoNotExist{
			ID: pr.HeadRepoID,
		}
	} else if err := pr.LoadBaseRepo(); err != nil {
		log.Error("LoadBaseRepo: %v", err)
		return "", fmt.Errorf("LoadBaseRepo: %v", err)
	} else if pr.BaseRepo == nil {
		log.Error("Pr %d BaseRepo %d does not exist", pr.ID, pr.BaseRepoID)
		return "", &models.ErrRepoNotExist{
			ID: pr.BaseRepoID,
		}
	} else if err := pr.HeadRepo.GetOwner(); err != nil {
		log.Error("HeadRepo.GetOwner: %v", err)
		return "", fmt.Errorf("HeadRepo.GetOwner: %v", err)
	} else if err := pr.BaseRepo.GetOwner(); err != nil {
		log.Error("BaseRepo.GetOwner: %v", err)
		return "", fmt.Errorf("BaseRepo.GetOwner: %v", err)
	}

	// Clone base repo.
	tmpBasePath, err := models.CreateTemporaryPath("pull")
	if err != nil {
		log.Error("CreateTemporaryPath: %v", err)
		return "", err
	}

	baseRepoPath := pr.BaseRepo.RepoPath()
	headRepoPath := pr.HeadRepo.RepoPath()

	if err := git.InitRepository(tmpBasePath, false); err != nil {
		log.Error("git init tmpBasePath: %v", err)
		if err := models.RemoveTemporaryPath(tmpBasePath); err != nil {
			log.Error("CreateTempRepo: RemoveTemporaryPath: %s", err)
		}
		return "", err
	}

	remoteRepoName := "head_repo"
	baseBranch := "base"

	// Add head repo remote.
	addCacheRepo := func(staging, cache string) error {
		p := filepath.Join(staging, ".git", "objects", "info", "alternates")
		f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			log.Error("Could not create .git/objects/info/alternates file in %s: %v", staging, err)
			return err
		}
		defer f.Close()
		data := filepath.Join(cache, "objects")
		if _, err := fmt.Fprintln(f, data); err != nil {
			log.Error("Could not write to .git/objects/info/alternates file in %s: %v", staging, err)
			return err
		}
		return nil
	}

	if err := addCacheRepo(tmpBasePath, baseRepoPath); err != nil {
		log.Error("Unable to add base repository to temporary repo [%s -> %s]: %v", pr.BaseRepo.FullName(), tmpBasePath, err)
		if err := models.RemoveTemporaryPath(tmpBasePath); err != nil {
			log.Error("CreateTempRepo: RemoveTemporaryPath: %s", err)
		}
		return "", fmt.Errorf("Unable to add base repository to temporary repo [%s -> tmpBasePath]: %v", pr.BaseRepo.FullName(), err)
	}

	var outbuf, errbuf strings.Builder
	if err := git.NewCommand("remote", "add", "-t", pr.BaseBranch, "-m", pr.BaseBranch, "origin", baseRepoPath).RunInDirPipeline(tmpBasePath, &outbuf, &errbuf); err != nil {
		log.Error("Unable to add base repository as origin [%s -> %s]: %v\n%s\n%s", pr.BaseRepo.FullName(), tmpBasePath, err, outbuf.String(), errbuf.String())
		if err := models.RemoveTemporaryPath(tmpBasePath); err != nil {
			log.Error("CreateTempRepo: RemoveTemporaryPath: %s", err)
		}
		return "", fmt.Errorf("Unable to add base repository as origin [%s -> tmpBasePath]: %v\n%s\n%s", pr.BaseRepo.FullName(), err, outbuf.String(), errbuf.String())
	}
	outbuf.Reset()
	errbuf.Reset()

	if err := git.NewCommand("fetch", "origin", "--no-tags", "--", pr.BaseBranch+":"+baseBranch, pr.BaseBranch+":original_"+baseBranch).RunInDirPipeline(tmpBasePath, &outbuf, &errbuf); err != nil {
		log.Error("Unable to fetch origin base branch [%s:%s -> base, original_base in %s]: %v:\n%s\n%s", pr.BaseRepo.FullName(), pr.BaseBranch, tmpBasePath, err, outbuf.String(), errbuf.String())
		if err := models.RemoveTemporaryPath(tmpBasePath); err != nil {
			log.Error("CreateTempRepo: RemoveTemporaryPath: %s", err)
		}
		return "", fmt.Errorf("Unable to fetch origin base branch [%s:%s -> base, original_base in tmpBasePath]: %v\n%s\n%s", pr.BaseRepo.FullName(), pr.BaseBranch, err, outbuf.String(), errbuf.String())
	}
	outbuf.Reset()
	errbuf.Reset()

	if err := git.NewCommand("symbolic-ref", "HEAD", git.BranchPrefix+baseBranch).RunInDirPipeline(tmpBasePath, &outbuf, &errbuf); err != nil {
		log.Error("Unable to set HEAD as base branch [%s]: %v\n%s\n%s", tmpBasePath, err, outbuf.String(), errbuf.String())
		if err := models.RemoveTemporaryPath(tmpBasePath); err != nil {
			log.Error("CreateTempRepo: RemoveTemporaryPath: %s", err)
		}
		return "", fmt.Errorf("Unable to set HEAD as base branch [tmpBasePath]: %v\n%s\n%s", err, outbuf.String(), errbuf.String())
	}
	outbuf.Reset()
	errbuf.Reset()

	if err := addCacheRepo(tmpBasePath, headRepoPath); err != nil {
		log.Error("Unable to add head repository to temporary repo [%s -> %s]: %v", pr.HeadRepo.FullName(), tmpBasePath, err)
		if err := models.RemoveTemporaryPath(tmpBasePath); err != nil {
			log.Error("CreateTempRepo: RemoveTemporaryPath: %s", err)
		}
		return "", fmt.Errorf("Unable to head base repository to temporary repo [%s -> tmpBasePath]: %v", pr.HeadRepo.FullName(), err)
	}

	if err := git.NewCommand("remote", "add", remoteRepoName, headRepoPath).RunInDirPipeline(tmpBasePath, &outbuf, &errbuf); err != nil {
		log.Error("Unable to add head repository as head_repo [%s -> %s]: %v\n%s\n%s", pr.HeadRepo.FullName(), tmpBasePath, err, outbuf.String(), errbuf.String())
		if err := models.RemoveTemporaryPath(tmpBasePath); err != nil {
			log.Error("CreateTempRepo: RemoveTemporaryPath: %s", err)
		}
		return "", fmt.Errorf("Unable to add head repository as head_repo [%s -> tmpBasePath]: %v\n%s\n%s", pr.HeadRepo.FullName(), err, outbuf.String(), errbuf.String())
	}
	outbuf.Reset()
	errbuf.Reset()

	trackingBranch := "tracking"
	// Fetch head branch
	if err := git.NewCommand("fetch", "--no-tags", remoteRepoName, git.BranchPrefix+pr.HeadBranch+":"+trackingBranch).RunInDirPipeline(tmpBasePath, &outbuf, &errbuf); err != nil {
		if err := models.RemoveTemporaryPath(tmpBasePath); err != nil {
			log.Error("CreateTempRepo: RemoveTemporaryPath: %s", err)
		}
		if !git.IsBranchExist(pr.HeadRepo.RepoPath(), pr.HeadBranch) {
			return "", models.ErrBranchDoesNotExist{
				BranchName: pr.HeadBranch,
			}
		}
		log.Error("Unable to fetch head_repo head branch [%s:%s -> tracking in %s]: %v:\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, tmpBasePath, err, outbuf.String(), errbuf.String())
		return "", fmt.Errorf("Unable to fetch head_repo head branch [%s:%s -> tracking in tmpBasePath]: %v\n%s\n%s", pr.HeadRepo.FullName(), pr.HeadBranch, err, outbuf.String(), errbuf.String())
	}
	outbuf.Reset()
	errbuf.Reset()

	return tmpBasePath, nil
}
