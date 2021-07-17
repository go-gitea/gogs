// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
	"golang.org/x/crypto/ssh"

	"xorm.io/builder"
)

// KeyType specifies the key type
type KeyType int

const (
	// KeyTypeUser specifies the user key
	KeyTypeUser = iota + 1
	// KeyTypeDeploy specifies the deploy key
	KeyTypeDeploy
	// KeyTypePrincipal specifies the authorized principal key
	KeyTypePrincipal
)

// PublicKey represents a user or deploy SSH public key.
type PublicKey struct {
	ID            int64      `xorm:"pk autoincr"`
	OwnerID       int64      `xorm:"INDEX NOT NULL"`
	Name          string     `xorm:"NOT NULL"`
	Fingerprint   string     `xorm:"INDEX NOT NULL"`
	Content       string     `xorm:"TEXT NOT NULL"`
	Mode          AccessMode `xorm:"NOT NULL DEFAULT 2"`
	Type          KeyType    `xorm:"NOT NULL DEFAULT 1"`
	LoginSourceID int64      `xorm:"NOT NULL DEFAULT 0"`

	CreatedUnix       timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix       timeutil.TimeStamp `xorm:"updated"`
	HasRecentActivity bool               `xorm:"-"`
	HasUsed           bool               `xorm:"-"`
}

// AfterLoad is invoked from XORM after setting the values of all fields of this object.
func (key *PublicKey) AfterLoad() {
	key.HasUsed = key.UpdatedUnix > key.CreatedUnix
	key.HasRecentActivity = key.UpdatedUnix.AddDuration(7*24*time.Hour) > timeutil.TimeStampNow()
}

// OmitEmail returns content of public key without email address.
func (key *PublicKey) OmitEmail() string {
	return strings.Join(strings.Split(key.Content, " ")[:2], " ")
}

// AuthorizedString returns formatted public key string for authorized_keys file.
//
// TODO: Consider dropping this function
func (key *PublicKey) AuthorizedString() string {
	return AuthorizedStringForKey(key)
}

func addKey(e Engine, key *PublicKey) (err error) {
	if len(key.Fingerprint) == 0 {
		key.Fingerprint, err = calcFingerprint(key.Content)
		if err != nil {
			return err
		}
	}

	// Save SSH key.
	if _, err = e.Insert(key); err != nil {
		return err
	}

	return appendAuthorizedKeysToFile(key)
}

// AddPublicKey adds new public key to database and authorized_keys file.
func AddPublicKey(ownerID int64, name, content string, loginSourceID int64) (*PublicKey, error) {
	log.Trace(content)

	fingerprint, err := calcFingerprint(content)
	if err != nil {
		return nil, err
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return nil, err
	}

	if err := checkKeyFingerprint(sess, fingerprint); err != nil {
		return nil, err
	}

	// Key name of same user cannot be duplicated.
	has, err := sess.
		Where("owner_id = ? AND name = ?", ownerID, name).
		Get(new(PublicKey))
	if err != nil {
		return nil, err
	} else if has {
		return nil, ErrKeyNameAlreadyUsed{ownerID, name}
	}

	key := &PublicKey{
		OwnerID:       ownerID,
		Name:          name,
		Fingerprint:   fingerprint,
		Content:       content,
		Mode:          AccessModeWrite,
		Type:          KeyTypeUser,
		LoginSourceID: loginSourceID,
	}
	if err = addKey(sess, key); err != nil {
		return nil, fmt.Errorf("addKey: %v", err)
	}

	return key, sess.Commit()
}

// GetPublicKeyByID returns public key by given ID.
func GetPublicKeyByID(keyID int64) (*PublicKey, error) {
	key := new(PublicKey)
	has, err := x.
		ID(keyID).
		Get(key)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrKeyNotExist{keyID}
	}
	return key, nil
}

func searchPublicKeyByContentWithEngine(e Engine, content string) (*PublicKey, error) {
	key := new(PublicKey)
	has, err := e.
		Where("content like ?", content+"%").
		Get(key)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrKeyNotExist{}
	}
	return key, nil
}

// SearchPublicKeyByContent searches content as prefix (leak e-mail part)
// and returns public key found.
func SearchPublicKeyByContent(content string) (*PublicKey, error) {
	return searchPublicKeyByContentWithEngine(x, content)
}

func searchPublicKeyByContentExactWithEngine(e Engine, content string) (*PublicKey, error) {
	key := new(PublicKey)
	has, err := e.
		Where("content = ?", content).
		Get(key)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrKeyNotExist{}
	}
	return key, nil
}

// SearchPublicKeyByContentExact searches content
// and returns public key found.
func SearchPublicKeyByContentExact(content string) (*PublicKey, error) {
	return searchPublicKeyByContentExactWithEngine(x, content)
}

// SearchPublicKey returns a list of public keys matching the provided arguments.
func SearchPublicKey(uid int64, fingerprint string) ([]*PublicKey, error) {
	keys := make([]*PublicKey, 0, 5)
	cond := builder.NewCond()
	if uid != 0 {
		cond = cond.And(builder.Eq{"owner_id": uid})
	}
	if fingerprint != "" {
		cond = cond.And(builder.Eq{"fingerprint": fingerprint})
	}
	return keys, x.Where(cond).Find(&keys)
}

// ListPublicKeys returns a list of public keys belongs to given user.
func ListPublicKeys(uid int64, listOptions ListOptions) ([]*PublicKey, error) {
	sess := x.Where("owner_id = ? AND type != ?", uid, KeyTypePrincipal)
	if listOptions.Page != 0 {
		sess = listOptions.setSessionPagination(sess)

		keys := make([]*PublicKey, 0, listOptions.PageSize)
		return keys, sess.Find(&keys)
	}

	keys := make([]*PublicKey, 0, 5)
	return keys, sess.Find(&keys)
}

// ListPublicKeysBySource returns a list of synchronized public keys for a given user and login source.
func ListPublicKeysBySource(uid, loginSourceID int64) ([]*PublicKey, error) {
	keys := make([]*PublicKey, 0, 5)
	return keys, x.
		Where("owner_id = ? AND login_source_id = ?", uid, loginSourceID).
		Find(&keys)
}

// UpdatePublicKeyUpdated updates public key use time.
func UpdatePublicKeyUpdated(id int64) error {
	// Check if key exists before update as affected rows count is unreliable
	//    and will return 0 affected rows if two updates are made at the same time
	if cnt, err := x.ID(id).Count(&PublicKey{}); err != nil {
		return err
	} else if cnt != 1 {
		return ErrKeyNotExist{id}
	}

	_, err := x.ID(id).Cols("updated_unix").Update(&PublicKey{
		UpdatedUnix: timeutil.TimeStampNow(),
	})
	if err != nil {
		return err
	}
	return nil
}

// deletePublicKeys does the actual key deletion but does not update authorized_keys file.
func deletePublicKeys(e Engine, keyIDs ...int64) error {
	if len(keyIDs) == 0 {
		return nil
	}

	_, err := e.In("id", keyIDs).Delete(new(PublicKey))
	return err
}

// PublicKeysAreExternallyManaged returns whether the provided KeyID represents an externally managed Key
func PublicKeysAreExternallyManaged(keys []*PublicKey) ([]bool, error) {
	sources := make([]*LoginSource, 0, 5)
	externals := make([]bool, len(keys))
keyloop:
	for i, key := range keys {
		if key.LoginSourceID == 0 {
			externals[i] = false
			continue keyloop
		}

		var source *LoginSource

	sourceloop:
		for _, s := range sources {
			if s.ID == key.LoginSourceID {
				source = s
				break sourceloop
			}
		}

		if source == nil {
			var err error
			source, err = GetLoginSourceByID(key.LoginSourceID)
			if err != nil {
				if IsErrLoginSourceNotExist(err) {
					externals[i] = false
					sources[i] = &LoginSource{
						ID: key.LoginSourceID,
					}
					continue keyloop
				}
				return nil, err
			}
		}

		if sshKeyProvider, ok := source.Cfg.(SSHKeyProvider); ok && sshKeyProvider.ProvidesSSHKeys() {
			// Disable setting SSH keys for this user
			externals[i] = true
		}
	}

	return externals, nil
}

// PublicKeyIsExternallyManaged returns whether the provided KeyID represents an externally managed Key
func PublicKeyIsExternallyManaged(id int64) (bool, error) {
	key, err := GetPublicKeyByID(id)
	if err != nil {
		return false, err
	}
	if key.LoginSourceID == 0 {
		return false, nil
	}
	source, err := GetLoginSourceByID(key.LoginSourceID)
	if err != nil {
		if IsErrLoginSourceNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if sshKeyProvider, ok := source.Cfg.(SSHKeyProvider); ok && sshKeyProvider.ProvidesSSHKeys() {
		// Disable setting SSH keys for this user
		return true, nil
	}
	return false, nil
}

// DeletePublicKey deletes SSH key information both in database and authorized_keys file.
func DeletePublicKey(doer *User, id int64) (err error) {
	key, err := GetPublicKeyByID(id)
	if err != nil {
		return err
	}

	// Check if user has access to delete this key.
	if !doer.IsAdmin && doer.ID != key.OwnerID {
		return ErrKeyAccessDenied{doer.ID, key.ID, "public"}
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if err = deletePublicKeys(sess, id); err != nil {
		return err
	}

	if err = sess.Commit(); err != nil {
		return err
	}
	sess.Close()

	if key.Type == KeyTypePrincipal {
		return RewriteAllPrincipalKeys()
	}

	return RewriteAllPublicKeys()
}

// deleteKeysMarkedForDeletion returns true if ssh keys needs update
func deleteKeysMarkedForDeletion(keys []string) (bool, error) {
	// Start session
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return false, err
	}

	// Delete keys marked for deletion
	var sshKeysNeedUpdate bool
	for _, KeyToDelete := range keys {
		key, err := searchPublicKeyByContentWithEngine(sess, KeyToDelete)
		if err != nil {
			log.Error("SearchPublicKeyByContent: %v", err)
			continue
		}
		if err = deletePublicKeys(sess, key.ID); err != nil {
			log.Error("deletePublicKeys: %v", err)
			continue
		}
		sshKeysNeedUpdate = true
	}

	if err := sess.Commit(); err != nil {
		return false, err
	}

	return sshKeysNeedUpdate, nil
}

// AddPublicKeysBySource add a users public keys. Returns true if there are changes.
func AddPublicKeysBySource(usr *User, s *LoginSource, sshPublicKeys []string) bool {
	var sshKeysNeedUpdate bool
	for _, sshKey := range sshPublicKeys {
		var err error
		found := false
		keys := []byte(sshKey)
	loop:
		for len(keys) > 0 && err == nil {
			var out ssh.PublicKey
			// We ignore options as they are not relevant to Gitea
			out, _, _, keys, err = ssh.ParseAuthorizedKey(keys)
			if err != nil {
				break loop
			}
			found = true
			marshalled := string(ssh.MarshalAuthorizedKey(out))
			marshalled = marshalled[:len(marshalled)-1]
			sshKeyName := fmt.Sprintf("%s-%s", s.Name, ssh.FingerprintSHA256(out))

			if _, err := AddPublicKey(usr.ID, sshKeyName, marshalled, s.ID); err != nil {
				if IsErrKeyAlreadyExist(err) {
					log.Trace("AddPublicKeysBySource[%s]: Public SSH Key %s already exists for user", sshKeyName, usr.Name)
				} else {
					log.Error("AddPublicKeysBySource[%s]: Error adding Public SSH Key for user %s: %v", sshKeyName, usr.Name, err)
				}
			} else {
				log.Trace("AddPublicKeysBySource[%s]: Added Public SSH Key for user %s", sshKeyName, usr.Name)
				sshKeysNeedUpdate = true
			}
		}
		if !found && err != nil {
			log.Warn("AddPublicKeysBySource[%s]: Skipping invalid Public SSH Key for user %s: %v", s.Name, usr.Name, sshKey)
		}
	}
	return sshKeysNeedUpdate
}

// SynchronizePublicKeys updates a users public keys. Returns true if there are changes.
func SynchronizePublicKeys(usr *User, s *LoginSource, sshPublicKeys []string) bool {
	var sshKeysNeedUpdate bool

	log.Trace("synchronizePublicKeys[%s]: Handling Public SSH Key synchronization for user %s", s.Name, usr.Name)

	// Get Public Keys from DB with current LDAP source
	var giteaKeys []string
	keys, err := ListPublicKeysBySource(usr.ID, s.ID)
	if err != nil {
		log.Error("synchronizePublicKeys[%s]: Error listing Public SSH Keys for user %s: %v", s.Name, usr.Name, err)
	}

	for _, v := range keys {
		giteaKeys = append(giteaKeys, v.OmitEmail())
	}

	// Process the provided keys to remove duplicates and name part
	var providedKeys []string
	for _, v := range sshPublicKeys {
		sshKeySplit := strings.Split(v, " ")
		if len(sshKeySplit) > 1 {
			key := strings.Join(sshKeySplit[:2], " ")
			if !util.ExistsInSlice(key, providedKeys) {
				providedKeys = append(providedKeys, key)
			}
		}
	}

	// Check if Public Key sync is needed
	if util.IsEqualSlice(giteaKeys, providedKeys) {
		log.Trace("synchronizePublicKeys[%s]: Public Keys are already in sync for %s (Source:%v/DB:%v)", s.Name, usr.Name, len(providedKeys), len(giteaKeys))
		return false
	}
	log.Trace("synchronizePublicKeys[%s]: Public Key needs update for user %s (Source:%v/DB:%v)", s.Name, usr.Name, len(providedKeys), len(giteaKeys))

	// Add new Public SSH Keys that doesn't already exist in DB
	var newKeys []string
	for _, key := range providedKeys {
		if !util.ExistsInSlice(key, giteaKeys) {
			newKeys = append(newKeys, key)
		}
	}
	if AddPublicKeysBySource(usr, s, newKeys) {
		sshKeysNeedUpdate = true
	}

	// Mark keys from DB that no longer exist in the source for deletion
	var giteaKeysToDelete []string
	for _, giteaKey := range giteaKeys {
		if !util.ExistsInSlice(giteaKey, providedKeys) {
			log.Trace("synchronizePublicKeys[%s]: Marking Public SSH Key for deletion for user %s: %v", s.Name, usr.Name, giteaKey)
			giteaKeysToDelete = append(giteaKeysToDelete, giteaKey)
		}
	}

	// Delete keys from DB that no longer exist in the source
	needUpd, err := deleteKeysMarkedForDeletion(giteaKeysToDelete)
	if err != nil {
		log.Error("synchronizePublicKeys[%s]: Error deleting Public Keys marked for deletion for user %s: %v", s.Name, usr.Name, err)
	}
	if needUpd {
		sshKeysNeedUpdate = true
	}

	return sshKeysNeedUpdate
}
