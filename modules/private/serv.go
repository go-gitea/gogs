// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package private

import (
	"fmt"
	"net/http"
	"net/url"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/setting"
	jsoniter "github.com/json-iterator/go"
)

// KeyAndOwner is the response from ServNoCommand
type KeyAndOwner struct {
	Key   *models.PublicKey `json:"key"`
	Owner *models.User      `json:"user"`
}

// ServNoCommand returns information about the provided key
func ServNoCommand(keyID int64) (*models.PublicKey, *models.User, error) {
	reqURL := setting.LocalURL + fmt.Sprintf("api/internal/serv/none/%d",
		keyID)
	resp, err := newInternalRequest(reqURL, "GET").Response()
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("%s", decodeJSONError(resp).Err)
	}

	var keyAndOwner KeyAndOwner
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.NewDecoder(resp.Body).Decode(&keyAndOwner); err != nil {
		return nil, nil, err
	}
	return keyAndOwner.Key, keyAndOwner.Owner, nil
}

// ServCommandResults are the results of a call to the private route serv
type ServCommandResults struct {
	IsWiki      bool
	IsDeployKey bool
	KeyID       int64
	KeyName     string
	UserName    string
	UserEmail   string
	UserID      int64
	OwnerName   string
	RepoName    string
	RepoID      int64
}

// ErrServCommand is an error returned from ServCommmand.
type ErrServCommand struct {
	Results    ServCommandResults
	Err        string
	StatusCode int
}

func (err ErrServCommand) Error() string {
	return err.Err
}

// IsErrServCommand checks if an error is a ErrServCommand.
func IsErrServCommand(err error) bool {
	_, ok := err.(ErrServCommand)
	return ok
}

// ServCommand preps for a serv call
func ServCommand(keyID int64, ownerName, repoName string, mode models.AccessMode, verbs ...string) (*ServCommandResults, error) {
	reqURL := setting.LocalURL + fmt.Sprintf("api/internal/serv/command/%d/%s/%s?mode=%d",
		keyID,
		url.PathEscape(ownerName),
		url.PathEscape(repoName),
		mode)
	for _, verb := range verbs {
		if verb != "" {
			reqURL += fmt.Sprintf("&verb=%s", url.QueryEscape(verb))
		}
	}

	resp, err := newInternalRequest(reqURL, "GET").Response()
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	if resp.StatusCode != http.StatusOK {
		var errServCommand ErrServCommand
		if err := json.NewDecoder(resp.Body).Decode(&errServCommand); err != nil {
			return nil, err
		}
		errServCommand.StatusCode = resp.StatusCode
		return nil, errServCommand
	}
	var results ServCommandResults
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, err
	}
	return &results, nil

}
