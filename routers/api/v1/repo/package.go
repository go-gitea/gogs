// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/packages/docker"
	"code.gitea.io/gitea/modules/setting"
)

// GetPackage get a single package of a repository
func GetPackage(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/packages/{type}/{name} repository repoGetPackage
	// ---
	// summary: Get a package
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
	// - name: type
	//   in: path
	//   description: type of package
	//   type: string
	//   required: true
	// - name: name
	//   in: path
	//   description: name of package
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Package"
	//   "404":
	//     "$ref": "#/responses/notFound"
	typ := ctx.Params("type")
	name := ctx.Params("name")
	if typ != models.PackageTypeDockerImage.String() || !setting.HasDockerPlugin() {
		ctx.NotFound()
		return
	}
	pkg, err := models.GetPackage(ctx.Repo.Repository.ID, models.PackageTypeDockerImage, name)
	if err != nil {
		if models.IsErrPackageNotExist(err) {
			ctx.NotFound()
		}
		ctx.InternalServerError(err)
		return
	}

	if err := pkg.LoadRepo(); err != nil {
		ctx.InternalServerError(err)
		return
	}

	if err := pkg.Repo.GetOwner(); err != nil {
		ctx.InternalServerError(err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToPackage(pkg))
}

// ListPackageVersions list versions of a package
func ListPackageVersions(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/packages/{type}/{name}/versions repository repoListPackageVersions
	// ---
	// summary: list versions of a package
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
	// - name: type
	//   in: path
	//   description: type of package
	//   type: string
	//   required: true
	// - name: name
	//   in: path
	//   description: name of package
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/PackageVersionList"
	//   "404":
	//     "$ref": "#/responses/notFound"
	typ := ctx.Params("type")
	name := ctx.Params("name")
	if typ != models.PackageTypeDockerImage.String() || !setting.HasDockerPlugin() {
		ctx.NotFound()
		return
	}
	pkg, err := models.GetPackage(ctx.Repo.Repository.ID, models.PackageTypeDockerImage, name)
	if err != nil {
		if models.IsErrPackageNotExist(err) {
			ctx.NotFound()
		}
		ctx.InternalServerError(err)
		return
	}

	if err := pkg.LoadRepo(); err != nil {
		ctx.InternalServerError(err)
		return
	}

	// generate token
	perms := []docker.AuthzResult{
		{
			Scope: docker.AuthScope{
				Type:    "repository",
				Class:   "",
				Name:    pkg.Repo.OwnerName + "/" + pkg.Repo.Name + "/" + pkg.Name,
				Actions: []string{"pull"},
			},
			AutorizedActions: []string{"pull"},
		},
	}
	account := ""
	if ctx.User != nil {
		account = ctx.User.Name
	}
	token, err := docker.GenerateToken(docker.GenerateTokenOptions{
		Account:      account,
		IssuerName:   setting.Docker.IssuerName,
		AuthzResults: perms,
		PublicKey:    &setting.Docker.PublicKey,
		PrivateKey:   &setting.Docker.PrivateKey,
		ServiceName:  setting.Docker.ServiceName,
		Expiration:   setting.Docker.Expiration,
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "generateToken", err)
		return
	}

	api := &docker.API{
		APIBasePath: setting.Docker.APIBasePath,
		Token:       token,
		TimeOut:     5 * time.Minute,
		Ctx:         ctx.Req.Context(),
	}

	rs, err := api.ListImageTags(pkg.Repo.OwnerName + "/" + pkg.Repo.Name + "/" + pkg.Name)
	if err != nil {
		ctx.Error(500, "docker.ListImageTags", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.DockerToVersionList(rs.Tags))
}
