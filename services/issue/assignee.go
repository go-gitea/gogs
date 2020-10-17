// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issue

import (
	"fmt"

	"code.gitea.io/gitea/models"
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
			// This function also does comments and hooks, which is why we call it seperatly instead of directly removing the assignees here
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
func ReviewRequest(issue *models.Issue, doer *models.User, reviewer *models.User, isAdd bool) (err error) {
	var comment *models.Comment
	if isAdd {
		comment, err = models.AddReviewRequest(issue, reviewer, doer)
	} else {
		comment, err = models.RemoveReviewRequest(issue, reviewer, doer)
	}

	if err != nil {
		return
	}

	if comment != nil {
		notification.NotifyPullReviewRequest(doer, issue, reviewer, isAdd, comment)
	}

	return
}

// IsLegalReviewRequest check review request permission
func IsLegalReviewRequest(reviewer, doer *models.User, isAdd bool, issue *models.Issue, permDoer models.Permission) error {
	if !issue.IsPull {
		return models.ErrIllLegalReviewRequest{
			Msg: fmt.Sprintf("Issue is not a Pull Request [issue_id: %d]", issue.ID),
		}
	}

	if err := issue.LoadRepo(); err != nil {
		return err
	}

	if reviewer.IsOrganization() {
		return models.ErrIllLegalReviewRequest{
			Msg: fmt.Sprintf("Organization can't be added as reviewer [user_id: %d, repo_id: %d]", reviewer.ID, issue.Repo.ID),
		}
	}
	if doer.IsOrganization() {
		return models.ErrIllLegalReviewRequest{
			Msg: fmt.Sprintf("Organization can't be doer to add reviewer [user_id: %d, repo_id: %d]", doer.ID, issue.Repo.ID),
		}
	}

	permReviewer, err := models.GetUserRepoPermission(issue.Repo, reviewer)
	if err != nil {
		return err
	}

	lastreview, err := models.GetReviewerByIssueIDAndUserID(issue.ID, reviewer.ID)
	if err != nil {
		return err
	}

	var pemResult bool
	if isAdd {
		pemResult = permReviewer.CanAccessAny(models.AccessModeRead, models.UnitTypePullRequests)
		if !pemResult {
			return models.ErrIllLegalReviewRequest{
				Msg: fmt.Sprintf("Reviewer can't read [user_id: %d, repo_name: %s]", reviewer.ID, issue.Repo.Name),
			}
		}

		pemResult = permDoer.CanAccessAny(models.AccessModeWrite, models.UnitTypePullRequests)
		if !pemResult {
			if pemResult, err = models.IsOfficialReviewer(issue, doer); err != nil {
				return err
			}
			if !pemResult {
				return models.ErrIllLegalReviewRequest{
					Msg: fmt.Sprintf("Doer can't choose reviewer [user_id: %d, repo_name: %s, issue_id: %d]", doer.ID, issue.Repo.Name, issue.ID),
				}
			}
		}

		if doer.ID == reviewer.ID {
			return models.ErrIllLegalReviewRequest{
				Msg: fmt.Sprintf("Doer can't be reviewer [user_id: %d, repo_name: %s]", doer.ID, issue.Repo.Name),
			}
		}

		if reviewer.ID == issue.PosterID {
			return models.ErrIllLegalReviewRequest{
				Msg: fmt.Sprintf("Poster of pr can't be reviewer [user_id: %d, repo_name: %s]", reviewer.ID, issue.Repo.Name),
			}
		}
	} else {
		if lastreview.Type == models.ReviewTypeRequest && lastreview.ReviewerID == doer.ID {
			return nil
		}

		if !permDoer.IsAdmin() {
			return models.ErrIllLegalReviewRequest{
				Msg: fmt.Sprintf("Doer is not admin [user_id: %d, repo_name: %s]", doer.ID, issue.Repo.Name),
			}
		}
	}

	return nil
}

// TeamReviewRequest add or remove a review request from a team for this PR, and make comment for it.
func TeamReviewRequest(issue *models.Issue, doer *models.User, reviewer *models.Team, isAdd bool) (err error) {
	var comment *models.Comment
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

	return nil
}
