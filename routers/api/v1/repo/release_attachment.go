// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/upload"
	"code.gitea.io/gitea/modules/web"
)

// GetReleaseAttachment gets a single attachment of the release
func GetReleaseAttachment(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/releases/assets/{attachment_id} repository repoGetReleaseAttachment
	// ---
	// summary: Get a release attachment
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
	// - name: attachment_id
	//   in: path
	//   description: id of the attachment to get
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Attachment"

	ownerName := ctx.Params(":username")
	repoName := ctx.Params(":reponame")
	attachID := ctx.ParamsInt64(":asset")
	attach, err := models.GetAttachmentByID(attachID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetAttachmentByID", err)
		return
	}
	repo, err := models.GetRepositoryByOwnerAndName(ownerName, repoName)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetRepositoryByOwnerAndName", err)
		return
	}
	repoLinked, _, err := attach.LinkedRepository()
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "LinkedRepository", err)
		return
	}
	if repo.ID != repoLinked.ID {
		log.Info("User requested attachment is not in repository, repository_id %v, release_id %v, attachment_id: %v", repo.ID, attach.ReleaseID, attachID)
		ctx.NotFound()
		return
	}
	if attach.ReleaseID == 0 {
		log.Info("User requested attachment is not in release, release_id %v, attachment_id: %v", attach.ReleaseID, attachID)
		ctx.NotFound()
		return
	}

	ctx.JSON(http.StatusOK, convert.ToReleaseAttachment(attach))
}

// GetReleaseAttachmentDeprecated gets a single attachment of the release
func GetReleaseAttachmentDeprecated(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/releases/{id}/assets/{attachment_id} repository repoGetReleaseAttachmentDeprecated
	// ---
	// summary: Get a release attachment
	// deprecated: true
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
	// - name: id
	//   in: path
	//   description: id of the release
	//   type: integer
	//   format: int64
	//   required: true
	// - name: attachment_id
	//   in: path
	//   description: id of the attachment to get
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Attachment"

	releaseID := ctx.ParamsInt64(":id")
	attachID := ctx.ParamsInt64(":asset")
	attach, err := models.GetAttachmentByID(attachID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetAttachmentByID", err)
		return
	}
	if attach.ReleaseID != releaseID {
		log.Info("User requested attachment is not in release, release_id %v, attachment_id: %v", releaseID, attachID)
		ctx.NotFound()
		return
	}
	// FIXME Should prove the existence of the given repo, but results in unnecessary database requests
	ctx.JSON(http.StatusOK, convert.ToReleaseAttachment(attach))
}

// ListReleaseAttachments lists all attachments of the release
func ListReleaseAttachments(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/releases/{id}/assets repository repoListReleaseAttachments
	// ---
	// summary: List release's attachments
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
	// - name: id
	//   in: path
	//   description: id of the release
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/AttachmentList"

	releaseID := ctx.ParamsInt64(":id")
	release, err := models.GetReleaseByID(releaseID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetReleaseByID", err)
		return
	}
	if release.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound()
		return
	}
	if err := release.LoadAttributes(); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadAttributes", err)
		return
	}
	ctx.JSON(http.StatusOK, convert.ToRelease(release).Attachments)
}

// CreateReleaseAttachment creates an attachment and saves the given file
func CreateReleaseAttachment(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/releases/{id}/assets repository repoCreateReleaseAttachment
	// ---
	// summary: Create a release attachment
	// produces:
	// - application/json
	// consumes:
	// - multipart/form-data
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
	// - name: id
	//   in: path
	//   description: id of the release
	//   type: integer
	//   format: int64
	//   required: true
	// - name: name
	//   in: query
	//   description: name of the attachment
	//   type: string
	//   required: false
	// - name: attachment
	//   in: formData
	//   description: attachment to upload
	//   type: file
	//   required: true
	// responses:
	//   "201":
	//     "$ref": "#/responses/Attachment"
	//   "400":
	//     "$ref": "#/responses/error"

	// Check if attachments are enabled
	if !setting.Attachment.Enabled {
		ctx.NotFound("Attachment is not enabled")
		return
	}

	// Check if release exists an load release
	releaseID := ctx.ParamsInt64(":id")
	release, err := models.GetReleaseByID(releaseID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetReleaseByID", err)
		return
	}

	// Get uploaded file from request
	file, header, err := ctx.Req.FormFile("attachment")
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetFile", err)
		return
	}
	defer file.Close()

	buf := make([]byte, 1024)
	n, _ := file.Read(buf)
	if n > 0 {
		buf = buf[:n]
	}

	// Check if the filetype is allowed by the settings
	err = upload.Verify(buf, header.Filename, setting.Repository.Release.AllowedTypes)
	if err != nil {
		ctx.Error(http.StatusBadRequest, "DetectContentType", err)
		return
	}

	var filename = header.Filename
	if query := ctx.Query("name"); query != "" {
		filename = query
	}

	// Create a new attachment and save the file
	attach, err := models.NewAttachment(&models.Attachment{
		UploaderID: ctx.User.ID,
		Name:       filename,
		ReleaseID:  release.ID,
	}, buf, file)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "NewAttachment", err)
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToReleaseAttachment(attach))
}

// EditReleaseAttachment updates the given attachment
func EditReleaseAttachment(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/releases/assets/{attachment_id} repository repoEditReleaseAttachment
	// ---
	// summary: Edit a release attachment
	// produces:
	// - application/json
	// consumes:
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
	// - name: attachment_id
	//   in: path
	//   description: id of the attachment to edit
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditAttachmentOptions"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Attachment"

	form := web.GetForm(ctx).(*api.EditAttachmentOptions)

	ownerName := ctx.Params(":username")
	repoName := ctx.Params(":reponame")
	attachID := ctx.ParamsInt64(":asset")
	attach, err := models.GetAttachmentByID(attachID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetAttachmentByID", err)
		return
	}
	repo, err := models.GetRepositoryByOwnerAndName(ownerName, repoName)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetRepositoryByOwnerAndName", err)
		return
	}
	repoLinked, _, err := attach.LinkedRepository()
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "LinkedRepository", err)
		return
	}
	if repo.ID != repoLinked.ID {
		log.Info("User requested attachment is not in repository, repository_id %v, release_id %v, attachment_id: %v", repo.ID, attach.ReleaseID, attachID)
		ctx.NotFound()
		return
	}
	if attach.ReleaseID == 0 {
		log.Info("User requested attachment is not in release, release_id %v, attachment_id: %v", attach.ReleaseID, attachID)
		ctx.NotFound()
		return
	}
	if form.Name != "" {
		attach.Name = form.Name
	}

	if err := models.UpdateAttachment(attach); err != nil {
		ctx.Error(http.StatusInternalServerError, "UpdateAttachment", attach)
	}
	ctx.JSON(http.StatusCreated, convert.ToReleaseAttachment(attach))
}

// EditReleaseAttachmentDeprecated updates the given attachment
func EditReleaseAttachmentDeprecated(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/releases/{id}/assets/{attachment_id} repository repoEditReleaseAttachmentDeprecated
	// ---
	// summary: Edit a release attachment
	// deprecated: true
	// produces:
	// - application/json
	// consumes:
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
	// - name: id
	//   in: path
	//   description: id of the release
	//   type: integer
	//   format: int64
	//   required: true
	// - name: attachment_id
	//   in: path
	//   description: id of the attachment to edit
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditAttachmentOptions"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Attachment"

	form := web.GetForm(ctx).(*api.EditAttachmentOptions)

	// Check if release exists an load release
	releaseID := ctx.ParamsInt64(":id")
	attachID := ctx.ParamsInt64(":asset")
	attach, err := models.GetAttachmentByID(attachID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetAttachmentByID", err)
		return
	}
	if attach.ReleaseID != releaseID {
		log.Info("User requested attachment is not in release, release_id %v, attachment_id: %v", releaseID, attachID)
		ctx.NotFound()
		return
	}
	// FIXME Should prove the existence of the given repo, but results in unnecessary database requests
	if form.Name != "" {
		attach.Name = form.Name
	}

	if err := models.UpdateAttachment(attach); err != nil {
		ctx.Error(http.StatusInternalServerError, "UpdateAttachment", attach)
	}
	ctx.JSON(http.StatusCreated, convert.ToReleaseAttachment(attach))
}

// DeleteReleaseAttachment delete a given attachment
func DeleteReleaseAttachment(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/releases/assets/{attachment_id} repository repoDeleteReleaseAttachment
	// ---
	// summary: Delete a release attachment
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
	// - name: attachment_id
	//   in: path
	//   description: id of the attachment to delete
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"

	ownerName := ctx.Params(":username")
	repoName := ctx.Params(":reponame")
	attachID := ctx.ParamsInt64(":asset")
	attach, err := models.GetAttachmentByID(attachID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetAttachmentByID", err)
		return
	}
	repo, err := models.GetRepositoryByOwnerAndName(ownerName, repoName)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetRepositoryByOwnerAndName", err)
		return
	}
	repoLinked, _, err := attach.LinkedRepository()
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "LinkedRepository", err)
		return
	}
	if repo.ID != repoLinked.ID {
		log.Info("User requested attachment is not in repository, repository_id %v, release_id %v, attachment_id: %v", repo.ID, attach.ReleaseID, attachID)
		ctx.NotFound()
		return
	}
	if attach.ReleaseID == 0 {
		log.Info("User requested attachment is not in release, release_id %v, attachment_id: %v", attach.ReleaseID, attachID)
		ctx.NotFound()
		return
	}

	if err := models.DeleteAttachment(attach, true); err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteAttachment", err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// DeleteReleaseAttachmentDeprecated delete a given attachment
func DeleteReleaseAttachmentDeprecated(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/releases/{id}/assets/{attachment_id} repository repoDeleteReleaseAttachmentDeprecated
	// ---
	// summary: Delete a release attachment
	// deprecated: true
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
	// - name: id
	//   in: path
	//   description: id of the release
	//   type: integer
	//   format: int64
	//   required: true
	// - name: attachment_id
	//   in: path
	//   description: id of the attachment to delete
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"

	// Check if release exists an load release
	releaseID := ctx.ParamsInt64(":id")
	attachID := ctx.ParamsInt64(":asset")
	attach, err := models.GetAttachmentByID(attachID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetAttachmentByID", err)
		return
	}
	if attach.ReleaseID != releaseID {
		log.Info("User requested attachment is not in release, release_id %v, attachment_id: %v", releaseID, attachID)
		ctx.NotFound()
		return
	}
	// FIXME Should prove the existence of the given repo, but results in unnecessary database requests

	if err := models.DeleteAttachment(attach, true); err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteAttachment", err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
