// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/url"
	"path"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"

	"github.com/editorconfig/editorconfig-core-go/v2"
	"github.com/unknwon/com"
)

// IssueTemplateDirCandidates issue templates directory
var IssueTemplateDirCandidates = []string{
	"ISSUE_TEMPLATE",
	"issue_template",
	".gitea/ISSUE_TEMPLATE",
	".gitea/issue_template",
	".github/ISSUE_TEMPLATE",
	".github/issue_template",
	".gitlab/ISSUE_TEMPLATE",
	".gitlab/issue_template",
}

// PullRequest contains information to make a pull request
type PullRequest struct {
	BaseRepo *models.Repository
	Allowed  bool
	SameRepo bool
	HeadInfo string // [<user>:]<branch>
}

// Repository contains information to operate a repository
type Repository struct {
	models.Permission
	IsWatching   bool
	IsViewBranch bool
	IsViewTag    bool
	IsViewCommit bool
	Repository   *models.Repository
	Owner        *models.User
	Commit       *git.Commit
	Tag          *git.Tag
	GitRepo      *git.Repository
	BranchName   string
	TagName      string
	TreePath     string
	CommitID     string
	RepoLink     string
	CloneLink    models.CloneLink
	CommitsCount int64
	Mirror       *models.Mirror

	PullRequest *PullRequest
}

// CanEnableEditor returns true if repository is editable and user has proper access level.
func (r *Repository) CanEnableEditor() bool {
	return r.Permission.CanWrite(models.UnitTypeCode) && r.Repository.CanEnableEditor() && r.IsViewBranch && !r.Repository.IsArchived
}

// CanCreateBranch returns true if repository is editable and user has proper access level.
func (r *Repository) CanCreateBranch() bool {
	return r.Permission.CanWrite(models.UnitTypeCode) && r.Repository.CanCreateBranch()
}

// RepoMustNotBeArchived checks if a repo is archived
func RepoMustNotBeArchived() func(ctx *Context) {
	return func(ctx *Context) {
		if ctx.Repo.Repository.IsArchived {
			ctx.NotFound("IsArchived", fmt.Errorf(ctx.Tr("repo.archive.title")))
		}
	}
}

// CanCommitToBranchResults represents the results of CanCommitToBranch
type CanCommitToBranchResults struct {
	CanCommitToBranch bool
	EditorEnabled     bool
	UserCanPush       bool
	RequireSigned     bool
	WillSign          bool
	SigningKey        string
	WontSignReason    string
}

// CanCommitToBranch returns true if repository is editable and user has proper access level
//   and branch is not protected for push
func (r *Repository) CanCommitToBranch(doer *models.User) (CanCommitToBranchResults, error) {
	protectedBranch, err := models.GetProtectedBranchBy(r.Repository.ID, r.BranchName)

	if err != nil {
		return CanCommitToBranchResults{}, err
	}
	userCanPush := true
	requireSigned := false
	if protectedBranch != nil {
		userCanPush = protectedBranch.CanUserPush(doer.ID)
		requireSigned = protectedBranch.RequireSignedCommits
	}

	sign, keyID, _, err := r.Repository.SignCRUDAction(doer, r.Repository.RepoPath(), git.BranchPrefix+r.BranchName)

	canCommit := r.CanEnableEditor() && userCanPush
	if requireSigned {
		canCommit = canCommit && sign
	}
	wontSignReason := ""
	if err != nil {
		if models.IsErrWontSign(err) {
			wontSignReason = string(err.(*models.ErrWontSign).Reason)
			err = nil
		} else {
			wontSignReason = "error"
		}
	}

	return CanCommitToBranchResults{
		CanCommitToBranch: canCommit,
		EditorEnabled:     r.CanEnableEditor(),
		UserCanPush:       userCanPush,
		RequireSigned:     requireSigned,
		WillSign:          sign,
		SigningKey:        keyID,
		WontSignReason:    wontSignReason,
	}, err
}

// CanUseTimetracker returns whether or not a user can use the timetracker.
func (r *Repository) CanUseTimetracker(issue *models.Issue, user *models.User) bool {
	// Checking for following:
	// 1. Is timetracker enabled
	// 2. Is the user a contributor, admin, poster or assignee and do the repository policies require this?
	isAssigned, _ := models.IsUserAssignedToIssue(issue, user)
	return r.Repository.IsTimetrackerEnabled() && (!r.Repository.AllowOnlyContributorsToTrackTime() ||
		r.Permission.CanWriteIssuesOrPulls(issue.IsPull) || issue.IsPoster(user.ID) || isAssigned)
}

// CanCreateIssueDependencies returns whether or not a user can create dependencies.
func (r *Repository) CanCreateIssueDependencies(user *models.User, isPull bool) bool {
	return r.Repository.IsDependenciesEnabled() && r.Permission.CanWriteIssuesOrPulls(isPull)
}

// GetCommitsCount returns cached commit count for current view
func (r *Repository) GetCommitsCount() (int64, error) {
	var contextName string
	if r.IsViewBranch {
		contextName = r.BranchName
	} else if r.IsViewTag {
		contextName = r.TagName
	} else {
		contextName = r.CommitID
	}
	return cache.GetInt64(r.Repository.GetCommitsCountCacheKey(contextName, r.IsViewBranch || r.IsViewTag), func() (int64, error) {
		return r.Commit.CommitsCount()
	})
}

// GetCommitGraphsCount returns cached commit count for current view
func (r *Repository) GetCommitGraphsCount(hidePRRefs bool, branches []string, files []string) (int64, error) {
	cacheKey := fmt.Sprintf("commits-count-%d-graph-%t-%s-%s", r.Repository.ID, hidePRRefs, branches, files)

	return cache.GetInt64(cacheKey, func() (int64, error) {
		if len(branches) == 0 {
			return git.AllCommitsCount(r.Repository.RepoPath(), hidePRRefs, files...)
		}
		return git.CommitsCountFiles(r.Repository.RepoPath(), branches, files)
	})
}

// BranchNameSubURL sub-URL for the BranchName field
func (r *Repository) BranchNameSubURL() string {
	switch {
	case r.IsViewBranch:
		return "branch/" + r.BranchName
	case r.IsViewTag:
		return "tag/" + r.BranchName
	case r.IsViewCommit:
		return "commit/" + r.BranchName
	}
	log.Error("Unknown view type for repo: %v", r)
	return ""
}

// FileExists returns true if a file exists in the given repo branch
func (r *Repository) FileExists(path string, branch string) (bool, error) {
	if branch == "" {
		branch = r.Repository.DefaultBranch
	}
	commit, err := r.GitRepo.GetBranchCommit(branch)
	if err != nil {
		return false, err
	}
	if _, err := commit.GetTreeEntryByPath(path); err != nil {
		return false, err
	}
	return true, nil
}

// GetEditorconfig returns the .editorconfig definition if found in the
// HEAD of the default repo branch.
func (r *Repository) GetEditorconfig() (*editorconfig.Editorconfig, error) {
	if r.GitRepo == nil {
		return nil, nil
	}
	commit, err := r.GitRepo.GetBranchCommit(r.Repository.DefaultBranch)
	if err != nil {
		return nil, err
	}
	treeEntry, err := commit.GetTreeEntryByPath(".editorconfig")
	if err != nil {
		return nil, err
	}
	if treeEntry.Blob().Size() >= setting.UI.MaxDisplayFileSize {
		return nil, git.ErrNotExist{ID: "", RelPath: ".editorconfig"}
	}
	reader, err := treeEntry.Blob().DataAsync()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return editorconfig.Parse(reader)
}

// RetrieveBaseRepo retrieves base repository
func RetrieveBaseRepo(ctx *Context, repo *models.Repository) {
	// Non-fork repository will not return error in this method.
	if err := repo.GetBaseRepo(); err != nil {
		if models.IsErrRepoNotExist(err) {
			repo.IsFork = false
			repo.ForkID = 0
			return
		}
		ctx.ServerError("GetBaseRepo", err)
		return
	} else if err = repo.BaseRepo.GetOwner(); err != nil {
		ctx.ServerError("BaseRepo.GetOwner", err)
		return
	}
}

// RetrieveTemplateRepo retrieves template repository used to generate this repository
func RetrieveTemplateRepo(ctx *Context, repo *models.Repository) {
	// Non-generated repository will not return error in this method.
	if err := repo.GetTemplateRepo(); err != nil {
		if models.IsErrRepoNotExist(err) {
			repo.TemplateID = 0
			return
		}
		ctx.ServerError("GetTemplateRepo", err)
		return
	} else if err = repo.TemplateRepo.GetOwner(); err != nil {
		ctx.ServerError("TemplateRepo.GetOwner", err)
		return
	}

	perm, err := models.GetUserRepoPermission(repo.TemplateRepo, ctx.User)
	if err != nil {
		ctx.ServerError("GetUserRepoPermission", err)
		return
	}

	if !perm.CanRead(models.UnitTypeCode) {
		repo.TemplateID = 0
	}
}

// ComposeGoGetImport returns go-get-import meta content.
func ComposeGoGetImport(owner, repo string) string {
	/// setting.AppUrl is guaranteed to be parse as url
	appURL, _ := url.Parse(setting.AppURL)

	return path.Join(appURL.Host, setting.AppSubURL, url.PathEscape(owner), url.PathEscape(repo))
}

// EarlyResponseForGoGetMeta responses appropriate go-get meta with status 200
// if user does not have actual access to the requested repository,
// or the owner or repository does not exist at all.
// This is particular a workaround for "go get" command which does not respect
// .netrc file.
func EarlyResponseForGoGetMeta(ctx *Context) {
	username := ctx.Params(":username")
	reponame := strings.TrimSuffix(ctx.Params(":reponame"), ".git")
	if username == "" || reponame == "" {
		ctx.PlainText(400, []byte("invalid repository path"))
		return
	}
	ctx.PlainText(200, []byte(com.Expand(`<meta name="go-import" content="{GoGetImport} git {CloneLink}">`,
		map[string]string{
			"GoGetImport": ComposeGoGetImport(username, reponame),
			"CloneLink":   models.ComposeHTTPSCloneURL(username, reponame),
		})))
}

// RedirectToRepo redirect to a differently-named repository
func RedirectToRepo(ctx *Context, redirectRepoID int64) {
	ownerName := ctx.Params(":username")
	previousRepoName := ctx.Params(":reponame")

	repo, err := models.GetRepositoryByID(redirectRepoID)
	if err != nil {
		ctx.ServerError("GetRepositoryByID", err)
		return
	}

	redirectPath := strings.Replace(
		ctx.Req.URL.Path,
		fmt.Sprintf("%s/%s", ownerName, previousRepoName),
		repo.FullName(),
		1,
	)
	if ctx.Req.URL.RawQuery != "" {
		redirectPath += "?" + ctx.Req.URL.RawQuery
	}
	ctx.Redirect(path.Join(setting.AppSubURL, redirectPath))
}

func repoAssignment(ctx *Context, repo *models.Repository) {
	var err error
	if err = repo.GetOwner(); err != nil {
		ctx.ServerError("GetOwner", err)
		return
	}

	ctx.Repo.Permission, err = models.GetUserRepoPermission(repo, ctx.User)
	if err != nil {
		ctx.ServerError("GetUserRepoPermission", err)
		return
	}

	// Check access.
	if ctx.Repo.Permission.AccessMode == models.AccessModeNone {
		if ctx.Query("go-get") == "1" {
			EarlyResponseForGoGetMeta(ctx)
			return
		}
		ctx.NotFound("no access right", nil)
		return
	}
	ctx.Data["HasAccess"] = true
	ctx.Data["Permission"] = &ctx.Repo.Permission

	if repo.IsMirror {
		var err error
		ctx.Repo.Mirror, err = models.GetMirrorByRepoID(repo.ID)
		if err != nil {
			ctx.ServerError("GetMirrorByRepoID", err)
			return
		}
		ctx.Data["MirrorEnablePrune"] = ctx.Repo.Mirror.EnablePrune
		ctx.Data["MirrorInterval"] = ctx.Repo.Mirror.Interval
		ctx.Data["Mirror"] = ctx.Repo.Mirror
	}
	if err = repo.LoadPushMirrors(); err != nil {
		ctx.ServerError("LoadPushMirrors", err)
		return
	}

	ctx.Repo.Repository = repo
	ctx.Data["RepoName"] = ctx.Repo.Repository.Name
	ctx.Data["IsEmptyRepo"] = ctx.Repo.Repository.IsEmpty
}

// RepoIDAssignment returns a handler which assigns the repo to the context.
func RepoIDAssignment() func(ctx *Context) {
	return func(ctx *Context) {
		repoID := ctx.ParamsInt64(":repoid")

		// Get repository.
		repo, err := models.GetRepositoryByID(repoID)
		if err != nil {
			if models.IsErrRepoNotExist(err) {
				ctx.NotFound("GetRepositoryByID", nil)
			} else {
				ctx.ServerError("GetRepositoryByID", err)
			}
			return
		}

		repoAssignment(ctx, repo)
	}
}

// RepoAssignment returns a middleware to handle repository assignment
func RepoAssignment(ctx *Context) (cancel context.CancelFunc) {
	var (
		owner *models.User
		err   error
	)

	userName := ctx.Params(":username")
	repoName := ctx.Params(":reponame")
	repoName = strings.TrimSuffix(repoName, ".git")

	// Check if the user is the same as the repository owner
	if ctx.IsSigned && ctx.User.LowerName == strings.ToLower(userName) {
		owner = ctx.User
	} else {
		owner, err = models.GetUserByName(userName)
		if err != nil {
			if models.IsErrUserNotExist(err) {
				if ctx.Query("go-get") == "1" {
					EarlyResponseForGoGetMeta(ctx)
					return
				}
				ctx.NotFound("GetUserByName", nil)
			} else {
				ctx.ServerError("GetUserByName", err)
			}
			return
		}
	}
	ctx.Repo.Owner = owner
	ctx.Data["Username"] = ctx.Repo.Owner.Name

	// Get repository.
	repo, err := models.GetRepositoryByName(owner.ID, repoName)
	if err != nil {
		if models.IsErrRepoNotExist(err) {
			redirectRepoID, err := models.LookupRepoRedirect(owner.ID, repoName)
			if err == nil {
				RedirectToRepo(ctx, redirectRepoID)
			} else if models.IsErrRepoRedirectNotExist(err) {
				if ctx.Query("go-get") == "1" {
					EarlyResponseForGoGetMeta(ctx)
					return
				}
				ctx.NotFound("GetRepositoryByName", nil)
			} else {
				ctx.ServerError("LookupRepoRedirect", err)
			}
		} else {
			ctx.ServerError("GetRepositoryByName", err)
		}
		return
	}
	repo.Owner = owner

	repoAssignment(ctx, repo)
	if ctx.Written() {
		return
	}

	ctx.Repo.RepoLink = repo.Link()
	ctx.Data["RepoLink"] = ctx.Repo.RepoLink
	ctx.Data["RepoRelPath"] = ctx.Repo.Owner.Name + "/" + ctx.Repo.Repository.Name

	unit, err := ctx.Repo.Repository.GetUnit(models.UnitTypeExternalTracker)
	if err == nil {
		ctx.Data["RepoExternalIssuesLink"] = unit.ExternalTrackerConfig().ExternalTrackerURL
	}

	ctx.Data["NumTags"], err = models.GetReleaseCountByRepoID(ctx.Repo.Repository.ID, models.FindReleasesOptions{
		IncludeTags: true,
	})
	if err != nil {
		ctx.ServerError("GetReleaseCountByRepoID", err)
		return
	}
	ctx.Data["NumReleases"], err = models.GetReleaseCountByRepoID(ctx.Repo.Repository.ID, models.FindReleasesOptions{})
	if err != nil {
		ctx.ServerError("GetReleaseCountByRepoID", err)
		return
	}

	ctx.Data["Title"] = owner.Name + "/" + repo.Name
	ctx.Data["Repository"] = repo
	ctx.Data["Owner"] = ctx.Repo.Repository.Owner
	ctx.Data["IsRepositoryOwner"] = ctx.Repo.IsOwner()
	ctx.Data["IsRepositoryAdmin"] = ctx.Repo.IsAdmin()
	ctx.Data["RepoOwnerIsOrganization"] = repo.Owner.IsOrganization()
	ctx.Data["CanWriteCode"] = ctx.Repo.CanWrite(models.UnitTypeCode)
	ctx.Data["CanWriteIssues"] = ctx.Repo.CanWrite(models.UnitTypeIssues)
	ctx.Data["CanWritePulls"] = ctx.Repo.CanWrite(models.UnitTypePullRequests)

	if ctx.Data["CanSignedUserFork"], err = ctx.Repo.Repository.CanUserFork(ctx.User); err != nil {
		ctx.ServerError("CanUserFork", err)
		return
	}

	ctx.Data["DisableSSH"] = setting.SSH.Disabled
	ctx.Data["ExposeAnonSSH"] = setting.SSH.ExposeAnonymous
	ctx.Data["DisableHTTP"] = setting.Repository.DisableHTTPGit
	ctx.Data["RepoSearchEnabled"] = setting.Indexer.RepoIndexerEnabled
	ctx.Data["CloneLink"] = repo.CloneLink()
	ctx.Data["WikiCloneLink"] = repo.WikiCloneLink()

	if ctx.IsSigned {
		ctx.Data["IsWatchingRepo"] = models.IsWatching(ctx.User.ID, repo.ID)
		ctx.Data["IsStaringRepo"] = models.IsStaring(ctx.User.ID, repo.ID)
	}

	if repo.IsFork {
		RetrieveBaseRepo(ctx, repo)
		if ctx.Written() {
			return
		}
	}

	if repo.IsGenerated() {
		RetrieveTemplateRepo(ctx, repo)
		if ctx.Written() {
			return
		}
	}

	// Disable everything when the repo is being created
	if ctx.Repo.Repository.IsBeingCreated() {
		ctx.Data["BranchName"] = ctx.Repo.Repository.DefaultBranch
		return
	}

	gitRepo, err := git.OpenRepository(models.RepoPath(userName, repoName))
	if err != nil {
		ctx.ServerError("RepoAssignment Invalid repo "+models.RepoPath(userName, repoName), err)
		return
	}
	ctx.Repo.GitRepo = gitRepo

	// We opened it, we should close it
	cancel = func() {
		// If it's been set to nil then assume someone else has closed it.
		if ctx.Repo.GitRepo != nil {
			ctx.Repo.GitRepo.Close()
		}
	}

	// Stop at this point when the repo is empty.
	if ctx.Repo.Repository.IsEmpty {
		ctx.Data["BranchName"] = ctx.Repo.Repository.DefaultBranch
		return
	}

	tags, err := ctx.Repo.GitRepo.GetTags()
	if err != nil {
		ctx.ServerError("GetTags", err)
		return
	}
	ctx.Data["Tags"] = tags

	brs, _, err := ctx.Repo.GitRepo.GetBranches(0, 0)
	if err != nil {
		ctx.ServerError("GetBranches", err)
		return
	}
	ctx.Data["Branches"] = brs
	ctx.Data["BranchesCount"] = len(brs)

	ctx.Data["TagName"] = ctx.Repo.TagName

	// If not branch selected, try default one.
	// If default branch doesn't exists, fall back to some other branch.
	if len(ctx.Repo.BranchName) == 0 {
		if len(ctx.Repo.Repository.DefaultBranch) > 0 && gitRepo.IsBranchExist(ctx.Repo.Repository.DefaultBranch) {
			ctx.Repo.BranchName = ctx.Repo.Repository.DefaultBranch
		} else if len(brs) > 0 {
			ctx.Repo.BranchName = brs[0]
		}
	}
	ctx.Data["BranchName"] = ctx.Repo.BranchName
	ctx.Data["CommitID"] = ctx.Repo.CommitID

	// People who have push access or have forked repository can propose a new pull request.
	canPush := ctx.Repo.CanWrite(models.UnitTypeCode) || (ctx.IsSigned && ctx.User.HasForkedRepo(ctx.Repo.Repository.ID))
	canCompare := false

	// Pull request is allowed if this is a fork repository
	// and base repository accepts pull requests.
	if repo.BaseRepo != nil && repo.BaseRepo.AllowsPulls() {
		canCompare = true
		ctx.Data["BaseRepo"] = repo.BaseRepo
		ctx.Repo.PullRequest.BaseRepo = repo.BaseRepo
		ctx.Repo.PullRequest.Allowed = canPush
		ctx.Repo.PullRequest.HeadInfo = ctx.Repo.Owner.Name + ":" + ctx.Repo.BranchName
	} else if repo.AllowsPulls() {
		// Or, this is repository accepts pull requests between branches.
		canCompare = true
		ctx.Data["BaseRepo"] = repo
		ctx.Repo.PullRequest.BaseRepo = repo
		ctx.Repo.PullRequest.Allowed = canPush
		ctx.Repo.PullRequest.SameRepo = true
		ctx.Repo.PullRequest.HeadInfo = ctx.Repo.BranchName
	}
	ctx.Data["CanCompareOrPull"] = canCompare
	ctx.Data["PullRequestCtx"] = ctx.Repo.PullRequest

	if ctx.Repo.Repository.Status == models.RepositoryPendingTransfer {
		repoTransfer, err := models.GetPendingRepositoryTransfer(ctx.Repo.Repository)
		if err != nil {
			ctx.ServerError("GetPendingRepositoryTransfer", err)
			return
		}

		if err := repoTransfer.LoadAttributes(); err != nil {
			ctx.ServerError("LoadRecipient", err)
			return
		}

		ctx.Data["RepoTransfer"] = repoTransfer
		if ctx.User != nil {
			ctx.Data["CanUserAcceptTransfer"] = repoTransfer.CanUserAcceptTransfer(ctx.User)
		}
	}

	if ctx.Query("go-get") == "1" {
		ctx.Data["GoGetImport"] = ComposeGoGetImport(owner.Name, repo.Name)
		prefix := setting.AppURL + path.Join(owner.Name, repo.Name, "src", "branch", ctx.Repo.BranchName)
		ctx.Data["GoDocDirectory"] = prefix + "{/dir}"
		ctx.Data["GoDocFile"] = prefix + "{/dir}/{file}#L{line}"
	}
	return
}

// RepoRefType type of repo reference
type RepoRefType int

const (
	// RepoRefLegacy unknown type, make educated guess and redirect.
	// for backward compatibility with previous URL scheme
	RepoRefLegacy RepoRefType = iota
	// RepoRefAny is for usage where educated guess is needed
	// but redirect can not be made
	RepoRefAny
	// RepoRefBranch branch
	RepoRefBranch
	// RepoRefTag tag
	RepoRefTag
	// RepoRefCommit commit
	RepoRefCommit
	// RepoRefBlob blob
	RepoRefBlob
)

// RepoRef handles repository reference names when the ref name is not
// explicitly given
func RepoRef() func(*Context) context.CancelFunc {
	// since no ref name is explicitly specified, ok to just use branch
	return RepoRefByType(RepoRefBranch)
}

// RefTypeIncludesBranches returns true if ref type can be a branch
func (rt RepoRefType) RefTypeIncludesBranches() bool {
	if rt == RepoRefLegacy || rt == RepoRefAny || rt == RepoRefBranch {
		return true
	}
	return false
}

// RefTypeIncludesTags returns true if ref type can be a tag
func (rt RepoRefType) RefTypeIncludesTags() bool {
	if rt == RepoRefLegacy || rt == RepoRefAny || rt == RepoRefTag {
		return true
	}
	return false
}

func getRefNameFromPath(ctx *Context, path string, isExist func(string) bool) string {
	refName := ""
	parts := strings.Split(path, "/")
	for i, part := range parts {
		refName = strings.TrimPrefix(refName+"/"+part, "/")
		if isExist(refName) {
			ctx.Repo.TreePath = strings.Join(parts[i+1:], "/")
			return refName
		}
	}
	return ""
}

func getRefName(ctx *Context, pathType RepoRefType) string {
	path := ctx.Params("*")
	switch pathType {
	case RepoRefLegacy, RepoRefAny:
		if refName := getRefName(ctx, RepoRefBranch); len(refName) > 0 {
			return refName
		}
		if refName := getRefName(ctx, RepoRefTag); len(refName) > 0 {
			return refName
		}
		// For legacy and API support only full commit sha
		parts := strings.Split(path, "/")
		if len(parts) > 0 && len(parts[0]) == 40 {
			ctx.Repo.TreePath = strings.Join(parts[1:], "/")
			return parts[0]
		}
		if refName := getRefName(ctx, RepoRefBlob); len(refName) > 0 {
			return refName
		}
		ctx.Repo.TreePath = path
		return ctx.Repo.Repository.DefaultBranch
	case RepoRefBranch:
		return getRefNameFromPath(ctx, path, ctx.Repo.GitRepo.IsBranchExist)
	case RepoRefTag:
		return getRefNameFromPath(ctx, path, ctx.Repo.GitRepo.IsTagExist)
	case RepoRefCommit:
		parts := strings.Split(path, "/")
		if len(parts) > 0 && len(parts[0]) >= 7 && len(parts[0]) <= 40 {
			ctx.Repo.TreePath = strings.Join(parts[1:], "/")
			return parts[0]
		}
	case RepoRefBlob:
		_, err := ctx.Repo.GitRepo.GetBlob(path)
		if err != nil {
			return ""
		}
		return path
	default:
		log.Error("Unrecognized path type: %v", path)
	}
	return ""
}

// RepoRefByType handles repository reference name for a specific type
// of repository reference
func RepoRefByType(refType RepoRefType, ignoreNotExistErr ...bool) func(*Context) context.CancelFunc {
	return func(ctx *Context) (cancel context.CancelFunc) {
		// Empty repository does not have reference information.
		if ctx.Repo.Repository.IsEmpty {
			return
		}

		var (
			refName string
			err     error
		)

		if ctx.Repo.GitRepo == nil {
			repoPath := models.RepoPath(ctx.Repo.Owner.Name, ctx.Repo.Repository.Name)
			ctx.Repo.GitRepo, err = git.OpenRepository(repoPath)
			if err != nil {
				ctx.ServerError("RepoRef Invalid repo "+repoPath, err)
				return
			}
			// We opened it, we should close it
			cancel = func() {
				// If it's been set to nil then assume someone else has closed it.
				if ctx.Repo.GitRepo != nil {
					ctx.Repo.GitRepo.Close()
				}
			}
		}

		// Get default branch.
		if len(ctx.Params("*")) == 0 {
			refName = ctx.Repo.Repository.DefaultBranch
			ctx.Repo.BranchName = refName
			if !ctx.Repo.GitRepo.IsBranchExist(refName) {
				brs, _, err := ctx.Repo.GitRepo.GetBranches(0, 0)
				if err != nil {
					ctx.ServerError("GetBranches", err)
					return
				} else if len(brs) == 0 {
					err = fmt.Errorf("No branches in non-empty repository %s",
						ctx.Repo.GitRepo.Path)
					ctx.ServerError("GetBranches", err)
					return
				}
				refName = brs[0]
			}
			ctx.Repo.Commit, err = ctx.Repo.GitRepo.GetBranchCommit(refName)
			if err != nil {
				ctx.ServerError("GetBranchCommit", err)
				return
			}
			ctx.Repo.CommitID = ctx.Repo.Commit.ID.String()
			ctx.Repo.IsViewBranch = true

		} else {
			refName = getRefName(ctx, refType)
			ctx.Repo.BranchName = refName
			if refType.RefTypeIncludesBranches() && ctx.Repo.GitRepo.IsBranchExist(refName) {
				ctx.Repo.IsViewBranch = true

				ctx.Repo.Commit, err = ctx.Repo.GitRepo.GetBranchCommit(refName)
				if err != nil {
					ctx.ServerError("GetBranchCommit", err)
					return
				}
				ctx.Repo.CommitID = ctx.Repo.Commit.ID.String()

			} else if refType.RefTypeIncludesTags() && ctx.Repo.GitRepo.IsTagExist(refName) {
				ctx.Repo.IsViewTag = true
				ctx.Repo.Commit, err = ctx.Repo.GitRepo.GetTagCommit(refName)
				if err != nil {
					ctx.ServerError("GetTagCommit", err)
					return
				}
				ctx.Repo.CommitID = ctx.Repo.Commit.ID.String()
			} else if len(refName) >= 7 && len(refName) <= 40 {
				ctx.Repo.IsViewCommit = true
				ctx.Repo.CommitID = refName

				ctx.Repo.Commit, err = ctx.Repo.GitRepo.GetCommit(refName)
				if err != nil {
					ctx.NotFound("GetCommit", err)
					return
				}
				// If short commit ID add canonical link header
				if len(refName) < 40 {
					ctx.Header().Set("Link", fmt.Sprintf("<%s>; rel=\"canonical\"",
						util.URLJoin(setting.AppURL, strings.Replace(ctx.Req.URL.RequestURI(), refName, ctx.Repo.Commit.ID.String(), 1))))
				}
			} else {
				if len(ignoreNotExistErr) > 0 && ignoreNotExistErr[0] {
					return
				}
				ctx.NotFound("RepoRef invalid repo", fmt.Errorf("branch or tag not exist: %s", refName))
				return
			}

			if refType == RepoRefLegacy {
				// redirect from old URL scheme to new URL scheme
				ctx.Redirect(path.Join(
					setting.AppSubURL,
					strings.TrimSuffix(ctx.Req.URL.Path, ctx.Params("*")),
					ctx.Repo.BranchNameSubURL(),
					ctx.Repo.TreePath))
				return
			}
		}

		ctx.Data["BranchName"] = ctx.Repo.BranchName
		ctx.Data["BranchNameSubURL"] = ctx.Repo.BranchNameSubURL()
		ctx.Data["CommitID"] = ctx.Repo.CommitID
		ctx.Data["TreePath"] = ctx.Repo.TreePath
		ctx.Data["IsViewBranch"] = ctx.Repo.IsViewBranch
		ctx.Data["IsViewTag"] = ctx.Repo.IsViewTag
		ctx.Data["IsViewCommit"] = ctx.Repo.IsViewCommit
		ctx.Data["CanCreateBranch"] = ctx.Repo.CanCreateBranch()

		ctx.Repo.CommitsCount, err = ctx.Repo.GetCommitsCount()
		if err != nil {
			ctx.ServerError("GetCommitsCount", err)
			return
		}
		ctx.Data["CommitsCount"] = ctx.Repo.CommitsCount
		return
	}
}

// GitHookService checks if repository Git hooks service has been enabled.
func GitHookService() func(ctx *Context) {
	return func(ctx *Context) {
		if !ctx.User.CanEditGitHook() {
			ctx.NotFound("GitHookService", nil)
			return
		}
	}
}

// UnitTypes returns a middleware to set unit types to context variables.
func UnitTypes() func(ctx *Context) {
	return func(ctx *Context) {
		ctx.Data["UnitTypeCode"] = models.UnitTypeCode
		ctx.Data["UnitTypeIssues"] = models.UnitTypeIssues
		ctx.Data["UnitTypePullRequests"] = models.UnitTypePullRequests
		ctx.Data["UnitTypeReleases"] = models.UnitTypeReleases
		ctx.Data["UnitTypeWiki"] = models.UnitTypeWiki
		ctx.Data["UnitTypeExternalWiki"] = models.UnitTypeExternalWiki
		ctx.Data["UnitTypeExternalTracker"] = models.UnitTypeExternalTracker
		ctx.Data["UnitTypeProjects"] = models.UnitTypeProjects
	}
}

// IssueTemplatesFromDefaultBranch checks for issue templates in the repo's default branch
func (ctx *Context) IssueTemplatesFromDefaultBranch() []api.IssueTemplate {
	var issueTemplates []api.IssueTemplate
	if ctx.Repo.Commit == nil {
		var err error
		ctx.Repo.Commit, err = ctx.Repo.GitRepo.GetBranchCommit(ctx.Repo.Repository.DefaultBranch)
		if err != nil {
			return issueTemplates
		}
	}

	for _, dirName := range IssueTemplateDirCandidates {
		tree, err := ctx.Repo.Commit.SubTree(dirName)
		if err != nil {
			continue
		}
		entries, err := tree.ListEntries()
		if err != nil {
			return issueTemplates
		}
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".md") {
				if entry.Blob().Size() >= setting.UI.MaxDisplayFileSize {
					log.Debug("Issue template is too large: %s", entry.Name())
					continue
				}
				r, err := entry.Blob().DataAsync()
				if err != nil {
					log.Debug("DataAsync: %v", err)
					continue
				}
				closed := false
				defer func() {
					if !closed {
						_ = r.Close()
					}
				}()
				data, err := ioutil.ReadAll(r)
				if err != nil {
					log.Debug("ReadAll: %v", err)
					continue
				}
				_ = r.Close()
				var it api.IssueTemplate
				content, err := markdown.ExtractMetadata(string(data), &it)
				if err != nil {
					log.Debug("ExtractMetadata: %v", err)
					continue
				}
				it.Content = content
				it.FileName = entry.Name()
				if it.Valid() {
					issueTemplates = append(issueTemplates, it)
				}
			}
		}
		if len(issueTemplates) > 0 {
			return issueTemplates
		}
	}
	return issueTemplates
}
