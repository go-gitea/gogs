// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"net/url"
	"path"
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/unknwon/i18n"
)

func testCreateBranch(t testing.TB, session *TestSession, user, repo, oldRefSubURL, newBranchName string, expectedStatus int) string {
	var csrf string
	if expectedStatus == http.StatusNotFound {
		csrf = GetCSRF(t, session, path.Join(user, repo, "src/branch/master"))
	} else {
		csrf = GetCSRF(t, session, path.Join(user, repo, "src", oldRefSubURL))
	}
	req := NewRequestWithValues(t, "POST", path.Join(user, repo, "branches/_new", oldRefSubURL), map[string]string{
		"_csrf":           csrf,
		"new_branch_name": newBranchName,
	})
	resp := session.MakeRequest(t, req, expectedStatus)
	if expectedStatus != http.StatusFound {
		return ""
	}
	return test.RedirectURL(resp)
}

func TestCreateBranch(t *testing.T) {
	onGiteaRun(t, testCreateBranches)
}

func testCreateBranches(t *testing.T, giteaURL *url.URL) {
	tests := []struct {
		OldRefSubURL   string
		NewBranch      string
		CreateRelease  string
		FlashMessage   string
		ExpectedStatus int
	}{
		{
			OldRefSubURL:   "branch/master",
			NewBranch:      "feature/test1",
			ExpectedStatus: http.StatusFound,
			FlashMessage:   i18n.Tr("en", "repo.branch.create_success", "feature/test1"),
		},
		{
			OldRefSubURL:   "branch/master",
			NewBranch:      "",
			ExpectedStatus: http.StatusFound,
			FlashMessage:   i18n.Tr("en", "form.NewBranchName") + i18n.Tr("en", "form.require_error"),
		},
		{
			OldRefSubURL:   "branch/master",
			NewBranch:      "feature=test1",
			ExpectedStatus: http.StatusFound,
			FlashMessage:   i18n.Tr("en", "repo.branch.create_success", "feature=test1"),
		},
		{
			OldRefSubURL:   "branch/master",
			NewBranch:      strings.Repeat("b", 101),
			ExpectedStatus: http.StatusFound,
			FlashMessage:   i18n.Tr("en", "form.NewBranchName") + i18n.Tr("en", "form.max_size_error", "100"),
		},
		{
			OldRefSubURL:   "branch/master",
			NewBranch:      "master",
			ExpectedStatus: http.StatusFound,
			FlashMessage:   i18n.Tr("en", "repo.branch.branch_already_exists", "master"),
		},
		{
			OldRefSubURL:   "branch/master",
			NewBranch:      "master/test",
			ExpectedStatus: http.StatusFound,
			FlashMessage:   i18n.Tr("en", "repo.branch.branch_name_conflict", "master/test", "master"),
		},
		{
			OldRefSubURL:   "commit/acd1d892867872cb47f3993468605b8aa59aa2e0",
			NewBranch:      "feature/test2",
			ExpectedStatus: http.StatusNotFound,
		},
		{
			OldRefSubURL:   "commit/65f1bf27bc3bf70f64657658635e66094edbcb4d",
			NewBranch:      "feature/test3",
			ExpectedStatus: http.StatusFound,
			FlashMessage:   i18n.Tr("en", "repo.branch.create_success", "feature/test3"),
		},
		{
			OldRefSubURL:   "branch/master",
			NewBranch:      "v1.0.0",
			CreateRelease:  "v1.0.0",
			ExpectedStatus: http.StatusFound,
			FlashMessage:   i18n.Tr("en", "repo.branch.tag_collision", "v1.0.0"),
		},
		{
			OldRefSubURL:   "tag/v1.0.0",
			NewBranch:      "feature/test4",
			CreateRelease:  "v1.0.1",
			ExpectedStatus: http.StatusFound,
			FlashMessage:   i18n.Tr("en", "repo.branch.create_success", "feature/test4"),
		},
	}
	for _, test := range tests {
		session := loginUser(t, "user2")
		if test.CreateRelease != "" {
			createNewRelease(t, session, "/user2/repo1", test.CreateRelease, test.CreateRelease, false, false)
		}
		redirectURL := testCreateBranch(t, session, "user2", "repo1", test.OldRefSubURL, test.NewBranch, test.ExpectedStatus)
		if test.ExpectedStatus == http.StatusFound {
			req := NewRequest(t, "GET", redirectURL)
			resp := session.MakeRequest(t, req, http.StatusOK)
			htmlDoc := NewHTMLParser(t, resp.Body)
			assert.Equal(t,
				test.FlashMessage,
				strings.TrimSpace(htmlDoc.doc.Find(".ui.message").Text()),
			)
		}
	}
}

func TestCreateBranchInvalidCSRF(t *testing.T) {
	defer prepareTestEnv(t)()
	session := loginUser(t, "user2")
	req := NewRequestWithValues(t, "POST", "user2/repo1/branches/_new/branch/master", map[string]string{
		"_csrf":           "fake_csrf",
		"new_branch_name": "test",
	})
	resp := session.MakeRequest(t, req, http.StatusFound)
	loc := resp.Header().Get("Location")
	assert.Equal(t, setting.AppSubURL+"/", loc)
	resp = session.MakeRequest(t, NewRequest(t, "GET", loc), http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	assert.Equal(t,
		"Bad Request: Invalid CSRF token",
		strings.TrimSpace(htmlDoc.doc.Find(".ui.message").Text()),
	)
}
