// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/services/pull"

	"github.com/stretchr/testify/assert"
	"github.com/unknwon/i18n"
)

func testPullMerge(t *testing.T, session *TestSession, user, repo, pullnum string, mergeStyle models.MergeStyle) *httptest.ResponseRecorder {
	req := NewRequest(t, "GET", path.Join(user, repo, "pulls", pullnum))
	resp := session.MakeRequest(t, req, http.StatusOK)

	// Click the little green button to create a pull
	htmlDoc := NewHTMLParser(t, resp.Body)
	link, exists := htmlDoc.doc.Find(".ui.form." + string(mergeStyle) + "-fields > form").Attr("action")
	assert.True(t, exists, "The template has changed")
	req = NewRequestWithValues(t, "POST", link, map[string]string{
		"_csrf": htmlDoc.GetCSRF(),
		"do":    string(mergeStyle),
	})
	resp = session.MakeRequest(t, req, http.StatusFound)

	return resp
}

func testPullCleanUp(t *testing.T, session *TestSession, user, repo, pullnum string) *httptest.ResponseRecorder {
	req := NewRequest(t, "GET", path.Join(user, repo, "pulls", pullnum))
	resp := session.MakeRequest(t, req, http.StatusOK)

	// Click the little green button to create a pull
	htmlDoc := NewHTMLParser(t, resp.Body)
	link, exists := htmlDoc.doc.Find(".timeline-item .delete-button").Attr("data-url")
	assert.True(t, exists, "The template has changed")
	req = NewRequestWithValues(t, "POST", link, map[string]string{
		"_csrf": htmlDoc.GetCSRF(),
	})
	resp = session.MakeRequest(t, req, http.StatusOK)

	return resp
}

func checkChannelWithTimeout(c <-chan interface{}, timeout time.Duration, callback func(interface{}) bool) bool {
	timer := time.NewTimer(500 * time.Millisecond)
	for {
		select {
		case received := <-c:
			if callback(received) {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				return true
			}
		case <-timer.C:
			return false
		}
	}
}

func TestPullMerge(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		createPullNotified, unregisterNewPull := notifierListener.RegisterChannel("NotifyNewPullRequest", 0, &models.PullRequest{})
		defer unregisterNewPull()

		mergePullNotified, unregisterMergePull := notifierListener.RegisterChannel("NotifyMergePullRequest", 0, &models.PullRequest{})
		defer unregisterMergePull()

		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1-TestPullMerge")
		testEditFile(t, session, "user1", "repo1-TestPullMerge", "master", "README.md", "Hello, World (Edited)\n")

		resp := testPullCreate(t, session, "user1", "repo1-TestPullMerge", "master", "This is a pull title")

		isOurPR := func(received interface{}) bool {
			pr := received.(*models.PullRequest)
			pr.LoadBaseRepo()
			pr.LoadHeadRepo()
			return pr.BaseRepo.FullName() == "user2/repo1" &&
				pr.BaseBranch == "master" &&
				pr.HeadRepo.FullName() == "user1/repo1-TestPullMerge" &&
				pr.HeadBranch == "master"
		}

		assert.True(t, checkChannelWithTimeout(createPullNotified, 500*time.Millisecond, isOurPR), "Failed to be notified pull created")

		elem := strings.Split(test.RedirectURL(resp), "/")
		assert.EqualValues(t, "pulls", elem[3])

		testPullMerge(t, session, elem[1], elem[2], elem[4], models.MergeStyleMerge)
		assert.True(t, checkChannelWithTimeout(mergePullNotified, 500*time.Millisecond, isOurPR), "Failed to be notified pull merged")

		assert.False(t, checkChannelWithTimeout(createPullNotified, 100*time.Millisecond, isOurPR), "Duplicate notified pull created")
		assert.False(t, checkChannelWithTimeout(mergePullNotified, 100*time.Millisecond, isOurPR), "Duplicate notified pull merged")
	})
}

func TestPullRebase(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		mergePullNotified, deferable := notifierListener.RegisterChannel("NotifyMergePullRequest", 0, &models.PullRequest{})
		defer deferable()

		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1")
		testEditFile(t, session, "user1", "repo1", "master", "README.md", "Hello, World (Edited)\n")

		resp := testPullCreate(t, session, "user1", "repo1", "master", "This is a pull title")

		elem := strings.Split(test.RedirectURL(resp), "/")
		assert.EqualValues(t, "pulls", elem[3])
		testPullMerge(t, session, elem[1], elem[2], elem[4], models.MergeStyleRebase)
		select {
		case <-mergePullNotified:
		case <-time.After(500 * time.Millisecond):
			assert.Fail(t, "Took too long to notify!")
		}
	})
}

func TestPullRebaseMerge(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		mergePullNotified, deferable := notifierListener.RegisterChannel("NotifyMergePullRequest", 0, &models.PullRequest{})
		defer deferable()

		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1")
		testEditFile(t, session, "user1", "repo1", "master", "README.md", "Hello, World (Edited)\n")

		resp := testPullCreate(t, session, "user1", "repo1", "master", "This is a pull title")

		elem := strings.Split(test.RedirectURL(resp), "/")
		assert.EqualValues(t, "pulls", elem[3])
		testPullMerge(t, session, elem[1], elem[2], elem[4], models.MergeStyleRebaseMerge)

		select {
		case <-mergePullNotified:
		case <-time.After(500 * time.Millisecond):
			assert.Fail(t, "Took too long to notify!")
		}
	})
}

func TestPullSquash(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		mergePullNotified, deferable := notifierListener.RegisterChannel("NotifyMergePullRequest", 0, &models.PullRequest{})
		defer deferable()

		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1")
		testEditFile(t, session, "user1", "repo1", "master", "README.md", "Hello, World (Edited)\n")
		testEditFile(t, session, "user1", "repo1", "master", "README.md", "Hello, World (Edited!)\n")

		resp := testPullCreate(t, session, "user1", "repo1", "master", "This is a pull title")

		elem := strings.Split(test.RedirectURL(resp), "/")
		assert.EqualValues(t, "pulls", elem[3])
		testPullMerge(t, session, elem[1], elem[2], elem[4], models.MergeStyleSquash)

		select {
		case <-mergePullNotified:
		case <-time.After(500 * time.Millisecond):
			assert.Fail(t, "Took too long to notify!")
		}
	})
}

func TestPullCleanUpAfterMerge(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1")
		testEditFileToNewBranch(t, session, "user1", "repo1", "master", "feature/test", "README.md", "Hello, World (Edited - TestPullCleanUpAfterMerge)\n")

		resp := testPullCreate(t, session, "user1", "repo1", "feature/test", "This is a pull title")

		elem := strings.Split(test.RedirectURL(resp), "/")
		assert.EqualValues(t, "pulls", elem[3])
		testPullMerge(t, session, elem[1], elem[2], elem[4], models.MergeStyleMerge)

		// Check PR branch deletion
		resp = testPullCleanUp(t, session, elem[1], elem[2], elem[4])
		respJSON := struct {
			Redirect string
		}{}
		DecodeJSON(t, resp, &respJSON)

		assert.NotEmpty(t, respJSON.Redirect, "Redirected URL is not found")

		elem = strings.Split(respJSON.Redirect, "/")
		assert.EqualValues(t, "pulls", elem[3])

		// Check branch deletion result
		req := NewRequest(t, "GET", respJSON.Redirect)
		resp = session.MakeRequest(t, req, http.StatusOK)

		htmlDoc := NewHTMLParser(t, resp.Body)
		resultMsg := htmlDoc.doc.Find(".ui.message>p").Text()

		assert.EqualValues(t, "Branch 'user1/feature/test' has been deleted.", resultMsg)
	})
}

func TestCantMergeWorkInProgress(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1")
		testEditFile(t, session, "user1", "repo1", "master", "README.md", "Hello, World (Edited)\n")

		resp := testPullCreate(t, session, "user1", "repo1", "master", "[wip] This is a pull title")

		req := NewRequest(t, "GET", resp.Header().Get("Location"))
		resp = session.MakeRequest(t, req, http.StatusOK)
		htmlDoc := NewHTMLParser(t, resp.Body)
		text := strings.TrimSpace(htmlDoc.doc.Find(".attached.header > .text.grey").Last().Text())
		assert.NotEmpty(t, text, "Can't find WIP text")

		// remove <strong /> from lang
		expected := i18n.Tr("en", "repo.pulls.cannot_merge_work_in_progress", "[wip]")
		replacer := strings.NewReplacer("<strong>", "", "</strong>", "")
		assert.Equal(t, replacer.Replace(expected), text, "Unable to find WIP text")
	})
}

func TestCantMergeConflict(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1")
		testEditFileToNewBranch(t, session, "user1", "repo1", "master", "conflict", "README.md", "Hello, World (Edited Once)\n")
		testEditFileToNewBranch(t, session, "user1", "repo1", "master", "base", "README.md", "Hello, World (Edited Twice)\n")

		// Use API to create a conflicting pr
		token := getTokenForLoggedInUser(t, session)
		req := NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls?token=%s", "user1", "repo1", token), &api.CreatePullRequestOption{
			Head:  "conflict",
			Base:  "base",
			Title: "create a conflicting pr",
		})
		session.MakeRequest(t, req, 201)

		// Now this PR will be marked conflict - or at least a race will do - so drop down to pure code at this point...
		user1 := models.AssertExistsAndLoadBean(t, &models.User{
			Name: "user1",
		}).(*models.User)
		repo1 := models.AssertExistsAndLoadBean(t, &models.Repository{
			OwnerID: user1.ID,
			Name:    "repo1",
		}).(*models.Repository)

		pr := models.AssertExistsAndLoadBean(t, &models.PullRequest{
			HeadRepoID: repo1.ID,
			BaseRepoID: repo1.ID,
			HeadBranch: "conflict",
			BaseBranch: "base",
		}).(*models.PullRequest)

		gitRepo, err := git.OpenRepository(models.RepoPath(user1.Name, repo1.Name))
		assert.NoError(t, err)

		err = pull.Merge(pr, user1, gitRepo, models.MergeStyleMerge, "CONFLICT")
		assert.Error(t, err, "Merge should return an error due to conflict")
		assert.True(t, models.IsErrMergeConflicts(err), "Merge error is not a conflict error")

		err = pull.Merge(pr, user1, gitRepo, models.MergeStyleRebase, "CONFLICT")
		assert.Error(t, err, "Merge should return an error due to conflict")
		assert.True(t, models.IsErrRebaseConflicts(err), "Merge error is not a conflict error")
	})
}

func TestCantMergeUnrelated(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, giteaURL *url.URL) {
		session := loginUser(t, "user1")
		testRepoFork(t, session, "user2", "repo1", "user1", "repo1")
		testEditFileToNewBranch(t, session, "user1", "repo1", "master", "base", "README.md", "Hello, World (Edited Twice)\n")

		// Now we want to create a commit on a branch that is totally unrelated to our current head
		// Drop down to pure code at this point
		user1 := models.AssertExistsAndLoadBean(t, &models.User{
			Name: "user1",
		}).(*models.User)
		repo1 := models.AssertExistsAndLoadBean(t, &models.Repository{
			OwnerID: user1.ID,
			Name:    "repo1",
		}).(*models.Repository)
		path := models.RepoPath(user1.Name, repo1.Name)

		_, err := git.NewCommand("read-tree", "--empty").RunInDir(path)
		assert.NoError(t, err)

		stdin := bytes.NewBufferString("Unrelated File")
		var stdout strings.Builder
		err = git.NewCommand("hash-object", "-w", "--stdin").RunInDirFullPipeline(path, &stdout, nil, stdin)
		assert.NoError(t, err)
		sha := strings.TrimSpace(stdout.String())

		_, err = git.NewCommand("update-index", "--add", "--replace", "--cacheinfo", "100644", sha, "somewher-over-the-rainbow").RunInDir(path)
		assert.NoError(t, err)

		treeSha, err := git.NewCommand("write-tree").RunInDir(path)
		assert.NoError(t, err)
		treeSha = strings.TrimSpace(treeSha)

		commitTimeStr := time.Now().Format(time.RFC3339)
		doerSig := user1.NewGitSig()
		env := append(os.Environ(),
			"GIT_AUTHOR_NAME="+doerSig.Name,
			"GIT_AUTHOR_EMAIL="+doerSig.Email,
			"GIT_AUTHOR_DATE="+commitTimeStr,
			"GIT_COMMITTER_NAME="+doerSig.Name,
			"GIT_COMMITTER_EMAIL="+doerSig.Email,
			"GIT_COMMITTER_DATE="+commitTimeStr,
		)

		messageBytes := new(bytes.Buffer)
		_, _ = messageBytes.WriteString("Unrelated")
		_, _ = messageBytes.WriteString("\n")

		stdout.Reset()
		err = git.NewCommand("commit-tree", treeSha).RunInDirTimeoutEnvFullPipeline(env, -1, path, &stdout, nil, messageBytes)
		assert.NoError(t, err)
		commitSha := strings.TrimSpace(stdout.String())

		_, err = git.NewCommand("branch", "unrelated", commitSha).RunInDir(path)
		assert.NoError(t, err)

		testEditFileToNewBranch(t, session, "user1", "repo1", "master", "conflict", "README.md", "Hello, World (Edited Once)\n")

		// Use API to create a conflicting pr
		token := getTokenForLoggedInUser(t, session)
		req := NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls?token=%s", "user1", "repo1", token), &api.CreatePullRequestOption{
			Head:  "unrelated",
			Base:  "base",
			Title: "create an unrelated pr",
		})
		session.MakeRequest(t, req, 201)

		// Now this PR could be marked conflict - or at least a race may occur - so drop down to pure code at this point...
		gitRepo, err := git.OpenRepository(path)
		assert.NoError(t, err)
		pr := models.AssertExistsAndLoadBean(t, &models.PullRequest{
			HeadRepoID: repo1.ID,
			BaseRepoID: repo1.ID,
			HeadBranch: "unrelated",
			BaseBranch: "base",
		}).(*models.PullRequest)

		err = pull.Merge(pr, user1, gitRepo, models.MergeStyleMerge, "UNRELATED")
		assert.Error(t, err, "Merge should return an error due to unrelated")
		assert.True(t, models.IsErrMergeUnrelatedHistories(err), "Merge error is not a unrelated histories error")
	})
}
