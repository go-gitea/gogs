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
	comment_service "code.gitea.io/gitea/services/comments"
)

// GetIssueCommentAttachment gets a single attachment of the comment
func GetIssueCommentAttachment(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/issues/comments/assets/{attachment_id} issue issueGetIssueCommentAttachment
	// ---
	// summary: Get a comment attachment
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
	if attach.CommentID == 0 {
		log.Info("User requested attachment is not in comment, attachment_id: %v", attachID)
		ctx.NotFound()
		return
	}

	ctx.JSON(http.StatusOK, convert.ToCommentAttachment(attach))
}

// ListIssueCommentAttachments lists all attachments of the comment
func ListIssueCommentAttachments(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/issues/comments/{id}/assets issue issueListIssueCommentAttachments
	// ---
	// summary: List comment's attachments
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
	//   description: id of the comment
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/AttachmentList"

	commentID := ctx.ParamsInt64(":id")
	comment, err := models.GetCommentByID(commentID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetCommentByID", err)
		return
	}

	if err := comment.LoadAttachments(); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadAttachments", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToComment(comment).Attachments)
}

// CreateIssueCommentAttachment creates an attachment and saves the given file
func CreateIssueCommentAttachment(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/issues/comments/{id}/assets issue issueCreateIssueCommentAttachment
	// ---
	// summary: Create a comment attachment
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
	//   description: id of the comment
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
	// Check if comment exists and load comment
	commentID := ctx.ParamsInt64(":id")
	comment, err := models.GetCommentByID(commentID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetCommentByID", err)
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
		CommentID:  comment.ID,
		IssueID:    comment.IssueID,
	}, buf, file)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "NewAttachment", err)
		return
	}
	if err := comment.LoadAttachments(); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadAttachments", err)
		return
	}

	if err = comment_service.UpdateComment(comment, ctx.User, comment.Content); err != nil {
		ctx.ServerError("UpdateComment", err)
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToCommentAttachment(attach))
}

// EditIssueCommentAttachment updates the given attachment
func EditIssueCommentAttachment(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/issues/comments/assets/{attachment_id} issue issueEditIssueCommentAttachment
	// ---
	// summary: Edit a comment attachment
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
	attach, err := models.GetAttachmentByID(attachID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetAttachmentByID", err)
		return
	}
	if attach.CommentID == 0 {
		log.Info("User requested attachment is not in comment, attachment_id: %v", attachID)
		ctx.NotFound()
		return
	}
	if form.Name != "" {
		attach.Name = form.Name
	}

	if err := models.UpdateAttachment(attach); err != nil {
		ctx.Error(http.StatusInternalServerError, "UpdateAttachment", attach)
	}
	ctx.JSON(http.StatusCreated, convert.ToCommentAttachment(attach))
}

// DeleteIssueCommentAttachment delete a given attachment
func DeleteIssueCommentAttachment(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/issues/comments/assets/{attachment_id} issue issueDeleteIssueCommentAttachment
	// ---
	// summary: Delete a comment attachment
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
	attach, err := models.GetAttachmentByID(attachID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetAttachmentByID", err)
		return
	}
	if attach.CommentID == 0 {
		log.Info("User requested attachment is not in comment, attachment_id: %v", attachID)
		ctx.NotFound()
		return
	}

	if err := models.DeleteAttachment(attach, true); err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteAttachment", err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
