// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"net/http/httptest"
	"path"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
)

func TestRepoCommits(t *testing.T) {
	defer prepareTestEnv(t)()

	session := loginUser(t, "user2")

	// Request repository commits page
	req := NewRequest(t, "GET", "/user2/repo1/commits/branch/master")
	resp := session.MakeRequest(t, req, http.StatusOK)

	doc := NewHTMLParser(t, resp.Body)
	commitURL, exists := doc.doc.Find("#commits-table tbody tr td.sha a").Attr("href")
	assert.True(t, exists)
	assert.NotEmpty(t, commitURL)
}

func doTestRepoCommitWithStatus(t *testing.T, state string, classes ...string) {
	defer prepareTestEnv(t)()

	session := loginUser(t, "user2")
	token := getTokenForLoggedInUser(t, session)

	// Request repository commits page
	req := NewRequest(t, "GET", "/user2/repo1/commits/branch/master")
	resp := session.MakeRequest(t, req, http.StatusOK)

	doc := NewHTMLParser(t, resp.Body)
	// Get first commit URL
	commitURL, exists := doc.doc.Find("#commits-table tbody tr td.sha a").Attr("href")
	assert.True(t, exists)
	assert.NotEmpty(t, commitURL)

	// Call API to add status for commit
	req = NewRequestWithJSON(t, "POST", "/api/v1/repos/user2/repo1/statuses/"+path.Base(commitURL)+"?token="+token,
		api.CreateStatusOption{
			State:       api.CommitStatusState(state),
			TargetURL:   "http://test.ci/",
			Description: "",
			Context:     "testci",
		},
	)

	resp = session.MakeRequest(t, req, http.StatusCreated)

	req = NewRequest(t, "GET", "/user2/repo1/commits/branch/master")
	resp = session.MakeRequest(t, req, http.StatusOK)

	doc = NewHTMLParser(t, resp.Body)
	// Check if commit status is displayed in message column
	sel := doc.doc.Find("#commits-table tbody tr td.message a.commit-statuses-trigger i.commit-status")
	assert.Equal(t, 1, sel.Length())
	for _, class := range classes {
		assert.True(t, sel.HasClass(class))
	}

	//By SHA
	req = NewRequest(t, "GET", "/api/v1/repos/user2/repo1/commits/"+path.Base(commitURL)+"/statuses")
	testRepoCommitsWithStatus(t, session.MakeRequest(t, req, http.StatusOK), state)
	//By Ref
	req = NewRequest(t, "GET", "/api/v1/repos/user2/repo1/commits/master/statuses")
	testRepoCommitsWithStatus(t, session.MakeRequest(t, req, http.StatusOK), state)
	req = NewRequest(t, "GET", "/api/v1/repos/user2/repo1/commits/v1.1/statuses")
	testRepoCommitsWithStatus(t, session.MakeRequest(t, req, http.StatusOK), state)
}

func testRepoCommitsWithStatus(t *testing.T, resp *httptest.ResponseRecorder, state string) {
	decoder := json.NewDecoder(resp.Body)
	statuses := []*api.CommitStatus{}
	assert.NoError(t, decoder.Decode(&statuses))
	assert.Len(t, statuses, 1)
	for _, s := range statuses {
		assert.Equal(t, api.CommitStatusState(state), s.State)
		assert.Equal(t, setting.AppURL+"api/v1/repos/user2/repo1/statuses/65f1bf27bc3bf70f64657658635e66094edbcb4d", s.URL)
		assert.Equal(t, "http://test.ci/", s.TargetURL)
		assert.Equal(t, "", s.Description)
		assert.Equal(t, "testci", s.Context)
	}
}

func TestRepoCommitsWithStatusPending(t *testing.T) {
	doTestRepoCommitWithStatus(t, "pending", "circle", "yellow")
}

func TestRepoCommitsWithStatusSuccess(t *testing.T) {
	doTestRepoCommitWithStatus(t, "success", "check", "green")
}

func TestRepoCommitsWithStatusError(t *testing.T) {
	doTestRepoCommitWithStatus(t, "error", "warning", "red")
}

func TestRepoCommitsWithStatusFailure(t *testing.T) {
	doTestRepoCommitWithStatus(t, "failure", "remove", "red")
}

func TestRepoCommitsWithStatusWarning(t *testing.T) {
	doTestRepoCommitWithStatus(t, "warning", "warning", "sign", "yellow")
}
