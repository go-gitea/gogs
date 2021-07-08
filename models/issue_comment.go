// Copyright 2018 The Gitea Authors.
// Copyright 2016 The Gogs Authors.
// All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"container/list"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/references"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	jsoniter "github.com/json-iterator/go"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// CommentType defines whether a comment is just a simple comment, an action (like close) or a reference.
type CommentType int

// define unknown comment type
const (
	CommentTypeUnknown CommentType = -1
)

// Enumerate all the comment types
const (
	// 0 Plain comment, can be associated with a commit (CommitID > 0) and a line (LineNum > 0)
	CommentTypeComment CommentType = iota
	CommentTypeReopen              // 1
	CommentTypeClose               // 2

	// 3 References.
	CommentTypeIssueRef
	// 4 Reference from a commit (not part of a pull request)
	CommentTypeCommitRef
	// 5 Reference from a comment
	CommentTypeCommentRef
	// 6 Reference from a pull request
	CommentTypePullRef
	// 7 Labels changed
	CommentTypeLabel
	// 8 Milestone changed
	CommentTypeMilestone
	// 9 Assignees changed
	CommentTypeAssignees
	// 10 Change Title
	CommentTypeChangeTitle
	// 11 Delete Branch
	CommentTypeDeleteBranch
	// 12 Start a stopwatch for time tracking
	CommentTypeStartTracking
	// 13 Stop a stopwatch for time tracking
	CommentTypeStopTracking
	// 14 Add time manual for time tracking
	CommentTypeAddTimeManual
	// 15 Cancel a stopwatch for time tracking
	CommentTypeCancelTracking
	// 16 Added a due date
	CommentTypeAddedDeadline
	// 17 Modified the due date
	CommentTypeModifiedDeadline
	// 18 Removed a due date
	CommentTypeRemovedDeadline
	// 19 Dependency added
	CommentTypeAddDependency
	// 20 Dependency removed
	CommentTypeRemoveDependency
	// 21 Comment a line of code
	CommentTypeCode
	// 22 Reviews a pull request by giving general feedback
	CommentTypeReview
	// 23 Lock an issue, giving only collaborators access
	CommentTypeLock
	// 24 Unlocks a previously locked issue
	CommentTypeUnlock
	// 25 Change pull request's target branch
	CommentTypeChangeTargetBranch
	// 26 Delete time manual for time tracking
	CommentTypeDeleteTimeManual
	// 27 add or remove Request from one
	CommentTypeReviewRequest
	// 28 merge pull request
	CommentTypeMergePull
	// 29 push to PR head branch
	CommentTypePullPush
	// 30 Project changed
	CommentTypeProject
	// 31 Project board changed
	CommentTypeProjectBoard
	// Dismiss Review
	CommentTypeDismissReview
)

// CommentTag defines comment tag type
type CommentTag int

// Enumerate all the comment tag types
const (
	CommentTagNone CommentTag = iota
	CommentTagPoster
	CommentTagWriter
	CommentTagOwner
)

// Comment represents a comment in commit and issue page.
type Comment struct {
	ID               int64       `xorm:"pk autoincr"`
	Type             CommentType `xorm:"INDEX"`
	PosterID         int64       `xorm:"INDEX"`
	Poster           *User       `xorm:"-"`
	OriginalAuthor   string
	OriginalAuthorID int64
	IssueID          int64  `xorm:"INDEX"`
	Issue            *Issue `xorm:"-"`
	LabelID          int64
	Label            *Label   `xorm:"-"`
	AddedLabels      []*Label `xorm:"-"`
	RemovedLabels    []*Label `xorm:"-"`
	OldProjectID     int64
	ProjectID        int64
	OldProject       *Project `xorm:"-"`
	Project          *Project `xorm:"-"`
	OldMilestoneID   int64
	MilestoneID      int64
	OldMilestone     *Milestone `xorm:"-"`
	Milestone        *Milestone `xorm:"-"`
	TimeID           int64
	Time             *TrackedTime `xorm:"-"`
	AssigneeID       int64
	RemovedAssignee  bool
	Assignee         *User `xorm:"-"`
	AssigneeTeamID   int64 `xorm:"NOT NULL DEFAULT 0"`
	AssigneeTeam     *Team `xorm:"-"`
	ResolveDoerID    int64
	ResolveDoer      *User `xorm:"-"`
	OldTitle         string
	NewTitle         string
	OldRef           string
	NewRef           string
	DependentIssueID int64
	DependentIssue   *Issue `xorm:"-"`

	CommitID        int64
	Line            int64 // - previous line / + proposed line
	TreePath        string
	Content         string `xorm:"TEXT"`
	RenderedContent string `xorm:"-"`

	// Path represents the 4 lines of code cemented by this comment
	Patch       string `xorm:"-"`
	PatchQuoted string `xorm:"TEXT patch"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`

	// Reference issue in commit message
	CommitSHA string `xorm:"VARCHAR(40)"`

	Attachments []*Attachment `xorm:"-"`
	Reactions   ReactionList  `xorm:"-"`

	// For view issue page.
	ShowTag CommentTag `xorm:"-"`

	Review      *Review `xorm:"-"`
	ReviewID    int64   `xorm:"index"`
	Invalidated bool

	// Reference an issue or pull from another comment, issue or PR
	// All information is about the origin of the reference
	RefRepoID    int64                 `xorm:"index"` // Repo where the referencing
	RefIssueID   int64                 `xorm:"index"`
	RefCommentID int64                 `xorm:"index"`    // 0 if origin is Issue title or content (or PR's)
	RefAction    references.XRefAction `xorm:"SMALLINT"` // What happens if RefIssueID resolves
	RefIsPull    bool

	RefRepo    *Repository `xorm:"-"`
	RefIssue   *Issue      `xorm:"-"`
	RefComment *Comment    `xorm:"-"`

	Commits     *list.List `xorm:"-"`
	OldCommit   string     `xorm:"-"`
	NewCommit   string     `xorm:"-"`
	CommitsNum  int64      `xorm:"-"`
	IsForcePush bool       `xorm:"-"`
}

// PushActionContent is content of push pull comment
type PushActionContent struct {
	IsForcePush bool     `json:"is_force_push"`
	CommitIDs   []string `json:"commit_ids"`
}

// LoadIssue loads issue from database
func (c *Comment) LoadIssue() (err error) {
	return c.loadIssue(x)
}

func (c *Comment) loadIssue(e Engine) (err error) {
	if c.Issue != nil {
		return nil
	}
	c.Issue, err = getIssueByID(e, c.IssueID)
	return
}

// BeforeInsert will be invoked by XORM before inserting a record
func (c *Comment) BeforeInsert() {
	c.PatchQuoted = c.Patch
	if !utf8.ValidString(c.Patch) {
		c.PatchQuoted = strconv.Quote(c.Patch)
	}
}

// BeforeUpdate will be invoked by XORM before updating a record
func (c *Comment) BeforeUpdate() {
	c.PatchQuoted = c.Patch
	if !utf8.ValidString(c.Patch) {
		c.PatchQuoted = strconv.Quote(c.Patch)
	}
}

// AfterLoad is invoked from XORM after setting the values of all fields of this object.
func (c *Comment) AfterLoad(session *xorm.Session) {
	c.Patch = c.PatchQuoted
	if len(c.PatchQuoted) > 0 && c.PatchQuoted[0] == '"' {
		unquoted, err := strconv.Unquote(c.PatchQuoted)
		if err == nil {
			c.Patch = unquoted
		}
	}
}

func (c *Comment) loadPoster(e Engine) (err error) {
	if c.PosterID <= 0 || c.Poster != nil {
		return nil
	}

	c.Poster, err = getUserByID(e, c.PosterID)
	if err != nil {
		if IsErrUserNotExist(err) {
			c.PosterID = -1
			c.Poster = NewGhostUser()
		} else {
			log.Error("getUserByID[%d]: %v", c.ID, err)
		}
	}
	return err
}

// AfterDelete is invoked from XORM after the object is deleted.
func (c *Comment) AfterDelete() {
	if c.ID <= 0 {
		return
	}

	_, err := DeleteAttachmentsByComment(c.ID, true)
	if err != nil {
		log.Info("Could not delete files for comment %d on issue #%d: %s", c.ID, c.IssueID, err)
	}
}

// HTMLURL formats a URL-string to the issue-comment
func (c *Comment) HTMLURL() string {
	err := c.LoadIssue()
	if err != nil { // Silently dropping errors :unamused:
		log.Error("LoadIssue(%d): %v", c.IssueID, err)
		return ""
	}
	err = c.Issue.loadRepo(x)
	if err != nil { // Silently dropping errors :unamused:
		log.Error("loadRepo(%d): %v", c.Issue.RepoID, err)
		return ""
	}
	if c.Type == CommentTypeCode {
		if c.ReviewID == 0 {
			return fmt.Sprintf("%s/files#%s", c.Issue.HTMLURL(), c.HashTag())
		}
		if c.Review == nil {
			if err := c.LoadReview(); err != nil {
				log.Warn("LoadReview(%d): %v", c.ReviewID, err)
				return fmt.Sprintf("%s/files#%s", c.Issue.HTMLURL(), c.HashTag())
			}
		}
		if c.Review.Type <= ReviewTypePending {
			return fmt.Sprintf("%s/files#%s", c.Issue.HTMLURL(), c.HashTag())
		}
	}
	return fmt.Sprintf("%s#%s", c.Issue.HTMLURL(), c.HashTag())
}

// APIURL formats a API-string to the issue-comment
func (c *Comment) APIURL() string {
	err := c.LoadIssue()
	if err != nil { // Silently dropping errors :unamused:
		log.Error("LoadIssue(%d): %v", c.IssueID, err)
		return ""
	}
	err = c.Issue.loadRepo(x)
	if err != nil { // Silently dropping errors :unamused:
		log.Error("loadRepo(%d): %v", c.Issue.RepoID, err)
		return ""
	}

	return fmt.Sprintf("%s/issues/comments/%d", c.Issue.Repo.APIURL(), c.ID)
}

// IssueURL formats a URL-string to the issue
func (c *Comment) IssueURL() string {
	err := c.LoadIssue()
	if err != nil { // Silently dropping errors :unamused:
		log.Error("LoadIssue(%d): %v", c.IssueID, err)
		return ""
	}

	if c.Issue.IsPull {
		return ""
	}

	err = c.Issue.loadRepo(x)
	if err != nil { // Silently dropping errors :unamused:
		log.Error("loadRepo(%d): %v", c.Issue.RepoID, err)
		return ""
	}
	return c.Issue.HTMLURL()
}

// PRURL formats a URL-string to the pull-request
func (c *Comment) PRURL() string {
	err := c.LoadIssue()
	if err != nil { // Silently dropping errors :unamused:
		log.Error("LoadIssue(%d): %v", c.IssueID, err)
		return ""
	}

	err = c.Issue.loadRepo(x)
	if err != nil { // Silently dropping errors :unamused:
		log.Error("loadRepo(%d): %v", c.Issue.RepoID, err)
		return ""
	}

	if !c.Issue.IsPull {
		return ""
	}
	return c.Issue.HTMLURL()
}

// CommentHashTag returns unique hash tag for comment id.
func CommentHashTag(id int64) string {
	return fmt.Sprintf("issuecomment-%d", id)
}

// HashTag returns unique hash tag for comment.
func (c *Comment) HashTag() string {
	return CommentHashTag(c.ID)
}

// EventTag returns unique event hash tag for comment.
func (c *Comment) EventTag() string {
	return fmt.Sprintf("event-%d", c.ID)
}

// LoadLabel if comment.Type is CommentTypeLabel, then load Label
func (c *Comment) LoadLabel() error {
	var label Label
	has, err := x.ID(c.LabelID).Get(&label)
	if err != nil {
		return err
	} else if has {
		c.Label = &label
	} else {
		// Ignore Label is deleted, but not clear this table
		log.Warn("Commit %d cannot load label %d", c.ID, c.LabelID)
	}

	return nil
}

// LoadProject if comment.Type is CommentTypeProject, then load project.
func (c *Comment) LoadProject() error {
	if c.OldProjectID > 0 {
		var oldProject Project
		has, err := x.ID(c.OldProjectID).Get(&oldProject)
		if err != nil {
			return err
		} else if has {
			c.OldProject = &oldProject
		}
	}

	if c.ProjectID > 0 {
		var project Project
		has, err := x.ID(c.ProjectID).Get(&project)
		if err != nil {
			return err
		} else if has {
			c.Project = &project
		}
	}

	return nil
}

// LoadMilestone if comment.Type is CommentTypeMilestone, then load milestone
func (c *Comment) LoadMilestone() error {
	if c.OldMilestoneID > 0 {
		var oldMilestone Milestone
		has, err := x.ID(c.OldMilestoneID).Get(&oldMilestone)
		if err != nil {
			return err
		} else if has {
			c.OldMilestone = &oldMilestone
		}
	}

	if c.MilestoneID > 0 {
		var milestone Milestone
		has, err := x.ID(c.MilestoneID).Get(&milestone)
		if err != nil {
			return err
		} else if has {
			c.Milestone = &milestone
		}
	}
	return nil
}

// LoadPoster loads comment poster
func (c *Comment) LoadPoster() error {
	return c.loadPoster(x)
}

// LoadAttachments loads attachments
func (c *Comment) LoadAttachments() error {
	if len(c.Attachments) > 0 {
		return nil
	}

	var err error
	c.Attachments, err = getAttachmentsByCommentID(x, c.ID)
	if err != nil {
		log.Error("getAttachmentsByCommentID[%d]: %v", c.ID, err)
	}
	return nil
}

// UpdateAttachments update attachments by UUIDs for the comment
func (c *Comment) UpdateAttachments(uuids []string) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	attachments, err := getAttachmentsByUUIDs(sess, uuids)
	if err != nil {
		return fmt.Errorf("getAttachmentsByUUIDs [uuids: %v]: %v", uuids, err)
	}
	for i := 0; i < len(attachments); i++ {
		attachments[i].IssueID = c.IssueID
		attachments[i].CommentID = c.ID
		if err := updateAttachment(sess, attachments[i]); err != nil {
			return fmt.Errorf("update attachment [id: %d]: %v", attachments[i].ID, err)
		}
	}
	return sess.Commit()
}

// LoadAssigneeUserAndTeam if comment.Type is CommentTypeAssignees, then load assignees
func (c *Comment) LoadAssigneeUserAndTeam() error {
	var err error

	if c.AssigneeID > 0 && c.Assignee == nil {
		c.Assignee, err = getUserByID(x, c.AssigneeID)
		if err != nil {
			if !IsErrUserNotExist(err) {
				return err
			}
			c.Assignee = NewGhostUser()
		}
	} else if c.AssigneeTeamID > 0 && c.AssigneeTeam == nil {
		if err = c.LoadIssue(); err != nil {
			return err
		}

		if err = c.Issue.LoadRepo(); err != nil {
			return err
		}

		if err = c.Issue.Repo.GetOwner(); err != nil {
			return err
		}

		if c.Issue.Repo.Owner.IsOrganization() {
			c.AssigneeTeam, err = GetTeamByID(c.AssigneeTeamID)
			if err != nil && !IsErrTeamNotExist(err) {
				return err
			}
		}
	}
	return nil
}

// LoadResolveDoer if comment.Type is CommentTypeCode and ResolveDoerID not zero, then load resolveDoer
func (c *Comment) LoadResolveDoer() (err error) {
	if c.ResolveDoerID == 0 || c.Type != CommentTypeCode {
		return nil
	}
	c.ResolveDoer, err = getUserByID(x, c.ResolveDoerID)
	if err != nil {
		if IsErrUserNotExist(err) {
			c.ResolveDoer = NewGhostUser()
			err = nil
		}
	}
	return
}

// IsResolved check if an code comment is resolved
func (c *Comment) IsResolved() bool {
	return c.ResolveDoerID != 0 && c.Type == CommentTypeCode
}

// LoadDepIssueDetails loads Dependent Issue Details
func (c *Comment) LoadDepIssueDetails() (err error) {
	if c.DependentIssueID <= 0 || c.DependentIssue != nil {
		return nil
	}
	c.DependentIssue, err = getIssueByID(x, c.DependentIssueID)
	return err
}

// LoadTime loads the associated time for a CommentTypeAddTimeManual
func (c *Comment) LoadTime() error {
	if c.Time != nil || c.TimeID == 0 {
		return nil
	}
	var err error
	c.Time, err = GetTrackedTimeByID(c.TimeID)
	return err
}

func (c *Comment) loadReactions(e Engine, repo *Repository) (err error) {
	if c.Reactions != nil {
		return nil
	}
	c.Reactions, err = findReactions(e, FindReactionsOptions{
		IssueID:   c.IssueID,
		CommentID: c.ID,
	})
	if err != nil {
		return err
	}
	// Load reaction user data
	if _, err := c.Reactions.loadUsers(e, repo); err != nil {
		return err
	}
	return nil
}

// LoadReactions loads comment reactions
func (c *Comment) LoadReactions(repo *Repository) error {
	return c.loadReactions(x, repo)
}

func (c *Comment) loadReview(e Engine) (err error) {
	if c.Review == nil {
		if c.Review, err = getReviewByID(e, c.ReviewID); err != nil {
			return err
		}
	}
	c.Review.Issue = c.Issue
	return nil
}

// LoadReview loads the associated review
func (c *Comment) LoadReview() error {
	return c.loadReview(x)
}

var notEnoughLines = regexp.MustCompile(`fatal: file .* has only \d+ lines?`)

func (c *Comment) checkInvalidation(doer *User, repo *git.Repository, branch string) error {
	// FIXME differentiate between previous and proposed line
	commit, err := repo.LineBlame(branch, repo.Path, c.TreePath, uint(c.UnsignedLine()))
	if err != nil && (strings.Contains(err.Error(), "fatal: no such path") || notEnoughLines.MatchString(err.Error())) {
		c.Invalidated = true
		return UpdateComment(c, doer)
	}
	if err != nil {
		return err
	}
	if c.CommitSHA != "" && c.CommitSHA != commit.ID.String() {
		c.Invalidated = true
		return UpdateComment(c, doer)
	}
	return nil
}

// CheckInvalidation checks if the line of code comment got changed by another commit.
// If the line got changed the comment is going to be invalidated.
func (c *Comment) CheckInvalidation(repo *git.Repository, doer *User, branch string) error {
	return c.checkInvalidation(doer, repo, branch)
}

// DiffSide returns "previous" if Comment.Line is a LOC of the previous changes and "proposed" if it is a LOC of the proposed changes.
func (c *Comment) DiffSide() string {
	if c.Line < 0 {
		return "previous"
	}
	return "proposed"
}

// UnsignedLine returns the LOC of the code comment without + or -
func (c *Comment) UnsignedLine() uint64 {
	if c.Line < 0 {
		return uint64(c.Line * -1)
	}
	return uint64(c.Line)
}

// CodeCommentURL returns the url to a comment in code
func (c *Comment) CodeCommentURL() string {
	err := c.LoadIssue()
	if err != nil { // Silently dropping errors :unamused:
		log.Error("LoadIssue(%d): %v", c.IssueID, err)
		return ""
	}
	err = c.Issue.loadRepo(x)
	if err != nil { // Silently dropping errors :unamused:
		log.Error("loadRepo(%d): %v", c.Issue.RepoID, err)
		return ""
	}
	return fmt.Sprintf("%s/files#%s", c.Issue.HTMLURL(), c.HashTag())
}

// LoadPushCommits Load push commits
func (c *Comment) LoadPushCommits() (err error) {
	if c.Content == "" || c.Commits != nil || c.Type != CommentTypePullPush {
		return nil
	}

	var data PushActionContent

	json := jsoniter.ConfigCompatibleWithStandardLibrary
	err = json.Unmarshal([]byte(c.Content), &data)
	if err != nil {
		return
	}

	c.IsForcePush = data.IsForcePush

	if c.IsForcePush {
		if len(data.CommitIDs) != 2 {
			return nil
		}
		c.OldCommit = data.CommitIDs[0]
		c.NewCommit = data.CommitIDs[1]
	} else {
		repoPath := c.Issue.Repo.RepoPath()
		gitRepo, err := git.OpenRepository(repoPath)
		if err != nil {
			return err
		}
		defer gitRepo.Close()

		c.Commits = gitRepo.GetCommitsFromIDs(data.CommitIDs)
		c.CommitsNum = int64(c.Commits.Len())
		if c.CommitsNum > 0 {
			c.Commits = ValidateCommitsWithEmails(c.Commits)
			c.Commits = ParseCommitsWithSignature(c.Commits, c.Issue.Repo)
			c.Commits = ParseCommitsWithStatus(c.Commits, c.Issue.Repo)
		}
	}

	return err
}

func createComment(e *xorm.Session, opts *CreateCommentOptions) (_ *Comment, err error) {
	var LabelID int64
	if opts.Label != nil {
		LabelID = opts.Label.ID
	}

	comment := &Comment{
		Type:             opts.Type,
		PosterID:         opts.Doer.ID,
		Poster:           opts.Doer,
		IssueID:          opts.Issue.ID,
		LabelID:          LabelID,
		OldMilestoneID:   opts.OldMilestoneID,
		MilestoneID:      opts.MilestoneID,
		OldProjectID:     opts.OldProjectID,
		ProjectID:        opts.ProjectID,
		TimeID:           opts.TimeID,
		RemovedAssignee:  opts.RemovedAssignee,
		AssigneeID:       opts.AssigneeID,
		AssigneeTeamID:   opts.AssigneeTeamID,
		CommitID:         opts.CommitID,
		CommitSHA:        opts.CommitSHA,
		Line:             opts.LineNum,
		Content:          opts.Content,
		OldTitle:         opts.OldTitle,
		NewTitle:         opts.NewTitle,
		OldRef:           opts.OldRef,
		NewRef:           opts.NewRef,
		DependentIssueID: opts.DependentIssueID,
		TreePath:         opts.TreePath,
		ReviewID:         opts.ReviewID,
		Patch:            opts.Patch,
		RefRepoID:        opts.RefRepoID,
		RefIssueID:       opts.RefIssueID,
		RefCommentID:     opts.RefCommentID,
		RefAction:        opts.RefAction,
		RefIsPull:        opts.RefIsPull,
		IsForcePush:      opts.IsForcePush,
		Invalidated:      opts.Invalidated,
	}
	if _, err = e.Insert(comment); err != nil {
		return nil, err
	}

	if err = opts.Repo.getOwner(e); err != nil {
		return nil, err
	}

	if err = updateCommentInfos(e, opts, comment); err != nil {
		return nil, err
	}

	if err = comment.addCrossReferences(e, opts.Doer, false); err != nil {
		return nil, err
	}

	return comment, nil
}

func updateCommentInfos(e *xorm.Session, opts *CreateCommentOptions, comment *Comment) (err error) {
	// Check comment type.
	switch opts.Type {
	case CommentTypeCode:
		if comment.ReviewID != 0 {
			if comment.Review == nil {
				if err := comment.loadReview(e); err != nil {
					return err
				}
			}
			if comment.Review.Type <= ReviewTypePending {
				return nil
			}
		}
		fallthrough
	case CommentTypeReview:
		fallthrough
	case CommentTypeComment:
		if _, err = e.Exec("UPDATE `issue` SET num_comments=num_comments+1 WHERE id=?", opts.Issue.ID); err != nil {
			return err
		}

		// Check attachments
		attachments, err := getAttachmentsByUUIDs(e, opts.Attachments)
		if err != nil {
			return fmt.Errorf("getAttachmentsByUUIDs [uuids: %v]: %v", opts.Attachments, err)
		}

		for i := range attachments {
			attachments[i].IssueID = opts.Issue.ID
			attachments[i].CommentID = comment.ID
			// No assign value could be 0, so ignore AllCols().
			if _, err = e.ID(attachments[i].ID).Update(attachments[i]); err != nil {
				return fmt.Errorf("update attachment [%d]: %v", attachments[i].ID, err)
			}
		}
	case CommentTypeReopen, CommentTypeClose:
		if err = opts.Issue.updateClosedNum(e); err != nil {
			return err
		}
	}
	// update the issue's updated_unix column
	return updateIssueCols(e, opts.Issue, "updated_unix")
}

func createDeadlineComment(e *xorm.Session, doer *User, issue *Issue, newDeadlineUnix timeutil.TimeStamp) (*Comment, error) {
	var content string
	var commentType CommentType

	// newDeadline = 0 means deleting
	if newDeadlineUnix == 0 {
		commentType = CommentTypeRemovedDeadline
		content = issue.DeadlineUnix.Format("2006-01-02")
	} else if issue.DeadlineUnix == 0 {
		// Check if the new date was added or modified
		// If the actual deadline is 0 => deadline added
		commentType = CommentTypeAddedDeadline
		content = newDeadlineUnix.Format("2006-01-02")
	} else { // Otherwise modified
		commentType = CommentTypeModifiedDeadline
		content = newDeadlineUnix.Format("2006-01-02") + "|" + issue.DeadlineUnix.Format("2006-01-02")
	}

	if err := issue.loadRepo(e); err != nil {
		return nil, err
	}

	opts := &CreateCommentOptions{
		Type:    commentType,
		Doer:    doer,
		Repo:    issue.Repo,
		Issue:   issue,
		Content: content,
	}
	comment, err := createComment(e, opts)
	if err != nil {
		return nil, err
	}
	return comment, nil
}

// Creates issue dependency comment
func createIssueDependencyComment(e *xorm.Session, doer *User, issue, dependentIssue *Issue, add bool) (err error) {
	cType := CommentTypeAddDependency
	if !add {
		cType = CommentTypeRemoveDependency
	}
	if err = issue.loadRepo(e); err != nil {
		return
	}

	// Make two comments, one in each issue
	opts := &CreateCommentOptions{
		Type:             cType,
		Doer:             doer,
		Repo:             issue.Repo,
		Issue:            issue,
		DependentIssueID: dependentIssue.ID,
	}
	if _, err = createComment(e, opts); err != nil {
		return
	}

	opts = &CreateCommentOptions{
		Type:             cType,
		Doer:             doer,
		Repo:             issue.Repo,
		Issue:            dependentIssue,
		DependentIssueID: issue.ID,
	}
	_, err = createComment(e, opts)
	return
}

// CreateCommentOptions defines options for creating comment
type CreateCommentOptions struct {
	Type  CommentType
	Doer  *User
	Repo  *Repository
	Issue *Issue
	Label *Label

	DependentIssueID int64
	OldMilestoneID   int64
	MilestoneID      int64
	OldProjectID     int64
	ProjectID        int64
	TimeID           int64
	AssigneeID       int64
	AssigneeTeamID   int64
	RemovedAssignee  bool
	OldTitle         string
	NewTitle         string
	OldRef           string
	NewRef           string
	CommitID         int64
	CommitSHA        string
	Patch            string
	LineNum          int64
	TreePath         string
	ReviewID         int64
	Content          string
	Attachments      []string // UUIDs of attachments
	RefRepoID        int64
	RefIssueID       int64
	RefCommentID     int64
	RefAction        references.XRefAction
	RefIsPull        bool
	IsForcePush      bool
	Invalidated      bool
}

// CreateComment creates comment of issue or commit.
func CreateComment(opts *CreateCommentOptions) (comment *Comment, err error) {
	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return nil, err
	}

	comment, err = createComment(sess, opts)
	if err != nil {
		return nil, err
	}

	if err = sess.Commit(); err != nil {
		return nil, err
	}

	return comment, nil
}

// CreateRefComment creates a commit reference comment to issue.
func CreateRefComment(doer *User, repo *Repository, issue *Issue, content, commitSHA string) error {
	if len(commitSHA) == 0 {
		return fmt.Errorf("cannot create reference with empty commit SHA")
	}

	// Check if same reference from same commit has already existed.
	has, err := x.Get(&Comment{
		Type:      CommentTypeCommitRef,
		IssueID:   issue.ID,
		CommitSHA: commitSHA,
	})
	if err != nil {
		return fmt.Errorf("check reference comment: %v", err)
	} else if has {
		return nil
	}

	_, err = CreateComment(&CreateCommentOptions{
		Type:      CommentTypeCommitRef,
		Doer:      doer,
		Repo:      repo,
		Issue:     issue,
		CommitSHA: commitSHA,
		Content:   content,
	})
	return err
}

// GetCommentByID returns the comment by given ID.
func GetCommentByID(id int64) (*Comment, error) {
	return getCommentByID(x, id)
}

func getCommentByID(e Engine, id int64) (*Comment, error) {
	c := new(Comment)
	has, err := e.ID(id).Get(c)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrCommentNotExist{id, 0}
	}
	return c, nil
}

// FindCommentsOptions describes the conditions to Find comments
type FindCommentsOptions struct {
	ListOptions
	RepoID   int64
	IssueID  int64
	ReviewID int64
	Since    int64
	Before   int64
	Line     int64
	TreePath string
	Type     CommentType
}

func (opts *FindCommentsOptions) toConds() builder.Cond {
	cond := builder.NewCond()
	if opts.RepoID > 0 {
		cond = cond.And(builder.Eq{"issue.repo_id": opts.RepoID})
	}
	if opts.IssueID > 0 {
		cond = cond.And(builder.Eq{"comment.issue_id": opts.IssueID})
	}
	if opts.ReviewID > 0 {
		cond = cond.And(builder.Eq{"comment.review_id": opts.ReviewID})
	}
	if opts.Since > 0 {
		cond = cond.And(builder.Gte{"comment.updated_unix": opts.Since})
	}
	if opts.Before > 0 {
		cond = cond.And(builder.Lte{"comment.updated_unix": opts.Before})
	}
	if opts.Type != CommentTypeUnknown {
		cond = cond.And(builder.Eq{"comment.type": opts.Type})
	}
	if opts.Line != 0 {
		cond = cond.And(builder.Eq{"comment.line": opts.Line})
	}
	if len(opts.TreePath) > 0 {
		cond = cond.And(builder.Eq{"comment.tree_path": opts.TreePath})
	}
	return cond
}

func findComments(e Engine, opts FindCommentsOptions) ([]*Comment, error) {
	comments := make([]*Comment, 0, 10)
	sess := e.Where(opts.toConds())
	if opts.RepoID > 0 {
		sess.Join("INNER", "issue", "issue.id = comment.issue_id")
	}

	if opts.Page != 0 {
		sess = opts.setSessionPagination(sess)
	}

	// WARNING: If you change this order you will need to fix createCodeComment

	return comments, sess.
		Asc("comment.created_unix").
		Asc("comment.id").
		Find(&comments)
}

// FindComments returns all comments according options
func FindComments(opts FindCommentsOptions) ([]*Comment, error) {
	return findComments(x, opts)
}

// UpdateComment updates information of comment.
func UpdateComment(c *Comment, doer *User) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if _, err := sess.ID(c.ID).AllCols().Update(c); err != nil {
		return err
	}
	if err := c.loadIssue(sess); err != nil {
		return err
	}
	if err := c.addCrossReferences(sess, doer, true); err != nil {
		return err
	}
	if err := sess.Commit(); err != nil {
		return fmt.Errorf("Commit: %v", err)
	}

	return nil
}

// DeleteComment deletes the comment
func DeleteComment(comment *Comment) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if err := deleteComment(sess, comment); err != nil {
		return err
	}

	return sess.Commit()
}

func deleteComment(e Engine, comment *Comment) error {
	if _, err := e.Delete(&Comment{
		ID: comment.ID,
	}); err != nil {
		return err
	}

	if comment.Type == CommentTypeComment {
		if _, err := e.Exec("UPDATE `issue` SET num_comments = num_comments - 1 WHERE id = ?", comment.IssueID); err != nil {
			return err
		}
	}
	if _, err := e.Where("comment_id = ?", comment.ID).Cols("is_deleted").Update(&Action{IsDeleted: true}); err != nil {
		return err
	}

	if err := comment.neuterCrossReferences(e); err != nil {
		return err
	}

	return deleteReaction(e, &ReactionOptions{Comment: comment})
}

// CodeComments represents comments on code by using this structure: FILENAME -> LINE (+ == proposed; - == previous) -> COMMENTS
type CodeComments map[string]map[int64][]*Comment

func fetchCodeComments(e Engine, issue *Issue, currentUser *User) (CodeComments, error) {
	return fetchCodeCommentsByReview(e, issue, currentUser, nil)
}

func fetchCodeCommentsByReview(e Engine, issue *Issue, currentUser *User, review *Review) (CodeComments, error) {
	pathToLineToComment := make(CodeComments)
	if review == nil {
		review = &Review{ID: 0}
	}
	opts := FindCommentsOptions{
		Type:     CommentTypeCode,
		IssueID:  issue.ID,
		ReviewID: review.ID,
	}

	comments, err := findCodeComments(e, opts, issue, currentUser, review)
	if err != nil {
		return nil, err
	}

	for _, comment := range comments {
		if pathToLineToComment[comment.TreePath] == nil {
			pathToLineToComment[comment.TreePath] = make(map[int64][]*Comment)
		}
		pathToLineToComment[comment.TreePath][comment.Line] = append(pathToLineToComment[comment.TreePath][comment.Line], comment)
	}
	return pathToLineToComment, nil
}

func findCodeComments(e Engine, opts FindCommentsOptions, issue *Issue, currentUser *User, review *Review) ([]*Comment, error) {
	var comments []*Comment
	if review == nil {
		review = &Review{ID: 0}
	}
	conds := opts.toConds()
	if review.ID == 0 {
		conds = conds.And(builder.Eq{"invalidated": false})
	}
	if err := e.Where(conds).
		Asc("comment.created_unix").
		Asc("comment.id").
		Find(&comments); err != nil {
		return nil, err
	}

	if err := issue.loadRepo(e); err != nil {
		return nil, err
	}

	if err := CommentList(comments).loadPosters(e); err != nil {
		return nil, err
	}

	// Find all reviews by ReviewID
	reviews := make(map[int64]*Review)
	ids := make([]int64, 0, len(comments))
	for _, comment := range comments {
		if comment.ReviewID != 0 {
			ids = append(ids, comment.ReviewID)
		}
	}
	if err := e.In("id", ids).Find(&reviews); err != nil {
		return nil, err
	}

	n := 0
	for _, comment := range comments {
		if re, ok := reviews[comment.ReviewID]; ok && re != nil {
			// If the review is pending only the author can see the comments (except if the review is set)
			if review.ID == 0 && re.Type == ReviewTypePending &&
				(currentUser == nil || currentUser.ID != re.ReviewerID) {
				continue
			}
			comment.Review = re
		}
		comments[n] = comment
		n++

		if err := comment.LoadResolveDoer(); err != nil {
			return nil, err
		}

		if err := comment.LoadReactions(issue.Repo); err != nil {
			return nil, err
		}

		var err error
		if comment.RenderedContent, err = markdown.RenderString(&markup.RenderContext{
			URLPrefix: issue.Repo.Link(),
			Metas:     issue.Repo.ComposeMetas(),
		}, comment.Content); err != nil {
			return nil, err
		}
	}
	return comments[:n], nil
}

// FetchCodeCommentsByLine fetches the code comments for a given treePath and line number
func FetchCodeCommentsByLine(issue *Issue, currentUser *User, treePath string, line int64) ([]*Comment, error) {
	opts := FindCommentsOptions{
		Type:     CommentTypeCode,
		IssueID:  issue.ID,
		TreePath: treePath,
		Line:     line,
	}
	return findCodeComments(x, opts, issue, currentUser, nil)
}

// FetchCodeComments will return a 2d-map: ["Path"]["Line"] = Comments at line
func FetchCodeComments(issue *Issue, currentUser *User) (CodeComments, error) {
	return fetchCodeComments(x, issue, currentUser)
}

// UpdateCommentsMigrationsByType updates comments' migrations information via given git service type and original id and poster id
func UpdateCommentsMigrationsByType(tp structs.GitServiceType, originalAuthorID string, posterID int64) error {
	_, err := x.Table("comment").
		Where(builder.In("issue_id",
			builder.Select("issue.id").
				From("issue").
				InnerJoin("repository", "issue.repo_id = repository.id").
				Where(builder.Eq{
					"repository.original_service_type": tp,
				}),
		)).
		And("comment.original_author_id = ?", originalAuthorID).
		Update(map[string]interface{}{
			"poster_id":          posterID,
			"original_author":    "",
			"original_author_id": 0,
		})
	return err
}

// CreatePushPullComment create push code to pull base comment
func CreatePushPullComment(pusher *User, pr *PullRequest, oldCommitID, newCommitID string) (comment *Comment, err error) {
	if pr.HasMerged || oldCommitID == "" || newCommitID == "" {
		return nil, nil
	}

	ops := &CreateCommentOptions{
		Type: CommentTypePullPush,
		Doer: pusher,
		Repo: pr.BaseRepo,
	}

	var data PushActionContent

	data.CommitIDs, data.IsForcePush, err = getCommitIDsFromRepo(pr.BaseRepo, oldCommitID, newCommitID, pr.BaseBranch)
	if err != nil {
		return nil, err
	}

	ops.Issue = pr.Issue

	json := jsoniter.ConfigCompatibleWithStandardLibrary
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	ops.Content = string(dataJSON)

	comment, err = CreateComment(ops)

	return
}

// getCommitsFromRepo get commit IDs from repo in between oldCommitID and newCommitID
// isForcePush will be true if oldCommit isn't on the branch
// Commit on baseBranch will skip
func getCommitIDsFromRepo(repo *Repository, oldCommitID, newCommitID, baseBranch string) (commitIDs []string, isForcePush bool, err error) {
	repoPath := repo.RepoPath()
	gitRepo, err := git.OpenRepository(repoPath)
	if err != nil {
		return nil, false, err
	}
	defer gitRepo.Close()

	oldCommit, err := gitRepo.GetCommit(oldCommitID)
	if err != nil {
		return nil, false, err
	}

	if err = oldCommit.LoadBranchName(); err != nil {
		return nil, false, err
	}

	if len(oldCommit.Branch) == 0 {
		commitIDs = make([]string, 2)
		commitIDs[0] = oldCommitID
		commitIDs[1] = newCommitID

		return commitIDs, true, err
	}

	newCommit, err := gitRepo.GetCommit(newCommitID)
	if err != nil {
		return nil, false, err
	}

	var (
		commits      *list.List
		commitChecks map[string]commitBranchCheckItem
	)
	commits, err = newCommit.CommitsBeforeUntil(oldCommitID)
	if err != nil {
		return nil, false, err
	}

	commitIDs = make([]string, 0, commits.Len())
	commitChecks = make(map[string]commitBranchCheckItem)

	for e := commits.Front(); e != nil; e = e.Next() {
		commitChecks[e.Value.(*git.Commit).ID.String()] = commitBranchCheckItem{
			Commit:  e.Value.(*git.Commit),
			Checked: false,
		}
	}

	if err = commitBranchCheck(gitRepo, newCommit, oldCommitID, baseBranch, commitChecks); err != nil {
		return
	}

	for e := commits.Back(); e != nil; e = e.Prev() {
		commitID := e.Value.(*git.Commit).ID.String()
		if item, ok := commitChecks[commitID]; ok && item.Checked {
			commitIDs = append(commitIDs, commitID)
		}
	}

	return
}

type commitBranchCheckItem struct {
	Commit  *git.Commit
	Checked bool
}

func commitBranchCheck(gitRepo *git.Repository, startCommit *git.Commit, endCommitID, baseBranch string, commitList map[string]commitBranchCheckItem) (err error) {
	var (
		item     commitBranchCheckItem
		ok       bool
		listItem *list.Element
		tmp      string
	)

	if startCommit.ID.String() == endCommitID {
		return
	}

	checkStack := list.New()
	checkStack.PushBack(startCommit.ID.String())
	listItem = checkStack.Back()

	for listItem != nil {
		tmp = listItem.Value.(string)
		checkStack.Remove(listItem)

		if item, ok = commitList[tmp]; !ok {
			listItem = checkStack.Back()
			continue
		}

		if item.Commit.ID.String() == endCommitID {
			listItem = checkStack.Back()
			continue
		}

		if err = item.Commit.LoadBranchName(); err != nil {
			return
		}

		if item.Commit.Branch == baseBranch {
			listItem = checkStack.Back()
			continue
		}

		if item.Checked {
			listItem = checkStack.Back()
			continue
		}

		item.Checked = true
		commitList[tmp] = item

		parentNum := item.Commit.ParentCount()
		for i := 0; i < parentNum; i++ {
			var parentCommit *git.Commit
			parentCommit, err = item.Commit.Parent(i)
			if err != nil {
				return
			}
			checkStack.PushBack(parentCommit.ID.String())
		}

		listItem = checkStack.Back()
	}
	return nil
}
