// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/unknwon/i18n"
)

func createNewRelease(t *testing.T, session *TestSession, repoURL, tag, title string, preRelease, draft bool) {
	req := NewRequest(t, "GET", repoURL+"/releases/new")
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)

	link, exists := htmlDoc.doc.Find("form.ui.form").Attr("action")
	assert.True(t, exists, "The template has changed")

	postData := map[string]string{
		"_csrf":      htmlDoc.GetCSRF(),
		"tag_name":   tag,
		"tag_target": "master",
		"title":      title,
		"content":    "",
	}
	if preRelease {
		postData["prerelease"] = "on"
	}
	if draft {
		postData["draft"] = "Save Draft"
	}
	req = NewRequestWithValues(t, "POST", link, postData)

	resp = session.MakeRequest(t, req, http.StatusFound)

	test.RedirectURL(resp) // check that redirect URL exists
}

func checkLatestReleaseAndCount(t *testing.T, session *TestSession, repoURL, version, label string, count int) {
	req := NewRequest(t, "GET", repoURL+"/releases")
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	labelText := htmlDoc.doc.Find("#release-list > li .meta .label").First().Text()
	assert.EqualValues(t, label, labelText)
	titleText := htmlDoc.doc.Find("#release-list > li .detail h4 a").First().Text()
	assert.EqualValues(t, version, titleText)

	releaseList := htmlDoc.doc.Find("#release-list > li")
	assert.EqualValues(t, count, releaseList.Length())
}

func TestViewReleases(t *testing.T) {
	defer prepareTestEnv(t)()

	session := loginUser(t, "user2")
	req := NewRequest(t, "GET", "/user2/repo1/releases")
	session.MakeRequest(t, req, http.StatusOK)

	// if CI is to slow this test fail, so lets wait a bit
	time.Sleep(time.Millisecond * 100)
}

func TestViewReleasesNoLogin(t *testing.T) {
	defer prepareTestEnv(t)()

	req := NewRequest(t, "GET", "/user2/repo1/releases")
	MakeRequest(t, req, http.StatusOK)
}

func TestCreateRelease(t *testing.T) {
	defer prepareTestEnv(t)()

	session := loginUser(t, "user2")
	createNewRelease(t, session, "/user2/repo1", "v0.0.1", "v0.0.1", false, false)

	checkLatestReleaseAndCount(t, session, "/user2/repo1", "v0.0.1", i18n.Tr("en", "repo.release.stable"), 4)
}

func TestCreateReleasePreRelease(t *testing.T) {
	defer prepareTestEnv(t)()

	session := loginUser(t, "user2")
	createNewRelease(t, session, "/user2/repo1", "v0.0.1", "v0.0.1", true, false)

	checkLatestReleaseAndCount(t, session, "/user2/repo1", "v0.0.1", i18n.Tr("en", "repo.release.prerelease"), 4)
}

func TestCreateReleaseDraft(t *testing.T) {
	defer prepareTestEnv(t)()

	session := loginUser(t, "user2")
	createNewRelease(t, session, "/user2/repo1", "v0.0.1", "v0.0.1", false, true)

	checkLatestReleaseAndCount(t, session, "/user2/repo1", "v0.0.1", i18n.Tr("en", "repo.release.draft"), 4)
}

func TestCreateReleasePaging(t *testing.T) {
	defer prepareTestEnv(t)()

	oldAPIDefaultNum := setting.API.DefaultPagingNum
	defer func() {
		setting.API.DefaultPagingNum = oldAPIDefaultNum
	}()
	setting.API.DefaultPagingNum = 10

	session := loginUser(t, "user2")
	// Create enough releases to have paging
	for i := 0; i < 12; i++ {
		version := fmt.Sprintf("v0.0.%d", i)
		createNewRelease(t, session, "/user2/repo1", version, version, false, false)
	}
	createNewRelease(t, session, "/user2/repo1", "v0.0.12", "v0.0.12", false, true)

	checkLatestReleaseAndCount(t, session, "/user2/repo1", "v0.0.12", i18n.Tr("en", "repo.release.draft"), 10)

	// Check that user4 does not see draft and still see 10 latest releases
	session2 := loginUser(t, "user4")
	checkLatestReleaseAndCount(t, session2, "/user2/repo1", "v0.0.11", i18n.Tr("en", "repo.release.stable"), 10)
}

func TestViewReleaseListNoLogin(t *testing.T) {
	defer prepareTestEnv(t)()

	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)

	link := repo.Link() + "/releases"

	req := NewRequest(t, "GET", link)
	rsp := MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, rsp.Body)
	releases := htmlDoc.Find("#release-list li.ui.grid")
	assert.Equal(t, 2, releases.Length())

	links := make([]string, 0, 5)
	releases.Each(func(i int, s *goquery.Selection) {
		link, exist := s.Find(".release-list-title a").Attr("href")
		if !exist {
			return
		}
		links = append(links, link)
	})

	assert.EqualValues(t, []string{"/user2/repo1/releases/tag/v1.0", "/user2/repo1/releases/tag/v1.1"}, links)
}

func TestViewReleaseListLogin(t *testing.T) {
	defer prepareTestEnv(t)()

	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)

	link := repo.Link() + "/releases"

	session := loginUser(t, "user1")
	req := NewRequest(t, "GET", link)
	rsp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, rsp.Body)
	releases := htmlDoc.Find("#release-list li.ui.grid")
	assert.Equal(t, 3, releases.Length())

	links := make([]string, 0, 5)
	releases.Each(func(i int, s *goquery.Selection) {
		link, exist := s.Find(".release-list-title a").Attr("href")
		if !exist {
			return
		}
		links = append(links, link)
	})

	assert.EqualValues(t, []string{
		"/user2/repo1/releases/tag/draft-release",
		"/user2/repo1/releases/tag/v1.0",
		"/user2/repo1/releases/tag/v1.1",
	}, links)
}

func TestViewTagsList(t *testing.T) {
	defer prepareTestEnv(t)()

	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)

	link := repo.Link() + "/tags"

	session := loginUser(t, "user1")
	req := NewRequest(t, "GET", link)
	rsp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, rsp.Body)
	tags := htmlDoc.Find(".tag-list tr")
	assert.Equal(t, 3, tags.Length())

	tagNames := make([]string, 0, 5)
	tags.Each(func(i int, s *goquery.Selection) {
		tagNames = append(tagNames, s.Find(".tag a.df.ac").Text())
	})

	assert.EqualValues(t, []string{"v1.0", "delete-tag", "v1.1"}, tagNames)
}
