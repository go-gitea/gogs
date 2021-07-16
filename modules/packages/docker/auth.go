// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package docker

import (
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
)

// Authorized Permission check
func Authorized(doer *models.User, requestedAccess []ResourceActions) ([]ResourceActions, error) {
	rs := make([]ResourceActions, 0, len(requestedAccess))
	repos := make(map[string]*models.Repository)
	repoPerms := make(map[int64]models.Permission)

	for _, access := range requestedAccess {
		log.Debug("Authorized Name `%s` Type `%s` Action %v", access.Name, access.Type, access.Actions)

		if access.Type == "repository" {
			hasPush := false

			for _, action := range access.Actions {
				if action == "push" || action == "*" {
					hasPush = true
				}
			}
			if hasPush && doer == nil {
				continue
			}
			splits := strings.SplitN(access.Name, "/", 3)
			if len(splits) < 2 {
				continue
			}

			var err error
			owner, repoName := splits[0], splits[1]

			repo, has := repos[owner+"/"+repoName]
			if !has {
				repo, err = models.GetRepositoryByOwnerAndName(owner, repoName)
				if err != nil {
					if models.IsErrRepoNotExist(err) {
						continue
					}
					return nil, err
				}
				config := repo.MustGetUnit(models.UnitTypePackages).PackagesConfig()
				if !config.EnableContainerRegistry {
					continue
				}
				repos[owner+"/"+repoName] = repo
			}

			perm, has := repoPerms[repo.ID]
			if !has {
				perm, err = models.GetUserRepoPermission(repo, doer)
				if err != nil {
					return nil, err
				}
				repoPerms[repo.ID] = perm
			}

			accessMode := models.AccessModeRead
			if hasPush {
				accessMode = models.AccessModeWrite
			}
			if !perm.CanAccess(accessMode, models.UnitTypePackages) {
				continue
			}

		} else if access.Type == "registry" {
			if access.Name != "catalog" {
				log.Debug("Unknown registry resource: %s", access.Name)
				continue
			}
			if doer == nil || !doer.IsAdmin {
				continue
			}

		} else {
			log.Debug("Skipping unsupported resource type: %s", access.Type)
			continue
		}

		rs = append(rs, access)
	}

	return rs, nil
}
