// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
)

// ToCommitUser convert a git.Signature to an api.CommitUser
func ToCommitUser(sig *git.Signature) *api.CommitUser {
	return &api.CommitUser{
		Identity: api.Identity{
			Name:  sig.Name,
			Email: sig.Email,
		},
		Date: sig.When.UTC().Format(time.RFC3339),
	}
}

// ToCommitMeta convert a git.Tag to an api.CommitMeta
func ToCommitMeta(repo *models.Repository, tag *git.Tag) *api.CommitMeta {
	return &api.CommitMeta{
		SHA:     tag.Object.String(),
		URL:     util.URLJoin(repo.APIURL(), "git/commits", tag.ID.String()),
		Created: tag.Tagger.When,
	}
}

// ToPayloadCommit convert a git.Commit to api.PayloadCommit
func ToPayloadCommit(repo *models.Repository, c *git.Commit) *api.PayloadCommit {
	authorUsername := ""
	if author, err := models.GetUserByEmail(c.Author.Email); err == nil {
		authorUsername = author.Name
	} else if !models.IsErrUserNotExist(err) {
		log.Error("GetUserByEmail: %v", err)
	}

	committerUsername := ""
	if committer, err := models.GetUserByEmail(c.Committer.Email); err == nil {
		committerUsername = committer.Name
	} else if !models.IsErrUserNotExist(err) {
		log.Error("GetUserByEmail: %v", err)
	}

	return &api.PayloadCommit{
		ID:      c.ID.String(),
		Message: c.Message(),
		URL:     util.URLJoin(repo.HTMLURL(), "commit", c.ID.String()),
		Author: &api.PayloadUser{
			Name:     c.Author.Name,
			Email:    c.Author.Email,
			UserName: authorUsername,
		},
		Committer: &api.PayloadUser{
			Name:     c.Committer.Name,
			Email:    c.Committer.Email,
			UserName: committerUsername,
		},
		Timestamp:    c.Author.When,
		Verification: ToVerification(c),
	}
}

// ToCommit convert a git.Commit to api.Commit
func ToCommit(repo *models.Repository, commit *git.Commit, userCache map[string]*models.User) (*api.Commit, error) {

	var apiAuthor, apiCommitter *api.User

	// Retrieve author and committer information

	var cacheAuthor *models.User
	var ok bool
	if userCache == nil {
		cacheAuthor = (*models.User)(nil)
		ok = false
	} else {
		cacheAuthor, ok = userCache[commit.Author.Email]
	}

	if ok {
		apiAuthor = ToUser(cacheAuthor, nil)
	} else {
		author, err := models.GetUserByEmail(commit.Author.Email)
		if err != nil && !models.IsErrUserNotExist(err) {
			return nil, err
		} else if err == nil {
			apiAuthor = ToUser(author, nil)
			if userCache != nil {
				userCache[commit.Author.Email] = author
			}
		}
	}

	var cacheCommitter *models.User
	if userCache == nil {
		cacheCommitter = (*models.User)(nil)
		ok = false
	} else {
		cacheCommitter, ok = userCache[commit.Committer.Email]
	}

	if ok {
		apiCommitter = ToUser(cacheCommitter, nil)
	} else {
		committer, err := models.GetUserByEmail(commit.Committer.Email)
		if err != nil && !models.IsErrUserNotExist(err) {
			return nil, err
		} else if err == nil {
			apiCommitter = ToUser(committer, nil)
			if userCache != nil {
				userCache[commit.Committer.Email] = committer
			}
		}
	}

	// Retrieve parent(s) of the commit
	apiParents := make([]*api.CommitMeta, commit.ParentCount())
	for i := 0; i < commit.ParentCount(); i++ {
		sha, _ := commit.ParentID(i)
		apiParents[i] = &api.CommitMeta{
			URL: repo.APIURL() + "/git/commits/" + sha.String(),
			SHA: sha.String(),
		}
	}

	// Retrieve files affected by the commit
	fileStatus, err := git.GetCommitFileStatus(repo.RepoPath(), commit.ID.String())
	if err != nil {
		return nil, err
	}
	affectedFileList := make([]*api.CommitAffectedFiles, 0, len(fileStatus.Added)+len(fileStatus.Removed)+len(fileStatus.Modified))
	for _, files := range [][]string{fileStatus.Added, fileStatus.Removed, fileStatus.Modified} {
		for _, filename := range files {
			affectedFileList = append(affectedFileList, &api.CommitAffectedFiles{
				Filename: filename,
			})
		}
	}

	return &api.Commit{
		CommitMeta: &api.CommitMeta{
			URL: repo.APIURL() + "/git/commits/" + commit.ID.String(),
			SHA: commit.ID.String(),
		},
		HTMLURL: repo.HTMLURL() + "/commit/" + commit.ID.String(),
		RepoCommit: &api.RepoCommit{
			URL: repo.APIURL() + "/git/commits/" + commit.ID.String(),
			Author: &api.CommitUser{
				Identity: api.Identity{
					Name:  commit.Author.Name,
					Email: commit.Author.Email,
				},
				Date: commit.Author.When.Format(time.RFC3339),
			},
			Committer: &api.CommitUser{
				Identity: api.Identity{
					Name:  commit.Committer.Name,
					Email: commit.Committer.Email,
				},
				Date: commit.Committer.When.Format(time.RFC3339),
			},
			Message: commit.Message(),
			Tree: &api.CommitMeta{
				URL: repo.APIURL() + "/git/trees/" + commit.ID.String(),
				SHA: commit.ID.String(),
			},
		},
		Author:    apiAuthor,
		Committer: apiCommitter,
		Parents:   apiParents,
		Files:     affectedFileList,
	}, nil
}
