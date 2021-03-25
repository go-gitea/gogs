// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/ssh"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
)

func withKeyFile(t *testing.T, keyname string, callback func(string)) {
	tmpDir, err := ioutil.TempDir("", "key-file")
	assert.NoError(t, err)
	defer util.RemoveAll(tmpDir)

	err = os.Chmod(tmpDir, 0o700)
	assert.NoError(t, err)

	keyFile := filepath.Join(tmpDir, keyname)
	err = ssh.GenKeyPair(keyFile)
	assert.NoError(t, err)

	err = ioutil.WriteFile(path.Join(tmpDir, "ssh"), []byte("#!/bin/bash\n"+
		"ssh -o \"UserKnownHostsFile=/dev/null\" -o \"StrictHostKeyChecking=no\" -o \"IdentitiesOnly=yes\" -i \""+keyFile+"\" \"$@\""), 0o700)
	assert.NoError(t, err)

	// Setup ssh wrapper
	os.Setenv("GIT_SSH", path.Join(tmpDir, "ssh"))
	os.Setenv("GIT_SSH_COMMAND",
		"ssh -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -o IdentitiesOnly=yes -i \""+keyFile+"\"")
	os.Setenv("GIT_SSH_VARIANT", "ssh")

	callback(keyFile)
}

func createSSHUrl(gitPath string, u *url.URL) *url.URL {
	u2 := *u
	u2.Scheme = "ssh"
	u2.User = url.User("git")
	u2.Host = fmt.Sprintf("%s:%d", setting.SSH.ListenHost, setting.SSH.ListenPort)
	u2.Path = gitPath
	return &u2
}

func allowLFSFilters() []string {
	// Now here we should explicitly allow lfs filters to run
	filteredLFSGlobalArgs := make([]string, len(git.GlobalCommandArgs))
	j := 0
	for _, arg := range git.GlobalCommandArgs {
		if strings.Contains(arg, "lfs") {
			j--
		} else {
			filteredLFSGlobalArgs[j] = arg
			j++
		}
	}
	return filteredLFSGlobalArgs[:j]
}

func onGiteaRun(t *testing.T, callback func(*testing.T, *url.URL), prepare ...bool) {
	if len(prepare) == 0 || prepare[0] {
		defer prepareTestEnv(t, 1)()
	}
	s := http.Server{
		Handler: c,
	}

	u, err := url.Parse(setting.AppURL)
	assert.NoError(t, err)
	listener, err := net.Listen("tcp", u.Host)
	i := 0
	for err != nil && i <= 10 {
		time.Sleep(100 * time.Millisecond)
		listener, err = net.Listen("tcp", u.Host)
		i++
	}
	assert.NoError(t, err)
	u.Host = listener.Addr().String()

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		s.Shutdown(ctx)
		cancel()
	}()

	go s.Serve(listener)
	// Started by config go ssh.Listen(setting.SSH.ListenHost, setting.SSH.ListenPort, setting.SSH.ServerCiphers, setting.SSH.ServerKeyExchanges, setting.SSH.ServerMACs)

	callback(t, u)
}

func doGitClone(dstLocalPath string, u *url.URL) func(*testing.T) {
	return func(t *testing.T) {
		assert.NoError(t, git.CloneWithArgs(context.Background(), u.String(), dstLocalPath, allowLFSFilters(), git.CloneRepoOptions{}))
		exist, err := util.IsExist(filepath.Join(dstLocalPath, "README.md"))
		assert.NoError(t, err)
		assert.True(t, exist)
	}
}

func doGitCloneFail(u *url.URL) func(*testing.T) {
	return func(t *testing.T) {
		tmpDir, err := ioutil.TempDir("", "doGitCloneFail")
		assert.NoError(t, err)
		defer util.RemoveAll(tmpDir)
		assert.Error(t, git.Clone(u.String(), tmpDir, git.CloneRepoOptions{}))
		exist, err := util.IsExist(filepath.Join(tmpDir, "README.md"))
		assert.NoError(t, err)
		assert.False(t, exist)
	}
}

func doGitInitTestRepository(dstPath string) func(*testing.T) {
	return func(t *testing.T) {
		// Init repository in dstPath
		assert.NoError(t, git.InitRepository(dstPath, false))
		assert.NoError(t, ioutil.WriteFile(filepath.Join(dstPath, "README.md"), []byte(fmt.Sprintf("# Testing Repository\n\nOriginally created in: %s", dstPath)), 0o644))
		assert.NoError(t, git.AddChanges(dstPath, true))
		signature := git.Signature{
			Email: "test@example.com",
			Name:  "test",
			When:  time.Now(),
		}
		assert.NoError(t, git.CommitChanges(dstPath, git.CommitChangesOptions{
			Committer: &signature,
			Author:    &signature,
			Message:   "Initial Commit",
		}))
	}
}

func doGitAddRemote(dstPath, remoteName string, u *url.URL) func(*testing.T) {
	return func(t *testing.T) {
		_, err := git.NewCommand("remote", "add", remoteName, u.String()).RunInDir(dstPath)
		assert.NoError(t, err)
	}
}

func doGitPushTestRepository(dstPath string, args ...string) func(*testing.T) {
	return func(t *testing.T) {
		_, err := git.NewCommand(append([]string{"push", "-u"}, args...)...).RunInDir(dstPath)
		assert.NoError(t, err)
	}
}

func doGitPushTestRepositoryFail(dstPath string, args ...string) func(*testing.T) {
	return func(t *testing.T) {
		_, err := git.NewCommand(append([]string{"push"}, args...)...).RunInDir(dstPath)
		assert.Error(t, err)
	}
}

func doGitCreateBranch(dstPath, branch string) func(*testing.T) {
	return func(t *testing.T) {
		_, err := git.NewCommand("checkout", "-b", branch).RunInDir(dstPath)
		assert.NoError(t, err)
	}
}

func doGitCheckoutBranch(dstPath string, args ...string) func(*testing.T) {
	return func(t *testing.T) {
		_, err := git.NewCommandNoGlobals(append(append(allowLFSFilters(), "checkout"), args...)...).RunInDir(dstPath)
		assert.NoError(t, err)
	}
}

func doGitMerge(dstPath string, args ...string) func(*testing.T) {
	return func(t *testing.T) {
		_, err := git.NewCommand(append([]string{"merge"}, args...)...).RunInDir(dstPath)
		assert.NoError(t, err)
	}
}

func doGitPull(dstPath string, args ...string) func(*testing.T) {
	return func(t *testing.T) {
		_, err := git.NewCommandNoGlobals(append(append(allowLFSFilters(), "pull"), args...)...).RunInDir(dstPath)
		assert.NoError(t, err)
	}
}
