// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"github.com/go-xorm/xorm"
)

const (
	// PriorityDefault defines the default priority
	PriorityDefault = 0
)

// UpdateIssuePriority updates the priority
func UpdateIssuePriority(issue *Issue) error {
	if err := issue.loadRepo(x); err != nil {
		return err
	}

	if issue.Priority < PriorityDefault {
		return ErrIssueInvalidPriority{ID: issue.ID, RepoID: issue.Repo.ID, DesiredPriority: issue.Priority}
	}

	_, err := AutoTransaction(func(session *xorm.Session) (interface{}, error) {
		return nil, updateIssueCols(session, &Issue{ID: issue.ID, Priority: issue.Priority}, "priority")
	}, x)

	return err
}

// PinIssue to pin an issue
func PinIssue(issue *Issue, doer *User) error {
	if err := issue.loadRepo(x); err != nil {
		return err
	}

	if has, err := HasAccess(doer.ID, issue.Repo); err != nil {
		return err
	} else if !has {
		return ErrUserDoesNotHaveAccessToRepo{UserID: doer.ID, RepoName: issue.Repo.Name}
	}

	_, err := AutoTransaction(func(session *xorm.Session) (interface{}, error) {
		var p int64
		_, err := session.Table("issue").
			Select("MAX(priority)").Where("repo_id=? and is_pull=0", issue.Repo.ID).Get(&p)
		if err != nil {
			return nil, err
		}

		_, err = session.Table("issue").Where("id = ?", issue.ID).
			Update(map[string]interface{}{"priority": p + 10})
		if err != nil {
			return nil, err
		}

		return nil, nil
	}, x)

	return err
}
