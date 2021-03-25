// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/indexer/issues"
	"code.gitea.io/gitea/modules/references"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
)

func getIssuesSelection(t testing.TB, htmlDoc *HTMLDoc) *goquery.Selection {
	issueList := htmlDoc.doc.Find(".issue.list")
	assert.EqualValues(t, 1, issueList.Length())
	return issueList.Find("li").Find(".title")
}

func getIssue(t *testing.T, repoID int64, issueSelection *goquery.Selection) *models.Issue {
	href, exists := issueSelection.Attr("href")
	assert.True(t, exists)
	indexStr := href[strings.LastIndexByte(href, '/')+1:]
	index, err := strconv.Atoi(indexStr)
	assert.NoError(t, err, "Invalid issue href: %s", href)
	return models.AssertExistsAndLoadBean(t, &models.Issue{RepoID: repoID, Index: int64(index)}).(*models.Issue)
}

func assertMatch(t testing.TB, issue *models.Issue, keyword string) {
	matches := strings.Contains(strings.ToLower(issue.Title), keyword) ||
		strings.Contains(strings.ToLower(issue.Content), keyword)
	for _, comment := range issue.Comments {
		matches = matches || strings.Contains(
			strings.ToLower(comment.Content),
			keyword,
		)
	}
	assert.True(t, matches)
}

func TestNoLoginViewIssues(t *testing.T) {
	defer prepareTestEnv(t)()

	req := NewRequest(t, "GET", "/user2/repo1/issues")
	MakeRequest(t, req, http.StatusOK)
}

func TestViewIssuesSortByType(t *testing.T) {
	defer prepareTestEnv(t)()

	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 1}).(*models.User)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)

	session := loginUser(t, user.Name)
	req := NewRequest(t, "GET", repo.RelLink()+"/issues?type=created_by")
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	issuesSelection := getIssuesSelection(t, htmlDoc)
	expectedNumIssues := models.GetCount(t,
		&models.Issue{RepoID: repo.ID, PosterID: user.ID},
		models.Cond("is_closed=?", false),
		models.Cond("is_pull=?", false),
	)
	if expectedNumIssues > setting.UI.IssuePagingNum {
		expectedNumIssues = setting.UI.IssuePagingNum
	}
	assert.EqualValues(t, expectedNumIssues, issuesSelection.Length())

	issuesSelection.Each(func(_ int, selection *goquery.Selection) {
		issue := getIssue(t, repo.ID, selection)
		assert.EqualValues(t, user.ID, issue.PosterID)
	})
}

func TestViewIssuesKeyword(t *testing.T) {
	defer prepareTestEnv(t)()

	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	issue := models.AssertExistsAndLoadBean(t, &models.Issue{
		RepoID: repo.ID,
		Index:  1,
	}).(*models.Issue)
	issues.UpdateIssueIndexer(issue)
	time.Sleep(time.Second * 1)
	const keyword = "first"
	req := NewRequestf(t, "GET", "%s/issues?q=%s", repo.RelLink(), keyword)
	resp := MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	issuesSelection := getIssuesSelection(t, htmlDoc)
	assert.EqualValues(t, 1, issuesSelection.Length())
	issuesSelection.Each(func(_ int, selection *goquery.Selection) {
		issue := getIssue(t, repo.ID, selection)
		assert.False(t, issue.IsClosed)
		assert.False(t, issue.IsPull)
		assertMatch(t, issue, keyword)
	})
}

func TestNoLoginViewIssue(t *testing.T) {
	defer prepareTestEnv(t)()

	req := NewRequest(t, "GET", "/user2/repo1/issues/1")
	MakeRequest(t, req, http.StatusOK)
}

func testNewIssue(t *testing.T, session *TestSession, user, repo, title, content string) string {
	req := NewRequest(t, "GET", path.Join(user, repo, "issues", "new"))
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	link, exists := htmlDoc.doc.Find("form.ui.form").Attr("action")
	assert.True(t, exists, "The template has changed")
	req = NewRequestWithValues(t, "POST", link, map[string]string{
		"_csrf":   htmlDoc.GetCSRF(),
		"title":   title,
		"content": content,
	})
	resp = session.MakeRequest(t, req, http.StatusFound)

	issueURL := test.RedirectURL(resp)
	req = NewRequest(t, "GET", issueURL)
	resp = session.MakeRequest(t, req, http.StatusOK)

	htmlDoc = NewHTMLParser(t, resp.Body)
	val := htmlDoc.doc.Find("#issue-title").Text()
	assert.Equal(t, title, val)
	val = htmlDoc.doc.Find(".comment .render-content p").First().Text()
	assert.Equal(t, content, val)

	return issueURL
}

func testIssueAddComment(t *testing.T, session *TestSession, issueURL, content, status string) int64 {
	req := NewRequest(t, "GET", issueURL)
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)
	link, exists := htmlDoc.doc.Find("#comment-form").Attr("action")
	assert.True(t, exists, "The template has changed")

	commentCount := htmlDoc.doc.Find(".comment-list .comment .render-content").Length()

	req = NewRequestWithValues(t, "POST", link, map[string]string{
		"_csrf":   htmlDoc.GetCSRF(),
		"content": content,
		"status":  status,
	})
	resp = session.MakeRequest(t, req, http.StatusFound)

	req = NewRequest(t, "GET", test.RedirectURL(resp))
	resp = session.MakeRequest(t, req, http.StatusOK)

	htmlDoc = NewHTMLParser(t, resp.Body)

	val := htmlDoc.doc.Find(".comment-list .comment .render-content p").Eq(commentCount).Text()
	assert.Equal(t, content, val)

	idAttr, has := htmlDoc.doc.Find(".comment-list .comment").Eq(commentCount).Attr("id")
	idStr := idAttr[strings.LastIndexByte(idAttr, '-')+1:]
	assert.True(t, has)
	id, err := strconv.Atoi(idStr)
	assert.NoError(t, err)
	return int64(id)
}

func TestNewIssue(t *testing.T) {
	defer prepareTestEnv(t)()
	session := loginUser(t, "user2")
	testNewIssue(t, session, "user2", "repo1", "Title", "Description")
}

func TestIssueCommentClose(t *testing.T) {
	defer prepareTestEnv(t)()
	session := loginUser(t, "user2")
	issueURL := testNewIssue(t, session, "user2", "repo1", "Title", "Description")
	testIssueAddComment(t, session, issueURL, "Test comment 1", "")
	testIssueAddComment(t, session, issueURL, "Test comment 2", "")
	testIssueAddComment(t, session, issueURL, "Test comment 3", "close")

	// Validate that issue content has not been updated
	req := NewRequest(t, "GET", issueURL)
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)
	val := htmlDoc.doc.Find(".comment-list .comment .render-content p").First().Text()
	assert.Equal(t, "Description", val)
}

func TestIssueReaction(t *testing.T) {
	defer prepareTestEnv(t)()
	session := loginUser(t, "user2")
	issueURL := testNewIssue(t, session, "user2", "repo1", "Title", "Description")

	req := NewRequest(t, "GET", issueURL)
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)

	req = NewRequestWithValues(t, "POST", path.Join(issueURL, "/reactions/react"), map[string]string{
		"_csrf":   htmlDoc.GetCSRF(),
		"content": "8ball",
	})
	session.MakeRequest(t, req, http.StatusInternalServerError)
	req = NewRequestWithValues(t, "POST", path.Join(issueURL, "/reactions/react"), map[string]string{
		"_csrf":   htmlDoc.GetCSRF(),
		"content": "eyes",
	})
	session.MakeRequest(t, req, http.StatusOK)
	req = NewRequestWithValues(t, "POST", path.Join(issueURL, "/reactions/unreact"), map[string]string{
		"_csrf":   htmlDoc.GetCSRF(),
		"content": "eyes",
	})
	session.MakeRequest(t, req, http.StatusOK)
}

func TestIssueCrossReference(t *testing.T) {
	defer prepareTestEnv(t)()

	// Issue that will be referenced
	_, issueBase := testIssueWithBean(t, "user2", 1, "Title", "Description")

	// Ref from issue title
	issueRefURL, issueRef := testIssueWithBean(t, "user2", 1, fmt.Sprintf("Title ref #%d", issueBase.Index), "Description")
	models.AssertExistsAndLoadBean(t, &models.Comment{
		IssueID:      issueBase.ID,
		RefRepoID:    1,
		RefIssueID:   issueRef.ID,
		RefCommentID: 0,
		RefIsPull:    false,
		RefAction:    references.XRefActionNone,
	})

	// Edit title, neuter ref
	testIssueChangeInfo(t, "user2", issueRefURL, "title", "Title no ref")
	models.AssertExistsAndLoadBean(t, &models.Comment{
		IssueID:      issueBase.ID,
		RefRepoID:    1,
		RefIssueID:   issueRef.ID,
		RefCommentID: 0,
		RefIsPull:    false,
		RefAction:    references.XRefActionNeutered,
	})

	// Ref from issue content
	issueRefURL, issueRef = testIssueWithBean(t, "user2", 1, "TitleXRef", fmt.Sprintf("Description ref #%d", issueBase.Index))
	models.AssertExistsAndLoadBean(t, &models.Comment{
		IssueID:      issueBase.ID,
		RefRepoID:    1,
		RefIssueID:   issueRef.ID,
		RefCommentID: 0,
		RefIsPull:    false,
		RefAction:    references.XRefActionNone,
	})

	// Edit content, neuter ref
	testIssueChangeInfo(t, "user2", issueRefURL, "content", "Description no ref")
	models.AssertExistsAndLoadBean(t, &models.Comment{
		IssueID:      issueBase.ID,
		RefRepoID:    1,
		RefIssueID:   issueRef.ID,
		RefCommentID: 0,
		RefIsPull:    false,
		RefAction:    references.XRefActionNeutered,
	})

	// Ref from a comment
	session := loginUser(t, "user2")
	commentID := testIssueAddComment(t, session, issueRefURL, fmt.Sprintf("Adding ref from comment #%d", issueBase.Index), "")
	comment := &models.Comment{
		IssueID:      issueBase.ID,
		RefRepoID:    1,
		RefIssueID:   issueRef.ID,
		RefCommentID: commentID,
		RefIsPull:    false,
		RefAction:    references.XRefActionNone,
	}
	models.AssertExistsAndLoadBean(t, comment)

	// Ref from a different repository
	issueRefURL, issueRef = testIssueWithBean(t, "user12", 10, "TitleXRef", fmt.Sprintf("Description ref user2/repo1#%d", issueBase.Index))
	models.AssertExistsAndLoadBean(t, &models.Comment{
		IssueID:      issueBase.ID,
		RefRepoID:    10,
		RefIssueID:   issueRef.ID,
		RefCommentID: 0,
		RefIsPull:    false,
		RefAction:    references.XRefActionNone,
	})
}

func testIssueWithBean(t *testing.T, user string, repoID int64, title, content string) (string, *models.Issue) {
	session := loginUser(t, user)
	issueURL := testNewIssue(t, session, user, fmt.Sprintf("repo%d", repoID), title, content)
	indexStr := issueURL[strings.LastIndexByte(issueURL, '/')+1:]
	index, err := strconv.Atoi(indexStr)
	assert.NoError(t, err, "Invalid issue href: %s", issueURL)
	issue := &models.Issue{RepoID: repoID, Index: int64(index)}
	models.AssertExistsAndLoadBean(t, issue)
	return issueURL, issue
}

func testIssueChangeInfo(t *testing.T, user, issueURL, info, value string) {
	session := loginUser(t, user)

	req := NewRequest(t, "GET", issueURL)
	resp := session.MakeRequest(t, req, http.StatusOK)
	htmlDoc := NewHTMLParser(t, resp.Body)

	req = NewRequestWithValues(t, "POST", path.Join(issueURL, info), map[string]string{
		"_csrf": htmlDoc.GetCSRF(),
		info:    value,
	})
	_ = session.MakeRequest(t, req, http.StatusOK)
}

func TestIssueRedirect(t *testing.T) {
	defer prepareTestEnv(t)()
	session := loginUser(t, "user2")

	// Test external tracker where style not set (shall default numeric)
	req := NewRequest(t, "GET", path.Join("org26", "repo_external_tracker", "issues", "1"))
	resp := session.MakeRequest(t, req, http.StatusFound)
	assert.Equal(t, "https://tracker.com/org26/repo_external_tracker/issues/1", test.RedirectURL(resp))

	// Test external tracker with numeric style
	req = NewRequest(t, "GET", path.Join("org26", "repo_external_tracker_numeric", "issues", "1"))
	resp = session.MakeRequest(t, req, http.StatusFound)
	assert.Equal(t, "https://tracker.com/org26/repo_external_tracker_numeric/issues/1", test.RedirectURL(resp))

	// Test external tracker with alphanumeric style (for a pull request)
	req = NewRequest(t, "GET", path.Join("org26", "repo_external_tracker_alpha", "issues", "1"))
	resp = session.MakeRequest(t, req, http.StatusFound)
	assert.Equal(t, "/"+path.Join("org26", "repo_external_tracker_alpha", "pulls", "1"), test.RedirectURL(resp))
}
