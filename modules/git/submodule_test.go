// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestGetRefURL(t *testing.T) {
	var kases = []struct {
		refURL       string
		prefixURL    string
		parentPath   string
		SSHDomain    string
		expect       string
		subModuleMap map[string]string
	}{
		{"git://github.com/user1/repo1", "/", "user1/repo2", "", "http://github.com/user1/repo1", map[string]string{}},
		{"https://localhost/user1/repo1.git", "/", "user1/repo2", "", "https://localhost/user1/repo1", map[string]string{}},
		{"http://localhost/user1/repo1.git", "/", "owner/reponame", "", "http://localhost/user1/repo1", map[string]string{}},
		{"git@github.com:user1/repo1.git", "/", "owner/reponame", "", "http://github.com/user1/repo1", map[string]string{}},
		{"ssh://git@git.zefie.net:2222/zefie/lge_g6_kernel_scripts.git", "/", "zefie/lge_g6_kernel", "", "http://git.zefie.net/zefie/lge_g6_kernel_scripts", map[string]string{}},
		{"git@git.zefie.net:2222/zefie/lge_g6_kernel_scripts.git", "/", "zefie/lge_g6_kernel", "", "http://git.zefie.net/2222/zefie/lge_g6_kernel_scripts", map[string]string{}},
		{"git@try.gitea.io:go-gitea/gitea", "https://try.gitea.io/", "go-gitea/sdk", "", "https://try.gitea.io/go-gitea/gitea", map[string]string{}},
		{"ssh://git@try.gitea.io:9999/go-gitea/gitea", "https://try.gitea.io/", "go-gitea/sdk", "", "https://try.gitea.io/go-gitea/gitea", map[string]string{}},
		{"git://git@try.gitea.io:9999/go-gitea/gitea", "https://try.gitea.io/", "go-gitea/sdk", "", "https://try.gitea.io/go-gitea/gitea", map[string]string{}},
		{"ssh://git@127.0.0.1:9999/go-gitea/gitea", "https://127.0.0.1:3000/", "go-gitea/sdk", "", "https://127.0.0.1:3000/go-gitea/gitea", map[string]string{}},
		{"https://gitea.com:3000/user1/repo1.git", "https://127.0.0.1:3000/", "user/repo2", "", "https://gitea.com:3000/user1/repo1", map[string]string{}},
		{"https://example.gitea.com/gitea/user1/repo1.git", "https://example.gitea.com/gitea/", "", "user/repo2", "https://example.gitea.com/gitea/user1/repo1", map[string]string{}},
		{"https://username:password@github.com/username/repository.git", "/", "username/repository2", "", "https://username:password@github.com/username/repository", map[string]string{}},
		{"somethingbad", "https://127.0.0.1:3000/go-gitea/gitea", "/", "", "", map[string]string{}},
		{"git@localhost:user/repo", "https://localhost/", "user2/repo1", "", "https://localhost/user/repo", map[string]string{}},
		{"../path/to/repo.git/", "https://localhost/", "user/repo2", "", "https://localhost/user/path/to/repo.git", map[string]string{}},
		{"ssh://git@ssh.gitea.io:2222/go-gitea/gitea", "https://try.gitea.io/", "go-gitea/sdk", "ssh.gitea.io", "https://try.gitea.io/go-gitea/gitea", map[string]string{}},
		{"ssh://git@ssh.gitea.io:2222/go-gitea/gitea", "https://try.gitea.io/", "go-gitea/sdk", "try.gitea.io", "https://try.gitea.io/go-gitea/gitea", map[string]string{
			"ssh://git@ssh.gitea.io:2222": "https://try.gitea.io",
		}},
		{"git@ssh.gitea.io:go-gitea/gitea", "https://try.gitea.io/", "go-gitea/sdk", "try.gitea.io", "https://try.gitea.io/go-gitea/gitea", map[string]string{
			"git@ssh.gitea.io":            "https://try.gitea.io",
			"ssh://git@ssh.gitea.io:2222": "Wrong",
		}},
		{"ssh://git@ssh.gitea.io/go-gitea/gitea", "https://try.gitea.io/", "go-gitea/sdk", "try.gitea.io", "https://try.gitea.io/go-gitea/gitea", map[string]string{
			"ssh://git@ssh.gitea.io":      "https://try.gitea.io",
			"ssh://git@ssh.gitea.io:2222": "Wrong",
		}},
		{"ssh://git@ssh.gitea.io/go-gitea/gitea", "https://try.gitea.io/", "go-gitea/sdk", "try.gitea.io", "https://try.gitea.io/go-gitea/gitea", map[string]string{
			"git@ssh.gitea.io": "https://try.gitea.io",
		}},
		{"ssh://git@ssh.gitea.io/go-gitea/gitea", "https://try.gitea.io/", "go-gitea/sdk", "try.gitea.io", "https://try.gitea.io/go-gitea/gitea", map[string]string{
			"git@ssh.gitea.io": "https://try.gitea.io",
		}},
		{"ssh://git@ssh.gitea.io/go-gitea/gitea", "https://try.gitea.io/", "go-gitea/sdk", "try.gitea.io", "https://try.gitea.io/go-gitea/gitea", map[string]string{
			"git@ssh.gitea.io:go-gitea/gitea": "https://try.gitea.io/go-gitea/gitea",
		}},
	}
	orig := setting.Git.SubModuleMap
	for _, kase := range kases {
		setting.Git.SubModuleMap = kase.subModuleMap
		assert.EqualValues(t, kase.expect, getRefURL(kase.refURL, kase.prefixURL, kase.parentPath, kase.SSHDomain))
	}
	setting.Git.SubModuleMap = orig
}
