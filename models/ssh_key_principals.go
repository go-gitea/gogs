// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"errors"
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/setting"
)

// __________       .__              .__             .__
// \______   _______|__| ____   ____ |_____________  |  |   ______
//  |     ___\_  __ |  |/    \_/ ___\|  \____ \__  \ |  |  /  ___/
//  |    |    |  | \|  |   |  \  \___|  |  |_> / __ \|  |__\___ \
//  |____|    |__|  |__|___|  /\___  |__|   __(____  |____/____  >
//                          \/     \/   |__|       \/          \/
//
// This file contains functions related to principals

// AddPrincipalKey adds new principal to database and authorized_principals file.
func AddPrincipalKey(ownerID int64, content string, loginSourceID int64) (*PublicKey, error) {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return nil, err
	}

	// Principals cannot be duplicated.
	has, err := sess.
		Where("content = ? AND type = ?", content, KeyTypePrincipal).
		Get(new(PublicKey))
	if err != nil {
		return nil, err
	} else if has {
		return nil, ErrKeyAlreadyExist{0, "", content}
	}

	key := &PublicKey{
		OwnerID:       ownerID,
		Name:          content,
		Content:       content,
		Mode:          AccessModeWrite,
		Type:          KeyTypePrincipal,
		LoginSourceID: loginSourceID,
	}
	if err = addPrincipalKey(sess, key); err != nil {
		return nil, fmt.Errorf("addKey: %v", err)
	}

	if err = sess.Commit(); err != nil {
		return nil, err
	}

	sess.Close()

	return key, RewriteAllPrincipalKeys()
}

func addPrincipalKey(e Engine, key *PublicKey) (err error) {
	// Save Key representing a principal.
	if _, err = e.Insert(key); err != nil {
		return err
	}

	return nil
}

// CheckPrincipalKeyString strips spaces and returns an error if the given principal contains newlines
func CheckPrincipalKeyString(user *User, content string) (_ string, err error) {
	if setting.SSH.Disabled {
		return "", ErrSSHDisabled{}
	}

	content = strings.TrimSpace(content)
	if strings.ContainsAny(content, "\r\n") {
		return "", errors.New("only a single line with a single principal please")
	}

	// check all the allowed principals, email, username or anything
	// if any matches, return ok
	for _, v := range setting.SSH.AuthorizedPrincipalsAllow {
		switch v {
		case "anything":
			return content, nil
		case "email":
			emails, err := GetEmailAddresses(user.ID)
			if err != nil {
				return "", err
			}
			for _, email := range emails {
				if !email.IsActivated {
					continue
				}
				if content == email.Email {
					return content, nil
				}
			}

		case "username":
			if content == user.Name {
				return content, nil
			}
		}
	}

	return "", fmt.Errorf("didn't match allowed principals: %s", setting.SSH.AuthorizedPrincipalsAllow)
}

// ListPrincipalKeys returns a list of principals belongs to given user.
func ListPrincipalKeys(uid int64, listOptions ListOptions) ([]*PublicKey, error) {
	sess := x.Where("owner_id = ? AND type = ?", uid, KeyTypePrincipal)
	if listOptions.Page != 0 {
		sess = listOptions.setSessionPagination(sess)

		keys := make([]*PublicKey, 0, listOptions.PageSize)
		return keys, sess.Find(&keys)
	}

	keys := make([]*PublicKey, 0, 5)
	return keys, sess.Find(&keys)
}
