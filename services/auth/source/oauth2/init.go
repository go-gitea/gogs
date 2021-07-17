// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package oauth2

import (
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/google/uuid"
	"github.com/markbates/goth/gothic"
)

// SessionTableName is the table name that OAuth2 will use to store things
const SessionTableName = "oauth2_session"

// UsersStoreKey is the key for the store
const UsersStoreKey = "gitea-oauth2-sessions"

// ProviderHeaderKey is the HTTP header key
const ProviderHeaderKey = "gitea-oauth2-provider"

// Init initializes the oauth source
func Init() error {
	if err := InitSigningKey(); err != nil {
		return err
	}

	store, err := models.CreateStore(SessionTableName, UsersStoreKey)
	if err != nil {
		return err
	}

	// according to the Goth lib:
	// set the maxLength of the cookies stored on the disk to a larger number to prevent issues with:
	// securecookie: the value is too long
	// when using OpenID Connect , since this can contain a large amount of extra information in the id_token

	// Note, when using the FilesystemStore only the session.ID is written to a browser cookie, so this is explicit for the storage on disk
	store.MaxLength(setting.OAuth2.MaxTokenLength)
	gothic.Store = store

	gothic.SetState = func(req *http.Request) string {
		return uuid.New().String()
	}

	gothic.GetProviderName = func(req *http.Request) (string, error) {
		return req.Header.Get(ProviderHeaderKey), nil
	}

	return initOAuth2LoginSources()
}

// ResetOAuth2 clears existing OAuth2 providers and loads them from DB
func ResetOAuth2() error {
	ClearProviders()
	return initOAuth2LoginSources()
}

// initOAuth2LoginSources is used to load and register all active OAuth2 providers
func initOAuth2LoginSources() error {
	loginSources, _ := models.GetActiveOAuth2ProviderLoginSources()
	for _, source := range loginSources {
		oauth2Source, ok := source.Cfg.(*Source)
		if !ok {
			continue
		}
		err := oauth2Source.RegisterSource()
		if err != nil {
			log.Critical("Unable to register source: %s due to Error: %v. This source will be disabled.", source.Name, err)
			source.IsActive = false
			if err = models.UpdateSource(source); err != nil {
				log.Critical("Unable to update source %s to disable it. Error: %v", err)
				return err
			}
		}
	}
	return nil
}
