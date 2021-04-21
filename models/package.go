// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"strings"

	"code.gitea.io/gitea/modules/timeutil"
)

// PackageType type of package
type PackageType int64

const (
	// PackageTypeDockerImage a docker image (need docker_auth_port)
	PackageTypeDockerImage PackageType = iota
)

func (t PackageType) String() string {
	if t == PackageTypeDockerImage {
		return "docker"
	}
	return ""
}

// Package an image message tmp
type Package struct {
	ID        int64 `xorm:"pk autoincr"`
	Name      string
	LowerName string `xorm:"INDEX"`

	RepoID int64       `xorm:"INDEX"`
	Repo   *Repository `xorm:"-"`

	Type PackageType

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

// AddPackageOptions options for add package
type AddPackageOptions struct {
	Repo *Repository
	Name string
	Type PackageType
}

// AddPackage add a new package
func AddPackage(option AddPackageOptions) error {
	pkg := &Package{
		Name:   option.Name,
		Type:   option.Type,
		RepoID: option.Repo.ID,
	}
	pkg.LowerName = strings.ToLower(pkg.Name)
	_, err := x.Insert(pkg)
	return err
}

// GetPackage get a package
func GetPackage(repoID int64, typ PackageType, name string) (*Package, error) {
	return getPackage(x, repoID, typ, name)
}

func getPackage(e Engine, repoID int64, typ PackageType, name string) (*Package, error) {
	lowName := strings.ToLower(name)
	pkg := new(Package)
	has, err := e.Where("repo_id = ? AND type = ? AND lower_name = ?",
		repoID, typ, lowName).Get(pkg)

	if err != nil {
		return nil, err
	}

	if !has {
		return nil, ErrPackageNotExist{
			RepoID: repoID,
			Name:   name,
			Type:   typ,
		}
	}

	return pkg, nil
}

// updateCols updates some columns
func (pkg *Package) updateCols(cols ...string) error {
	_, err := x.ID(pkg.ID).Cols(cols...).Update(pkg)
	return err
}

// UpdateLastUpdated update last update time
func (pkg *Package) UpdateLastUpdated(updateTime timeutil.TimeStamp) error {
	pkg.UpdatedUnix = updateTime
	return pkg.updateCols("updated_unix")
}

// LoadRepo load package's repo
func (pkg *Package) LoadRepo(cols ...string) (err error) {
	if pkg.Repo != nil {
		return nil
	}

	pkg.Repo, err = GetRepositoryByID(pkg.RepoID)
	return err
}
