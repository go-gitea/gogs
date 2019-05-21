// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

const (
	littleSize = 1024              //1ko
	bigSize    = 128 * 1024 * 1024 //128Mo
)

func TestGit(t *testing.T) {
	onGiteaRun(t, testGit)
}

func testGit(t *testing.T, u *url.URL) {
	username := "user2"
	baseAPITestContext := NewAPITestContext(t, username, "repo1")

	u.Path = baseAPITestContext.GitPath()

	t.Run("HTTP", func(t *testing.T) {
		PrintCurrentTest(t)
		httpContext := baseAPITestContext
		httpContext.Reponame = "repo-tmp-17"

		dstPath, err := ioutil.TempDir("", httpContext.Reponame)
		var little, big, littleLFS, bigLFS string

		assert.NoError(t, err)
		defer os.RemoveAll(dstPath)
		t.Run("Standard", func(t *testing.T) {
			PrintCurrentTest(t)
			ensureAnonymousClone(t, u)

			t.Run("CreateRepo", doAPICreateRepository(httpContext, false))

			u.Path = httpContext.GitPath()
			u.User = url.UserPassword(username, userPassword)

			t.Run("Clone", doGitClone(dstPath, u))

			t.Run("PushCommit", func(t *testing.T) {
				PrintCurrentTest(t)
				prefix := "data-file-"
				t.Run("Little", func(t *testing.T) {
					PrintCurrentTest(t)
					little = commitAndPush(t, littleSize, dstPath, prefix)
				})
				t.Run("Big", func(t *testing.T) {
					if testing.Short() {
						return
					}
					PrintCurrentTest(t)
					big = commitAndPush(t, bigSize, dstPath, prefix)
				})
			})
		})
		t.Run("LFS", func(t *testing.T) {
			PrintCurrentTest(t)
			t.Run("PushCommit", func(t *testing.T) {
				PrintCurrentTest(t)
				//Setup git LFS
				prefix := "lfs-data-file-"

				_, err = git.NewCommand("lfs").AddArguments("install").RunInDir(dstPath)
				assert.NoError(t, err)
				_, err = git.NewCommand("lfs").AddArguments("track", prefix+"*").RunInDir(dstPath)
				assert.NoError(t, err)
				err = git.AddChanges(dstPath, false, ".gitattributes")
				assert.NoError(t, err)

				t.Run("Little", func(t *testing.T) {
					PrintCurrentTest(t)
					littleLFS = commitAndPush(t, littleSize, dstPath, prefix)
					lockFileTest(t, littleLFS, dstPath)
				})
				t.Run("Big", func(t *testing.T) {
					if testing.Short() {
						return
					}
					PrintCurrentTest(t)
					bigLFS = commitAndPush(t, bigSize, dstPath, prefix)
					lockFileTest(t, bigLFS, dstPath)
				})
			})
			t.Run("Locks", func(t *testing.T) {
				PrintCurrentTest(t)
				lockTest(t, u.String(), dstPath)
			})
		})
		t.Run("Raw", func(t *testing.T) {
			PrintCurrentTest(t)
			session := loginUser(t, "user2")

			// Request raw paths
			req := NewRequest(t, "GET", path.Join("/user2/repo-tmp-17/raw/branch/master/", little))
			resp := session.MakeRequest(t, req, http.StatusOK)
			assert.Equal(t, littleSize, resp.Body.Len())

			req = NewRequest(t, "GET", path.Join("/user2/repo-tmp-17/raw/branch/master/", littleLFS))
			resp = session.MakeRequest(t, req, http.StatusOK)
			assert.NotEqual(t, littleSize, resp.Body.Len())
			assert.Contains(t, resp.Body.String(), models.LFSMetaFileIdentifier)

			if !testing.Short() {
				req = NewRequest(t, "GET", path.Join("/user2/repo-tmp-17/raw/branch/master/", big))
				nilResp := session.MakeRequestNilResponseRecorder(t, req, http.StatusOK)
				assert.Equal(t, bigSize, nilResp.Length)

				req = NewRequest(t, "GET", path.Join("/user2/repo-tmp-17/raw/branch/master/", bigLFS))
				resp = session.MakeRequest(t, req, http.StatusOK)
				assert.NotEqual(t, bigSize, resp.Body.Len())
				assert.Contains(t, resp.Body.String(), models.LFSMetaFileIdentifier)
			}

		})
		t.Run("Media", func(t *testing.T) {
			PrintCurrentTest(t)
			session := loginUser(t, "user2")

			// Request media paths
			req := NewRequest(t, "GET", path.Join("/user2/repo-tmp-17/media/branch/master/", little))
			resp := session.MakeRequestNilResponseRecorder(t, req, http.StatusOK)
			assert.Equal(t, littleSize, resp.Length)

			req = NewRequest(t, "GET", path.Join("/user2/repo-tmp-17/media/branch/master/", littleLFS))
			resp = session.MakeRequestNilResponseRecorder(t, req, http.StatusOK)
			assert.Equal(t, littleSize, resp.Length)

			if !testing.Short() {
				req = NewRequest(t, "GET", path.Join("/user2/repo-tmp-17/media/branch/master/", big))
				resp = session.MakeRequestNilResponseRecorder(t, req, http.StatusOK)
				assert.Equal(t, bigSize, resp.Length)

				req = NewRequest(t, "GET", path.Join("/user2/repo-tmp-17/media/branch/master/", bigLFS))
				resp = session.MakeRequestNilResponseRecorder(t, req, http.StatusOK)
				assert.Equal(t, bigSize, resp.Length)
			}
		})
		t.Run("BranchProtectMerge", doBranchProtectPRMerge(httpContext.Username, httpContext.Reponame, dstPath))
	})
	t.Run("SSH", func(t *testing.T) {
		PrintCurrentTest(t)
		sshContext := baseAPITestContext
		sshContext.Reponame = "repo-tmp-18"
		keyname := "my-testing-key"
		//Setup key the user ssh key
		withKeyFile(t, keyname, func(keyFile string) {
			t.Run("CreateUserKey", doAPICreateUserKey(sshContext, "test-key", keyFile))
			PrintCurrentTest(t)

			//Setup remote link
			sshURL := createSSHUrl(sshContext.GitPath(), u)

			//Setup clone folder
			dstPath, err := ioutil.TempDir("", sshContext.Reponame)
			assert.NoError(t, err)
			defer os.RemoveAll(dstPath)
			var little, big, littleLFS, bigLFS string

			t.Run("Standard", func(t *testing.T) {
				PrintCurrentTest(t)
				t.Run("CreateRepo", doAPICreateRepository(sshContext, false))

				//TODO get url from api
				t.Run("Clone", doGitClone(dstPath, sshURL))

				//time.Sleep(5 * time.Minute)
				t.Run("PushCommit", func(t *testing.T) {
					PrintCurrentTest(t)
					prefix := "data-file-"
					t.Run("Little", func(t *testing.T) {
						PrintCurrentTest(t)
						little = commitAndPush(t, littleSize, dstPath, prefix)
					})
					t.Run("Big", func(t *testing.T) {
						if testing.Short() {
							return
						}
						PrintCurrentTest(t)
						big = commitAndPush(t, bigSize, dstPath, prefix)
					})
				})
			})
			t.Run("LFS", func(t *testing.T) {
				PrintCurrentTest(t)

				t.Run("PushCommit", func(t *testing.T) {
					PrintCurrentTest(t)
					//Setup git LFS
					prefix := "lfs-data-file-"
					_, err = git.NewCommand("lfs").AddArguments("install").RunInDir(dstPath)
					assert.NoError(t, err)
					_, err = git.NewCommand("lfs").AddArguments("track", prefix+"*").RunInDir(dstPath)
					assert.NoError(t, err)
					err = git.AddChanges(dstPath, false, ".gitattributes")
					assert.NoError(t, err)

					t.Run("Little", func(t *testing.T) {
						PrintCurrentTest(t)
						littleLFS = commitAndPush(t, littleSize, dstPath, prefix)
						lockFileTest(t, littleLFS, dstPath)

					})
					t.Run("Big", func(t *testing.T) {
						if testing.Short() {
							return
						}
						PrintCurrentTest(t)
						bigLFS = commitAndPush(t, bigSize, dstPath, prefix)
						lockFileTest(t, bigLFS, dstPath)

					})
				})
				t.Run("Locks", func(t *testing.T) {
					PrintCurrentTest(t)
					lockTest(t, u.String(), dstPath)
				})
			})
			t.Run("Raw", func(t *testing.T) {
				PrintCurrentTest(t)
				session := loginUser(t, "user2")

				// Request raw paths
				req := NewRequest(t, "GET", path.Join("/user2/repo-tmp-18/raw/branch/master/", little))
				resp := session.MakeRequest(t, req, http.StatusOK)
				assert.Equal(t, littleSize, resp.Body.Len())

				req = NewRequest(t, "GET", path.Join("/user2/repo-tmp-18/raw/branch/master/", littleLFS))
				resp = session.MakeRequest(t, req, http.StatusOK)
				assert.NotEqual(t, littleSize, resp.Body.Len())
				assert.Contains(t, resp.Body.String(), models.LFSMetaFileIdentifier)

				if !testing.Short() {
					req = NewRequest(t, "GET", path.Join("/user2/repo-tmp-18/raw/branch/master/", big))
					resp = session.MakeRequest(t, req, http.StatusOK)
					assert.Equal(t, bigSize, resp.Body.Len())

					req = NewRequest(t, "GET", path.Join("/user2/repo-tmp-18/raw/branch/master/", bigLFS))
					resp = session.MakeRequest(t, req, http.StatusOK)
					assert.NotEqual(t, bigSize, resp.Body.Len())
					assert.Contains(t, resp.Body.String(), models.LFSMetaFileIdentifier)
				}
			})
			t.Run("Media", func(t *testing.T) {
				PrintCurrentTest(t)
				session := loginUser(t, "user2")

				// Request media paths
				req := NewRequest(t, "GET", path.Join("/user2/repo-tmp-18/media/branch/master/", little))
				resp := session.MakeRequest(t, req, http.StatusOK)
				assert.Equal(t, littleSize, resp.Body.Len())

				req = NewRequest(t, "GET", path.Join("/user2/repo-tmp-18/media/branch/master/", littleLFS))
				resp = session.MakeRequest(t, req, http.StatusOK)
				assert.Equal(t, littleSize, resp.Body.Len())

				if !testing.Short() {
					req = NewRequest(t, "GET", path.Join("/user2/repo-tmp-18/media/branch/master/", big))
					resp = session.MakeRequest(t, req, http.StatusOK)
					assert.Equal(t, bigSize, resp.Body.Len())

					req = NewRequest(t, "GET", path.Join("/user2/repo-tmp-18/media/branch/master/", bigLFS))
					resp = session.MakeRequest(t, req, http.StatusOK)
					assert.Equal(t, bigSize, resp.Body.Len())
				}
			})
			t.Run("BranchProtectMerge", doBranchProtectPRMerge(sshContext.Username, sshContext.Reponame, dstPath))
		})

	})
}

func ensureAnonymousClone(t *testing.T, u *url.URL) {
	dstLocalPath, err := ioutil.TempDir("", "repo1")
	assert.NoError(t, err)
	defer os.RemoveAll(dstLocalPath)
	t.Run("CloneAnonymous", doGitClone(dstLocalPath, u))

}

func lockTest(t *testing.T, remote, repoPath string) {
	lockFileTest(t, "README.md", repoPath)
}

func lockFileTest(t *testing.T, filename, repoPath string) {
	_, err := git.NewCommand("lfs").AddArguments("locks").RunInDir(repoPath)
	assert.NoError(t, err)
	_, err = git.NewCommand("lfs").AddArguments("lock", filename).RunInDir(repoPath)
	assert.NoError(t, err)
	_, err = git.NewCommand("lfs").AddArguments("locks").RunInDir(repoPath)
	assert.NoError(t, err)
	_, err = git.NewCommand("lfs").AddArguments("unlock", filename).RunInDir(repoPath)
	assert.NoError(t, err)
}

func commitAndPush(t *testing.T, size int, repoPath, prefix string) string {
	name, err := generateCommitWithNewData(size, repoPath, "user2@example.com", "User Two", prefix)
	assert.NoError(t, err)
	_, err = git.NewCommand("push", "origin", "master").RunInDir(repoPath) //Push
	assert.NoError(t, err)
	return name
}

func generateCommitWithNewData(size int, repoPath, email, fullName, prefix string) (string, error) {
	//Generate random file
	data := make([]byte, size)
	_, err := rand.Read(data)
	if err != nil {
		return "", err
	}
	tmpFile, err := ioutil.TempFile(repoPath, prefix)
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()
	_, err = tmpFile.Write(data)
	if err != nil {
		return "", err
	}

	//Commit
	err = git.AddChanges(repoPath, false, filepath.Base(tmpFile.Name()))
	if err != nil {
		return "", err
	}
	err = git.CommitChanges(repoPath, git.CommitChangesOptions{
		Committer: &git.Signature{
			Email: email,
			Name:  fullName,
			When:  time.Now(),
		},
		Author: &git.Signature{
			Email: email,
			Name:  fullName,
			When:  time.Now(),
		},
		Message: fmt.Sprintf("Testing commit @ %v", time.Now()),
	})
	return filepath.Base(tmpFile.Name()), err
}

func doBranchProtectPRMerge(username, reponame, dstPath string) func(t *testing.T) {
	return func(t *testing.T) {
		PrintCurrentTest(t)
		t.Run("CreateBranchProtected", doGitCreateBranch(dstPath, "protected"))
		t.Run("PushProtectedBranch", doGitPushTestRepository(dstPath, "origin", "protected"))

		ctx := NewAPITestContext(t, username, reponame)
		t.Run("ProtectProtectedBranchNoWhitelist", doProtectBranch(ctx, "protected", ""))
		t.Run("GenerateCommit", func(t *testing.T) {
			_, err := generateCommitWithNewData(littleSize, dstPath, "user2@example.com", "User Two", "branch-data-file-")
			assert.NoError(t, err)
		})
		t.Run("FailToPushToProtectedBranch", doGitPushTestRepositoryFail(dstPath, "origin", "protected"))
		t.Run("PushToUnprotectedBranch", doGitPushTestRepository(dstPath, "origin", "protected:unprotected"))
		var pr api.PullRequest
		var err error
		t.Run("CreatePullRequest", func(t *testing.T) {
			pr, err = doAPICreatePullRequest(ctx, username, reponame, "protected", "unprotected")(t)
			assert.NoError(t, err)
		})
		t.Run("MergePR", doAPIMergePullRequest(ctx, username, reponame, pr.Index))
		t.Run("PullProtected", doGitPull(dstPath, "origin", "protected"))
		t.Run("ProtectProtectedBranchWhitelist", doProtectBranch(ctx, "protected", username))

		t.Run("CheckoutMaster", doGitCheckoutBranch(dstPath, "master"))
		t.Run("CreateBranchForced", doGitCreateBranch(dstPath, "toforce"))
		t.Run("GenerateCommit", func(t *testing.T) {
			_, err := generateCommitWithNewData(littleSize, dstPath, "user2@example.com", "User Two", "branch-data-file-")
			assert.NoError(t, err)
		})
		t.Run("FailToForcePushToProtectedBranch", doGitPushTestRepositoryFail(dstPath, "-f", "origin", "toforce:protected"))
		t.Run("MergeProtectedToToforce", doGitMerge(dstPath, "protected"))
		t.Run("PushToProtectedBranch", doGitPushTestRepository(dstPath, "origin", "toforce:protected"))
		t.Run("CheckoutMasterAgain", doGitCheckoutBranch(dstPath, "master"))
	}
}

func doProtectBranch(ctx APITestContext, branch string, userToWhitelist string) func(t *testing.T) {
	// We are going to just use the owner to set the protection.
	return func(t *testing.T) {
		csrf := GetCSRF(t, ctx.Session, fmt.Sprintf("/%s/%s/settings/branches", url.PathEscape(ctx.Username), url.PathEscape(ctx.Reponame)))

		if userToWhitelist == "" {
			// Change branch to protected
			req := NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/settings/branches/%s", url.PathEscape(ctx.Username), url.PathEscape(ctx.Reponame), url.PathEscape(branch)), map[string]string{
				"_csrf":     csrf,
				"protected": "on",
			})
			ctx.Session.MakeRequest(t, req, http.StatusFound)
		} else {
			user, err := models.GetUserByName(userToWhitelist)
			assert.NoError(t, err)
			// Change branch to protected
			req := NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/settings/branches/%s", url.PathEscape(ctx.Username), url.PathEscape(ctx.Reponame), url.PathEscape(branch)), map[string]string{
				"_csrf":            csrf,
				"protected":        "on",
				"enable_whitelist": "on",
				"whitelist_users":  strconv.FormatInt(user.ID, 10),
			})
			ctx.Session.MakeRequest(t, req, http.StatusFound)
		}
		// Check if master branch has been locked successfully
		flashCookie := ctx.Session.GetCookie("macaron_flash")
		assert.NotNil(t, flashCookie)
		assert.EqualValues(t, "success%3DBranch%2Bprotection%2Bfor%2Bbranch%2B%2527"+url.QueryEscape(branch)+"%2527%2Bhas%2Bbeen%2Bupdated.", flashCookie.Value)
	}
}
