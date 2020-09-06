// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"crypto/md5"
	"fmt"
	"net/url"
	"strings"

	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/setting"
)

// EmailHash represents a pre-generated hash map
type EmailHash struct {
	Hash  string `xorm:"pk varchar(32)"`
	Email string `xorm:"UNIQUE NOT NULL"`
}

// TableName sets the table name to `email_hash`
func (e *EmailHash) TableName() string {
	return tbEmailHash[1 : len(tbEmailHash)-1]
}

// GetEmailForHash converts a provided md5sum to the email
func GetEmailForHash(md5Sum string) (string, error) {
	return cache.GetString("Avatar:"+md5Sum, func() (string, error) {
		emailHash := EmailHash{
			Hash: strings.ToLower(strings.TrimSpace(md5Sum)),
		}

		_, err := x.Get(&emailHash)
		return emailHash.Email, err
	})
}

// AvatarLink returns an avatar link for a provided email
func AvatarLink(email string) string {
	lowerEmail := strings.ToLower(strings.TrimSpace(email))
	sum := fmt.Sprintf("%x", md5.Sum([]byte(lowerEmail)))
	_, _ = cache.GetString("Avatar:"+sum, func() (string, error) {
		emailHash := &EmailHash{
			Email: lowerEmail,
			Hash:  sum,
		}
		// OK we're going to open a session just because I think that that might hide away any problems with postgres reporting errors
		sess := x.NewSession()
		defer sess.Close()
		if err := sess.Begin(); err != nil {
			// we don't care about any DB problem just return the lowerEmail
			return lowerEmail, nil
		}
		_, _ = sess.Insert(emailHash)
		if err := sess.Commit(); err != nil {
			// Seriously we don't care about any DB problems just return the lowerEmail - we expect the transaction to fail most of the time
			return lowerEmail, nil
		}
		return lowerEmail, nil
	})
	return setting.AppSubURL + "/avatar/" + url.PathEscape(sum)
}
