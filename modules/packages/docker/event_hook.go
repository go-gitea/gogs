// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package docker

import (
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/timeutil"
	"github.com/docker/distribution/notifications"
)

// HandleEvents handle event listen
func HandleEvents(data *notifications.Envelope) error {
	repos := make(map[string]*models.Repository)
	pkgs := make(map[string]*models.Package)
	for _, event := range data.Events {
		if event.Action == notifications.EventActionPush {
			var (
				owner    string
				repoName string
				image    string
				err      error
			)
			splits := strings.SplitN(event.Target.Repository, "/", 3)
			if len(splits) != 3 {
				continue
			}
			owner = splits[0]
			repoName = splits[1]
			image = splits[2]

			repo, has := repos[owner+"/"+repoName]
			if !has {
				repo, err = models.GetRepositoryByOwnerAndName(owner, repoName)
				if err != nil {
					if models.IsErrRepoNotExist(err) {
						continue
					}
					return err
				}
				repos[owner+"/"+repoName] = repo
			}

			pkg, has := pkgs[event.Target.Repository]
			if !has {
				pkg, err = models.GetPackage(repo.ID, models.PackageTypeDockerImage, image)
				if err != nil {
					if models.IsErrPackageNotExist(err) {
						// create a new pkg
						err = models.AddPackage(models.AddPackageOptions{
							Repo: repo,
							Name: image,
							Type: models.PackageTypeDockerImage,
						})
						if err != nil {
							return err
						}
						continue
					}
					return err
				}
				if pkg != nil {
					pkgs[event.Target.Repository] = pkg
				}
			}

			// update update time
			err = pkg.UpdateLastUpdated(timeutil.TimeStamp(event.Timestamp.Unix()))
			if err != nil {
				return err
			}
		}
	}

	return nil
}
