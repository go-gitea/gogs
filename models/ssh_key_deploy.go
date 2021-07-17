// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"time"

	"code.gitea.io/gitea/modules/timeutil"
	"xorm.io/builder"
	"xorm.io/xorm"
)

// ________                .__                 ____  __.
// \______ \   ____ ______ |  |   ____ ___.__.|    |/ _|____ ___.__.
//  |    |  \_/ __ \\____ \|  |  /  _ <   |  ||      <_/ __ <   |  |
//  |    `   \  ___/|  |_> >  |_(  <_> )___  ||    |  \  ___/\___  |
// /_______  /\___  >   __/|____/\____// ____||____|__ \___  > ____|
//         \/     \/|__|               \/             \/   \/\/
//
// This file contains functions specific to DeployKeys

// DeployKey represents deploy key information and its relation with repository.
type DeployKey struct {
	ID          int64 `xorm:"pk autoincr"`
	KeyID       int64 `xorm:"UNIQUE(s) INDEX"`
	RepoID      int64 `xorm:"UNIQUE(s) INDEX"`
	Name        string
	Fingerprint string
	Content     string `xorm:"-"`

	Mode AccessMode `xorm:"NOT NULL DEFAULT 1"`

	CreatedUnix       timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix       timeutil.TimeStamp `xorm:"updated"`
	HasRecentActivity bool               `xorm:"-"`
	HasUsed           bool               `xorm:"-"`
}

// AfterLoad is invoked from XORM after setting the values of all fields of this object.
func (key *DeployKey) AfterLoad() {
	key.HasUsed = key.UpdatedUnix > key.CreatedUnix
	key.HasRecentActivity = key.UpdatedUnix.AddDuration(7*24*time.Hour) > timeutil.TimeStampNow()
}

// GetContent gets associated public key content.
func (key *DeployKey) GetContent() error {
	pkey, err := GetPublicKeyByID(key.KeyID)
	if err != nil {
		return err
	}
	key.Content = pkey.Content
	return nil
}

// IsReadOnly checks if the key can only be used for read operations
func (key *DeployKey) IsReadOnly() bool {
	return key.Mode == AccessModeRead
}

func checkDeployKey(e Engine, keyID, repoID int64, name string) error {
	// Note: We want error detail, not just true or false here.
	has, err := e.
		Where("key_id = ? AND repo_id = ?", keyID, repoID).
		Get(new(DeployKey))
	if err != nil {
		return err
	} else if has {
		return ErrDeployKeyAlreadyExist{keyID, repoID}
	}

	has, err = e.
		Where("repo_id = ? AND name = ?", repoID, name).
		Get(new(DeployKey))
	if err != nil {
		return err
	} else if has {
		return ErrDeployKeyNameAlreadyUsed{repoID, name}
	}

	return nil
}

// addDeployKey adds new key-repo relation.
func addDeployKey(e *xorm.Session, keyID, repoID int64, name, fingerprint string, mode AccessMode) (*DeployKey, error) {
	if err := checkDeployKey(e, keyID, repoID, name); err != nil {
		return nil, err
	}

	key := &DeployKey{
		KeyID:       keyID,
		RepoID:      repoID,
		Name:        name,
		Fingerprint: fingerprint,
		Mode:        mode,
	}
	_, err := e.Insert(key)
	return key, err
}

// HasDeployKey returns true if public key is a deploy key of given repository.
func HasDeployKey(keyID, repoID int64) bool {
	has, _ := x.
		Where("key_id = ? AND repo_id = ?", keyID, repoID).
		Get(new(DeployKey))
	return has
}

// AddDeployKey add new deploy key to database and authorized_keys file.
func AddDeployKey(repoID int64, name, content string, readOnly bool) (*DeployKey, error) {
	fingerprint, err := calcFingerprint(content)
	if err != nil {
		return nil, err
	}

	accessMode := AccessModeRead
	if !readOnly {
		accessMode = AccessModeWrite
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return nil, err
	}

	pkey := &PublicKey{
		Fingerprint: fingerprint,
	}
	has, err := sess.Get(pkey)
	if err != nil {
		return nil, err
	}

	if has {
		if pkey.Type != KeyTypeDeploy {
			return nil, ErrKeyAlreadyExist{0, fingerprint, ""}
		}
	} else {
		// First time use this deploy key.
		pkey.Mode = accessMode
		pkey.Type = KeyTypeDeploy
		pkey.Content = content
		pkey.Name = name
		if err = addKey(sess, pkey); err != nil {
			return nil, fmt.Errorf("addKey: %v", err)
		}
	}

	key, err := addDeployKey(sess, pkey.ID, repoID, name, pkey.Fingerprint, accessMode)
	if err != nil {
		return nil, err
	}

	return key, sess.Commit()
}

// GetDeployKeyByID returns deploy key by given ID.
func GetDeployKeyByID(id int64) (*DeployKey, error) {
	return getDeployKeyByID(x, id)
}

func getDeployKeyByID(e Engine, id int64) (*DeployKey, error) {
	key := new(DeployKey)
	has, err := e.ID(id).Get(key)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrDeployKeyNotExist{id, 0, 0}
	}
	return key, nil
}

// GetDeployKeyByRepo returns deploy key by given public key ID and repository ID.
func GetDeployKeyByRepo(keyID, repoID int64) (*DeployKey, error) {
	return getDeployKeyByRepo(x, keyID, repoID)
}

func getDeployKeyByRepo(e Engine, keyID, repoID int64) (*DeployKey, error) {
	key := &DeployKey{
		KeyID:  keyID,
		RepoID: repoID,
	}
	has, err := e.Get(key)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrDeployKeyNotExist{0, keyID, repoID}
	}
	return key, nil
}

// UpdateDeployKeyCols updates deploy key information in the specified columns.
func UpdateDeployKeyCols(key *DeployKey, cols ...string) error {
	_, err := x.ID(key.ID).Cols(cols...).Update(key)
	return err
}

// UpdateDeployKey updates deploy key information.
func UpdateDeployKey(key *DeployKey) error {
	_, err := x.ID(key.ID).AllCols().Update(key)
	return err
}

// DeleteDeployKey deletes deploy key from its repository authorized_keys file if needed.
func DeleteDeployKey(doer *User, id int64) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	if err := deleteDeployKey(sess, doer, id); err != nil {
		return err
	}
	return sess.Commit()
}

func deleteDeployKey(sess Engine, doer *User, id int64) error {
	key, err := getDeployKeyByID(sess, id)
	if err != nil {
		if IsErrDeployKeyNotExist(err) {
			return nil
		}
		return fmt.Errorf("GetDeployKeyByID: %v", err)
	}

	// Check if user has access to delete this key.
	if !doer.IsAdmin {
		repo, err := getRepositoryByID(sess, key.RepoID)
		if err != nil {
			return fmt.Errorf("GetRepositoryByID: %v", err)
		}
		has, err := isUserRepoAdmin(sess, repo, doer)
		if err != nil {
			return fmt.Errorf("GetUserRepoPermission: %v", err)
		} else if !has {
			return ErrKeyAccessDenied{doer.ID, key.ID, "deploy"}
		}
	}

	if _, err = sess.ID(key.ID).Delete(new(DeployKey)); err != nil {
		return fmt.Errorf("delete deploy key [%d]: %v", key.ID, err)
	}

	// Check if this is the last reference to same key content.
	has, err := sess.
		Where("key_id = ?", key.KeyID).
		Get(new(DeployKey))
	if err != nil {
		return err
	} else if !has {
		if err = deletePublicKeys(sess, key.KeyID); err != nil {
			return err
		}

		// after deleted the public keys, should rewrite the public keys file
		if err = rewriteAllPublicKeys(sess); err != nil {
			return err
		}
	}

	return nil
}

// ListDeployKeys returns all deploy keys by given repository ID.
func ListDeployKeys(repoID int64, listOptions ListOptions) ([]*DeployKey, error) {
	return listDeployKeys(x, repoID, listOptions)
}

func listDeployKeys(e Engine, repoID int64, listOptions ListOptions) ([]*DeployKey, error) {
	sess := e.Where("repo_id = ?", repoID)
	if listOptions.Page != 0 {
		sess = listOptions.setSessionPagination(sess)

		keys := make([]*DeployKey, 0, listOptions.PageSize)
		return keys, sess.Find(&keys)
	}

	keys := make([]*DeployKey, 0, 5)
	return keys, sess.Find(&keys)
}

// SearchDeployKeys returns a list of deploy keys matching the provided arguments.
func SearchDeployKeys(repoID, keyID int64, fingerprint string) ([]*DeployKey, error) {
	keys := make([]*DeployKey, 0, 5)
	cond := builder.NewCond()
	if repoID != 0 {
		cond = cond.And(builder.Eq{"repo_id": repoID})
	}
	if keyID != 0 {
		cond = cond.And(builder.Eq{"key_id": keyID})
	}
	if fingerprint != "" {
		cond = cond.And(builder.Eq{"fingerprint": fingerprint})
	}
	return keys, x.Where(cond).Find(&keys)
}
