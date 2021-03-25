// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

// getRepoEditOptionFromRepo gets the options for an existing repo exactly as is
func getRepoEditOptionFromRepo(repo *models.Repository) *api.EditRepoOption {
	name := repo.Name
	description := repo.Description
	website := repo.Website
	private := repo.IsPrivate
	hasIssues := false
	var internalTracker *api.InternalTracker
	var externalTracker *api.ExternalTracker
	if unit, err := repo.GetUnit(models.UnitTypeIssues); err == nil {
		config := unit.IssuesConfig()
		hasIssues = true
		internalTracker = &api.InternalTracker{
			EnableTimeTracker:                config.EnableTimetracker,
			AllowOnlyContributorsToTrackTime: config.AllowOnlyContributorsToTrackTime,
			EnableIssueDependencies:          config.EnableDependencies,
		}
	} else if unit, err := repo.GetUnit(models.UnitTypeExternalTracker); err == nil {
		config := unit.ExternalTrackerConfig()
		hasIssues = true
		externalTracker = &api.ExternalTracker{
			ExternalTrackerURL:    config.ExternalTrackerURL,
			ExternalTrackerFormat: config.ExternalTrackerFormat,
			ExternalTrackerStyle:  config.ExternalTrackerStyle,
		}
	}
	hasWiki := false
	var externalWiki *api.ExternalWiki
	if _, err := repo.GetUnit(models.UnitTypeWiki); err == nil {
		hasWiki = true
	} else if unit, err := repo.GetUnit(models.UnitTypeExternalWiki); err == nil {
		hasWiki = true
		config := unit.ExternalWikiConfig()
		externalWiki = &api.ExternalWiki{
			ExternalWikiURL: config.ExternalWikiURL,
		}
	}
	defaultBranch := repo.DefaultBranch
	hasPullRequests := false
	ignoreWhitespaceConflicts := false
	allowMerge := false
	allowRebase := false
	allowRebaseMerge := false
	allowSquash := false
	if unit, err := repo.GetUnit(models.UnitTypePullRequests); err == nil {
		config := unit.PullRequestsConfig()
		hasPullRequests = true
		ignoreWhitespaceConflicts = config.IgnoreWhitespaceConflicts
		allowMerge = config.AllowMerge
		allowRebase = config.AllowRebase
		allowRebaseMerge = config.AllowRebaseMerge
		allowSquash = config.AllowSquash
	}
	archived := repo.IsArchived
	return &api.EditRepoOption{
		Name:                      &name,
		Description:               &description,
		Website:                   &website,
		Private:                   &private,
		HasIssues:                 &hasIssues,
		ExternalTracker:           externalTracker,
		InternalTracker:           internalTracker,
		HasWiki:                   &hasWiki,
		ExternalWiki:              externalWiki,
		DefaultBranch:             &defaultBranch,
		HasPullRequests:           &hasPullRequests,
		IgnoreWhitespaceConflicts: &ignoreWhitespaceConflicts,
		AllowMerge:                &allowMerge,
		AllowRebase:               &allowRebase,
		AllowRebaseMerge:          &allowRebaseMerge,
		AllowSquash:               &allowSquash,
		Archived:                  &archived,
	}
}

// getNewRepoEditOption Gets the options to change everything about an existing repo by adding to strings or changing
// the boolean
func getNewRepoEditOption(opts *api.EditRepoOption) *api.EditRepoOption {
	// Gives a new property to everything
	name := *opts.Name + "renamed"
	description := "new description"
	website := "http://wwww.newwebsite.com"
	private := !*opts.Private
	hasIssues := !*opts.HasIssues
	hasWiki := !*opts.HasWiki
	defaultBranch := "master"
	hasPullRequests := !*opts.HasPullRequests
	ignoreWhitespaceConflicts := !*opts.IgnoreWhitespaceConflicts
	allowMerge := !*opts.AllowMerge
	allowRebase := !*opts.AllowRebase
	allowRebaseMerge := !*opts.AllowRebaseMerge
	allowSquash := !*opts.AllowSquash
	archived := !*opts.Archived

	return &api.EditRepoOption{
		Name:                      &name,
		Description:               &description,
		Website:                   &website,
		Private:                   &private,
		DefaultBranch:             &defaultBranch,
		HasIssues:                 &hasIssues,
		HasWiki:                   &hasWiki,
		HasPullRequests:           &hasPullRequests,
		IgnoreWhitespaceConflicts: &ignoreWhitespaceConflicts,
		AllowMerge:                &allowMerge,
		AllowRebase:               &allowRebase,
		AllowRebaseMerge:          &allowRebaseMerge,
		AllowSquash:               &allowSquash,
		Archived:                  &archived,
	}
}

func TestAPIRepoEdit(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)               // owner of the repo1 & repo16
		user3 := models.AssertExistsAndLoadBean(t, &models.User{ID: 3}).(*models.User)               // owner of the repo3, is an org
		user4 := models.AssertExistsAndLoadBean(t, &models.User{ID: 4}).(*models.User)               // owner of neither repos
		repo1 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)   // public repo
		repo3 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 3}).(*models.Repository)   // public repo
		repo16 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 16}).(*models.Repository) // private repo

		// Get user2's token
		session := loginUser(t, user2.Name)
		token2 := getTokenForLoggedInUser(t, session)
		session = emptyTestSession(t)
		// Get user4's token
		session = loginUser(t, user4.Name)
		token4 := getTokenForLoggedInUser(t, session)
		session = emptyTestSession(t)

		// Test editing a repo1 which user2 owns, changing name and many properties
		origRepoEditOption := getRepoEditOptionFromRepo(repo1)
		repoEditOption := getNewRepoEditOption(origRepoEditOption)
		url := fmt.Sprintf("/api/v1/repos/%s/%s?token=%s", user2.Name, repo1.Name, token2)
		req := NewRequestWithJSON(t, "PATCH", url, &repoEditOption)
		resp := session.MakeRequest(t, req, http.StatusOK)
		var repo api.Repository
		DecodeJSON(t, resp, &repo)
		assert.NotNil(t, repo)
		// check response
		assert.Equal(t, *repoEditOption.Name, repo.Name)
		assert.Equal(t, *repoEditOption.Description, repo.Description)
		assert.Equal(t, *repoEditOption.Website, repo.Website)
		assert.Equal(t, *repoEditOption.Archived, repo.Archived)
		// check repo1 from database
		repo1edited := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
		repo1editedOption := getRepoEditOptionFromRepo(repo1edited)
		assert.Equal(t, *repoEditOption.Name, *repo1editedOption.Name)
		assert.Equal(t, *repoEditOption.Description, *repo1editedOption.Description)
		assert.Equal(t, *repoEditOption.Website, *repo1editedOption.Website)
		assert.Equal(t, *repoEditOption.Archived, *repo1editedOption.Archived)
		assert.Equal(t, *repoEditOption.Private, *repo1editedOption.Private)
		assert.Equal(t, *repoEditOption.HasWiki, *repo1editedOption.HasWiki)

		// Test editing repo1 to use internal issue and wiki (default)
		*repoEditOption.HasIssues = true
		repoEditOption.ExternalTracker = nil
		repoEditOption.InternalTracker = &api.InternalTracker{
			EnableTimeTracker:                false,
			AllowOnlyContributorsToTrackTime: false,
			EnableIssueDependencies:          false,
		}
		*repoEditOption.HasWiki = true
		repoEditOption.ExternalWiki = nil
		url = fmt.Sprintf("/api/v1/repos/%s/%s?token=%s", user2.Name, *repoEditOption.Name, token2)
		req = NewRequestWithJSON(t, "PATCH", url, &repoEditOption)
		resp = session.MakeRequest(t, req, http.StatusOK)
		DecodeJSON(t, resp, &repo)
		assert.NotNil(t, repo)
		// check repo1 was written to database
		repo1edited = models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
		repo1editedOption = getRepoEditOptionFromRepo(repo1edited)
		assert.Equal(t, *repo1editedOption.HasIssues, true)
		assert.Nil(t, repo1editedOption.ExternalTracker)
		assert.Equal(t, *repo1editedOption.InternalTracker, *repoEditOption.InternalTracker)
		assert.Equal(t, *repo1editedOption.HasWiki, true)
		assert.Nil(t, repo1editedOption.ExternalWiki)

		// Test editing repo1 to use external issue and wiki
		repoEditOption.ExternalTracker = &api.ExternalTracker{
			ExternalTrackerURL:    "http://www.somewebsite.com",
			ExternalTrackerFormat: "http://www.somewebsite.com/{user}/{repo}?issue={index}",
			ExternalTrackerStyle:  "alphanumeric",
		}
		repoEditOption.ExternalWiki = &api.ExternalWiki{
			ExternalWikiURL: "http://www.somewebsite.com",
		}
		req = NewRequestWithJSON(t, "PATCH", url, &repoEditOption)
		resp = session.MakeRequest(t, req, http.StatusOK)
		DecodeJSON(t, resp, &repo)
		assert.NotNil(t, repo)
		// check repo1 was written to database
		repo1edited = models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
		repo1editedOption = getRepoEditOptionFromRepo(repo1edited)
		assert.Equal(t, *repo1editedOption.HasIssues, true)
		assert.Equal(t, *repo1editedOption.ExternalTracker, *repoEditOption.ExternalTracker)
		assert.Equal(t, *repo1editedOption.HasWiki, true)
		assert.Equal(t, *repo1editedOption.ExternalWiki, *repoEditOption.ExternalWiki)

		// Do some tests with invalid URL for external tracker and wiki
		repoEditOption.ExternalTracker.ExternalTrackerURL = "htp://www.somewebsite.com"
		req = NewRequestWithJSON(t, "PATCH", url, &repoEditOption)
		resp = session.MakeRequest(t, req, http.StatusUnprocessableEntity)
		repoEditOption.ExternalTracker.ExternalTrackerURL = "http://www.somewebsite.com"
		repoEditOption.ExternalTracker.ExternalTrackerFormat = "http://www.somewebsite.com/{user/{repo}?issue={index}"
		req = NewRequestWithJSON(t, "PATCH", url, &repoEditOption)
		resp = session.MakeRequest(t, req, http.StatusUnprocessableEntity)
		repoEditOption.ExternalTracker.ExternalTrackerFormat = "http://www.somewebsite.com/{user}/{repo}?issue={index}"
		repoEditOption.ExternalWiki.ExternalWikiURL = "htp://www.somewebsite.com"
		req = NewRequestWithJSON(t, "PATCH", url, &repoEditOption)
		resp = session.MakeRequest(t, req, http.StatusUnprocessableEntity)

		// Test small repo change through API with issue and wiki option not set; They shall not be touched.
		*repoEditOption.Description = "small change"
		repoEditOption.HasIssues = nil
		repoEditOption.ExternalTracker = nil
		repoEditOption.HasWiki = nil
		repoEditOption.ExternalWiki = nil
		req = NewRequestWithJSON(t, "PATCH", url, &repoEditOption)
		resp = session.MakeRequest(t, req, http.StatusOK)
		DecodeJSON(t, resp, &repo)
		assert.NotNil(t, repo)
		// check repo1 was written to database
		repo1edited = models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
		repo1editedOption = getRepoEditOptionFromRepo(repo1edited)
		assert.Equal(t, *repo1editedOption.Description, *repoEditOption.Description)
		assert.Equal(t, *repo1editedOption.HasIssues, true)
		assert.NotNil(t, *repo1editedOption.ExternalTracker)
		assert.Equal(t, *repo1editedOption.HasWiki, true)
		assert.NotNil(t, *repo1editedOption.ExternalWiki)

		// reset repo in db
		url = fmt.Sprintf("/api/v1/repos/%s/%s?token=%s", user2.Name, *repoEditOption.Name, token2)
		req = NewRequestWithJSON(t, "PATCH", url, &origRepoEditOption)
		_ = session.MakeRequest(t, req, http.StatusOK)

		// Test editing a non-existing repo
		name := "repodoesnotexist"
		url = fmt.Sprintf("/api/v1/repos/%s/%s?token=%s", user2.Name, name, token2)
		req = NewRequestWithJSON(t, "PATCH", url, &api.EditRepoOption{Name: &name})
		_ = session.MakeRequest(t, req, http.StatusNotFound)

		// Test editing repo16 by user4 who does not have write access
		origRepoEditOption = getRepoEditOptionFromRepo(repo16)
		repoEditOption = getNewRepoEditOption(origRepoEditOption)
		url = fmt.Sprintf("/api/v1/repos/%s/%s?token=%s", user2.Name, repo16.Name, token4)
		req = NewRequestWithJSON(t, "PATCH", url, &repoEditOption)
		session.MakeRequest(t, req, http.StatusNotFound)

		// Tests a repo with no token given so will fail
		origRepoEditOption = getRepoEditOptionFromRepo(repo16)
		repoEditOption = getNewRepoEditOption(origRepoEditOption)
		url = fmt.Sprintf("/api/v1/repos/%s/%s", user2.Name, repo16.Name)
		req = NewRequestWithJSON(t, "PATCH", url, &repoEditOption)
		_ = session.MakeRequest(t, req, http.StatusNotFound)

		// Test using access token for a private repo that the user of the token owns
		origRepoEditOption = getRepoEditOptionFromRepo(repo16)
		repoEditOption = getNewRepoEditOption(origRepoEditOption)
		url = fmt.Sprintf("/api/v1/repos/%s/%s?token=%s", user2.Name, repo16.Name, token2)
		req = NewRequestWithJSON(t, "PATCH", url, &repoEditOption)
		_ = session.MakeRequest(t, req, http.StatusOK)
		// reset repo in db
		url = fmt.Sprintf("/api/v1/repos/%s/%s?token=%s", user2.Name, *repoEditOption.Name, token2)
		req = NewRequestWithJSON(t, "PATCH", url, &origRepoEditOption)
		_ = session.MakeRequest(t, req, http.StatusOK)

		// Test making a repo public that is private
		repo16 = models.AssertExistsAndLoadBean(t, &models.Repository{ID: 16}).(*models.Repository)
		assert.True(t, repo16.IsPrivate)
		private := false
		repoEditOption = &api.EditRepoOption{
			Private: &private,
		}
		url = fmt.Sprintf("/api/v1/repos/%s/%s?token=%s", user2.Name, repo16.Name, token2)
		req = NewRequestWithJSON(t, "PATCH", url, &repoEditOption)
		_ = session.MakeRequest(t, req, http.StatusOK)
		repo16 = models.AssertExistsAndLoadBean(t, &models.Repository{ID: 16}).(*models.Repository)
		assert.False(t, repo16.IsPrivate)
		// Make it private again
		private = true
		repoEditOption.Private = &private
		req = NewRequestWithJSON(t, "PATCH", url, &repoEditOption)
		_ = session.MakeRequest(t, req, http.StatusOK)

		// Test using org repo "user3/repo3" where user2 is a collaborator
		origRepoEditOption = getRepoEditOptionFromRepo(repo3)
		repoEditOption = getNewRepoEditOption(origRepoEditOption)
		url = fmt.Sprintf("/api/v1/repos/%s/%s?token=%s", user3.Name, repo3.Name, token2)
		req = NewRequestWithJSON(t, "PATCH", url, &repoEditOption)
		session.MakeRequest(t, req, http.StatusOK)
		// reset repo in db
		url = fmt.Sprintf("/api/v1/repos/%s/%s?token=%s", user3.Name, *repoEditOption.Name, token2)
		req = NewRequestWithJSON(t, "PATCH", url, &origRepoEditOption)
		_ = session.MakeRequest(t, req, http.StatusOK)

		// Test using org repo "user3/repo3" with no user token
		origRepoEditOption = getRepoEditOptionFromRepo(repo3)
		repoEditOption = getNewRepoEditOption(origRepoEditOption)
		url = fmt.Sprintf("/api/v1/repos/%s/%s", user3.Name, repo3.Name)
		req = NewRequestWithJSON(t, "PATCH", url, &repoEditOption)
		session.MakeRequest(t, req, http.StatusNotFound)

		// Test using repo "user2/repo1" where user4 is a NOT collaborator
		origRepoEditOption = getRepoEditOptionFromRepo(repo1)
		repoEditOption = getNewRepoEditOption(origRepoEditOption)
		url = fmt.Sprintf("/api/v1/repos/%s/%s?token=%s", user2.Name, repo1.Name, token4)
		req = NewRequestWithJSON(t, "PATCH", url, &repoEditOption)
		session.MakeRequest(t, req, http.StatusForbidden)
	})
}
