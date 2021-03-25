// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"math/rand"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"
)

func BenchmarkRepo(b *testing.B) {
	samples := []struct {
		url       string
		name      string
		skipShort bool
	}{
		{url: "https://github.com/go-gitea/gitea.git", name: "gitea"},
		{url: "https://github.com/ethantkoenig/manyfiles.git", name: "manyfiles"},
		{url: "https://github.com/moby/moby.git", name: "moby", skipShort: true},
		{url: "https://github.com/golang/go.git", name: "go", skipShort: true},
		{url: "https://github.com/torvalds/linux.git", name: "linux", skipShort: true},
	}
	defer prepareTestEnv(b)()
	session := loginUser(b, "user2")
	b.ResetTimer()

	for _, s := range samples {
		b.Run(s.name, func(b *testing.B) {
			if testing.Short() && s.skipShort {
				b.Skip("skipping test in short mode.")
			}
			b.Run("Migrate", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					testRepoMigrate(b, session, s.url, s.name)
				}
			})
			b.Run("Access", func(b *testing.B) {
				var branches []*api.Branch
				b.Run("APIBranchList", func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						req := NewRequestf(b, "GET", "/api/v1/repos/%s/%s/branches", "user2", s.name)
						resp := session.MakeRequest(b, req, http.StatusOK)
						b.StopTimer()
						if len(branches) == 0 {
							DecodeJSON(b, resp, &branches) // Store for next phase
						}
						b.StartTimer()
					}
				})
				branchCount := len(branches)
				b.Run("WebViewCommit", func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						req := NewRequestf(b, "GET", "/%s/%s/commit/%s", "user2", s.name, branches[i%branchCount].Commit.ID)
						session.MakeRequest(b, req, http.StatusOK)
					}
				})
			})
		})
	}
}

// StringWithCharset random string (from https://www.calhoun.io/creating-random-strings-in-go/)
func StringWithCharset(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func BenchmarkRepoBranchCommit(b *testing.B) {
	samples := []int64{1, 3, 15, 16}
	defer prepareTestEnv(b)()
	b.ResetTimer()

	for _, repoID := range samples {
		b.StopTimer()
		repo := models.AssertExistsAndLoadBean(b, &models.Repository{ID: repoID}).(*models.Repository)
		b.StartTimer()
		b.Run(repo.Name, func(b *testing.B) {
			owner := models.AssertExistsAndLoadBean(b, &models.User{ID: repo.OwnerID}).(*models.User)
			session := loginUser(b, owner.LoginName)
			b.ResetTimer()
			b.Run("Create", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					b.StopTimer()
					branchName := StringWithCharset(5+rand.Intn(10), "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
					b.StartTimer()
					testCreateBranch(b, session, owner.LoginName, repo.Name, "branch/master", branchName, http.StatusFound)
				}
			})
			b.Run("Access", func(b *testing.B) {
				var branches []*api.Branch
				req := NewRequestf(b, "GET", "/api/v1/%s/branches", repo.FullName())
				resp := session.MakeRequest(b, req, http.StatusOK)
				DecodeJSON(b, resp, &branches)
				branchCount := len(branches)
				b.ResetTimer() // We measure from here
				for i := 0; i < b.N; i++ {
					req := NewRequestf(b, "GET", "/%s/%s/commits/%s", owner.Name, repo.Name, branches[i%branchCount].Name)
					session.MakeRequest(b, req, http.StatusOK)
				}
			})
		})
	}
}

// TODO list commits /repos/{owner}/{repo}/commits
