// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repofiles

import (
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/notification"
	"code.gitea.io/gitea/modules/references"
	"code.gitea.io/gitea/modules/repository"
)

const (
	secondsByMinute = float64(time.Minute / time.Second) // seconds in a minute
	secondsByHour   = 60 * secondsByMinute               // seconds in an hour
	secondsByDay    = 8 * secondsByHour                  // seconds in a day
	secondsByWeek   = 5 * secondsByDay                   // seconds in a week
	secondsByMonth  = 4 * secondsByWeek                  // seconds in a month
)

var reDuration = regexp.MustCompile(`(?i)^(?:(\d+([\.,]\d+)?)(?:mo))?(?:(\d+([\.,]\d+)?)(?:w))?(?:(\d+([\.,]\d+)?)(?:d))?(?:(\d+([\.,]\d+)?)(?:h))?(?:(\d+([\.,]\d+)?)(?:m))?$`)

// getIssueFromRef returns the issue referenced by a ref. Returns a nil *Issue
// if the provided ref references a non-existent issue.
func getIssueFromRef(repo *models.Repository, index int64) (*models.Issue, error) {
	issue, err := models.GetIssueByIndex(repo.ID, index)
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return issue, nil
}

// timeLogToAmount parses time log string and returns amount in seconds
func timeLogToAmount(str string) int64 {
	matches := reDuration.FindAllStringSubmatch(str, -1)
	if len(matches) == 0 {
		return 0
	}

	match := matches[0]

	var a int64

	// months
	if len(match[1]) > 0 {
		mo, _ := strconv.ParseFloat(strings.Replace(match[1], ",", ".", 1), 64)
		a += int64(mo * secondsByMonth)
	}

	// weeks
	if len(match[3]) > 0 {
		w, _ := strconv.ParseFloat(strings.Replace(match[3], ",", ".", 1), 64)
		a += int64(w * secondsByWeek)
	}

	// days
	if len(match[5]) > 0 {
		d, _ := strconv.ParseFloat(strings.Replace(match[5], ",", ".", 1), 64)
		a += int64(d * secondsByDay)
	}

	// hours
	if len(match[7]) > 0 {
		h, _ := strconv.ParseFloat(strings.Replace(match[7], ",", ".", 1), 64)
		a += int64(h * secondsByHour)
	}

	// minutes
	if len(match[9]) > 0 {
		d, _ := strconv.ParseFloat(strings.Replace(match[9], ",", ".", 1), 64)
		a += int64(d * secondsByMinute)
	}

	return a
}

func issueAddTime(issue *models.Issue, doer *models.User, time time.Time, timeLog string) error {
	amount := timeLogToAmount(timeLog)
	if amount == 0 {
		return nil
	}

	_, err := models.AddTime(doer, issue, amount, time)
	return err
}

func changeIssueStatus(repo *models.Repository, issue *models.Issue, doer *models.User, closed bool) error {
	stopTimerIfAvailable := func(doer *models.User, issue *models.Issue) error {
		if models.StopwatchExists(doer.ID, issue.ID) {
			if err := models.CreateOrStopIssueStopwatch(doer, issue); err != nil {
				return err
			}
		}

		return nil
	}

	issue.Repo = repo
	comment, err := issue.ChangeStatus(doer, closed)
	if err != nil {
		// Don't return an error when dependencies are open as this would let the push fail
		if models.IsErrDependenciesLeft(err) {
			return stopTimerIfAvailable(doer, issue)
		}
		return err
	}

	notification.NotifyIssueChangeStatus(doer, issue, comment, closed)

	return stopTimerIfAvailable(doer, issue)
}

// UpdateIssuesCommit checks if issues are manipulated by commit message.
func UpdateIssuesCommit(doer *models.User, repo *models.Repository, commits []*repository.PushCommit, branchName string) error {
	// Commits are appended in the reverse order.
	for i := len(commits) - 1; i >= 0; i-- {
		c := commits[i]

		type markKey struct {
			ID     int64
			Action references.XRefAction
		}

		refMarked := make(map[markKey]bool)
		var refRepo *models.Repository
		var refIssue *models.Issue
		var err error
		for _, ref := range references.FindAllIssueReferences(c.Message) {

			// issue is from another repo
			if len(ref.Owner) > 0 && len(ref.Name) > 0 {
				refRepo, err = models.GetRepositoryFromMatch(ref.Owner, ref.Name)
				if err != nil {
					continue
				}
			} else {
				refRepo = repo
			}
			if refIssue, err = getIssueFromRef(refRepo, ref.Index); err != nil {
				return err
			}
			if refIssue == nil {
				continue
			}

			perm, err := models.GetUserRepoPermission(refRepo, doer)
			if err != nil {
				return err
			}

			key := markKey{ID: refIssue.ID, Action: ref.Action}
			if refMarked[key] {
				continue
			}
			refMarked[key] = true

			// FIXME: this kind of condition is all over the code, it should be consolidated in a single place
			canclose := perm.IsAdmin() || perm.IsOwner() || perm.CanWriteIssuesOrPulls(refIssue.IsPull) || refIssue.PosterID == doer.ID
			cancomment := canclose || perm.CanReadIssuesOrPulls(refIssue.IsPull)

			// Don't proceed if the user can't comment
			if !cancomment {
				continue
			}

			message := fmt.Sprintf(`<a href="%s/commit/%s">%s</a>`, repo.Link(), c.Sha1, html.EscapeString(strings.SplitN(c.Message, "\n", 2)[0]))
			if err = models.CreateRefComment(doer, refRepo, refIssue, message, c.Sha1); err != nil {
				return err
			}

			// Only issues can be closed/reopened this way, and user needs the correct permissions
			if refIssue.IsPull || !canclose {
				continue
			}

			// Only process closing/reopening keywords
			if ref.Action != references.XRefActionCloses && ref.Action != references.XRefActionReopens {
				continue
			}

			if !repo.CloseIssuesViaCommitInAnyBranch {
				// If the issue was specified to be in a particular branch, don't allow commits in other branches to close it
				if refIssue.Ref != "" {
					if branchName != refIssue.Ref {
						continue
					}
					// Otherwise, only process commits to the default branch
				} else if branchName != repo.DefaultBranch {
					continue
				}
			}
			close := (ref.Action == references.XRefActionCloses)
			if close && len(ref.TimeLog) > 0 {
				if err := issueAddTime(refIssue, doer, c.Timestamp, ref.TimeLog); err != nil {
					return err
				}
			}
			if close != refIssue.IsClosed {
				if err := changeIssueStatus(refRepo, refIssue, doer, close); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
