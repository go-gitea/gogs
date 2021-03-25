// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestAPIIssueSubscriptions(t *testing.T) {
	defer prepareTestEnv(t)()

	issue1 := models.AssertExistsAndLoadBean(t, &models.Issue{ID: 1}).(*models.Issue)
	issue2 := models.AssertExistsAndLoadBean(t, &models.Issue{ID: 2}).(*models.Issue)
	issue3 := models.AssertExistsAndLoadBean(t, &models.Issue{ID: 3}).(*models.Issue)
	issue4 := models.AssertExistsAndLoadBean(t, &models.Issue{ID: 4}).(*models.Issue)
	issue5 := models.AssertExistsAndLoadBean(t, &models.Issue{ID: 8}).(*models.Issue)

	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: issue1.PosterID}).(*models.User)

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session)

	testSubscription := func(issue *models.Issue, isWatching bool) {
		issueRepo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: issue.RepoID}).(*models.Repository)

		urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/subscriptions/check?token=%s", issueRepo.OwnerName, issueRepo.Name, issue.Index, token)
		req := NewRequest(t, "GET", urlStr)
		resp := session.MakeRequest(t, req, http.StatusOK)
		wi := new(api.WatchInfo)
		DecodeJSON(t, resp, wi)

		assert.EqualValues(t, isWatching, wi.Subscribed)
		assert.EqualValues(t, !isWatching, wi.Ignored)
		assert.EqualValues(t, issue.APIURL()+"/subscriptions", wi.URL)
		assert.EqualValues(t, issue.CreatedUnix, wi.CreatedAt.Unix())
		assert.EqualValues(t, issueRepo.APIURL(), wi.RepositoryURL)
	}

	testSubscription(issue1, true)
	testSubscription(issue2, true)
	testSubscription(issue3, true)
	testSubscription(issue4, false)
	testSubscription(issue5, false)

	issue1Repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: issue1.RepoID}).(*models.Repository)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/subscriptions/%s?token=%s", issue1Repo.OwnerName, issue1Repo.Name, issue1.Index, owner.Name, token)
	req := NewRequest(t, "DELETE", urlStr)
	session.MakeRequest(t, req, http.StatusCreated)
	testSubscription(issue1, false)

	req = NewRequest(t, "DELETE", urlStr)
	session.MakeRequest(t, req, http.StatusOK)
	testSubscription(issue1, false)

	issue5Repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: issue5.RepoID}).(*models.Repository)
	urlStr = fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/subscriptions/%s?token=%s", issue5Repo.OwnerName, issue5Repo.Name, issue5.Index, owner.Name, token)
	req = NewRequest(t, "PUT", urlStr)
	session.MakeRequest(t, req, http.StatusCreated)
	testSubscription(issue5, true)

	req = NewRequest(t, "PUT", urlStr)
	session.MakeRequest(t, req, http.StatusOK)
	testSubscription(issue5, true)
}
