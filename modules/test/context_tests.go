// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package test

import (
	scontext "context"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/web/middleware"

	"github.com/go-chi/chi"
	"github.com/stretchr/testify/assert"
	"github.com/unrolled/render"
)

// MockContext mock context for unit tests
func MockContext(t *testing.T, path string) *context.Context {
	requestURL, err := url.Parse(path)
	assert.NoError(t, err)

	var resp = &mockResponseWriter{}
	var req = &http.Request{
		URL:  requestURL,
		Form: url.Values{},
	}
	var ctx = context.Context{
		BaseContext: context.NewBaseContext(resp, req, map[string]interface{}{}),
		Render:      &mockRender{},
		Flash: &middleware.Flash{
			Values: make(url.Values),
		},
		Locale: &mockLocale{},
	}

	chiCtx := chi.NewRouteContext()
	req = req.WithContext(scontext.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))
	ctx.Req = context.WithContext(req, &ctx)
	return &ctx
}

// LoadRepo load a repo into a test context.
func LoadRepo(t *testing.T, ctx *context.Context, repoID int64) {
	ctx.Repo = &context.Repository{}
	ctx.Repo.Repository = models.AssertExistsAndLoadBean(t, &models.Repository{ID: repoID}).(*models.Repository)
	var err error
	ctx.Repo.Owner, err = models.GetUserByID(ctx.Repo.Repository.OwnerID)
	assert.NoError(t, err)
	ctx.Repo.RepoLink = ctx.Repo.Repository.Link()
	ctx.Repo.Permission, err = models.GetUserRepoPermission(ctx.Repo.Repository, ctx.User)
	assert.NoError(t, err)
}

// LoadRepoCommit loads a repo's commit into a test context.
func LoadRepoCommit(t *testing.T, ctx *context.Context) {
	gitRepo, err := git.OpenRepository(ctx.Repo.Repository.RepoPath())
	assert.NoError(t, err)
	defer gitRepo.Close()
	branch, err := gitRepo.GetHEADBranch()
	assert.NoError(t, err)
	assert.NotNil(t, branch)
	if branch != nil {
		ctx.Repo.Commit, err = gitRepo.GetBranchCommit(branch.Name)
		assert.NoError(t, err)
	}
}

// LoadUser load a user into a test context.
func LoadUser(t *testing.T, ctx *context.Context, userID int64) {
	ctx.User = models.AssertExistsAndLoadBean(t, &models.User{ID: userID}).(*models.User)
}

// LoadGitRepo load a git repo into a test context. Requires that ctx.Repo has
// already been populated.
func LoadGitRepo(t *testing.T, ctx *context.Context) {
	assert.NoError(t, ctx.Repo.Repository.GetOwner())
	var err error
	ctx.Repo.GitRepo, err = git.OpenRepository(ctx.Repo.Repository.RepoPath())
	assert.NoError(t, err)
}

type mockLocale struct{}

func (l mockLocale) Language() string {
	return "en"
}

func (l mockLocale) Tr(s string, _ ...interface{}) string {
	return s
}

type mockResponseWriter struct {
	httptest.ResponseRecorder
	size int
}

func (rw *mockResponseWriter) Write(b []byte) (int, error) {
	rw.size += len(b)
	return rw.ResponseRecorder.Write(b)
}

func (rw *mockResponseWriter) Status() int {
	return rw.ResponseRecorder.Code
}

func (rw *mockResponseWriter) Written() bool {
	return rw.ResponseRecorder.Code > 0
}

func (rw *mockResponseWriter) Size() int {
	return rw.size
}

func (rw *mockResponseWriter) Push(target string, opts *http.PushOptions) error {
	return nil
}

type mockRender struct {
}

func (tr *mockRender) TemplateLookup(tmpl string) *template.Template {
	return nil
}

func (tr *mockRender) HTML(w io.Writer, status int, _ string, _ interface{}, _ ...render.HTMLOptions) error {
	if resp, ok := w.(http.ResponseWriter); ok {
		resp.WriteHeader(status)
	}
	return nil
}
