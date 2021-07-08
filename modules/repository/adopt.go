// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"github.com/gobwas/glob"
)

// AdoptRepository adopts a repository for the user/organization.
func AdoptRepository(doer, u *models.User, opts models.CreateRepoOptions) (*models.Repository, error) {
	if !doer.IsAdmin && !u.CanCreateRepo() {
		return nil, models.ErrReachLimitOfRepo{
			Limit: u.MaxRepoCreation,
		}
	}

	if len(opts.DefaultBranch) == 0 {
		opts.DefaultBranch = setting.Repository.DefaultBranch
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

	if err := models.WithTx(func(ctx models.DBContext) error {
		repoPath := models.RepoPath(u.Name, repo.Name)
		isExist, err := util.IsExist(repoPath)
		if err != nil {
			log.Error("Unable to check if %s exists. Error: %v", repoPath, err)
			return err
		}
		if !isExist {
			return models.ErrRepoNotExist{
				OwnerName: u.Name,
				Name:      repo.Name,
			}
		}

		if err := models.CreateRepository(ctx, doer, u, repo, true); err != nil {
			return err
		}
		if err := adoptRepository(ctx, repoPath, doer, repo, opts); err != nil {
			return fmt.Errorf("createDelegateHooks: %v", err)
		}

		// Initialize Issue Labels if selected
		if len(opts.IssueLabels) > 0 {
			if err := models.InitializeLabels(ctx, repo.ID, opts.IssueLabels, false); err != nil {
				return fmt.Errorf("InitializeLabels: %v", err)
			}
		}

		if stdout, err := git.NewCommand("update-server-info").
			SetDescription(fmt.Sprintf("CreateRepository(git update-server-info): %s", repoPath)).
			RunInDir(repoPath); err != nil {
			log.Error("CreateRepository(git update-server-info) in %v: Stdout: %s\nError: %v", repo, stdout, err)
			return fmt.Errorf("CreateRepository(git update-server-info): %v", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return repo, nil
}

// DeleteUnadoptedRepository deletes unadopted repository files from the filesystem
func DeleteUnadoptedRepository(doer, u *models.User, repoName string) error {
	if err := models.IsUsableRepoName(repoName); err != nil {
		return err
	}

	repoPath := models.RepoPath(u.Name, repoName)
	isExist, err := util.IsExist(repoPath)
	if err != nil {
		log.Error("Unable to check if %s exists. Error: %v", repoPath, err)
		return err
	}
	if !isExist {
		return models.ErrRepoNotExist{
			OwnerName: u.Name,
			Name:      repoName,
		}
	}

	if exist, err := models.IsRepositoryExist(u, repoName); err != nil {
		return err
	} else if exist {
		return models.ErrRepoAlreadyExist{
			Uname: u.Name,
			Name:  repoName,
		}
	}

	return util.RemoveAll(repoPath)
}

// ListUnadoptedRepositories lists all the unadopted repositories that match the provided query
func ListUnadoptedRepositories(query string, opts *models.ListOptions) ([]string, int, error) {
	globUser, _ := glob.Compile("*")
	globRepo, _ := glob.Compile("*")

	qsplit := strings.SplitN(query, "/", 2)
	if len(qsplit) > 0 && len(query) > 0 {
		var err error
		globUser, err = glob.Compile(qsplit[0])
		if err != nil {
			log.Info("Invalid glob expression '%s' (skipped): %v", qsplit[0], err)
		}
		if len(qsplit) > 1 {
			globRepo, err = glob.Compile(qsplit[1])
			if err != nil {
				log.Info("Invalid glob expression '%s' (skipped): %v", qsplit[1], err)
			}
		}
	}
	start := (opts.Page - 1) * opts.PageSize
	end := start + opts.PageSize

	repoNamesToCheck := make([]string, 0, opts.PageSize)

	repoNames := make([]string, 0, opts.PageSize)
	var ctxUser *models.User

	count := 0

	// We're going to iterate by pagesize.
	root := filepath.Join(setting.RepoRootPath)
	if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() || path == root {
			return nil
		}

		if !strings.ContainsRune(path[len(root)+1:], filepath.Separator) {
			// Got a new user

			// Clean up old repoNamesToCheck
			if len(repoNamesToCheck) > 0 {
				repos, _, err := models.GetUserRepositories(&models.SearchRepoOptions{Actor: ctxUser, Private: true, ListOptions: models.ListOptions{
					Page:     1,
					PageSize: opts.PageSize,
				}, LowerNames: repoNamesToCheck})
				if err != nil {
					return err
				}
				for _, name := range repoNamesToCheck {
					found := false
				repoLoopCatchup:
					for i, repo := range repos {
						if repo.LowerName == name {
							found = true
							repos = append(repos[:i], repos[i+1:]...)
							break repoLoopCatchup
						}
					}
					if !found {
						if count >= start && count < end {
							repoNames = append(repoNames, fmt.Sprintf("%s/%s", ctxUser.Name, name))
						}
						count++
					}
				}
				repoNamesToCheck = repoNamesToCheck[:0]
			}

			if !globUser.Match(info.Name()) {
				return filepath.SkipDir
			}

			ctxUser, err = models.GetUserByName(info.Name())
			if err != nil {
				if models.IsErrUserNotExist(err) {
					log.Debug("Missing user: %s", info.Name())
					return filepath.SkipDir
				}
				return err
			}
			return nil
		}

		name := info.Name()

		if !strings.HasSuffix(name, ".git") {
			return filepath.SkipDir
		}
		name = name[:len(name)-4]
		if models.IsUsableRepoName(name) != nil || strings.ToLower(name) != name || !globRepo.Match(name) {
			return filepath.SkipDir
		}
		if count < end {
			repoNamesToCheck = append(repoNamesToCheck, name)
			if len(repoNamesToCheck) >= opts.PageSize {
				repos, _, err := models.GetUserRepositories(&models.SearchRepoOptions{Actor: ctxUser, Private: true, ListOptions: models.ListOptions{
					Page:     1,
					PageSize: opts.PageSize,
				}, LowerNames: repoNamesToCheck})
				if err != nil {
					return err
				}
				for _, name := range repoNamesToCheck {
					found := false
				repoLoop:
					for i, repo := range repos {
						if repo.LowerName == name {
							found = true
							repos = append(repos[:i], repos[i+1:]...)
							break repoLoop
						}
					}
					if !found {
						if count >= start && count < end {
							repoNames = append(repoNames, fmt.Sprintf("%s/%s", ctxUser.Name, name))
						}
						count++
					}
				}
				repoNamesToCheck = repoNamesToCheck[:0]
			}
			return filepath.SkipDir
		}
		count++
		return filepath.SkipDir
	}); err != nil {
		return nil, 0, err
	}

	if len(repoNamesToCheck) > 0 {
		repos, _, err := models.GetUserRepositories(&models.SearchRepoOptions{Actor: ctxUser, Private: true, ListOptions: models.ListOptions{
			Page:     1,
			PageSize: opts.PageSize,
		}, LowerNames: repoNamesToCheck})
		if err != nil {
			return nil, 0, err
		}
		for _, name := range repoNamesToCheck {
			found := false
		repoLoop:
			for i, repo := range repos {
				if repo.LowerName == name {
					found = true
					repos = append(repos[:i], repos[i+1:]...)
					break repoLoop
				}
			}
			if !found {
				if count >= start && count < end {
					repoNames = append(repoNames, fmt.Sprintf("%s/%s", ctxUser.Name, name))
				}
				count++
			}
		}
	}
	return repoNames, count, nil
}
