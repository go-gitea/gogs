// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/test"
	"github.com/stretchr/testify/assert"
)

func TestUpdateIssuePriority(t *testing.T) {
	models.PrepareTestEnv(t)

	ctx := test.MockContext(t, "user2/repo1/issues/1/pin")
	test.LoadUser(t, ctx, 2)
	test.LoadRepo(t, ctx, 1)
	test.LoadGitRepo(t, ctx)
	PinIssue(ctx)
	assert.EqualValues(t, http.StatusOK, ctx.Resp.Status())
	models.AssertExistsAndLoadBean(t, &models.Issue{
		ID: 1,
	}, models.Cond("priority = ?", 10))

	ctx = test.MockContext(t, "user2/repo1/issues/1/unpin")
	test.LoadUser(t, ctx, 2)
	test.LoadRepo(t, ctx, 1)
	test.LoadGitRepo(t, ctx)
	UnpinIssue(ctx)
	assert.EqualValues(t, http.StatusOK, ctx.Resp.Status())
	models.AssertExistsAndLoadBean(t, &models.Issue{
		ID: 1,
	}, models.Cond("priority = ?", models.PriorityDefault))
}
