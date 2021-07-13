// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"

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

	forkedUserCtx := NewAPITestContext(t, "user4", "repo1")

	t.Run("HTTP", func(t *testing.T) {
		defer PrintCurrentTest(t)()
		ensureAnonymousClone(t, u)
		httpContext := baseAPITestContext
		httpContext.Reponame = "repo-tmp-17"
		forkedUserCtx.Reponame = httpContext.Reponame

		dstPath, err := ioutil.TempDir("", httpContext.Reponame)
		assert.NoError(t, err)
		defer util.RemoveAll(dstPath)

		t.Run("CreateRepoInDifferentUser", doAPICreateRepository(forkedUserCtx, false))
		t.Run("AddUserAsCollaborator", doAPIAddCollaborator(forkedUserCtx, httpContext.Username, models.AccessModeRead))

		t.Run("ForkFromDifferentUser", doAPIForkRepository(httpContext, forkedUserCtx.Username))

		u.Path = httpContext.GitPath()
		u.User = url.UserPassword(username, userPassword)

		t.Run("Clone", doGitClone(dstPath, u))

		little, big := standardCommitAndPushTest(t, dstPath)
		littleLFS, bigLFS := lfsCommitAndPushTest(t, dstPath)
		rawTest(t, &httpContext, little, big, littleLFS, bigLFS)
		mediaTest(t, &httpContext, little, big, littleLFS, bigLFS)

		t.Run("BranchProtectMerge", doBranchProtectPRMerge(&httpContext, dstPath))
		t.Run("CreatePRAndSetManuallyMerged", doCreatePRAndSetManuallyMerged(httpContext, httpContext, dstPath, "master", "test-manually-merge"))
		t.Run("MergeFork", func(t *testing.T) {
			defer PrintCurrentTest(t)()
			t.Run("CreatePRAndMerge", doMergeFork(httpContext, forkedUserCtx, "master", httpContext.Username+":master"))
			rawTest(t, &forkedUserCtx, little, big, littleLFS, bigLFS)
			mediaTest(t, &forkedUserCtx, little, big, littleLFS, bigLFS)
		})

		t.Run("PushCreate", doPushCreate(httpContext, u))
	})
	t.Run("SSH", func(t *testing.T) {
		defer PrintCurrentTest(t)()
		sshContext := baseAPITestContext
		sshContext.Reponame = "repo-tmp-18"
		keyname := "my-testing-key"
		forkedUserCtx.Reponame = sshContext.Reponame
		t.Run("CreateRepoInDifferentUser", doAPICreateRepository(forkedUserCtx, false))
		t.Run("AddUserAsCollaborator", doAPIAddCollaborator(forkedUserCtx, sshContext.Username, models.AccessModeRead))
		t.Run("ForkFromDifferentUser", doAPIForkRepository(sshContext, forkedUserCtx.Username))

		//Setup key the user ssh key
		withKeyFile(t, keyname, func(keyFile string) {
			t.Run("CreateUserKey", doAPICreateUserKey(sshContext, "test-key", keyFile))

			//Setup remote link
			//TODO: get url from api
			sshURL := createSSHUrl(sshContext.GitPath(), u)

			//Setup clone folder
			dstPath, err := ioutil.TempDir("", sshContext.Reponame)
			assert.NoError(t, err)
			defer util.RemoveAll(dstPath)

			t.Run("Clone", doGitClone(dstPath, sshURL))

			little, big := standardCommitAndPushTest(t, dstPath)
			littleLFS, bigLFS := lfsCommitAndPushTest(t, dstPath)
			rawTest(t, &sshContext, little, big, littleLFS, bigLFS)
			mediaTest(t, &sshContext, little, big, littleLFS, bigLFS)

			t.Run("BranchProtectMerge", doBranchProtectPRMerge(&sshContext, dstPath))
			t.Run("MergeFork", func(t *testing.T) {
				defer PrintCurrentTest(t)()
				t.Run("CreatePRAndMerge", doMergeFork(sshContext, forkedUserCtx, "master", sshContext.Username+":master"))
				rawTest(t, &forkedUserCtx, little, big, littleLFS, bigLFS)
				mediaTest(t, &forkedUserCtx, little, big, littleLFS, bigLFS)
			})

			t.Run("PushCreate", doPushCreate(sshContext, sshURL))
		})
	})
}

func ensureAnonymousClone(t *testing.T, u *url.URL) {
	dstLocalPath, err := ioutil.TempDir("", "repo1")
	assert.NoError(t, err)
	defer util.RemoveAll(dstLocalPath)
	t.Run("CloneAnonymous", doGitClone(dstLocalPath, u))

}

func standardCommitAndPushTest(t *testing.T, dstPath string) (little, big string) {
	t.Run("Standard", func(t *testing.T) {
		defer PrintCurrentTest(t)()
		little, big = commitAndPushTest(t, dstPath, "data-file-")
	})
	return
}

func lfsCommitAndPushTest(t *testing.T, dstPath string) (littleLFS, bigLFS string) {
	t.Run("LFS", func(t *testing.T) {
		defer PrintCurrentTest(t)()
		git.CheckLFSVersion()
		if !setting.LFS.StartServer {
			t.Skip()
			return
		}
		prefix := "lfs-data-file-"
		_, err := git.NewCommand("lfs").AddArguments("install").RunInDir(dstPath)
		assert.NoError(t, err)
		_, err = git.NewCommand("lfs").AddArguments("track", prefix+"*").RunInDir(dstPath)
		assert.NoError(t, err)
		err = git.AddChanges(dstPath, false, ".gitattributes")
		assert.NoError(t, err)

		err = git.CommitChangesWithArgs(dstPath, allowLFSFilters(), git.CommitChangesOptions{
			Committer: &git.Signature{
				Email: "user2@example.com",
				Name:  "User Two",
				When:  time.Now(),
			},
			Author: &git.Signature{
				Email: "user2@example.com",
				Name:  "User Two",
				When:  time.Now(),
			},
			Message: fmt.Sprintf("Testing commit @ %v", time.Now()),
		})
		assert.NoError(t, err)

		littleLFS, bigLFS = commitAndPushTest(t, dstPath, prefix)

		t.Run("Locks", func(t *testing.T) {
			defer PrintCurrentTest(t)()
			lockTest(t, dstPath)
		})
	})
	return
}

func commitAndPushTest(t *testing.T, dstPath, prefix string) (little, big string) {
	t.Run("PushCommit", func(t *testing.T) {
		defer PrintCurrentTest(t)()
		t.Run("Little", func(t *testing.T) {
			defer PrintCurrentTest(t)()
			little = doCommitAndPush(t, littleSize, dstPath, prefix)
		})
		t.Run("Big", func(t *testing.T) {
			if testing.Short() {
				t.Skip("Skipping test in short mode.")
				return
			}
			defer PrintCurrentTest(t)()
			big = doCommitAndPush(t, bigSize, dstPath, prefix)
		})
	})
	return
}

func rawTest(t *testing.T, ctx *APITestContext, little, big, littleLFS, bigLFS string) {
	t.Run("Raw", func(t *testing.T) {
		defer PrintCurrentTest(t)()
		username := ctx.Username
		reponame := ctx.Reponame

		session := loginUser(t, username)

		// Request raw paths
		req := NewRequest(t, "GET", path.Join("/", username, reponame, "/raw/branch/master/", little))
		resp := session.MakeRequestNilResponseRecorder(t, req, http.StatusOK)
		assert.Equal(t, littleSize, resp.Length)

		git.CheckLFSVersion()
		if setting.LFS.StartServer {
			req = NewRequest(t, "GET", path.Join("/", username, reponame, "/raw/branch/master/", littleLFS))
			resp := session.MakeRequest(t, req, http.StatusOK)
			assert.NotEqual(t, littleSize, resp.Body.Len())
			assert.LessOrEqual(t, resp.Body.Len(), 1024)
			if resp.Body.Len() != littleSize && resp.Body.Len() <= 1024 {
				assert.Contains(t, resp.Body.String(), lfs.MetaFileIdentifier)
			}
		}

		if !testing.Short() {
			req = NewRequest(t, "GET", path.Join("/", username, reponame, "/raw/branch/master/", big))
			resp := session.MakeRequestNilResponseRecorder(t, req, http.StatusOK)
			assert.Equal(t, bigSize, resp.Length)

			if setting.LFS.StartServer {
				req = NewRequest(t, "GET", path.Join("/", username, reponame, "/raw/branch/master/", bigLFS))
				resp := session.MakeRequest(t, req, http.StatusOK)
				assert.NotEqual(t, bigSize, resp.Body.Len())
				if resp.Body.Len() != bigSize && resp.Body.Len() <= 1024 {
					assert.Contains(t, resp.Body.String(), lfs.MetaFileIdentifier)
				}
			}
		}
	})
}

func mediaTest(t *testing.T, ctx *APITestContext, little, big, littleLFS, bigLFS string) {
	t.Run("Media", func(t *testing.T) {
		defer PrintCurrentTest(t)()

		username := ctx.Username
		reponame := ctx.Reponame

		session := loginUser(t, username)

		// Request media paths
		req := NewRequest(t, "GET", path.Join("/", username, reponame, "/media/branch/master/", little))
		resp := session.MakeRequestNilResponseRecorder(t, req, http.StatusOK)
		assert.Equal(t, littleSize, resp.Length)

		git.CheckLFSVersion()
		if setting.LFS.StartServer {
			req = NewRequest(t, "GET", path.Join("/", username, reponame, "/media/branch/master/", littleLFS))
			resp = session.MakeRequestNilResponseRecorder(t, req, http.StatusOK)
			assert.Equal(t, littleSize, resp.Length)
		}

		if !testing.Short() {
			req = NewRequest(t, "GET", path.Join("/", username, reponame, "/media/branch/master/", big))
			resp = session.MakeRequestNilResponseRecorder(t, req, http.StatusOK)
			assert.Equal(t, bigSize, resp.Length)

			if setting.LFS.StartServer {
				req = NewRequest(t, "GET", path.Join("/", username, reponame, "/media/branch/master/", bigLFS))
				resp = session.MakeRequestNilResponseRecorder(t, req, http.StatusOK)
				assert.Equal(t, bigSize, resp.Length)
			}
		}
	})
}

func lockTest(t *testing.T, repoPath string) {
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

func doCommitAndPush(t *testing.T, size int, repoPath, prefix string) string {
	name, err := generateCommitWithNewData(size, repoPath, "user2@example.com", "User Two", prefix)
	assert.NoError(t, err)
	_, err = git.NewCommand("push", "origin", "master").RunInDir(repoPath) //Push
	assert.NoError(t, err)
	return name
}

func generateCommitWithNewData(size int, repoPath, email, fullName, prefix string) (string, error) {
	//Generate random file
	bufSize := 4 * 1024
	if bufSize > size {
		bufSize = size
	}

	buffer := make([]byte, bufSize)

	tmpFile, err := ioutil.TempFile(repoPath, prefix)
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()
	written := 0
	for written < size {
		n := size - written
		if n > bufSize {
			n = bufSize
		}
		_, err := rand.Read(buffer[:n])
		if err != nil {
			return "", err
		}
		n, err = tmpFile.Write(buffer[:n])
		if err != nil {
			return "", err
		}
		written += n
	}
	if err != nil {
		return "", err
	}

	//Commit
	// Now here we should explicitly allow lfs filters to run
	globalArgs := allowLFSFilters()
	err = git.AddChangesWithArgs(repoPath, globalArgs, false, filepath.Base(tmpFile.Name()))
	if err != nil {
		return "", err
	}
	err = git.CommitChangesWithArgs(repoPath, globalArgs, git.CommitChangesOptions{
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

func doBranchProtectPRMerge(baseCtx *APITestContext, dstPath string) func(t *testing.T) {
	return func(t *testing.T) {
		defer PrintCurrentTest(t)()
		t.Run("CreateBranchProtected", doGitCreateBranch(dstPath, "protected"))
		t.Run("PushProtectedBranch", doGitPushTestRepository(dstPath, "origin", "protected"))

		ctx := NewAPITestContext(t, baseCtx.Username, baseCtx.Reponame)
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
			pr, err = doAPICreatePullRequest(ctx, baseCtx.Username, baseCtx.Reponame, "protected", "unprotected")(t)
			assert.NoError(t, err)
		})
		t.Run("GenerateCommit", func(t *testing.T) {
			_, err := generateCommitWithNewData(littleSize, dstPath, "user2@example.com", "User Two", "branch-data-file-")
			assert.NoError(t, err)
		})
		t.Run("PushToUnprotectedBranch", doGitPushTestRepository(dstPath, "origin", "protected:unprotected-2"))
		var pr2 api.PullRequest
		t.Run("CreatePullRequest", func(t *testing.T) {
			pr2, err = doAPICreatePullRequest(ctx, baseCtx.Username, baseCtx.Reponame, "unprotected", "unprotected-2")(t)
			assert.NoError(t, err)
		})
		t.Run("MergePR2", doAPIMergePullRequest(ctx, baseCtx.Username, baseCtx.Reponame, pr2.Index))
		t.Run("MergePR", doAPIMergePullRequest(ctx, baseCtx.Username, baseCtx.Reponame, pr.Index))
		t.Run("PullProtected", doGitPull(dstPath, "origin", "protected"))
		t.Run("ProtectProtectedBranchWhitelist", doProtectBranch(ctx, "protected", baseCtx.Username))

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
				"enable_push":      "whitelist",
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

func doMergeFork(ctx, baseCtx APITestContext, baseBranch, headBranch string) func(t *testing.T) {
	return func(t *testing.T) {
		defer PrintCurrentTest(t)()
		var pr api.PullRequest
		var err error

		// Create a test pullrequest
		t.Run("CreatePullRequest", func(t *testing.T) {
			pr, err = doAPICreatePullRequest(ctx, baseCtx.Username, baseCtx.Reponame, baseBranch, headBranch)(t)
			assert.NoError(t, err)
		})

		// Ensure the PR page works
		t.Run("EnsureCanSeePull", doEnsureCanSeePull(baseCtx, pr))

		// Then get the diff string
		var diffHash string
		var diffLength int
		t.Run("GetDiff", func(t *testing.T) {
			req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s/pulls/%d.diff", url.PathEscape(baseCtx.Username), url.PathEscape(baseCtx.Reponame), pr.Index))
			resp := ctx.Session.MakeRequestNilResponseHashSumRecorder(t, req, http.StatusOK)
			diffHash = string(resp.Hash.Sum(nil))
			diffLength = resp.Length
		})

		// Now: Merge the PR & make sure that doesn't break the PR page or change its diff
		t.Run("MergePR", doAPIMergePullRequest(baseCtx, baseCtx.Username, baseCtx.Reponame, pr.Index))
		t.Run("EnsureCanSeePull", doEnsureCanSeePull(baseCtx, pr))
		t.Run("CheckPR", func(t *testing.T) {
			oldMergeBase := pr.MergeBase
			pr2, err := doAPIGetPullRequest(baseCtx, baseCtx.Username, baseCtx.Reponame, pr.Index)(t)
			assert.NoError(t, err)
			assert.Equal(t, oldMergeBase, pr2.MergeBase)
		})
		t.Run("EnsurDiffNoChange", doEnsureDiffNoChange(baseCtx, pr, diffHash, diffLength))

		// Then: Delete the head branch & make sure that doesn't break the PR page or change its diff
		t.Run("DeleteHeadBranch", doBranchDelete(baseCtx, baseCtx.Username, baseCtx.Reponame, headBranch))
		t.Run("EnsureCanSeePull", doEnsureCanSeePull(baseCtx, pr))
		t.Run("EnsureDiffNoChange", doEnsureDiffNoChange(baseCtx, pr, diffHash, diffLength))

		// Delete the head repository & make sure that doesn't break the PR page or change its diff
		t.Run("DeleteHeadRepository", doAPIDeleteRepository(ctx))
		t.Run("EnsureCanSeePull", doEnsureCanSeePull(baseCtx, pr))
		t.Run("EnsureDiffNoChange", doEnsureDiffNoChange(baseCtx, pr, diffHash, diffLength))
	}
}

func doCreatePRAndSetManuallyMerged(ctx, baseCtx APITestContext, dstPath, baseBranch, headBranch string) func(t *testing.T) {
	return func(t *testing.T) {
		defer PrintCurrentTest(t)()
		var (
			pr           api.PullRequest
			err          error
			lastCommitID string
		)

		trueBool := true
		falseBool := false

		t.Run("AllowSetManuallyMergedAndSwitchOffAutodetectManualMerge", doAPIEditRepository(baseCtx, &api.EditRepoOption{
			HasPullRequests:       &trueBool,
			AllowManualMerge:      &trueBool,
			AutodetectManualMerge: &falseBool,
		}))

		t.Run("CreateHeadBranch", doGitCreateBranch(dstPath, headBranch))
		t.Run("PushToHeadBranch", doGitPushTestRepository(dstPath, "origin", headBranch))
		t.Run("CreateEmptyPullRequest", func(t *testing.T) {
			pr, err = doAPICreatePullRequest(ctx, baseCtx.Username, baseCtx.Reponame, baseBranch, headBranch)(t)
			assert.NoError(t, err)
		})
		lastCommitID = pr.Base.Sha
		t.Run("ManuallyMergePR", doAPIManuallyMergePullRequest(ctx, baseCtx.Username, baseCtx.Reponame, lastCommitID, pr.Index))
	}
}

func doEnsureCanSeePull(ctx APITestContext, pr api.PullRequest) func(t *testing.T) {
	return func(t *testing.T) {
		req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s/pulls/%d", url.PathEscape(ctx.Username), url.PathEscape(ctx.Reponame), pr.Index))
		ctx.Session.MakeRequest(t, req, http.StatusOK)
		req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/pulls/%d/files", url.PathEscape(ctx.Username), url.PathEscape(ctx.Reponame), pr.Index))
		ctx.Session.MakeRequest(t, req, http.StatusOK)
		req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s/pulls/%d/commits", url.PathEscape(ctx.Username), url.PathEscape(ctx.Reponame), pr.Index))
		ctx.Session.MakeRequest(t, req, http.StatusOK)
	}
}

func doEnsureDiffNoChange(ctx APITestContext, pr api.PullRequest, diffHash string, diffLength int) func(t *testing.T) {
	return func(t *testing.T) {
		req := NewRequest(t, "GET", fmt.Sprintf("/%s/%s/pulls/%d.diff", url.PathEscape(ctx.Username), url.PathEscape(ctx.Reponame), pr.Index))
		resp := ctx.Session.MakeRequestNilResponseHashSumRecorder(t, req, http.StatusOK)
		actual := string(resp.Hash.Sum(nil))
		actualLength := resp.Length

		equal := diffHash == actual
		assert.True(t, equal, "Unexpected change in the diff string: expected hash: %s size: %d but was actually: %s size: %d", hex.EncodeToString([]byte(diffHash)), diffLength, hex.EncodeToString([]byte(actual)), actualLength)
	}
}

func doPushCreate(ctx APITestContext, u *url.URL) func(t *testing.T) {
	return func(t *testing.T) {
		defer PrintCurrentTest(t)()

		// create a context for a currently non-existent repository
		ctx.Reponame = fmt.Sprintf("repo-tmp-push-create-%s", u.Scheme)
		u.Path = ctx.GitPath()

		// Create a temporary directory
		tmpDir, err := ioutil.TempDir("", ctx.Reponame)
		assert.NoError(t, err)
		defer util.RemoveAll(tmpDir)

		// Now create local repository to push as our test and set its origin
		t.Run("InitTestRepository", doGitInitTestRepository(tmpDir))
		t.Run("AddRemote", doGitAddRemote(tmpDir, "origin", u))

		// Disable "Push To Create" and attempt to push
		setting.Repository.EnablePushCreateUser = false
		t.Run("FailToPushAndCreateTestRepository", doGitPushTestRepositoryFail(tmpDir, "origin", "master"))

		// Enable "Push To Create"
		setting.Repository.EnablePushCreateUser = true

		// Assert that cloning from a non-existent repository does not create it and that it definitely wasn't create above
		t.Run("FailToCloneFromNonExistentRepository", doGitCloneFail(u))

		// Then "Push To Create"x
		t.Run("SuccessfullyPushAndCreateTestRepository", doGitPushTestRepository(tmpDir, "origin", "master"))

		// Finally, fetch repo from database and ensure the correct repository has been created
		repo, err := models.GetRepositoryByOwnerAndName(ctx.Username, ctx.Reponame)
		assert.NoError(t, err)
		assert.False(t, repo.IsEmpty)
		assert.True(t, repo.IsPrivate)

		// Now add a remote that is invalid to "Push To Create"
		invalidCtx := ctx
		invalidCtx.Reponame = fmt.Sprintf("invalid/repo-tmp-push-create-%s", u.Scheme)
		u.Path = invalidCtx.GitPath()
		t.Run("AddInvalidRemote", doGitAddRemote(tmpDir, "invalid", u))

		// Fail to "Push To Create" the invalid
		t.Run("FailToPushAndCreateInvalidTestRepository", doGitPushTestRepositoryFail(tmpDir, "invalid", "master"))
	}
}

func doBranchDelete(ctx APITestContext, owner, repo, branch string) func(*testing.T) {
	return func(t *testing.T) {
		csrf := GetCSRF(t, ctx.Session, fmt.Sprintf("/%s/%s/branches", url.PathEscape(owner), url.PathEscape(repo)))

		req := NewRequestWithValues(t, "POST", fmt.Sprintf("/%s/%s/branches/delete?name=%s", url.PathEscape(owner), url.PathEscape(repo), url.QueryEscape(branch)), map[string]string{
			"_csrf": csrf,
		})
		ctx.Session.MakeRequest(t, req, http.StatusOK)
	}
}
