// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"

	"code.gitea.io/gitea/modules/util"

	"xorm.io/xorm"
)

// IssueAssignees saves all issue assignees
type IssueAssignees struct {
	ID         int64 `xorm:"pk autoincr"`
	AssigneeID int64 `xorm:"INDEX"`
	IssueID    int64 `xorm:"INDEX"`
}

// TableName sets the table name to `issue_assignees`
func (ia *IssueAssignees) TableName() string {
	return tbIssueAssignees[1 : len(tbIssueAssignees)-1]
}

// LoadAssignees load assignees of this issue.
func (issue *Issue) LoadAssignees() error {
	return issue.loadAssignees(x)
}

// This loads all assignees of an issue
func (issue *Issue) loadAssignees(e Engine) (err error) {
	// Reset maybe preexisting assignees
	issue.Assignees = []*User{}

	err = e.Table(tbUser).
		Join("INNER", tbIssueAssignees, "assignee_id = "+tbUser+".id").
		Where(tbIssueAssignees+".issue_id = ?", issue.ID).
		Find(&issue.Assignees)

	if err != nil {
		return err
	}

	// Check if we have at least one assignee and if yes put it in as `Assignee`
	if len(issue.Assignees) > 0 {
		issue.Assignee = issue.Assignees[0]
	}

	return
}

// GetAssigneeIDsByIssue returns the IDs of users assigned to an issue
// but skips joining with `user` for performance reasons.
// User permissions must be verified elsewhere if required.
func GetAssigneeIDsByIssue(issueID int64) ([]int64, error) {
	userIDs := make([]int64, 0, 5)
	return userIDs, x.Table(tbIssueAssignees).
		Cols("assignee_id").
		Where("issue_id = ?", issueID).
		Distinct("assignee_id").
		Find(&userIDs)
}

// GetAssigneesByIssue returns everyone assigned to that issue
func GetAssigneesByIssue(issue *Issue) (assignees []*User, err error) {
	return getAssigneesByIssue(x, issue)
}

func getAssigneesByIssue(e Engine, issue *Issue) (assignees []*User, err error) {
	err = issue.loadAssignees(e)
	if err != nil {
		return assignees, err
	}

	return issue.Assignees, nil
}

// IsUserAssignedToIssue returns true when the user is assigned to the issue
func IsUserAssignedToIssue(issue *Issue, user *User) (isAssigned bool, err error) {
	return isUserAssignedToIssue(x, issue, user)
}

func isUserAssignedToIssue(e Engine, issue *Issue, user *User) (isAssigned bool, err error) {
	return e.Get(&IssueAssignees{IssueID: issue.ID, AssigneeID: user.ID})
}

// ClearAssigneeByUserID deletes all assignments of an user
func clearAssigneeByUserID(sess *xorm.Session, userID int64) (err error) {
	_, err = sess.Delete(&IssueAssignees{AssigneeID: userID})
	return
}

// ToggleAssignee changes a user between assigned and not assigned for this issue, and make issue comment for it.
func (issue *Issue) ToggleAssignee(doer *User, assigneeID int64) (removed bool, comment *Comment, err error) {
	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return false, nil, err
	}

	removed, comment, err = issue.toggleAssignee(sess, doer, assigneeID, false)
	if err != nil {
		return false, nil, err
	}

	if err := sess.Commit(); err != nil {
		return false, nil, err
	}

	return removed, comment, nil
}

func (issue *Issue) toggleAssignee(sess *xorm.Session, doer *User, assigneeID int64, isCreate bool) (removed bool, comment *Comment, err error) {
	removed, err = toggleUserAssignee(sess, issue, assigneeID)
	if err != nil {
		return false, nil, fmt.Errorf("UpdateIssueUserByAssignee: %v", err)
	}

	// Repo infos
	if err = issue.loadRepo(sess); err != nil {
		return false, nil, fmt.Errorf("loadRepo: %v", err)
	}

	var opts = &CreateCommentOptions{
		Type:            CommentTypeAssignees,
		Doer:            doer,
		Repo:            issue.Repo,
		Issue:           issue,
		RemovedAssignee: removed,
		AssigneeID:      assigneeID,
	}
	// Comment
	comment, err = createComment(sess, opts)
	if err != nil {
		return false, nil, fmt.Errorf("createComment: %v", err)
	}

	// if pull request is in the middle of creation - don't call webhook
	if isCreate {
		return removed, comment, err
	}

	return removed, comment, nil
}

// toggles user assignee state in database
func toggleUserAssignee(e *xorm.Session, issue *Issue, assigneeID int64) (removed bool, err error) {

	// Check if the user exists
	assignee, err := getUserByID(e, assigneeID)
	if err != nil {
		return false, err
	}

	// Check if the submitted user is already assigned, if yes delete him otherwise add him
	var i int
	for i = 0; i < len(issue.Assignees); i++ {
		if issue.Assignees[i].ID == assigneeID {
			break
		}
	}

	assigneeIn := IssueAssignees{AssigneeID: assigneeID, IssueID: issue.ID}

	toBeDeleted := i < len(issue.Assignees)
	if toBeDeleted {
		issue.Assignees = append(issue.Assignees[:i], issue.Assignees[i:]...)
		_, err = e.Delete(assigneeIn)
		if err != nil {
			return toBeDeleted, err
		}
	} else {
		issue.Assignees = append(issue.Assignees, assignee)
		_, err = e.Insert(assigneeIn)
		if err != nil {
			return toBeDeleted, err
		}
	}

	return toBeDeleted, nil
}

// MakeIDsFromAPIAssigneesToAdd returns an array with all assignee IDs
func MakeIDsFromAPIAssigneesToAdd(oneAssignee string, multipleAssignees []string) (assigneeIDs []int64, err error) {

	var requestAssignees []string

	// Keeping the old assigning method for compatibility reasons
	if oneAssignee != "" && !util.IsStringInSlice(oneAssignee, multipleAssignees) {
		requestAssignees = append(requestAssignees, oneAssignee)
	}

	//Prevent empty assignees
	if len(multipleAssignees) > 0 && multipleAssignees[0] != "" {
		requestAssignees = append(requestAssignees, multipleAssignees...)
	}

	// Get the IDs of all assignees
	assigneeIDs, err = GetUserIDsByNames(requestAssignees, false)

	return
}
