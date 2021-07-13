// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package common

import (
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/httpcache"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/typesniffer"
)

// ServeBlob download a git.Blob
func ServeBlob(ctx *context.Context, blob *git.Blob) error {
	if httpcache.HandleGenericETagCache(ctx.Req, ctx.Resp, `"`+blob.ID.String()+`"`) {
		return nil
	}

	dataRc, err := blob.DataAsync()
	if err != nil {
		return err
	}
	defer func() {
		if err = dataRc.Close(); err != nil {
			log.Error("ServeBlob: Close: %v", err)
		}
	}()

	return ServeData(ctx, ctx.Repo.TreePath, blob.Size(), dataRc)
}

// ServeData download file from io.Reader
func ServeData(ctx *context.Context, name string, size int64, reader io.Reader) error {
	buf := make([]byte, 1024)
	n, err := reader.Read(buf)
	if err != nil && err != io.EOF {
		return err
	}
	if n >= 0 {
		buf = buf[:n]
	}

	ctx.Resp.Header().Set("Cache-Control", "public,max-age=86400")

	if size >= 0 {
		ctx.Resp.Header().Set("Content-Length", fmt.Sprintf("%d", size))
	} else {
		log.Error("ServeData called to serve data: %s with size < 0: %d", name, size)
	}
	name = path.Base(name)

	// Google Chrome dislike commas in filenames, so let's change it to a space
	name = strings.ReplaceAll(name, ",", " ")

	st := typesniffer.DetectContentType(buf)

	mappedMimeType := ""
	if setting.MimeTypeMap.Enabled {
		fileExtension := strings.ToLower(filepath.Ext(name))
		mappedMimeType = setting.MimeTypeMap.Map[fileExtension]
	}
	if st.IsText() || ctx.QueryBool("render") {
		cs, err := charset.DetectEncoding(buf)
		if err != nil {
			log.Error("Detect raw file %s charset failed: %v, using by default utf-8", name, err)
			cs = "utf-8"
		}
		if mappedMimeType == "" {
			mappedMimeType = "text/plain"
		}
		ctx.Resp.Header().Set("Content-Type", mappedMimeType+"; charset="+strings.ToLower(cs))
	} else {
		ctx.Resp.Header().Set("Access-Control-Expose-Headers", "Content-Disposition")
		if mappedMimeType != "" {
			ctx.Resp.Header().Set("Content-Type", mappedMimeType)
		}
		if (st.IsImage() || st.IsPDF()) && (setting.UI.SVG.Enabled || !st.IsSvgImage()) {
			ctx.Resp.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, name))
			if st.IsSvgImage() {
				ctx.Resp.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
				ctx.Resp.Header().Set("X-Content-Type-Options", "nosniff")
				ctx.Resp.Header().Set("Content-Type", typesniffer.SvgMimeType)
			}
		} else {
			ctx.Resp.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
		}
	}

	_, err = ctx.Resp.Write(buf)
	if err != nil {
		return err
	}
	_, err = io.Copy(ctx.Resp, reader)
	return err
}
