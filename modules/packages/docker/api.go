// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package docker

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// API docker api port
type API struct {
	APIBasePath string
	Token       string
	TimeOut     time.Duration
	Ctx         context.Context
}

// ListImageTags Listing Image Tags
// GET /v2/<name>/tags/list
func (a *API) ListImageTags(name string) (*TagsAPIResponse, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", a.APIBasePath+"/v2/"+name+"/tags/list", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+a.Token)

	ctx, cancel := context.WithTimeout(a.Ctx, a.TimeOut)
	defer cancel()

	req = req.WithContext(ctx)

	rsp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close()

	v := new(TagsAPIResponse)
	decoder := json.NewDecoder(rsp.Body)
	if err = decoder.Decode(v); err != nil {
		return nil, err
	}

	return v, nil
}

// TagsAPIResponse list tags response
type TagsAPIResponse struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}
