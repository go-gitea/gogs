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
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestAPILFSLocksNotStarted(t *testing.T) {
	defer prepareTestEnv(t)()
	setting.LFS.StartServer = false
	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)

	req := NewRequestf(t, "GET", "/%s/%s.git/info/lfs/locks", user.Name, repo.Name)
	MakeRequest(t, req, http.StatusNotFound)
	req = NewRequestf(t, "POST", "/%s/%s.git/info/lfs/locks", user.Name, repo.Name)
	MakeRequest(t, req, http.StatusNotFound)
	req = NewRequestf(t, "GET", "/%s/%s.git/info/lfs/locks/verify", user.Name, repo.Name)
	MakeRequest(t, req, http.StatusNotFound)
	req = NewRequestf(t, "GET", "/%s/%s.git/info/lfs/locks/10/unlock", user.Name, repo.Name)
	MakeRequest(t, req, http.StatusNotFound)
}

func TestAPILFSLocksNotLogin(t *testing.T) {
	defer prepareTestEnv(t)()
	setting.LFS.StartServer = true
	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)

	req := NewRequestf(t, "GET", "/%s/%s.git/info/lfs/locks", user.Name, repo.Name)
	req.Header.Set("Accept", "application/vnd.git-lfs+json")
	resp := MakeRequest(t, req, http.StatusUnauthorized)
	var lfsLockError api.LFSLockError
	DecodeJSON(t, resp, &lfsLockError)
	assert.Equal(t, "Unauthorized", lfsLockError.Message)
}

func TestAPILFSLocksLogged(t *testing.T) {
	defer prepareTestEnv(t)()
	setting.LFS.StartServer = true
	user2 := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User) // in org 3
	user4 := models.AssertExistsAndLoadBean(t, &models.User{ID: 4}).(*models.User) // in org 3

	repo1 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	repo3 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 3}).(*models.Repository) // own by org 3

	tests := []struct {
		user       *models.User
		repo       *models.Repository
		path       string
		httpResult int
		addTime    []int
	}{
		{user: user2, repo: repo1, path: "foo/bar.zip", httpResult: http.StatusCreated, addTime: []int{0}},
		{user: user2, repo: repo1, path: "path/test", httpResult: http.StatusCreated, addTime: []int{0}},
		{user: user2, repo: repo1, path: "path/test", httpResult: http.StatusConflict},
		{user: user2, repo: repo1, path: "Foo/BaR.zip", httpResult: http.StatusConflict},
		{user: user2, repo: repo1, path: "Foo/Test/../subFOlder/../Relative/../BaR.zip", httpResult: http.StatusConflict},
		{user: user4, repo: repo1, path: "FoO/BaR.zip", httpResult: http.StatusUnauthorized},
		{user: user4, repo: repo1, path: "path/test-user4", httpResult: http.StatusUnauthorized},
		{user: user2, repo: repo1, path: "patH/Test-user4", httpResult: http.StatusCreated, addTime: []int{0}},
		{user: user2, repo: repo1, path: "some/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/path", httpResult: http.StatusCreated, addTime: []int{0}},

		{user: user2, repo: repo3, path: "test/foo/bar.zip", httpResult: http.StatusCreated, addTime: []int{1, 2}},
		{user: user4, repo: repo3, path: "test/foo/bar.zip", httpResult: http.StatusConflict},
		{user: user4, repo: repo3, path: "test/foo/bar.bin", httpResult: http.StatusCreated, addTime: []int{1, 2}},
	}

	resultsTests := []struct {
		user        *models.User
		repo        *models.Repository
		totalCount  int
		oursCount   int
		theirsCount int
		locksOwners []*models.User
		locksTimes  []time.Time
	}{
		{user: user2, repo: repo1, totalCount: 4, oursCount: 4, theirsCount: 0, locksOwners: []*models.User{user2, user2, user2, user2}, locksTimes: []time.Time{}},
		{user: user2, repo: repo3, totalCount: 2, oursCount: 1, theirsCount: 1, locksOwners: []*models.User{user2, user4}, locksTimes: []time.Time{}},
		{user: user4, repo: repo3, totalCount: 2, oursCount: 1, theirsCount: 1, locksOwners: []*models.User{user2, user4}, locksTimes: []time.Time{}},
	}

	deleteTests := []struct {
		user   *models.User
		repo   *models.Repository
		lockID string
	}{}

	// create locks
	for _, test := range tests {
		session := loginUser(t, test.user.Name)
		req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/%s.git/info/lfs/locks", test.repo.FullName()), map[string]string{"path": test.path})
		req.Header.Set("Accept", "application/vnd.git-lfs+json")
		req.Header.Set("Content-Type", "application/vnd.git-lfs+json")
		resp := session.MakeRequest(t, req, test.httpResult)
		if len(test.addTime) > 0 {
			var lfsLock api.LFSLockResponse
			DecodeJSON(t, resp, &lfsLock)
			assert.EqualValues(t, lfsLock.Lock.LockedAt.Format(time.RFC3339), lfsLock.Lock.LockedAt.Format(time.RFC3339Nano)) // locked at should be rounded to second
			for _, id := range test.addTime {
				resultsTests[id].locksTimes = append(resultsTests[id].locksTimes, time.Now())
			}
		}
	}

	// check creation
	for _, test := range resultsTests {
		session := loginUser(t, test.user.Name)
		req := NewRequestf(t, "GET", "/%s.git/info/lfs/locks", test.repo.FullName())
		req.Header.Set("Accept", "application/vnd.git-lfs+json")
		resp := session.MakeRequest(t, req, http.StatusOK)
		var lfsLocks api.LFSLockList
		DecodeJSON(t, resp, &lfsLocks)
		assert.Len(t, lfsLocks.Locks, test.totalCount)
		for i, lock := range lfsLocks.Locks {
			assert.EqualValues(t, test.locksOwners[i].DisplayName(), lock.Owner.Name)
			assert.WithinDuration(t, test.locksTimes[i], lock.LockedAt, 10*time.Second)
			assert.EqualValues(t, lock.LockedAt.Format(time.RFC3339), lock.LockedAt.Format(time.RFC3339Nano)) // locked at should be rounded to second
		}

		req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/%s.git/info/lfs/locks/verify", test.repo.FullName()), map[string]string{})
		req.Header.Set("Accept", "application/vnd.git-lfs+json")
		req.Header.Set("Content-Type", "application/vnd.git-lfs+json")
		resp = session.MakeRequest(t, req, http.StatusOK)
		var lfsLocksVerify api.LFSLockListVerify
		DecodeJSON(t, resp, &lfsLocksVerify)
		assert.Len(t, lfsLocksVerify.Ours, test.oursCount)
		assert.Len(t, lfsLocksVerify.Theirs, test.theirsCount)
		for _, lock := range lfsLocksVerify.Ours {
			assert.EqualValues(t, test.user.DisplayName(), lock.Owner.Name)
			deleteTests = append(deleteTests, struct {
				user   *models.User
				repo   *models.Repository
				lockID string
			}{test.user, test.repo, lock.ID})
		}
		for _, lock := range lfsLocksVerify.Theirs {
			assert.NotEqual(t, test.user.DisplayName(), lock.Owner.Name)
		}
	}

	// remove all locks
	for _, test := range deleteTests {
		session := loginUser(t, test.user.Name)
		req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/%s.git/info/lfs/locks/%s/unlock", test.repo.FullName(), test.lockID), map[string]string{})
		req.Header.Set("Accept", "application/vnd.git-lfs+json")
		req.Header.Set("Content-Type", "application/vnd.git-lfs+json")
		resp := session.MakeRequest(t, req, http.StatusOK)
		var lfsLockRep api.LFSLockResponse
		DecodeJSON(t, resp, &lfsLockRep)
		assert.Equal(t, test.lockID, lfsLockRep.Lock.ID)
		assert.Equal(t, test.user.DisplayName(), lfsLockRep.Lock.Owner.Name)
	}

	// check that we don't have any lock
	for _, test := range resultsTests {
		session := loginUser(t, test.user.Name)
		req := NewRequestf(t, "GET", "/%s.git/info/lfs/locks", test.repo.FullName())
		req.Header.Set("Accept", "application/vnd.git-lfs+json")
		resp := session.MakeRequest(t, req, http.StatusOK)
		var lfsLocks api.LFSLockList
		DecodeJSON(t, resp, &lfsLocks)
		assert.Len(t, lfsLocks.Locks, 0)
	}
}
