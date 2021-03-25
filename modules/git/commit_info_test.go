// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/util"
	"github.com/stretchr/testify/assert"
)

const (
	testReposDir      = "tests/repos/"
	benchmarkReposDir = "benchmark/repos/"
)

func cloneRepo(url, dir, name string) (string, error) {
	repoDir := filepath.Join(dir, name)
	if _, err := os.Stat(repoDir); err == nil {
		return repoDir, nil
	}
	return repoDir, Clone(url, repoDir, CloneRepoOptions{
		Mirror:  false,
		Bare:    false,
		Quiet:   true,
		Timeout: 5 * time.Minute,
	})
}

func testGetCommitsInfo(t *testing.T, repo1 *Repository) {
	// these test case are specific to the repo1 test repo
	testCases := []struct {
		CommitID           string
		Path               string
		ExpectedIDs        map[string]string
		ExpectedTreeCommit string
	}{
		{"8d92fc957a4d7cfd98bc375f0b7bb189a0d6c9f2", "", map[string]string{
			"file1.txt": "95bb4d39648ee7e325106df01a621c530863a653",
			"file2.txt": "8d92fc957a4d7cfd98bc375f0b7bb189a0d6c9f2",
		}, "8d92fc957a4d7cfd98bc375f0b7bb189a0d6c9f2"},
		{"2839944139e0de9737a044f78b0e4b40d989a9e3", "", map[string]string{
			"file1.txt":   "2839944139e0de9737a044f78b0e4b40d989a9e3",
			"branch1.txt": "9c9aef8dd84e02bc7ec12641deb4c930a7c30185",
		}, "2839944139e0de9737a044f78b0e4b40d989a9e3"},
		{"5c80b0245c1c6f8343fa418ec374b13b5d4ee658", "branch2", map[string]string{
			"branch2.txt": "5c80b0245c1c6f8343fa418ec374b13b5d4ee658",
		}, "5c80b0245c1c6f8343fa418ec374b13b5d4ee658"},
		{"feaf4ba6bc635fec442f46ddd4512416ec43c2c2", "", map[string]string{
			"file1.txt": "95bb4d39648ee7e325106df01a621c530863a653",
			"file2.txt": "8d92fc957a4d7cfd98bc375f0b7bb189a0d6c9f2",
			"foo":       "37991dec2c8e592043f47155ce4808d4580f9123",
		}, "feaf4ba6bc635fec442f46ddd4512416ec43c2c2"},
	}
	for _, testCase := range testCases {
		commit, err := repo1.GetCommit(testCase.CommitID)
		assert.NoError(t, err)
		assert.NotNil(t, commit)
		assert.NotNil(t, commit.Tree)
		assert.NotNil(t, commit.Tree.repo)

		tree, err := commit.Tree.SubTree(testCase.Path)
		assert.NotNil(t, tree, "tree is nil for testCase CommitID %s in Path %s", testCase.CommitID, testCase.Path)
		assert.NotNil(t, tree.repo, "repo is nil for testCase CommitID %s in Path %s", testCase.CommitID, testCase.Path)

		assert.NoError(t, err)
		entries, err := tree.ListEntries()
		assert.NoError(t, err)
		commitsInfo, treeCommit, err := entries.GetCommitsInfo(commit, testCase.Path, nil)
		assert.NoError(t, err)
		if err != nil {
			t.FailNow()
		}
		assert.Equal(t, testCase.ExpectedTreeCommit, treeCommit.ID.String())
		assert.Len(t, commitsInfo, len(testCase.ExpectedIDs))
		for _, commitInfo := range commitsInfo {
			entry := commitInfo.Entry
			commit := commitInfo.Commit
			expectedID, ok := testCase.ExpectedIDs[entry.Name()]
			if !assert.True(t, ok) {
				continue
			}
			assert.Equal(t, expectedID, commit.ID.String())
		}
	}
}

func TestEntries_GetCommitsInfo(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")
	bareRepo1, err := OpenRepository(bareRepo1Path)
	assert.NoError(t, err)
	defer bareRepo1.Close()

	testGetCommitsInfo(t, bareRepo1)

	clonedPath, err := cloneRepo(bareRepo1Path, testReposDir, "repo1_TestEntries_GetCommitsInfo")
	assert.NoError(t, err)
	defer util.RemoveAll(clonedPath)
	clonedRepo1, err := OpenRepository(clonedPath)
	assert.NoError(t, err)
	defer clonedRepo1.Close()

	testGetCommitsInfo(t, clonedRepo1)
}

func BenchmarkEntries_GetCommitsInfo(b *testing.B) {
	benchmarks := []struct {
		url  string
		name string
	}{
		{url: "https://github.com/go-gitea/gitea.git", name: "gitea"},
		{url: "https://github.com/ethantkoenig/manyfiles.git", name: "manyfiles"},
		{url: "https://github.com/moby/moby.git", name: "moby"},
		{url: "https://github.com/golang/go.git", name: "go"},
		{url: "https://github.com/torvalds/linux.git", name: "linux"},
	}
	for _, benchmark := range benchmarks {
		var commit *Commit
		var entries Entries
		var repo *Repository
		if repoPath, err := cloneRepo(benchmark.url, benchmarkReposDir, benchmark.name); err != nil {
			b.Fatal(err)
		} else if repo, err = OpenRepository(repoPath); err != nil {
			b.Fatal(err)
		} else if commit, err = repo.GetBranchCommit("master"); err != nil {
			repo.Close()
			b.Fatal(err)
		} else if entries, err = commit.Tree.ListEntries(); err != nil {
			repo.Close()
			b.Fatal(err)
		}
		entries.Sort()
		b.ResetTimer()
		b.Run(benchmark.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _, err := entries.GetCommitsInfo(commit, "", nil)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
		repo.Close()
	}
}
