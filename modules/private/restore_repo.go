// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package private

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"code.gitea.io/gitea/modules/setting"
	jsoniter "github.com/json-iterator/go"
)

// RestoreParams structure holds a data for restore repository
type RestoreParams struct {
	RepoDir   string
	OwnerName string
	RepoName  string
	Units     []string
}

// RestoreRepo calls the internal RestoreRepo function
func RestoreRepo(ctx context.Context, repoDir, ownerName, repoName string, units []string) (int, string) {
	reqURL := setting.LocalURL + "api/internal/restore_repo"

	req := newInternalRequest(ctx, reqURL, "POST")
	req.SetTimeout(3*time.Second, 0) // since the request will spend much time, don't timeout
	req = req.Header("Content-Type", "application/json")
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	jsonBytes, _ := json.Marshal(RestoreParams{
		RepoDir:   repoDir,
		OwnerName: ownerName,
		RepoName:  repoName,
		Units:     units,
	})
	req.Body(jsonBytes)
	resp, err := req.Response()
	if err != nil {
		return http.StatusInternalServerError, fmt.Sprintf("Unable to contact gitea: %v, could you confirm it's running?", err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		var ret = struct {
			Err string `json:"err"`
		}{}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return http.StatusInternalServerError, fmt.Sprintf("Response body error: %v", err.Error())
		}
		if err := json.Unmarshal(body, &ret); err != nil {
			return http.StatusInternalServerError, fmt.Sprintf("Response body Unmarshal error: %v", err.Error())
		}
	}

	return http.StatusOK, fmt.Sprintf("Restore repo %s/%s successfully", ownerName, repoName)
}
