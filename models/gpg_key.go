// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"bytes"
	"container/list"
	"crypto"
	"encoding/base64"
	"fmt"
	"hash"
	"io"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/keybase/go-crypto/openpgp"
	"github.com/keybase/go-crypto/openpgp/armor"
	"github.com/keybase/go-crypto/openpgp/packet"
	"xorm.io/xorm"
)

// GPGKey represents a GPG key.
type GPGKey struct {
	ID                int64              `xorm:"pk autoincr"`
	OwnerID           int64              `xorm:"INDEX NOT NULL"`
	KeyID             string             `xorm:"INDEX CHAR(16) NOT NULL"`
	PrimaryKeyID      string             `xorm:"CHAR(16)"`
	Content           string             `xorm:"TEXT NOT NULL"`
	CreatedUnix       timeutil.TimeStamp `xorm:"created"`
	ExpiredUnix       timeutil.TimeStamp
	AddedUnix         timeutil.TimeStamp
	SubsKey           []*GPGKey `xorm:"-"`
	Emails            []*EmailAddress
	CanSign           bool
	CanEncryptComms   bool
	CanEncryptStorage bool
	CanCertify        bool
}

// GPGKeyImport the original import of key
type GPGKeyImport struct {
	KeyID   string `xorm:"pk CHAR(16) NOT NULL"`
	Content string `xorm:"TEXT NOT NULL"`
}

// BeforeInsert will be invoked by XORM before inserting a record
func (key *GPGKey) BeforeInsert() {
	key.AddedUnix = timeutil.TimeStampNow()
}

// AfterLoad is invoked from XORM after setting the values of all fields of this object.
func (key *GPGKey) AfterLoad(session *xorm.Session) {
	err := session.Where("primary_key_id=?", key.KeyID).Find(&key.SubsKey)
	if err != nil {
		log.Error("Find Sub GPGkeys[%s]: %v", key.KeyID, err)
	}
}

// ListGPGKeys returns a list of public keys belongs to given user.
func ListGPGKeys(uid int64, listOptions ListOptions) ([]*GPGKey, error) {
	return listGPGKeys(x, uid, listOptions)
}

func listGPGKeys(e Engine, uid int64, listOptions ListOptions) ([]*GPGKey, error) {
	sess := e.Table(&GPGKey{}).Where("owner_id=? AND primary_key_id=''", uid)
	if listOptions.Page != 0 {
		sess = listOptions.setSessionPagination(sess)
	}

	keys := make([]*GPGKey, 0, 2)
	return keys, sess.Find(&keys)
}

// GetGPGKeyByID returns public key by given ID.
func GetGPGKeyByID(keyID int64) (*GPGKey, error) {
	key := new(GPGKey)
	has, err := x.ID(keyID).Get(key)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrGPGKeyNotExist{keyID}
	}
	return key, nil
}

// GetGPGKeysByKeyID returns public key by given ID.
func GetGPGKeysByKeyID(keyID string) ([]*GPGKey, error) {
	keys := make([]*GPGKey, 0, 1)
	return keys, x.Where("key_id=?", keyID).Find(&keys)
}

// GetGPGImportByKeyID returns the import public armored key by given KeyID.
func GetGPGImportByKeyID(keyID string) (*GPGKeyImport, error) {
	key := new(GPGKeyImport)
	has, err := x.ID(keyID).Get(key)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrGPGKeyImportNotExist{keyID}
	}
	return key, nil
}

// checkArmoredGPGKeyString checks if the given key string is a valid GPG armored key.
// The function returns the actual public key on success
func checkArmoredGPGKeyString(content string) (openpgp.EntityList, error) {
	list, err := openpgp.ReadArmoredKeyRing(strings.NewReader(content))
	if err != nil {
		return nil, ErrGPGKeyParsing{err}
	}
	return list, nil
}

// addGPGKey add key, import and subkeys to database
func addGPGKey(e Engine, key *GPGKey, content string) (err error) {
	// Add GPGKeyImport
	if _, err = e.Insert(GPGKeyImport{
		KeyID:   key.KeyID,
		Content: content,
	}); err != nil {
		return err
	}
	// Save GPG primary key.
	if _, err = e.Insert(key); err != nil {
		return err
	}
	// Save GPG subs key.
	for _, subkey := range key.SubsKey {
		if err := addGPGSubKey(e, subkey); err != nil {
			return err
		}
	}
	return nil
}

// addGPGSubKey add subkeys to database
func addGPGSubKey(e Engine, key *GPGKey) (err error) {
	// Save GPG primary key.
	if _, err = e.Insert(key); err != nil {
		return err
	}
	// Save GPG subs key.
	for _, subkey := range key.SubsKey {
		if err := addGPGSubKey(e, subkey); err != nil {
			return err
		}
	}
	return nil
}

// AddGPGKey adds new public key to database.
func AddGPGKey(ownerID int64, content string) ([]*GPGKey, error) {
	ekeys, err := checkArmoredGPGKeyString(content)
	if err != nil {
		return nil, err
	}
	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return nil, err
	}
	keys := make([]*GPGKey, 0, len(ekeys))
	for _, ekey := range ekeys {
		// Key ID cannot be duplicated.
		has, err := sess.Where("key_id=?", ekey.PrimaryKey.KeyIdString()).
			Get(new(GPGKey))
		if err != nil {
			return nil, err
		} else if has {
			return nil, ErrGPGKeyIDAlreadyUsed{ekey.PrimaryKey.KeyIdString()}
		}

		// Get DB session

		key, err := parseGPGKey(ownerID, ekey)
		if err != nil {
			return nil, err
		}

		if err = addGPGKey(sess, key, content); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, sess.Commit()
}

// base64EncPubKey encode public key content to base 64
func base64EncPubKey(pubkey *packet.PublicKey) (string, error) {
	var w bytes.Buffer
	err := pubkey.Serialize(&w)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(w.Bytes()), nil
}

// base64DecPubKey decode public key content from base 64
func base64DecPubKey(content string) (*packet.PublicKey, error) {
	b, err := readerFromBase64(content)
	if err != nil {
		return nil, err
	}
	// Read key
	p, err := packet.Read(b)
	if err != nil {
		return nil, err
	}
	// Check type
	pkey, ok := p.(*packet.PublicKey)
	if !ok {
		return nil, fmt.Errorf("key is not a public key")
	}
	return pkey, nil
}

// GPGKeyToEntity retrieve the imported key and the traducted entity
func GPGKeyToEntity(k *GPGKey) (*openpgp.Entity, error) {
	impKey, err := GetGPGImportByKeyID(k.KeyID)
	if err != nil {
		return nil, err
	}
	keys, err := checkArmoredGPGKeyString(impKey.Content)
	if err != nil {
		return nil, err
	}
	return keys[0], err
}

// parseSubGPGKey parse a sub Key
func parseSubGPGKey(ownerID int64, primaryID string, pubkey *packet.PublicKey, expiry time.Time) (*GPGKey, error) {
	content, err := base64EncPubKey(pubkey)
	if err != nil {
		return nil, err
	}
	return &GPGKey{
		OwnerID:           ownerID,
		KeyID:             pubkey.KeyIdString(),
		PrimaryKeyID:      primaryID,
		Content:           content,
		CreatedUnix:       timeutil.TimeStamp(pubkey.CreationTime.Unix()),
		ExpiredUnix:       timeutil.TimeStamp(expiry.Unix()),
		CanSign:           pubkey.CanSign(),
		CanEncryptComms:   pubkey.PubKeyAlgo.CanEncrypt(),
		CanEncryptStorage: pubkey.PubKeyAlgo.CanEncrypt(),
		CanCertify:        pubkey.PubKeyAlgo.CanSign(),
	}, nil
}

// getExpiryTime extract the expire time of primary key based on sig
func getExpiryTime(e *openpgp.Entity) time.Time {
	expiry := time.Time{}
	// Extract self-sign for expire date based on : https://github.com/golang/crypto/blob/master/openpgp/keys.go#L165
	var selfSig *packet.Signature
	for _, ident := range e.Identities {
		if selfSig == nil {
			selfSig = ident.SelfSignature
		} else if ident.SelfSignature.IsPrimaryId != nil && *ident.SelfSignature.IsPrimaryId {
			selfSig = ident.SelfSignature
			break
		}
	}
	if selfSig.KeyLifetimeSecs != nil {
		expiry = e.PrimaryKey.CreationTime.Add(time.Duration(*selfSig.KeyLifetimeSecs) * time.Second)
	}
	return expiry
}

// parseGPGKey parse a PrimaryKey entity (primary key + subs keys + self-signature)
func parseGPGKey(ownerID int64, e *openpgp.Entity) (*GPGKey, error) {
	pubkey := e.PrimaryKey
	expiry := getExpiryTime(e)

	// Parse Subkeys
	subkeys := make([]*GPGKey, len(e.Subkeys))
	for i, k := range e.Subkeys {
		subs, err := parseSubGPGKey(ownerID, pubkey.KeyIdString(), k.PublicKey, expiry)
		if err != nil {
			return nil, ErrGPGKeyParsing{ParseError: err}
		}
		subkeys[i] = subs
	}

	// Check emails
	userEmails, err := GetEmailAddresses(ownerID)
	if err != nil {
		return nil, err
	}

	emails := make([]*EmailAddress, 0, len(e.Identities))
	for _, ident := range e.Identities {
		if ident.Revocation != nil {
			continue
		}
		email := strings.ToLower(strings.TrimSpace(ident.UserId.Email))
		for _, e := range userEmails {
			if e.LowerEmail == email {
				emails = append(emails, e)
				break
			}
		}
	}

	// In the case no email as been found
	if len(emails) == 0 {
		failedEmails := make([]string, 0, len(e.Identities))
		for _, ident := range e.Identities {
			failedEmails = append(failedEmails, ident.UserId.Email)
		}
		return nil, ErrGPGNoEmailFound{failedEmails}
	}

	content, err := base64EncPubKey(pubkey)
	if err != nil {
		return nil, err
	}
	return &GPGKey{
		OwnerID:           ownerID,
		KeyID:             pubkey.KeyIdString(),
		PrimaryKeyID:      "",
		Content:           content,
		CreatedUnix:       timeutil.TimeStamp(pubkey.CreationTime.Unix()),
		ExpiredUnix:       timeutil.TimeStamp(expiry.Unix()),
		Emails:            emails,
		SubsKey:           subkeys,
		CanSign:           pubkey.CanSign(),
		CanEncryptComms:   pubkey.PubKeyAlgo.CanEncrypt(),
		CanEncryptStorage: pubkey.PubKeyAlgo.CanEncrypt(),
		CanCertify:        pubkey.PubKeyAlgo.CanSign(),
	}, nil
}

// deleteGPGKey does the actual key deletion
func deleteGPGKey(e *xorm.Session, keyID string) (int64, error) {
	if keyID == "" {
		return 0, fmt.Errorf("empty KeyId forbidden") // Should never happen but just to be sure
	}
	// Delete imported key
	n, err := e.Where("key_id=?", keyID).Delete(new(GPGKeyImport))
	if err != nil {
		return n, err
	}
	return e.Where("key_id=?", keyID).Or("primary_key_id=?", keyID).Delete(new(GPGKey))
}

// DeleteGPGKey deletes GPG key information in database.
func DeleteGPGKey(doer *User, id int64) (err error) {
	key, err := GetGPGKeyByID(id)
	if err != nil {
		if IsErrGPGKeyNotExist(err) {
			return nil
		}
		return fmt.Errorf("GetPublicKeyByID: %v", err)
	}

	// Check if user has access to delete this key.
	if !doer.IsAdmin && doer.ID != key.OwnerID {
		return ErrGPGKeyAccessDenied{doer.ID, key.ID}
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if _, err = deleteGPGKey(sess, key.KeyID); err != nil {
		return err
	}

	return sess.Commit()
}

// CommitVerification represents a commit validation of signature
type CommitVerification struct {
	Verified       bool
	Warning        bool
	Reason         string
	SigningUser    *User
	CommittingUser *User
	SigningEmail   string
	SigningKey     *GPGKey
	TrustStatus    string
}

// SignCommit represents a commit with validation of signature.
type SignCommit struct {
	Verification *CommitVerification
	*UserCommit
}

const (
	// BadSignature is used as the reason when the signature has a KeyID that is in the db
	// but no key that has that ID verifies the signature. This is a suspicious failure.
	BadSignature = "gpg.error.probable_bad_signature"
	// BadDefaultSignature is used as the reason when the signature has a KeyID that matches the
	// default Key but is not verified by the default key. This is a suspicious failure.
	BadDefaultSignature = "gpg.error.probable_bad_default_signature"
	// NoKeyFound is used as the reason when no key can be found to verify the signature.
	NoKeyFound = "gpg.error.no_gpg_keys_found"
)

func readerFromBase64(s string) (io.Reader, error) {
	bs, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	return bytes.NewBuffer(bs), nil
}

func populateHash(hashFunc crypto.Hash, msg []byte) (hash.Hash, error) {
	h := hashFunc.New()
	if _, err := h.Write(msg); err != nil {
		return nil, err
	}
	return h, nil
}

// readArmoredSign read an armored signature block with the given type. https://sourcegraph.com/github.com/golang/crypto/-/blob/openpgp/read.go#L24:6-24:17
func readArmoredSign(r io.Reader) (body io.Reader, err error) {
	block, err := armor.Decode(r)
	if err != nil {
		return
	}
	if block.Type != openpgp.SignatureType {
		return nil, fmt.Errorf("expected '" + openpgp.SignatureType + "', got: " + block.Type)
	}
	return block.Body, nil
}

func extractSignature(s string) (*packet.Signature, error) {
	r, err := readArmoredSign(strings.NewReader(s))
	if err != nil {
		return nil, fmt.Errorf("Failed to read signature armor")
	}
	p, err := packet.Read(r)
	if err != nil {
		return nil, fmt.Errorf("Failed to read signature packet")
	}
	sig, ok := p.(*packet.Signature)
	if !ok {
		return nil, fmt.Errorf("Packet is not a signature")
	}
	return sig, nil
}

func verifySign(s *packet.Signature, h hash.Hash, k *GPGKey) error {
	// Check if key can sign
	if !k.CanSign {
		return fmt.Errorf("key can not sign")
	}
	// Decode key
	pkey, err := base64DecPubKey(k.Content)
	if err != nil {
		return err
	}
	return pkey.VerifySignature(h, s)
}

func hashAndVerify(sig *packet.Signature, payload string, k *GPGKey, committer, signer *User, email string) *CommitVerification {
	// Generating hash of commit
	hash, err := populateHash(sig.Hash, []byte(payload))
	if err != nil { // Skipping failed to generate hash
		log.Error("PopulateHash: %v", err)
		return &CommitVerification{
			CommittingUser: committer,
			Verified:       false,
			Reason:         "gpg.error.generate_hash",
		}
	}

	if err := verifySign(sig, hash, k); err == nil {
		return &CommitVerification{ // Everything is ok
			CommittingUser: committer,
			Verified:       true,
			Reason:         fmt.Sprintf("%s / %s", signer.Name, k.KeyID),
			SigningUser:    signer,
			SigningKey:     k,
			SigningEmail:   email,
		}
	}
	return nil
}

func hashAndVerifyWithSubKeys(sig *packet.Signature, payload string, k *GPGKey, committer, signer *User, email string) *CommitVerification {
	commitVerification := hashAndVerify(sig, payload, k, committer, signer, email)
	if commitVerification != nil {
		return commitVerification
	}

	// And test also SubsKey
	for _, sk := range k.SubsKey {
		commitVerification := hashAndVerify(sig, payload, sk, committer, signer, email)
		if commitVerification != nil {
			return commitVerification
		}
	}
	return nil
}

func hashAndVerifyForKeyID(sig *packet.Signature, payload string, committer *User, keyID, name, email string) *CommitVerification {
	if keyID == "" {
		return nil
	}
	keys, err := GetGPGKeysByKeyID(keyID)
	if err != nil {
		log.Error("GetGPGKeysByKeyID: %v", err)
		return &CommitVerification{
			CommittingUser: committer,
			Verified:       false,
			Reason:         "gpg.error.failed_retrieval_gpg_keys",
		}
	}
	if len(keys) == 0 {
		return nil
	}
	for _, key := range keys {
		var primaryKeys []*GPGKey
		if key.PrimaryKeyID != "" {
			primaryKeys, err = GetGPGKeysByKeyID(key.PrimaryKeyID)
			if err != nil {
				log.Error("GetGPGKeysByKeyID: %v", err)
				return &CommitVerification{
					CommittingUser: committer,
					Verified:       false,
					Reason:         "gpg.error.failed_retrieval_gpg_keys",
				}
			}
		}
		activated := false
		if len(email) != 0 {
			for _, e := range key.Emails {
				if e.IsActivated && strings.EqualFold(e.Email, email) {
					activated = true
					email = e.Email
					break
				}
			}
			if !activated {
				for _, pkey := range primaryKeys {
					for _, e := range pkey.Emails {
						if e.IsActivated && strings.EqualFold(e.Email, email) {
							activated = true
							email = e.Email
							break
						}
					}
					if activated {
						break
					}
				}
			}
		} else {
			for _, e := range key.Emails {
				if e.IsActivated {
					activated = true
					email = e.Email
					break
				}
			}
			if !activated {
				for _, pkey := range primaryKeys {
					for _, e := range pkey.Emails {
						if e.IsActivated {
							activated = true
							email = e.Email
							break
						}
					}
					if activated {
						break
					}
				}
			}
		}

		if !activated {
			continue
		}
		signer := &User{
			Name:  name,
			Email: email,
		}
		if key.OwnerID != 0 {
			owner, err := GetUserByID(key.OwnerID)
			if err == nil {
				signer = owner
			} else if !IsErrUserNotExist(err) {
				log.Error("Failed to GetUserByID: %d for key ID: %d (%s) %v", key.OwnerID, key.ID, key.KeyID, err)
				return &CommitVerification{
					CommittingUser: committer,
					Verified:       false,
					Reason:         "gpg.error.no_committer_account",
				}
			}
		}
		commitVerification := hashAndVerifyWithSubKeys(sig, payload, key, committer, signer, email)
		if commitVerification != nil {
			return commitVerification
		}
	}
	// This is a bad situation ... We have a key id that is in our database but the signature doesn't match.
	return &CommitVerification{
		CommittingUser: committer,
		Verified:       false,
		Warning:        true,
		Reason:         BadSignature,
	}
}

// ParseCommitWithSignature check if signature is good against keystore.
func ParseCommitWithSignature(c *git.Commit) *CommitVerification {
	var committer *User
	if c.Committer != nil {
		var err error
		// Find Committer account
		committer, err = GetUserByEmail(c.Committer.Email) // This finds the user by primary email or activated email so commit will not be valid if email is not
		if err != nil {                                    // Skipping not user for committer
			committer = &User{
				Name:  c.Committer.Name,
				Email: c.Committer.Email,
			}
			// We can expect this to often be an ErrUserNotExist. in the case
			// it is not, however, it is important to log it.
			if !IsErrUserNotExist(err) {
				log.Error("GetUserByEmail: %v", err)
				return &CommitVerification{
					CommittingUser: committer,
					Verified:       false,
					Reason:         "gpg.error.no_committer_account",
				}
			}

		}
	}

	// If no signature just report the committer
	if c.Signature == nil {
		return &CommitVerification{
			CommittingUser: committer,
			Verified:       false,                         // Default value
			Reason:         "gpg.error.not_signed_commit", // Default value
		}
	}

	// Parsing signature
	sig, err := extractSignature(c.Signature.Signature)
	if err != nil { // Skipping failed to extract sign
		log.Error("SignatureRead err: %v", err)
		return &CommitVerification{
			CommittingUser: committer,
			Verified:       false,
			Reason:         "gpg.error.extract_sign",
		}
	}

	keyID := ""
	if sig.IssuerKeyId != nil && (*sig.IssuerKeyId) != 0 {
		keyID = fmt.Sprintf("%X", *sig.IssuerKeyId)
	}
	if keyID == "" && sig.IssuerFingerprint != nil && len(sig.IssuerFingerprint) > 0 {
		keyID = fmt.Sprintf("%X", sig.IssuerFingerprint[12:20])
	}
	defaultReason := NoKeyFound

	// First check if the sig has a keyID and if so just look at that
	if commitVerification := hashAndVerifyForKeyID(
		sig,
		c.Signature.Payload,
		committer,
		keyID,
		setting.AppName,
		""); commitVerification != nil {
		if commitVerification.Reason == BadSignature {
			defaultReason = BadSignature
		} else {
			return commitVerification
		}
	}

	// Now try to associate the signature with the committer, if present
	if committer.ID != 0 {
		keys, err := ListGPGKeys(committer.ID, ListOptions{})
		if err != nil { // Skipping failed to get gpg keys of user
			log.Error("ListGPGKeys: %v", err)
			return &CommitVerification{
				CommittingUser: committer,
				Verified:       false,
				Reason:         "gpg.error.failed_retrieval_gpg_keys",
			}
		}

		for _, k := range keys {
			// Pre-check (& optimization) that emails attached to key can be attached to the committer email and can validate
			canValidate := false
			email := ""
			for _, e := range k.Emails {
				if e.IsActivated && strings.EqualFold(e.Email, c.Committer.Email) {
					canValidate = true
					email = e.Email
					break
				}
			}
			if !canValidate {
				continue // Skip this key
			}

			commitVerification := hashAndVerifyWithSubKeys(sig, c.Signature.Payload, k, committer, committer, email)
			if commitVerification != nil {
				return commitVerification
			}
		}
	}

	if setting.Repository.Signing.SigningKey != "" && setting.Repository.Signing.SigningKey != "default" && setting.Repository.Signing.SigningKey != "none" {
		// OK we should try the default key
		gpgSettings := git.GPGSettings{
			Sign:  true,
			KeyID: setting.Repository.Signing.SigningKey,
			Name:  setting.Repository.Signing.SigningName,
			Email: setting.Repository.Signing.SigningEmail,
		}
		if err := gpgSettings.LoadPublicKeyContent(); err != nil {
			log.Error("Error getting default signing key: %s %v", gpgSettings.KeyID, err)
		} else if commitVerification := verifyWithGPGSettings(&gpgSettings, sig, c.Signature.Payload, committer, keyID); commitVerification != nil {
			if commitVerification.Reason == BadSignature {
				defaultReason = BadSignature
			} else {
				return commitVerification
			}
		}
	}

	defaultGPGSettings, err := c.GetRepositoryDefaultPublicGPGKey(false)
	if err != nil {
		log.Error("Error getting default public gpg key: %v", err)
	} else if defaultGPGSettings == nil {
		log.Warn("Unable to get defaultGPGSettings for unattached commit: %s", c.ID.String())
	} else if defaultGPGSettings.Sign {
		if commitVerification := verifyWithGPGSettings(defaultGPGSettings, sig, c.Signature.Payload, committer, keyID); commitVerification != nil {
			if commitVerification.Reason == BadSignature {
				defaultReason = BadSignature
			} else {
				return commitVerification
			}
		}
	}

	return &CommitVerification{ // Default at this stage
		CommittingUser: committer,
		Verified:       false,
		Warning:        defaultReason != NoKeyFound,
		Reason:         defaultReason,
		SigningKey: &GPGKey{
			KeyID: keyID,
		},
	}
}

func verifyWithGPGSettings(gpgSettings *git.GPGSettings, sig *packet.Signature, payload string, committer *User, keyID string) *CommitVerification {
	// First try to find the key in the db
	if commitVerification := hashAndVerifyForKeyID(sig, payload, committer, gpgSettings.KeyID, gpgSettings.Name, gpgSettings.Email); commitVerification != nil {
		return commitVerification
	}

	// Otherwise we have to parse the key
	ekeys, err := checkArmoredGPGKeyString(gpgSettings.PublicKeyContent)
	if err != nil {
		log.Error("Unable to get default signing key: %v", err)
		return &CommitVerification{
			CommittingUser: committer,
			Verified:       false,
			Reason:         "gpg.error.generate_hash",
		}
	}
	for _, ekey := range ekeys {
		pubkey := ekey.PrimaryKey
		content, err := base64EncPubKey(pubkey)
		if err != nil {
			return &CommitVerification{
				CommittingUser: committer,
				Verified:       false,
				Reason:         "gpg.error.generate_hash",
			}
		}
		k := &GPGKey{
			Content: content,
			CanSign: pubkey.CanSign(),
			KeyID:   pubkey.KeyIdString(),
		}
		for _, subKey := range ekey.Subkeys {
			content, err := base64EncPubKey(subKey.PublicKey)
			if err != nil {
				return &CommitVerification{
					CommittingUser: committer,
					Verified:       false,
					Reason:         "gpg.error.generate_hash",
				}
			}
			k.SubsKey = append(k.SubsKey, &GPGKey{
				Content: content,
				CanSign: subKey.PublicKey.CanSign(),
				KeyID:   subKey.PublicKey.KeyIdString(),
			})
		}
		if commitVerification := hashAndVerifyWithSubKeys(sig, payload, k, committer, &User{
			Name:  gpgSettings.Name,
			Email: gpgSettings.Email,
		}, gpgSettings.Email); commitVerification != nil {
			return commitVerification
		}
		if keyID == k.KeyID {
			// This is a bad situation ... We have a key id that matches our default key but the signature doesn't match.
			return &CommitVerification{
				CommittingUser: committer,
				Verified:       false,
				Warning:        true,
				Reason:         BadSignature,
			}
		}
	}
	return nil
}

// ParseCommitsWithSignature checks if signaute of commits are corresponding to users gpg keys.
func ParseCommitsWithSignature(oldCommits *list.List, repository *Repository) *list.List {
	var (
		newCommits = list.New()
		e          = oldCommits.Front()
	)
	keyMap := map[string]bool{}

	for e != nil {
		c := e.Value.(UserCommit)
		signCommit := SignCommit{
			UserCommit:   &c,
			Verification: ParseCommitWithSignature(c.Commit),
		}

		_ = CalculateTrustStatus(signCommit.Verification, repository, &keyMap)

		newCommits.PushBack(signCommit)
		e = e.Next()
	}
	return newCommits
}

// CalculateTrustStatus will calculate the TrustStatus for a commit verification within a repository
func CalculateTrustStatus(verification *CommitVerification, repository *Repository, keyMap *map[string]bool) (err error) {
	if !verification.Verified {
		return
	}

	// There are several trust models in Gitea
	trustModel := repository.GetTrustModel()

	// In the Committer trust model a signature is trusted if it matches the committer
	// - it doesn't matter if they're a collaborator, the owner, Gitea or Github
	// NB: This model is commit verification only
	if trustModel == CommitterTrustModel {
		// default to "unmatched"
		verification.TrustStatus = "unmatched"

		// We can only verify against users in our database but the default key will match
		// against by email if it is not in the db.
		if (verification.SigningUser.ID != 0 &&
			verification.CommittingUser.ID == verification.SigningUser.ID) ||
			(verification.SigningUser.ID == 0 && verification.CommittingUser.ID == 0 &&
				verification.SigningUser.Email == verification.CommittingUser.Email) {
			verification.TrustStatus = "trusted"
		}
		return
	}

	// Now we drop to the more nuanced trust models...
	verification.TrustStatus = "trusted"

	if verification.SigningUser.ID == 0 {
		// This commit is signed by the default key - but this key is not assigned to a user in the DB.

		// However in the CollaboratorCommitterTrustModel we cannot mark this as trusted
		// unless the default key matches the email of a non-user.
		if trustModel == CollaboratorCommitterTrustModel && (verification.CommittingUser.ID != 0 ||
			verification.SigningUser.Email != verification.CommittingUser.Email) {
			verification.TrustStatus = "untrusted"
		}
		return
	}

	var isMember bool
	if keyMap != nil {
		var has bool
		isMember, has = (*keyMap)[verification.SigningKey.KeyID]
		if !has {
			isMember, err = repository.IsOwnerMemberCollaborator(verification.SigningUser.ID)
			(*keyMap)[verification.SigningKey.KeyID] = isMember
		}
	} else {
		isMember, err = repository.IsOwnerMemberCollaborator(verification.SigningUser.ID)
	}

	if !isMember {
		verification.TrustStatus = "untrusted"
		if verification.CommittingUser.ID != verification.SigningUser.ID {
			// The committing user and the signing user are not the same
			// This should be marked as questionable unless the signing user is a collaborator/team member etc.
			verification.TrustStatus = "unmatched"
		}
	} else if trustModel == CollaboratorCommitterTrustModel && verification.CommittingUser.ID != verification.SigningUser.ID {
		// The committing user and the signing user are not the same and our trustmodel states that they must match
		verification.TrustStatus = "unmatched"
	}

	return
}
