// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"os"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
)

// RepoTransfer is used to manage repository transfers
type RepoTransfer struct {
	ID          int64 `xorm:"pk autoincr"`
	DoerID      int64
	Doer        *User `xorm:"-"`
	RecipientID int64
	Recipient   *User `xorm:"-"`
	RepoID      int64
	TeamIDs     []int64
	Teams       []*Team `xorm:"-"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX NOT NULL created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX NOT NULL updated"`
}

// LoadAttributes fetches the transfer recipient from the database
func (r *RepoTransfer) LoadAttributes() error {
	if r.Recipient == nil {
		u, err := GetUserByID(r.RecipientID)
		if err != nil {
			return err
		}

		r.Recipient = u
	}

	if r.Recipient.IsOrganization() && len(r.TeamIDs) != len(r.Teams) {
		for _, v := range r.TeamIDs {
			team, err := GetTeamByID(v)
			if err != nil {
				return err
			}

			if team.OrgID != r.Recipient.ID {
				return fmt.Errorf("team %d belongs not to org %d", v, r.Recipient.ID)
			}

			r.Teams = append(r.Teams, team)
		}
	}

	if r.Doer == nil {
		u, err := GetUserByID(r.DoerID)
		if err != nil {
			return err
		}

		r.Doer = u
	}

	return nil
}

// CanUserAcceptTransfer checks if the user has the rights to accept/decline a repo transfer.
// For user, it checks if it's himself
// For organizations, it checks if the user is able to create repos
func (r *RepoTransfer) CanUserAcceptTransfer(u *User) bool {
	if err := r.LoadAttributes(); err != nil {
		log.Error("LoadAttributes: %v", err)
		return false
	}

	if !r.Recipient.IsOrganization() {
		return r.RecipientID == u.ID
	}

	allowed, err := CanCreateOrgRepo(r.RecipientID, u.ID)
	if err != nil {
		log.Error("CanCreateOrgRepo: %v", err)
		return false
	}

	return allowed
}

// GetPendingRepositoryTransfer fetches the most recent and ongoing transfer
// process for the repository
func GetPendingRepositoryTransfer(repo *Repository) (*RepoTransfer, error) {
	transfer := new(RepoTransfer)

	has, err := x.Where("repo_id = ? ", repo.ID).Get(transfer)
	if err != nil {
		return nil, err
	}

	if !has {
		return nil, ErrNoPendingRepoTransfer{RepoID: repo.ID}
	}

	return transfer, nil
}

func deleteRepositoryTransfer(e Engine, repoID int64) error {
	_, err := e.Where("repo_id = ?", repoID).Delete(&RepoTransfer{})
	return err
}

// CancelRepositoryTransfer marks the repository as ready and remove pending transfer entry,
// thus cancel the transfer process.
func CancelRepositoryTransfer(repo *Repository) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	repo.Status = RepositoryReady
	if err := updateRepositoryCols(sess, repo, "status"); err != nil {
		return err
	}

	if err := deleteRepositoryTransfer(sess, repo.ID); err != nil {
		return err
	}

	return sess.Commit()
}

// TestRepositoryReadyForTransfer make sure repo is ready to transfer
func TestRepositoryReadyForTransfer(status RepositoryStatus) error {
	switch status {
	case RepositoryBeingMigrated:
		return fmt.Errorf("repo is not ready, currently migrating")
	case RepositoryPendingTransfer:
		return ErrRepoTransferInProgress{}
	}
	return nil
}

// CreatePendingRepositoryTransfer transfer a repo from one owner to a new one.
// it marks the repository transfer as "pending"
func CreatePendingRepositoryTransfer(doer, newOwner *User, repoID int64, teams []*Team) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	repo, err := getRepositoryByID(sess, repoID)
	if err != nil {
		return err
	}

	// Make sure repo is ready to transfer
	if err := TestRepositoryReadyForTransfer(repo.Status); err != nil {
		return err
	}

	repo.Status = RepositoryPendingTransfer
	if err := updateRepositoryCols(sess, repo, "status"); err != nil {
		return err
	}

	// Check if new owner has repository with same name.
	if has, err := isRepositoryExist(sess, newOwner, repo.Name); err != nil {
		return fmt.Errorf("IsRepositoryExist: %v", err)
	} else if has {
		return ErrRepoAlreadyExist{newOwner.LowerName, repo.Name}
	}

	transfer := &RepoTransfer{
		RepoID:      repo.ID,
		RecipientID: newOwner.ID,
		CreatedUnix: timeutil.TimeStampNow(),
		UpdatedUnix: timeutil.TimeStampNow(),
		DoerID:      doer.ID,
		TeamIDs:     make([]int64, 0, len(teams)),
	}

	for k := range teams {
		transfer.TeamIDs = append(transfer.TeamIDs, teams[k].ID)
	}

	if _, err := sess.Insert(transfer); err != nil {
		return err
	}

	return sess.Commit()
}

// TransferOwnership transfers all corresponding repository items from old user to new one.
func TransferOwnership(doer *User, newOwnerName string, repo *Repository) (err error) {
	repoRenamed := false
	wikiRenamed := false
	oldOwnerName := doer.Name

	defer func() {
		if !repoRenamed && !wikiRenamed {
			return
		}

		recoverErr := recover()
		if err == nil && recoverErr == nil {
			return
		}

		if repoRenamed {
			if err := util.Rename(RepoPath(newOwnerName, repo.Name), RepoPath(oldOwnerName, repo.Name)); err != nil {
				log.Critical("Unable to move repository %s/%s directory from %s back to correct place %s: %v", oldOwnerName, repo.Name, RepoPath(newOwnerName, repo.Name), RepoPath(oldOwnerName, repo.Name), err)
			}
		}

		if wikiRenamed {
			if err := util.Rename(WikiPath(newOwnerName, repo.Name), WikiPath(oldOwnerName, repo.Name)); err != nil {
				log.Critical("Unable to move wiki for repository %s/%s directory from %s back to correct place %s: %v", oldOwnerName, repo.Name, WikiPath(newOwnerName, repo.Name), WikiPath(oldOwnerName, repo.Name), err)
			}
		}

		if recoverErr != nil {
			log.Error("Panic within TransferOwnership: %v\n%s", recoverErr, log.Stack(2))
			panic(recoverErr)
		}
	}()

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return fmt.Errorf("sess.Begin: %v", err)
	}

	newOwner, err := getUserByName(sess, newOwnerName)
	if err != nil {
		return fmt.Errorf("get new owner '%s': %v", newOwnerName, err)
	}
	newOwnerName = newOwner.Name // ensure capitalisation matches

	// Check if new owner has repository with same name.
	if has, err := isRepositoryExist(sess, newOwner, repo.Name); err != nil {
		return fmt.Errorf("IsRepositoryExist: %v", err)
	} else if has {
		return ErrRepoAlreadyExist{newOwnerName, repo.Name}
	}

	oldOwner := repo.Owner
	oldOwnerName = oldOwner.Name

	// Note: we have to set value here to make sure recalculate accesses is based on
	// new owner.
	repo.OwnerID = newOwner.ID
	repo.Owner = newOwner
	repo.OwnerName = newOwner.Name

	// Update repository.
	if _, err := sess.ID(repo.ID).Update(repo); err != nil {
		return fmt.Errorf("update owner: %v", err)
	}

	// Remove redundant collaborators.
	collaborators, err := repo.getCollaborators(sess, ListOptions{})
	if err != nil {
		return fmt.Errorf("getCollaborators: %v", err)
	}

	// Dummy object.
	collaboration := &Collaboration{RepoID: repo.ID}
	for _, c := range collaborators {
		if c.ID != newOwner.ID {
			isMember, err := isOrganizationMember(sess, newOwner.ID, c.ID)
			if err != nil {
				return fmt.Errorf("IsOrgMember: %v", err)
			} else if !isMember {
				continue
			}
		}
		collaboration.UserID = c.ID
		if _, err := sess.Delete(collaboration); err != nil {
			return fmt.Errorf("remove collaborator '%d': %v", c.ID, err)
		}
	}

	// Remove old team-repository relations.
	if oldOwner.IsOrganization() {
		if err := oldOwner.removeOrgRepo(sess, repo.ID); err != nil {
			return fmt.Errorf("removeOrgRepo: %v", err)
		}
	}

	if newOwner.IsOrganization() {
		if err := newOwner.getTeams(sess); err != nil {
			return fmt.Errorf("GetTeams: %v", err)
		}
		for _, t := range newOwner.Teams {
			if t.IncludesAllRepositories {
				if err := t.addRepository(sess, repo); err != nil {
					return fmt.Errorf("addRepository: %v", err)
				}
			}
		}
	} else if err := repo.recalculateAccesses(sess); err != nil {
		// Organization called this in addRepository method.
		return fmt.Errorf("recalculateAccesses: %v", err)
	}

	// Update repository count.
	if _, err := sess.Exec("UPDATE `user` SET num_repos=num_repos+1 WHERE id=?", newOwner.ID); err != nil {
		return fmt.Errorf("increase new owner repository count: %v", err)
	} else if _, err := sess.Exec("UPDATE `user` SET num_repos=num_repos-1 WHERE id=?", oldOwner.ID); err != nil {
		return fmt.Errorf("decrease old owner repository count: %v", err)
	}

	if err := watchRepo(sess, doer.ID, repo.ID, true); err != nil {
		return fmt.Errorf("watchRepo: %v", err)
	}

	// Remove watch for organization.
	if oldOwner.IsOrganization() {
		if err := watchRepo(sess, oldOwner.ID, repo.ID, false); err != nil {
			return fmt.Errorf("watchRepo [false]: %v", err)
		}
	}

	// Delete labels that belong to the old organization and comments that added these labels
	if oldOwner.IsOrganization() {
		if _, err := sess.Exec(`DELETE FROM issue_label WHERE issue_label.id IN (
			SELECT il_too.id FROM (
				SELECT il_too_too.id
					FROM issue_label AS il_too_too
						INNER JOIN label ON il_too_too.label_id = label.id
						INNER JOIN issue on issue.id = il_too_too.issue_id
					WHERE
						issue.repo_id = ? AND ((label.org_id = 0 AND issue.repo_id != label.repo_id) OR (label.repo_id = 0 AND label.org_id != ?))
		) AS il_too )`, repo.ID, newOwner.ID); err != nil {
			return fmt.Errorf("Unable to remove old org labels: %v", err)
		}

		if _, err := sess.Exec(`DELETE FROM comment WHERE comment.id IN (
			SELECT il_too.id FROM (
				SELECT com.id
					FROM comment AS com
						INNER JOIN label ON com.label_id = label.id
						INNER JOIN issue ON issue.id = com.issue_id
					WHERE
						com.type = ? AND issue.repo_id = ? AND ((label.org_id = 0 AND issue.repo_id != label.repo_id) OR (label.repo_id = 0 AND label.org_id != ?))
		) AS il_too)`, CommentTypeLabel, repo.ID, newOwner.ID); err != nil {
			return fmt.Errorf("Unable to remove old org label comments: %v", err)
		}
	}

	// Rename remote repository to new path and delete local copy.
	dir := UserPath(newOwner.Name)

	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return fmt.Errorf("Failed to create dir %s: %v", dir, err)
	}

	if err := util.Rename(RepoPath(oldOwner.Name, repo.Name), RepoPath(newOwner.Name, repo.Name)); err != nil {
		return fmt.Errorf("rename repository directory: %v", err)
	}
	repoRenamed = true

	// Rename remote wiki repository to new path and delete local copy.
	wikiPath := WikiPath(oldOwner.Name, repo.Name)

	if isExist, err := util.IsExist(wikiPath); err != nil {
		log.Error("Unable to check if %s exists. Error: %v", wikiPath, err)
		return err
	} else if isExist {
		if err := util.Rename(wikiPath, WikiPath(newOwner.Name, repo.Name)); err != nil {
			return fmt.Errorf("rename repository wiki: %v", err)
		}
		wikiRenamed = true
	}

	if err := deleteRepositoryTransfer(sess, repo.ID); err != nil {
		return fmt.Errorf("deleteRepositoryTransfer: %v", err)
	}
	repo.Status = RepositoryReady
	if err := updateRepositoryCols(sess, repo, "status"); err != nil {
		return err
	}

	// If there was previously a redirect at this location, remove it.
	if err := deleteRepoRedirect(sess, newOwner.ID, repo.Name); err != nil {
		return fmt.Errorf("delete repo redirect: %v", err)
	}

	if err := newRepoRedirect(sess, oldOwner.ID, repo.ID, repo.Name, repo.Name); err != nil {
		return fmt.Errorf("newRepoRedirect: %v", err)
	}

	return sess.Commit()
}
