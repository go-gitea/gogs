// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"xorm.io/xorm"
)

func addBranchProtectionCanPushAndEnableWhitelist(x *xorm.Engine) error {
	type ProtectedBranch struct {
		CanPush                   bool    `xorm:"NOT NULL DEFAULT false"`
		EnableApprovalsWhitelist  bool    `xorm:"NOT NULL DEFAULT false"`
		ApprovalsWhitelistUserIDs []int64 `xorm:"JSON TEXT"`
		ApprovalsWhitelistTeamIDs []int64 `xorm:"JSON TEXT"`
		RequiredApprovals         int64   `xorm:"NOT NULL DEFAULT 0"`
	}

	type User struct {
		ID   int64 `xorm:"pk autoincr"`
		Type int

		// Permissions
		IsAdmin      bool
		IsRestricted bool `xorm:"NOT NULL DEFAULT false"`
		Visibility   int  `xorm:"NOT NULL DEFAULT 0"`
	}

	type Review struct {
		ID       int64 `xorm:"pk autoincr"`
		Official bool  `xorm:"NOT NULL DEFAULT false"`

		ReviewerID int64 `xorm:"index"`
		IssueID    int64 `xorm:"index"`
	}

	if err := x.Sync2(new(ProtectedBranch)); err != nil {
		return err
	}

	if err := x.Sync2(new(Review)); err != nil {
		return err
	}

	const (
		// ReviewTypeApprove approves changes
		ReviewTypeApprove int = 1
		// ReviewTypeReject gives feedback blocking merge
		ReviewTypeReject int = 3

		// VisibleTypePublic Visible for everyone
		VisibleTypePublic int = 0
		// VisibleTypePrivate Visible only for organization's members
		VisibleTypePrivate int = 2

		// UnitTypeCode is unit type code
		UnitTypeCode int = 1

		// AccessModeNone no access
		AccessModeNone int = 0
		// AccessModeRead read access
		AccessModeRead int = 1
		// AccessModeWrite write access
		AccessModeWrite int = 2
		// AccessModeOwner owner access
		AccessModeOwner int = 4
	)

	// Repository represents a git repository.
	type Repository struct {
		ID      int64 `xorm:"pk autoincr"`
		OwnerID int64 `xorm:"UNIQUE(s) index"`

		IsPrivate bool `xorm:"INDEX"`
	}

	type PullRequest struct {
		ID int64 `xorm:"pk autoincr"`

		BaseRepoID int64 `xorm:"INDEX"`
		BaseBranch string
	}

	// RepoUnit describes all units of a repository
	type RepoUnit struct {
		ID     int64
		RepoID int64 `xorm:"INDEX(s)"`
		Type   int   `xorm:"INDEX(s)"`
	}

	type Permission struct {
		AccessMode int
		Units      []*RepoUnit
		UnitsMode  map[int]int
	}

	type TeamUser struct {
		ID     int64 `xorm:"pk autoincr"`
		TeamID int64 `xorm:"UNIQUE(s)"`
		UID    int64 `xorm:"UNIQUE(s)"`
	}

	type Collaboration struct {
		ID     int64 `xorm:"pk autoincr"`
		RepoID int64 `xorm:"UNIQUE(s) INDEX NOT NULL"`
		UserID int64 `xorm:"UNIQUE(s) INDEX NOT NULL"`
		Mode   int   `xorm:"DEFAULT 2 NOT NULL"`
	}

	type Access struct {
		ID     int64 `xorm:"pk autoincr"`
		UserID int64 `xorm:"UNIQUE(s)"`
		RepoID int64 `xorm:"UNIQUE(s)"`
		Mode   int
	}

	type TeamUnit struct {
		ID     int64 `xorm:"pk autoincr"`
		OrgID  int64 `xorm:"INDEX"`
		TeamID int64 `xorm:"UNIQUE(s)"`
		Type   int   `xorm:"UNIQUE(s)"`
	}

	// Team represents a organization team.
	type Team struct {
		ID        int64 `xorm:"pk autoincr"`
		OrgID     int64 `xorm:"INDEX"`
		Authorize int
	}

	// getUserRepoPermission static function based on models.IsOfficialReviewer at 5d78792385
	getUserRepoPermission := func(sess *xorm.Session, repo *Repository, user *User) (Permission, error) {
		var perm Permission

		repoOwner := new(User)
		has, err := sess.ID(repo.OwnerID).Get(repoOwner)
		if err != nil || !has {
			return perm, err
		}

		// Prevent strangers from checking out public repo of private organization
		// Allow user if they are collaborator of a repo within a private organization but not a member of the organization itself
		hasOrgVisible := true
		// Not SignedUser
		if user == nil {
			hasOrgVisible = repoOwner.Visibility == VisibleTypePublic
		} else if !user.IsAdmin {
			hasMemberWithUserID, err := sess.
				Where("uid=?", user.ID).
				And("org_id=?", repoOwner.ID).
				Table("org_user").
				Exist()
			if err != nil {
				hasOrgVisible = false
			}
			if (repoOwner.Visibility == VisibleTypePrivate || user.IsRestricted) && !hasMemberWithUserID {
				hasOrgVisible = false
			}
		}

		isCollaborator, err := sess.Get(&Collaboration{RepoID: repo.ID, UserID: user.ID})
		if err != nil {
			return perm, err
		}

		if repoOwner.Type == 1 && !hasOrgVisible && !isCollaborator {
			perm.AccessMode = AccessModeNone
			return perm, err
		}

		var units []*RepoUnit
		if err := sess.Where("repo_id = ?", repo.ID).Find(&units); err != nil {
			return perm, err
		}
		perm.Units = units

		// anonymous visit public repo
		if user == nil {
			perm.AccessMode = AccessModeRead
			return perm, err
		}

		// Admin or the owner has super access to the repository
		if user.IsAdmin || user.ID == repo.OwnerID {
			perm.AccessMode = AccessModeOwner
			return perm, err
		}

		accessLevel := func(user *User, repo *Repository) (int, error) {
			mode := AccessModeNone
			var userID int64
			restricted := false

			if user != nil {
				userID = user.ID
				restricted = user.IsRestricted
			}

			if !restricted && !repo.IsPrivate {
				mode = AccessModeRead
			}

			if userID == 0 {
				return mode, nil
			}

			if userID == repo.OwnerID {
				return AccessModeOwner, nil
			}

			a := &Access{UserID: userID, RepoID: repo.ID}
			if has, err := sess.Get(a); !has || err != nil {
				return mode, err
			}
			return a.Mode, nil
		}

		// plain user
		perm.AccessMode, err = accessLevel(user, repo)
		if err != nil {
			return perm, err
		}

		// If Owner is no Org
		if repoOwner.Type != 1 {
			return perm, err
		}

		perm.UnitsMode = make(map[int]int)

		// Collaborators on organization
		if isCollaborator {
			for _, u := range units {
				perm.UnitsMode[u.Type] = perm.AccessMode
			}
		}

		// get units mode from teams
		var teams []*Team
		err = sess.
			Join("INNER", "team_user", "team_user.team_id = team.id").
			Join("INNER", "team_repo", "team_repo.team_id = team.id").
			Where("team.org_id = ?", repo.OwnerID).
			And("team_user.uid=?", user.ID).
			And("team_repo.repo_id=?", repo.ID).
			Find(&teams)
		if err != nil {
			return perm, err
		}

		// if user in an owner team
		for _, team := range teams {
			if team.Authorize >= AccessModeOwner {
				perm.AccessMode = AccessModeOwner
				perm.UnitsMode = nil
				return perm, err
			}
		}

		for _, u := range units {
			var found bool
			for _, team := range teams {

				var teamU []*TeamUnit
				var unitEnabled bool
				err = sess.Where("team_id = ?", team.ID).Find(&teamU)

				for _, tu := range teamU {
					if tu.Type == u.Type {
						unitEnabled = true
						break
					}
				}

				if unitEnabled {
					m := perm.UnitsMode[u.Type]
					if m < team.Authorize {
						perm.UnitsMode[u.Type] = team.Authorize
					}
					found = true
				}
			}

			// for a public repo on an organization, a non-restricted user has read permission on non-team defined units.
			if !found && !repo.IsPrivate && !user.IsRestricted {
				if _, ok := perm.UnitsMode[u.Type]; !ok {
					perm.UnitsMode[u.Type] = AccessModeRead
				}
			}
		}

		// remove no permission units
		perm.Units = make([]*RepoUnit, 0, len(units))
		for t := range perm.UnitsMode {
			for _, u := range units {
				if u.Type == t {
					perm.Units = append(perm.Units, u)
				}
			}
		}

		return perm, err
	}

	// isOfficialReviewer static function based on 5d78792385
	isOfficialReviewer := func(sess *xorm.Session, issueID int64, reviewer *User) (bool, error) {
		pr := new(PullRequest)
		has, err := sess.ID(issueID).Get(pr)
		if err != nil {
			return false, err
		} else if !has {
			return false, fmt.Errorf("PullRequest for issueID %d not exist", issueID)
		}

		baseRepo := new(Repository)
		has, err = sess.ID(pr.BaseRepoID).Get(baseRepo)
		if err != nil {
			return false, err
		} else if !has {
			return false, fmt.Errorf("baseRepo with id %d not exist", pr.BaseRepoID)
		}
		protectedBranch := new(ProtectedBranch)
		has, err = sess.Where("repo_id=? AND branch_name=?", baseRepo.ID, pr.BaseBranch).Get(protectedBranch)
		if err != nil {
			return false, err
		}
		if !has {
			return false, nil
		}

		if !protectedBranch.EnableApprovalsWhitelist {

			perm, err := getUserRepoPermission(sess, baseRepo, reviewer)
			if err != nil {
				return false, err
			}
			if perm.UnitsMode == nil {
				for _, u := range perm.Units {
					if u.Type == UnitTypeCode {
						return AccessModeWrite <= perm.AccessMode, nil
					}
				}
				return false, nil
			}
			return AccessModeWrite <= perm.UnitsMode[UnitTypeCode], nil
		}
		for _, id := range protectedBranch.ApprovalsWhitelistUserIDs {
			if id == reviewer.ID {
				return true, nil
			}
		}

		// isUserInTeams
		return sess.Where("uid=?", reviewer.ID).In("team_id", protectedBranch.ApprovalsWhitelistTeamIDs).Exist(new(TeamUser))
	}

	if _, err := x.Exec("UPDATE `protected_branch` SET `enable_whitelist` = ? WHERE enable_whitelist IS NULL", false); err != nil {
		return err
	}
	if _, err := x.Exec("UPDATE `protected_branch` SET `can_push` = `enable_whitelist`"); err != nil {
		return err
	}
	if _, err := x.Exec("UPDATE `protected_branch` SET `enable_approvals_whitelist` = ? WHERE `required_approvals` > ?", true, 0); err != nil {
		return err
	}

	var pageSize int64 = 20
	qresult, err := x.QueryInterface("SELECT max(id) as max_id FROM issue")
	if err != nil {
		return err
	}
	var totalIssues int64
	totalIssues, ok := qresult[0]["max_id"].(int64)
	if !ok {
		// If there are no issues at all we ignore it
		return nil
	}
	totalPages := totalIssues / pageSize

	executeBody := func(page, pageSize int64) error {
		// Find latest review of each user in each pull request, and set official field if appropriate
		reviews := []*Review{}

		if err := x.SQL("SELECT * FROM review WHERE id IN (SELECT max(id) as id FROM review WHERE issue_id > ? AND issue_id <= ? AND type in (?, ?) GROUP BY issue_id, reviewer_id)",
			page*pageSize, (page+1)*pageSize, ReviewTypeApprove, ReviewTypeReject).
			Find(&reviews); err != nil {
			return err
		}

		if len(reviews) == 0 {
			return nil
		}

		sess := x.NewSession()
		defer sess.Close()

		if err := sess.Begin(); err != nil {
			return err
		}

		var updated int
		for _, review := range reviews {
			reviewer := new(User)
			has, err := sess.ID(review.ReviewerID).Get(reviewer)
			if err != nil || !has {
				// Error might occur if user doesn't exist, ignore it.
				continue
			}

			official, err := isOfficialReviewer(sess, review.IssueID, reviewer)
			if err != nil {
				// Branch might not be proteced or other error, ignore it.
				continue
			}
			review.Official = official
			updated++
			if _, err := sess.ID(review.ID).Cols("official").Update(review); err != nil {
				return err
			}
		}

		if updated > 0 {
			return sess.Commit()
		}
		return nil
	}

	var page int64
	for page = 0; page <= totalPages; page++ {
		if err := executeBody(page, pageSize); err != nil {
			return err
		}
	}

	return nil
}
