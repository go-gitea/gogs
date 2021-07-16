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
	issue_service "code.gitea.io/gitea/services/issue"
)

// GetIssueAttachment gets a single attachment of the issue
func GetIssueAttachment(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/issues/assets/{attachment_id} issue issueGetIssueAttachment
	// ---
	// summary: Get a issue attachment
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

	attachID := ctx.ParamsInt64(":asset")
	attach, err := models.GetAttachmentByID(attachID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetAttachmentByID", err)
		return
	}
	if attach.IssueID == 0 {
		log.Debug("Requested attachment[%d] is not in an issue.", attachID)
		ctx.NotFound()
		return
	}
	issue, err := models.GetIssueByID(attach.IssueID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetIssueByID", err)
		return
	}
	if issue.RepoID != ctx.Repo.Repository.ID {
		log.Debug("Requested attachment[%d] belongs to issue[%d, #%d] which is not in Repo: %-v.", attachID, issue.ID, issue.Index, ctx.Repo.Repository)
		ctx.NotFound()
		return
	}
	unitType := models.UnitTypeIssues
	if issue.IsPull {
		unitType = models.UnitTypePullRequests
	}
	if !ctx.IsUserRepoReaderSpecific(unitType) && !ctx.IsUserRepoAdmin() && !ctx.IsUserSiteAdmin() {
		ctx.Error(http.StatusForbidden, "reqRepoReader", "user should have a permission to read repo")
		return
	}

	ctx.JSON(http.StatusOK, convert.ToIssueAttachment(attach))
}

// ListIssueAttachments lists all attachments of the issue
func ListIssueAttachments(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/issues/{index}/assets issue issueListIssueAttachments
	// ---
	// summary: List issue's attachments
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
	// - name: index
	//   in: path
	//   description: id of the issue
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/AttachmentList"

	issueID := ctx.ParamsInt64(":index")
	issue, err := models.GetIssueByID(issueID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetIssueByID", err)
		return
	}
	if issue.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound()
		return
	}
	if err := issue.LoadAttributes(); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadAttributes", err)
		return
	}
	ctx.JSON(http.StatusOK, convert.ToAPIIssue(issue).Attachments)
}

// CreateIssueAttachment creates an attachment and saves the given file
func CreateIssueAttachment(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/issues/{index}/assets issue issueCreateIssueAttachment
	// ---
	// summary: Create a issue attachment
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
	// - name: index
	//   in: path
	//   description: id of the issue
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

	// Check if issue exists an load issue
	issueID := ctx.ParamsInt64(":index")
	issue, err := models.GetIssueWithAttrsByID(issueID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetIssueByID", err)
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
	err = upload.Verify(buf, header.Filename, setting.Attachment.AllowedTypes)
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
		IssueID:    issue.ID,
	}, buf, file)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "NewAttachment", err)
		return
	}
	issue.Attachments = append(issue.Attachments, attach)

	if err := issue_service.ChangeContent(issue, ctx.User, issue.Content); err != nil {
		ctx.ServerError("ChangeContent", err)
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToIssueAttachment(attach))
}

func reqIssueAttachment(ctx *context.APIContext, attachID int64) *models.Attachment {
	attach, err := models.GetAttachmentByID(attachID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetAttachmentByID", err)
		return nil
	}
	if attach.IssueID == 0 {
		log.Debug("Requested attachment[%d] is not in an issue.", attachID)
		ctx.NotFound()
		return nil
	}
	issue, err := models.GetIssueByID(attach.IssueID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetIssueByID", err)
		return nil
	}
	if issue.RepoID != ctx.Repo.Repository.ID {
		log.Debug("Requested attachment[%d] belongs to issue[%d, #%d] which is not in Repo: %-v.", attachID, issue.ID, issue.Index, ctx.Repo.Repository)
		ctx.NotFound()
		return nil
	}
	unitType := models.UnitTypeIssues
	if issue.IsPull {
		unitType = models.UnitTypePullRequests
	}
	if !ctx.IsUserRepoWriter([]models.UnitType{unitType}) && !ctx.IsUserRepoAdmin() && !ctx.IsUserSiteAdmin() {
		ctx.Error(http.StatusForbidden, "reqRepoWriter", "user should have a permission to write to a repo")
		return nil
	}
	return attach
}

// EditIssueAttachment updates the given attachment
func EditIssueAttachment(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/issues/assets/{attachment_id} issue issueEditIssueAttachment
	// ---
	// summary: Edit a issue attachment
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

	attachID := ctx.ParamsInt64(":asset")
	attach := reqIssueAttachment(ctx, attachID)
	if attach == nil {
		return
	}
	if form.Name != "" {
		attach.Name = form.Name
	}

	if err := models.UpdateAttachment(attach); err != nil {
		ctx.Error(http.StatusInternalServerError, "UpdateAttachment", attach)
	}
	ctx.JSON(http.StatusCreated, convert.ToIssueAttachment(attach))
}

// DeleteIssueAttachment delete a given attachment
func DeleteIssueAttachment(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/issues/assets/{attachment_id} issue issueDeleteIssueAttachment
	// ---
	// summary: Delete a issue attachment
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

	attachID := ctx.ParamsInt64(":asset")
	attach := reqIssueAttachment(ctx, attachID)
	if attach == nil {
		return
	}
	if err := models.DeleteAttachment(attach, true); err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteAttachment", err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
