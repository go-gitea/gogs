// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"path"
	"strings"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
)

func TestViewRepo(t *testing.T) {
	defer prepareTestEnv(t)()

	req := NewRequest(t, "GET", "/user2/repo1")
	MakeRequest(t, req, http.StatusOK)

	req = NewRequest(t, "GET", "/user3/repo3")
	MakeRequest(t, req, http.StatusNotFound)

	session := loginUser(t, "user1")
	session.MakeRequest(t, req, http.StatusNotFound)
}

func testViewRepo(t *testing.T) {
	defer prepareTestEnv(t)()

	req := NewRequest(t, "GET", "/user3/repo3")
	session := loginUser(t, "user2")
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	files := htmlDoc.doc.Find("#repo-files-table  > TBODY > TR")

	type file struct {
		fileName   string
		commitID   string
		commitMsg  string
		commitTime string
	}

	var items []file

	files.Each(func(i int, s *goquery.Selection) {
		tds := s.Find("td")
		var f file
		tds.Each(func(i int, s *goquery.Selection) {
			if i == 0 {
				f.fileName = strings.TrimSpace(s.Text())
			} else if i == 1 {
				a := s.Find("a")
				f.commitMsg = strings.TrimSpace(a.Text())
				l, _ := a.Attr("href")
				f.commitID = path.Base(l)
			}
		})

		f.commitTime, _ = s.Find("span.time-since").Attr("title")
		items = append(items, f)
	})

	commitT := time.Date(2017, time.June, 14, 13, 54, 21, 0, time.UTC).In(time.Local).Format(time.RFC1123)
	assert.EqualValues(t, []file{
		{
			fileName:   "doc",
			commitID:   "2a47ca4b614a9f5a43abbd5ad851a54a616ffee6",
			commitMsg:  "init project",
			commitTime: commitT,
		},
		{
			fileName:   "README.md",
			commitID:   "2a47ca4b614a9f5a43abbd5ad851a54a616ffee6",
			commitMsg:  "init project",
			commitTime: commitT,
		},
	}, items)
}

func TestViewRepo2(t *testing.T) {
	// no last commit cache
	testViewRepo(t)

	// enable last commit cache for all repositories
	oldCommitsCount := setting.CacheService.LastCommit.CommitsCount
	setting.CacheService.LastCommit.CommitsCount = 0

	ccr := &repository.CommitCacheRequest{
		Repo:     "user3/repo3",
		CommitID: "master",
		TreePath: "",
	}
	err := ccr.Do()
	assert.NoError(t, err)
	// first view should hit the cache
	testViewRepo(t)
	// second view will hit the cache
	testViewRepo(t)
	setting.CacheService.LastCommit.CommitsCount = oldCommitsCount
}

func TestViewRepo3(t *testing.T) {
	defer prepareTestEnv(t)()

	req := NewRequest(t, "GET", "/user3/repo3")
	session := loginUser(t, "user4")
	session.MakeRequest(t, req, http.StatusOK)
}

func TestViewRepo1CloneLinkAnonymous(t *testing.T) {
	defer prepareTestEnv(t)()

	req := NewRequest(t, "GET", "/user2/repo1")
	resp := MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	link, exists := htmlDoc.doc.Find("#repo-clone-https").Attr("data-link")
	assert.True(t, exists, "The template has changed")
	assert.Equal(t, setting.AppURL+"user2/repo1.git", link)
	_, exists = htmlDoc.doc.Find("#repo-clone-ssh").Attr("data-link")
	assert.False(t, exists)
}

func TestViewRepo1CloneLinkAuthorized(t *testing.T) {
	defer prepareTestEnv(t)()

	session := loginUser(t, "user2")

	req := NewRequest(t, "GET", "/user2/repo1")
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	link, exists := htmlDoc.doc.Find("#repo-clone-https").Attr("data-link")
	assert.True(t, exists, "The template has changed")
	assert.Equal(t, setting.AppURL+"user2/repo1.git", link)
	link, exists = htmlDoc.doc.Find("#repo-clone-ssh").Attr("data-link")
	assert.True(t, exists, "The template has changed")
	sshURL := fmt.Sprintf("ssh://%s@%s:%d/user2/repo1.git", setting.SSH.BuiltinServerUser, setting.SSH.Domain, setting.SSH.Port)
	assert.Equal(t, sshURL, link)
}

func TestViewRepoWithSymlinks(t *testing.T) {
	defer prepareTestEnv(t)()

	session := loginUser(t, "user2")

	req := NewRequest(t, "GET", "/user2/repo20.git")
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	files := htmlDoc.doc.Find("#repo-files-table > TBODY > TR > TD.name > SPAN.truncate")
	items := files.Map(func(i int, s *goquery.Selection) string {
		cls, _ := s.Find("SVG").Attr("class")
		file := strings.Trim(s.Find("A").Text(), " \t\n")
		return fmt.Sprintf("%s: %s", file, cls)
	})
	assert.Len(t, items, 5)
	assert.Equal(t, "a: svg octicon-file-directory", items[0])
	assert.Equal(t, "link_b: svg octicon-file-submodule", items[1])
	assert.Equal(t, "link_d: svg octicon-file-symlink-file", items[2])
	assert.Equal(t, "link_hi: svg octicon-file-symlink-file", items[3])
	assert.Equal(t, "link_link: svg octicon-file-symlink-file", items[4])
}

// TestViewAsRepoAdmin tests PR #2167
func TestViewAsRepoAdmin(t *testing.T) {
	for user, expectedNoDescription := range map[string]bool{
		"user2": true,
		"user4": false,
	} {
		defer prepareTestEnv(t)()

		session := loginUser(t, user)

		req := NewRequest(t, "GET", "/user2/repo1.git")
		resp := session.MakeRequest(t, req, http.StatusOK)

		htmlDoc := NewHTMLParser(t, resp.Body)
		noDescription := htmlDoc.doc.Find("#repo-desc").Children()

		assert.Equal(t, expectedNoDescription, noDescription.HasClass("no-description"))
	}
}
