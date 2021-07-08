// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func createReviewsForCodeComments(x *xorm.Engine) error {
	// Review
	type Review struct {
		ID               int64 `xorm:"pk autoincr"`
		Type             int
		ReviewerID       int64 `xorm:"index"`
		OriginalAuthor   string
		OriginalAuthorID int64
		IssueID          int64  `xorm:"index"`
		Content          string `xorm:"TEXT"`
		// Official is a review made by an assigned approver (counts towards approval)
		Official bool   `xorm:"NOT NULL DEFAULT false"`
		CommitID string `xorm:"VARCHAR(40)"`
		Stale    bool   `xorm:"NOT NULL DEFAULT false"`

		CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
	}

	const ReviewTypeComment = 2

	// Comment represents a comment in commit and issue page.
	type Comment struct {
		ID               int64 `xorm:"pk autoincr"`
		Type             int   `xorm:"INDEX"`
		PosterID         int64 `xorm:"INDEX"`
		OriginalAuthor   string
		OriginalAuthorID int64
		IssueID          int64 `xorm:"INDEX"`
		LabelID          int64
		OldProjectID     int64
		ProjectID        int64
		OldMilestoneID   int64
		MilestoneID      int64
		AssigneeID       int64
		RemovedAssignee  bool
		ResolveDoerID    int64
		OldTitle         string
		NewTitle         string
		OldRef           string
		NewRef           string
		DependentIssueID int64

		CommitID int64
		Line     int64 // - previous line / + proposed line
		TreePath string
		Content  string `xorm:"TEXT"`

		// Path represents the 4 lines of code cemented by this comment
		PatchQuoted string `xorm:"TEXT patch"`

		CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`

		// Reference issue in commit message
		CommitSHA string `xorm:"VARCHAR(40)"`

		ReviewID    int64 `xorm:"index"`
		Invalidated bool

		// Reference an issue or pull from another comment, issue or PR
		// All information is about the origin of the reference
		RefRepoID    int64 `xorm:"index"` // Repo where the referencing
		RefIssueID   int64 `xorm:"index"`
		RefCommentID int64 `xorm:"index"`    // 0 if origin is Issue title or content (or PR's)
		RefAction    int   `xorm:"SMALLINT"` // What happens if RefIssueID resolves
		RefIsPull    bool
	}

	if err := x.Sync2(new(Review), new(Comment)); err != nil {
		return err
	}

	updateComment := func(comments []*Comment) error {
		sess := x.NewSession()
		defer sess.Close()
		if err := sess.Begin(); err != nil {
			return err
		}

		for _, comment := range comments {
			review := &Review{
				Type:             ReviewTypeComment,
				ReviewerID:       comment.PosterID,
				IssueID:          comment.IssueID,
				Official:         false,
				CommitID:         comment.CommitSHA,
				Stale:            comment.Invalidated,
				OriginalAuthor:   comment.OriginalAuthor,
				OriginalAuthorID: comment.OriginalAuthorID,
				CreatedUnix:      comment.CreatedUnix,
				UpdatedUnix:      comment.CreatedUnix,
			}
			if _, err := sess.NoAutoTime().Insert(review); err != nil {
				return err
			}

			reviewComment := &Comment{
				Type:             22,
				PosterID:         comment.PosterID,
				Content:          "",
				IssueID:          comment.IssueID,
				ReviewID:         review.ID,
				OriginalAuthor:   comment.OriginalAuthor,
				OriginalAuthorID: comment.OriginalAuthorID,
				CreatedUnix:      comment.CreatedUnix,
				UpdatedUnix:      comment.CreatedUnix,
			}
			if _, err := sess.NoAutoTime().Insert(reviewComment); err != nil {
				return err
			}

			comment.ReviewID = review.ID
			if _, err := sess.ID(comment.ID).Cols("review_id").NoAutoTime().Update(comment); err != nil {
				return err
			}
		}

		return sess.Commit()
	}

	start := 0
	batchSize := 100
	for {
		comments := make([]*Comment, 0, batchSize)
		if err := x.Where("review_id = 0 and type = 21").Limit(batchSize, start).Find(&comments); err != nil {
			return err
		}

		if err := updateComment(comments); err != nil {
			return err
		}

		start += len(comments)

		if len(comments) < batchSize {
			break
		}
	}

	return nil
}
