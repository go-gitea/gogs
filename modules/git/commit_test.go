// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommitsCount(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")

	commitsCount, err := CommitsCount(bareRepo1Path, "8006ff9adbf0cb94da7dad9e537e53817f9fa5c0")
	assert.NoError(t, err)
	assert.Equal(t, int64(3), commitsCount)
}

func TestGetFullCommitID(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")

	id, err := GetFullCommitID(bareRepo1Path, "8006ff9a")
	assert.NoError(t, err)
	assert.Equal(t, "8006ff9adbf0cb94da7dad9e537e53817f9fa5c0", id)
}

func TestGetFullCommitIDError(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")

	id, err := GetFullCommitID(bareRepo1Path, "unknown")
	assert.Empty(t, id)
	if assert.Error(t, err) {
		assert.EqualError(t, err, "object does not exist [id: unknown, rel_path: ]")
	}
}

func TestCommitFromReader(t *testing.T) {
	commitString := `feaf4ba6bc635fec442f46ddd4512416ec43c2c2 commit 1074
tree f1a6cb52b2d16773290cefe49ad0684b50a4f930
parent 37991dec2c8e592043f47155ce4808d4580f9123
author silverwind <me@silverwind.io> 1563741793 +0200
committer silverwind <me@silverwind.io> 1563741793 +0200
gpgsig -----BEGIN PGP SIGNATURE-----
` + " " + `
 iQIzBAABCAAdFiEEWPb2jX6FS2mqyJRQLmK0HJOGlEMFAl00zmEACgkQLmK0HJOG
 lEMDFBAAhQKKqLD1VICygJMEB8t1gBmNLgvziOLfpX4KPWdPtBk3v/QJ7OrfMrVK
 xlC4ZZyx6yMm1Q7GzmuWykmZQJ9HMaHJ49KAbh5MMjjV/+OoQw9coIdo8nagRUld
 vX8QHzNZ6Agx77xHuDJZgdHKpQK3TrMDsxzoYYMvlqoLJIDXE1Sp7KYNy12nhdRg
 R6NXNmW8oMZuxglkmUwayMiPS+N4zNYqv0CXYzlEqCOgq9MJUcAMHt+KpiST+sm6
 FWkJ9D+biNPyQ9QKf1AE4BdZia4lHfPYU/C/DEL/a5xQuuop/zMQZoGaIA4p2zGQ
 /maqYxEIM/yRBQpT1jlODKPJrMEgx7SgY2hRU47YZ4fj6350fb6fNBtiiMAfJbjL
 S3Gh85E9fm3hJaNSPKAaJFYL1Ya2svuWfgHj677C56UcmYis7fhiiy1aJuYdHnSm
 sD53z/f0J+We4VZjY+pidvA9BGZPFVdR3wd3xGs8/oH6UWaLJAMGkLG6dDb3qDLm
 1LFZwsX8sdD32i1SiWanYQYSYMyFWr0awi4xdoMtYCL7uKBYtwtPyvq3cj4IrJlb
 mfeFhT57UbE4qukTDIQ0Y0WM40UYRTakRaDY7ubhXgLgx09Cnp9XTVMsHgT6j9/i
 1pxsB104XLWjQHTjr1JtiaBQEwFh9r2OKTcpvaLcbNtYpo7CzOs=
 =FRsO
 -----END PGP SIGNATURE-----

empty commit`

	sha := SHA1{0xfe, 0xaf, 0x4b, 0xa6, 0xbc, 0x63, 0x5f, 0xec, 0x44, 0x2f, 0x46, 0xdd, 0xd4, 0x51, 0x24, 0x16, 0xec, 0x43, 0xc2, 0xc2}
	gitRepo, err := OpenRepository(filepath.Join(testReposDir, "repo1_bare"))
	assert.NoError(t, err)
	assert.NotNil(t, gitRepo)

	commitFromReader, err := CommitFromReader(gitRepo, sha, strings.NewReader(commitString))
	assert.NoError(t, err)
	if !assert.NotNil(t, commitFromReader) {
		return
	}
	assert.EqualValues(t, sha, commitFromReader.ID)
	assert.EqualValues(t, `-----BEGIN PGP SIGNATURE-----

iQIzBAABCAAdFiEEWPb2jX6FS2mqyJRQLmK0HJOGlEMFAl00zmEACgkQLmK0HJOG
lEMDFBAAhQKKqLD1VICygJMEB8t1gBmNLgvziOLfpX4KPWdPtBk3v/QJ7OrfMrVK
xlC4ZZyx6yMm1Q7GzmuWykmZQJ9HMaHJ49KAbh5MMjjV/+OoQw9coIdo8nagRUld
vX8QHzNZ6Agx77xHuDJZgdHKpQK3TrMDsxzoYYMvlqoLJIDXE1Sp7KYNy12nhdRg
R6NXNmW8oMZuxglkmUwayMiPS+N4zNYqv0CXYzlEqCOgq9MJUcAMHt+KpiST+sm6
FWkJ9D+biNPyQ9QKf1AE4BdZia4lHfPYU/C/DEL/a5xQuuop/zMQZoGaIA4p2zGQ
/maqYxEIM/yRBQpT1jlODKPJrMEgx7SgY2hRU47YZ4fj6350fb6fNBtiiMAfJbjL
S3Gh85E9fm3hJaNSPKAaJFYL1Ya2svuWfgHj677C56UcmYis7fhiiy1aJuYdHnSm
sD53z/f0J+We4VZjY+pidvA9BGZPFVdR3wd3xGs8/oH6UWaLJAMGkLG6dDb3qDLm
1LFZwsX8sdD32i1SiWanYQYSYMyFWr0awi4xdoMtYCL7uKBYtwtPyvq3cj4IrJlb
mfeFhT57UbE4qukTDIQ0Y0WM40UYRTakRaDY7ubhXgLgx09Cnp9XTVMsHgT6j9/i
1pxsB104XLWjQHTjr1JtiaBQEwFh9r2OKTcpvaLcbNtYpo7CzOs=
=FRsO
-----END PGP SIGNATURE-----
`, commitFromReader.Signature.Signature)
	assert.EqualValues(t, `tree f1a6cb52b2d16773290cefe49ad0684b50a4f930
parent 37991dec2c8e592043f47155ce4808d4580f9123
author silverwind <me@silverwind.io> 1563741793 +0200
committer silverwind <me@silverwind.io> 1563741793 +0200

empty commit`, commitFromReader.Signature.Payload)
	assert.EqualValues(t, "silverwind <me@silverwind.io>", commitFromReader.Author.String())

	commitFromReader2, err := CommitFromReader(gitRepo, sha, strings.NewReader(commitString+"\n\n"))
	assert.NoError(t, err)
	commitFromReader.CommitMessage += "\n\n"
	commitFromReader.Signature.Payload += "\n\n"
	assert.EqualValues(t, commitFromReader, commitFromReader2)
}

func TestHasPreviousCommit(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")

	repo, err := OpenRepository(bareRepo1Path)
	assert.NoError(t, err)

	commit, err := repo.GetCommit("8006ff9adbf0cb94da7dad9e537e53817f9fa5c0")
	assert.NoError(t, err)

	parentSHA := MustIDFromString("8d92fc957a4d7cfd98bc375f0b7bb189a0d6c9f2")
	notParentSHA := MustIDFromString("2839944139e0de9737a044f78b0e4b40d989a9e3")

	haz, err := commit.HasPreviousCommit(parentSHA)
	assert.NoError(t, err)
	assert.True(t, haz)

	hazNot, err := commit.HasPreviousCommit(notParentSHA)
	assert.NoError(t, err)
	assert.False(t, hazNot)

	selfNot, err := commit.HasPreviousCommit(commit.ID)
	assert.NoError(t, err)
	assert.False(t, selfNot)
}

func TestParseCommitFileStatus(t *testing.T) {
	type testcase struct {
		output   string
		added    []string
		removed  []string
		modified []string
	}

	kases := []testcase{
		{
			// Merge commit
			output: "MM\x00options/locale/locale_en-US.ini\x00",
			modified: []string{
				"options/locale/locale_en-US.ini",
			},
			added:   []string{},
			removed: []string{},
		},
		{
			// Spaces commit
			output: "D\x00b\x00D\x00b b/b\x00A\x00b b/b b/b b/b\x00A\x00b b/b b/b b/b b/b\x00",
			removed: []string{
				"b",
				"b b/b",
			},
			modified: []string{},
			added: []string{
				"b b/b b/b b/b",
				"b b/b b/b b/b b/b",
			},
		},
		{
			// larger commit
			output: "M\x00go.mod\x00M\x00go.sum\x00M\x00modules/ssh/ssh.go\x00M\x00vendor/github.com/gliderlabs/ssh/circle.yml\x00M\x00vendor/github.com/gliderlabs/ssh/context.go\x00A\x00vendor/github.com/gliderlabs/ssh/go.mod\x00A\x00vendor/github.com/gliderlabs/ssh/go.sum\x00M\x00vendor/github.com/gliderlabs/ssh/server.go\x00M\x00vendor/github.com/gliderlabs/ssh/session.go\x00M\x00vendor/github.com/gliderlabs/ssh/ssh.go\x00M\x00vendor/golang.org/x/sys/unix/mkerrors.sh\x00M\x00vendor/golang.org/x/sys/unix/syscall_darwin.go\x00M\x00vendor/golang.org/x/sys/unix/zerrors_darwin_amd64.go\x00M\x00vendor/golang.org/x/sys/unix/zerrors_darwin_arm64.go\x00M\x00vendor/golang.org/x/sys/unix/zerrors_freebsd_386.go\x00M\x00vendor/golang.org/x/sys/unix/zerrors_freebsd_amd64.go\x00M\x00vendor/golang.org/x/sys/unix/zerrors_freebsd_arm.go\x00M\x00vendor/golang.org/x/sys/unix/zerrors_freebsd_arm64.go\x00M\x00vendor/golang.org/x/sys/unix/zerrors_linux.go\x00M\x00vendor/golang.org/x/sys/unix/ztypes_darwin_amd64.go\x00M\x00vendor/golang.org/x/sys/unix/ztypes_darwin_arm64.go\x00M\x00vendor/golang.org/x/sys/unix/ztypes_dragonfly_amd64.go\x00M\x00vendor/golang.org/x/sys/unix/ztypes_freebsd_386.go\x00M\x00vendor/golang.org/x/sys/unix/ztypes_freebsd_amd64.go\x00M\x00vendor/golang.org/x/sys/unix/ztypes_freebsd_arm.go\x00M\x00vendor/golang.org/x/sys/unix/ztypes_freebsd_arm64.go\x00M\x00vendor/golang.org/x/sys/unix/ztypes_netbsd_386.go\x00M\x00vendor/golang.org/x/sys/unix/ztypes_netbsd_amd64.go\x00M\x00vendor/golang.org/x/sys/unix/ztypes_netbsd_arm.go\x00M\x00vendor/golang.org/x/sys/unix/ztypes_netbsd_arm64.go\x00M\x00vendor/modules.txt\x00",
			modified: []string{
				"go.mod",
				"go.sum",
				"modules/ssh/ssh.go",
				"vendor/github.com/gliderlabs/ssh/circle.yml",
				"vendor/github.com/gliderlabs/ssh/context.go",
				"vendor/github.com/gliderlabs/ssh/server.go",
				"vendor/github.com/gliderlabs/ssh/session.go",
				"vendor/github.com/gliderlabs/ssh/ssh.go",
				"vendor/golang.org/x/sys/unix/mkerrors.sh",
				"vendor/golang.org/x/sys/unix/syscall_darwin.go",
				"vendor/golang.org/x/sys/unix/zerrors_darwin_amd64.go",
				"vendor/golang.org/x/sys/unix/zerrors_darwin_arm64.go",
				"vendor/golang.org/x/sys/unix/zerrors_freebsd_386.go",
				"vendor/golang.org/x/sys/unix/zerrors_freebsd_amd64.go",
				"vendor/golang.org/x/sys/unix/zerrors_freebsd_arm.go",
				"vendor/golang.org/x/sys/unix/zerrors_freebsd_arm64.go",
				"vendor/golang.org/x/sys/unix/zerrors_linux.go",
				"vendor/golang.org/x/sys/unix/ztypes_darwin_amd64.go",
				"vendor/golang.org/x/sys/unix/ztypes_darwin_arm64.go",
				"vendor/golang.org/x/sys/unix/ztypes_dragonfly_amd64.go",
				"vendor/golang.org/x/sys/unix/ztypes_freebsd_386.go",
				"vendor/golang.org/x/sys/unix/ztypes_freebsd_amd64.go",
				"vendor/golang.org/x/sys/unix/ztypes_freebsd_arm.go",
				"vendor/golang.org/x/sys/unix/ztypes_freebsd_arm64.go",
				"vendor/golang.org/x/sys/unix/ztypes_netbsd_386.go",
				"vendor/golang.org/x/sys/unix/ztypes_netbsd_amd64.go",
				"vendor/golang.org/x/sys/unix/ztypes_netbsd_arm.go",
				"vendor/golang.org/x/sys/unix/ztypes_netbsd_arm64.go",
				"vendor/modules.txt",
			},
			added: []string{
				"vendor/github.com/gliderlabs/ssh/go.mod",
				"vendor/github.com/gliderlabs/ssh/go.sum",
			},
			removed: []string{},
		},
		{
			// git 1.7.2 adds an unnecessary \x00 on merge commit
			output: "\x00MM\x00options/locale/locale_en-US.ini\x00",
			modified: []string{
				"options/locale/locale_en-US.ini",
			},
			added:   []string{},
			removed: []string{},
		},
		{
			// git 1.7.2 adds an unnecessary \n on normal commit
			output: "\nD\x00b\x00D\x00b b/b\x00A\x00b b/b b/b b/b\x00A\x00b b/b b/b b/b b/b\x00",
			removed: []string{
				"b",
				"b b/b",
			},
			modified: []string{},
			added: []string{
				"b b/b b/b b/b",
				"b b/b b/b b/b b/b",
			},
		},
	}

	for _, kase := range kases {
		fileStatus := NewCommitFileStatus()
		parseCommitFileStatus(fileStatus, strings.NewReader(kase.output))

		assert.Equal(t, kase.added, fileStatus.Added)
		assert.Equal(t, kase.removed, fileStatus.Removed)
		assert.Equal(t, kase.modified, fileStatus.Modified)
	}

}
