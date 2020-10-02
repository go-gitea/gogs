// Copyright 2019 The Gitea Authors.
// All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pull

import (
	"context"
	"fmt"
	"io"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/notification"
	"code.gitea.io/gitea/modules/setting"
)

// CreateCodeComment creates a comment on the code line
func CreateCodeComment(doer *models.User, gitRepo *git.Repository, issue *models.Issue, line int64, content string, treePath string, isReview bool, replyReviewID int64, latestCommitID string) (*models.Comment, error) {

	var (
		existsReview bool
		err          error
	)

	// CreateCodeComment() is used for:
	// - Single comments
	// - Comments that are part of a review
	// - Comments that reply to an existing review

	if !isReview && replyReviewID != 0 {
		// It's not part of a review; maybe a reply to a review comment or a single comment.
		// Check if there are reviews for that line already; if there are, this is a reply
		if existsReview, err = models.ReviewExists(issue, treePath, line); err != nil {
			return nil, err
		}
	}

	// Comments that are replies don't require a review header to show up in the issue view
	if !isReview && existsReview {
		if err = issue.LoadRepo(); err != nil {
			return nil, err
		}

		comment, err := createCodeComment(
			doer,
			issue.Repo,
			issue,
			content,
			treePath,
			line,
			replyReviewID,
		)
		if err != nil {
			return nil, err
		}

		notification.NotifyCreateIssueComment(doer, issue.Repo, issue, comment)

		return comment, nil
	}

	review, err := models.GetCurrentReview(doer, issue)
	if err != nil {
		if !models.IsErrReviewNotExist(err) {
			return nil, err
		}

		review, err = models.CreateReview(models.CreateReviewOptions{
			Type:     models.ReviewTypePending,
			Reviewer: doer,
			Issue:    issue,
			Official: false,
			CommitID: latestCommitID,
		})
		if err != nil {
			return nil, err
		}
	}

	comment, err := createCodeComment(
		doer,
		issue.Repo,
		issue,
		content,
		treePath,
		line,
		review.ID,
	)
	if err != nil {
		return nil, err
	}

	if !isReview && !existsReview {
		// Submit the review we've just created so the comment shows up in the issue view
		if _, _, err = SubmitReview(doer, gitRepo, issue, models.ReviewTypeComment, "", latestCommitID); err != nil {
			return nil, err
		}
	}

	// NOTICE: if it's a pending review the notifications will not be fired until user submit review.

	return comment, nil
}

// createCodeComment creates a plain code comment at the specified line / path
func createCodeComment(doer *models.User, repo *models.Repository, issue *models.Issue, content, treePath string, line, reviewID int64) (*models.Comment, error) {
	var commitID, patch string
	if err := issue.LoadPullRequest(); err != nil {
		return nil, fmt.Errorf("GetPullRequestByIssueID: %v", err)
	}
	pr := issue.PullRequest
	if err := pr.LoadBaseRepo(); err != nil {
		return nil, fmt.Errorf("LoadHeadRepo: %v", err)
	}
	gitRepo, err := git.OpenRepository(pr.BaseRepo.RepoPath())
	if err != nil {
		return nil, fmt.Errorf("OpenRepository: %v", err)
	}
	defer gitRepo.Close()

	// FIXME validate treePath
	// Get latest commit referencing the commented line
	// No need for get commit for base branch changes
	if line > 0 {
		commit, err := gitRepo.LineBlame(pr.GetGitRefName(), gitRepo.Path, treePath, uint(line))
		if err == nil {
			commitID = commit.ID.String()
		} else if !strings.Contains(err.Error(), "exit status 128 - fatal: no such path") {
			return nil, fmt.Errorf("LineBlame[%s, %s, %s, %d]: %v", pr.GetGitRefName(), gitRepo.Path, treePath, line, err)
		}
	}

	// Only fetch diff if comment is review comment
	if reviewID != 0 {
		headCommitID, err := gitRepo.GetRefCommitID(pr.GetGitRefName())
		if err != nil {
			return nil, fmt.Errorf("GetRefCommitID[%s]: %v", pr.GetGitRefName(), err)
		}

		ctx, cancel := context.WithCancel(git.DefaultContext)
		preader, pwriter := io.Pipe()
		go func() {
			err = git.GetRepoRawDiffForFile(ctx, gitRepo, pr.MergeBase, headCommitID, git.RawDiffNormal, treePath, pwriter)
			_ = pwriter.CloseWithError(err)
		}()

		patch, err = git.CutDiffAroundLine(preader, int64((&models.Comment{Line: line}).UnsignedLine()), line < 0, setting.UI.CodeCommentLines)
		_ = preader.Close()
		cancel()
		if err != nil {
			return nil, fmt.Errorf("GetRawDiffForLine[%s, %s, %s, %s]: %v", err, gitRepo.Path, pr.MergeBase, headCommitID, treePath)
		}
	}
	return models.CreateComment(&models.CreateCommentOptions{
		Type:      models.CommentTypeCode,
		Doer:      doer,
		Repo:      repo,
		Issue:     issue,
		Content:   content,
		LineNum:   line,
		TreePath:  treePath,
		CommitSHA: commitID,
		ReviewID:  reviewID,
		Patch:     patch,
	})
}

// SubmitReview creates a review out of the existing pending review or creates a new one if no pending review exist
func SubmitReview(doer *models.User, gitRepo *git.Repository, issue *models.Issue, reviewType models.ReviewType, content, commitID string) (*models.Review, *models.Comment, error) {
	pr, err := issue.GetPullRequest()
	if err != nil {
		return nil, nil, err
	}

	var stale bool
	if reviewType != models.ReviewTypeApprove && reviewType != models.ReviewTypeReject {
		stale = false
	} else {
		headCommitID, err := gitRepo.GetRefCommitID(pr.GetGitRefName())
		if err != nil {
			return nil, nil, err
		}

		if headCommitID == commitID {
			stale = false
		} else {
			stale, err = checkIfPRContentChanged(pr, commitID, headCommitID)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	review, comm, err := models.SubmitReview(doer, issue, reviewType, content, commitID, stale)
	if err != nil {
		return nil, nil, err
	}

	notification.NotifyPullRequestReview(pr, review, comm)

	return review, comm, nil
}
