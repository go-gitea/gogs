// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
)

// ReviewType defines the sort of feedback a review gives
type ReviewType int

// ReviewTypeUnknown unknown review type
const ReviewTypeUnknown ReviewType = -1

const (
	// ReviewTypePending is a review which is not published yet
	ReviewTypePending ReviewType = iota
	// ReviewTypeApprove approves changes
	ReviewTypeApprove
	// ReviewTypeComment gives general feedback
	ReviewTypeComment
	// ReviewTypeReject gives feedback blocking merge
	ReviewTypeReject
	// ReviewTypeRequest request review from others
	ReviewTypeRequest
)

// Icon returns the corresponding icon for the review type
func (rt ReviewType) Icon() string {
	switch rt {
	case ReviewTypeApprove:
		return "check"
	case ReviewTypeReject:
		return "diff"
	case ReviewTypeComment:
		return "comment"
	case ReviewTypeRequest:
		return "dot-fill"
	default:
		return "comment"
	}
}

// Review represents collection of code comments giving feedback for a PR
type Review struct {
	ID               int64 `xorm:"pk autoincr"`
	Type             ReviewType
	Reviewer         *User `xorm:"-"`
	ReviewerID       int64 `xorm:"index"`
	OriginalAuthor   string
	OriginalAuthorID int64
	Issue            *Issue `xorm:"-"`
	IssueID          int64  `xorm:"index"`
	Content          string `xorm:"TEXT"`
	// Official is a review made by an assigned approver (counts towards approval)
	Official bool   `xorm:"NOT NULL DEFAULT false"`
	CommitID string `xorm:"VARCHAR(40)"`
	Stale    bool   `xorm:"NOT NULL DEFAULT false"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`

	// CodeComments are the initial code comments of the review
	CodeComments CodeComments `xorm:"-"`

	Comments []*Comment `xorm:"-"`
}

// TableName sets the table name to `review`
func (r *Review) TableName() string {
	return tbReview[1 : len(tbReview)-1]
}

func (r *Review) loadCodeComments(e Engine) (err error) {
	if r.CodeComments != nil {
		return
	}
	if err = r.loadIssue(e); err != nil {
		return
	}
	r.CodeComments, err = fetchCodeCommentsByReview(e, r.Issue, nil, r)
	return
}

// LoadCodeComments loads CodeComments
func (r *Review) LoadCodeComments() error {
	return r.loadCodeComments(x)
}

func (r *Review) loadIssue(e Engine) (err error) {
	if r.Issue != nil {
		return
	}
	r.Issue, err = getIssueByID(e, r.IssueID)
	return
}

func (r *Review) loadReviewer(e Engine) (err error) {
	if r.Reviewer != nil || r.ReviewerID == 0 {
		return nil
	}
	r.Reviewer, err = getUserByID(e, r.ReviewerID)
	return
}

// LoadReviewer loads reviewer
func (r *Review) LoadReviewer() error {
	return r.loadReviewer(x)
}

func (r *Review) loadAttributes(e Engine) (err error) {
	if err = r.loadIssue(e); err != nil {
		return
	}
	if err = r.loadCodeComments(e); err != nil {
		return
	}
	if err = r.loadReviewer(e); err != nil {
		return
	}
	return
}

// LoadAttributes loads all attributes except CodeComments
func (r *Review) LoadAttributes() error {
	return r.loadAttributes(x)
}

func getReviewByID(e Engine, id int64) (*Review, error) {
	review := new(Review)
	if has, err := e.ID(id).Get(review); err != nil {
		return nil, err
	} else if !has {
		return nil, ErrReviewNotExist{ID: id}
	} else {
		return review, nil
	}
}

// GetReviewByID returns the review by the given ID
func GetReviewByID(id int64) (*Review, error) {
	return getReviewByID(x, id)
}

// FindReviewOptions represent possible filters to find reviews
type FindReviewOptions struct {
	ListOptions
	Type         ReviewType
	IssueID      int64
	ReviewerID   int64
	OfficialOnly bool
}

func (opts *FindReviewOptions) toCond() builder.Cond {
	var cond = builder.NewCond()
	if opts.IssueID > 0 {
		cond = cond.And(builder.Eq{"issue_id": opts.IssueID})
	}
	if opts.ReviewerID > 0 {
		cond = cond.And(builder.Eq{"reviewer_id": opts.ReviewerID})
	}
	if opts.Type != ReviewTypeUnknown {
		cond = cond.And(builder.Eq{"type": opts.Type})
	}
	if opts.OfficialOnly {
		cond = cond.And(builder.Eq{"official": true})
	}
	return cond
}

func findReviews(e Engine, opts FindReviewOptions) ([]*Review, error) {
	reviews := make([]*Review, 0, 10)
	sess := e.Where(opts.toCond())
	if opts.Page > 0 {
		sess = opts.ListOptions.setSessionPagination(sess)
	}
	return reviews, sess.
		Asc("created_unix").
		Asc("id").
		Find(&reviews)
}

// FindReviews returns reviews passing FindReviewOptions
func FindReviews(opts FindReviewOptions) ([]*Review, error) {
	return findReviews(x, opts)
}

// CreateReviewOptions represent the options to create a review. Type, Issue and Reviewer are required.
type CreateReviewOptions struct {
	Content  string
	Type     ReviewType
	Issue    *Issue
	Reviewer *User
	Official bool
	CommitID string
	Stale    bool
}

// IsOfficialReviewer check if reviewer can make official reviews in issue (counts towards required approvals)
func IsOfficialReviewer(issue *Issue, reviewer *User) (bool, error) {
	return isOfficialReviewer(x, issue, reviewer)
}

func isOfficialReviewer(e Engine, issue *Issue, reviewer *User) (bool, error) {
	pr, err := getPullRequestByIssueID(e, issue.ID)
	if err != nil {
		return false, err
	}
	if err = pr.loadProtectedBranch(e); err != nil {
		return false, err
	}
	if pr.ProtectedBranch == nil {
		return false, nil
	}

	return pr.ProtectedBranch.isUserOfficialReviewer(e, reviewer)
}

func createReview(e Engine, opts CreateReviewOptions) (*Review, error) {
	review := &Review{
		Type:       opts.Type,
		Issue:      opts.Issue,
		IssueID:    opts.Issue.ID,
		Reviewer:   opts.Reviewer,
		ReviewerID: opts.Reviewer.ID,
		Content:    opts.Content,
		Official:   opts.Official,
		CommitID:   opts.CommitID,
		Stale:      opts.Stale,
	}
	if _, err := e.Insert(review); err != nil {
		return nil, err
	}

	return review, nil
}

// CreateReview creates a new review based on opts
func CreateReview(opts CreateReviewOptions) (*Review, error) {
	return createReview(x, opts)
}

func getCurrentReview(e Engine, reviewer *User, issue *Issue) (*Review, error) {
	if reviewer == nil {
		return nil, nil
	}
	reviews, err := findReviews(e, FindReviewOptions{
		Type:       ReviewTypePending,
		IssueID:    issue.ID,
		ReviewerID: reviewer.ID,
	})
	if err != nil {
		return nil, err
	}
	if len(reviews) == 0 {
		return nil, ErrReviewNotExist{}
	}
	reviews[0].Reviewer = reviewer
	reviews[0].Issue = issue
	return reviews[0], nil
}

// ReviewExists returns whether a review exists for a particular line of code in the PR
func ReviewExists(issue *Issue, treePath string, line int64) (bool, error) {
	return x.Cols("id").Exist(&Comment{IssueID: issue.ID, TreePath: treePath, Line: line, Type: CommentTypeCode})
}

// GetCurrentReview returns the current pending review of reviewer for given issue
func GetCurrentReview(reviewer *User, issue *Issue) (*Review, error) {
	return getCurrentReview(x, reviewer, issue)
}

// ContentEmptyErr represents an content empty error
type ContentEmptyErr struct {
}

func (ContentEmptyErr) Error() string {
	return "Review content is empty"
}

// IsContentEmptyErr returns true if err is a ContentEmptyErr
func IsContentEmptyErr(err error) bool {
	_, ok := err.(ContentEmptyErr)
	return ok
}

// SubmitReview creates a review out of the existing pending review or creates a new one if no pending review exist
func SubmitReview(doer *User, issue *Issue, reviewType ReviewType, content, commitID string, stale bool) (*Review, *Comment, error) {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return nil, nil, err
	}

	var official = false

	review, err := getCurrentReview(sess, doer, issue)
	if err != nil {
		if !IsErrReviewNotExist(err) {
			return nil, nil, err
		}

		if reviewType != ReviewTypeApprove && len(strings.TrimSpace(content)) == 0 {
			return nil, nil, ContentEmptyErr{}
		}

		if reviewType == ReviewTypeApprove || reviewType == ReviewTypeReject {
			// Only reviewers latest review of type approve and reject shall count as "official", so existing reviews needs to be cleared
			if _, err := sess.Exec("UPDATE "+tbReview+" SET official=? WHERE issue_id=? AND reviewer_id=?",
				false, issue.ID, doer.ID); err != nil {
				return nil, nil, err
			}
			official, err = isOfficialReviewer(sess, issue, doer)
			if err != nil {
				return nil, nil, err
			}
		}

		// No current review. Create a new one!
		review, err = createReview(sess, CreateReviewOptions{
			Type:     reviewType,
			Issue:    issue,
			Reviewer: doer,
			Content:  content,
			Official: official,
			CommitID: commitID,
			Stale:    stale,
		})
		if err != nil {
			return nil, nil, err
		}
	} else {
		if err := review.loadCodeComments(sess); err != nil {
			return nil, nil, err
		}
		if reviewType != ReviewTypeApprove && len(review.CodeComments) == 0 && len(strings.TrimSpace(content)) == 0 {
			return nil, nil, ContentEmptyErr{}
		}

		if reviewType == ReviewTypeApprove || reviewType == ReviewTypeReject {
			// Only reviewers latest review of type approve and reject shall count as "official", so existing reviews needs to be cleared
			if _, err := sess.Exec("UPDATE "+tbReview+" SET official=? WHERE issue_id=? AND reviewer_id=?",
				false, issue.ID, doer.ID); err != nil {
				return nil, nil, err
			}
			official, err = isOfficialReviewer(sess, issue, doer)
			if err != nil {
				return nil, nil, err
			}
		}

		review.Official = official
		review.Issue = issue
		review.Content = content
		review.Type = reviewType
		review.CommitID = commitID
		review.Stale = stale

		if _, err := sess.ID(review.ID).Cols("content, type, official, commit_id, stale").Update(review); err != nil {
			return nil, nil, err
		}
	}

	comm, err := createComment(sess, &CreateCommentOptions{
		Type:     CommentTypeReview,
		Doer:     doer,
		Content:  review.Content,
		Issue:    issue,
		Repo:     issue.Repo,
		ReviewID: review.ID,
	})
	if err != nil || comm == nil {
		return nil, nil, err
	}

	comm.Review = review
	return review, comm, sess.Commit()
}

// GetReviewersByIssueID gets the latest review of each reviewer for a pull request
func GetReviewersByIssueID(issueID int64) (reviews []*Review, err error) {
	reviewsUnfiltered := []*Review{}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return nil, err
	}

	// Get latest review of each reviwer, sorted in order they were made
	if err := sess.SQL("SELECT * FROM "+tbReview+" WHERE id IN (SELECT max(id) as id FROM "+tbReview+" WHERE issue_id = ? AND type in (?, ?, ?) GROUP BY issue_id, reviewer_id) ORDER BY "+tbReview+".updated_unix ASC",
		issueID, ReviewTypeApprove, ReviewTypeReject, ReviewTypeRequest).
		Find(&reviewsUnfiltered); err != nil {
		return nil, err
	}

	// Load reviewer and skip if user is deleted
	for _, review := range reviewsUnfiltered {
		if err = review.loadReviewer(sess); err != nil {
			if !IsErrUserNotExist(err) {
				return nil, err
			}
		} else {
			reviews = append(reviews, review)
		}
	}

	return reviews, nil
}

// GetReviewerByIssueIDAndUserID get the latest review of reviewer for a pull request
func GetReviewerByIssueIDAndUserID(issueID, userID int64) (review *Review, err error) {
	return getReviewerByIssueIDAndUserID(x, issueID, userID)
}

func getReviewerByIssueIDAndUserID(e Engine, issueID, userID int64) (review *Review, err error) {
	review = new(Review)

	if _, err := e.SQL("SELECT * FROM "+tbReview+" WHERE id IN (SELECT max(id) as id FROM "+tbReview+" WHERE issue_id = ? AND reviewer_id = ? AND type in (?, ?, ?))",
		issueID, userID, ReviewTypeApprove, ReviewTypeReject, ReviewTypeRequest).
		Get(review); err != nil {
		return nil, err
	}

	return
}

// MarkReviewsAsStale marks existing reviews as stale
func MarkReviewsAsStale(issueID int64) (err error) {
	_, err = x.Exec("UPDATE "+tbReview+" SET stale=? WHERE issue_id=?", true, issueID)

	return
}

// MarkReviewsAsNotStale marks existing reviews as not stale for a giving commit SHA
func MarkReviewsAsNotStale(issueID int64, commitID string) (err error) {
	_, err = x.Exec("UPDATE "+tbReview+" SET stale=? WHERE issue_id=? AND commit_id=?", false, issueID, commitID)

	return
}

// InsertReviews inserts review and review comments
func InsertReviews(reviews []*Review) error {
	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	for _, review := range reviews {
		if _, err := sess.NoAutoTime().Insert(review); err != nil {
			return err
		}

		if _, err := sess.NoAutoTime().Insert(&Comment{
			Type:             CommentTypeReview,
			Content:          review.Content,
			PosterID:         review.ReviewerID,
			OriginalAuthor:   review.OriginalAuthor,
			OriginalAuthorID: review.OriginalAuthorID,
			IssueID:          review.IssueID,
			ReviewID:         review.ID,
			CreatedUnix:      review.CreatedUnix,
			UpdatedUnix:      review.UpdatedUnix,
		}); err != nil {
			return err
		}

		for _, c := range review.Comments {
			c.ReviewID = review.ID
		}

		if len(review.Comments) > 0 {
			if _, err := sess.NoAutoTime().Insert(review.Comments); err != nil {
				return err
			}
		}
	}

	return sess.Commit()
}

// AddReviewRequest add a review request from one reviewer
func AddReviewRequest(issue *Issue, reviewer *User, doer *User) (comment *Comment, err error) {
	review, err := GetReviewerByIssueIDAndUserID(issue.ID, reviewer.ID)
	if err != nil {
		return
	}

	// skip it when reviewer hase been request to review
	if review != nil && review.Type == ReviewTypeRequest {
		return nil, nil
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return nil, err
	}

	var official bool
	official, err = isOfficialReviewer(sess, issue, reviewer)

	if err != nil {
		return nil, err
	}

	if !official {
		official, err = isOfficialReviewer(sess, issue, doer)

		if err != nil {
			return nil, err
		}
	}

	if official {
		if _, err := sess.Exec("UPDATE "+tbReview+" SET official=? WHERE issue_id=? AND reviewer_id=?", false, issue.ID, reviewer.ID); err != nil {
			return nil, err
		}
	}

	_, err = createReview(sess, CreateReviewOptions{
		Type:     ReviewTypeRequest,
		Issue:    issue,
		Reviewer: reviewer,
		Official: official,
		Stale:    false,
	})

	if err != nil {
		return
	}

	comment, err = createComment(sess, &CreateCommentOptions{
		Type:            CommentTypeReviewRequest,
		Doer:            doer,
		Repo:            issue.Repo,
		Issue:           issue,
		RemovedAssignee: false,       // Use RemovedAssignee as !isRequest
		AssigneeID:      reviewer.ID, // Use AssigneeID as reviewer ID
	})

	if err != nil {
		return nil, err
	}

	return comment, sess.Commit()
}

//RemoveReviewRequest remove a review request from one reviewer
func RemoveReviewRequest(issue *Issue, reviewer *User, doer *User) (comment *Comment, err error) {
	review, err := GetReviewerByIssueIDAndUserID(issue.ID, reviewer.ID)
	if err != nil {
		return
	}

	if review.Type != ReviewTypeRequest {
		return nil, nil
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return nil, err
	}

	_, err = sess.Delete(review)
	if err != nil {
		return nil, err
	}

	var official bool
	official, err = isOfficialReviewer(sess, issue, reviewer)
	if err != nil {
		return
	}

	if official {
		// recalculate which is the latest official review from that user
		var review *Review

		review, err = getReviewerByIssueIDAndUserID(sess, issue.ID, reviewer.ID)
		if err != nil {
			return nil, err
		}

		if review != nil {
			if _, err := sess.Exec("UPDATE "+tbReview+" SET official=? WHERE id=?", true, review.ID); err != nil {
				return nil, err
			}
		}
	}

	if err != nil {
		return nil, err
	}

	comment, err = createComment(sess, &CreateCommentOptions{
		Type:            CommentTypeReviewRequest,
		Doer:            doer,
		Repo:            issue.Repo,
		Issue:           issue,
		RemovedAssignee: true,        // Use RemovedAssignee as !isRequest
		AssigneeID:      reviewer.ID, // Use AssigneeID as reviewer ID
	})

	if err != nil {
		return nil, err
	}

	return comment, sess.Commit()
}

// MarkConversation Add or remove Conversation mark for a code comment
func MarkConversation(comment *Comment, doer *User, isResolve bool) (err error) {
	if comment.Type != CommentTypeCode {
		return nil
	}

	if isResolve {
		if comment.ResolveDoerID != 0 {
			return nil
		}

		if _, err = x.Exec("UPDATE "+tbComment+" SET resolve_doer_id=? WHERE id=?", doer.ID, comment.ID); err != nil {
			return err
		}
	} else {
		if comment.ResolveDoerID == 0 {
			return nil
		}

		if _, err = x.Exec("UPDATE "+tbComment+" SET resolve_doer_id=? WHERE id=?", 0, comment.ID); err != nil {
			return err
		}
	}

	return nil
}

// CanMarkConversation  Add or remove Conversation mark for a code comment permission check
// the PR writer , offfcial reviewer and poster can do it
func CanMarkConversation(issue *Issue, doer *User) (permResult bool, err error) {
	if doer == nil || issue == nil {
		return false, fmt.Errorf("issue or doer is nil")
	}

	if doer.ID != issue.PosterID {
		if err = issue.LoadRepo(); err != nil {
			return false, err
		}

		perm, err := GetUserRepoPermission(issue.Repo, doer)
		if err != nil {
			return false, err
		}

		permResult = perm.CanAccess(AccessModeWrite, UnitTypePullRequests)
		if !permResult {
			if permResult, err = IsOfficialReviewer(issue, doer); err != nil {
				return false, err
			}
		}

		if !permResult {
			return false, nil
		}
	}

	return true, nil
}

// DeleteReview delete a review and it's code comments
func DeleteReview(r *Review) error {
	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	if r.ID == 0 {
		return fmt.Errorf("review is not allowed to be 0")
	}

	opts := FindCommentsOptions{
		Type:     CommentTypeCode,
		IssueID:  r.IssueID,
		ReviewID: r.ID,
	}

	if _, err := sess.Where(opts.toConds()).Delete(new(Comment)); err != nil {
		return err
	}

	opts = FindCommentsOptions{
		Type:     CommentTypeReview,
		IssueID:  r.IssueID,
		ReviewID: r.ID,
	}

	if _, err := sess.Where(opts.toConds()).Delete(new(Comment)); err != nil {
		return err
	}

	if _, err := sess.ID(r.ID).Delete(new(Review)); err != nil {
		return err
	}

	return sess.Commit()
}

// GetCodeCommentsCount return count of CodeComments a Review has
func (r *Review) GetCodeCommentsCount() int {
	opts := FindCommentsOptions{
		Type:     CommentTypeCode,
		IssueID:  r.IssueID,
		ReviewID: r.ID,
	}
	conds := opts.toConds()
	if r.ID == 0 {
		conds = conds.And(builder.Eq{"invalidated": false})
	}

	count, err := x.Where(conds).Count(new(Comment))
	if err != nil {
		return 0
	}
	return int(count)
}

// HTMLURL formats a URL-string to the related review issue-comment
func (r *Review) HTMLURL() string {
	opts := FindCommentsOptions{
		Type:     CommentTypeReview,
		IssueID:  r.IssueID,
		ReviewID: r.ID,
	}
	comment := new(Comment)
	has, err := x.Where(opts.toConds()).Get(comment)
	if err != nil || !has {
		return ""
	}
	return comment.HTMLURL()
}
