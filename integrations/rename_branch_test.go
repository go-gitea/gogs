// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	"github.com/stretchr/testify/assert"
)

func TestRenameBranch(t *testing.T) {
	// get branch setting page
	session := loginUser(t, "user2")
	req := NewRequest(t, "GET", "/user2/repo1/settings/branches")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)

	postData := map[string]string{
		"_csrf": htmlDoc.GetCSRF(),
		"from":  "master",
		"to":    "main",
	}
	req = NewRequestWithValues(t, "POST", "/user2/repo1/settings/rename_branch", postData)
	session.MakeRequest(t, req, http.StatusFound)

	// check new branch link
	req = NewRequestWithValues(t, "GET", "/user2/repo1/src/branch/main/README.md", postData)
	session.MakeRequest(t, req, http.StatusOK)

	// check old branch link
	req = NewRequestWithValues(t, "GET", "/user2/repo1/src/branch/master/README.md", postData)
	resp = session.MakeRequest(t, req, http.StatusFound)
	location := resp.HeaderMap.Get("Location")
	assert.Equal(t, "/user2/repo1/src/branch/main/README.md", location)

	// check db
	repo1 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	assert.Equal(t, "main", repo1.DefaultBranch)
}
