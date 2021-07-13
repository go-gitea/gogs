// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Package private includes all internal routes. The package name internal is ideal but Golang is not allowed, so we use private as package name instead.
package private

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"code.gitea.io/gitea/models"
	gitea_context "code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/private"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	pull_service "code.gitea.io/gitea/services/pull"
	repo_service "code.gitea.io/gitea/services/repository"
)

func verifyCommits(oldCommitID, newCommitID string, repo *git.Repository, env []string) error {
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		log.Error("Unable to create os.Pipe for %s", repo.Path)
		return err
	}
	defer func() {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
	}()

	// This is safe as force pushes are already forbidden
	err = git.NewCommand("rev-list", oldCommitID+"..."+newCommitID).
		RunInDirTimeoutEnvFullPipelineFunc(env, -1, repo.Path,
			stdoutWriter, nil, nil,
			func(ctx context.Context, cancel context.CancelFunc) error {
				_ = stdoutWriter.Close()
				err := readAndVerifyCommitsFromShaReader(stdoutReader, repo, env)
				if err != nil {
					log.Error("%v", err)
					cancel()
				}
				_ = stdoutReader.Close()
				return err
			})
	if err != nil && !isErrUnverifiedCommit(err) {
		log.Error("Unable to check commits from %s to %s in %s: %v", oldCommitID, newCommitID, repo.Path, err)
	}
	return err
}

func readAndVerifyCommitsFromShaReader(input io.ReadCloser, repo *git.Repository, env []string) error {
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		line := scanner.Text()
		err := readAndVerifyCommit(line, repo, env)
		if err != nil {
			log.Error("%v", err)
			return err
		}
	}
	return scanner.Err()
}

func readAndVerifyCommit(sha string, repo *git.Repository, env []string) error {
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		log.Error("Unable to create pipe for %s: %v", repo.Path, err)
		return err
	}
	defer func() {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
	}()
	hash := git.MustIDFromString(sha)

	return git.NewCommand("cat-file", "commit", sha).
		RunInDirTimeoutEnvFullPipelineFunc(env, -1, repo.Path,
			stdoutWriter, nil, nil,
			func(ctx context.Context, cancel context.CancelFunc) error {
				_ = stdoutWriter.Close()
				commit, err := git.CommitFromReader(repo, hash, stdoutReader)
				if err != nil {
					return err
				}
				verification := models.ParseCommitWithSignature(commit)
				if !verification.Verified {
					cancel()
					return &errUnverifiedCommit{
						commit.ID.String(),
					}
				}
				return nil
			})
}

type errUnverifiedCommit struct {
	sha string
}

func (e *errUnverifiedCommit) Error() string {
	return fmt.Sprintf("Unverified commit: %s", e.sha)
}

func isErrUnverifiedCommit(err error) bool {
	_, ok := err.(*errUnverifiedCommit)
	return ok
}

// HookPreReceive checks whether a individual commit is acceptable
func HookPreReceive(ctx *gitea_context.PrivateContext) {
	opts := web.GetForm(ctx).(*private.HookOptions)
	ownerName := ctx.Params(":owner")
	repoName := ctx.Params(":repo")
	repo, err := models.GetRepositoryByOwnerAndName(ownerName, repoName)
	if err != nil {
		log.Error("Unable to get repository: %s/%s Error: %v", ownerName, repoName, err)
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: err.Error(),
		})
		return
	}
	repo.OwnerName = ownerName
	gitRepo, err := git.OpenRepository(repo.RepoPath())
	if err != nil {
		log.Error("Unable to get git repository for: %s/%s Error: %v", ownerName, repoName, err)
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: err.Error(),
		})
		return
	}
	defer gitRepo.Close()

	// Generate git environment for checking commits
	env := os.Environ()
	if opts.GitAlternativeObjectDirectories != "" {
		env = append(env,
			private.GitAlternativeObjectDirectories+"="+opts.GitAlternativeObjectDirectories)
	}
	if opts.GitObjectDirectory != "" {
		env = append(env,
			private.GitObjectDirectory+"="+opts.GitObjectDirectory)
	}
	if opts.GitQuarantinePath != "" {
		env = append(env,
			private.GitQuarantinePath+"="+opts.GitQuarantinePath)
	}

	protectedTags, err := repo.GetProtectedTags()
	if err != nil {
		log.Error("Unable to get protected tags for %-v Error: %v", repo, err)
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: err.Error(),
		})
		return
	}

	// Iterate across the provided old commit IDs
	for i := range opts.OldCommitIDs {
		oldCommitID := opts.OldCommitIDs[i]
		newCommitID := opts.NewCommitIDs[i]
		refFullName := opts.RefFullNames[i]

		if strings.HasPrefix(refFullName, git.BranchPrefix) {
			branchName := strings.TrimPrefix(refFullName, git.BranchPrefix)
			if branchName == repo.DefaultBranch && newCommitID == git.EmptySHA {
				log.Warn("Forbidden: Branch: %s is the default branch in %-v and cannot be deleted", branchName, repo)
				ctx.JSON(http.StatusForbidden, private.Response{
					Err: fmt.Sprintf("branch %s is the default branch and cannot be deleted", branchName),
				})
				return
			}

			protectBranch, err := models.GetProtectedBranchBy(repo.ID, branchName)
			if err != nil {
				log.Error("Unable to get protected branch: %s in %-v Error: %v", branchName, repo, err)
				ctx.JSON(http.StatusInternalServerError, private.Response{
					Err: err.Error(),
				})
				return
			}

			// Allow pushes to non-protected branches
			if protectBranch == nil || !protectBranch.IsProtected() {
				continue
			}

			// This ref is a protected branch.
			//
			// First of all we need to enforce absolutely:
			//
			// 1. Detect and prevent deletion of the branch
			if newCommitID == git.EmptySHA {
				log.Warn("Forbidden: Branch: %s in %-v is protected from deletion", branchName, repo)
				ctx.JSON(http.StatusForbidden, private.Response{
					Err: fmt.Sprintf("branch %s is protected from deletion", branchName),
				})
				return
			}

			// 2. Disallow force pushes to protected branches
			if git.EmptySHA != oldCommitID {
				output, err := git.NewCommand("rev-list", "--max-count=1", oldCommitID, "^"+newCommitID).RunInDirWithEnv(repo.RepoPath(), env)
				if err != nil {
					log.Error("Unable to detect force push between: %s and %s in %-v Error: %v", oldCommitID, newCommitID, repo, err)
					ctx.JSON(http.StatusInternalServerError, private.Response{
						Err: fmt.Sprintf("Fail to detect force push: %v", err),
					})
					return
				} else if len(output) > 0 {
					log.Warn("Forbidden: Branch: %s in %-v is protected from force push", branchName, repo)
					ctx.JSON(http.StatusForbidden, private.Response{
						Err: fmt.Sprintf("branch %s is protected from force push", branchName),
					})
					return

				}
			}

			// 3. Enforce require signed commits
			if protectBranch.RequireSignedCommits {
				err := verifyCommits(oldCommitID, newCommitID, gitRepo, env)
				if err != nil {
					if !isErrUnverifiedCommit(err) {
						log.Error("Unable to check commits from %s to %s in %-v: %v", oldCommitID, newCommitID, repo, err)
						ctx.JSON(http.StatusInternalServerError, private.Response{
							Err: fmt.Sprintf("Unable to check commits from %s to %s: %v", oldCommitID, newCommitID, err),
						})
						return
					}
					unverifiedCommit := err.(*errUnverifiedCommit).sha
					log.Warn("Forbidden: Branch: %s in %-v is protected from unverified commit %s", branchName, repo, unverifiedCommit)
					ctx.JSON(http.StatusForbidden, private.Response{
						Err: fmt.Sprintf("branch %s is protected from unverified commit %s", branchName, unverifiedCommit),
					})
					return
				}
			}

			// Now there are several tests which can be overridden:
			//
			// 4. Check protected file patterns - this is overridable from the UI
			changedProtectedfiles := false
			protectedFilePath := ""

			globs := protectBranch.GetProtectedFilePatterns()
			if len(globs) > 0 {
				_, err := pull_service.CheckFileProtection(oldCommitID, newCommitID, globs, 1, env, gitRepo)
				if err != nil {
					if !models.IsErrFilePathProtected(err) {
						log.Error("Unable to check file protection for commits from %s to %s in %-v: %v", oldCommitID, newCommitID, repo, err)
						ctx.JSON(http.StatusInternalServerError, private.Response{
							Err: fmt.Sprintf("Unable to check file protection for commits from %s to %s: %v", oldCommitID, newCommitID, err),
						})
						return
					}

					changedProtectedfiles = true
					protectedFilePath = err.(models.ErrFilePathProtected).Path
				}
			}

			// 5. Check if the doer is allowed to push
			canPush := false
			if opts.IsDeployKey {
				canPush = !changedProtectedfiles && protectBranch.CanPush && (!protectBranch.EnableWhitelist || protectBranch.WhitelistDeployKeys)
			} else {
				canPush = !changedProtectedfiles && protectBranch.CanUserPush(opts.UserID)
			}

			// 6. If we're not allowed to push directly
			if !canPush {
				// Is this is a merge from the UI/API?
				if opts.PullRequestID == 0 {
					// 6a. If we're not merging from the UI/API then there are two ways we got here:
					//
					// We are changing a protected file and we're not allowed to do that
					if changedProtectedfiles {
						log.Warn("Forbidden: Branch: %s in %-v is protected from changing file %s", branchName, repo, protectedFilePath)
						ctx.JSON(http.StatusForbidden, private.Response{
							Err: fmt.Sprintf("branch %s is protected from changing file %s", branchName, protectedFilePath),
						})
						return
					}

					// Or we're simply not able to push to this protected branch
					log.Warn("Forbidden: User %d is not allowed to push to protected branch: %s in %-v", opts.UserID, branchName, repo)
					ctx.JSON(http.StatusForbidden, private.Response{
						Err: fmt.Sprintf("Not allowed to push to protected branch %s", branchName),
					})
					return
				}
				// 6b. Merge (from UI or API)

				// Get the PR, user and permissions for the user in the repository
				pr, err := models.GetPullRequestByID(opts.PullRequestID)
				if err != nil {
					log.Error("Unable to get PullRequest %d Error: %v", opts.PullRequestID, err)
					ctx.JSON(http.StatusInternalServerError, private.Response{
						Err: fmt.Sprintf("Unable to get PullRequest %d Error: %v", opts.PullRequestID, err),
					})
					return
				}
				user, err := models.GetUserByID(opts.UserID)
				if err != nil {
					log.Error("Unable to get User id %d Error: %v", opts.UserID, err)
					ctx.JSON(http.StatusInternalServerError, private.Response{
						Err: fmt.Sprintf("Unable to get User id %d Error: %v", opts.UserID, err),
					})
					return
				}
				perm, err := models.GetUserRepoPermission(repo, user)
				if err != nil {
					log.Error("Unable to get Repo permission of repo %s/%s of User %s", repo.OwnerName, repo.Name, user.Name, err)
					ctx.JSON(http.StatusInternalServerError, private.Response{
						Err: fmt.Sprintf("Unable to get Repo permission of repo %s/%s of User %s: %v", repo.OwnerName, repo.Name, user.Name, err),
					})
					return
				}

				// Now check if the user is allowed to merge PRs for this repository
				allowedMerge, err := pull_service.IsUserAllowedToMerge(pr, perm, user)
				if err != nil {
					log.Error("Error calculating if allowed to merge: %v", err)
					ctx.JSON(http.StatusInternalServerError, private.Response{
						Err: fmt.Sprintf("Error calculating if allowed to merge: %v", err),
					})
					return
				}

				if !allowedMerge {
					log.Warn("Forbidden: User %d is not allowed to push to protected branch: %s in %-v and is not allowed to merge pr #%d", opts.UserID, branchName, repo, pr.Index)
					ctx.JSON(http.StatusForbidden, private.Response{
						Err: fmt.Sprintf("Not allowed to push to protected branch %s", branchName),
					})
					return
				}

				// If we're an admin for the repository we can ignore status checks, reviews and override protected files
				if perm.IsAdmin() {
					continue
				}

				// Now if we're not an admin - we can't overwrite protected files so fail now
				if changedProtectedfiles {
					log.Warn("Forbidden: Branch: %s in %-v is protected from changing file %s", branchName, repo, protectedFilePath)
					ctx.JSON(http.StatusForbidden, private.Response{
						Err: fmt.Sprintf("branch %s is protected from changing file %s", branchName, protectedFilePath),
					})
					return
				}

				// Check all status checks and reviews are ok
				if err := pull_service.CheckPRReadyToMerge(pr, true); err != nil {
					if models.IsErrNotAllowedToMerge(err) {
						log.Warn("Forbidden: User %d is not allowed push to protected branch %s in %-v and pr #%d is not ready to be merged: %s", opts.UserID, branchName, repo, pr.Index, err.Error())
						ctx.JSON(http.StatusForbidden, private.Response{
							Err: fmt.Sprintf("Not allowed to push to protected branch %s and pr #%d is not ready to be merged: %s", branchName, opts.PullRequestID, err.Error()),
						})
						return
					}
					log.Error("Unable to check if mergable: protected branch %s in %-v and pr #%d. Error: %v", opts.UserID, branchName, repo, pr.Index, err)
					ctx.JSON(http.StatusInternalServerError, private.Response{
						Err: fmt.Sprintf("Unable to get status of pull request %d. Error: %v", opts.PullRequestID, err),
					})
					return
				}
			}
		} else if strings.HasPrefix(refFullName, git.TagPrefix) {
			tagName := strings.TrimPrefix(refFullName, git.TagPrefix)

			isAllowed, err := models.IsUserAllowedToControlTag(protectedTags, tagName, opts.UserID)
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, private.Response{
					Err: err.Error(),
				})
				return
			}
			if !isAllowed {
				log.Warn("Forbidden: Tag %s in %-v is protected", tagName, repo)
				ctx.JSON(http.StatusForbidden, private.Response{
					Err: fmt.Sprintf("Tag %s is protected", tagName),
				})
				return
			}
		} else {
			log.Error("Unexpected ref: %s", refFullName)
			ctx.JSON(http.StatusInternalServerError, private.Response{
				Err: fmt.Sprintf("Unexpected ref: %s", refFullName),
			})
		}
	}

	ctx.PlainText(http.StatusOK, []byte("ok"))
}

// HookPostReceive updates services and users
func HookPostReceive(ctx *gitea_context.PrivateContext) {
	opts := web.GetForm(ctx).(*private.HookOptions)
	ownerName := ctx.Params(":owner")
	repoName := ctx.Params(":repo")

	var repo *models.Repository
	updates := make([]*repo_module.PushUpdateOptions, 0, len(opts.OldCommitIDs))
	wasEmpty := false

	for i := range opts.OldCommitIDs {
		refFullName := opts.RefFullNames[i]

		// Only trigger activity updates for changes to branches or
		// tags.  Updates to other refs (eg, refs/notes, refs/changes,
		// or other less-standard refs spaces are ignored since there
		// may be a very large number of them).
		if strings.HasPrefix(refFullName, git.BranchPrefix) || strings.HasPrefix(refFullName, git.TagPrefix) {
			if repo == nil {
				var err error
				repo, err = models.GetRepositoryByOwnerAndName(ownerName, repoName)
				if err != nil {
					log.Error("Failed to get repository: %s/%s Error: %v", ownerName, repoName, err)
					ctx.JSON(http.StatusInternalServerError, private.HookPostReceiveResult{
						Err: fmt.Sprintf("Failed to get repository: %s/%s Error: %v", ownerName, repoName, err),
					})
					return
				}
				if repo.OwnerName == "" {
					repo.OwnerName = ownerName
				}
				wasEmpty = repo.IsEmpty
			}

			option := repo_module.PushUpdateOptions{
				RefFullName:  refFullName,
				OldCommitID:  opts.OldCommitIDs[i],
				NewCommitID:  opts.NewCommitIDs[i],
				PusherID:     opts.UserID,
				PusherName:   opts.UserName,
				RepoUserName: ownerName,
				RepoName:     repoName,
			}
			updates = append(updates, &option)
			if repo.IsEmpty && option.IsBranch() && (option.BranchName() == "master" || option.BranchName() == "main") {
				// put the master/main branch first
				copy(updates[1:], updates)
				updates[0] = &option
			}
		}
	}

	if repo != nil && len(updates) > 0 {
		if err := repo_service.PushUpdates(updates); err != nil {
			log.Error("Failed to Update: %s/%s Total Updates: %d", ownerName, repoName, len(updates))
			for i, update := range updates {
				log.Error("Failed to Update: %s/%s Update: %d/%d: Branch: %s", ownerName, repoName, i, len(updates), update.BranchName())
			}
			log.Error("Failed to Update: %s/%s Error: %v", ownerName, repoName, err)

			ctx.JSON(http.StatusInternalServerError, private.HookPostReceiveResult{
				Err: fmt.Sprintf("Failed to Update: %s/%s Error: %v", ownerName, repoName, err),
			})
			return
		}
	}

	// Push Options
	if repo != nil && len(opts.GitPushOptions) > 0 {
		repo.IsPrivate = opts.GitPushOptions.Bool(private.GitPushOptionRepoPrivate, repo.IsPrivate)
		repo.IsTemplate = opts.GitPushOptions.Bool(private.GitPushOptionRepoTemplate, repo.IsTemplate)
		if err := models.UpdateRepositoryCols(repo, "is_private", "is_template"); err != nil {
			log.Error("Failed to Update: %s/%s Error: %v", ownerName, repoName, err)
			ctx.JSON(http.StatusInternalServerError, private.HookPostReceiveResult{
				Err: fmt.Sprintf("Failed to Update: %s/%s Error: %v", ownerName, repoName, err),
			})
		}
	}

	results := make([]private.HookPostReceiveBranchResult, 0, len(opts.OldCommitIDs))

	// We have to reload the repo in case its state is changed above
	repo = nil
	var baseRepo *models.Repository

	for i := range opts.OldCommitIDs {
		refFullName := opts.RefFullNames[i]
		newCommitID := opts.NewCommitIDs[i]

		branch := git.RefEndName(opts.RefFullNames[i])

		if newCommitID != git.EmptySHA && strings.HasPrefix(refFullName, git.BranchPrefix) {
			if repo == nil {
				var err error
				repo, err = models.GetRepositoryByOwnerAndName(ownerName, repoName)
				if err != nil {
					log.Error("Failed to get repository: %s/%s Error: %v", ownerName, repoName, err)
					ctx.JSON(http.StatusInternalServerError, private.HookPostReceiveResult{
						Err:          fmt.Sprintf("Failed to get repository: %s/%s Error: %v", ownerName, repoName, err),
						RepoWasEmpty: wasEmpty,
					})
					return
				}
				if repo.OwnerName == "" {
					repo.OwnerName = ownerName
				}

				if !repo.AllowsPulls() {
					// We can stop there's no need to go any further
					ctx.JSON(http.StatusOK, private.HookPostReceiveResult{
						RepoWasEmpty: wasEmpty,
					})
					return
				}
				baseRepo = repo

				if repo.IsFork {
					if err := repo.GetBaseRepo(); err != nil {
						log.Error("Failed to get Base Repository of Forked repository: %-v Error: %v", repo, err)
						ctx.JSON(http.StatusInternalServerError, private.HookPostReceiveResult{
							Err:          fmt.Sprintf("Failed to get Base Repository of Forked repository: %-v Error: %v", repo, err),
							RepoWasEmpty: wasEmpty,
						})
						return
					}
					baseRepo = repo.BaseRepo
				}
			}

			if !repo.IsFork && branch == baseRepo.DefaultBranch {
				results = append(results, private.HookPostReceiveBranchResult{})
				continue
			}

			pr, err := models.GetUnmergedPullRequest(repo.ID, baseRepo.ID, branch, baseRepo.DefaultBranch)
			if err != nil && !models.IsErrPullRequestNotExist(err) {
				log.Error("Failed to get active PR in: %-v Branch: %s to: %-v Branch: %s Error: %v", repo, branch, baseRepo, baseRepo.DefaultBranch, err)
				ctx.JSON(http.StatusInternalServerError, private.HookPostReceiveResult{
					Err: fmt.Sprintf(
						"Failed to get active PR in: %-v Branch: %s to: %-v Branch: %s Error: %v", repo, branch, baseRepo, baseRepo.DefaultBranch, err),
					RepoWasEmpty: wasEmpty,
				})
				return
			}

			if pr == nil {
				if repo.IsFork {
					branch = fmt.Sprintf("%s:%s", repo.OwnerName, branch)
				}
				results = append(results, private.HookPostReceiveBranchResult{
					Message: setting.Git.PullRequestPushMessage && repo.AllowsPulls(),
					Create:  true,
					Branch:  branch,
					URL:     fmt.Sprintf("%s/compare/%s...%s", baseRepo.HTMLURL(), util.PathEscapeSegments(baseRepo.DefaultBranch), util.PathEscapeSegments(branch)),
				})
			} else {
				results = append(results, private.HookPostReceiveBranchResult{
					Message: setting.Git.PullRequestPushMessage && repo.AllowsPulls(),
					Create:  false,
					Branch:  branch,
					URL:     fmt.Sprintf("%s/pulls/%d", baseRepo.HTMLURL(), pr.Index),
				})
			}
		}
	}
	ctx.JSON(http.StatusOK, private.HookPostReceiveResult{
		Results:      results,
		RepoWasEmpty: wasEmpty,
	})
}

// SetDefaultBranch updates the default branch
func SetDefaultBranch(ctx *gitea_context.PrivateContext) {
	ownerName := ctx.Params(":owner")
	repoName := ctx.Params(":repo")
	branch := ctx.Params(":branch")
	repo, err := models.GetRepositoryByOwnerAndName(ownerName, repoName)
	if err != nil {
		log.Error("Failed to get repository: %s/%s Error: %v", ownerName, repoName, err)
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: fmt.Sprintf("Failed to get repository: %s/%s Error: %v", ownerName, repoName, err),
		})
		return
	}
	if repo.OwnerName == "" {
		repo.OwnerName = ownerName
	}

	repo.DefaultBranch = branch
	gitRepo, err := git.OpenRepository(repo.RepoPath())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: fmt.Sprintf("Failed to get git repository: %s/%s Error: %v", ownerName, repoName, err),
		})
		return
	}
	if err := gitRepo.SetDefaultBranch(repo.DefaultBranch); err != nil {
		if !git.IsErrUnsupportedVersion(err) {
			gitRepo.Close()
			ctx.JSON(http.StatusInternalServerError, private.Response{
				Err: fmt.Sprintf("Unable to set default branch on repository: %s/%s Error: %v", ownerName, repoName, err),
			})
			return
		}
	}
	gitRepo.Close()

	if err := repo.UpdateDefaultBranch(); err != nil {
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: fmt.Sprintf("Unable to set default branch on repository: %s/%s Error: %v", ownerName, repoName, err),
		})
		return
	}
	ctx.PlainText(http.StatusOK, []byte("success"))
}
