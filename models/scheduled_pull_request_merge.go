// Copyright 2021 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"code.gitea.io/gitea/modules/timeutil"
)

// ScheduledPullRequestMerge represents a pull request scheduled for merging when checks succeed
type ScheduledPullRequestMerge struct {
	ID          int64              `xorm:"pk autoincr"`
	PullID      int64              `xorm:"BIGINT"`
	DoerID      int64              `xorm:"BIGINT"`
	Doer        *User              `xorm:"-"`
	MergeStyle  MergeStyle         `xorm:"varchar(50)"`
	Message     string             `xorm:"TEXT"`
	CreatedUnix timeutil.TimeStamp `xorm:"created"`
}

// ScheduleAutoMerge schedules a pull request to be merged when all checks succeed
func ScheduleAutoMerge(doer *User, pullID int64, style MergeStyle, message string) error {
	sess := x.NewSession()
	if err := sess.Begin(); err != nil {
		return err
	}
	defer sess.Close()

	// Check if we already have a merge scheduled for that pull request
	if exists, _, err := getScheduledPullRequestMergeByPullID(sess, pullID); err != nil {
		return err
	} else if exists {
		return ErrPullRequestAlreadyScheduledToAutoMerge{PullID: pullID}
	}

	if _, err := sess.Insert(&ScheduledPullRequestMerge{
		DoerID:     doer.ID,
		PullID:     pullID,
		MergeStyle: style,
		Message:    message,
	}); err != nil {
		return err
	}

	pr, err := getPullRequestByID(sess, pullID)
	if err != nil {
		return err
	}

	if _, err := createAutoMergeComment(sess, CommentTypePRScheduledToAutoMerge, pr, doer); err != nil {
		return err
	}

	return sess.Commit()
}

// GetScheduledPullRequestMergeByPullID gets a scheduled pull request merge by pull request id
func GetScheduledPullRequestMergeByPullID(pullID int64) (bool, *ScheduledPullRequestMerge, error) {
	return getScheduledPullRequestMergeByPullID(x, pullID)
}

func getScheduledPullRequestMergeByPullID(e Engine, pullID int64) (bool, *ScheduledPullRequestMerge, error) {
	scheduledPRM := &ScheduledPullRequestMerge{}
	exists, err := e.Where("pull_id = ?", pullID).Get(scheduledPRM)
	if err != nil || !exists {
		return false, nil, err
	}

	doer, err := getUserByID(e, scheduledPRM.DoerID)
	if err != nil {
		return false, nil, err
	}

	scheduledPRM.Doer = doer
	return true, scheduledPRM, nil
}

// RemoveScheduledPullRequestMerge cancels a previously scheduled pull request
func RemoveScheduledPullRequestMerge(doer *User, pullID int64, comment bool) error {
	sess := x.NewSession()
	if err := sess.Begin(); err != nil {
		return err
	}
	defer sess.Close()

	exist, scheduledPRM, err := getScheduledPullRequestMergeByPullID(sess, pullID)
	if err != nil {
		return err
	} else if !exist {
		return ErrNotExist{ID: pullID}
	}

	if _, err := sess.ID(scheduledPRM.ID).Delete(&ScheduledPullRequestMerge{}); err != nil {
		return err
	}

	// if pull got merged we dont need to add a "auto-merge canceled comment"
	if !comment || doer == nil {
		return sess.Commit()
	}

	pr, err := getPullRequestByID(sess, pullID)
	if err != nil {
		return err
	}

	if _, err := createAutoMergeComment(sess, CommentTypePRUnScheduledToAutoMerge, pr, doer); err != nil {
		return err
	}

	return sess.Commit()
}
