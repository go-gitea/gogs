// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issue

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/notification"
	"code.gitea.io/gitea/modules/util"
)

// NewIssue creates new issue with labels for repository.
func NewIssue(repo *models.Repository, issue *models.Issue, labelIDs []int64, uuids []string, assigneeIDs []int64) error {
	if err := models.NewIssue(repo, issue, labelIDs, uuids); err != nil {
		return err
	}

	for _, assigneeID := range assigneeIDs {
		if err := AddAssigneeIfNotAssigned(issue, issue.Poster, assigneeID); err != nil {
			return err
		}
	}

	mentions, err := issue.FindAndUpdateIssueMentions(models.DefaultDBContext(), issue.Poster, issue.Content)
	if err != nil {
		return err
	}

	notification.NotifyNewIssue(issue, mentions)
	if len(issue.Labels) > 0 {
		notification.NotifyIssueChangeLabels(issue.Poster, issue, issue.Labels, nil)
	}
	if issue.Milestone != nil {
		notification.NotifyIssueChangeMilestone(issue.Poster, issue, 0)
	}

	return nil
}

// ChangeTitle changes the title of this issue, as the given user.
func ChangeTitle(issue *models.Issue, doer *models.User, title string) (err error) {
	oldTitle := issue.Title
	issue.Title = title

	if err = issue.ChangeTitle(doer, oldTitle); err != nil {
		return
	}

	notification.NotifyIssueChangeTitle(doer, issue, oldTitle)

	return nil
}

// ChangeIssueRef changes the branch of this issue, as the given user.
func ChangeIssueRef(issue *models.Issue, doer *models.User, ref string) error {
	oldRef := issue.Ref
	issue.Ref = ref

	if err := issue.ChangeRef(doer, oldRef); err != nil {
		return err
	}

	notification.NotifyIssueChangeRef(doer, issue, oldRef)

	return nil
}

// UpdateAssignees is a helper function to add or delete one or multiple issue assignee(s)
// Deleting is done the GitHub way (quote from their api documentation):
// https://developer.github.com/v3/issues/#edit-an-issue
// "assignees" (array): Logins for Users to assign to this issue.
// Pass one or more user logins to replace the set of assignees on this Issue.
// Send an empty array ([]) to clear all assignees from the Issue.
func UpdateAssignees(issue *models.Issue, oneAssignee string, multipleAssignees []string, doer *models.User) (err error) {
	var allNewAssignees []*models.User

	// Keep the old assignee thingy for compatibility reasons
	if oneAssignee != "" {
		// Prevent double adding assignees
		var isDouble bool
		for _, assignee := range multipleAssignees {
			if assignee == oneAssignee {
				isDouble = true
				break
			}
		}

		if !isDouble {
			multipleAssignees = append(multipleAssignees, oneAssignee)
		}
	}

	// Loop through all assignees to add them
	for _, assigneeName := range multipleAssignees {
		assignee, err := models.GetUserByName(assigneeName)
		if err != nil {
			return err
		}

		allNewAssignees = append(allNewAssignees, assignee)
	}

	// Delete all old assignees not passed
	if err = DeleteNotPassedAssignee(issue, doer, allNewAssignees); err != nil {
		return err
	}

	// Add all new assignees
	// Update the assignee. The function will check if the user exists, is already
	// assigned (which he shouldn't as we deleted all assignees before) and
	// has access to the repo.
	for _, assignee := range allNewAssignees {
		// Extra method to prevent double adding (which would result in removing)
		err = AddAssigneeIfNotAssigned(issue, doer, assignee.ID)
		if err != nil {
			return err
		}
	}

	return
}

// AddAssigneeIfNotAssigned adds an assignee only if he isn't already assigned to the issue.
// Also checks for access of assigned user
func AddAssigneeIfNotAssigned(issue *models.Issue, doer *models.User, assigneeID int64) (err error) {
	assignee, err := models.GetUserByID(assigneeID)
	if err != nil {
		return err
	}

	// Check if the user is already assigned
	isAssigned, err := models.IsUserAssignedToIssue(issue, assignee)
	if err != nil {
		return err
	}
	if isAssigned {
		// nothing to to
		return nil
	}

	valid, err := models.CanBeAssigned(assignee, issue.Repo, issue.IsPull)
	if err != nil {
		return err
	}
	if !valid {
		return models.ErrUserDoesNotHaveAccessToRepo{UserID: assigneeID, RepoName: issue.Repo.Name}
	}

	_, _, err = ToggleAssignee(issue, doer, assigneeID)
	if err != nil {
		return err
	}

	return nil
}

// GetRefEndNamesAndURLs retrieves the ref end names (e.g. refs/heads/branch-name -> branch-name)
// and their respective URLs.
func GetRefEndNamesAndURLs(issues []*models.Issue, repoLink string) (map[int64]string, map[int64]string) {
	issueRefEndNames := make(map[int64]string, len(issues))
	issueRefURLs := make(map[int64]string, len(issues))
	for _, issue := range issues {
		if issue.Ref != "" {
			issueRefEndNames[issue.ID] = git.RefEndName(issue.Ref)
			issueRefURLs[issue.ID] = git.RefURL(repoLink, util.PathEscapeSegments(issue.Ref))
		}
	}
	return issueRefEndNames, issueRefURLs
}
