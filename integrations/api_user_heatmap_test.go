// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.package models

package integrations

import (
	"fmt"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"

	"github.com/stretchr/testify/assert"
)

func TestUserHeatmap(t *testing.T) {
	defer prepareTestEnv(t)()
	adminUsername := "user1"
	normalUsername := "user2"
	session := loginUser(t, adminUsername)

	urlStr := fmt.Sprintf("/api/v1/users/%s/heatmap", normalUsername)
	req := NewRequest(t, "GET", urlStr)
	resp := session.MakeRequest(t, req, http.StatusOK)
	var heatmap []*models.UserHeatmapData
	DecodeJSON(t, resp, &heatmap)
	var dummyheatmap []*models.UserHeatmapData
	dummyheatmap = append(dummyheatmap, &models.UserHeatmapData{Timestamp: 1603227600, Contributions: 1})

	assert.Equal(t, dummyheatmap, heatmap)
}
