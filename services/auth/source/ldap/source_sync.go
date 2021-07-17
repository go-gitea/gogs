// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ldap

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
)

// Sync causes this ldap source to synchronize its users with the db
func (source *Source) Sync(ctx context.Context, updateExisting bool) error {
	log.Trace("Doing: SyncExternalUsers[%s]", source.loginSource.Name)

	var existingUsers []int64
	isAttributeSSHPublicKeySet := len(strings.TrimSpace(source.AttributeSSHPublicKey)) > 0
	var sshKeysNeedUpdate bool

	// Find all users with this login type - FIXME: Should this be an iterator?
	users, err := models.GetUsersBySource(source.loginSource)
	if err != nil {
		log.Error("SyncExternalUsers: %v", err)
		return err
	}
	select {
	case <-ctx.Done():
		log.Warn("SyncExternalUsers: Cancelled before update of %s", source.loginSource.Name)
		return models.ErrCancelledf("Before update of %s", source.loginSource.Name)
	default:
	}

	sr, err := source.SearchEntries()
	if err != nil {
		log.Error("SyncExternalUsers LDAP source failure [%s], skipped", source.loginSource.Name)
		return nil
	}

	if len(sr) == 0 {
		if !source.AllowDeactivateAll {
			log.Error("LDAP search found no entries but did not report an error. Refusing to deactivate all users")
			return nil
		}
		log.Warn("LDAP search found no entries but did not report an error. All users will be deactivated as per settings")
	}

	for _, su := range sr {
		select {
		case <-ctx.Done():
			log.Warn("SyncExternalUsers: Cancelled at update of %s before completed update of users", source.loginSource.Name)
			// Rewrite authorized_keys file if LDAP Public SSH Key attribute is set and any key was added or removed
			if sshKeysNeedUpdate {
				err = models.RewriteAllPublicKeys()
				if err != nil {
					log.Error("RewriteAllPublicKeys: %v", err)
				}
			}
			return models.ErrCancelledf("During update of %s before completed update of users", source.loginSource.Name)
		default:
		}
		if len(su.Username) == 0 {
			continue
		}

		if len(su.Mail) == 0 {
			su.Mail = fmt.Sprintf("%s@localhost", su.Username)
		}

		var usr *models.User
		// Search for existing user
		for _, du := range users {
			if du.LowerName == strings.ToLower(su.Username) {
				usr = du
				break
			}
		}

		fullName := composeFullName(su.Name, su.Surname, su.Username)
		// If no existing user found, create one
		if usr == nil {
			log.Trace("SyncExternalUsers[%s]: Creating user %s", source.loginSource.Name, su.Username)

			usr = &models.User{
				LowerName:    strings.ToLower(su.Username),
				Name:         su.Username,
				FullName:     fullName,
				LoginType:    source.loginSource.Type,
				LoginSource:  source.loginSource.ID,
				LoginName:    su.Username,
				Email:        su.Mail,
				IsAdmin:      su.IsAdmin,
				IsRestricted: su.IsRestricted,
				IsActive:     true,
			}

			err = models.CreateUser(usr)

			if err != nil {
				log.Error("SyncExternalUsers[%s]: Error creating user %s: %v", source.loginSource.Name, su.Username, err)
			} else if isAttributeSSHPublicKeySet {
				log.Trace("SyncExternalUsers[%s]: Adding LDAP Public SSH Keys for user %s", source.loginSource.Name, usr.Name)
				if models.AddPublicKeysBySource(usr, source.loginSource, su.SSHPublicKey) {
					sshKeysNeedUpdate = true
				}
			}
		} else if updateExisting {
			existingUsers = append(existingUsers, usr.ID)

			// Synchronize SSH Public Key if that attribute is set
			if isAttributeSSHPublicKeySet && models.SynchronizePublicKeys(usr, source.loginSource, su.SSHPublicKey) {
				sshKeysNeedUpdate = true
			}

			// Check if user data has changed
			if (len(source.AdminFilter) > 0 && usr.IsAdmin != su.IsAdmin) ||
				(len(source.RestrictedFilter) > 0 && usr.IsRestricted != su.IsRestricted) ||
				!strings.EqualFold(usr.Email, su.Mail) ||
				usr.FullName != fullName ||
				!usr.IsActive {

				log.Trace("SyncExternalUsers[%s]: Updating user %s", source.loginSource.Name, usr.Name)

				usr.FullName = fullName
				usr.Email = su.Mail
				// Change existing admin flag only if AdminFilter option is set
				if len(source.AdminFilter) > 0 {
					usr.IsAdmin = su.IsAdmin
				}
				// Change existing restricted flag only if RestrictedFilter option is set
				if !usr.IsAdmin && len(source.RestrictedFilter) > 0 {
					usr.IsRestricted = su.IsRestricted
				}
				usr.IsActive = true

				err = models.UpdateUserCols(usr, "full_name", "email", "is_admin", "is_restricted", "is_active")
				if err != nil {
					log.Error("SyncExternalUsers[%s]: Error updating user %s: %v", source.loginSource.Name, usr.Name, err)
				}
			}
		}
	}

	// Rewrite authorized_keys file if LDAP Public SSH Key attribute is set and any key was added or removed
	if sshKeysNeedUpdate {
		err = models.RewriteAllPublicKeys()
		if err != nil {
			log.Error("RewriteAllPublicKeys: %v", err)
		}
	}

	select {
	case <-ctx.Done():
		log.Warn("SyncExternalUsers: Cancelled during update of %s before delete users", source.loginSource.Name)
		return models.ErrCancelledf("During update of %s before delete users", source.loginSource.Name)
	default:
	}

	// Deactivate users not present in LDAP
	if updateExisting {
		for _, usr := range users {
			found := false
			for _, uid := range existingUsers {
				if usr.ID == uid {
					found = true
					break
				}
			}
			if !found {
				log.Trace("SyncExternalUsers[%s]: Deactivating user %s", source.loginSource.Name, usr.Name)

				usr.IsActive = false
				err = models.UpdateUserCols(usr, "is_active")
				if err != nil {
					log.Error("SyncExternalUsers[%s]: Error deactivating user %s: %v", source.loginSource.Name, usr.Name, err)
				}
			}
		}
	}
	return nil
}
