// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issue

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
)

// DeleteNotPassedAssignee deletes all assignees who aren't passed via the "assignees" array
func DeleteNotPassedAssignee(issue *models.Issue, doer *models.User, assignees []*models.User) (err error) {
	var found bool

	for _, assignee := range issue.Assignees {

		found = false
		for _, alreadyAssignee := range assignees {
			if assignee.ID == alreadyAssignee.ID {
				found = true
				break
			}
		}

		if !found {
			// This function also does comments and hooks, which is why we call it separately instead of directly removing the assignees here
			if _, _, err := ToggleAssignee(issue, doer, assignee.ID); err != nil {
				return err
			}
		}
	}

	return nil
}

// ToggleAssignee changes a user between assigned and not assigned for this issue, and make issue comment for it.
func ToggleAssignee(issue *models.Issue, doer *models.User, assigneeID int64) (removed bool, comment *models.Comment, err error) {
	removed, comment, err = issue.ToggleAssignee(doer, assigneeID)
	if err != nil {
		return
	}

	assignee, err1 := models.GetUserByID(assigneeID)
	if err1 != nil {
		err = err1
		return
	}

	notification.NotifyIssueChangeAssignee(doer, issue, assignee, removed, comment)

	return
}

// ReviewRequest add or remove a review request from a user for this PR, and make comment for it.
func ReviewRequest(issue *models.Issue, doer *models.User, reviewer *models.User, isAdd bool) (comment *models.Comment, err error) {
	if isAdd {
		comment, err = models.AddReviewRequest(issue, reviewer, doer)
	} else {
		comment, err = models.RemoveReviewRequest(issue, reviewer, doer)
	}

	if err != nil {
		return nil, err
	}

	if comment != nil {
		notification.NotifyPullReviewRequest(doer, issue, reviewer, isAdd, comment)
	}

	return
}

// IsValidReviewRequest Check permission for ReviewRequest
func IsValidReviewRequest(reviewer, doer *models.User, isAdd bool, issue *models.Issue, permDoer *models.Permission) error {
	if reviewer.IsOrganization() {
		return models.ErrNotValidReviewRequest{
			Reason: "Organization can't be added as reviewer",
			UserID: doer.ID,
			RepoID: issue.Repo.ID,
		}
	}
	if doer.IsOrganization() {
		return models.ErrNotValidReviewRequest{
			Reason: "Organization can't be doer to add reviewer",
			UserID: doer.ID,
			RepoID: issue.Repo.ID,
		}
	}

	permReviewer, err := models.GetUserRepoPermission(issue.Repo, reviewer)
	if err != nil {
		return err
	}

	if permDoer == nil {
		permDoer = new(models.Permission)
		*permDoer, err = models.GetUserRepoPermission(issue.Repo, doer)
		if err != nil {
			return err
		}
	}

	lastreview, err := models.GetReviewByIssueIDAndUserID(issue.ID, reviewer.ID)
	if err != nil && !models.IsErrReviewNotExist(err) {
		return err
	}

	var pemResult bool
	if isAdd {
		pemResult = permReviewer.CanAccessAny(models.AccessModeRead, models.UnitTypePullRequests)
		if !pemResult {
			return models.ErrNotValidReviewRequest{
				Reason: "Reviewer can't read",
				UserID: doer.ID,
				RepoID: issue.Repo.ID,
			}
		}

		if doer.ID == issue.PosterID && issue.OriginalAuthorID == 0 && lastreview != nil && lastreview.Type != models.ReviewTypeRequest {
			return nil
		}

		pemResult = permDoer.CanAccessAny(models.AccessModeWrite, models.UnitTypePullRequests)
		if !pemResult {
			pemResult, err = models.IsOfficialReviewer(issue, doer)
			if err != nil {
				return err
			}
			if !pemResult {
				return models.ErrNotValidReviewRequest{
					Reason: "Doer can't choose reviewer",
					UserID: doer.ID,
					RepoID: issue.Repo.ID,
				}
			}
		}

		if doer.ID == reviewer.ID {
			return models.ErrNotValidReviewRequest{
				Reason: "doer can't be reviewer",
				UserID: doer.ID,
				RepoID: issue.Repo.ID,
			}
		}

		if reviewer.ID == issue.PosterID && issue.OriginalAuthorID == 0 {
			return models.ErrNotValidReviewRequest{
				Reason: "poster of pr can't be reviewer",
				UserID: doer.ID,
				RepoID: issue.Repo.ID,
			}
		}
	} else {
		if lastreview != nil && lastreview.Type == models.ReviewTypeRequest && lastreview.ReviewerID == doer.ID {
			return nil
		}

		pemResult = permDoer.IsAdmin()
		if !pemResult {
			return models.ErrNotValidReviewRequest{
				Reason: "Doer is not admin",
				UserID: doer.ID,
				RepoID: issue.Repo.ID,
			}
		}
	}

	return nil
}

// IsValidTeamReviewRequest Check permission for ReviewRequest Team
func IsValidTeamReviewRequest(reviewer *models.Team, doer *models.User, isAdd bool, issue *models.Issue) error {
	if doer.IsOrganization() {
		return models.ErrNotValidReviewRequest{
			Reason: "Organization can't be doer to add reviewer",
			UserID: doer.ID,
			RepoID: issue.Repo.ID,
		}
	}

	permission, err := models.GetUserRepoPermission(issue.Repo, doer)
	if err != nil {
		log.Error("Unable to GetUserRepoPermission for %-v in %-v#%d", doer, issue.Repo, issue.Index)
		return err
	}

	if isAdd {
		if issue.Repo.IsPrivate {
			hasTeam := models.HasTeamRepo(reviewer.OrgID, reviewer.ID, issue.RepoID)

			if !hasTeam {
				return models.ErrNotValidReviewRequest{
					Reason: "Reviewing team can't read repo",
					UserID: doer.ID,
					RepoID: issue.Repo.ID,
				}
			}
		}

		doerCanWrite := permission.CanAccessAny(models.AccessModeWrite, models.UnitTypePullRequests)
		if !doerCanWrite {
			official, err := models.IsOfficialReviewer(issue, doer)
			if err != nil {
				log.Error("Unable to Check if IsOfficialReviewer for %-v in %-v#%d", doer, issue.Repo, issue.Index)
				return err
			}
			if !official {
				return models.ErrNotValidReviewRequest{
					Reason: "Doer can't choose reviewer",
					UserID: doer.ID,
					RepoID: issue.Repo.ID,
				}
			}
		}
	} else if !permission.IsAdmin() {
		return models.ErrNotValidReviewRequest{
			Reason: "Only admin users can remove team requests. Doer is not admin",
			UserID: doer.ID,
			RepoID: issue.Repo.ID,
		}
	}

	return nil
}

// TeamReviewRequest add or remove a review request from a team for this PR, and make comment for it.
func TeamReviewRequest(issue *models.Issue, doer *models.User, reviewer *models.Team, isAdd bool) (comment *models.Comment, err error) {
	if isAdd {
		comment, err = models.AddTeamReviewRequest(issue, reviewer, doer)
	} else {
		comment, err = models.RemoveTeamReviewRequest(issue, reviewer, doer)
	}

	if err != nil {
		return
	}

	if comment == nil || !isAdd {
		return
	}

	// notify all user in this team
	if err = comment.LoadIssue(); err != nil {
		return
	}

	if err = reviewer.GetMembers(&models.SearchMembersOptions{}); err != nil {
		return
	}

	for _, member := range reviewer.Members {
		if member.ID == comment.Issue.PosterID {
			continue
		}
		comment.AssigneeID = member.ID
		notification.NotifyPullReviewRequest(doer, issue, member, isAdd, comment)
	}

	return
}
