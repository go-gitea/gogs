// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/upload"
)

// UploadIssueAttachment response for Issue/PR attachments
func UploadIssueAttachment(ctx *context.Context) {
	uploadAttachment(ctx, setting.Attachment.AllowedTypes)
}

// UploadReleaseAttachment response for uploading release attachments
func UploadReleaseAttachment(ctx *context.Context) {
	uploadAttachment(ctx, setting.Repository.Release.AllowedTypes)
}

// UploadAttachment response for uploading attachments
func uploadAttachment(ctx *context.Context, allowedTypes string) {
	if !setting.Attachment.Enabled {
		ctx.Error(404, "attachment is not enabled")
		return
	}

	file, header, err := ctx.Req.FormFile("file")
	if err != nil {
		ctx.Error(500, fmt.Sprintf("FormFile: %v", err))
		return
	}
	defer file.Close()

	buf := make([]byte, 1024)
	n, _ := file.Read(buf)
	if n > 0 {
		buf = buf[:n]
	}

	err = upload.Verify(buf, header.Filename, allowedTypes)
	if err != nil {
		ctx.Error(400, err.Error())
		return
	}

	attach, err := models.NewAttachment(&models.Attachment{
		UploaderID: ctx.User.ID,
		Name:       header.Filename,
	}, buf, file)
	if err != nil {
		ctx.Error(500, fmt.Sprintf("NewAttachment: %v", err))
		return
	}

	log.Trace("New attachment uploaded: %s", attach.UUID)
	ctx.JSON(200, map[string]string{
		"uuid": attach.UUID,
	})
}

// DeleteAttachment response for deleting issue's attachment
func DeleteAttachment(ctx *context.Context) {
	file := ctx.Query("file")
	attach, err := models.GetAttachmentByUUID(file)
	if err != nil {
		ctx.Error(400, err.Error())
		return
	}
	if !ctx.IsSigned || (ctx.User.ID != attach.UploaderID) {
		ctx.Error(403)
		return
	}
	err = models.DeleteAttachment(attach, true)
	if err != nil {
		ctx.Error(500, fmt.Sprintf("DeleteAttachment: %v", err))
		return
	}
	ctx.JSON(200, map[string]string{
		"uuid": attach.UUID,
	})
}

// GetAttachment serve attachements
func GetAttachment(ctx *context.Context) {
	attach, err := models.GetAttachmentByUUID(ctx.Params(":uuid"))
	if err != nil {
		if models.IsErrAttachmentNotExist(err) {
			ctx.Error(404)
		} else {
			ctx.ServerError("GetAttachmentByUUID", err)
		}
		return
	}

	repository, unitType, err := attach.LinkedRepository()
	if err != nil {
		ctx.ServerError("LinkedRepository", err)
		return
	}

	if repository == nil { // If not linked
		if !(ctx.IsSigned && attach.UploaderID == ctx.User.ID) { // We block if not the uploader
			ctx.Error(http.StatusNotFound)
			return
		}
	} else { // If we have the repository we check access
		perm, err := models.GetUserRepoPermission(repository, ctx.User)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetUserRepoPermission", err.Error())
			return
		}
		if !perm.CanRead(unitType) {
			ctx.Error(http.StatusNotFound)
			return
		}
	}

	if setting.Attachment.ServeDirect {
		// If we have a signed url (S3, object storage), redirect to this directly.
		u, err := storage.Attachments.URL(attach.RelativePath(), attach.Name)

		if u != nil && err == nil {
			if err := attach.IncreaseDownloadCount(); err != nil {
				ctx.ServerError("Update", err)
				return
			}

			ctx.Redirect(u.String())
			return
		}
	}

	// If we have matched and access to release or issue
	fr, err := storage.Attachments.Open(attach.RelativePath())
	if err != nil {
		ctx.ServerError("Open", err)
		return
	}
	defer fr.Close()

	if err := attach.IncreaseDownloadCount(); err != nil {
		ctx.ServerError("Update", err)
		return
	}

	if err = ServeData(ctx, attach.Name, attach.Size, fr); err != nil {
		ctx.ServerError("ServeData", err)
		return
	}
}
