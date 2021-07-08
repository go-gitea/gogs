// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"net/http"
	"net/url"
	"path"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"github.com/unknwon/com"
)

func goGet(ctx *context.Context) {
	if ctx.Req.Method != "GET" || ctx.Query("go-get") != "1" || len(ctx.Req.URL.Query()) > 1 {
		return
	}

	parts := strings.SplitN(ctx.Req.URL.EscapedPath(), "/", 4)

	if len(parts) < 3 {
		return
	}

	ownerName := parts[1]
	repoName := parts[2]

	// Quick responses appropriate go-get meta with status 200
	// regardless of if user have access to the repository,
	// or the repository does not exist at all.
	// This is particular a workaround for "go get" command which does not respect
	// .netrc file.

	trimmedRepoName := strings.TrimSuffix(repoName, ".git")

	if ownerName == "" || trimmedRepoName == "" {
		_, _ = ctx.Write([]byte(`<!doctype html>
<html>
	<body>
		invalid import path
	</body>
</html>
`))
		ctx.Status(400)
		return
	}
	branchName := setting.Repository.DefaultBranch

	repo, err := models.GetRepositoryByOwnerAndName(ownerName, repoName)
	if err == nil && len(repo.DefaultBranch) > 0 {
		branchName = repo.DefaultBranch
	}
	prefix := setting.AppURL + path.Join(url.PathEscape(ownerName), url.PathEscape(repoName), "src", "branch", util.PathEscapeSegments(branchName))

	appURL, _ := url.Parse(setting.AppURL)

	insecure := ""
	if appURL.Scheme == string(setting.HTTP) {
		insecure = "--insecure "
	}
	ctx.Header().Set("Content-Type", "text/html")
	ctx.Status(http.StatusOK)
	_, _ = ctx.Write([]byte(com.Expand(`<!doctype html>
<html>
	<head>
		<meta name="go-import" content="{GoGetImport} git {CloneLink}">
		<meta name="go-source" content="{GoGetImport} _ {GoDocDirectory} {GoDocFile}">
	</head>
	<body>
		go get {Insecure}{GoGetImport}
	</body>
</html>
`, map[string]string{
		"GoGetImport":    context.ComposeGoGetImport(ctx, ownerName, trimmedRepoName),
		"CloneLink":      models.ComposeHTTPSCloneURL(ownerName, repoName),
		"GoDocDirectory": prefix + "{/dir}",
		"GoDocFile":      prefix + "{/dir}/{file}#L{line}",
		"Insecure":       insecure,
	})))
}
