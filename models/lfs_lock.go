// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"

	"xorm.io/xorm"
)

// LFSLock represents a git lfs lock of repository.
type LFSLock struct {
	ID      int64       `xorm:"pk autoincr"`
	Repo    *Repository `xorm:"-"`
	RepoID  int64       `xorm:"INDEX NOT NULL"`
	Owner   *User       `xorm:"-"`
	OwnerID int64       `xorm:"INDEX NOT NULL"`
	Path    string      `xorm:"TEXT"`
	Created time.Time   `xorm:"created"`
}

//TableName sets the table name to `lfs_lock`
func (l *LFSLock) TableName() string {
	return tbLFSLock[1 : len(tbLFSLock)-1]
}

// BeforeInsert is invoked from XORM before inserting an object of this type.
func (l *LFSLock) BeforeInsert() {
	l.OwnerID = l.Owner.ID
	l.RepoID = l.Repo.ID
	l.Path = cleanPath(l.Path)
}

// AfterLoad is invoked from XORM after setting the values of all fields of this object.
func (l *LFSLock) AfterLoad(session *xorm.Session) {
	var err error
	l.Owner, err = getUserByID(session, l.OwnerID)
	if err != nil {
		log.Error("LFS lock AfterLoad failed OwnerId[%d] not found: %v", l.OwnerID, err)
	}
	l.Repo, err = getRepositoryByID(session, l.RepoID)
	if err != nil {
		log.Error("LFS lock AfterLoad failed RepoId[%d] not found: %v", l.RepoID, err)
	}
}

func cleanPath(p string) string {
	return path.Clean("/" + p)[1:]
}

// APIFormat convert a Release to lfs.LFSLock
func (l *LFSLock) APIFormat() *api.LFSLock {
	return &api.LFSLock{
		ID:       strconv.FormatInt(l.ID, 10),
		Path:     l.Path,
		LockedAt: l.Created.Round(time.Second),
		Owner: &api.LFSLockOwner{
			Name: l.Owner.DisplayName(),
		},
	}
}

// CreateLFSLock creates a new lock.
func CreateLFSLock(lock *LFSLock) (*LFSLock, error) {
	err := CheckLFSAccessForRepo(lock.Owner, lock.Repo, AccessModeWrite)
	if err != nil {
		return nil, err
	}

	lock.Path = cleanPath(lock.Path)

	l, err := GetLFSLock(lock.Repo, lock.Path)
	if err == nil {
		return l, ErrLFSLockAlreadyExist{lock.RepoID, lock.Path}
	}
	if !IsErrLFSLockNotExist(err) {
		return nil, err
	}

	_, err = x.InsertOne(lock)
	return lock, err
}

// GetLFSLock returns release by given path.
func GetLFSLock(repo *Repository, path string) (*LFSLock, error) {
	path = cleanPath(path)
	rel := &LFSLock{RepoID: repo.ID}
	has, err := x.Where("lower(path) = ?", strings.ToLower(path)).Get(rel)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrLFSLockNotExist{0, repo.ID, path}
	}
	return rel, nil
}

// GetLFSLockByID returns release by given id.
func GetLFSLockByID(id int64) (*LFSLock, error) {
	lock := new(LFSLock)
	has, err := x.ID(id).Get(lock)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrLFSLockNotExist{id, 0, ""}
	}
	return lock, nil
}

// GetLFSLockByRepoID returns a list of locks of repository.
func GetLFSLockByRepoID(repoID int64, page, pageSize int) ([]*LFSLock, error) {
	sess := x.NewSession()
	defer sess.Close()

	if page >= 0 && pageSize > 0 {
		start := 0
		if page > 0 {
			start = (page - 1) * pageSize
		}
		sess.Limit(pageSize, start)
	}
	lfsLocks := make([]*LFSLock, 0, pageSize)
	return lfsLocks, sess.Find(&lfsLocks, &LFSLock{RepoID: repoID})
}

// CountLFSLockByRepoID returns a count of all LFSLocks associated with a repository.
func CountLFSLockByRepoID(repoID int64) (int64, error) {
	return x.Count(&LFSLock{RepoID: repoID})
}

// DeleteLFSLockByID deletes a lock by given ID.
func DeleteLFSLockByID(id int64, u *User, force bool) (*LFSLock, error) {
	lock, err := GetLFSLockByID(id)
	if err != nil {
		return nil, err
	}

	err = CheckLFSAccessForRepo(u, lock.Repo, AccessModeWrite)
	if err != nil {
		return nil, err
	}

	if !force && u.ID != lock.OwnerID {
		return nil, fmt.Errorf("user doesn't own lock and force flag is not set")
	}

	_, err = x.ID(id).Delete(new(LFSLock))
	return lock, err
}

//CheckLFSAccessForRepo check needed access mode base on action
func CheckLFSAccessForRepo(u *User, repo *Repository, mode AccessMode) error {
	if u == nil {
		return ErrLFSUnauthorizedAction{repo.ID, "undefined", mode}
	}
	perm, err := GetUserRepoPermission(repo, u)
	if err != nil {
		return err
	}
	if !perm.CanAccess(mode, UnitTypeCode) {
		return ErrLFSUnauthorizedAction{repo.ID, u.DisplayName(), mode}
	}
	return nil
}
