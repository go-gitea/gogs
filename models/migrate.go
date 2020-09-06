// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"code.gitea.io/gitea/modules/structs"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// InsertMilestones creates milestones of repository.
func InsertMilestones(ms ...*Milestone) (err error) {
	if len(ms) == 0 {
		return nil
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	// to return the id, so we should not use batch insert
	for _, m := range ms {
		if _, err = sess.NoAutoTime().Insert(m); err != nil {
			return err
		}
	}

	if _, err = sess.Exec("UPDATE "+tbRepository+" SET num_milestones = num_milestones + ? WHERE id = ?",
		len(ms),
		ms[0].RepoID); err != nil {
		return err
	}
	return sess.Commit()
}

// InsertIssues insert issues to database
func InsertIssues(issues ...*Issue) error {
	sess := x.NewSession()
	if err := sess.Begin(); err != nil {
		return err
	}

	for _, issue := range issues {
		if err := insertIssue(sess, issue); err != nil {
			return err
		}
	}
	return sess.Commit()
}

func insertIssue(sess *xorm.Session, issue *Issue) error {
	if _, err := sess.NoAutoTime().Insert(issue); err != nil {
		return err
	}
	var issueLabels = make([]IssueLabel, 0, len(issue.Labels))
	var labelIDs = make([]int64, 0, len(issue.Labels))
	for _, label := range issue.Labels {
		issueLabels = append(issueLabels, IssueLabel{
			IssueID: issue.ID,
			LabelID: label.ID,
		})
		labelIDs = append(labelIDs, label.ID)
	}
	if len(issueLabels) > 0 {
		if _, err := sess.Insert(issueLabels); err != nil {
			return err
		}
	}

	for _, reaction := range issue.Reactions {
		reaction.IssueID = issue.ID
	}

	if len(issue.Reactions) > 0 {
		if _, err := sess.Insert(issue.Reactions); err != nil {
			return err
		}
	}

	cols := make([]string, 0)
	if !issue.IsPull {
		sess.ID(issue.RepoID).Incr("num_issues")
		cols = append(cols, "num_issues")
		if issue.IsClosed {
			sess.Incr("num_closed_issues")
			cols = append(cols, "num_closed_issues")
		}
	} else {
		sess.ID(issue.RepoID).Incr("num_pulls")
		cols = append(cols, "num_pulls")
		if issue.IsClosed {
			sess.Incr("num_closed_pulls")
			cols = append(cols, "num_closed_pulls")
		}
	}
	if _, err := sess.NoAutoTime().Cols(cols...).Update(issue.Repo); err != nil {
		return err
	}

	cols = []string{"num_issues"}
	sess.Incr("num_issues")
	if issue.IsClosed {
		sess.Incr("num_closed_issues")
		cols = append(cols, "num_closed_issues")
	}
	if _, err := sess.In("id", labelIDs).NoAutoTime().Cols(cols...).Update(new(Label)); err != nil {
		return err
	}

	if issue.MilestoneID > 0 {
		cols = []string{"num_issues"}
		sess.Incr("num_issues")
		cl := "num_closed_issues"
		if issue.IsClosed {
			sess.Incr("num_closed_issues")
			cols = append(cols, "num_closed_issues")
			cl = "(num_closed_issues + 1)"
		}

		if _, err := sess.ID(issue.MilestoneID).
			SetExpr("completeness", cl+" * 100 / (num_issues + 1)").
			NoAutoTime().Cols(cols...).
			Update(new(Milestone)); err != nil {
			return err
		}
	}

	return nil
}

// InsertIssueComments inserts many comments of issues.
func InsertIssueComments(comments []*Comment) error {
	if len(comments) == 0 {
		return nil
	}

	var issueIDs = make(map[int64]bool)
	for _, comment := range comments {
		issueIDs[comment.IssueID] = true
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	for _, comment := range comments {
		if _, err := sess.NoAutoTime().Insert(comment); err != nil {
			return err
		}

		for _, reaction := range comment.Reactions {
			reaction.IssueID = comment.IssueID
			reaction.CommentID = comment.ID
		}
		if len(comment.Reactions) > 0 {
			if _, err := sess.Insert(comment.Reactions); err != nil {
				return err
			}
		}
	}

	for issueID := range issueIDs {
		if _, err := sess.Exec("UPDATE "+tbIssue+" set num_comments = (SELECT count(*) FROM "+tbComment+" WHERE issue_id = ?) WHERE id = ?", issueID, issueID); err != nil {
			return err
		}
	}
	return sess.Commit()
}

// InsertPullRequests inserted pull requests
func InsertPullRequests(prs ...*PullRequest) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	for _, pr := range prs {
		if err := insertIssue(sess, pr.Issue); err != nil {
			return err
		}
		pr.IssueID = pr.Issue.ID
		if _, err := sess.NoAutoTime().Insert(pr); err != nil {
			return err
		}
	}

	return sess.Commit()
}

// InsertReleases migrates release
func InsertReleases(rels ...*Release) error {
	sess := x.NewSession()
	if err := sess.Begin(); err != nil {
		return err
	}

	for _, rel := range rels {
		if _, err := sess.NoAutoTime().Insert(rel); err != nil {
			return err
		}

		if len(rel.Attachments) > 0 {
			for i := range rel.Attachments {
				rel.Attachments[i].ReleaseID = rel.ID
			}

			if _, err := sess.NoAutoTime().Insert(rel.Attachments); err != nil {
				return err
			}
		}
	}

	return sess.Commit()
}

func migratedIssueCond(tp structs.GitServiceType) builder.Cond {
	return builder.In("issue_id",
		builder.Select(tbIssue+".id").
			From(tbIssue).
			InnerJoin(tbRepository, tbIssue+".repo_id = "+tbRepository+".id").
			Where(builder.Eq{
				tbRepository + ".original_service_type": tp,
			}),
	)
}

// UpdateReviewsMigrationsByType updates reviews' migrations information via given git service type and original id and poster id
func UpdateReviewsMigrationsByType(tp structs.GitServiceType, originalAuthorID string, posterID int64) error {
	_, err := x.Table(tbReview).
		Where("original_author_id = ?", originalAuthorID).
		And(migratedIssueCond(tp)).
		Update(map[string]interface{}{
			"reviewer_id":        posterID,
			"original_author":    "",
			"original_author_id": 0,
		})
	return err
}
