// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestPullRequest_APIFormat(t *testing.T) {
	// with HeadRepo
	assert.NoError(t, models.PrepareTestDatabase())
	headRepo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	pr := models.AssertExistsAndLoadBean(t, &models.PullRequest{ID: 1}).(*models.PullRequest)
	assert.NoError(t, pr.LoadAttributes())
	assert.NoError(t, pr.LoadIssue())
	apiPullRequest := ToAPIPullRequest(pr)
	assert.NotNil(t, apiPullRequest)
	assert.EqualValues(t, &structs.PRBranchInfo{
		Name:       "branch1",
		Ref:        "refs/pull/2/head",
		Sha:        "4a357436d925b5c974181ff12a994538ddc5a269",
		RepoID:     1,
		Repository: ToRepo(headRepo, models.AccessModeNone),
	}, apiPullRequest.Head)

	// withOut HeadRepo
	pr = models.AssertExistsAndLoadBean(t, &models.PullRequest{ID: 1}).(*models.PullRequest)
	assert.NoError(t, pr.LoadIssue())
	assert.NoError(t, pr.LoadAttributes())
	// simulate fork deletion
	pr.HeadRepo = nil
	pr.HeadRepoID = 100000
	apiPullRequest = ToAPIPullRequest(pr)
	assert.NotNil(t, apiPullRequest)
	assert.Nil(t, apiPullRequest.Head.Repository)
	assert.EqualValues(t, -1, apiPullRequest.Head.RepoID)
}
