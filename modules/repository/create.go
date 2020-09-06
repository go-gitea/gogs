// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/unknwon/com"
)

// CreateRepository creates a repository for the user/organization.
func CreateRepository(doer, u *models.User, opts models.CreateRepoOptions) (*models.Repository, error) {
	if !doer.IsAdmin && !u.CanCreateRepo() {
		return nil, models.ErrReachLimitOfRepo{
			Limit: u.MaxRepoCreation,
		}
	}

	repo := &models.Repository{
		OwnerID:                         u.ID,
		Owner:                           u,
		OwnerName:                       u.Name,
		Name:                            opts.Name,
		LowerName:                       strings.ToLower(opts.Name),
		Description:                     opts.Description,
		OriginalURL:                     opts.OriginalURL,
		OriginalServiceType:             opts.GitServiceType,
		IsPrivate:                       opts.IsPrivate,
		IsFsckEnabled:                   !opts.IsMirror,
		CloseIssuesViaCommitInAnyBranch: setting.Repository.DefaultCloseIssuesViaCommitsInAnyBranch,
		Status:                          opts.Status,
		IsEmpty:                         !opts.AutoInit,
	}

	overwriteOrAdopt := (!opts.IsMirror && opts.AdoptPreExisting && (doer.IsAdmin || setting.Repository.AllowAdoptionOfUnadoptedRepositories)) ||
		(opts.OverwritePreExisting && (doer.IsAdmin || setting.Repository.AllowOverwriteOfUnadoptedRepositories))

	if err := models.WithTx(func(ctx models.DBContext) error {
		if err := models.CreateRepository(ctx, doer, u, repo, overwriteOrAdopt); err != nil {
			return err
		}

		// No need for init mirror.
		if opts.IsMirror {
			return nil
		}

		shouldInit := true

		repoPath := models.RepoPath(u.Name, repo.Name)
		if com.IsExist(repoPath) {
			// repo already exists - We have two or three options.
			// 1. We fail stating that the directory exists
			// 2. We create the db repository to go with this data and adopt the git repo
			// 3. We delete it and start afresh
			//
			// Previously Gitea would just delete and start afresh - this was naughty.
			if opts.AdoptPreExisting {
				shouldInit = false
				if err := adoptRepository(ctx, repoPath, doer, repo, opts); err != nil {
					return fmt.Errorf("createDelegateHooks: %v", err)
				}
			} else if opts.OverwritePreExisting {
				log.Warn("An already existing repository was deleted at %s", repoPath)
				if err := util.RemoveAll(repoPath); err != nil {
					log.Error("Unable to remove already existing repository at %s: Error: %v", repoPath, err)
					return fmt.Errorf(
						"unable to delete repo directory %s/%s: %v", u.Name, repo.Name, err)
				}
			} else {
				log.Error("Files already exist in %s and we are not going to adopt or delete.", repoPath)
				return models.ErrRepoFilesAlreadyExist{
					Uname: u.Name,
					Name:  repo.Name,
				}
			}
		}

		if shouldInit {
			if err := initRepository(ctx, repoPath, doer, repo, opts); err != nil {
				if err2 := util.RemoveAll(repoPath); err2 != nil {
					log.Error("initRepository: %v", err)
					return fmt.Errorf(
						"delete repo directory %s/%s failed(2): %v", u.Name, repo.Name, err2)
				}
				return fmt.Errorf("initRepository: %v", err)
			}
		}

		// Initialize Issue Labels if selected
		if len(opts.IssueLabels) > 0 {
			if err := models.InitializeLabels(ctx, repo.ID, opts.IssueLabels, false); err != nil {
				if shouldInit {
					if errDelete := models.DeleteRepository(doer, u.ID, repo.ID); errDelete != nil {
						log.Error("Rollback deleteRepository: %v", errDelete)
					}
				}
				return fmt.Errorf("InitializeLabels: %v", err)
			}
		}

		if stdout, err := git.NewCommand("update-server-info").
			SetDescription(fmt.Sprintf("CreateRepository(git update-server-info): %s", repoPath)).
			RunInDir(repoPath); err != nil {
			log.Error("CreateRepository(git update-server-info) in %v: Stdout: %s\nError: %v", repo, stdout, err)
			if shouldInit {
				if errDelete := models.DeleteRepository(doer, u.ID, repo.ID); errDelete != nil {
					log.Error("Rollback deleteRepository: %v", errDelete)
				}
			}
			return fmt.Errorf("CreateRepository(git update-server-info): %v", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return repo, nil
}
