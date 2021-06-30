// Copyright 2017 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"crypto/sha1"
	"fmt"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

// CommitStatus holds a single Status of a single Commit
type CommitStatus struct {
	ID          int64                 `xorm:"pk autoincr"`
	Index       int64                 `xorm:"INDEX UNIQUE(repo_sha_index)"`
	RepoID      int64                 `xorm:"INDEX UNIQUE(repo_sha_index)"`
	Repo        *Repository           `xorm:"-"`
	State       api.CommitStatusState `xorm:"VARCHAR(7) NOT NULL"`
	SHA         string                `xorm:"VARCHAR(64) NOT NULL INDEX UNIQUE(repo_sha_index)"`
	TargetURL   string                `xorm:"TEXT"`
	Description string                `xorm:"TEXT"`
	ContextHash string                `xorm:"char(40) index"`
	Context     string                `xorm:"TEXT"`
	Creator     *User                 `xorm:"-"`
	CreatorID   int64

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

func (status *CommitStatus) loadAttributes(e Engine) (err error) {
	if status.Repo == nil {
		status.Repo, err = getRepositoryByID(e, status.RepoID)
		if err != nil {
			return fmt.Errorf("getRepositoryByID [%d]: %v", status.RepoID, err)
		}
	}
	if status.Creator == nil && status.CreatorID > 0 {
		status.Creator, err = getUserByID(e, status.CreatorID)
		if err != nil {
			return fmt.Errorf("getUserByID [%d]: %v", status.CreatorID, err)
		}
	}
	return nil
}

// APIURL returns the absolute APIURL to this commit-status.
func (status *CommitStatus) APIURL() string {
	_ = status.loadAttributes(x)
	return fmt.Sprintf("%sapi/v1/repos/%s/statuses/%s",
		setting.AppURL, status.Repo.FullName(), status.SHA)
}

// CalcCommitStatus returns commit status state via some status, the commit statues should order by id desc
func CalcCommitStatus(statuses []*CommitStatus) *CommitStatus {
	var lastStatus *CommitStatus
	var state api.CommitStatusState
	for _, status := range statuses {
		if status.State.NoBetterThan(state) {
			state = status.State
			lastStatus = status
		}
	}
	if lastStatus == nil {
		if len(statuses) > 0 {
			lastStatus = statuses[0]
		} else {
			lastStatus = &CommitStatus{}
		}
	}
	return lastStatus
}

// CommitStatusOptions holds the options for query commit statuses
type CommitStatusOptions struct {
	ListOptions
	State    string
	SortType string
}

// GetCommitStatuses returns all statuses for a given commit.
func GetCommitStatuses(repo *Repository, sha string, opts *CommitStatusOptions) ([]*CommitStatus, int64, error) {
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.PageSize <= 0 {
		opts.Page = ItemsPerPage
	}

	countSession := listCommitStatusesStatement(repo, sha, opts)
	countSession = opts.setSessionPagination(countSession)
	maxResults, err := countSession.Count(new(CommitStatus))
	if err != nil {
		log.Error("Count PRs: %v", err)
		return nil, maxResults, err
	}

	statuses := make([]*CommitStatus, 0, opts.PageSize)
	findSession := listCommitStatusesStatement(repo, sha, opts)
	findSession = opts.setSessionPagination(findSession)
	sortCommitStatusesSession(findSession, opts.SortType)
	return statuses, maxResults, findSession.Find(&statuses)
}

func listCommitStatusesStatement(repo *Repository, sha string, opts *CommitStatusOptions) *xorm.Session {
	sess := x.Where("repo_id = ?", repo.ID).And("sha = ?", sha)
	switch opts.State {
	case "pending", "success", "error", "failure", "warning":
		sess.And("state = ?", opts.State)
	}
	return sess
}

func sortCommitStatusesSession(sess *xorm.Session, sortType string) {
	switch sortType {
	case "oldest":
		sess.Asc("created_unix")
	case "recentupdate":
		sess.Desc("updated_unix")
	case "leastupdate":
		sess.Asc("updated_unix")
	case "leastindex":
		sess.Desc("index")
	case "highestindex":
		sess.Asc("index")
	default:
		sess.Desc("created_unix")
	}
}

// GetLatestCommitStatus returns all statuses with a unique context for a given commit.
func GetLatestCommitStatus(repoID int64, sha string, listOptions ListOptions) ([]*CommitStatus, error) {
	return getLatestCommitStatus(x, repoID, sha, listOptions)
}

func getLatestCommitStatus(e Engine, repoID int64, sha string, listOptions ListOptions) ([]*CommitStatus, error) {
	ids := make([]int64, 0, 10)
	sess := e.Table(&CommitStatus{}).
		Where("repo_id = ?", repoID).And("sha = ?", sha).
		Select("max( id ) as id").
		GroupBy("context_hash").OrderBy("max( id ) desc")

	sess = listOptions.setSessionPagination(sess)

	err := sess.Find(&ids)
	if err != nil {
		return nil, err
	}
	statuses := make([]*CommitStatus, 0, len(ids))
	if len(ids) == 0 {
		return statuses, nil
	}
	return statuses, x.In("id", ids).Find(&statuses)
}

// FindRepoRecentCommitStatusContexts returns repository's recent commit status contexts
func FindRepoRecentCommitStatusContexts(repoID int64, before time.Duration) ([]string, error) {
	start := timeutil.TimeStampNow().AddDuration(-before)
	ids := make([]int64, 0, 10)
	if err := x.Table("commit_status").
		Where("repo_id = ?", repoID).
		And("updated_unix >= ?", start).
		Select("max( id ) as id").
		GroupBy("context_hash").OrderBy("max( id ) desc").
		Find(&ids); err != nil {
		return nil, err
	}

	contexts := make([]string, 0, len(ids))
	if len(ids) == 0 {
		return contexts, nil
	}
	return contexts, x.Select("context").Table("commit_status").In("id", ids).Find(&contexts)
}

// NewCommitStatusOptions holds options for creating a CommitStatus
type NewCommitStatusOptions struct {
	Repo         *Repository
	Creator      *User
	SHA          string
	CommitStatus *CommitStatus
}

// NewCommitStatus save commit statuses into database
func NewCommitStatus(opts NewCommitStatusOptions) error {
	if opts.Repo == nil {
		return fmt.Errorf("NewCommitStatus[nil, %s]: no repository specified", opts.SHA)
	}

	repoPath := opts.Repo.RepoPath()
	if opts.Creator == nil {
		return fmt.Errorf("NewCommitStatus[%s, %s]: no user specified", repoPath, opts.SHA)
	}

	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return fmt.Errorf("NewCommitStatus[repo_id: %d, user_id: %d, sha: %s]: %v", opts.Repo.ID, opts.Creator.ID, opts.SHA, err)
	}

	opts.CommitStatus.Description = strings.TrimSpace(opts.CommitStatus.Description)
	opts.CommitStatus.Context = strings.TrimSpace(opts.CommitStatus.Context)
	opts.CommitStatus.TargetURL = strings.TrimSpace(opts.CommitStatus.TargetURL)
	opts.CommitStatus.SHA = opts.SHA
	opts.CommitStatus.CreatorID = opts.Creator.ID
	opts.CommitStatus.RepoID = opts.Repo.ID

	// Get the next Status Index
	var nextIndex int64
	lastCommitStatus := &CommitStatus{
		SHA:    opts.SHA,
		RepoID: opts.Repo.ID,
	}
	has, err := sess.Desc("index").Limit(1).Get(lastCommitStatus)
	if err != nil {
		if err := sess.Rollback(); err != nil {
			log.Error("NewCommitStatus: sess.Rollback: %v", err)
		}
		return fmt.Errorf("NewCommitStatus[%s, %s]: %v", repoPath, opts.SHA, err)
	}
	if has {
		log.Debug("NewCommitStatus[%s, %s]: found", repoPath, opts.SHA)
		nextIndex = lastCommitStatus.Index
	}
	opts.CommitStatus.Index = nextIndex + 1
	log.Debug("NewCommitStatus[%s, %s]: %d", repoPath, opts.SHA, opts.CommitStatus.Index)

	opts.CommitStatus.ContextHash = hashCommitStatusContext(opts.CommitStatus.Context)

	// Insert new CommitStatus
	if _, err = sess.Insert(opts.CommitStatus); err != nil {
		if err := sess.Rollback(); err != nil {
			log.Error("Insert CommitStatus: sess.Rollback: %v", err)
		}
		return fmt.Errorf("Insert CommitStatus[%s, %s]: %v", repoPath, opts.SHA, err)
	}

	return sess.Commit()
}

// SignCommitWithStatuses represents a commit with validation of signature and status state.
type SignCommitWithStatuses struct {
	Status   *CommitStatus
	Statuses []*CommitStatus
	*SignCommit
}

// ParseCommitsWithStatus checks commits latest statuses and calculates its worst status state
func ParseCommitsWithStatus(oldCommits []*SignCommit, repo *Repository) []*SignCommitWithStatuses {
	newCommits := make([]*SignCommitWithStatuses, 0, len(oldCommits))

	for _, c := range oldCommits {
		commit := &SignCommitWithStatuses{
			SignCommit: c,
		}
		statuses, err := GetLatestCommitStatus(repo.ID, commit.ID.String(), ListOptions{})
		if err != nil {
			log.Error("GetLatestCommitStatus: %v", err)
		} else {
			commit.Statuses = statuses
			commit.Status = CalcCommitStatus(statuses)
		}

		newCommits = append(newCommits, commit)
	}
	return newCommits
}

// hashCommitStatusContext hash context
func hashCommitStatusContext(context string) string {
	return fmt.Sprintf("%x", sha1.Sum([]byte(context)))
}
