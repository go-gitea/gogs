// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"testing"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatrixPayload(t *testing.T) {
	t.Run("Create", func(t *testing.T) {
		p := createTestPayload()

		d := new(MatrixPayloadUnsafe)
		pl, err := d.Create(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MatrixPayloadUnsafe{}, pl)

		assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo):[test](http://localhost:3000/test/repo/src/branch/test)] branch created by user1", pl.(*MatrixPayloadUnsafe).Body)
		assert.Equal(t, `[<a href="http://localhost:3000/test/repo">test/repo</a>:<a href="http://localhost:3000/test/repo/src/branch/test">test</a>] branch created by user1`, pl.(*MatrixPayloadUnsafe).FormattedBody)
	})

	t.Run("Delete", func(t *testing.T) {
		p := deleteTestPayload()

		d := new(MatrixPayloadUnsafe)
		pl, err := d.Delete(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MatrixPayloadUnsafe{}, pl)

		assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo):test] branch deleted by user1", pl.(*MatrixPayloadUnsafe).Body)
		assert.Equal(t, `[<a href="http://localhost:3000/test/repo">test/repo</a>:test] branch deleted by user1`, pl.(*MatrixPayloadUnsafe).FormattedBody)
	})

	t.Run("Fork", func(t *testing.T) {
		p := forkTestPayload()

		d := new(MatrixPayloadUnsafe)
		pl, err := d.Fork(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MatrixPayloadUnsafe{}, pl)

		assert.Equal(t, "[test/repo2](http://localhost:3000/test/repo2) is forked to [test/repo](http://localhost:3000/test/repo)", pl.(*MatrixPayloadUnsafe).Body)
		assert.Equal(t, `<a href="http://localhost:3000/test/repo2">test/repo2</a> is forked to <a href="http://localhost:3000/test/repo">test/repo</a>`, pl.(*MatrixPayloadUnsafe).FormattedBody)
	})

	t.Run("Push", func(t *testing.T) {
		p := pushTestPayload()

		d := new(MatrixPayloadUnsafe)
		pl, err := d.Push(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MatrixPayloadUnsafe{}, pl)

		assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] user1 pushed 2 commits to [test](http://localhost:3000/test/repo/src/branch/test):\n[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778): commit message - user1\n[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778): commit message - user1", pl.(*MatrixPayloadUnsafe).Body)
		assert.Equal(t, `[<a href="http://localhost:3000/test/repo">test/repo</a>] user1 pushed 2 commits to <a href="http://localhost:3000/test/repo/src/branch/test">test</a>:<br><a href="http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778">2020558</a>: commit message - user1<br><a href="http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778">2020558</a>: commit message - user1`, pl.(*MatrixPayloadUnsafe).FormattedBody)
	})

	t.Run("Issue", func(t *testing.T) {
		p := issueTestPayload()

		d := new(MatrixPayloadUnsafe)
		p.Action = api.HookIssueOpened
		pl, err := d.Issue(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MatrixPayloadUnsafe{}, pl)

		assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] Issue opened: [#2 crash](http://localhost:3000/test/repo/issues/2) by [user1](https://try.gitea.io/user1)", pl.(*MatrixPayloadUnsafe).Body)
		assert.Equal(t, `[<a href="http://localhost:3000/test/repo">test/repo</a>] Issue opened: <a href="http://localhost:3000/test/repo/issues/2">#2 crash</a> by <a href="https://try.gitea.io/user1">user1</a>`, pl.(*MatrixPayloadUnsafe).FormattedBody)

		p.Action = api.HookIssueClosed
		pl, err = d.Issue(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MatrixPayloadUnsafe{}, pl)

		assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] Issue closed: [#2 crash](http://localhost:3000/test/repo/issues/2) by [user1](https://try.gitea.io/user1)", pl.(*MatrixPayloadUnsafe).Body)
		assert.Equal(t, `[<a href="http://localhost:3000/test/repo">test/repo</a>] Issue closed: <a href="http://localhost:3000/test/repo/issues/2">#2 crash</a> by <a href="https://try.gitea.io/user1">user1</a>`, pl.(*MatrixPayloadUnsafe).FormattedBody)
	})

	t.Run("IssueComment", func(t *testing.T) {
		p := issueCommentTestPayload()

		d := new(MatrixPayloadUnsafe)
		pl, err := d.IssueComment(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MatrixPayloadUnsafe{}, pl)

		assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] New comment on issue [#2 crash](http://localhost:3000/test/repo/issues/2) by [user1](https://try.gitea.io/user1)", pl.(*MatrixPayloadUnsafe).Body)
		assert.Equal(t, `[<a href="http://localhost:3000/test/repo">test/repo</a>] New comment on issue <a href="http://localhost:3000/test/repo/issues/2">#2 crash</a> by <a href="https://try.gitea.io/user1">user1</a>`, pl.(*MatrixPayloadUnsafe).FormattedBody)
	})

	t.Run("PullRequest", func(t *testing.T) {
		p := pullRequestTestPayload()

		d := new(MatrixPayloadUnsafe)
		pl, err := d.PullRequest(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MatrixPayloadUnsafe{}, pl)

		assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] Pull request opened: [#12 Fix bug](http://localhost:3000/test/repo/pulls/12) by [user1](https://try.gitea.io/user1)", pl.(*MatrixPayloadUnsafe).Body)
		assert.Equal(t, `[<a href="http://localhost:3000/test/repo">test/repo</a>] Pull request opened: <a href="http://localhost:3000/test/repo/pulls/12">#12 Fix bug</a> by <a href="https://try.gitea.io/user1">user1</a>`, pl.(*MatrixPayloadUnsafe).FormattedBody)
	})

	t.Run("PullRequestComment", func(t *testing.T) {
		p := pullRequestCommentTestPayload()

		d := new(MatrixPayloadUnsafe)
		pl, err := d.IssueComment(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MatrixPayloadUnsafe{}, pl)

		assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] New comment on pull request [#12 Fix bug](http://localhost:3000/test/repo/pulls/12) by [user1](https://try.gitea.io/user1)", pl.(*MatrixPayloadUnsafe).Body)
		assert.Equal(t, `[<a href="http://localhost:3000/test/repo">test/repo</a>] New comment on pull request <a href="http://localhost:3000/test/repo/pulls/12">#12 Fix bug</a> by <a href="https://try.gitea.io/user1">user1</a>`, pl.(*MatrixPayloadUnsafe).FormattedBody)
	})

	t.Run("Review", func(t *testing.T) {
		p := pullRequestTestPayload()
		p.Action = api.HookIssueReviewed

		d := new(MatrixPayloadUnsafe)
		pl, err := d.Review(p, models.HookEventPullRequestReviewApproved)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MatrixPayloadUnsafe{}, pl)

		assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] Pull request review approved: [#12 Fix bug](http://localhost:3000/test/repo/pulls/12) by [user1](https://try.gitea.io/user1)", pl.(*MatrixPayloadUnsafe).Body)
		assert.Equal(t, `[<a href="http://localhost:3000/test/repo">test/repo</a>] Pull request review approved: [#12 Fix bug](http://localhost:3000/test/repo/pulls/12) by <a href="https://try.gitea.io/user1">user1</a>`, pl.(*MatrixPayloadUnsafe).FormattedBody)
	})

	t.Run("Repository", func(t *testing.T) {
		p := repositoryTestPayload()

		d := new(MatrixPayloadUnsafe)
		pl, err := d.Repository(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MatrixPayloadUnsafe{}, pl)

		assert.Equal(t, `[[test/repo](http://localhost:3000/test/repo)] Repository created by [user1](https://try.gitea.io/user1)`, pl.(*MatrixPayloadUnsafe).Body)
		assert.Equal(t, `[<a href="http://localhost:3000/test/repo">test/repo</a>] Repository created by <a href="https://try.gitea.io/user1">user1</a>`, pl.(*MatrixPayloadUnsafe).FormattedBody)
	})

	t.Run("Release", func(t *testing.T) {
		p := pullReleaseTestPayload()

		d := new(MatrixPayloadUnsafe)
		pl, err := d.Release(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MatrixPayloadUnsafe{}, pl)

		assert.Equal(t, "[[test/repo](http://localhost:3000/test/repo)] Release created: [v1.0](http://localhost:3000/test/repo/src/v1.0) by [user1](https://try.gitea.io/user1)", pl.(*MatrixPayloadUnsafe).Body)
		assert.Equal(t, `[<a href="http://localhost:3000/test/repo">test/repo</a>] Release created: <a href="http://localhost:3000/test/repo/src/v1.0">v1.0</a> by <a href="https://try.gitea.io/user1">user1</a>`, pl.(*MatrixPayloadUnsafe).FormattedBody)
	})
}

func TestMatrixJSONPayload(t *testing.T) {
	p := pushTestPayload()

	pl, err := new(MatrixPayloadUnsafe).Push(p)
	require.NoError(t, err)
	require.NotNil(t, pl)
	require.IsType(t, &MatrixPayloadUnsafe{}, pl)

	json, err := pl.JSONPayload()
	require.NoError(t, err)
	assert.NotEmpty(t, json)
}

func TestMatrixHookRequest(t *testing.T) {
	w := &models.Webhook{}

	h := &models.HookTask{
		PayloadContent: `{
  "body": "[[user1/test](http://localhost:3000/user1/test)] user1 pushed 1 commit to [master](http://localhost:3000/user1/test/src/branch/master):\n[5175ef2](http://localhost:3000/user1/test/commit/5175ef26201c58b035a3404b3fe02b4e8d436eee): Merge pull request 'Change readme.md' (#2) from add-matrix-webhook into master\n\nReviewed-on: http://localhost:3000/user1/test/pulls/2\n - user1",
  "msgtype": "m.notice",
  "format": "org.matrix.custom.html",
  "formatted_body": "[\u003ca href=\"http://localhost:3000/user1/test\"\u003euser1/test\u003c/a\u003e] user1 pushed 1 commit to \u003ca href=\"http://localhost:3000/user1/test/src/branch/master\"\u003emaster\u003c/a\u003e:\u003cbr\u003e\u003ca href=\"http://localhost:3000/user1/test/commit/5175ef26201c58b035a3404b3fe02b4e8d436eee\"\u003e5175ef2\u003c/a\u003e: Merge pull request 'Change readme.md' (#2) from add-matrix-webhook into master\n\nReviewed-on: http://localhost:3000/user1/test/pulls/2\n - user1",
  "io.gitea.commits": [
    {
      "id": "5175ef26201c58b035a3404b3fe02b4e8d436eee",
      "message": "Merge pull request 'Change readme.md' (#2) from add-matrix-webhook into master\n\nReviewed-on: http://localhost:3000/user1/test/pulls/2\n",
      "url": "http://localhost:3000/user1/test/commit/5175ef26201c58b035a3404b3fe02b4e8d436eee",
      "author": {
        "name": "user1",
        "email": "user@mail.com",
        "username": ""
      },
      "committer": {
        "name": "user1",
        "email": "user@mail.com",
        "username": ""
      },
      "verification": null,
      "timestamp": "0001-01-01T00:00:00Z",
      "added": null,
      "removed": null,
      "modified": null
    }
  ],
  "access_token": "dummy_access_token"
}`,
	}

	wantPayloadContent := `{
  "body": "[[user1/test](http://localhost:3000/user1/test)] user1 pushed 1 commit to [master](http://localhost:3000/user1/test/src/branch/master):\n[5175ef2](http://localhost:3000/user1/test/commit/5175ef26201c58b035a3404b3fe02b4e8d436eee): Merge pull request 'Change readme.md' (#2) from add-matrix-webhook into master\n\nReviewed-on: http://localhost:3000/user1/test/pulls/2\n - user1",
  "msgtype": "m.notice",
  "format": "org.matrix.custom.html",
  "formatted_body": "[\u003ca href=\"http://localhost:3000/user1/test\"\u003euser1/test\u003c/a\u003e] user1 pushed 1 commit to \u003ca href=\"http://localhost:3000/user1/test/src/branch/master\"\u003emaster\u003c/a\u003e:\u003cbr\u003e\u003ca href=\"http://localhost:3000/user1/test/commit/5175ef26201c58b035a3404b3fe02b4e8d436eee\"\u003e5175ef2\u003c/a\u003e: Merge pull request 'Change readme.md' (#2) from add-matrix-webhook into master\n\nReviewed-on: http://localhost:3000/user1/test/pulls/2\n - user1",
  "io.gitea.commits": [
    {
      "id": "5175ef26201c58b035a3404b3fe02b4e8d436eee",
      "message": "Merge pull request 'Change readme.md' (#2) from add-matrix-webhook into master\n\nReviewed-on: http://localhost:3000/user1/test/pulls/2\n",
      "url": "http://localhost:3000/user1/test/commit/5175ef26201c58b035a3404b3fe02b4e8d436eee",
      "author": {
        "name": "user1",
        "email": "user@mail.com",
        "username": ""
      },
      "committer": {
        "name": "user1",
        "email": "user@mail.com",
        "username": ""
      },
      "verification": null,
      "timestamp": "0001-01-01T00:00:00Z",
      "added": null,
      "removed": null,
      "modified": null
    }
  ]
}`

	req, err := getMatrixHookRequest(w, h)
	require.NoError(t, err)
	require.NotNil(t, req)

	assert.Equal(t, "Bearer dummy_access_token", req.Header.Get("Authorization"))
	assert.Equal(t, wantPayloadContent, h.PayloadContent)
}

func Test_getTxnID(t *testing.T) {
	type args struct {
		payload []byte
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name:    "dummy payload",
			args:    args{payload: []byte("Hello World")},
			want:    "0a4d55a8d778e5022fab701977c5d840bbc486d0",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getMatrixTxnID(tt.args.payload)
			if (err != nil) != tt.wantErr {
				t.Errorf("getMatrixTxnID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
