// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"code.gitea.io/gitea/modules/timeutil"
)

// IssueWatch is connection request for receiving issue notification.
type IssueWatch struct {
	ID          int64              `xorm:"pk autoincr"`
	UserID      int64              `xorm:"UNIQUE(watch) NOT NULL"`
	IssueID     int64              `xorm:"UNIQUE(watch) NOT NULL"`
	IsWatching  bool               `xorm:"NOT NULL"`
	CreatedUnix timeutil.TimeStamp `xorm:"created NOT NULL"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated NOT NULL"`
}

// TableName sets the table name to `issue_watch`
func (i *IssueWatch) TableName() string {
	return tbIssueWatch[1 : len(tbIssueWatch)-1]
}

// IssueWatchList contains IssueWatch
type IssueWatchList []*IssueWatch

// CreateOrUpdateIssueWatch set watching for a user and issue
func CreateOrUpdateIssueWatch(userID, issueID int64, isWatching bool) error {
	iw, exists, err := getIssueWatch(x, userID, issueID)
	if err != nil {
		return err
	}

	if !exists {
		iw = &IssueWatch{
			UserID:     userID,
			IssueID:    issueID,
			IsWatching: isWatching,
		}

		if _, err := x.Insert(iw); err != nil {
			return err
		}
	} else {
		iw.IsWatching = isWatching

		if _, err := x.ID(iw.ID).Cols("is_watching", "updated_unix").Update(iw); err != nil {
			return err
		}
	}
	return nil
}

// GetIssueWatch returns all IssueWatch objects from db by user and issue
// the current Web-UI need iw object for watchers AND explicit non-watchers
func GetIssueWatch(userID, issueID int64) (iw *IssueWatch, exists bool, err error) {
	return getIssueWatch(x, userID, issueID)
}

// Return watcher AND explicit non-watcher if entry in db exist
func getIssueWatch(e Engine, userID, issueID int64) (iw *IssueWatch, exists bool, err error) {
	iw = new(IssueWatch)
	exists, err = e.
		Where("user_id = ?", userID).
		And("issue_id = ?", issueID).
		Get(iw)
	return
}

// CheckIssueWatch check if an user is watching an issue
// it takes participants and repo watch into account
func CheckIssueWatch(user *User, issue *Issue) (bool, error) {
	iw, exist, err := getIssueWatch(x, user.ID, issue.ID)
	if err != nil {
		return false, err
	}
	if exist {
		return iw.IsWatching, nil
	}
	w, err := getWatch(x, user.ID, issue.RepoID)
	if err != nil {
		return false, err
	}
	return isWatchMode(w.Mode) || IsUserParticipantsOfIssue(user, issue), nil
}

// GetIssueWatchersIDs returns IDs of subscribers or explicit unsubscribers to a given issue id
// but avoids joining with `user` for performance reasons
// User permissions must be verified elsewhere if required
func GetIssueWatchersIDs(issueID int64, watching bool) ([]int64, error) {
	return getIssueWatchersIDs(x, issueID, watching)
}

func getIssueWatchersIDs(e Engine, issueID int64, watching bool) ([]int64, error) {
	ids := make([]int64, 0, 64)
	return ids, e.Table(tbIssueWatch).
		Where("issue_id=?", issueID).
		And("is_watching = ?", watching).
		Select("user_id").
		Find(&ids)
}

// GetIssueWatchers returns watchers/unwatchers of a given issue
func GetIssueWatchers(issueID int64, listOptions ListOptions) (IssueWatchList, error) {
	return getIssueWatchers(x, issueID, listOptions)
}

func getIssueWatchers(e Engine, issueID int64, listOptions ListOptions) (IssueWatchList, error) {
	sess := e.
		Where(tbIssueWatch+".issue_id = ?", issueID).
		And(tbIssueWatch+".is_watching = ?", true).
		And(tbUser+".is_active = ?", true).
		And(tbUser+".prohibit_login = ?", false).
		Join("INNER", tbUser, tbUser+".id = "+tbIssueWatch+".user_id")

	if listOptions.Page != 0 {
		sess = listOptions.setSessionPagination(sess)
		watches := make([]*IssueWatch, 0, listOptions.PageSize)
		return watches, sess.Find(&watches)
	}
	watches := make([]*IssueWatch, 0, 8)
	return watches, sess.Find(&watches)
}

func removeIssueWatchersByRepoID(e Engine, userID int64, repoID int64) error {
	_, err := e.
		Join("INNER", tbIssue, tbIssue+".id = "+tbIssueWatch+".issue_id AND "+tbIssue+".repo_id = ?", repoID).
		Where(tbIssueWatch+".user_id = ?", userID).
		Delete(new(IssueWatch))
	return err
}
