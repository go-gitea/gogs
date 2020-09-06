// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"time"

	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
)

// Stopwatch represents a stopwatch for time tracking.
type Stopwatch struct {
	ID          int64              `xorm:"pk autoincr"`
	IssueID     int64              `xorm:"INDEX"`
	UserID      int64              `xorm:"INDEX"`
	CreatedUnix timeutil.TimeStamp `xorm:"created"`
}

// TableName sets the table name to `stop_watch`
func (sw *Stopwatch) TableName() string {
	return tbStopwatch[1 : len(tbStopwatch)-1]
}

// Stopwatches is a List ful of Stopwatch
type Stopwatches []Stopwatch

func getStopwatch(e Engine, userID, issueID int64) (sw *Stopwatch, exists bool, err error) {
	sw = new(Stopwatch)
	exists, err = e.
		Where("user_id = ?", userID).
		And("issue_id = ?", issueID).
		Get(sw)
	return
}

// GetUserStopwatches return list of all stopwatches of a user
func GetUserStopwatches(userID int64, listOptions ListOptions) (*Stopwatches, error) {
	sws := new(Stopwatches)
	sess := x.Where(tbStopwatch+".user_id = ?", userID)
	if listOptions.Page != 0 {
		sess = listOptions.setSessionPagination(sess)
	}

	err := sess.Find(sws)
	if err != nil {
		return nil, err
	}
	return sws, nil
}

// StopwatchExists returns true if the stopwatch exists
func StopwatchExists(userID int64, issueID int64) bool {
	_, exists, _ := getStopwatch(x, userID, issueID)
	return exists
}

// HasUserStopwatch returns true if the user has a stopwatch
func HasUserStopwatch(userID int64) (exists bool, sw *Stopwatch, err error) {
	sw = new(Stopwatch)
	exists, err = x.
		Where("user_id = ?", userID).
		Get(sw)
	return
}

// CreateOrStopIssueStopwatch will create or remove a stopwatch and will log it into issue's timeline.
func CreateOrStopIssueStopwatch(user *User, issue *Issue) error {
	sw, exists, err := getStopwatch(x, user.ID, issue.ID)
	if err != nil {
		return err
	}
	if err := issue.loadRepo(x); err != nil {
		return err
	}

	if exists {
		// Create tracked time out of the time difference between start date and actual date
		timediff := time.Now().Unix() - int64(sw.CreatedUnix)

		// Create TrackedTime
		tt := &TrackedTime{
			Created: time.Now(),
			IssueID: issue.ID,
			UserID:  user.ID,
			Time:    timediff,
		}

		if _, err := x.Insert(tt); err != nil {
			return err
		}

		if _, err := CreateComment(&CreateCommentOptions{
			Doer:    user,
			Issue:   issue,
			Repo:    issue.Repo,
			Content: SecToTime(timediff),
			Type:    CommentTypeStopTracking,
		}); err != nil {
			return err
		}
		if _, err := x.Delete(sw); err != nil {
			return err
		}
	} else {
		//if another stopwatch is running: stop it
		exists, sw, err := HasUserStopwatch(user.ID)
		if err != nil {
			return err
		}
		if exists {
			issue, err := getIssueByID(x, sw.IssueID)
			if err != nil {
				return err
			}
			if err := CreateOrStopIssueStopwatch(user, issue); err != nil {
				return err
			}
		}

		// Create stopwatch
		sw = &Stopwatch{
			UserID:  user.ID,
			IssueID: issue.ID,
		}

		if _, err := x.Insert(sw); err != nil {
			return err
		}

		if _, err := CreateComment(&CreateCommentOptions{
			Doer:  user,
			Issue: issue,
			Repo:  issue.Repo,
			Type:  CommentTypeStartTracking,
		}); err != nil {
			return err
		}
	}
	return nil
}

// CancelStopwatch removes the given stopwatch and logs it into issue's timeline.
func CancelStopwatch(user *User, issue *Issue) error {
	sw, exists, err := getStopwatch(x, user.ID, issue.ID)
	if err != nil {
		return err
	}

	if exists {
		if _, err := x.Delete(sw); err != nil {
			return err
		}

		if err := issue.loadRepo(x); err != nil {
			return err
		}

		if _, err := CreateComment(&CreateCommentOptions{
			Doer:  user,
			Issue: issue,
			Repo:  issue.Repo,
			Type:  CommentTypeCancelTracking,
		}); err != nil {
			return err
		}
	}
	return nil
}

// SecToTime converts an amount of seconds to a human-readable string (example: 66s -> 1min 6s)
func SecToTime(duration int64) string {
	seconds := duration % 60
	minutes := (duration / (60)) % 60
	hours := duration / (60 * 60)

	var hrs string

	if hours > 0 {
		hrs = fmt.Sprintf("%dh", hours)
	}
	if minutes > 0 {
		if hours == 0 {
			hrs = fmt.Sprintf("%dmin", minutes)
		} else {
			hrs = fmt.Sprintf("%s %dmin", hrs, minutes)
		}
	}
	if seconds > 0 {
		if hours == 0 && minutes == 0 {
			hrs = fmt.Sprintf("%ds", seconds)
		} else {
			hrs = fmt.Sprintf("%s %ds", hrs, seconds)
		}
	}

	return hrs
}

// APIFormat convert Stopwatch type to api.StopWatch type
func (sw *Stopwatch) APIFormat() (api.StopWatch, error) {
	issue, err := getIssueByID(x, sw.IssueID)
	if err != nil {
		return api.StopWatch{}, err
	}
	if err := issue.LoadRepo(); err != nil {
		return api.StopWatch{}, err
	}
	return api.StopWatch{
		Created:       sw.CreatedUnix.AsTime(),
		IssueIndex:    issue.Index,
		IssueTitle:    issue.Title,
		RepoOwnerName: issue.Repo.OwnerName,
		RepoName:      issue.Repo.Name,
	}, nil
}

// APIFormat convert Stopwatches type to api.StopWatches type
func (sws Stopwatches) APIFormat() (api.StopWatches, error) {
	result := api.StopWatches(make([]api.StopWatch, 0, len(sws)))

	issueCache := make(map[int64]*Issue)
	repoCache := make(map[int64]*Repository)
	var (
		issue *Issue
		repo  *Repository
		ok    bool
		err   error
	)

	for _, sw := range sws {
		issue, ok = issueCache[sw.IssueID]
		if !ok {
			issue, err = GetIssueByID(sw.IssueID)
			if err != nil {
				return nil, err
			}
		}
		repo, ok = repoCache[issue.RepoID]
		if !ok {
			repo, err = GetRepositoryByID(issue.RepoID)
			if err != nil {
				return nil, err
			}
		}

		result = append(result, api.StopWatch{
			Created:       sw.CreatedUnix.AsTime(),
			IssueIndex:    issue.Index,
			IssueTitle:    issue.Title,
			RepoOwnerName: repo.OwnerName,
			RepoName:      repo.Name,
		})
	}
	return result, nil
}
