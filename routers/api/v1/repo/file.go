// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/repofiles"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/common"
	"code.gitea.io/gitea/routers/web/repo"
)

// GetRawFile get a file by path on a repository
func GetRawFile(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/raw/{filepath} repository repoGetRawFile
	// ---
	// summary: Get a file from a repository
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: filepath
	//   in: path
	//   description: filepath of the file to get
	//   type: string
	//   required: true
	// - name: ref
	//   in: query
	//   description: "The name of the commit/branch/tag. Default the repository’s default branch (usually master)"
	//   type: string
	//   required: false
	// responses:
	//   200:
	//     description: success
	//   "404":
	//     "$ref": "#/responses/notFound"

	if ctx.Repo.Repository.IsEmpty {
		ctx.NotFound()
		return
	}

	commit := ctx.Repo.Commit

	if ref := ctx.QueryTrim("ref"); len(ref) > 0 {
		var err error
		commit, err = ctx.Repo.GitRepo.GetCommit(ref)
		if err != nil {
			if git.IsErrNotExist(err) {
				ctx.NotFound()
			} else {
				ctx.Error(http.StatusInternalServerError, "GetBlobByPath", err)
			}
			return
		}
	}

	blob, err := commit.GetBlobByPath(ctx.Repo.TreePath)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetBlobByPath", err)
		}
		return
	}
	if err = common.ServeBlob(ctx.Context, blob); err != nil {
		ctx.Error(http.StatusInternalServerError, "ServeBlob", err)
	}
}

// GetArchive get archive of a repository
func GetArchive(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/archive/{archive} repository repoGetArchive
	// ---
	// summary: Get an archive of a repository
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: archive
	//   in: path
	//   description: the git reference for download with attached archive format (e.g. master.zip)
	//   type: string
	//   required: true
	// responses:
	//   200:
	//     description: success
	//   "404":
	//     "$ref": "#/responses/notFound"

	repoPath := models.RepoPath(ctx.Params(":username"), ctx.Params(":reponame"))
	gitRepo, err := git.OpenRepository(repoPath)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "OpenRepository", err)
		return
	}
	ctx.Repo.GitRepo = gitRepo
	defer gitRepo.Close()

	repo.Download(ctx.Context)
}

// GetEditorconfig get editor config of a repository
func GetEditorconfig(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/editorconfig/{filepath} repository repoGetEditorConfig
	// ---
	// summary: Get the EditorConfig definitions of a file in a repository
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: filepath
	//   in: path
	//   description: filepath of file to get
	//   type: string
	//   required: true
	// responses:
	//   200:
	//     description: success
	//   "404":
	//     "$ref": "#/responses/notFound"

	ec, err := ctx.Repo.GetEditorconfig()
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetEditorconfig", err)
		}
		return
	}

	fileName := ctx.Params("filename")
	def, err := ec.GetDefinitionForFilename(fileName)
	if def == nil {
		ctx.NotFound(err)
		return
	}
	ctx.JSON(http.StatusOK, def)
}

// canWriteFiles returns true if repository is editable and user has proper access level.
func canWriteFiles(r *context.Repository) bool {
	return r.Permission.CanWrite(models.UnitTypeCode) && !r.Repository.IsMirror && !r.Repository.IsArchived
}

// canReadFiles returns true if repository is readable and user has proper access level.
func canReadFiles(r *context.Repository) bool {
	return r.Permission.CanRead(models.UnitTypeCode)
}

// CreateFile handles API call for creating a file
func CreateFile(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/contents/{filepath} repository repoCreateFile
	// ---
	// summary: Create a file in a repository
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: filepath
	//   in: path
	//   description: path of the file to create
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/CreateFileOptions"
	// responses:
	//   "201":
	//     "$ref": "#/responses/FileResponse"
	//   "403":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/error"

	apiOpts := web.GetForm(ctx).(*api.CreateFileOptions)
	if ctx.Repo.Repository.IsEmpty {
		ctx.Error(http.StatusUnprocessableEntity, "RepoIsEmpty", fmt.Errorf("repo is empty"))
	}

	if apiOpts.BranchName == "" {
		apiOpts.BranchName = ctx.Repo.Repository.DefaultBranch
	}

	opts := &repofiles.UpdateRepoFileOptions{
		Content:   apiOpts.Content,
		IsNewFile: true,
		Message:   apiOpts.Message,
		TreePath:  ctx.Params("*"),
		OldBranch: apiOpts.BranchName,
		NewBranch: apiOpts.NewBranchName,
		Committer: &repofiles.IdentityOptions{
			Name:  apiOpts.Committer.Name,
			Email: apiOpts.Committer.Email,
		},
		Author: &repofiles.IdentityOptions{
			Name:  apiOpts.Author.Name,
			Email: apiOpts.Author.Email,
		},
		Dates: &repofiles.CommitDateOptions{
			Author:    apiOpts.Dates.Author,
			Committer: apiOpts.Dates.Committer,
		},
		Signoff: apiOpts.Signoff,
	}
	if opts.Dates.Author.IsZero() {
		opts.Dates.Author = time.Now()
	}
	if opts.Dates.Committer.IsZero() {
		opts.Dates.Committer = time.Now()
	}

	if opts.Message == "" {
		opts.Message = ctx.Tr("repo.editor.add", opts.TreePath)
	}

	if fileResponse, err := createOrUpdateFile(ctx, opts); err != nil {
		handleCreateOrUpdateFileError(ctx, err)
	} else {
		ctx.JSON(http.StatusCreated, fileResponse)
	}
}

// UpdateFile handles API call for updating a file
func UpdateFile(ctx *context.APIContext) {
	// swagger:operation PUT /repos/{owner}/{repo}/contents/{filepath} repository repoUpdateFile
	// ---
	// summary: Update a file in a repository
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: filepath
	//   in: path
	//   description: path of the file to update
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/UpdateFileOptions"
	// responses:
	//   "200":
	//     "$ref": "#/responses/FileResponse"
	//   "403":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/error"
	apiOpts := web.GetForm(ctx).(*api.UpdateFileOptions)
	if ctx.Repo.Repository.IsEmpty {
		ctx.Error(http.StatusUnprocessableEntity, "RepoIsEmpty", fmt.Errorf("repo is empty"))
	}

	if apiOpts.BranchName == "" {
		apiOpts.BranchName = ctx.Repo.Repository.DefaultBranch
	}

	opts := &repofiles.UpdateRepoFileOptions{
		Content:      apiOpts.Content,
		SHA:          apiOpts.SHA,
		IsNewFile:    false,
		Message:      apiOpts.Message,
		FromTreePath: apiOpts.FromPath,
		TreePath:     ctx.Params("*"),
		OldBranch:    apiOpts.BranchName,
		NewBranch:    apiOpts.NewBranchName,
		Committer: &repofiles.IdentityOptions{
			Name:  apiOpts.Committer.Name,
			Email: apiOpts.Committer.Email,
		},
		Author: &repofiles.IdentityOptions{
			Name:  apiOpts.Author.Name,
			Email: apiOpts.Author.Email,
		},
		Dates: &repofiles.CommitDateOptions{
			Author:    apiOpts.Dates.Author,
			Committer: apiOpts.Dates.Committer,
		},
		Signoff: apiOpts.Signoff,
	}
	if opts.Dates.Author.IsZero() {
		opts.Dates.Author = time.Now()
	}
	if opts.Dates.Committer.IsZero() {
		opts.Dates.Committer = time.Now()
	}

	if opts.Message == "" {
		opts.Message = ctx.Tr("repo.editor.update", opts.TreePath)
	}

	if fileResponse, err := createOrUpdateFile(ctx, opts); err != nil {
		handleCreateOrUpdateFileError(ctx, err)
	} else {
		ctx.JSON(http.StatusOK, fileResponse)
	}
}

func handleCreateOrUpdateFileError(ctx *context.APIContext, err error) {
	if models.IsErrUserCannotCommit(err) || models.IsErrFilePathProtected(err) {
		ctx.Error(http.StatusForbidden, "Access", err)
		return
	}
	if models.IsErrBranchAlreadyExists(err) || models.IsErrFilenameInvalid(err) || models.IsErrSHADoesNotMatch(err) ||
		models.IsErrFilePathInvalid(err) || models.IsErrRepoFileAlreadyExists(err) {
		ctx.Error(http.StatusUnprocessableEntity, "Invalid", err)
		return
	}
	if models.IsErrBranchDoesNotExist(err) || git.IsErrBranchNotExist(err) {
		ctx.Error(http.StatusNotFound, "BranchDoesNotExist", err)
		return
	}

	ctx.Error(http.StatusInternalServerError, "UpdateFile", err)
}

// Called from both CreateFile or UpdateFile to handle both
func createOrUpdateFile(ctx *context.APIContext, opts *repofiles.UpdateRepoFileOptions) (*api.FileResponse, error) {
	if !canWriteFiles(ctx.Repo) {
		return nil, models.ErrUserDoesNotHaveAccessToRepo{
			UserID:   ctx.User.ID,
			RepoName: ctx.Repo.Repository.LowerName,
		}
	}

	content, err := base64.StdEncoding.DecodeString(opts.Content)
	if err != nil {
		return nil, err
	}
	opts.Content = string(content)

	return repofiles.CreateOrUpdateRepoFile(ctx.Repo.Repository, ctx.User, opts)
}

// DeleteFile Delete a fle in a repository
func DeleteFile(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/contents/{filepath} repository repoDeleteFile
	// ---
	// summary: Delete a file in a repository
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: filepath
	//   in: path
	//   description: path of the file to delete
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/DeleteFileOptions"
	// responses:
	//   "200":
	//     "$ref": "#/responses/FileDeleteResponse"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/error"

	apiOpts := web.GetForm(ctx).(*api.DeleteFileOptions)
	if !canWriteFiles(ctx.Repo) {
		ctx.Error(http.StatusForbidden, "DeleteFile", models.ErrUserDoesNotHaveAccessToRepo{
			UserID:   ctx.User.ID,
			RepoName: ctx.Repo.Repository.LowerName,
		})
		return
	}

	if apiOpts.BranchName == "" {
		apiOpts.BranchName = ctx.Repo.Repository.DefaultBranch
	}

	opts := &repofiles.DeleteRepoFileOptions{
		Message:   apiOpts.Message,
		OldBranch: apiOpts.BranchName,
		NewBranch: apiOpts.NewBranchName,
		SHA:       apiOpts.SHA,
		TreePath:  ctx.Params("*"),
		Committer: &repofiles.IdentityOptions{
			Name:  apiOpts.Committer.Name,
			Email: apiOpts.Committer.Email,
		},
		Author: &repofiles.IdentityOptions{
			Name:  apiOpts.Author.Name,
			Email: apiOpts.Author.Email,
		},
		Dates: &repofiles.CommitDateOptions{
			Author:    apiOpts.Dates.Author,
			Committer: apiOpts.Dates.Committer,
		},
		Signoff: apiOpts.Signoff,
	}
	if opts.Dates.Author.IsZero() {
		opts.Dates.Author = time.Now()
	}
	if opts.Dates.Committer.IsZero() {
		opts.Dates.Committer = time.Now()
	}

	if opts.Message == "" {
		opts.Message = ctx.Tr("repo.editor.delete", opts.TreePath)
	}

	if fileResponse, err := repofiles.DeleteRepoFile(ctx.Repo.Repository, ctx.User, opts); err != nil {
		if git.IsErrBranchNotExist(err) || models.IsErrRepoFileDoesNotExist(err) || git.IsErrNotExist(err) {
			ctx.Error(http.StatusNotFound, "DeleteFile", err)
			return
		} else if models.IsErrBranchAlreadyExists(err) ||
			models.IsErrFilenameInvalid(err) ||
			models.IsErrSHADoesNotMatch(err) ||
			models.IsErrCommitIDDoesNotMatch(err) ||
			models.IsErrSHAOrCommitIDNotProvided(err) {
			ctx.Error(http.StatusBadRequest, "DeleteFile", err)
			return
		} else if models.IsErrUserCannotCommit(err) {
			ctx.Error(http.StatusForbidden, "DeleteFile", err)
			return
		}
		ctx.Error(http.StatusInternalServerError, "DeleteFile", err)
	} else {
		ctx.JSON(http.StatusOK, fileResponse) // FIXME on APIv2: return http.StatusNoContent
	}
}

// GetContents Get the metadata and contents (if a file) of an entry in a repository, or a list of entries if a dir
func GetContents(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/contents/{filepath} repository repoGetContents
	// ---
	// summary: Gets the metadata and contents (if a file) of an entry in a repository, or a list of entries if a dir
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: filepath
	//   in: path
	//   description: path of the dir, file, symlink or submodule in the repo
	//   type: string
	//   required: true
	// - name: ref
	//   in: query
	//   description: "The name of the commit/branch/tag. Default the repository’s default branch (usually master)"
	//   type: string
	//   required: false
	// responses:
	//   "200":
	//     "$ref": "#/responses/ContentsResponse"
	//   "404":
	//     "$ref": "#/responses/notFound"

	if !canReadFiles(ctx.Repo) {
		ctx.Error(http.StatusInternalServerError, "GetContentsOrList", models.ErrUserDoesNotHaveAccessToRepo{
			UserID:   ctx.User.ID,
			RepoName: ctx.Repo.Repository.LowerName,
		})
		return
	}

	treePath := ctx.Params("*")
	ref := ctx.QueryTrim("ref")

	if fileList, err := repofiles.GetContentsOrList(ctx.Repo.Repository, treePath, ref); err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound("GetContentsOrList", err)
			return
		}
		ctx.Error(http.StatusInternalServerError, "GetContentsOrList", err)
	} else {
		ctx.JSON(http.StatusOK, fileList)
	}
}

// GetContentsList Get the metadata of all the entries of the root dir
func GetContentsList(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/contents repository repoGetContentsList
	// ---
	// summary: Gets the metadata of all the entries of the root dir
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: ref
	//   in: query
	//   description: "The name of the commit/branch/tag. Default the repository’s default branch (usually master)"
	//   type: string
	//   required: false
	// responses:
	//   "200":
	//     "$ref": "#/responses/ContentsListResponse"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// same as GetContents(), this function is here because swagger fails if path is empty in GetContents() interface
	GetContents(ctx)
}
