// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"context"
	"fmt"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"github.com/gobwas/glob"
)

// ProtectedBranch struct
type ProtectedBranch struct {
	ID                            int64  `xorm:"pk autoincr"`
	RepoID                        int64  `xorm:"UNIQUE(s)"`
	BranchName                    string `xorm:"UNIQUE(s)"`
	CanPush                       bool   `xorm:"NOT NULL DEFAULT false"`
	EnableWhitelist               bool
	WhitelistUserIDs              []int64  `xorm:"JSON TEXT"`
	WhitelistTeamIDs              []int64  `xorm:"JSON TEXT"`
	EnableMergeWhitelist          bool     `xorm:"NOT NULL DEFAULT false"`
	WhitelistDeployKeys           bool     `xorm:"NOT NULL DEFAULT false"`
	MergeWhitelistUserIDs         []int64  `xorm:"JSON TEXT"`
	MergeWhitelistTeamIDs         []int64  `xorm:"JSON TEXT"`
	EnableStatusCheck             bool     `xorm:"NOT NULL DEFAULT false"`
	StatusCheckContexts           []string `xorm:"JSON TEXT"`
	EnableApprovalsWhitelist      bool     `xorm:"NOT NULL DEFAULT false"`
	ApprovalsWhitelistUserIDs     []int64  `xorm:"JSON TEXT"`
	ApprovalsWhitelistTeamIDs     []int64  `xorm:"JSON TEXT"`
	RequiredApprovals             int64    `xorm:"NOT NULL DEFAULT 0"`
	BlockOnRejectedReviews        bool     `xorm:"NOT NULL DEFAULT false"`
	BlockOnOfficialReviewRequests bool     `xorm:"NOT NULL DEFAULT false"`
	BlockOnOutdatedBranch         bool     `xorm:"NOT NULL DEFAULT false"`
	DismissStaleApprovals         bool     `xorm:"NOT NULL DEFAULT false"`
	RequireSignedCommits          bool     `xorm:"NOT NULL DEFAULT false"`
	ProtectedFilePatterns         string   `xorm:"TEXT"`
	UnprotectedFilePatterns       string   `xorm:"TEXT"`

	CreatedUnix timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
}

// IsProtected returns if the branch is protected
func (protectBranch *ProtectedBranch) IsProtected() bool {
	return protectBranch.ID > 0
}

// CanUserPush returns if some user could push to this protected branch
func (protectBranch *ProtectedBranch) CanUserPush(userID int64) bool {
	if !protectBranch.CanPush {
		return false
	}

	if !protectBranch.EnableWhitelist {
		if user, err := GetUserByID(userID); err != nil {
			log.Error("GetUserByID: %v", err)
			return false
		} else if repo, err := GetRepositoryByID(protectBranch.RepoID); err != nil {
			log.Error("GetRepositoryByID: %v", err)
			return false
		} else if writeAccess, err := HasAccessUnit(user, repo, UnitTypeCode, AccessModeWrite); err != nil {
			log.Error("HasAccessUnit: %v", err)
			return false
		} else {
			return writeAccess
		}
	}

	if base.Int64sContains(protectBranch.WhitelistUserIDs, userID) {
		return true
	}

	if len(protectBranch.WhitelistTeamIDs) == 0 {
		return false
	}

	in, err := IsUserInTeams(userID, protectBranch.WhitelistTeamIDs)
	if err != nil {
		log.Error("IsUserInTeams: %v", err)
		return false
	}
	return in
}

// IsUserMergeWhitelisted checks if some user is whitelisted to merge to this branch
func (protectBranch *ProtectedBranch) IsUserMergeWhitelisted(userID int64, permissionInRepo Permission) bool {
	if !protectBranch.EnableMergeWhitelist {
		// Then we need to fall back on whether the user has write permission
		return permissionInRepo.CanWrite(UnitTypeCode)
	}

	if base.Int64sContains(protectBranch.MergeWhitelistUserIDs, userID) {
		return true
	}

	if len(protectBranch.MergeWhitelistTeamIDs) == 0 {
		return false
	}

	in, err := IsUserInTeams(userID, protectBranch.MergeWhitelistTeamIDs)
	if err != nil {
		log.Error("IsUserInTeams: %v", err)
		return false
	}
	return in
}

// IsUserOfficialReviewer check if user is official reviewer for the branch (counts towards required approvals)
func (protectBranch *ProtectedBranch) IsUserOfficialReviewer(user *User) (bool, error) {
	return protectBranch.isUserOfficialReviewer(x, user)
}

func (protectBranch *ProtectedBranch) isUserOfficialReviewer(e Engine, user *User) (bool, error) {
	repo, err := getRepositoryByID(e, protectBranch.RepoID)
	if err != nil {
		return false, err
	}

	if !protectBranch.EnableApprovalsWhitelist {
		// Anyone with write access is considered official reviewer
		writeAccess, err := hasAccessUnit(e, user, repo, UnitTypeCode, AccessModeWrite)
		if err != nil {
			return false, err
		}
		return writeAccess, nil
	}

	if base.Int64sContains(protectBranch.ApprovalsWhitelistUserIDs, user.ID) {
		return true, nil
	}

	inTeam, err := isUserInTeams(e, user.ID, protectBranch.ApprovalsWhitelistTeamIDs)
	if err != nil {
		return false, err
	}

	return inTeam, nil
}

// HasEnoughApprovals returns true if pr has enough granted approvals.
func (protectBranch *ProtectedBranch) HasEnoughApprovals(pr *PullRequest) bool {
	if protectBranch.RequiredApprovals == 0 {
		return true
	}
	return protectBranch.GetGrantedApprovalsCount(pr) >= protectBranch.RequiredApprovals
}

// GetGrantedApprovalsCount returns the number of granted approvals for pr. A granted approval must be authored by a user in an approval whitelist.
func (protectBranch *ProtectedBranch) GetGrantedApprovalsCount(pr *PullRequest) int64 {
	sess := x.Where("issue_id = ?", pr.IssueID).
		And("type = ?", ReviewTypeApprove).
		And("official = ?", true).
		And("dismissed = ?", false)
	if protectBranch.DismissStaleApprovals {
		sess = sess.And("stale = ?", false)
	}
	approvals, err := sess.Count(new(Review))
	if err != nil {
		log.Error("GetGrantedApprovalsCount: %v", err)
		return 0
	}

	return approvals
}

// MergeBlockedByRejectedReview returns true if merge is blocked by rejected reviews
func (protectBranch *ProtectedBranch) MergeBlockedByRejectedReview(pr *PullRequest) bool {
	if !protectBranch.BlockOnRejectedReviews {
		return false
	}
	rejectExist, err := x.Where("issue_id = ?", pr.IssueID).
		And("type = ?", ReviewTypeReject).
		And("official = ?", true).
		And("dismissed = ?", false).
		Exist(new(Review))
	if err != nil {
		log.Error("MergeBlockedByRejectedReview: %v", err)
		return true
	}

	return rejectExist
}

// MergeBlockedByOfficialReviewRequests block merge because of some review request to official reviewer
// of from official review
func (protectBranch *ProtectedBranch) MergeBlockedByOfficialReviewRequests(pr *PullRequest) bool {
	if !protectBranch.BlockOnOfficialReviewRequests {
		return false
	}
	has, err := x.Where("issue_id = ?", pr.IssueID).
		And("type = ?", ReviewTypeRequest).
		And("official = ?", true).
		Exist(new(Review))
	if err != nil {
		log.Error("MergeBlockedByOfficialReviewRequests: %v", err)
		return true
	}

	return has
}

// MergeBlockedByOutdatedBranch returns true if merge is blocked by an outdated head branch
func (protectBranch *ProtectedBranch) MergeBlockedByOutdatedBranch(pr *PullRequest) bool {
	return protectBranch.BlockOnOutdatedBranch && pr.CommitsBehind > 0
}

// GetProtectedFilePatterns parses a semicolon separated list of protected file patterns and returns a glob.Glob slice
func (protectBranch *ProtectedBranch) GetProtectedFilePatterns() []glob.Glob {
	return getFilePatterns(protectBranch.ProtectedFilePatterns)
}

// GetUnprotectedFilePatterns parses a semicolon separated list of unprotected file patterns and returns a glob.Glob slice
func (protectBranch *ProtectedBranch) GetUnprotectedFilePatterns() []glob.Glob {
	return getFilePatterns(protectBranch.UnprotectedFilePatterns)
}

func getFilePatterns(filePatterns string) []glob.Glob {
	extarr := make([]glob.Glob, 0, 10)
	for _, expr := range strings.Split(strings.ToLower(filePatterns), ";") {
		expr = strings.TrimSpace(expr)
		if expr != "" {
			if g, err := glob.Compile(expr, '.', '/'); err != nil {
				log.Info("Invalid glob expression '%s' (skipped): %v", expr, err)
			} else {
				extarr = append(extarr, g)
			}
		}
	}
	return extarr
}

// MergeBlockedByProtectedFiles returns true if merge is blocked by protected files change
func (protectBranch *ProtectedBranch) MergeBlockedByProtectedFiles(pr *PullRequest) bool {
	glob := protectBranch.GetProtectedFilePatterns()
	if len(glob) == 0 {
		return false
	}

	return len(pr.ChangedProtectedFiles) > 0
}

// IsProtectedFile return if path is protected
func (protectBranch *ProtectedBranch) IsProtectedFile(patterns []glob.Glob, path string) bool {
	if len(patterns) == 0 {
		patterns = protectBranch.GetProtectedFilePatterns()
		if len(patterns) == 0 {
			return false
		}
	}

	lpath := strings.ToLower(strings.TrimSpace(path))

	r := false
	for _, pat := range patterns {
		if pat.Match(lpath) {
			r = true
			break
		}
	}

	return r
}

// IsUnprotectedFile return if path is unprotected
func (protectBranch *ProtectedBranch) IsUnprotectedFile(patterns []glob.Glob, path string) bool {
	if len(patterns) == 0 {
		patterns = protectBranch.GetUnprotectedFilePatterns()
		if len(patterns) == 0 {
			return false
		}
	}

	lpath := strings.ToLower(strings.TrimSpace(path))

	r := false
	for _, pat := range patterns {
		if pat.Match(lpath) {
			r = true
			break
		}
	}

	return r
}

// GetProtectedBranchBy getting protected branch by ID/Name
func GetProtectedBranchBy(repoID int64, branchName string) (*ProtectedBranch, error) {
	return getProtectedBranchBy(x, repoID, branchName)
}

func getProtectedBranchBy(e Engine, repoID int64, branchName string) (*ProtectedBranch, error) {
	rel := &ProtectedBranch{RepoID: repoID, BranchName: branchName}
	has, err := e.Get(rel)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, nil
	}
	return rel, nil
}

// WhitelistOptions represent all sorts of whitelists used for protected branches
type WhitelistOptions struct {
	UserIDs []int64
	TeamIDs []int64

	MergeUserIDs []int64
	MergeTeamIDs []int64

	ApprovalsUserIDs []int64
	ApprovalsTeamIDs []int64
}

// UpdateProtectBranch saves branch protection options of repository.
// If ID is 0, it creates a new record. Otherwise, updates existing record.
// This function also performs check if whitelist user and team's IDs have been changed
// to avoid unnecessary whitelist delete and regenerate.
func UpdateProtectBranch(repo *Repository, protectBranch *ProtectedBranch, opts WhitelistOptions) (err error) {
	if err = repo.GetOwner(); err != nil {
		return fmt.Errorf("GetOwner: %v", err)
	}

	whitelist, err := updateUserWhitelist(repo, protectBranch.WhitelistUserIDs, opts.UserIDs)
	if err != nil {
		return err
	}
	protectBranch.WhitelistUserIDs = whitelist

	whitelist, err = updateUserWhitelist(repo, protectBranch.MergeWhitelistUserIDs, opts.MergeUserIDs)
	if err != nil {
		return err
	}
	protectBranch.MergeWhitelistUserIDs = whitelist

	whitelist, err = updateApprovalWhitelist(repo, protectBranch.ApprovalsWhitelistUserIDs, opts.ApprovalsUserIDs)
	if err != nil {
		return err
	}
	protectBranch.ApprovalsWhitelistUserIDs = whitelist

	// if the repo is in an organization
	whitelist, err = updateTeamWhitelist(repo, protectBranch.WhitelistTeamIDs, opts.TeamIDs)
	if err != nil {
		return err
	}
	protectBranch.WhitelistTeamIDs = whitelist

	whitelist, err = updateTeamWhitelist(repo, protectBranch.MergeWhitelistTeamIDs, opts.MergeTeamIDs)
	if err != nil {
		return err
	}
	protectBranch.MergeWhitelistTeamIDs = whitelist

	whitelist, err = updateTeamWhitelist(repo, protectBranch.ApprovalsWhitelistTeamIDs, opts.ApprovalsTeamIDs)
	if err != nil {
		return err
	}
	protectBranch.ApprovalsWhitelistTeamIDs = whitelist

	// Make sure protectBranch.ID is not 0 for whitelists
	if protectBranch.ID == 0 {
		if _, err = x.Insert(protectBranch); err != nil {
			return fmt.Errorf("Insert: %v", err)
		}
		return nil
	}

	if _, err = x.ID(protectBranch.ID).AllCols().Update(protectBranch); err != nil {
		return fmt.Errorf("Update: %v", err)
	}

	return nil
}

// GetProtectedBranches get all protected branches
func (repo *Repository) GetProtectedBranches() ([]*ProtectedBranch, error) {
	protectedBranches := make([]*ProtectedBranch, 0)
	return protectedBranches, x.Find(&protectedBranches, &ProtectedBranch{RepoID: repo.ID})
}

// GetBranchProtection get the branch protection of a branch
func (repo *Repository) GetBranchProtection(branchName string) (*ProtectedBranch, error) {
	return GetProtectedBranchBy(repo.ID, branchName)
}

// IsProtectedBranch checks if branch is protected
func (repo *Repository) IsProtectedBranch(branchName string) (bool, error) {
	protectedBranch := &ProtectedBranch{
		RepoID:     repo.ID,
		BranchName: branchName,
	}

	has, err := x.Exist(protectedBranch)
	if err != nil {
		return true, err
	}
	return has, nil
}

// updateApprovalWhitelist checks whether the user whitelist changed and returns a whitelist with
// the users from newWhitelist which have explicit read or write access to the repo.
func updateApprovalWhitelist(repo *Repository, currentWhitelist, newWhitelist []int64) (whitelist []int64, err error) {
	hasUsersChanged := !util.IsSliceInt64Eq(currentWhitelist, newWhitelist)
	if !hasUsersChanged {
		return currentWhitelist, nil
	}

	whitelist = make([]int64, 0, len(newWhitelist))
	for _, userID := range newWhitelist {
		if reader, err := repo.IsReader(userID); err != nil {
			return nil, err
		} else if !reader {
			continue
		}
		whitelist = append(whitelist, userID)
	}

	return
}

// updateUserWhitelist checks whether the user whitelist changed and returns a whitelist with
// the users from newWhitelist which have write access to the repo.
func updateUserWhitelist(repo *Repository, currentWhitelist, newWhitelist []int64) (whitelist []int64, err error) {
	hasUsersChanged := !util.IsSliceInt64Eq(currentWhitelist, newWhitelist)
	if !hasUsersChanged {
		return currentWhitelist, nil
	}

	whitelist = make([]int64, 0, len(newWhitelist))
	for _, userID := range newWhitelist {
		user, err := GetUserByID(userID)
		if err != nil {
			return nil, fmt.Errorf("GetUserByID [user_id: %d, repo_id: %d]: %v", userID, repo.ID, err)
		}
		perm, err := GetUserRepoPermission(repo, user)
		if err != nil {
			return nil, fmt.Errorf("GetUserRepoPermission [user_id: %d, repo_id: %d]: %v", userID, repo.ID, err)
		}

		if !perm.CanWrite(UnitTypeCode) {
			continue // Drop invalid user ID
		}

		whitelist = append(whitelist, userID)
	}

	return
}

// updateTeamWhitelist checks whether the team whitelist changed and returns a whitelist with
// the teams from newWhitelist which have write access to the repo.
func updateTeamWhitelist(repo *Repository, currentWhitelist, newWhitelist []int64) (whitelist []int64, err error) {
	hasTeamsChanged := !util.IsSliceInt64Eq(currentWhitelist, newWhitelist)
	if !hasTeamsChanged {
		return currentWhitelist, nil
	}

	teams, err := GetTeamsWithAccessToRepo(repo.OwnerID, repo.ID, AccessModeRead)
	if err != nil {
		return nil, fmt.Errorf("GetTeamsWithAccessToRepo [org_id: %d, repo_id: %d]: %v", repo.OwnerID, repo.ID, err)
	}

	whitelist = make([]int64, 0, len(teams))
	for i := range teams {
		if util.IsInt64InSlice(teams[i].ID, newWhitelist) {
			whitelist = append(whitelist, teams[i].ID)
		}
	}

	return
}

// DeleteProtectedBranch removes ProtectedBranch relation between the user and repository.
func (repo *Repository) DeleteProtectedBranch(id int64) (err error) {
	protectedBranch := &ProtectedBranch{
		RepoID: repo.ID,
		ID:     id,
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if affected, err := sess.Delete(protectedBranch); err != nil {
		return err
	} else if affected != 1 {
		return fmt.Errorf("delete protected branch ID(%v) failed", id)
	}

	return sess.Commit()
}

// DeletedBranch struct
type DeletedBranch struct {
	ID          int64              `xorm:"pk autoincr"`
	RepoID      int64              `xorm:"UNIQUE(s) INDEX NOT NULL"`
	Name        string             `xorm:"UNIQUE(s) NOT NULL"`
	Commit      string             `xorm:"UNIQUE(s) NOT NULL"`
	DeletedByID int64              `xorm:"INDEX"`
	DeletedBy   *User              `xorm:"-"`
	DeletedUnix timeutil.TimeStamp `xorm:"INDEX created"`
}

// AddDeletedBranch adds a deleted branch to the database
func (repo *Repository) AddDeletedBranch(branchName, commit string, deletedByID int64) error {
	deletedBranch := &DeletedBranch{
		RepoID:      repo.ID,
		Name:        branchName,
		Commit:      commit,
		DeletedByID: deletedByID,
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if _, err := sess.InsertOne(deletedBranch); err != nil {
		return err
	}

	return sess.Commit()
}

// GetDeletedBranches returns all the deleted branches
func (repo *Repository) GetDeletedBranches() ([]*DeletedBranch, error) {
	deletedBranches := make([]*DeletedBranch, 0)
	return deletedBranches, x.Where("repo_id = ?", repo.ID).Desc("deleted_unix").Find(&deletedBranches)
}

// GetDeletedBranchByID get a deleted branch by its ID
func (repo *Repository) GetDeletedBranchByID(id int64) (*DeletedBranch, error) {
	deletedBranch := &DeletedBranch{}
	has, err := x.ID(id).Get(deletedBranch)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, nil
	}
	return deletedBranch, nil
}

// RemoveDeletedBranch removes a deleted branch from the database
func (repo *Repository) RemoveDeletedBranch(id int64) (err error) {
	deletedBranch := &DeletedBranch{
		RepoID: repo.ID,
		ID:     id,
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if affected, err := sess.Delete(deletedBranch); err != nil {
		return err
	} else if affected != 1 {
		return fmt.Errorf("remove deleted branch ID(%v) failed", id)
	}

	return sess.Commit()
}

// LoadUser loads the user that deleted the branch
// When there's no user found it returns a NewGhostUser
func (deletedBranch *DeletedBranch) LoadUser() {
	user, err := GetUserByID(deletedBranch.DeletedByID)
	if err != nil {
		user = NewGhostUser()
	}
	deletedBranch.DeletedBy = user
}

// RemoveDeletedBranch removes all deleted branches
func RemoveDeletedBranch(repoID int64, branch string) error {
	_, err := x.Where("repo_id=? AND name=?", repoID, branch).Delete(new(DeletedBranch))
	return err
}

// RemoveOldDeletedBranches removes old deleted branches
func RemoveOldDeletedBranches(ctx context.Context, olderThan time.Duration) {
	// Nothing to do for shutdown or terminate
	log.Trace("Doing: DeletedBranchesCleanup")

	deleteBefore := time.Now().Add(-olderThan)
	_, err := x.Where("deleted_unix < ?", deleteBefore.Unix()).Delete(new(DeletedBranch))
	if err != nil {
		log.Error("DeletedBranchesCleanup: %v", err)
	}
}
