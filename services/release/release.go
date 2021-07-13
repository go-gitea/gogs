// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package release

import (
	"errors"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification"
	"code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/timeutil"
)

func createTag(gitRepo *git.Repository, rel *models.Release, msg string) (bool, error) {
	var created bool
	// Only actual create when publish.
	if !rel.IsDraft {
		if !gitRepo.IsTagExist(rel.TagName) {
			if err := rel.LoadAttributes(); err != nil {
				log.Error("LoadAttributes: %v", err)
				return false, err
			}

			protectedTags, err := rel.Repo.GetProtectedTags()
			if err != nil {
				return false, fmt.Errorf("GetProtectedTags: %v", err)
			}
			isAllowed, err := models.IsUserAllowedToControlTag(protectedTags, rel.TagName, rel.PublisherID)
			if err != nil {
				return false, err
			}
			if !isAllowed {
				return false, models.ErrProtectedTagName{
					TagName: rel.TagName,
				}
			}

			commit, err := gitRepo.GetCommit(rel.Target)
			if err != nil {
				return false, fmt.Errorf("GetCommit: %v", err)
			}

			// Trim '--' prefix to prevent command line argument vulnerability.
			rel.TagName = strings.TrimPrefix(rel.TagName, "--")
			if len(msg) > 0 {
				if err = gitRepo.CreateAnnotatedTag(rel.TagName, msg, commit.ID.String()); err != nil {
					if strings.Contains(err.Error(), "is not a valid tag name") {
						return false, models.ErrInvalidTagName{
							TagName: rel.TagName,
						}
					}
					return false, err
				}
			} else if err = gitRepo.CreateTag(rel.TagName, commit.ID.String()); err != nil {
				if strings.Contains(err.Error(), "is not a valid tag name") {
					return false, models.ErrInvalidTagName{
						TagName: rel.TagName,
					}
				}
				return false, err
			}
			created = true
			rel.LowerTagName = strings.ToLower(rel.TagName)

			notification.NotifyPushCommits(
				rel.Publisher, rel.Repo,
				&repository.PushUpdateOptions{
					RefFullName: git.TagPrefix + rel.TagName,
					OldCommitID: git.EmptySHA,
					NewCommitID: commit.ID.String(),
				}, repository.NewPushCommits())
			notification.NotifyCreateRef(rel.Publisher, rel.Repo, "tag", git.TagPrefix+rel.TagName)
			rel.CreatedUnix = timeutil.TimeStampNow()
		}
		commit, err := gitRepo.GetTagCommit(rel.TagName)
		if err != nil {
			return false, fmt.Errorf("GetTagCommit: %v", err)
		}

		rel.Sha1 = commit.ID.String()
		rel.NumCommits, err = commit.CommitsCount()
		if err != nil {
			return false, fmt.Errorf("CommitsCount: %v", err)
		}

		if rel.PublisherID <= 0 {
			u, err := models.GetUserByEmail(commit.Author.Email)
			if err == nil {
				rel.PublisherID = u.ID
			}
		}
	} else {
		rel.CreatedUnix = timeutil.TimeStampNow()
	}
	return created, nil
}

// CreateRelease creates a new release of repository.
func CreateRelease(gitRepo *git.Repository, rel *models.Release, attachmentUUIDs []string, msg string) error {
	isExist, err := models.IsReleaseExist(rel.RepoID, rel.TagName)
	if err != nil {
		return err
	} else if isExist {
		return models.ErrReleaseAlreadyExist{
			TagName: rel.TagName,
		}
	}

	if _, err = createTag(gitRepo, rel, msg); err != nil {
		return err
	}

	rel.LowerTagName = strings.ToLower(rel.TagName)
	if err = models.InsertRelease(rel); err != nil {
		return err
	}

	if err = models.AddReleaseAttachments(models.DefaultDBContext(), rel.ID, attachmentUUIDs); err != nil {
		return err
	}

	if !rel.IsDraft {
		notification.NotifyNewRelease(rel)
	}

	return nil
}

// CreateNewTag creates a new repository tag
func CreateNewTag(doer *models.User, repo *models.Repository, commit, tagName, msg string) error {
	isExist, err := models.IsReleaseExist(repo.ID, tagName)
	if err != nil {
		return err
	} else if isExist {
		return models.ErrTagAlreadyExists{
			TagName: tagName,
		}
	}

	gitRepo, err := git.OpenRepository(repo.RepoPath())
	if err != nil {
		return err
	}
	defer gitRepo.Close()

	rel := &models.Release{
		RepoID:       repo.ID,
		Repo:         repo,
		PublisherID:  doer.ID,
		Publisher:    doer,
		TagName:      tagName,
		Target:       commit,
		IsDraft:      false,
		IsPrerelease: false,
		IsTag:        true,
	}

	if _, err = createTag(gitRepo, rel, msg); err != nil {
		return err
	}

	if err = models.InsertRelease(rel); err != nil {
		return err
	}

	return err
}

// UpdateRelease updates information, attachments of a release and will create tag if it's not a draft and tag not exist.
// addAttachmentUUIDs accept a slice of new created attachments' uuids which will be reassigned release_id as the created release
// delAttachmentUUIDs accept a slice of attachments' uuids which will be deleted from the release
// editAttachments accept a map of attachment uuid to new attachment name which will be updated with attachments.
func UpdateRelease(doer *models.User, gitRepo *git.Repository, rel *models.Release,
	addAttachmentUUIDs, delAttachmentUUIDs []string, editAttachments map[string]string) (err error) {
	if rel.ID == 0 {
		return errors.New("UpdateRelease only accepts an exist release")
	}
	isCreated, err := createTag(gitRepo, rel, "")
	if err != nil {
		return err
	}
	rel.LowerTagName = strings.ToLower(rel.TagName)

	ctx, commiter, err := models.TxDBContext()
	if err != nil {
		return err
	}
	defer commiter.Close()

	if err = models.UpdateRelease(ctx, rel); err != nil {
		return err
	}

	if err = models.AddReleaseAttachments(ctx, rel.ID, addAttachmentUUIDs); err != nil {
		return fmt.Errorf("AddReleaseAttachments: %v", err)
	}

	var deletedUUIDsMap = make(map[string]bool)
	if len(delAttachmentUUIDs) > 0 {
		// Check attachments
		attachments, err := models.GetAttachmentsByUUIDs(ctx, delAttachmentUUIDs)
		if err != nil {
			return fmt.Errorf("GetAttachmentsByUUIDs [uuids: %v]: %v", delAttachmentUUIDs, err)
		}
		for _, attach := range attachments {
			if attach.ReleaseID != rel.ID {
				return errors.New("delete attachement of release permission denied")
			}
			deletedUUIDsMap[attach.UUID] = true
		}

		if _, err := models.DeleteAttachments(ctx, attachments, false); err != nil {
			return fmt.Errorf("DeleteAttachments [uuids: %v]: %v", delAttachmentUUIDs, err)
		}
	}

	if len(editAttachments) > 0 {
		var updateAttachmentsList = make([]string, 0, len(editAttachments))
		for k := range editAttachments {
			updateAttachmentsList = append(updateAttachmentsList, k)
		}
		// Check attachments
		attachments, err := models.GetAttachmentsByUUIDs(ctx, updateAttachmentsList)
		if err != nil {
			return fmt.Errorf("GetAttachmentsByUUIDs [uuids: %v]: %v", updateAttachmentsList, err)
		}
		for _, attach := range attachments {
			if attach.ReleaseID != rel.ID {
				return errors.New("update attachement of release permission denied")
			}
		}

		for uuid, newName := range editAttachments {
			if !deletedUUIDsMap[uuid] {
				if err = models.UpdateAttachmentByUUID(ctx, &models.Attachment{
					UUID: uuid,
					Name: newName,
				}, "name"); err != nil {
					return err
				}
			}
		}
	}

	if err = commiter.Commit(); err != nil {
		return
	}

	for _, uuid := range delAttachmentUUIDs {
		if err := storage.Attachments.Delete(models.AttachmentRelativePath(uuid)); err != nil {
			// Even delete files failed, but the attachments has been removed from database, so we
			// should not return error but only record the error on logs.
			// users have to delete this attachments manually or we should have a
			// synchronize between database attachment table and attachment storage
			log.Error("delete attachment[uuid: %s] failed: %v", uuid, err)
		}
	}

	if !isCreated {
		notification.NotifyUpdateRelease(doer, rel)
		return
	}

	if !rel.IsDraft {
		notification.NotifyNewRelease(rel)
	}

	return err
}

// DeleteReleaseByID deletes a release and corresponding Git tag by given ID.
func DeleteReleaseByID(id int64, doer *models.User, delTag bool) error {
	rel, err := models.GetReleaseByID(id)
	if err != nil {
		return fmt.Errorf("GetReleaseByID: %v", err)
	}

	repo, err := models.GetRepositoryByID(rel.RepoID)
	if err != nil {
		return fmt.Errorf("GetRepositoryByID: %v", err)
	}

	if delTag {
		if stdout, err := git.NewCommand("tag", "-d", rel.TagName).
			SetDescription(fmt.Sprintf("DeleteReleaseByID (git tag -d): %d", rel.ID)).
			RunInDir(repo.RepoPath()); err != nil && !strings.Contains(err.Error(), "not found") {
			log.Error("DeleteReleaseByID (git tag -d): %d in %v Failed:\nStdout: %s\nError: %v", rel.ID, repo, stdout, err)
			return fmt.Errorf("git tag -d: %v", err)
		}

		notification.NotifyPushCommits(
			doer, repo,
			&repository.PushUpdateOptions{
				RefFullName: git.TagPrefix + rel.TagName,
				OldCommitID: rel.Sha1,
				NewCommitID: git.EmptySHA,
			}, repository.NewPushCommits())
		notification.NotifyDeleteRef(doer, repo, "tag", git.TagPrefix+rel.TagName)

		if err := models.DeleteReleaseByID(id); err != nil {
			return fmt.Errorf("DeleteReleaseByID: %v", err)
		}
	} else {
		rel.IsTag = true

		if err = models.UpdateRelease(models.DefaultDBContext(), rel); err != nil {
			return fmt.Errorf("Update: %v", err)
		}
	}

	rel.Repo = repo
	if err = rel.LoadAttributes(); err != nil {
		return fmt.Errorf("LoadAttributes: %v", err)
	}

	if err := models.DeleteAttachmentsByRelease(rel.ID); err != nil {
		return fmt.Errorf("DeleteAttachments: %v", err)
	}

	for i := range rel.Attachments {
		attachment := rel.Attachments[i]
		if err := storage.Attachments.Delete(attachment.RelativePath()); err != nil {
			log.Error("Delete attachment %s of release %s failed: %v", attachment.UUID, rel.ID, err)
		}
	}

	notification.NotifyDeleteRelease(doer, rel)

	return nil
}
