// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"
)

// ToPackage convert a models.Package to api.Package
func ToPackage(pkg *models.Package) *api.Package {
	rs := &api.Package{
		Name:    pkg.Name,
		Type:    pkg.Type.String(),
		Owner:   ToUser(pkg.Repo.Owner, nil),
		Repo:    ToRepo(pkg.Repo, models.AccessModeNone),
		Created: pkg.CreatedUnix.AsTimePtr(),
		Updated: pkg.UpdatedUnix.AsTimePtr(),
	}
	rs.Private = rs.Repo.Private
	return rs
}

// DockerToVersionList convert docker version list
func DockerToVersionList(vs []string) []*api.PackageVersion {
	rs := make([]*api.PackageVersion, 0, len(vs))
	for _, v := range vs {
		rs = append(rs, &api.PackageVersion{
			Name: v,
		})
	}
	return rs
}
