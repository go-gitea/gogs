// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"bufio"
	"encoding/csv"
	"errors"
	"fmt"
	"html"
	"net/http"
	"path"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/context"
	csv_module "code.gitea.io/gitea/modules/csv"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/upload"
	"code.gitea.io/gitea/services/gitdiff"
)

const (
	tplCompare     base.TplName = "repo/diff/compare"
	tplBlobExcerpt base.TplName = "repo/diff/blob_excerpt"
)

// setCompareContext sets context data.
func setCompareContext(ctx *context.Context, base *git.Commit, head *git.Commit, headTarget string) {
	ctx.Data["BaseCommit"] = base
	ctx.Data["HeadCommit"] = head

	ctx.Data["GetBlobByPathForCommit"] = func(commit *git.Commit, path string) *git.Blob {
		if commit == nil {
			return nil
		}

		blob, err := commit.GetBlobByPath(path)
		if err != nil {
			return nil
		}
		return blob
	}

	setPathsCompareContext(ctx, base, head, headTarget)
	setImageCompareContext(ctx)
	setCsvCompareContext(ctx)
}

// setPathsCompareContext sets context data for source and raw paths
func setPathsCompareContext(ctx *context.Context, base *git.Commit, head *git.Commit, headTarget string) {
	sourcePath := setting.AppSubURL + "/%s/src/commit/%s"
	rawPath := setting.AppSubURL + "/%s/raw/commit/%s"

	ctx.Data["SourcePath"] = fmt.Sprintf(sourcePath, headTarget, head.ID)
	ctx.Data["RawPath"] = fmt.Sprintf(rawPath, headTarget, head.ID)
	if base != nil {
		baseTarget := path.Join(ctx.Repo.Owner.Name, ctx.Repo.Repository.Name)
		ctx.Data["BeforeSourcePath"] = fmt.Sprintf(sourcePath, baseTarget, base.ID)
		ctx.Data["BeforeRawPath"] = fmt.Sprintf(rawPath, baseTarget, base.ID)
	}
}

// setImageCompareContext sets context data that is required by image compare template
func setImageCompareContext(ctx *context.Context) {
	ctx.Data["IsBlobAnImage"] = func(blob *git.Blob) bool {
		if blob == nil {
			return false
		}

		st, err := blob.GuessContentType()
		if err != nil {
			log.Error("GuessContentType failed: %v", err)
			return false
		}
		return st.IsImage() && (setting.UI.SVG.Enabled || !st.IsSvgImage())
	}
}

// setCsvCompareContext sets context data that is required by the CSV compare template
func setCsvCompareContext(ctx *context.Context) {
	ctx.Data["IsCsvFile"] = func(diffFile *gitdiff.DiffFile) bool {
		extension := strings.ToLower(filepath.Ext(diffFile.Name))
		return extension == ".csv" || extension == ".tsv"
	}

	type CsvDiffResult struct {
		Sections []*gitdiff.TableDiffSection
		Error    string
	}

	ctx.Data["CreateCsvDiff"] = func(diffFile *gitdiff.DiffFile, baseCommit *git.Commit, headCommit *git.Commit) CsvDiffResult {
		if diffFile == nil || baseCommit == nil || headCommit == nil {
			return CsvDiffResult{nil, ""}
		}

		errTooLarge := errors.New(ctx.Locale.Tr("repo.error.csv.too_large"))

		csvReaderFromCommit := func(c *git.Commit) (*csv.Reader, error) {
			blob, err := c.GetBlobByPath(diffFile.Name)
			if err != nil {
				return nil, err
			}

			if setting.UI.CSV.MaxFileSize != 0 && setting.UI.CSV.MaxFileSize < blob.Size() {
				return nil, errTooLarge
			}

			reader, err := blob.DataAsync()
			if err != nil {
				return nil, err
			}
			defer reader.Close()

			return csv_module.CreateReaderAndGuessDelimiter(charset.ToUTF8WithFallbackReader(reader))
		}

		baseReader, err := csvReaderFromCommit(baseCommit)
		if err == errTooLarge {
			return CsvDiffResult{nil, err.Error()}
		}
		headReader, err := csvReaderFromCommit(headCommit)
		if err == errTooLarge {
			return CsvDiffResult{nil, err.Error()}
		}

		sections, err := gitdiff.CreateCsvDiff(diffFile, baseReader, headReader)
		if err != nil {
			errMessage, err := csv_module.FormatError(err, ctx.Locale)
			if err != nil {
				log.Error("RenderCsvDiff failed: %v", err)
				return CsvDiffResult{nil, ""}
			}
			return CsvDiffResult{nil, errMessage}
		}
		return CsvDiffResult{sections, ""}
	}
}

// ParseCompareInfo parse compare info between two commit for preparing comparing references
func ParseCompareInfo(ctx *context.Context) (*models.User, *models.Repository, *git.Repository, *git.CompareInfo, string, string) {
	baseRepo := ctx.Repo.Repository

	// Get compared branches information
	// A full compare url is of the form:
	//
	// 1. /{:baseOwner}/{:baseRepoName}/compare/{:baseBranch}...{:headBranch}
	// 2. /{:baseOwner}/{:baseRepoName}/compare/{:baseBranch}...{:headOwner}:{:headBranch}
	// 3. /{:baseOwner}/{:baseRepoName}/compare/{:baseBranch}...{:headOwner}/{:headRepoName}:{:headBranch}
	//
	// Here we obtain the infoPath "{:baseBranch}...[{:headOwner}/{:headRepoName}:]{:headBranch}" as ctx.Params("*")
	// with the :baseRepo in ctx.Repo.
	//
	// Note: Generally :headRepoName is not provided here - we are only passed :headOwner.
	//
	// How do we determine the :headRepo?
	//
	// 1. If :headOwner is not set then the :headRepo = :baseRepo
	// 2. If :headOwner is set - then look for the fork of :baseRepo owned by :headOwner
	// 3. But... :baseRepo could be a fork of :headOwner's repo - so check that
	// 4. Now, :baseRepo and :headRepos could be forks of the same repo - so check that
	//
	// format: <base branch>...[<head repo>:]<head branch>
	// base<-head: master...head:feature
	// same repo: master...feature

	var (
		headUser   *models.User
		headRepo   *models.Repository
		headBranch string
		isSameRepo bool
		infoPath   string
		err        error
	)
	infoPath = ctx.Params("*")
	infos := strings.SplitN(infoPath, "...", 2)
	if len(infos) != 2 {
		log.Trace("ParseCompareInfo[%d]: not enough compared branches information %s", baseRepo.ID, infos)
		ctx.NotFound("CompareAndPullRequest", nil)
		return nil, nil, nil, nil, "", ""
	}

	ctx.Data["BaseName"] = baseRepo.OwnerName
	baseBranch := infos[0]
	ctx.Data["BaseBranch"] = baseBranch

	// If there is no head repository, it means compare between same repository.
	headInfos := strings.Split(infos[1], ":")
	if len(headInfos) == 1 {
		isSameRepo = true
		headUser = ctx.Repo.Owner
		headBranch = headInfos[0]

	} else if len(headInfos) == 2 {
		headInfosSplit := strings.Split(headInfos[0], "/")
		if len(headInfosSplit) == 1 {
			headUser, err = models.GetUserByName(headInfos[0])
			if err != nil {
				if models.IsErrUserNotExist(err) {
					ctx.NotFound("GetUserByName", nil)
				} else {
					ctx.ServerError("GetUserByName", err)
				}
				return nil, nil, nil, nil, "", ""
			}
			headBranch = headInfos[1]
			isSameRepo = headUser.ID == ctx.Repo.Owner.ID
			if isSameRepo {
				headRepo = baseRepo
			}
		} else {
			headRepo, err = models.GetRepositoryByOwnerAndName(headInfosSplit[0], headInfosSplit[1])
			if err != nil {
				if models.IsErrRepoNotExist(err) {
					ctx.NotFound("GetRepositoryByOwnerAndName", nil)
				} else {
					ctx.ServerError("GetRepositoryByOwnerAndName", err)
				}
				return nil, nil, nil, nil, "", ""
			}
			if err := headRepo.GetOwner(); err != nil {
				if models.IsErrUserNotExist(err) {
					ctx.NotFound("GetUserByName", nil)
				} else {
					ctx.ServerError("GetUserByName", err)
				}
				return nil, nil, nil, nil, "", ""
			}
			headBranch = headInfos[1]
			headUser = headRepo.Owner
			isSameRepo = headRepo.ID == ctx.Repo.Repository.ID
		}
	} else {
		ctx.NotFound("CompareAndPullRequest", nil)
		return nil, nil, nil, nil, "", ""
	}
	ctx.Data["HeadUser"] = headUser
	ctx.Data["HeadBranch"] = headBranch
	ctx.Repo.PullRequest.SameRepo = isSameRepo

	// Check if base branch is valid.
	baseIsCommit := ctx.Repo.GitRepo.IsCommitExist(baseBranch)
	baseIsBranch := ctx.Repo.GitRepo.IsBranchExist(baseBranch)
	baseIsTag := ctx.Repo.GitRepo.IsTagExist(baseBranch)
	if !baseIsCommit && !baseIsBranch && !baseIsTag {
		// Check if baseBranch is short sha commit hash
		if baseCommit, _ := ctx.Repo.GitRepo.GetCommit(baseBranch); baseCommit != nil {
			baseBranch = baseCommit.ID.String()
			ctx.Data["BaseBranch"] = baseBranch
			baseIsCommit = true
		} else {
			ctx.NotFound("IsRefExist", nil)
			return nil, nil, nil, nil, "", ""
		}
	}
	ctx.Data["BaseIsCommit"] = baseIsCommit
	ctx.Data["BaseIsBranch"] = baseIsBranch
	ctx.Data["BaseIsTag"] = baseIsTag
	ctx.Data["IsPull"] = true

	// Now we have the repository that represents the base

	// The current base and head repositories and branches may not
	// actually be the intended branches that the user wants to
	// create a pull-request from - but also determining the head
	// repo is difficult.

	// We will want therefore to offer a few repositories to set as
	// our base and head

	// 1. First if the baseRepo is a fork get the "RootRepo" it was
	// forked from
	var rootRepo *models.Repository
	if baseRepo.IsFork {
		err = baseRepo.GetBaseRepo()
		if err != nil {
			if !models.IsErrRepoNotExist(err) {
				ctx.ServerError("Unable to find root repo", err)
				return nil, nil, nil, nil, "", ""
			}
		} else {
			rootRepo = baseRepo.BaseRepo
		}
	}

	// 2. Now if the current user is not the owner of the baseRepo,
	// check if they have a fork of the base repo and offer that as
	// "OwnForkRepo"
	var ownForkRepo *models.Repository
	if ctx.User != nil && baseRepo.OwnerID != ctx.User.ID {
		repo, has := models.HasForkedRepo(ctx.User.ID, baseRepo.ID)
		if has {
			ownForkRepo = repo
			ctx.Data["OwnForkRepo"] = ownForkRepo
		}
	}

	has := headRepo != nil
	// 3. If the base is a forked from "RootRepo" and the owner of
	// the "RootRepo" is the :headUser - set headRepo to that
	if !has && rootRepo != nil && rootRepo.OwnerID == headUser.ID {
		headRepo = rootRepo
		has = true
	}

	// 4. If the ctx.User has their own fork of the baseRepo and the headUser is the ctx.User
	// set the headRepo to the ownFork
	if !has && ownForkRepo != nil && ownForkRepo.OwnerID == headUser.ID {
		headRepo = ownForkRepo
		has = true
	}

	// 5. If the headOwner has a fork of the baseRepo - use that
	if !has {
		headRepo, has = models.HasForkedRepo(headUser.ID, baseRepo.ID)
	}

	// 6. If the baseRepo is a fork and the headUser has a fork of that use that
	if !has && baseRepo.IsFork {
		headRepo, has = models.HasForkedRepo(headUser.ID, baseRepo.ForkID)
	}

	// 7. Otherwise if we're not the same repo and haven't found a repo give up
	if !isSameRepo && !has {
		ctx.Data["PageIsComparePull"] = false
	}

	// 8. Finally open the git repo
	var headGitRepo *git.Repository
	if isSameRepo {
		headRepo = ctx.Repo.Repository
		headGitRepo = ctx.Repo.GitRepo
	} else if has {
		headGitRepo, err = git.OpenRepository(headRepo.RepoPath())
		if err != nil {
			ctx.ServerError("OpenRepository", err)
			return nil, nil, nil, nil, "", ""
		}
		defer headGitRepo.Close()
	}

	ctx.Data["HeadRepo"] = headRepo

	// Now we need to assert that the ctx.User has permission to read
	// the baseRepo's code and pulls
	// (NOT headRepo's)
	permBase, err := models.GetUserRepoPermission(baseRepo, ctx.User)
	if err != nil {
		ctx.ServerError("GetUserRepoPermission", err)
		return nil, nil, nil, nil, "", ""
	}
	if !permBase.CanRead(models.UnitTypeCode) {
		if log.IsTrace() {
			log.Trace("Permission Denied: User: %-v cannot read code in Repo: %-v\nUser in baseRepo has Permissions: %-+v",
				ctx.User,
				baseRepo,
				permBase)
		}
		ctx.NotFound("ParseCompareInfo", nil)
		return nil, nil, nil, nil, "", ""
	}

	// If we're not merging from the same repo:
	if !isSameRepo {
		// Assert ctx.User has permission to read headRepo's codes
		permHead, err := models.GetUserRepoPermission(headRepo, ctx.User)
		if err != nil {
			ctx.ServerError("GetUserRepoPermission", err)
			return nil, nil, nil, nil, "", ""
		}
		if !permHead.CanRead(models.UnitTypeCode) {
			if log.IsTrace() {
				log.Trace("Permission Denied: User: %-v cannot read code in Repo: %-v\nUser in headRepo has Permissions: %-+v",
					ctx.User,
					headRepo,
					permHead)
			}
			ctx.NotFound("ParseCompareInfo", nil)
			return nil, nil, nil, nil, "", ""
		}
	}

	// If we have a rootRepo and it's different from:
	// 1. the computed base
	// 2. the computed head
	// then get the branches of it
	if rootRepo != nil &&
		rootRepo.ID != headRepo.ID &&
		rootRepo.ID != baseRepo.ID {
		perm, branches, tags, err := getBranchesAndTagsForRepo(ctx.User, rootRepo)
		if err != nil {
			ctx.ServerError("GetBranchesForRepo", err)
			return nil, nil, nil, nil, "", ""
		}
		if perm {
			ctx.Data["RootRepo"] = rootRepo
			ctx.Data["RootRepoBranches"] = branches
			ctx.Data["RootRepoTags"] = tags
		}
	}

	// If we have a ownForkRepo and it's different from:
	// 1. The computed base
	// 2. The computed head
	// 3. The rootRepo (if we have one)
	// then get the branches from it.
	if ownForkRepo != nil &&
		ownForkRepo.ID != headRepo.ID &&
		ownForkRepo.ID != baseRepo.ID &&
		(rootRepo == nil || ownForkRepo.ID != rootRepo.ID) {
		perm, branches, tags, err := getBranchesAndTagsForRepo(ctx.User, ownForkRepo)
		if err != nil {
			ctx.ServerError("GetBranchesForRepo", err)
			return nil, nil, nil, nil, "", ""
		}
		if perm {
			ctx.Data["OwnForkRepo"] = ownForkRepo
			ctx.Data["OwnForkRepoBranches"] = branches
			ctx.Data["OwnForkRepoTags"] = tags
		}
	}

	// Check if head branch is valid.
	headIsCommit := headGitRepo.IsCommitExist(headBranch)
	headIsBranch := headGitRepo.IsBranchExist(headBranch)
	headIsTag := headGitRepo.IsTagExist(headBranch)
	if !headIsCommit && !headIsBranch && !headIsTag {
		// Check if headBranch is short sha commit hash
		if headCommit, _ := headGitRepo.GetCommit(headBranch); headCommit != nil {
			headBranch = headCommit.ID.String()
			ctx.Data["HeadBranch"] = headBranch
			headIsCommit = true
		} else {
			ctx.NotFound("IsRefExist", nil)
			return nil, nil, nil, nil, "", ""
		}
	}
	ctx.Data["HeadIsCommit"] = headIsCommit
	ctx.Data["HeadIsBranch"] = headIsBranch
	ctx.Data["HeadIsTag"] = headIsTag

	// Treat as pull request if both references are branches
	if ctx.Data["PageIsComparePull"] == nil {
		ctx.Data["PageIsComparePull"] = headIsBranch && baseIsBranch
	}

	if ctx.Data["PageIsComparePull"] == true && !permBase.CanReadIssuesOrPulls(true) {
		if log.IsTrace() {
			log.Trace("Permission Denied: User: %-v cannot create/read pull requests in Repo: %-v\nUser in baseRepo has Permissions: %-+v",
				ctx.User,
				baseRepo,
				permBase)
		}
		ctx.NotFound("ParseCompareInfo", nil)
		return nil, nil, nil, nil, "", ""
	}

	baseBranchRef := baseBranch
	if baseIsBranch {
		baseBranchRef = git.BranchPrefix + baseBranch
	} else if baseIsTag {
		baseBranchRef = git.TagPrefix + baseBranch
	}
	headBranchRef := headBranch
	if headIsBranch {
		headBranchRef = git.BranchPrefix + headBranch
	} else if headIsTag {
		headBranchRef = git.TagPrefix + headBranch
	}

	compareInfo, err := headGitRepo.GetCompareInfo(baseRepo.RepoPath(), baseBranchRef, headBranchRef)
	if err != nil {
		ctx.ServerError("GetCompareInfo", err)
		return nil, nil, nil, nil, "", ""
	}
	ctx.Data["BeforeCommitID"] = compareInfo.MergeBase

	return headUser, headRepo, headGitRepo, compareInfo, baseBranch, headBranch
}

// PrepareCompareDiff renders compare diff page
func PrepareCompareDiff(
	ctx *context.Context,
	headUser *models.User,
	headRepo *models.Repository,
	headGitRepo *git.Repository,
	compareInfo *git.CompareInfo,
	baseBranch, headBranch string,
	whitespaceBehavior string) bool {

	var (
		repo  = ctx.Repo.Repository
		err   error
		title string
	)

	// Get diff information.
	ctx.Data["CommitRepoLink"] = headRepo.Link()

	headCommitID := compareInfo.HeadCommitID

	ctx.Data["AfterCommitID"] = headCommitID

	if headCommitID == compareInfo.MergeBase {
		ctx.Data["IsNothingToCompare"] = true
		if unit, err := repo.GetUnit(models.UnitTypePullRequests); err == nil {
			config := unit.PullRequestsConfig()

			if !config.AutodetectManualMerge {
				allowEmptyPr := !(baseBranch == headBranch && ctx.Repo.Repository.Name == headRepo.Name)
				ctx.Data["AllowEmptyPr"] = allowEmptyPr

				return !allowEmptyPr
			}

			ctx.Data["AllowEmptyPr"] = false
		}
		return true
	}

	diff, err := gitdiff.GetDiffRangeWithWhitespaceBehavior(models.RepoPath(headUser.Name, headRepo.Name),
		compareInfo.MergeBase, headCommitID, setting.Git.MaxGitDiffLines,
		setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles, whitespaceBehavior)
	if err != nil {
		ctx.ServerError("GetDiffRangeWithWhitespaceBehavior", err)
		return false
	}
	ctx.Data["Diff"] = diff
	ctx.Data["DiffNotAvailable"] = diff.NumFiles == 0

	headCommit, err := headGitRepo.GetCommit(headCommitID)
	if err != nil {
		ctx.ServerError("GetCommit", err)
		return false
	}

	baseGitRepo := ctx.Repo.GitRepo
	baseCommitID := compareInfo.BaseCommitID

	baseCommit, err := baseGitRepo.GetCommit(baseCommitID)
	if err != nil {
		ctx.ServerError("GetCommit", err)
		return false
	}

	compareInfo.Commits = models.ValidateCommitsWithEmails(compareInfo.Commits)
	compareInfo.Commits = models.ParseCommitsWithSignature(compareInfo.Commits, headRepo)
	compareInfo.Commits = models.ParseCommitsWithStatus(compareInfo.Commits, headRepo)
	ctx.Data["Commits"] = compareInfo.Commits
	ctx.Data["CommitCount"] = compareInfo.Commits.Len()

	if compareInfo.Commits.Len() == 1 {
		c := compareInfo.Commits.Front().Value.(models.SignCommitWithStatuses)
		title = strings.TrimSpace(c.UserCommit.Summary())

		body := strings.Split(strings.TrimSpace(c.UserCommit.Message()), "\n")
		if len(body) > 1 {
			ctx.Data["content"] = strings.Join(body[1:], "\n")
		}
	} else {
		title = headBranch
	}
	ctx.Data["title"] = title
	ctx.Data["Username"] = headUser.Name
	ctx.Data["Reponame"] = headRepo.Name

	headTarget := path.Join(headUser.Name, repo.Name)
	setCompareContext(ctx, baseCommit, headCommit, headTarget)

	return false
}

func getBranchesAndTagsForRepo(user *models.User, repo *models.Repository) (bool, []string, []string, error) {
	perm, err := models.GetUserRepoPermission(repo, user)
	if err != nil {
		return false, nil, nil, err
	}
	if !perm.CanRead(models.UnitTypeCode) {
		return false, nil, nil, nil
	}
	gitRepo, err := git.OpenRepository(repo.RepoPath())
	if err != nil {
		return false, nil, nil, err
	}
	defer gitRepo.Close()

	branches, _, err := gitRepo.GetBranches(0, 0)
	if err != nil {
		return false, nil, nil, err
	}
	tags, err := gitRepo.GetTags()
	if err != nil {
		return false, nil, nil, err
	}
	return true, branches, tags, nil
}

// CompareDiff show different from one commit to another commit
func CompareDiff(ctx *context.Context) {
	headUser, headRepo, headGitRepo, compareInfo, baseBranch, headBranch := ParseCompareInfo(ctx)

	if ctx.Written() {
		return
	}
	defer headGitRepo.Close()

	nothingToCompare := PrepareCompareDiff(ctx, headUser, headRepo, headGitRepo, compareInfo, baseBranch, headBranch,
		gitdiff.GetWhitespaceFlag(ctx.Data["WhitespaceBehavior"].(string)))
	if ctx.Written() {
		return
	}

	baseGitRepo := ctx.Repo.GitRepo
	baseTags, err := baseGitRepo.GetTags()
	if err != nil {
		ctx.ServerError("GetTags", err)
		return
	}
	ctx.Data["Tags"] = baseTags

	headBranches, _, err := headGitRepo.GetBranches(0, 0)
	if err != nil {
		ctx.ServerError("GetBranches", err)
		return
	}
	ctx.Data["HeadBranches"] = headBranches

	headTags, err := headGitRepo.GetTags()
	if err != nil {
		ctx.ServerError("GetTags", err)
		return
	}
	ctx.Data["HeadTags"] = headTags

	if ctx.Data["PageIsComparePull"] == true {
		pr, err := models.GetUnmergedPullRequest(headRepo.ID, ctx.Repo.Repository.ID, headBranch, baseBranch)
		if err != nil {
			if !models.IsErrPullRequestNotExist(err) {
				ctx.ServerError("GetUnmergedPullRequest", err)
				return
			}
		} else {
			ctx.Data["HasPullRequest"] = true
			ctx.Data["PullRequest"] = pr
			ctx.HTML(http.StatusOK, tplCompareDiff)
			return
		}

		if !nothingToCompare {
			// Setup information for new form.
			RetrieveRepoMetas(ctx, ctx.Repo.Repository, true)
			if ctx.Written() {
				return
			}
		}
	}
	beforeCommitID := ctx.Data["BeforeCommitID"].(string)
	afterCommitID := ctx.Data["AfterCommitID"].(string)

	ctx.Data["Title"] = "Comparing " + base.ShortSha(beforeCommitID) + "..." + base.ShortSha(afterCommitID)

	ctx.Data["IsRepoToolbarCommits"] = true
	ctx.Data["IsDiffCompare"] = true
	ctx.Data["RequireTribute"] = true
	ctx.Data["RequireSimpleMDE"] = true
	ctx.Data["PullRequestWorkInProgressPrefixes"] = setting.Repository.PullRequest.WorkInProgressPrefixes
	setTemplateIfExists(ctx, pullRequestTemplateKey, nil, pullRequestTemplateCandidates)
	ctx.Data["IsAttachmentEnabled"] = setting.Attachment.Enabled
	upload.AddUploadContext(ctx, "comment")

	ctx.Data["HasIssuesOrPullsWritePermission"] = ctx.Repo.CanWrite(models.UnitTypePullRequests)

	ctx.HTML(http.StatusOK, tplCompare)
}

// ExcerptBlob render blob excerpt contents
func ExcerptBlob(ctx *context.Context) {
	commitID := ctx.Params("sha")
	lastLeft := ctx.QueryInt("last_left")
	lastRight := ctx.QueryInt("last_right")
	idxLeft := ctx.QueryInt("left")
	idxRight := ctx.QueryInt("right")
	leftHunkSize := ctx.QueryInt("left_hunk_size")
	rightHunkSize := ctx.QueryInt("right_hunk_size")
	anchor := ctx.Query("anchor")
	direction := ctx.Query("direction")
	filePath := ctx.Query("path")
	gitRepo := ctx.Repo.GitRepo
	chunkSize := gitdiff.BlobExcerptChunkSize
	commit, err := gitRepo.GetCommit(commitID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetCommit")
		return
	}
	section := &gitdiff.DiffSection{
		FileName: filePath,
		Name:     filePath,
	}
	if direction == "up" && (idxLeft-lastLeft) > chunkSize {
		idxLeft -= chunkSize
		idxRight -= chunkSize
		leftHunkSize += chunkSize
		rightHunkSize += chunkSize
		section.Lines, err = getExcerptLines(commit, filePath, idxLeft-1, idxRight-1, chunkSize)
	} else if direction == "down" && (idxLeft-lastLeft) > chunkSize {
		section.Lines, err = getExcerptLines(commit, filePath, lastLeft, lastRight, chunkSize)
		lastLeft += chunkSize
		lastRight += chunkSize
	} else {
		offset := -1
		if direction == "down" {
			offset = 0
		}
		section.Lines, err = getExcerptLines(commit, filePath, lastLeft, lastRight, idxRight-lastRight+offset)
		leftHunkSize = 0
		rightHunkSize = 0
		idxLeft = lastLeft
		idxRight = lastRight
	}
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "getExcerptLines")
		return
	}
	if idxRight > lastRight {
		lineText := " "
		if rightHunkSize > 0 || leftHunkSize > 0 {
			lineText = fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", idxLeft, leftHunkSize, idxRight, rightHunkSize)
		}
		lineText = html.EscapeString(lineText)
		lineSection := &gitdiff.DiffLine{
			Type:    gitdiff.DiffLineSection,
			Content: lineText,
			SectionInfo: &gitdiff.DiffLineSectionInfo{
				Path:          filePath,
				LastLeftIdx:   lastLeft,
				LastRightIdx:  lastRight,
				LeftIdx:       idxLeft,
				RightIdx:      idxRight,
				LeftHunkSize:  leftHunkSize,
				RightHunkSize: rightHunkSize,
			}}
		if direction == "up" {
			section.Lines = append([]*gitdiff.DiffLine{lineSection}, section.Lines...)
		} else if direction == "down" {
			section.Lines = append(section.Lines, lineSection)
		}
	}
	ctx.Data["section"] = section
	ctx.Data["fileName"] = filePath
	ctx.Data["AfterCommitID"] = commitID
	ctx.Data["Anchor"] = anchor
	ctx.HTML(http.StatusOK, tplBlobExcerpt)
}

func getExcerptLines(commit *git.Commit, filePath string, idxLeft int, idxRight int, chunkSize int) ([]*gitdiff.DiffLine, error) {
	blob, err := commit.Tree.GetBlobByPath(filePath)
	if err != nil {
		return nil, err
	}
	reader, err := blob.DataAsync()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	scanner := bufio.NewScanner(reader)
	var diffLines []*gitdiff.DiffLine
	for line := 0; line < idxRight+chunkSize; line++ {
		if ok := scanner.Scan(); !ok {
			break
		}
		if line < idxRight {
			continue
		}
		lineText := scanner.Text()
		diffLine := &gitdiff.DiffLine{
			LeftIdx:  idxLeft + (line - idxRight) + 1,
			RightIdx: line + 1,
			Type:     gitdiff.DiffLinePlain,
			Content:  " " + lineText,
		}
		diffLines = append(diffLines, diffLine)
	}
	return diffLines, nil
}
