// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

func testSrcRouteRedirect(t *testing.T, session *TestSession, user, repo, route, expectedLocation string, expectedStatus int) {
	prefix := path.Join("/", user, repo, "src")

	// Make request
	req := NewRequest(t, "GET", path.Join(prefix, route))
	resp := session.MakeRequest(t, req, http.StatusFound)

	// Check Location header
	location := resp.HeaderMap.Get("Location")
	assert.Equal(t, path.Join(prefix, expectedLocation), location)

	// Perform redirect
	req = NewRequest(t, "GET", location)
	resp = session.MakeRequest(t, req, expectedStatus)
}

func setDefaultBranch(t *testing.T, session *TestSession, user, repo, branch string) {
	location := path.Join("/", user, repo, "settings/branches")
	csrf := GetCSRF(t, session, location)
	req := NewRequestWithValues(t, "POST", location, map[string]string{
		"_csrf":  csrf,
		"action": "default_branch",
		"branch": branch,
	})
	session.MakeRequest(t, req, http.StatusFound)
}

func TestNonasciiBranches(t *testing.T) {
	testRedirects := []struct {
		from   string
		to     string
		status int
	}{
		// Branches
		{
			from:   "master",
			to:     "branch/master",
			status: http.StatusOK,
		},
		{
			from:   "master/README.md",
			to:     "branch/master/README.md",
			status: http.StatusOK,
		},
		{
			from:   "master/badfile",
			to:     "branch/master/badfile",
			status: http.StatusNotFound, // it does not exists
		},
		{
			from:   "ГлавнаяВетка",
			to:     "branch/%d0%93%d0%bb%d0%b0%d0%b2%d0%bd%d0%b0%d1%8f%d0%92%d0%b5%d1%82%d0%ba%d0%b0",
			status: http.StatusOK,
		},
		{
			from:   "а/б/в",
			to:     "branch/%d0%b0/%d0%b1/%d0%b2",
			status: http.StatusOK,
		},
		{
			from:   "Grüßen/README.md",
			to:     "branch/Gr%c3%bc%c3%9fen/README.md",
			status: http.StatusOK,
		},
		{
			from:   "Plus+Is+Not+Space",
			to:     "branch/Plus+Is+Not+Space",
			status: http.StatusOK,
		},
		{
			from:   "Plus+Is+Not+Space/Файл.md",
			to:     "branch/Plus+Is+Not+Space/%d0%a4%d0%b0%d0%b9%d0%bb.md",
			status: http.StatusOK,
		},
		{
			from:   "Plus+Is+Not+Space/and+it+is+valid.md",
			to:     "branch/Plus+Is+Not+Space/and+it+is+valid.md",
			status: http.StatusOK,
		},
		{
			from:   "ブランチ",
			to:     "branch/%e3%83%96%e3%83%a9%e3%83%b3%e3%83%81",
			status: http.StatusOK,
		},
		// Tags
		{
			from:   "Тэг",
			to:     "tag/%d0%a2%d1%8d%d0%b3",
			status: http.StatusOK,
		},
		{
			from:   "Ё/人",
			to:     "tag/%d0%81/%e4%ba%ba",
			status: http.StatusOK,
		},
		{
			from:   "タグ",
			to:     "tag/%e3%82%bf%e3%82%b0",
			status: http.StatusOK,
		},
		{
			from:   "タグ/ファイル.md",
			to:     "tag/%e3%82%bf%e3%82%b0/%e3%83%95%e3%82%a1%e3%82%a4%e3%83%ab.md",
			status: http.StatusOK,
		},
		// Files
		{
			from:   "README.md",
			to:     "branch/Plus+Is+Not+Space/README.md",
			status: http.StatusOK,
		},
		{
			from:   "Файл.md",
			to:     "branch/Plus+Is+Not+Space/%d0%a4%d0%b0%d0%b9%d0%bb.md",
			status: http.StatusOK,
		},
		{
			from:   "ファイル.md",
			to:     "branch/Plus+Is+Not+Space/%e3%83%95%e3%82%a1%e3%82%a4%e3%83%ab.md",
			status: http.StatusNotFound, // it's not on default branch
		},
		// Same but url-encoded (few tests)
		{
			from:   "%E3%83%96%E3%83%A9%E3%83%B3%E3%83%81",
			to:     "branch/%e3%83%96%e3%83%a9%e3%83%b3%e3%83%81",
			status: http.StatusOK,
		},
		{
			from:   "%E3%82%BF%E3%82%b0",
			to:     "tag/%e3%82%bf%e3%82%b0",
			status: http.StatusOK,
		},
		{
			from:   "%D0%A4%D0%B0%D0%B9%D0%BB.md",
			to:     "branch/Plus+Is+Not+Space/%d0%a4%d0%b0%d0%b9%d0%bb.md",
			status: http.StatusOK,
		},
		{
			from:   "%D0%81%2F%E4%BA%BA",
			to:     "tag/%d0%81/%e4%ba%ba",
			status: http.StatusOK,
		},
		{
			from:   "Ё%2F%E4%BA%BA",
			to:     "tag/%d0%81/%e4%ba%ba",
			status: http.StatusOK,
		},
	}

	defer prepareTestEnv(t)()

	user := "user2"
	repo := "utf8"
	session := loginUser(t, user)

	setDefaultBranch(t, session, user, repo, "Plus+Is+Not+Space")

	for _, test := range testRedirects {
		testSrcRouteRedirect(t, session, user, repo, test.from, test.to, test.status)
	}

	setDefaultBranch(t, session, user, repo, "master")
}
