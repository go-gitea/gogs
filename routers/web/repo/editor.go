// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/repofiles"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/typesniffer"
	"code.gitea.io/gitea/modules/upload"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/utils"
	"code.gitea.io/gitea/services/forms"

	"github.com/goccy/go-json"
)

const (
	tplEditFile        base.TplName = "repo/editor/edit"
	tplEditDiffPreview base.TplName = "repo/editor/diff_preview"
	tplDeleteFile      base.TplName = "repo/editor/delete"
	tplUploadFile      base.TplName = "repo/editor/upload"

	frmCommitChoiceDirect    string = "direct"
	frmCommitChoiceNewBranch string = "commit-to-new-branch"
)

func renderCommitRights(ctx *context.Context) bool {
	canCommitToBranch, err := ctx.Repo.CanCommitToBranch(ctx.User)
	if err != nil {
		log.Error("CanCommitToBranch: %v", err)
	}
	ctx.Data["CanCommitToBranch"] = canCommitToBranch

	return canCommitToBranch.CanCommitToBranch
}

// getParentTreeFields returns list of parent tree names and corresponding tree paths
// based on given tree path.
func getParentTreeFields(treePath string) (treeNames []string, treePaths []string) {
	if len(treePath) == 0 {
		return treeNames, treePaths
	}

	treeNames = strings.Split(treePath, "/")
	treePaths = make([]string, len(treeNames))
	for i := range treeNames {
		treePaths[i] = strings.Join(treeNames[:i+1], "/")
	}
	return treeNames, treePaths
}

func editFile(ctx *context.Context, isNewFile bool) {
	ctx.Data["PageIsEdit"] = true
	ctx.Data["IsNewFile"] = isNewFile
	ctx.Data["RequireHighlightJS"] = true
	ctx.Data["RequireSimpleMDE"] = true
	canCommit := renderCommitRights(ctx)

	treePath := cleanUploadFileName(ctx.Repo.TreePath)
	if treePath != ctx.Repo.TreePath {
		if isNewFile {
			ctx.Redirect(path.Join(ctx.Repo.RepoLink, "_new", util.PathEscapeSegments(ctx.Repo.BranchName), util.PathEscapeSegments(treePath)))
		} else {
			ctx.Redirect(path.Join(ctx.Repo.RepoLink, "_edit", util.PathEscapeSegments(ctx.Repo.BranchName), util.PathEscapeSegments(treePath)))
		}
		return
	}

	treeNames, treePaths := getParentTreeFields(ctx.Repo.TreePath)

	if !isNewFile {
		entry, err := ctx.Repo.Commit.GetTreeEntryByPath(ctx.Repo.TreePath)
		if err != nil {
			ctx.NotFoundOrServerError("GetTreeEntryByPath", git.IsErrNotExist, err)
			return
		}

		// No way to edit a directory online.
		if entry.IsDir() {
			ctx.NotFound("entry.IsDir", nil)
			return
		}

		blob := entry.Blob()
		if blob.Size() >= setting.UI.MaxDisplayFileSize {
			ctx.NotFound("blob.Size", err)
			return
		}

		dataRc, err := blob.DataAsync()
		if err != nil {
			ctx.NotFound("blob.Data", err)
			return
		}

		defer dataRc.Close()

		ctx.Data["FileSize"] = blob.Size()
		ctx.Data["FileName"] = blob.Name()

		buf := make([]byte, 1024)
		n, _ := dataRc.Read(buf)
		buf = buf[:n]

		// Only some file types are editable online as text.
		if !typesniffer.DetectContentType(buf).IsRepresentableAsText() {
			ctx.NotFound("typesniffer.IsRepresentableAsText", nil)
			return
		}

		d, _ := ioutil.ReadAll(dataRc)
		if err := dataRc.Close(); err != nil {
			log.Error("Error whilst closing blob data: %v", err)
		}

		buf = append(buf, d...)
		if content, err := charset.ToUTF8WithErr(buf); err != nil {
			log.Error("ToUTF8WithErr: %v", err)
			ctx.Data["FileContent"] = string(buf)
		} else {
			ctx.Data["FileContent"] = content
		}
	} else {
		treeNames = append(treeNames, "") // Append empty string to allow user name the new file.
	}

	ctx.Data["TreeNames"] = treeNames
	ctx.Data["TreePaths"] = treePaths
	ctx.Data["BranchLink"] = ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL()
	ctx.Data["commit_summary"] = ""
	ctx.Data["commit_message"] = ""
	if canCommit {
		ctx.Data["commit_choice"] = frmCommitChoiceDirect
	} else {
		ctx.Data["commit_choice"] = frmCommitChoiceNewBranch
	}
	ctx.Data["new_branch_name"] = GetUniquePatchBranchName(ctx)
	ctx.Data["last_commit"] = ctx.Repo.CommitID
	ctx.Data["MarkdownFileExts"] = strings.Join(setting.Markdown.FileExtensions, ",")
	ctx.Data["LineWrapExtensions"] = strings.Join(setting.Repository.Editor.LineWrapExtensions, ",")
	ctx.Data["PreviewableFileModes"] = strings.Join(setting.Repository.Editor.PreviewableFileModes, ",")
	ctx.Data["Editorconfig"] = GetEditorConfig(ctx, treePath)

	ctx.HTML(http.StatusOK, tplEditFile)
}

// GetEditorConfig returns a editorconfig JSON string for given treePath or "null"
func GetEditorConfig(ctx *context.Context, treePath string) string {
	ec, err := ctx.Repo.GetEditorconfig()
	if err == nil {
		def, err := ec.GetDefinitionForFilename(treePath)
		if err == nil {
			jsonStr, _ := json.Marshal(def)
			return string(jsonStr)
		}
	}
	return "null"
}

// EditFile render edit file page
func EditFile(ctx *context.Context) {
	editFile(ctx, false)
}

// NewFile render create file page
func NewFile(ctx *context.Context) {
	editFile(ctx, true)
}

func editFilePost(ctx *context.Context, form forms.EditRepoFileForm, isNewFile bool) {
	canCommit := renderCommitRights(ctx)
	treeNames, treePaths := getParentTreeFields(form.TreePath)
	branchName := ctx.Repo.BranchName
	if form.CommitChoice == frmCommitChoiceNewBranch {
		branchName = form.NewBranchName
	}

	ctx.Data["PageIsEdit"] = true
	ctx.Data["PageHasPosted"] = true
	ctx.Data["IsNewFile"] = isNewFile
	ctx.Data["RequireHighlightJS"] = true
	ctx.Data["RequireSimpleMDE"] = true
	ctx.Data["TreePath"] = form.TreePath
	ctx.Data["TreeNames"] = treeNames
	ctx.Data["TreePaths"] = treePaths
	ctx.Data["BranchLink"] = ctx.Repo.RepoLink + "/src/branch/" + ctx.Repo.BranchName
	ctx.Data["FileContent"] = form.Content
	ctx.Data["commit_summary"] = form.CommitSummary
	ctx.Data["commit_message"] = form.CommitMessage
	ctx.Data["commit_choice"] = form.CommitChoice
	ctx.Data["new_branch_name"] = form.NewBranchName
	ctx.Data["last_commit"] = ctx.Repo.CommitID
	ctx.Data["MarkdownFileExts"] = strings.Join(setting.Markdown.FileExtensions, ",")
	ctx.Data["LineWrapExtensions"] = strings.Join(setting.Repository.Editor.LineWrapExtensions, ",")
	ctx.Data["PreviewableFileModes"] = strings.Join(setting.Repository.Editor.PreviewableFileModes, ",")
	ctx.Data["Editorconfig"] = GetEditorConfig(ctx, form.TreePath)

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplEditFile)
		return
	}

	// Cannot commit to a an existing branch if user doesn't have rights
	if branchName == ctx.Repo.BranchName && !canCommit {
		ctx.Data["Err_NewBranchName"] = true
		ctx.Data["commit_choice"] = frmCommitChoiceNewBranch
		ctx.RenderWithErr(ctx.Tr("repo.editor.cannot_commit_to_protected_branch", branchName), tplEditFile, &form)
		return
	}

	// CommitSummary is optional in the web form, if empty, give it a default message based on add or update
	// `message` will be both the summary and message combined
	message := strings.TrimSpace(form.CommitSummary)
	if len(message) == 0 {
		if isNewFile {
			message = ctx.Tr("repo.editor.add", form.TreePath)
		} else {
			message = ctx.Tr("repo.editor.update", form.TreePath)
		}
	}
	form.CommitMessage = strings.TrimSpace(form.CommitMessage)
	if len(form.CommitMessage) > 0 {
		message += "\n\n" + form.CommitMessage
	}

	if _, err := repofiles.CreateOrUpdateRepoFile(ctx.Repo.Repository, ctx.User, &repofiles.UpdateRepoFileOptions{
		LastCommitID: form.LastCommit,
		OldBranch:    ctx.Repo.BranchName,
		NewBranch:    branchName,
		FromTreePath: ctx.Repo.TreePath,
		TreePath:     form.TreePath,
		Message:      message,
		Content:      strings.ReplaceAll(form.Content, "\r", ""),
		IsNewFile:    isNewFile,
		Signoff:      form.Signoff,
	}); err != nil {
		// This is where we handle all the errors thrown by repofiles.CreateOrUpdateRepoFile
		if git.IsErrNotExist(err) {
			ctx.RenderWithErr(ctx.Tr("repo.editor.file_editing_no_longer_exists", ctx.Repo.TreePath), tplEditFile, &form)
		} else if models.IsErrLFSFileLocked(err) {
			ctx.Data["Err_TreePath"] = true
			ctx.RenderWithErr(ctx.Tr("repo.editor.upload_file_is_locked", err.(models.ErrLFSFileLocked).Path, err.(models.ErrLFSFileLocked).UserName), tplEditFile, &form)
		} else if models.IsErrFilenameInvalid(err) {
			ctx.Data["Err_TreePath"] = true
			ctx.RenderWithErr(ctx.Tr("repo.editor.filename_is_invalid", form.TreePath), tplEditFile, &form)
		} else if models.IsErrFilePathInvalid(err) {
			ctx.Data["Err_TreePath"] = true
			if fileErr, ok := err.(models.ErrFilePathInvalid); ok {
				switch fileErr.Type {
				case git.EntryModeSymlink:
					ctx.RenderWithErr(ctx.Tr("repo.editor.file_is_a_symlink", fileErr.Path), tplEditFile, &form)
				case git.EntryModeTree:
					ctx.RenderWithErr(ctx.Tr("repo.editor.filename_is_a_directory", fileErr.Path), tplEditFile, &form)
				case git.EntryModeBlob:
					ctx.RenderWithErr(ctx.Tr("repo.editor.directory_is_a_file", fileErr.Path), tplEditFile, &form)
				default:
					ctx.Error(http.StatusInternalServerError, err.Error())
				}
			} else {
				ctx.Error(http.StatusInternalServerError, err.Error())
			}
		} else if models.IsErrRepoFileAlreadyExists(err) {
			ctx.Data["Err_TreePath"] = true
			ctx.RenderWithErr(ctx.Tr("repo.editor.file_already_exists", form.TreePath), tplEditFile, &form)
		} else if git.IsErrBranchNotExist(err) {
			// For when a user adds/updates a file to a branch that no longer exists
			if branchErr, ok := err.(git.ErrBranchNotExist); ok {
				ctx.RenderWithErr(ctx.Tr("repo.editor.branch_does_not_exist", branchErr.Name), tplEditFile, &form)
			} else {
				ctx.Error(http.StatusInternalServerError, err.Error())
			}
		} else if models.IsErrBranchAlreadyExists(err) {
			// For when a user specifies a new branch that already exists
			ctx.Data["Err_NewBranchName"] = true
			if branchErr, ok := err.(models.ErrBranchAlreadyExists); ok {
				ctx.RenderWithErr(ctx.Tr("repo.editor.branch_already_exists", branchErr.BranchName), tplEditFile, &form)
			} else {
				ctx.Error(http.StatusInternalServerError, err.Error())
			}
		} else if models.IsErrCommitIDDoesNotMatch(err) {
			ctx.RenderWithErr(ctx.Tr("repo.editor.file_changed_while_editing", ctx.Repo.RepoLink+"/compare/"+form.LastCommit+"..."+ctx.Repo.CommitID), tplEditFile, &form)
		} else if git.IsErrPushOutOfDate(err) {
			ctx.RenderWithErr(ctx.Tr("repo.editor.file_changed_while_editing", ctx.Repo.RepoLink+"/compare/"+form.LastCommit+"..."+util.PathEscapeSegments(form.NewBranchName)), tplEditFile, &form)
		} else if git.IsErrPushRejected(err) {
			errPushRej := err.(*git.ErrPushRejected)
			if len(errPushRej.Message) == 0 {
				ctx.RenderWithErr(ctx.Tr("repo.editor.push_rejected_no_message"), tplEditFile, &form)
			} else {
				flashError, err := ctx.HTMLString(string(tplAlertDetails), map[string]interface{}{
					"Message": ctx.Tr("repo.editor.push_rejected"),
					"Summary": ctx.Tr("repo.editor.push_rejected_summary"),
					"Details": utils.SanitizeFlashErrorString(errPushRej.Message),
				})
				if err != nil {
					ctx.ServerError("editFilePost.HTMLString", err)
					return
				}
				ctx.RenderWithErr(flashError, tplEditFile, &form)
			}
		} else {
			flashError, err := ctx.HTMLString(string(tplAlertDetails), map[string]interface{}{
				"Message": ctx.Tr("repo.editor.fail_to_update_file", form.TreePath),
				"Summary": ctx.Tr("repo.editor.fail_to_update_file_summary"),
				"Details": utils.SanitizeFlashErrorString(err.Error()),
			})
			if err != nil {
				ctx.ServerError("editFilePost.HTMLString", err)
				return
			}
			ctx.RenderWithErr(flashError, tplEditFile, &form)
		}
	}

	if form.CommitChoice == frmCommitChoiceNewBranch && ctx.Repo.Repository.UnitEnabled(models.UnitTypePullRequests) {
		ctx.Redirect(ctx.Repo.RepoLink + "/compare/" + util.PathEscapeSegments(ctx.Repo.BranchName) + "..." + util.PathEscapeSegments(form.NewBranchName))
	} else {
		ctx.Redirect(ctx.Repo.RepoLink + "/src/branch/" + util.PathEscapeSegments(branchName) + "/" + util.PathEscapeSegments(form.TreePath))
	}
}

// EditFilePost response for editing file
func EditFilePost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.EditRepoFileForm)
	editFilePost(ctx, *form, false)
}

// NewFilePost response for creating file
func NewFilePost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.EditRepoFileForm)
	editFilePost(ctx, *form, true)
}

// DiffPreviewPost render preview diff page
func DiffPreviewPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.EditPreviewDiffForm)
	treePath := cleanUploadFileName(ctx.Repo.TreePath)
	if len(treePath) == 0 {
		ctx.Error(http.StatusInternalServerError, "file name to diff is invalid")
		return
	}

	entry, err := ctx.Repo.Commit.GetTreeEntryByPath(treePath)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetTreeEntryByPath: "+err.Error())
		return
	} else if entry.IsDir() {
		ctx.Error(http.StatusUnprocessableEntity)
		return
	}

	diff, err := repofiles.GetDiffPreview(ctx.Repo.Repository, ctx.Repo.BranchName, treePath, form.Content)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetDiffPreview: "+err.Error())
		return
	}

	if diff.NumFiles == 0 {
		ctx.PlainText(200, []byte(ctx.Tr("repo.editor.no_changes_to_show")))
		return
	}
	ctx.Data["File"] = diff.Files[0]

	ctx.HTML(http.StatusOK, tplEditDiffPreview)
}

// DeleteFile render delete file page
func DeleteFile(ctx *context.Context) {
	ctx.Data["PageIsDelete"] = true
	ctx.Data["BranchLink"] = ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL()
	treePath := cleanUploadFileName(ctx.Repo.TreePath)

	if treePath != ctx.Repo.TreePath {
		ctx.Redirect(path.Join(ctx.Repo.RepoLink, "_delete", util.PathEscapeSegments(ctx.Repo.BranchName), util.PathEscapeSegments(treePath)))
		return
	}

	ctx.Data["TreePath"] = treePath
	canCommit := renderCommitRights(ctx)

	ctx.Data["commit_summary"] = ""
	ctx.Data["commit_message"] = ""
	ctx.Data["last_commit"] = ctx.Repo.CommitID
	if canCommit {
		ctx.Data["commit_choice"] = frmCommitChoiceDirect
	} else {
		ctx.Data["commit_choice"] = frmCommitChoiceNewBranch
	}
	ctx.Data["new_branch_name"] = GetUniquePatchBranchName(ctx)

	ctx.HTML(http.StatusOK, tplDeleteFile)
}

// DeleteFilePost response for deleting file
func DeleteFilePost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.DeleteRepoFileForm)
	canCommit := renderCommitRights(ctx)
	branchName := ctx.Repo.BranchName
	if form.CommitChoice == frmCommitChoiceNewBranch {
		branchName = form.NewBranchName
	}

	ctx.Data["PageIsDelete"] = true
	ctx.Data["BranchLink"] = ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL()
	ctx.Data["TreePath"] = ctx.Repo.TreePath
	ctx.Data["commit_summary"] = form.CommitSummary
	ctx.Data["commit_message"] = form.CommitMessage
	ctx.Data["commit_choice"] = form.CommitChoice
	ctx.Data["new_branch_name"] = form.NewBranchName
	ctx.Data["last_commit"] = ctx.Repo.CommitID

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplDeleteFile)
		return
	}

	if branchName == ctx.Repo.BranchName && !canCommit {
		ctx.Data["Err_NewBranchName"] = true
		ctx.Data["commit_choice"] = frmCommitChoiceNewBranch
		ctx.RenderWithErr(ctx.Tr("repo.editor.cannot_commit_to_protected_branch", branchName), tplDeleteFile, &form)
		return
	}

	message := strings.TrimSpace(form.CommitSummary)
	if len(message) == 0 {
		message = ctx.Tr("repo.editor.delete", ctx.Repo.TreePath)
	}
	form.CommitMessage = strings.TrimSpace(form.CommitMessage)
	if len(form.CommitMessage) > 0 {
		message += "\n\n" + form.CommitMessage
	}

	if _, err := repofiles.DeleteRepoFile(ctx.Repo.Repository, ctx.User, &repofiles.DeleteRepoFileOptions{
		LastCommitID: form.LastCommit,
		OldBranch:    ctx.Repo.BranchName,
		NewBranch:    branchName,
		TreePath:     ctx.Repo.TreePath,
		Message:      message,
		Signoff:      form.Signoff,
	}); err != nil {
		// This is where we handle all the errors thrown by repofiles.DeleteRepoFile
		if git.IsErrNotExist(err) || models.IsErrRepoFileDoesNotExist(err) {
			ctx.RenderWithErr(ctx.Tr("repo.editor.file_deleting_no_longer_exists", ctx.Repo.TreePath), tplDeleteFile, &form)
		} else if models.IsErrFilenameInvalid(err) {
			ctx.Data["Err_TreePath"] = true
			ctx.RenderWithErr(ctx.Tr("repo.editor.filename_is_invalid", ctx.Repo.TreePath), tplDeleteFile, &form)
		} else if models.IsErrFilePathInvalid(err) {
			ctx.Data["Err_TreePath"] = true
			if fileErr, ok := err.(models.ErrFilePathInvalid); ok {
				switch fileErr.Type {
				case git.EntryModeSymlink:
					ctx.RenderWithErr(ctx.Tr("repo.editor.file_is_a_symlink", fileErr.Path), tplDeleteFile, &form)
				case git.EntryModeTree:
					ctx.RenderWithErr(ctx.Tr("repo.editor.filename_is_a_directory", fileErr.Path), tplDeleteFile, &form)
				case git.EntryModeBlob:
					ctx.RenderWithErr(ctx.Tr("repo.editor.directory_is_a_file", fileErr.Path), tplDeleteFile, &form)
				default:
					ctx.ServerError("DeleteRepoFile", err)
				}
			} else {
				ctx.ServerError("DeleteRepoFile", err)
			}
		} else if git.IsErrBranchNotExist(err) {
			// For when a user deletes a file to a branch that no longer exists
			if branchErr, ok := err.(git.ErrBranchNotExist); ok {
				ctx.RenderWithErr(ctx.Tr("repo.editor.branch_does_not_exist", branchErr.Name), tplDeleteFile, &form)
			} else {
				ctx.Error(http.StatusInternalServerError, err.Error())
			}
		} else if models.IsErrBranchAlreadyExists(err) {
			// For when a user specifies a new branch that already exists
			if branchErr, ok := err.(models.ErrBranchAlreadyExists); ok {
				ctx.RenderWithErr(ctx.Tr("repo.editor.branch_already_exists", branchErr.BranchName), tplDeleteFile, &form)
			} else {
				ctx.Error(http.StatusInternalServerError, err.Error())
			}
		} else if models.IsErrCommitIDDoesNotMatch(err) || git.IsErrPushOutOfDate(err) {
			ctx.RenderWithErr(ctx.Tr("repo.editor.file_changed_while_deleting", ctx.Repo.RepoLink+"/compare/"+form.LastCommit+"..."+ctx.Repo.CommitID), tplDeleteFile, &form)
		} else if git.IsErrPushRejected(err) {
			errPushRej := err.(*git.ErrPushRejected)
			if len(errPushRej.Message) == 0 {
				ctx.RenderWithErr(ctx.Tr("repo.editor.push_rejected_no_message"), tplDeleteFile, &form)
			} else {
				flashError, err := ctx.HTMLString(string(tplAlertDetails), map[string]interface{}{
					"Message": ctx.Tr("repo.editor.push_rejected"),
					"Summary": ctx.Tr("repo.editor.push_rejected_summary"),
					"Details": utils.SanitizeFlashErrorString(errPushRej.Message),
				})
				if err != nil {
					ctx.ServerError("DeleteFilePost.HTMLString", err)
					return
				}
				ctx.RenderWithErr(flashError, tplDeleteFile, &form)
			}
		} else {
			ctx.ServerError("DeleteRepoFile", err)
		}
	}

	ctx.Flash.Success(ctx.Tr("repo.editor.file_delete_success", ctx.Repo.TreePath))
	if form.CommitChoice == frmCommitChoiceNewBranch && ctx.Repo.Repository.UnitEnabled(models.UnitTypePullRequests) {
		ctx.Redirect(ctx.Repo.RepoLink + "/compare/" + util.PathEscapeSegments(ctx.Repo.BranchName) + "..." + util.PathEscapeSegments(form.NewBranchName))
	} else {
		treePath := path.Dir(ctx.Repo.TreePath)
		if treePath == "." {
			treePath = "" // the file deleted was in the root, so we return the user to the root directory
		}
		if len(treePath) > 0 {
			// Need to get the latest commit since it changed
			commit, err := ctx.Repo.GitRepo.GetBranchCommit(ctx.Repo.BranchName)
			if err == nil && commit != nil {
				// We have the comment, now find what directory we can return the user to
				// (must have entries)
				treePath = GetClosestParentWithFiles(treePath, commit)
			} else {
				treePath = "" // otherwise return them to the root of the repo
			}
		}
		ctx.Redirect(ctx.Repo.RepoLink + "/src/branch/" + util.PathEscapeSegments(branchName) + "/" + util.PathEscapeSegments(treePath))
	}
}

// UploadFile render upload file page
func UploadFile(ctx *context.Context) {
	ctx.Data["PageIsUpload"] = true
	ctx.Data["RequireTribute"] = true
	ctx.Data["RequireSimpleMDE"] = true
	upload.AddUploadContext(ctx, "repo")
	canCommit := renderCommitRights(ctx)
	treePath := cleanUploadFileName(ctx.Repo.TreePath)
	if treePath != ctx.Repo.TreePath {
		ctx.Redirect(path.Join(ctx.Repo.RepoLink, "_upload", util.PathEscapeSegments(ctx.Repo.BranchName), util.PathEscapeSegments(treePath)))
		return
	}
	ctx.Repo.TreePath = treePath

	treeNames, treePaths := getParentTreeFields(ctx.Repo.TreePath)
	if len(treeNames) == 0 {
		// We must at least have one element for user to input.
		treeNames = []string{""}
	}

	ctx.Data["TreeNames"] = treeNames
	ctx.Data["TreePaths"] = treePaths
	ctx.Data["BranchLink"] = ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL()
	ctx.Data["commit_summary"] = ""
	ctx.Data["commit_message"] = ""
	if canCommit {
		ctx.Data["commit_choice"] = frmCommitChoiceDirect
	} else {
		ctx.Data["commit_choice"] = frmCommitChoiceNewBranch
	}
	ctx.Data["new_branch_name"] = GetUniquePatchBranchName(ctx)

	ctx.HTML(http.StatusOK, tplUploadFile)
}

// UploadFilePost response for uploading file
func UploadFilePost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.UploadRepoFileForm)
	ctx.Data["PageIsUpload"] = true
	ctx.Data["RequireTribute"] = true
	ctx.Data["RequireSimpleMDE"] = true
	upload.AddUploadContext(ctx, "repo")
	canCommit := renderCommitRights(ctx)

	oldBranchName := ctx.Repo.BranchName
	branchName := oldBranchName

	if form.CommitChoice == frmCommitChoiceNewBranch {
		branchName = form.NewBranchName
	}

	form.TreePath = cleanUploadFileName(form.TreePath)

	treeNames, treePaths := getParentTreeFields(form.TreePath)
	if len(treeNames) == 0 {
		// We must at least have one element for user to input.
		treeNames = []string{""}
	}

	ctx.Data["TreePath"] = form.TreePath
	ctx.Data["TreeNames"] = treeNames
	ctx.Data["TreePaths"] = treePaths
	ctx.Data["BranchLink"] = ctx.Repo.RepoLink + "/src/branch/" + branchName
	ctx.Data["commit_summary"] = form.CommitSummary
	ctx.Data["commit_message"] = form.CommitMessage
	ctx.Data["commit_choice"] = form.CommitChoice
	ctx.Data["new_branch_name"] = branchName

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplUploadFile)
		return
	}

	if oldBranchName != branchName {
		if _, err := repo_module.GetBranch(ctx.Repo.Repository, branchName); err == nil {
			ctx.Data["Err_NewBranchName"] = true
			ctx.RenderWithErr(ctx.Tr("repo.editor.branch_already_exists", branchName), tplUploadFile, &form)
			return
		}
	} else if !canCommit {
		ctx.Data["Err_NewBranchName"] = true
		ctx.Data["commit_choice"] = frmCommitChoiceNewBranch
		ctx.RenderWithErr(ctx.Tr("repo.editor.cannot_commit_to_protected_branch", branchName), tplUploadFile, &form)
		return
	}

	var newTreePath string
	for _, part := range treeNames {
		newTreePath = path.Join(newTreePath, part)
		entry, err := ctx.Repo.Commit.GetTreeEntryByPath(newTreePath)
		if err != nil {
			if git.IsErrNotExist(err) {
				// Means there is no item with that name, so we're good
				break
			}

			ctx.ServerError("Repo.Commit.GetTreeEntryByPath", err)
			return
		}

		// User can only upload files to a directory.
		if !entry.IsDir() {
			ctx.Data["Err_TreePath"] = true
			ctx.RenderWithErr(ctx.Tr("repo.editor.directory_is_a_file", part), tplUploadFile, &form)
			return
		}
	}

	message := strings.TrimSpace(form.CommitSummary)
	if len(message) == 0 {
		message = ctx.Tr("repo.editor.upload_files_to_dir", form.TreePath)
	}

	form.CommitMessage = strings.TrimSpace(form.CommitMessage)
	if len(form.CommitMessage) > 0 {
		message += "\n\n" + form.CommitMessage
	}

	if err := repofiles.UploadRepoFiles(ctx.Repo.Repository, ctx.User, &repofiles.UploadRepoFileOptions{
		LastCommitID: ctx.Repo.CommitID,
		OldBranch:    oldBranchName,
		NewBranch:    branchName,
		TreePath:     form.TreePath,
		Message:      message,
		Files:        form.Files,
		Signoff:      form.Signoff,
	}); err != nil {
		if models.IsErrLFSFileLocked(err) {
			ctx.Data["Err_TreePath"] = true
			ctx.RenderWithErr(ctx.Tr("repo.editor.upload_file_is_locked", err.(models.ErrLFSFileLocked).Path, err.(models.ErrLFSFileLocked).UserName), tplUploadFile, &form)
		} else if models.IsErrFilenameInvalid(err) {
			ctx.Data["Err_TreePath"] = true
			ctx.RenderWithErr(ctx.Tr("repo.editor.filename_is_invalid", form.TreePath), tplUploadFile, &form)
		} else if models.IsErrFilePathInvalid(err) {
			ctx.Data["Err_TreePath"] = true
			fileErr := err.(models.ErrFilePathInvalid)
			switch fileErr.Type {
			case git.EntryModeSymlink:
				ctx.RenderWithErr(ctx.Tr("repo.editor.file_is_a_symlink", fileErr.Path), tplUploadFile, &form)
			case git.EntryModeTree:
				ctx.RenderWithErr(ctx.Tr("repo.editor.filename_is_a_directory", fileErr.Path), tplUploadFile, &form)
			case git.EntryModeBlob:
				ctx.RenderWithErr(ctx.Tr("repo.editor.directory_is_a_file", fileErr.Path), tplUploadFile, &form)
			default:
				ctx.Error(http.StatusInternalServerError, err.Error())
			}
		} else if models.IsErrRepoFileAlreadyExists(err) {
			ctx.Data["Err_TreePath"] = true
			ctx.RenderWithErr(ctx.Tr("repo.editor.file_already_exists", form.TreePath), tplUploadFile, &form)
		} else if git.IsErrBranchNotExist(err) {
			branchErr := err.(git.ErrBranchNotExist)
			ctx.RenderWithErr(ctx.Tr("repo.editor.branch_does_not_exist", branchErr.Name), tplUploadFile, &form)
		} else if models.IsErrBranchAlreadyExists(err) {
			// For when a user specifies a new branch that already exists
			ctx.Data["Err_NewBranchName"] = true
			branchErr := err.(models.ErrBranchAlreadyExists)
			ctx.RenderWithErr(ctx.Tr("repo.editor.branch_already_exists", branchErr.BranchName), tplUploadFile, &form)
		} else if git.IsErrPushOutOfDate(err) {
			ctx.RenderWithErr(ctx.Tr("repo.editor.file_changed_while_editing", ctx.Repo.RepoLink+"/compare/"+ctx.Repo.CommitID+"..."+util.PathEscapeSegments(form.NewBranchName)), tplUploadFile, &form)
		} else if git.IsErrPushRejected(err) {
			errPushRej := err.(*git.ErrPushRejected)
			if len(errPushRej.Message) == 0 {
				ctx.RenderWithErr(ctx.Tr("repo.editor.push_rejected_no_message"), tplUploadFile, &form)
			} else {
				flashError, err := ctx.HTMLString(string(tplAlertDetails), map[string]interface{}{
					"Message": ctx.Tr("repo.editor.push_rejected"),
					"Summary": ctx.Tr("repo.editor.push_rejected_summary"),
					"Details": utils.SanitizeFlashErrorString(errPushRej.Message),
				})
				if err != nil {
					ctx.ServerError("UploadFilePost.HTMLString", err)
					return
				}
				ctx.RenderWithErr(flashError, tplUploadFile, &form)
			}
		} else {
			// os.ErrNotExist - upload file missing in the intervening time?!
			log.Error("Error during upload to repo: %-v to filepath: %s on %s from %s: %v", ctx.Repo.Repository, form.TreePath, oldBranchName, form.NewBranchName, err)
			ctx.RenderWithErr(ctx.Tr("repo.editor.unable_to_upload_files", form.TreePath, err), tplUploadFile, &form)
		}
		return
	}

	if form.CommitChoice == frmCommitChoiceNewBranch && ctx.Repo.Repository.UnitEnabled(models.UnitTypePullRequests) {
		ctx.Redirect(ctx.Repo.RepoLink + "/compare/" + util.PathEscapeSegments(ctx.Repo.BranchName) + "..." + util.PathEscapeSegments(form.NewBranchName))
	} else {
		ctx.Redirect(ctx.Repo.RepoLink + "/src/branch/" + util.PathEscapeSegments(branchName) + "/" + util.PathEscapeSegments(form.TreePath))
	}
}

func cleanUploadFileName(name string) string {
	// Rebase the filename
	name = strings.Trim(path.Clean("/"+name), " /")
	// Git disallows any filenames to have a .git directory in them.
	for _, part := range strings.Split(name, "/") {
		if strings.ToLower(part) == ".git" {
			return ""
		}
	}
	return name
}

// UploadFileToServer upload file to server file dir not git
func UploadFileToServer(ctx *context.Context) {
	file, header, err := ctx.Req.FormFile("file")
	if err != nil {
		ctx.Error(http.StatusInternalServerError, fmt.Sprintf("FormFile: %v", err))
		return
	}
	defer file.Close()

	buf := make([]byte, 1024)
	n, _ := file.Read(buf)
	if n > 0 {
		buf = buf[:n]
	}

	err = upload.Verify(buf, header.Filename, setting.Repository.Upload.AllowedTypes)
	if err != nil {
		ctx.Error(http.StatusBadRequest, err.Error())
		return
	}

	name := cleanUploadFileName(header.Filename)
	if len(name) == 0 {
		ctx.Error(http.StatusInternalServerError, "Upload file name is invalid")
		return
	}

	upload, err := models.NewUpload(name, buf, file)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, fmt.Sprintf("NewUpload: %v", err))
		return
	}

	log.Trace("New file uploaded: %s", upload.UUID)
	ctx.JSON(http.StatusOK, map[string]string{
		"uuid": upload.UUID,
	})
}

// RemoveUploadFileFromServer remove file from server file dir
func RemoveUploadFileFromServer(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.RemoveUploadFileForm)
	if len(form.File) == 0 {
		ctx.Status(204)
		return
	}

	if err := models.DeleteUploadByUUID(form.File); err != nil {
		ctx.Error(http.StatusInternalServerError, fmt.Sprintf("DeleteUploadByUUID: %v", err))
		return
	}

	log.Trace("Upload file removed: %s", form.File)
	ctx.Status(204)
}

// GetUniquePatchBranchName Gets a unique branch name for a new patch branch
// It will be in the form of <username>-patch-<num> where <num> is the first branch of this format
// that doesn't already exist. If we exceed 1000 tries or an error is thrown, we just return "" so the user has to
// type in the branch name themselves (will be an empty field)
func GetUniquePatchBranchName(ctx *context.Context) string {
	prefix := ctx.User.LowerName + "-patch-"
	for i := 1; i <= 1000; i++ {
		branchName := fmt.Sprintf("%s%d", prefix, i)
		if _, err := repo_module.GetBranch(ctx.Repo.Repository, branchName); err != nil {
			if git.IsErrBranchNotExist(err) {
				return branchName
			}
			log.Error("GetUniquePatchBranchName: %v", err)
			return ""
		}
	}
	return ""
}

// GetClosestParentWithFiles Recursively gets the path of parent in a tree that has files (used when file in a tree is
// deleted). Returns "" for the root if no parents other than the root have files. If the given treePath isn't a
// SubTree or it has no entries, we go up one dir and see if we can return the user to that listing.
func GetClosestParentWithFiles(treePath string, commit *git.Commit) string {
	if len(treePath) == 0 || treePath == "." {
		return ""
	}
	// see if the tree has entries
	if tree, err := commit.SubTree(treePath); err != nil {
		// failed to get tree, going up a dir
		return GetClosestParentWithFiles(path.Dir(treePath), commit)
	} else if entries, err := tree.ListEntries(); err != nil || len(entries) == 0 {
		// no files in this dir, going up a dir
		return GetClosestParentWithFiles(path.Dir(treePath), commit)
	}
	return treePath
}
