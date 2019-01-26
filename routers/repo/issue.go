// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/Unknwon/com"
	"github.com/Unknwon/paginater"

	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/indexer"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/notification"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

const (
	tplIssues    base.TplName = "repo/issue/list"
	tplIssueNew  base.TplName = "repo/issue/new"
	tplIssueView base.TplName = "repo/issue/view"

	tplReactions base.TplName = "repo/issue/view_content/reactions"

	issueTemplateKey = "IssueTemplate"
)

var (
	// ErrFileTypeForbidden not allowed file type error
	ErrFileTypeForbidden = errors.New("File type is not allowed")
	// ErrTooManyFiles upload too many files
	ErrTooManyFiles = errors.New("Maximum number of files to upload exceeded")
	// IssueTemplateCandidates issue templates
	IssueTemplateCandidates = []string{
		"ISSUE_TEMPLATE.md",
		"issue_template.md",
		".gitea/ISSUE_TEMPLATE.md",
		".gitea/issue_template.md",
		".github/ISSUE_TEMPLATE.md",
		".github/issue_template.md",
	}
)

// MustEnableIssues check if repository enable internal issues
func MustEnableIssues(ctx *context.Context) {
	if !ctx.Repo.CanRead(models.UnitTypeIssues) &&
		!ctx.Repo.CanRead(models.UnitTypeExternalTracker) {
		ctx.NotFound("MustEnableIssues", nil)
		return
	}

	unit, err := ctx.Repo.Repository.GetUnit(models.UnitTypeExternalTracker)
	if err == nil {
		ctx.Redirect(unit.ExternalTrackerConfig().ExternalTrackerURL)
		return
	}
}

// MustAllowPulls check if repository enable pull requests and user have right to do that
func MustAllowPulls(ctx *context.Context) {
	if !ctx.Repo.Repository.CanEnablePulls() || !ctx.Repo.CanRead(models.UnitTypePullRequests) {
		ctx.NotFound("MustAllowPulls", nil)
		return
	}

	// User can send pull request if owns a forked repository.
	if ctx.IsSigned && ctx.User.HasForkedRepo(ctx.Repo.Repository.ID) {
		ctx.Repo.PullRequest.Allowed = true
		ctx.Repo.PullRequest.HeadInfo = ctx.User.Name + ":" + ctx.Repo.BranchName
	}
}

func issues(ctx *context.Context, milestoneID int64, isPullOption util.OptionalBool) {
	var err error
	viewType := ctx.Query("type")
	sortType := ctx.Query("sort")
	types := []string{"all", "your_repositories", "assigned", "created_by", "mentioned"}
	if !com.IsSliceContainsStr(types, viewType) {
		viewType = "all"
	}

	var (
		assigneeID  = ctx.QueryInt64("assignee")
		posterID    int64
		mentionedID int64
		forceEmpty  bool
	)

	if ctx.IsSigned {
		switch viewType {
		case "created_by":
			posterID = ctx.User.ID
		case "mentioned":
			mentionedID = ctx.User.ID
		}
	}

	repo := ctx.Repo.Repository
	var labelIDs []int64
	selectLabels := ctx.Query("labels")
	if len(selectLabels) > 0 && selectLabels != "0" {
		labelIDs, err = base.StringsToInt64s(strings.Split(selectLabels, ","))
		if err != nil {
			ctx.ServerError("StringsToInt64s", err)
			return
		}
	}
	isShowClosed := ctx.Query("state") == "closed"

	keyword := strings.Trim(ctx.Query("q"), " ")
	if bytes.Contains([]byte(keyword), []byte{0x00}) {
		keyword = ""
	}

	var issueIDs []int64
	if len(keyword) > 0 {
		issueIDs, err = indexer.SearchIssuesByKeyword(repo.ID, keyword)
		if len(issueIDs) == 0 {
			forceEmpty = true
		}
	}

	var issueStats *models.IssueStats
	if forceEmpty {
		issueStats = &models.IssueStats{}
	} else {
		issueStats, err = models.GetIssueStats(&models.IssueStatsOptions{
			RepoID:      repo.ID,
			Labels:      selectLabels,
			MilestoneID: milestoneID,
			AssigneeID:  assigneeID,
			MentionedID: mentionedID,
			PosterID:    posterID,
			IsPull:      isPullOption,
			IssueIDs:    issueIDs,
		})
		if err != nil {
			ctx.ServerError("GetIssueStats", err)
			return
		}
	}
	page := ctx.QueryInt("page")
	if page <= 1 {
		page = 1
	}

	var total int
	if !isShowClosed {
		total = int(issueStats.OpenCount)
	} else {
		total = int(issueStats.ClosedCount)
	}
	pager := paginater.New(total, setting.UI.IssuePagingNum, page, 5)
	ctx.Data["Page"] = pager

	var issues []*models.Issue
	if forceEmpty {
		issues = []*models.Issue{}
	} else {
		issues, err = models.Issues(&models.IssuesOptions{
			RepoIDs:     []int64{repo.ID},
			AssigneeID:  assigneeID,
			PosterID:    posterID,
			MentionedID: mentionedID,
			MilestoneID: milestoneID,
			Page:        pager.Current(),
			PageSize:    setting.UI.IssuePagingNum,
			IsClosed:    util.OptionalBoolOf(isShowClosed),
			IsPull:      isPullOption,
			LabelIDs:    labelIDs,
			SortType:    sortType,
			IssueIDs:    issueIDs,
		})
		if err != nil {
			ctx.ServerError("Issues", err)
			return
		}
	}

	// Get posters.
	for i := range issues {
		// Check read status
		if !ctx.IsSigned {
			issues[i].IsRead = true
		} else if err = issues[i].GetIsRead(ctx.User.ID); err != nil {
			ctx.ServerError("GetIsRead", err)
			return
		}
	}
	ctx.Data["Issues"] = issues

	// Get assignees.
	ctx.Data["Assignees"], err = repo.GetAssignees()
	if err != nil {
		ctx.ServerError("GetAssignees", err)
		return
	}

	labels, err := models.GetLabelsByRepoID(repo.ID, "")
	if err != nil {
		ctx.ServerError("GetLabelsByRepoID", err)
		return
	}
	for _, l := range labels {
		l.LoadSelectedLabelsAfterClick(labelIDs)
	}
	ctx.Data["Labels"] = labels
	ctx.Data["NumLabels"] = len(labels)

	if ctx.QueryInt64("assignee") == 0 {
		assigneeID = 0 // Reset ID to prevent unexpected selection of assignee.
	}

	ctx.Data["IssueStats"] = issueStats
	ctx.Data["SelectLabels"] = com.StrTo(selectLabels).MustInt64()
	ctx.Data["ViewType"] = viewType
	ctx.Data["SortType"] = sortType
	ctx.Data["MilestoneID"] = milestoneID
	ctx.Data["AssigneeID"] = assigneeID
	ctx.Data["IsShowClosed"] = isShowClosed
	ctx.Data["Keyword"] = keyword
	ctx.Data["IsWriter"] = ctx.Repo.IsWriter()
	if isShowClosed {
		ctx.Data["State"] = "closed"
	} else {
		ctx.Data["State"] = "open"
	}
}

// Issues render issues page
func Issues(ctx *context.Context) {
	isPullList := ctx.Params(":type") == "pulls"
	if isPullList {
		MustAllowPulls(ctx)
		if ctx.Written() {
			return
		}
		ctx.Data["Title"] = ctx.Tr("repo.pulls")
		ctx.Data["PageIsPullList"] = true
	} else {
		MustEnableIssues(ctx)
		if ctx.Written() {
			return
		}
		ctx.Data["Title"] = ctx.Tr("repo.issues")
		ctx.Data["PageIsIssueList"] = true
	}

	issues(ctx, ctx.QueryInt64("milestone"), util.OptionalBoolOf(isPullList))

	var err error
	// Get milestones.
	ctx.Data["Milestones"], err = models.GetMilestonesByRepoID(ctx.Repo.Repository.ID)
	if err != nil {
		ctx.ServerError("GetAllRepoMilestones", err)
		return
	}

	ctx.HTML(200, tplIssues)
}

// RetrieveRepoMilestonesAndAssignees find all the milestones and assignees of a repository
func RetrieveRepoMilestonesAndAssignees(ctx *context.Context, repo *models.Repository) {
	var err error
	ctx.Data["OpenMilestones"], err = models.GetMilestones(repo.ID, -1, false, "")
	if err != nil {
		ctx.ServerError("GetMilestones", err)
		return
	}
	ctx.Data["ClosedMilestones"], err = models.GetMilestones(repo.ID, -1, true, "")
	if err != nil {
		ctx.ServerError("GetMilestones", err)
		return
	}

	ctx.Data["Assignees"], err = repo.GetAssignees()
	if err != nil {
		ctx.ServerError("GetAssignees", err)
		return
	}
}

// RetrieveRepoMetas find all the meta information of a repository
func RetrieveRepoMetas(ctx *context.Context, repo *models.Repository) []*models.Label {
	if !ctx.Repo.CanWrite(models.UnitTypeIssues) {
		return nil
	}

	labels, err := models.GetLabelsByRepoID(repo.ID, "")
	if err != nil {
		ctx.ServerError("GetLabelsByRepoID", err)
		return nil
	}
	ctx.Data["Labels"] = labels

	RetrieveRepoMilestonesAndAssignees(ctx, repo)
	if ctx.Written() {
		return nil
	}

	brs, err := ctx.Repo.GitRepo.GetBranches()
	if err != nil {
		ctx.ServerError("GetBranches", err)
		return nil
	}
	ctx.Data["Branches"] = brs

	// Contains true if the user can create issue dependencies
	ctx.Data["CanCreateIssueDependencies"] = ctx.Repo.CanCreateIssueDependencies(ctx.User)

	return labels
}

func getFileContentFromDefaultBranch(ctx *context.Context, filename string) (string, bool) {
	var r io.Reader
	var bytes []byte

	if ctx.Repo.Commit == nil {
		var err error
		ctx.Repo.Commit, err = ctx.Repo.GitRepo.GetBranchCommit(ctx.Repo.Repository.DefaultBranch)
		if err != nil {
			return "", false
		}
	}

	entry, err := ctx.Repo.Commit.GetTreeEntryByPath(filename)
	if err != nil {
		return "", false
	}
	if entry.Blob().Size() >= setting.UI.MaxDisplayFileSize {
		return "", false
	}
	r, err = entry.Blob().Data()
	if err != nil {
		return "", false
	}
	bytes, err = ioutil.ReadAll(r)
	if err != nil {
		return "", false
	}
	return string(bytes), true
}

func setTemplateIfExists(ctx *context.Context, ctxDataKey string, possibleFiles []string) {
	for _, filename := range possibleFiles {
		content, found := getFileContentFromDefaultBranch(ctx, filename)
		if found {
			ctx.Data[ctxDataKey] = content
			return
		}
	}
}

// NewIssue render creating issue page
func NewIssue(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.issues.new")
	ctx.Data["PageIsIssueList"] = true
	ctx.Data["RequireHighlightJS"] = true
	ctx.Data["RequireSimpleMDE"] = true
	ctx.Data["RequireTribute"] = true
	ctx.Data["PullRequestWorkInProgressPrefixes"] = setting.Repository.PullRequest.WorkInProgressPrefixes

	milestoneID := ctx.QueryInt64("milestone")
	milestone, err := models.GetMilestoneByID(milestoneID)
	if err != nil {
		log.Error(4, "GetMilestoneByID: %d: %v", milestoneID, err)
	} else {
		ctx.Data["milestone_id"] = milestoneID
		ctx.Data["Milestone"] = milestone
	}

	setTemplateIfExists(ctx, issueTemplateKey, IssueTemplateCandidates)
	renderAttachmentSettings(ctx)

	RetrieveRepoMetas(ctx, ctx.Repo.Repository)
	if ctx.Written() {
		return
	}

	ctx.HTML(200, tplIssueNew)
}

// ValidateRepoMetas check and returns repository's meta informations
func ValidateRepoMetas(ctx *context.Context, form auth.CreateIssueForm, isPull bool) ([]int64, []int64, int64) {
	var (
		repo = ctx.Repo.Repository
		err  error
	)

	labels := RetrieveRepoMetas(ctx, ctx.Repo.Repository)
	if ctx.Written() {
		return nil, nil, 0
	}

	var labelIDs []int64
	hasSelected := false
	// Check labels.
	if len(form.LabelIDs) > 0 {
		labelIDs, err = base.StringsToInt64s(strings.Split(form.LabelIDs, ","))
		if err != nil {
			return nil, nil, 0
		}
		labelIDMark := base.Int64sToMap(labelIDs)

		for i := range labels {
			if labelIDMark[labels[i].ID] {
				labels[i].IsChecked = true
				hasSelected = true
			}
		}
	}

	ctx.Data["Labels"] = labels
	ctx.Data["HasSelectedLabel"] = hasSelected
	ctx.Data["label_ids"] = form.LabelIDs

	// Check milestone.
	milestoneID := form.MilestoneID
	if milestoneID > 0 {
		ctx.Data["Milestone"], err = repo.GetMilestoneByID(milestoneID)
		if err != nil {
			ctx.ServerError("GetMilestoneByID", err)
			return nil, nil, 0
		}
		ctx.Data["milestone_id"] = milestoneID
	}

	// Check assignees
	var assigneeIDs []int64
	if len(form.AssigneeIDs) > 0 {
		assigneeIDs, err = base.StringsToInt64s(strings.Split(form.AssigneeIDs, ","))
		if err != nil {
			return nil, nil, 0
		}

		// Check if the passed assignees actually exists and has write access to the repo
		for _, aID := range assigneeIDs {
			user, err := models.GetUserByID(aID)
			if err != nil {
				ctx.ServerError("GetUserByID", err)
				return nil, nil, 0
			}

			perm, err := models.GetUserRepoPermission(repo, user)
			if err != nil {
				ctx.ServerError("GetUserRepoPermission", err)
				return nil, nil, 0
			}
			if !perm.CanWriteIssuesOrPulls(isPull) {
				ctx.ServerError("CanWriteIssuesOrPulls", fmt.Errorf("No permission for %s", user.Name))
				return nil, nil, 0
			}
		}
	}

	// Keep the old assignee id thingy for compatibility reasons
	if form.AssigneeID > 0 {
		assigneeIDs = append(assigneeIDs, form.AssigneeID)
	}

	return labelIDs, assigneeIDs, milestoneID
}

// NewIssuePost response for creating new issue
func NewIssuePost(ctx *context.Context, form auth.CreateIssueForm) {
	ctx.Data["Title"] = ctx.Tr("repo.issues.new")
	ctx.Data["PageIsIssueList"] = true
	ctx.Data["RequireHighlightJS"] = true
	ctx.Data["RequireSimpleMDE"] = true
	ctx.Data["ReadOnly"] = false
	ctx.Data["PullRequestWorkInProgressPrefixes"] = setting.Repository.PullRequest.WorkInProgressPrefixes
	renderAttachmentSettings(ctx)

	var (
		repo        = ctx.Repo.Repository
		attachments []string
	)

	labelIDs, assigneeIDs, milestoneID := ValidateRepoMetas(ctx, form, false)
	if ctx.Written() {
		return
	}

	if setting.AttachmentEnabled {
		attachments = form.Files
	}

	if ctx.HasError() {
		ctx.HTML(200, tplIssueNew)
		return
	}

	if util.IsEmptyString(form.Title) {
		ctx.RenderWithErr(ctx.Tr("repo.issues.new.title_empty"), tplIssueNew, form)
		return
	}

	issue := &models.Issue{
		RepoID:      repo.ID,
		Title:       form.Title,
		PosterID:    ctx.User.ID,
		Poster:      ctx.User,
		MilestoneID: milestoneID,
		Content:     form.Content,
		Ref:         form.Ref,
	}
	if err := models.NewIssue(repo, issue, labelIDs, assigneeIDs, attachments); err != nil {
		if models.IsErrUserDoesNotHaveAccessToRepo(err) {
			ctx.Error(400, "UserDoesNotHaveAccessToRepo", err.Error())
			return
		}
		ctx.ServerError("NewIssue", err)
		return
	}

	notification.NotifyNewIssue(issue)

	log.Trace("Issue created: %d/%d", repo.ID, issue.ID)
	ctx.Redirect(ctx.Repo.RepoLink + "/issues/" + com.ToStr(issue.Index))
}

// commentTag returns the CommentTag for a comment in/with the given repo, poster and issue
func commentTag(repo *models.Repository, poster *models.User, issue *models.Issue) (models.CommentTag, error) {
	perm, err := models.GetUserRepoPermission(repo, poster)
	if err != nil {
		return models.CommentTagNone, err
	}
	if perm.IsOwner() {
		return models.CommentTagOwner, nil
	} else if poster.ID == issue.PosterID {
		return models.CommentTagPoster, nil
	} else if perm.CanWrite(models.UnitTypeCode) {
		return models.CommentTagWriter, nil
	}

	return models.CommentTagNone, nil
}

// ViewIssue render issue view page
func ViewIssue(ctx *context.Context) {
	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			ctx.NotFound("GetIssueByIndex", err)
		} else {
			ctx.ServerError("GetIssueByIndex", err)
		}
		return
	}

	// Make sure type and URL matches.
	if ctx.Params(":type") == "issues" && issue.IsPull {
		ctx.Redirect(ctx.Repo.RepoLink + "/pulls/" + com.ToStr(issue.Index))
		return
	} else if ctx.Params(":type") == "pulls" && !issue.IsPull {
		ctx.Redirect(ctx.Repo.RepoLink + "/issues/" + com.ToStr(issue.Index))
		return
	}

	if issue.IsPull {
		MustAllowPulls(ctx)
		if ctx.Written() {
			return
		}
		ctx.Data["PageIsPullList"] = true
		ctx.Data["PageIsPullConversation"] = true
	} else {
		MustEnableIssues(ctx)
		if ctx.Written() {
			return
		}
		ctx.Data["PageIsIssueList"] = true
	}

	ctx.Data["RequireHighlightJS"] = true
	ctx.Data["RequireDropzone"] = true
	ctx.Data["RequireTribute"] = true
	renderAttachmentSettings(ctx)

	err = issue.LoadAttributes()
	if err != nil {
		ctx.ServerError("GetIssueByIndex", err)
		return
	}

	ctx.Data["Title"] = fmt.Sprintf("#%d - %s", issue.Index, issue.Title)

	var iw *models.IssueWatch
	var exists bool
	if ctx.User != nil {
		iw, exists, err = models.GetIssueWatch(ctx.User.ID, issue.ID)
		if err != nil {
			ctx.ServerError("GetIssueWatch", err)
			return
		}
		if !exists {
			iw = &models.IssueWatch{
				UserID:     ctx.User.ID,
				IssueID:    issue.ID,
				IsWatching: models.IsWatching(ctx.User.ID, ctx.Repo.Repository.ID),
			}
		}
	}
	ctx.Data["IssueWatch"] = iw

	issue.RenderedContent = string(markdown.Render([]byte(issue.Content), ctx.Repo.RepoLink,
		ctx.Repo.Repository.ComposeMetas()))

	repo := ctx.Repo.Repository

	// Get more information if it's a pull request.
	if issue.IsPull {
		if issue.PullRequest.HasMerged {
			ctx.Data["DisableStatusChange"] = issue.PullRequest.HasMerged
			PrepareMergedViewPullInfo(ctx, issue)
		} else {
			PrepareViewPullInfo(ctx, issue)
		}
		if ctx.Written() {
			return
		}
	}

	// Metas.
	// Check labels.
	labelIDMark := make(map[int64]bool)
	for i := range issue.Labels {
		labelIDMark[issue.Labels[i].ID] = true
	}
	labels, err := models.GetLabelsByRepoID(repo.ID, "")
	if err != nil {
		ctx.ServerError("GetLabelsByRepoID", err)
		return
	}
	hasSelected := false
	for i := range labels {
		if labelIDMark[labels[i].ID] {
			labels[i].IsChecked = true
			hasSelected = true
		}
	}
	ctx.Data["HasSelectedLabel"] = hasSelected
	ctx.Data["Labels"] = labels

	// Check milestone and assignee.
	if ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull) {
		RetrieveRepoMilestonesAndAssignees(ctx, repo)
		if ctx.Written() {
			return
		}
	}

	if ctx.IsSigned {
		// Update issue-user.
		if err = issue.ReadBy(ctx.User.ID); err != nil {
			ctx.ServerError("ReadBy", err)
			return
		}
	}

	var (
		tag          models.CommentTag
		ok           bool
		marked       = make(map[int64]models.CommentTag)
		comment      *models.Comment
		participants = make([]*models.User, 1, 10)
	)
	if ctx.Repo.Repository.IsTimetrackerEnabled() {
		if ctx.IsSigned {
			// Deal with the stopwatch
			ctx.Data["IsStopwatchRunning"] = models.StopwatchExists(ctx.User.ID, issue.ID)
			if !ctx.Data["IsStopwatchRunning"].(bool) {
				var exists bool
				var sw *models.Stopwatch
				if exists, sw, err = models.HasUserStopwatch(ctx.User.ID); err != nil {
					ctx.ServerError("HasUserStopwatch", err)
					return
				}
				ctx.Data["HasUserStopwatch"] = exists
				if exists {
					// Add warning if the user has already a stopwatch
					var otherIssue *models.Issue
					if otherIssue, err = models.GetIssueByID(sw.IssueID); err != nil {
						ctx.ServerError("GetIssueByID", err)
						return
					}
					if err = otherIssue.LoadRepo(); err != nil {
						ctx.ServerError("LoadRepo", err)
						return
					}
					// Add link to the issue of the already running stopwatch
					ctx.Data["OtherStopwatchURL"] = otherIssue.HTMLURL()
				}
			}
			ctx.Data["CanUseTimetracker"] = ctx.Repo.CanUseTimetracker(issue, ctx.User)
		} else {
			ctx.Data["CanUseTimetracker"] = false
		}
		if ctx.Data["WorkingUsers"], err = models.TotalTimes(models.FindTrackedTimesOptions{IssueID: issue.ID}); err != nil {
			ctx.ServerError("TotalTimes", err)
			return
		}
	}

	// Check if the user can use the dependencies
	ctx.Data["CanCreateIssueDependencies"] = ctx.Repo.CanCreateIssueDependencies(ctx.User)

	// Render comments and and fetch participants.
	participants[0] = issue.Poster
	for _, comment = range issue.Comments {
		if err := comment.LoadPoster(); err != nil {
			ctx.ServerError("LoadPoster", err)
			return
		}

		if comment.Type == models.CommentTypeComment {
			if err := comment.LoadAttachments(); err != nil {
				ctx.ServerError("LoadAttachments", err)
				return
			}

			comment.RenderedContent = string(markdown.Render([]byte(comment.Content), ctx.Repo.RepoLink,
				ctx.Repo.Repository.ComposeMetas()))

			// Check tag.
			tag, ok = marked[comment.PosterID]
			if ok {
				comment.ShowTag = tag
				continue
			}

			comment.ShowTag, err = commentTag(repo, comment.Poster, issue)
			if err != nil {
				ctx.ServerError("commentTag", err)
				return
			}
			marked[comment.PosterID] = comment.ShowTag

			isAdded := false
			for j := range participants {
				if comment.Poster == participants[j] {
					isAdded = true
					break
				}
			}
			if !isAdded && !issue.IsPoster(comment.Poster.ID) {
				participants = append(participants, comment.Poster)
			}
		} else if comment.Type == models.CommentTypeLabel {
			if err = comment.LoadLabel(); err != nil {
				ctx.ServerError("LoadLabel", err)
				return
			}
		} else if comment.Type == models.CommentTypeMilestone {
			if err = comment.LoadMilestone(); err != nil {
				ctx.ServerError("LoadMilestone", err)
				return
			}
			ghostMilestone := &models.Milestone{
				ID:   -1,
				Name: ctx.Tr("repo.issues.deleted_milestone"),
			}
			if comment.OldMilestoneID > 0 && comment.OldMilestone == nil {
				comment.OldMilestone = ghostMilestone
			}
			if comment.MilestoneID > 0 && comment.Milestone == nil {
				comment.Milestone = ghostMilestone
			}
		} else if comment.Type == models.CommentTypeAssignees {
			if err = comment.LoadAssigneeUser(); err != nil {
				ctx.ServerError("LoadAssigneeUser", err)
				return
			}
		} else if comment.Type == models.CommentTypeRemoveDependency || comment.Type == models.CommentTypeAddDependency {
			if err = comment.LoadDepIssueDetails(); err != nil {
				ctx.ServerError("LoadDepIssueDetails", err)
				return
			}
		} else if comment.Type == models.CommentTypeCode || comment.Type == models.CommentTypeReview {
			if err = comment.LoadReview(); err != nil && !models.IsErrReviewNotExist(err) {
				ctx.ServerError("LoadReview", err)
				return
			}
			if comment.Review == nil {
				continue
			}
			if err = comment.Review.LoadAttributes(); err != nil {
				ctx.ServerError("Review.LoadAttributes", err)
				return
			}
			if err = comment.Review.LoadCodeComments(); err != nil {
				ctx.ServerError("Review.LoadCodeComments", err)
				return
			}
		}
	}

	if issue.IsPull {
		pull := issue.PullRequest
		pull.Issue = issue
		canDelete := false

		if ctx.IsSigned {
			if err := pull.GetHeadRepo(); err != nil {
				log.Error(4, "GetHeadRepo: %v", err)
			} else if pull.HeadRepo != nil && pull.HeadBranch != pull.HeadRepo.DefaultBranch {
				perm, err := models.GetUserRepoPermission(pull.HeadRepo, ctx.User)
				if err != nil {
					ctx.ServerError("GetUserRepoPermission", err)
					return
				}
				if perm.CanWrite(models.UnitTypeCode) {
					// Check if branch is not protected
					if protected, err := pull.HeadRepo.IsProtectedBranch(pull.HeadBranch, ctx.User); err != nil {
						log.Error(4, "IsProtectedBranch: %v", err)
					} else if !protected {
						canDelete = true
						ctx.Data["DeleteBranchLink"] = ctx.Repo.RepoLink + "/pulls/" + com.ToStr(issue.Index) + "/cleanup"
					}
				}
			}
		}

		prUnit, err := repo.GetUnit(models.UnitTypePullRequests)
		if err != nil {
			ctx.ServerError("GetUnit", err)
			return
		}
		prConfig := prUnit.PullRequestsConfig()

		ctx.Data["AllowMerge"] = ctx.Repo.CanWrite(models.UnitTypeCode)
		if err := pull.CheckUserAllowedToMerge(ctx.User); err != nil {
			if !models.IsErrNotAllowedToMerge(err) {
				ctx.ServerError("CheckUserAllowedToMerge", err)
				return
			}
			ctx.Data["AllowMerge"] = false
		}

		// Check correct values and select default
		if ms, ok := ctx.Data["MergeStyle"].(models.MergeStyle); !ok ||
			!prConfig.IsMergeStyleAllowed(ms) {
			if prConfig.AllowMerge {
				ctx.Data["MergeStyle"] = models.MergeStyleMerge
			} else if prConfig.AllowRebase {
				ctx.Data["MergeStyle"] = models.MergeStyleRebase
			} else if prConfig.AllowRebaseMerge {
				ctx.Data["MergeStyle"] = models.MergeStyleRebaseMerge
			} else if prConfig.AllowSquash {
				ctx.Data["MergeStyle"] = models.MergeStyleSquash
			} else {
				ctx.Data["MergeStyle"] = ""
			}
		}
		if err = pull.LoadProtectedBranch(); err != nil {
			ctx.ServerError("LoadProtectedBranch", err)
			return
		}
		if pull.ProtectedBranch != nil {
			cnt := pull.ProtectedBranch.GetGrantedApprovalsCount(pull)
			ctx.Data["IsBlockedByApprovals"] = pull.ProtectedBranch.RequiredApprovals > 0 && cnt < pull.ProtectedBranch.RequiredApprovals
			ctx.Data["GrantedApprovals"] = cnt
		}
		ctx.Data["IsPullBranchDeletable"] = canDelete && pull.HeadRepo != nil && git.IsBranchExist(pull.HeadRepo.RepoPath(), pull.HeadBranch)

		ctx.Data["PullReviewersWithType"], err = models.GetReviewersByPullID(issue.ID)
		if err != nil {
			ctx.ServerError("GetReviewersByPullID", err)
			return
		}
	}

	// Get Dependencies
	ctx.Data["BlockedByDependencies"], err = issue.BlockedByDependencies()
	ctx.Data["BlockingDependencies"], err = issue.BlockingDependencies()

	ctx.Data["Participants"] = participants
	ctx.Data["NumParticipants"] = len(participants)
	ctx.Data["Issue"] = issue
	ctx.Data["ReadOnly"] = true
	ctx.Data["SignInLink"] = setting.AppSubURL + "/user/login?redirect_to=" + ctx.Data["Link"].(string)
	ctx.Data["IsIssuePoster"] = ctx.IsSigned && issue.IsPoster(ctx.User.ID)
	ctx.Data["IsIssueWriter"] = ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull)
	ctx.HTML(200, tplIssueView)
}

// GetActionIssue will return the issue which is used in the context.
func GetActionIssue(ctx *context.Context) *models.Issue {
	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		ctx.NotFoundOrServerError("GetIssueByIndex", models.IsErrIssueNotExist, err)
		return nil
	}
	issue.Repo = ctx.Repo.Repository
	checkIssueRights(ctx, issue)
	if ctx.Written() {
		return nil
	}
	if err = issue.LoadAttributes(); err != nil {
		ctx.ServerError("LoadAttributes", nil)
		return nil
	}
	return issue
}

func checkIssueRights(ctx *context.Context, issue *models.Issue) {
	if issue.IsPull && !ctx.Repo.CanRead(models.UnitTypePullRequests) ||
		!issue.IsPull && !ctx.Repo.CanRead(models.UnitTypeIssues) {
		ctx.NotFound("IssueOrPullRequestUnitNotAllowed", nil)
	}
}

func getActionIssues(ctx *context.Context) []*models.Issue {
	commaSeparatedIssueIDs := ctx.Query("issue_ids")
	if len(commaSeparatedIssueIDs) == 0 {
		return nil
	}
	issueIDs := make([]int64, 0, 10)
	for _, stringIssueID := range strings.Split(commaSeparatedIssueIDs, ",") {
		issueID, err := strconv.ParseInt(stringIssueID, 10, 64)
		if err != nil {
			ctx.ServerError("ParseInt", err)
			return nil
		}
		issueIDs = append(issueIDs, issueID)
	}
	issues, err := models.GetIssuesByIDs(issueIDs)
	if err != nil {
		ctx.ServerError("GetIssuesByIDs", err)
		return nil
	}
	// Check access rights for all issues
	issueUnitEnabled := ctx.Repo.CanRead(models.UnitTypeIssues)
	prUnitEnabled := ctx.Repo.CanRead(models.UnitTypePullRequests)
	for _, issue := range issues {
		if issue.IsPull && !prUnitEnabled || !issue.IsPull && !issueUnitEnabled {
			ctx.NotFound("IssueOrPullRequestUnitNotAllowed", nil)
			return nil
		}
		if err = issue.LoadAttributes(); err != nil {
			ctx.ServerError("LoadAttributes", err)
			return nil
		}
	}
	return issues
}

// UpdateIssueTitle change issue's title
func UpdateIssueTitle(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if !ctx.IsSigned || (!issue.IsPoster(ctx.User.ID) && !ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull)) {
		ctx.Error(403)
		return
	}

	title := ctx.QueryTrim("title")
	if len(title) == 0 {
		ctx.Error(204)
		return
	}

	if err := issue.ChangeTitle(ctx.User, title); err != nil {
		ctx.ServerError("ChangeTitle", err)
		return
	}

	ctx.JSON(200, map[string]interface{}{
		"title": issue.Title,
	})
}

// UpdateIssueContent change issue's content
func UpdateIssueContent(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if !ctx.IsSigned || (ctx.User.ID != issue.PosterID && !ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull)) {
		ctx.Error(403)
		return
	}

	content := ctx.Query("content")
	if err := issue.ChangeContent(ctx.User, content); err != nil {
		ctx.ServerError("ChangeContent", err)
		return
	}

	ctx.JSON(200, map[string]interface{}{
		"content": string(markdown.Render([]byte(issue.Content), ctx.Query("context"), ctx.Repo.Repository.ComposeMetas())),
	})
}

// UpdateIssueMilestone change issue's milestone
func UpdateIssueMilestone(ctx *context.Context) {
	issues := getActionIssues(ctx)
	if ctx.Written() {
		return
	}

	milestoneID := ctx.QueryInt64("id")
	for _, issue := range issues {
		oldMilestoneID := issue.MilestoneID
		if oldMilestoneID == milestoneID {
			continue
		}
		issue.MilestoneID = milestoneID
		if err := models.ChangeMilestoneAssign(issue, ctx.User, oldMilestoneID); err != nil {
			ctx.ServerError("ChangeMilestoneAssign", err)
			return
		}
	}

	ctx.JSON(200, map[string]interface{}{
		"ok": true,
	})
}

// UpdateIssueAssignee change issue's assignee
func UpdateIssueAssignee(ctx *context.Context) {
	issues := getActionIssues(ctx)
	if ctx.Written() {
		return
	}

	assigneeID := ctx.QueryInt64("id")
	action := ctx.Query("action")

	for _, issue := range issues {
		switch action {
		case "clear":
			if err := models.DeleteNotPassedAssignee(issue, ctx.User, []*models.User{}); err != nil {
				ctx.ServerError("ClearAssignees", err)
				return
			}
		default:
			if err := issue.ChangeAssignee(ctx.User, assigneeID); err != nil {
				ctx.ServerError("ChangeAssignee", err)
				return
			}
		}
	}
	ctx.JSON(200, map[string]interface{}{
		"ok": true,
	})
}

// UpdateIssueStatus change issue's status
func UpdateIssueStatus(ctx *context.Context) {
	issues := getActionIssues(ctx)
	if ctx.Written() {
		return
	}

	var isClosed bool
	switch action := ctx.Query("action"); action {
	case "open":
		isClosed = false
	case "close":
		isClosed = true
	default:
		log.Warn("Unrecognized action: %s", action)
	}

	if _, err := models.IssueList(issues).LoadRepositories(); err != nil {
		ctx.ServerError("LoadRepositories", err)
		return
	}
	for _, issue := range issues {
		if issue.IsClosed != isClosed {
			if err := issue.ChangeStatus(ctx.User, isClosed); err != nil {
				if models.IsErrDependenciesLeft(err) {
					ctx.JSON(http.StatusPreconditionFailed, map[string]interface{}{
						"error": "cannot close this issue because it still has open dependencies",
					})
					return
				}
				ctx.ServerError("ChangeStatus", err)
				return
			}

			notification.NotifyIssueChangeStatus(ctx.User, issue, isClosed)
		}
	}
	ctx.JSON(200, map[string]interface{}{
		"ok": true,
	})
}

// NewComment create a comment for issue
func NewComment(ctx *context.Context, form auth.CreateCommentForm) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if !ctx.IsSigned || (ctx.User.ID != issue.PosterID && !ctx.Repo.CanReadIssuesOrPulls(issue.IsPull)) {
		ctx.Error(403)
		return
	}

	var attachments []string
	if setting.AttachmentEnabled {
		attachments = form.Files
	}

	if ctx.HasError() {
		ctx.Flash.Error(ctx.Data["ErrorMsg"].(string))
		ctx.Redirect(fmt.Sprintf("%s/issues/%d", ctx.Repo.RepoLink, issue.Index))
		return
	}

	var comment *models.Comment
	defer func() {
		// Check if issue admin/poster changes the status of issue.
		if (ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull) || (ctx.IsSigned && issue.IsPoster(ctx.User.ID))) &&
			(form.Status == "reopen" || form.Status == "close") &&
			!(issue.IsPull && issue.PullRequest.HasMerged) {

			// Duplication and conflict check should apply to reopen pull request.
			var pr *models.PullRequest

			if form.Status == "reopen" && issue.IsPull {
				pull := issue.PullRequest
				pr, err := models.GetUnmergedPullRequest(pull.HeadRepoID, pull.BaseRepoID, pull.HeadBranch, pull.BaseBranch)
				if err != nil {
					if !models.IsErrPullRequestNotExist(err) {
						ctx.ServerError("GetUnmergedPullRequest", err)
						return
					}
				}

				// Regenerate patch and test conflict.
				if pr == nil {
					if err = issue.PullRequest.UpdatePatch(); err != nil {
						ctx.ServerError("UpdatePatch", err)
						return
					}

					issue.PullRequest.AddToTaskQueue()
				}
			}

			if pr != nil {
				ctx.Flash.Info(ctx.Tr("repo.pulls.open_unmerged_pull_exists", pr.Index))
			} else {
				isClosed := form.Status == "close"
				if err := issue.ChangeStatus(ctx.User, isClosed); err != nil {
					log.Error(4, "ChangeStatus: %v", err)

					if models.IsErrDependenciesLeft(err) {
						if issue.IsPull {
							ctx.Flash.Error(ctx.Tr("repo.issues.dependency.pr_close_blocked"))
							ctx.Redirect(fmt.Sprintf("%s/pulls/%d", ctx.Repo.RepoLink, issue.Index), http.StatusSeeOther)
						} else {
							ctx.Flash.Error(ctx.Tr("repo.issues.dependency.issue_close_blocked"))
							ctx.Redirect(fmt.Sprintf("%s/issues/%d", ctx.Repo.RepoLink, issue.Index), http.StatusSeeOther)
						}
						return
					}
				} else {
					log.Trace("Issue [%d] status changed to closed: %v", issue.ID, issue.IsClosed)

					notification.NotifyIssueChangeStatus(ctx.User, issue, isClosed)
				}
			}
		}

		// Redirect to comment hashtag if there is any actual content.
		typeName := "issues"
		if issue.IsPull {
			typeName = "pulls"
		}
		if comment != nil {
			ctx.Redirect(fmt.Sprintf("%s/%s/%d#%s", ctx.Repo.RepoLink, typeName, issue.Index, comment.HashTag()))
		} else {
			ctx.Redirect(fmt.Sprintf("%s/%s/%d", ctx.Repo.RepoLink, typeName, issue.Index))
		}
	}()

	// Fix #321: Allow empty comments, as long as we have attachments.
	if len(form.Content) == 0 && len(attachments) == 0 {
		return
	}

	comment, err := models.CreateIssueComment(ctx.User, ctx.Repo.Repository, issue, form.Content, attachments)
	if err != nil {
		ctx.ServerError("CreateIssueComment", err)
		return
	}

	notification.NotifyCreateIssueComment(ctx.User, ctx.Repo.Repository, issue, comment)

	log.Trace("Comment created: %d/%d/%d", ctx.Repo.Repository.ID, issue.ID, comment.ID)
}

// UpdateCommentContent change comment of issue's content
func UpdateCommentContent(ctx *context.Context) {
	comment, err := models.GetCommentByID(ctx.ParamsInt64(":id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetCommentByID", models.IsErrCommentNotExist, err)
		return
	}

	if err := comment.LoadIssue(); err != nil {
		ctx.NotFoundOrServerError("LoadIssue", models.IsErrIssueNotExist, err)
		return
	}

	if !ctx.IsSigned || (ctx.User.ID != comment.PosterID && !ctx.Repo.CanWriteIssuesOrPulls(comment.Issue.IsPull)) {
		ctx.Error(403)
		return
	} else if comment.Type != models.CommentTypeComment && comment.Type != models.CommentTypeCode {
		ctx.Error(204)
		return
	}

	oldContent := comment.Content
	comment.Content = ctx.Query("content")
	if len(comment.Content) == 0 {
		ctx.JSON(200, map[string]interface{}{
			"content": "",
		})
		return
	}
	if err = models.UpdateComment(ctx.User, comment, oldContent); err != nil {
		ctx.ServerError("UpdateComment", err)
		return
	}

	notification.NotifyUpdateComment(ctx.User, comment, oldContent)

	ctx.JSON(200, map[string]interface{}{
		"content": string(markdown.Render([]byte(comment.Content), ctx.Query("context"), ctx.Repo.Repository.ComposeMetas())),
	})
}

// DeleteComment delete comment of issue
func DeleteComment(ctx *context.Context) {
	comment, err := models.GetCommentByID(ctx.ParamsInt64(":id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetCommentByID", models.IsErrCommentNotExist, err)
		return
	}

	if err := comment.LoadIssue(); err != nil {
		ctx.NotFoundOrServerError("LoadIssue", models.IsErrIssueNotExist, err)
		return
	}

	if !ctx.IsSigned || (ctx.User.ID != comment.PosterID && !ctx.Repo.CanWriteIssuesOrPulls(comment.Issue.IsPull)) {
		ctx.Error(403)
		return
	} else if comment.Type != models.CommentTypeComment && comment.Type != models.CommentTypeCode {
		ctx.Error(204)
		return
	}

	if err = models.DeleteComment(ctx.User, comment); err != nil {
		ctx.ServerError("DeleteCommentByID", err)
		return
	}

	notification.NotifyDeleteComment(ctx.User, comment)

	ctx.Status(200)
}

// ChangeIssueReaction create a reaction for issue
func ChangeIssueReaction(ctx *context.Context, form auth.ReactionForm) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if !ctx.IsSigned || (ctx.User.ID != issue.PosterID && !ctx.Repo.CanReadIssuesOrPulls(issue.IsPull)) {
		ctx.Error(403)
		return
	}

	if ctx.HasError() {
		ctx.ServerError("ChangeIssueReaction", errors.New(ctx.GetErrMsg()))
		return
	}

	switch ctx.Params(":action") {
	case "react":
		reaction, err := models.CreateIssueReaction(ctx.User, issue, form.Content)
		if err != nil {
			log.Info("CreateIssueReaction: %s", err)
			break
		}
		// Reload new reactions
		issue.Reactions = nil
		if err = issue.LoadAttributes(); err != nil {
			log.Info("issue.LoadAttributes: %s", err)
			break
		}

		log.Trace("Reaction for issue created: %d/%d/%d", ctx.Repo.Repository.ID, issue.ID, reaction.ID)
	case "unreact":
		if err := models.DeleteIssueReaction(ctx.User, issue, form.Content); err != nil {
			ctx.ServerError("DeleteIssueReaction", err)
			return
		}

		// Reload new reactions
		issue.Reactions = nil
		if err := issue.LoadAttributes(); err != nil {
			log.Info("issue.LoadAttributes: %s", err)
			break
		}

		log.Trace("Reaction for issue removed: %d/%d", ctx.Repo.Repository.ID, issue.ID)
	default:
		ctx.NotFound(fmt.Sprintf("Unknown action %s", ctx.Params(":action")), nil)
		return
	}

	if len(issue.Reactions) == 0 {
		ctx.JSON(200, map[string]interface{}{
			"empty": true,
			"html":  "",
		})
		return
	}

	html, err := ctx.HTMLString(string(tplReactions), map[string]interface{}{
		"ctx":       ctx.Data,
		"ActionURL": fmt.Sprintf("%s/issues/%d/reactions", ctx.Repo.RepoLink, issue.Index),
		"Reactions": issue.Reactions.GroupByType(),
	})
	if err != nil {
		ctx.ServerError("ChangeIssueReaction.HTMLString", err)
		return
	}
	ctx.JSON(200, map[string]interface{}{
		"html": html,
	})
}

// ChangeCommentReaction create a reaction for comment
func ChangeCommentReaction(ctx *context.Context, form auth.ReactionForm) {
	comment, err := models.GetCommentByID(ctx.ParamsInt64(":id"))
	if err != nil {
		ctx.NotFoundOrServerError("GetCommentByID", models.IsErrCommentNotExist, err)
		return
	}

	if err := comment.LoadIssue(); err != nil {
		ctx.NotFoundOrServerError("LoadIssue", models.IsErrIssueNotExist, err)
		return
	}

	if !ctx.IsSigned || (ctx.User.ID != comment.PosterID && !ctx.Repo.CanReadIssuesOrPulls(comment.Issue.IsPull)) {
		ctx.Error(403)
		return
	} else if comment.Type != models.CommentTypeComment && comment.Type != models.CommentTypeCode {
		ctx.Error(204)
		return
	}

	switch ctx.Params(":action") {
	case "react":
		reaction, err := models.CreateCommentReaction(ctx.User, comment.Issue, comment, form.Content)
		if err != nil {
			log.Info("CreateCommentReaction: %s", err)
			break
		}
		// Reload new reactions
		comment.Reactions = nil
		if err = comment.LoadReactions(); err != nil {
			log.Info("comment.LoadReactions: %s", err)
			break
		}

		log.Trace("Reaction for comment created: %d/%d/%d/%d", ctx.Repo.Repository.ID, comment.Issue.ID, comment.ID, reaction.ID)
	case "unreact":
		if err := models.DeleteCommentReaction(ctx.User, comment.Issue, comment, form.Content); err != nil {
			ctx.ServerError("DeleteCommentReaction", err)
			return
		}

		// Reload new reactions
		comment.Reactions = nil
		if err = comment.LoadReactions(); err != nil {
			log.Info("comment.LoadReactions: %s", err)
			break
		}

		log.Trace("Reaction for comment removed: %d/%d/%d", ctx.Repo.Repository.ID, comment.Issue.ID, comment.ID)
	default:
		ctx.NotFound(fmt.Sprintf("Unknown action %s", ctx.Params(":action")), nil)
		return
	}

	if len(comment.Reactions) == 0 {
		ctx.JSON(200, map[string]interface{}{
			"empty": true,
			"html":  "",
		})
		return
	}

	html, err := ctx.HTMLString(string(tplReactions), map[string]interface{}{
		"ctx":       ctx.Data,
		"ActionURL": fmt.Sprintf("%s/comments/%d/reactions", ctx.Repo.RepoLink, comment.ID),
		"Reactions": comment.Reactions.GroupByType(),
	})
	if err != nil {
		ctx.ServerError("ChangeCommentReaction.HTMLString", err)
		return
	}
	ctx.JSON(200, map[string]interface{}{
		"html": html,
	})
}

// UpdateIssuePriority updates an issue priority.
func UpdateIssuePriority(ctx *context.Context, form auth.EditPriorityForm) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	issue.Priority = form.Priority

	if err := models.UpdateIssuePriority(issue); err != nil {
		ctx.Error(http.StatusInternalServerError)

		return
	}

	ctx.JSON(http.StatusOK, tplIssues)
}

// PinIssue pin an issue by index
func PinIssue(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if err := models.PinIssue(issue, ctx.User); err != nil {
		ctx.Error(http.StatusInternalServerError)

		return
	}

	ctx.JSON(http.StatusOK, tplIssues)
}

// UnpinIssue pin an issue by index
func UnpinIssue(ctx *context.Context) {
	UpdateIssuePriority(ctx, auth.EditPriorityForm{Priority: models.PriorityDefault})
}
