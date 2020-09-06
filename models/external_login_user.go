// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"time"

	"code.gitea.io/gitea/modules/structs"

	"github.com/markbates/goth"
	"xorm.io/builder"
)

// ExternalLoginUser makes the connecting between some existing user and additional external login sources
type ExternalLoginUser struct {
	ExternalID        string                 `xorm:"pk NOT NULL"`
	UserID            int64                  `xorm:"INDEX NOT NULL"`
	LoginSourceID     int64                  `xorm:"pk NOT NULL"`
	RawData           map[string]interface{} `xorm:"TEXT JSON"`
	Provider          string                 `xorm:"index VARCHAR(25)"`
	Email             string
	Name              string
	FirstName         string
	LastName          string
	NickName          string
	Description       string
	AvatarURL         string
	Location          string
	AccessToken       string `xorm:"TEXT"`
	AccessTokenSecret string `xorm:"TEXT"`
	RefreshToken      string `xorm:"TEXT"`
	ExpiresAt         time.Time
}

// TableName sets the table name to `external_login_user`
func (e *ExternalLoginUser) TableName() string {
	return tbExternalLoginUser[1 : len(tbExternalLoginUser)-1]
}

// GetExternalLogin checks if a externalID in loginSourceID scope already exists
func GetExternalLogin(externalLoginUser *ExternalLoginUser) (bool, error) {
	return x.Get(externalLoginUser)
}

// ListAccountLinks returns a map with the ExternalLoginUser and its LoginSource
func ListAccountLinks(user *User) ([]*ExternalLoginUser, error) {
	externalAccounts := make([]*ExternalLoginUser, 0, 5)
	err := x.Where("user_id=?", user.ID).
		Desc("login_source_id").
		Find(&externalAccounts)

	if err != nil {
		return nil, err
	}

	return externalAccounts, nil
}

// LinkExternalToUser link the external user to the user
func LinkExternalToUser(user *User, externalLoginUser *ExternalLoginUser) error {
	has, err := x.Where("external_id=? AND login_source_id=?", externalLoginUser.ExternalID, externalLoginUser.LoginSourceID).
		NoAutoCondition().
		Exist(externalLoginUser)
	if err != nil {
		return err
	} else if has {
		return ErrExternalLoginUserAlreadyExist{externalLoginUser.ExternalID, user.ID, externalLoginUser.LoginSourceID}
	}

	_, err = x.Insert(externalLoginUser)
	return err
}

// RemoveAccountLink will remove all external login sources for the given user
func RemoveAccountLink(user *User, loginSourceID int64) (int64, error) {
	deleted, err := x.Delete(&ExternalLoginUser{UserID: user.ID, LoginSourceID: loginSourceID})
	if err != nil {
		return deleted, err
	}
	if deleted < 1 {
		return deleted, ErrExternalLoginUserNotExist{user.ID, loginSourceID}
	}
	return deleted, err
}

// removeAllAccountLinks will remove all external login sources for the given user
func removeAllAccountLinks(e Engine, user *User) error {
	_, err := e.Delete(&ExternalLoginUser{UserID: user.ID})
	return err
}

// GetUserIDByExternalUserID get user id according to provider and userID
func GetUserIDByExternalUserID(provider string, userID string) (int64, error) {
	var id int64
	_, err := x.Table("external_login_user").
		Select("user_id").
		Where("provider=?", provider).
		And("external_id=?", userID).
		Get(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

// UpdateExternalUser updates external user's information
func UpdateExternalUser(user *User, gothUser goth.User) error {
	loginSource, err := GetActiveOAuth2LoginSourceByName(gothUser.Provider)
	if err != nil {
		return err
	}
	externalLoginUser := &ExternalLoginUser{
		ExternalID:        gothUser.UserID,
		UserID:            user.ID,
		LoginSourceID:     loginSource.ID,
		RawData:           gothUser.RawData,
		Provider:          gothUser.Provider,
		Email:             gothUser.Email,
		Name:              gothUser.Name,
		FirstName:         gothUser.FirstName,
		LastName:          gothUser.LastName,
		NickName:          gothUser.NickName,
		Description:       gothUser.Description,
		AvatarURL:         gothUser.AvatarURL,
		Location:          gothUser.Location,
		AccessToken:       gothUser.AccessToken,
		AccessTokenSecret: gothUser.AccessTokenSecret,
		RefreshToken:      gothUser.RefreshToken,
		ExpiresAt:         gothUser.ExpiresAt,
	}

	has, err := x.Where("external_id=? AND login_source_id=?", gothUser.UserID, loginSource.ID).
		NoAutoCondition().
		Exist(externalLoginUser)
	if err != nil {
		return err
	} else if !has {
		return ErrExternalLoginUserNotExist{user.ID, loginSource.ID}
	}

	_, err = x.Where("external_id=? AND login_source_id=?", gothUser.UserID, loginSource.ID).AllCols().Update(externalLoginUser)
	return err
}

// FindExternalUserOptions represents an options to find external users
type FindExternalUserOptions struct {
	Provider string
	Limit    int
	Start    int
}

func (opts FindExternalUserOptions) toConds() builder.Cond {
	var cond = builder.NewCond()
	if len(opts.Provider) > 0 {
		cond = cond.And(builder.Eq{"provider": opts.Provider})
	}
	return cond
}

// FindExternalUsersByProvider represents external users via provider
func FindExternalUsersByProvider(opts FindExternalUserOptions) ([]ExternalLoginUser, error) {
	var users []ExternalLoginUser
	err := x.Where(opts.toConds()).
		Limit(opts.Limit, opts.Start).
		OrderBy("login_source_id ASC, external_id ASC").
		Find(&users)
	if err != nil {
		return nil, err
	}
	return users, nil
}

// UpdateMigrationsByType updates all migrated repositories' posterid from gitServiceType to replace originalAuthorID to posterID
func UpdateMigrationsByType(tp structs.GitServiceType, externalUserID string, userID int64) error {
	if err := UpdateIssuesMigrationsByType(tp, externalUserID, userID); err != nil {
		return err
	}

	if err := UpdateCommentsMigrationsByType(tp, externalUserID, userID); err != nil {
		return err
	}

	if err := UpdateReleasesMigrationsByType(tp, externalUserID, userID); err != nil {
		return err
	}

	if err := UpdateReactionsMigrationsByType(tp, externalUserID, userID); err != nil {
		return err
	}
	return UpdateReviewsMigrationsByType(tp, externalUserID, userID)
}
