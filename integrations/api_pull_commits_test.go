// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"
	"github.com/stretchr/testify/assert"
)

func TestAPIPullCommits(t *testing.T) {
	defer prepareTestEnv(t)()
	pullIssue := models.AssertExistsAndLoadBean(t, &models.PullRequest{ID: 2}).(*models.PullRequest)
	assert.NoError(t, pullIssue.LoadIssue())
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: pullIssue.HeadRepoID}).(*models.Repository)

	session := loginUser(t, "user2")
	req := NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/pulls/%d/commits", repo.OwnerName, repo.Name, pullIssue.Index)
	resp := session.MakeRequest(t, req, http.StatusOK)

	var commits []*api.Commit
	DecodeJSON(t, resp, &commits)

	if !assert.Len(t, commits, 2) {
		return
	}

	assert.Equal(t, "5f22f7d0d95d614d25a5b68592adb345a4b5c7fd", commits[0].SHA)
	assert.Equal(t, "4a357436d925b5c974181ff12a994538ddc5a269", commits[1].SHA)
}

// TODO add tests for already merged PR and closed PR
