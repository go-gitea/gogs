// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package upload

import (
	"net/http"
	"path"
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// ErrFileTypeForbidden not allowed file type error
type ErrFileTypeForbidden struct {
	Type string
}

// IsErrFileTypeForbidden checks if an error is a ErrFileTypeForbidden.
func IsErrFileTypeForbidden(err error) bool {
	_, ok := err.(ErrFileTypeForbidden)
	return ok
}

func (err ErrFileTypeForbidden) Error() string {
	return "This file extension or type is not allowed to be uploaded."
}

var (
	mimeTypeSuffixRe = regexp.MustCompile(`;.*$`)
	wildcardTypeRe   = regexp.MustCompile(`^[a-z]+/\*$`)
)

// Verify validates whether a file is allowed to be uploaded.
func Verify(buf []byte, fileName, allowedTypesStr string) error {
	allowedTypesStr = strings.ReplaceAll(allowedTypesStr, "|", ",") // compat for old config format

	allowedTypes := []string{}
	for _, entry := range strings.Split(allowedTypesStr, ",") {
		entry = strings.ToLower(strings.TrimSpace(entry))
		if entry != "" {
			allowedTypes = append(allowedTypes, entry)
		}
	}

	if len(allowedTypes) == 0 {
		return nil // everything is allowed
	}

	fullMimeType := http.DetectContentType(buf)
	mimeType := strings.TrimSpace(mimeTypeSuffixRe.ReplaceAllString(fullMimeType, ""))
	extension := strings.ToLower(path.Ext(fileName))

	// https://developer.mozilla.org/en-US/docs/Web/HTML/Element/input/file#Unique_file_type_specifiers
	for _, allowEntry := range allowedTypes {
		if allowEntry == "*/*" {
			return nil // everything allowed
		} else if strings.HasPrefix(allowEntry, ".") && allowEntry == extension {
			return nil // extension is allowed
		} else if mimeType == allowEntry {
			return nil // mime type is allowed
		} else if wildcardTypeRe.MatchString(allowEntry) && strings.HasPrefix(mimeType, allowEntry[:len(allowEntry)-1]) {
			return nil // wildcard match, e.g. image/*
		}
	}

	log.Info("Attachment with type %s blocked from upload", fullMimeType)
	return ErrFileTypeForbidden{Type: fullMimeType}
}

// AddUploadContext renders template values for dropzone
func AddUploadContext(ctx *context.Context, uploadType string) {
	if uploadType == "release" {
		ctx.Data["UploadUrl"] = ctx.Repo.RepoLink + "/releases/attachments"
		ctx.Data["UploadRemoveUrl"] = ctx.Repo.RepoLink + "/releases/attachments/remove"
		ctx.Data["UploadLinkUrl"] = ctx.Repo.RepoLink + "/releases/attachments"
		ctx.Data["UploadAccepts"] = strings.ReplaceAll(setting.Repository.Release.AllowedTypes, "|", ",")
		ctx.Data["UploadMaxFiles"] = setting.Attachment.MaxFiles
		ctx.Data["UploadMaxSize"] = setting.Attachment.MaxSize
	} else if uploadType == "comment" {
		ctx.Data["UploadUrl"] = ctx.Repo.RepoLink + "/issues/attachments"
		ctx.Data["UploadRemoveUrl"] = ctx.Repo.RepoLink + "/issues/attachments/remove"
		if len(ctx.Params(":index")) > 0 {
			ctx.Data["UploadLinkUrl"] = ctx.Repo.RepoLink + "/issues/" + ctx.Params(":index") + "/attachments"
		} else {
			ctx.Data["UploadLinkUrl"] = ctx.Repo.RepoLink + "/issues/attachments"
		}
		ctx.Data["UploadAccepts"] = strings.ReplaceAll(setting.Attachment.AllowedTypes, "|", ",")
		ctx.Data["UploadMaxFiles"] = setting.Attachment.MaxFiles
		ctx.Data["UploadMaxSize"] = setting.Attachment.MaxSize
	} else if uploadType == "repo" {
		ctx.Data["UploadUrl"] = ctx.Repo.RepoLink + "/upload-file"
		ctx.Data["UploadRemoveUrl"] = ctx.Repo.RepoLink + "/upload-remove"
		ctx.Data["UploadLinkUrl"] = ctx.Repo.RepoLink + "/upload-file"
		ctx.Data["UploadAccepts"] = strings.ReplaceAll(setting.Repository.Upload.AllowedTypes, "|", ",")
		ctx.Data["UploadMaxFiles"] = setting.Repository.Upload.MaxFiles
		ctx.Data["UploadMaxSize"] = setting.Repository.Upload.FileMaxSize
	}
}
