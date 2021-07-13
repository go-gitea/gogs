// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package archiver

import (
	"path/filepath"
	"testing"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	models.MainTest(m, filepath.Join("..", ".."))
}

func waitForCount(t *testing.T, num int) {

}

func TestArchive_Basic(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())

	ctx := test.MockContext(t, "user27/repo49")
	firstCommit, secondCommit := "51f84af23134", "aacbdfe9e1c4"

	test.LoadRepo(t, ctx, 49)
	test.LoadGitRepo(t, ctx)
	defer ctx.Repo.GitRepo.Close()

	bogusReq, err := NewRequest(ctx.Repo.Repository.ID, ctx.Repo.GitRepo, firstCommit+".zip")
	assert.NoError(t, err)
	assert.NotNil(t, bogusReq)
	assert.EqualValues(t, firstCommit+".zip", bogusReq.GetArchiveName())

	// Check a series of bogus requests.
	// Step 1, valid commit with a bad extension.
	bogusReq, err = NewRequest(ctx.Repo.Repository.ID, ctx.Repo.GitRepo, firstCommit+".dilbert")
	assert.Error(t, err)
	assert.Nil(t, bogusReq)

	// Step 2, missing commit.
	bogusReq, err = NewRequest(ctx.Repo.Repository.ID, ctx.Repo.GitRepo, "dbffff.zip")
	assert.Error(t, err)
	assert.Nil(t, bogusReq)

	// Step 3, doesn't look like branch/tag/commit.
	bogusReq, err = NewRequest(ctx.Repo.Repository.ID, ctx.Repo.GitRepo, "db.zip")
	assert.Error(t, err)
	assert.Nil(t, bogusReq)

	bogusReq, err = NewRequest(ctx.Repo.Repository.ID, ctx.Repo.GitRepo, "master.zip")
	assert.NoError(t, err)
	assert.NotNil(t, bogusReq)
	assert.EqualValues(t, "master.zip", bogusReq.GetArchiveName())

	bogusReq, err = NewRequest(ctx.Repo.Repository.ID, ctx.Repo.GitRepo, "test/archive.zip")
	assert.NoError(t, err)
	assert.NotNil(t, bogusReq)
	assert.EqualValues(t, "test-archive.zip", bogusReq.GetArchiveName())

	// Now two valid requests, firstCommit with valid extensions.
	zipReq, err := NewRequest(ctx.Repo.Repository.ID, ctx.Repo.GitRepo, firstCommit+".zip")
	assert.NoError(t, err)
	assert.NotNil(t, zipReq)

	tgzReq, err := NewRequest(ctx.Repo.Repository.ID, ctx.Repo.GitRepo, firstCommit+".tar.gz")
	assert.NoError(t, err)
	assert.NotNil(t, tgzReq)

	secondReq, err := NewRequest(ctx.Repo.Repository.ID, ctx.Repo.GitRepo, secondCommit+".zip")
	assert.NoError(t, err)
	assert.NotNil(t, secondReq)

	inFlight := make([]*ArchiveRequest, 3)
	inFlight[0] = zipReq
	inFlight[1] = tgzReq
	inFlight[2] = secondReq

	ArchiveRepository(zipReq)
	waitForCount(t, 1)
	ArchiveRepository(tgzReq)
	waitForCount(t, 2)
	ArchiveRepository(secondReq)
	waitForCount(t, 3)

	// Make sure sending an unprocessed request through doesn't affect the queue
	// count.
	ArchiveRepository(zipReq)

	// Sleep two seconds to make sure the queue doesn't change.
	time.Sleep(2 * time.Second)

	zipReq2, err := NewRequest(ctx.Repo.Repository.ID, ctx.Repo.GitRepo, firstCommit+".zip")
	assert.NoError(t, err)
	// This zipReq should match what's sitting in the queue, as we haven't
	// let it release yet.  From the consumer's point of view, this looks like
	// a long-running archive task.
	assert.Equal(t, zipReq, zipReq2)

	// We still have the other three stalled at completion, waiting to remove
	// from archiveInProgress.  Try to submit this new one before its
	// predecessor has cleared out of the queue.
	ArchiveRepository(zipReq2)

	// Now we'll submit a request and TimedWaitForCompletion twice, before and
	// after we release it.  We should trigger both the timeout and non-timeout
	// cases.
	timedReq, err := NewRequest(ctx.Repo.Repository.ID, ctx.Repo.GitRepo, secondCommit+".tar.gz")
	assert.NoError(t, err)
	assert.NotNil(t, timedReq)
	ArchiveRepository(timedReq)

	zipReq2, err = NewRequest(ctx.Repo.Repository.ID, ctx.Repo.GitRepo, firstCommit+".zip")
	assert.NoError(t, err)
	// Now, we're guaranteed to have released the original zipReq from the queue.
	// Ensure that we don't get handed back the released entry somehow, but they
	// should remain functionally equivalent in all fields.  The exception here
	// is zipReq.cchan, which will be non-nil because it's a completed request.
	// It's fine to go ahead and set it to nil now.

	assert.Equal(t, zipReq, zipReq2)
	assert.False(t, zipReq == zipReq2)

	// Same commit, different compression formats should have different names.
	// Ideally, the extension would match what we originally requested.
	assert.NotEqual(t, zipReq.GetArchiveName(), tgzReq.GetArchiveName())
	assert.NotEqual(t, zipReq.GetArchiveName(), secondReq.GetArchiveName())
}
