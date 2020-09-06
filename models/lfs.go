// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
)

// LFSMetaObject stores metadata for LFS tracked files.
type LFSMetaObject struct {
	ID           int64              `xorm:"pk autoincr"`
	Oid          string             `xorm:"UNIQUE(s) INDEX NOT NULL"`
	Size         int64              `xorm:"NOT NULL"`
	RepositoryID int64              `xorm:"UNIQUE(s) INDEX NOT NULL"`
	Existing     bool               `xorm:"-"`
	CreatedUnix  timeutil.TimeStamp `xorm:"created"`
}

// TableName sets the table name to `lfs_meta_object`
func (m *LFSMetaObject) TableName() string {
	return tbLfsMetaObject[1 : len(tbLfsMetaObject)-1]
}

// Pointer returns the string representation of an LFS pointer file
func (m *LFSMetaObject) Pointer() string {
	return fmt.Sprintf("%s\n%s%s\nsize %d\n", LFSMetaFileIdentifier, LFSMetaFileOidPrefix, m.Oid, m.Size)
}

// LFSTokenResponse defines the JSON structure in which the JWT token is stored.
// This structure is fetched via SSH and passed by the Git LFS client to the server
// endpoint for authorization.
type LFSTokenResponse struct {
	Header map[string]string `json:"header"`
	Href   string            `json:"href"`
}

var (
	// ErrLFSObjectNotExist is returned from lfs models functions in order
	// to differentiate between database and missing object errors.
	ErrLFSObjectNotExist = errors.New("LFS Meta object does not exist")
)

const (
	// LFSMetaFileIdentifier is the string appearing at the first line of LFS pointer files.
	// https://github.com/git-lfs/git-lfs/blob/master/docs/spec.md
	LFSMetaFileIdentifier = "version https://git-lfs.github.com/spec/v1"

	// LFSMetaFileOidPrefix appears in LFS pointer files on a line before the sha256 hash.
	LFSMetaFileOidPrefix = "oid sha256:"
)

// NewLFSMetaObject stores a given populated LFSMetaObject structure in the database
// if it is not already present.
func NewLFSMetaObject(m *LFSMetaObject) (*LFSMetaObject, error) {
	var err error

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return nil, err
	}

	has, err := sess.Get(m)
	if err != nil {
		return nil, err
	}

	if has {
		m.Existing = true
		return m, sess.Commit()
	}

	if _, err = sess.Insert(m); err != nil {
		return nil, err
	}

	return m, sess.Commit()
}

// GenerateLFSOid generates a Sha256Sum to represent an oid for arbitrary content
func GenerateLFSOid(content io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, content); err != nil {
		return "", err
	}
	sum := h.Sum(nil)
	return hex.EncodeToString(sum), nil
}

// GetLFSMetaObjectByOid selects a LFSMetaObject entry from database by its OID.
// It may return ErrLFSObjectNotExist or a database error. If the error is nil,
// the returned pointer is a valid LFSMetaObject.
func (repo *Repository) GetLFSMetaObjectByOid(oid string) (*LFSMetaObject, error) {
	if len(oid) == 0 {
		return nil, ErrLFSObjectNotExist
	}

	m := &LFSMetaObject{Oid: oid, RepositoryID: repo.ID}
	has, err := x.Get(m)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrLFSObjectNotExist
	}
	return m, nil
}

// RemoveLFSMetaObjectByOid removes a LFSMetaObject entry from database by its OID.
// It may return ErrLFSObjectNotExist or a database error.
func (repo *Repository) RemoveLFSMetaObjectByOid(oid string) (int64, error) {
	if len(oid) == 0 {
		return 0, ErrLFSObjectNotExist
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return -1, err
	}

	m := &LFSMetaObject{Oid: oid, RepositoryID: repo.ID}
	if _, err := sess.Delete(m); err != nil {
		return -1, err
	}

	count, err := sess.Count(&LFSMetaObject{Oid: oid})
	if err != nil {
		return count, err
	}

	return count, sess.Commit()
}

// GetLFSMetaObjects returns all LFSMetaObjects associated with a repository
func (repo *Repository) GetLFSMetaObjects(page, pageSize int) ([]*LFSMetaObject, error) {
	sess := x.NewSession()
	defer sess.Close()

	if page >= 0 && pageSize > 0 {
		start := 0
		if page > 0 {
			start = (page - 1) * pageSize
		}
		sess.Limit(pageSize, start)
	}
	lfsObjects := make([]*LFSMetaObject, 0, pageSize)
	return lfsObjects, sess.Find(&lfsObjects, &LFSMetaObject{RepositoryID: repo.ID})
}

// CountLFSMetaObjects returns a count of all LFSMetaObjects associated with a repository
func (repo *Repository) CountLFSMetaObjects() (int64, error) {
	return x.Count(&LFSMetaObject{RepositoryID: repo.ID})
}

// LFSObjectAccessible checks if a provided Oid is accessible to the user
func LFSObjectAccessible(user *User, oid string) (bool, error) {
	if user.IsAdmin {
		count, err := x.Count(&LFSMetaObject{Oid: oid})
		return (count > 0), err
	}
	cond := accessibleRepositoryCondition(user)
	count, err := x.Where(cond).
		Join("INNER", tbRepository, tbLfsMetaObject+".repository_id = "+tbRepository+".id").
		Count(&LFSMetaObject{Oid: oid})
	return (count > 0), err
}

// LFSAutoAssociate auto associates accessible LFSMetaObjects
func LFSAutoAssociate(metas []*LFSMetaObject, user *User, repoID int64) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	oids := make([]interface{}, len(metas))
	oidMap := make(map[string]*LFSMetaObject, len(metas))
	for i, meta := range metas {
		oids[i] = meta.Oid
		oidMap[meta.Oid] = meta
	}

	cond := builder.NewCond()
	if !user.IsAdmin {
		cond = builder.In(tbLfsMetaObject+".repository_id",
			builder.Select(tbRepository+".id").
				From(tbRepository).Where(accessibleRepositoryCondition(user)))
	}
	newMetas := make([]*LFSMetaObject, 0, len(metas))
	if err := sess.Cols("oid").Where(cond).In("oid", oids...).GroupBy("oid").Find(&newMetas); err != nil {
		return err
	}
	for i := range newMetas {
		newMetas[i].Size = oidMap[newMetas[i].Oid].Size
		newMetas[i].RepositoryID = repoID
	}
	if _, err := sess.InsertMulti(newMetas); err != nil {
		return err
	}

	return sess.Commit()
}
