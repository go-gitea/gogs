// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"
	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/assert"
)

func TestAPIPullReview(t *testing.T) {
	defer prepareTestEnv(t)()
	pullIssue := models.AssertExistsAndLoadBean(t, &models.Issue{ID: 3}).(*models.Issue)
	assert.NoError(t, pullIssue.LoadAttributes())
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: pullIssue.RepoID}).(*models.Repository)

	// test ListPullReviews
	session := loginUser(t, "user2")
	token := getTokenForLoggedInUser(t, session)
	req := NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/pulls/%d/reviews?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, token)
	resp := session.MakeRequest(t, req, http.StatusOK)

	var reviews []*api.PullReview
	DecodeJSON(t, resp, &reviews)
	if !assert.Len(t, reviews, 6) {
		return
	}
	for _, r := range reviews {
		assert.EqualValues(t, pullIssue.HTMLURL(), r.HTMLPullURL)
	}
	assert.EqualValues(t, 8, reviews[3].ID)
	assert.EqualValues(t, "APPROVED", reviews[3].State)
	assert.EqualValues(t, 0, reviews[3].CodeCommentsCount)
	assert.True(t, reviews[3].Stale)
	assert.False(t, reviews[3].Official)

	assert.EqualValues(t, 10, reviews[5].ID)
	assert.EqualValues(t, "REQUEST_CHANGES", reviews[5].State)
	assert.EqualValues(t, 1, reviews[5].CodeCommentsCount)
	assert.EqualValues(t, -1, reviews[5].Reviewer.ID) // ghost user
	assert.False(t, reviews[5].Stale)
	assert.True(t, reviews[5].Official)

	// test GetPullReview
	req = NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/pulls/%d/reviews/%d?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, reviews[3].ID, token)
	resp = session.MakeRequest(t, req, http.StatusOK)
	var review api.PullReview
	DecodeJSON(t, resp, &review)
	assert.EqualValues(t, *reviews[3], review)

	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/pulls/%d/reviews/%d?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, reviews[5].ID, token)
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &review)
	assert.EqualValues(t, *reviews[5], review)

	// test GetPullReviewComments
	comment := models.AssertExistsAndLoadBean(t, &models.Comment{ID: 7}).(*models.Comment)
	req = NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/pulls/%d/reviews/%d/comments?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, 10, token)
	resp = session.MakeRequest(t, req, http.StatusOK)
	var reviewComments []*api.PullReviewComment
	DecodeJSON(t, resp, &reviewComments)
	assert.Len(t, reviewComments, 1)
	assert.EqualValues(t, "Ghost", reviewComments[0].Poster.UserName)
	assert.EqualValues(t, "a review from a deleted user", reviewComments[0].Body)
	assert.EqualValues(t, comment.ID, reviewComments[0].ID)
	assert.EqualValues(t, comment.UpdatedUnix, reviewComments[0].Updated.Unix())
	assert.EqualValues(t, comment.HTMLURL(), reviewComments[0].HTMLURL)

	// test CreatePullReview
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/reviews?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, token), &api.CreatePullReviewOptions{
		Body: "body1",
		// Event: "" # will result in PENDING
		Comments: []api.CreatePullReviewComment{{
			Path:       "README.md",
			Body:       "first new line",
			OldLineNum: 0,
			NewLineNum: 1,
		}, {
			Path:       "README.md",
			Body:       "first old line",
			OldLineNum: 1,
			NewLineNum: 0,
		}, {
			Path:       "iso-8859-1.txt",
			Body:       "this line contains a non-utf-8 character",
			OldLineNum: 0,
			NewLineNum: 1,
		},
		},
	})
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &review)
	assert.EqualValues(t, 6, review.ID)
	assert.EqualValues(t, "PENDING", review.State)
	assert.EqualValues(t, 3, review.CodeCommentsCount)

	// test SubmitPullReview
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/reviews/%d?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, review.ID, token), &api.SubmitPullReviewOptions{
		Event: "APPROVED",
		Body:  "just two nits",
	})
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &review)
	assert.EqualValues(t, 6, review.ID)
	assert.EqualValues(t, "APPROVED", review.State)
	assert.EqualValues(t, 3, review.CodeCommentsCount)

	// test dismiss review
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/reviews/%d/dismissals?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, review.ID, token), &api.DismissPullReviewOptions{
		Message: "test",
	})
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &review)
	assert.EqualValues(t, 6, review.ID)
	assert.True(t, review.Dismissed)

	// test dismiss review
	req = NewRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/reviews/%d/undismissals?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, review.ID, token))
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &review)
	assert.EqualValues(t, 6, review.ID)
	assert.False(t, review.Dismissed)

	// test DeletePullReview
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/reviews?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, token), &api.CreatePullReviewOptions{
		Body:  "just a comment",
		Event: "COMMENT",
	})
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &review)
	assert.EqualValues(t, "COMMENT", review.State)
	assert.EqualValues(t, 0, review.CodeCommentsCount)
	req = NewRequestf(t, http.MethodDelete, "/api/v1/repos/%s/%s/pulls/%d/reviews/%d?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, review.ID, token)
	resp = session.MakeRequest(t, req, http.StatusNoContent)

	// test CreatePullReview Comment without body but with comments
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/reviews?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, token), &api.CreatePullReviewOptions{
		// Body:  "",
		Event: "COMMENT",
		Comments: []api.CreatePullReviewComment{{
			Path:       "README.md",
			Body:       "first new line",
			OldLineNum: 0,
			NewLineNum: 1,
		}, {
			Path:       "README.md",
			Body:       "first old line",
			OldLineNum: 1,
			NewLineNum: 0,
		},
		},
	})
	var commentReview api.PullReview

	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &commentReview)
	assert.EqualValues(t, "COMMENT", commentReview.State)
	assert.EqualValues(t, 2, commentReview.CodeCommentsCount)
	assert.EqualValues(t, "", commentReview.Body)
	assert.EqualValues(t, false, commentReview.Dismissed)

	// test CreatePullReview Comment with body but without comments
	commentBody := "This is a body of the comment."
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/reviews?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, token), &api.CreatePullReviewOptions{
		Body:     commentBody,
		Event:    "COMMENT",
		Comments: []api.CreatePullReviewComment{},
	})

	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &commentReview)
	assert.EqualValues(t, "COMMENT", commentReview.State)
	assert.EqualValues(t, 0, commentReview.CodeCommentsCount)
	assert.EqualValues(t, commentBody, commentReview.Body)
	assert.EqualValues(t, false, commentReview.Dismissed)

	// test CreatePullReview Comment without body and no comments
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/reviews?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, token), &api.CreatePullReviewOptions{
		Body:     "",
		Event:    "COMMENT",
		Comments: []api.CreatePullReviewComment{},
	})
	resp = session.MakeRequest(t, req, http.StatusUnprocessableEntity)
	errMap := make(map[string]interface{})
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	json.Unmarshal(resp.Body.Bytes(), &errMap)
	assert.EqualValues(t, "review event COMMENT requires a body or a comment", errMap["message"].(string))

	// test get review requests
	// to make it simple, use same api with get review
	pullIssue12 := models.AssertExistsAndLoadBean(t, &models.Issue{ID: 12}).(*models.Issue)
	assert.NoError(t, pullIssue12.LoadAttributes())
	repo3 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: pullIssue12.RepoID}).(*models.Repository)

	req = NewRequestf(t, http.MethodGet, "/api/v1/repos/%s/%s/pulls/%d/reviews?token=%s", repo3.OwnerName, repo3.Name, pullIssue12.Index, token)
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &reviews)
	assert.EqualValues(t, 11, reviews[0].ID)
	assert.EqualValues(t, "REQUEST_REVIEW", reviews[0].State)
	assert.EqualValues(t, 0, reviews[0].CodeCommentsCount)
	assert.False(t, reviews[0].Stale)
	assert.True(t, reviews[0].Official)
	assert.EqualValues(t, "test_team", reviews[0].ReviewerTeam.Name)

	assert.EqualValues(t, 12, reviews[1].ID)
	assert.EqualValues(t, "REQUEST_REVIEW", reviews[1].State)
	assert.EqualValues(t, 0, reviews[0].CodeCommentsCount)
	assert.False(t, reviews[1].Stale)
	assert.True(t, reviews[1].Official)
	assert.EqualValues(t, 1, reviews[1].Reviewer.ID)
}

func TestAPIPullReviewRequest(t *testing.T) {
	defer prepareTestEnv(t)()
	pullIssue := models.AssertExistsAndLoadBean(t, &models.Issue{ID: 3}).(*models.Issue)
	assert.NoError(t, pullIssue.LoadAttributes())
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: pullIssue.RepoID}).(*models.Repository)

	// Test add Review Request
	session := loginUser(t, "user2")
	token := getTokenForLoggedInUser(t, session)
	req := NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, token), &api.PullReviewRequestOptions{
		Reviewers: []string{"user4@example.com", "user8"},
	})
	session.MakeRequest(t, req, http.StatusCreated)

	// poster of pr can't be reviewer
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, token), &api.PullReviewRequestOptions{
		Reviewers: []string{"user1"},
	})
	session.MakeRequest(t, req, http.StatusUnprocessableEntity)

	// test user not exist
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, token), &api.PullReviewRequestOptions{
		Reviewers: []string{"testOther"},
	})
	session.MakeRequest(t, req, http.StatusNotFound)

	// Test Remove Review Request
	session2 := loginUser(t, "user4")
	token2 := getTokenForLoggedInUser(t, session2)

	req = NewRequestWithJSON(t, http.MethodDelete, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, token2), &api.PullReviewRequestOptions{
		Reviewers: []string{"user4"},
	})
	session.MakeRequest(t, req, http.StatusNoContent)

	// doer is not admin
	req = NewRequestWithJSON(t, http.MethodDelete, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, token2), &api.PullReviewRequestOptions{
		Reviewers: []string{"user8"},
	})
	session.MakeRequest(t, req, http.StatusUnprocessableEntity)

	req = NewRequestWithJSON(t, http.MethodDelete, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers?token=%s", repo.OwnerName, repo.Name, pullIssue.Index, token), &api.PullReviewRequestOptions{
		Reviewers: []string{"user8"},
	})
	session.MakeRequest(t, req, http.StatusNoContent)

	// Test team review request
	pullIssue12 := models.AssertExistsAndLoadBean(t, &models.Issue{ID: 12}).(*models.Issue)
	assert.NoError(t, pullIssue12.LoadAttributes())
	repo3 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: pullIssue12.RepoID}).(*models.Repository)

	// Test add Team Review Request
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers?token=%s", repo3.OwnerName, repo3.Name, pullIssue12.Index, token), &api.PullReviewRequestOptions{
		TeamReviewers: []string{"team1", "owners"},
	})
	session.MakeRequest(t, req, http.StatusCreated)

	// Test add Team Review Request to not allowned
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers?token=%s", repo3.OwnerName, repo3.Name, pullIssue12.Index, token), &api.PullReviewRequestOptions{
		TeamReviewers: []string{"test_team"},
	})
	session.MakeRequest(t, req, http.StatusUnprocessableEntity)

	// Test add Team Review Request to not exist
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers?token=%s", repo3.OwnerName, repo3.Name, pullIssue12.Index, token), &api.PullReviewRequestOptions{
		TeamReviewers: []string{"not_exist_team"},
	})
	session.MakeRequest(t, req, http.StatusNotFound)

	// Test Remove team Review Request
	req = NewRequestWithJSON(t, http.MethodDelete, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers?token=%s", repo3.OwnerName, repo3.Name, pullIssue12.Index, token), &api.PullReviewRequestOptions{
		TeamReviewers: []string{"team1"},
	})
	session.MakeRequest(t, req, http.StatusNoContent)

	// empty request test
	req = NewRequestWithJSON(t, http.MethodPost, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers?token=%s", repo3.OwnerName, repo3.Name, pullIssue12.Index, token), &api.PullReviewRequestOptions{})
	session.MakeRequest(t, req, http.StatusCreated)

	req = NewRequestWithJSON(t, http.MethodDelete, fmt.Sprintf("/api/v1/repos/%s/%s/pulls/%d/requested_reviewers?token=%s", repo3.OwnerName, repo3.Name, pullIssue12.Index, token), &api.PullReviewRequestOptions{})
	session.MakeRequest(t, req, http.StatusNoContent)
}
