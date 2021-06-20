// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/structs"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/webhook"
)

// ToEmail convert models.EmailAddress to api.Email
func ToEmail(email *models.EmailAddress) *api.Email {
	return &api.Email{
		Email:    email.Email,
		Verified: email.IsActivated,
		Primary:  email.IsPrimary,
	}
}

// ToBranch convert a git.Commit and git.Branch to an api.Branch
func ToBranch(repo *models.Repository, b *git.Branch, c *git.Commit, bp *models.ProtectedBranch, user *models.User, isRepoAdmin bool) (*api.Branch, error) {
	if bp == nil {
		var hasPerm bool
		var err error
		if user != nil {
			hasPerm, err = models.HasAccessUnit(user, repo, models.UnitTypeCode, models.AccessModeWrite)
			if err != nil {
				return nil, err
			}
		}

		return &api.Branch{
			Name:                b.Name,
			Commit:              ToPayloadCommit(repo, c),
			Protected:           false,
			RequiredApprovals:   0,
			EnableStatusCheck:   false,
			StatusCheckContexts: []string{},
			UserCanPush:         hasPerm,
			UserCanMerge:        hasPerm,
		}, nil
	}

	branch := &api.Branch{
		Name:                b.Name,
		Commit:              ToPayloadCommit(repo, c),
		Protected:           true,
		RequiredApprovals:   bp.RequiredApprovals,
		EnableStatusCheck:   bp.EnableStatusCheck,
		StatusCheckContexts: bp.StatusCheckContexts,
	}

	if isRepoAdmin {
		branch.EffectiveBranchProtectionName = bp.BranchName
	}

	if user != nil {
		permission, err := models.GetUserRepoPermission(repo, user)
		if err != nil {
			return nil, err
		}
		branch.UserCanPush = bp.CanUserPush(user.ID)
		branch.UserCanMerge = bp.IsUserMergeWhitelisted(user.ID, permission)
	}

	return branch, nil
}

// ToBranchProtection convert a ProtectedBranch to api.BranchProtection
func ToBranchProtection(bp *models.ProtectedBranch) *api.BranchProtection {
	pushWhitelistUsernames, err := models.GetUserNamesByIDs(bp.WhitelistUserIDs)
	if err != nil {
		log.Error("GetUserNamesByIDs (WhitelistUserIDs): %v", err)
	}
	mergeWhitelistUsernames, err := models.GetUserNamesByIDs(bp.MergeWhitelistUserIDs)
	if err != nil {
		log.Error("GetUserNamesByIDs (MergeWhitelistUserIDs): %v", err)
	}
	approvalsWhitelistUsernames, err := models.GetUserNamesByIDs(bp.ApprovalsWhitelistUserIDs)
	if err != nil {
		log.Error("GetUserNamesByIDs (ApprovalsWhitelistUserIDs): %v", err)
	}
	pushWhitelistTeams, err := models.GetTeamNamesByID(bp.WhitelistTeamIDs)
	if err != nil {
		log.Error("GetTeamNamesByID (WhitelistTeamIDs): %v", err)
	}
	mergeWhitelistTeams, err := models.GetTeamNamesByID(bp.MergeWhitelistTeamIDs)
	if err != nil {
		log.Error("GetTeamNamesByID (MergeWhitelistTeamIDs): %v", err)
	}
	approvalsWhitelistTeams, err := models.GetTeamNamesByID(bp.ApprovalsWhitelistTeamIDs)
	if err != nil {
		log.Error("GetTeamNamesByID (ApprovalsWhitelistTeamIDs): %v", err)
	}

	return &api.BranchProtection{
		BranchName:                    bp.BranchName,
		EnablePush:                    bp.CanPush,
		EnablePushWhitelist:           bp.EnableWhitelist,
		PushWhitelistUsernames:        pushWhitelistUsernames,
		PushWhitelistTeams:            pushWhitelistTeams,
		PushWhitelistDeployKeys:       bp.WhitelistDeployKeys,
		EnableMergeWhitelist:          bp.EnableMergeWhitelist,
		MergeWhitelistUsernames:       mergeWhitelistUsernames,
		MergeWhitelistTeams:           mergeWhitelistTeams,
		EnableStatusCheck:             bp.EnableStatusCheck,
		StatusCheckContexts:           bp.StatusCheckContexts,
		RequiredApprovals:             bp.RequiredApprovals,
		EnableApprovalsWhitelist:      bp.EnableApprovalsWhitelist,
		ApprovalsWhitelistUsernames:   approvalsWhitelistUsernames,
		ApprovalsWhitelistTeams:       approvalsWhitelistTeams,
		BlockOnRejectedReviews:        bp.BlockOnRejectedReviews,
		BlockOnOfficialReviewRequests: bp.BlockOnOfficialReviewRequests,
		BlockOnOutdatedBranch:         bp.BlockOnOutdatedBranch,
		DismissStaleApprovals:         bp.DismissStaleApprovals,
		RequireSignedCommits:          bp.RequireSignedCommits,
		ProtectedFilePatterns:         bp.ProtectedFilePatterns,
		Created:                       bp.CreatedUnix.AsTime(),
		Updated:                       bp.UpdatedUnix.AsTime(),
	}
}

// ToTag convert a git.Tag to an api.Tag
func ToTag(repo *models.Repository, t *git.Tag) *api.Tag {
	return &api.Tag{
		Name:       t.Name,
		Message:    strings.TrimSpace(t.Message),
		ID:         t.ID.String(),
		Commit:     ToCommitMeta(repo, t),
		ZipballURL: util.URLJoin(repo.HTMLURL(), "archive", t.Name+".zip"),
		TarballURL: util.URLJoin(repo.HTMLURL(), "archive", t.Name+".tar.gz"),
	}
}

// ToVerification convert a git.Commit.Signature to an api.PayloadCommitVerification
func ToVerification(c *git.Commit) *api.PayloadCommitVerification {
	verif := models.ParseCommitWithSignature(c)
	commitVerification := &api.PayloadCommitVerification{
		Verified: verif.Verified,
		Reason:   verif.Reason,
	}
	if c.Signature != nil {
		commitVerification.Signature = c.Signature.Signature
		commitVerification.Payload = c.Signature.Payload
	}
	if verif.SigningUser != nil {
		commitVerification.Signer = &structs.PayloadUser{
			Name:  verif.SigningUser.Name,
			Email: verif.SigningUser.Email,
		}
	}
	return commitVerification
}

// ToPublicKey convert models.PublicKey to api.PublicKey
func ToPublicKey(apiLink string, key *models.PublicKey) *api.PublicKey {
	return &api.PublicKey{
		ID:          key.ID,
		Key:         key.Content,
		URL:         fmt.Sprintf("%s%d", apiLink, key.ID),
		Title:       key.Name,
		Fingerprint: key.Fingerprint,
		Created:     key.CreatedUnix.AsTime(),
	}
}

// ToGPGKey converts models.GPGKey to api.GPGKey
func ToGPGKey(key *models.GPGKey) *api.GPGKey {
	subkeys := make([]*api.GPGKey, len(key.SubsKey))
	for id, k := range key.SubsKey {
		subkeys[id] = &api.GPGKey{
			ID:                k.ID,
			PrimaryKeyID:      k.PrimaryKeyID,
			KeyID:             k.KeyID,
			PublicKey:         k.Content,
			Created:           k.CreatedUnix.AsTime(),
			Expires:           k.ExpiredUnix.AsTime(),
			CanSign:           k.CanSign,
			CanEncryptComms:   k.CanEncryptComms,
			CanEncryptStorage: k.CanEncryptStorage,
			CanCertify:        k.CanSign,
		}
	}
	emails := make([]*api.GPGKeyEmail, len(key.Emails))
	for i, e := range key.Emails {
		emails[i] = ToGPGKeyEmail(e)
	}
	return &api.GPGKey{
		ID:                key.ID,
		PrimaryKeyID:      key.PrimaryKeyID,
		KeyID:             key.KeyID,
		PublicKey:         key.Content,
		Created:           key.CreatedUnix.AsTime(),
		Expires:           key.ExpiredUnix.AsTime(),
		Emails:            emails,
		SubsKey:           subkeys,
		CanSign:           key.CanSign,
		CanEncryptComms:   key.CanEncryptComms,
		CanEncryptStorage: key.CanEncryptStorage,
		CanCertify:        key.CanSign,
	}
}

// ToGPGKeyEmail convert models.EmailAddress to api.GPGKeyEmail
func ToGPGKeyEmail(email *models.EmailAddress) *api.GPGKeyEmail {
	return &api.GPGKeyEmail{
		Email:    email.Email,
		Verified: email.IsActivated,
	}
}

// ToHook convert models.Webhook to api.Hook
func ToHook(repoLink string, w *models.Webhook) *api.Hook {
	config := map[string]string{
		"url":          w.URL,
		"content_type": w.ContentType.Name(),
	}
	if w.Type == models.SLACK {
		s := webhook.GetSlackHook(w)
		config["channel"] = s.Channel
		config["username"] = s.Username
		config["icon_url"] = s.IconURL
		config["color"] = s.Color
	}

	return &api.Hook{
		ID:      w.ID,
		Type:    string(w.Type),
		URL:     fmt.Sprintf("%s/settings/hooks/%d", repoLink, w.ID),
		Active:  w.IsActive,
		Config:  config,
		Events:  w.EventsArray(),
		Updated: w.UpdatedUnix.AsTime(),
		Created: w.CreatedUnix.AsTime(),
	}
}

// ToGitHook convert git.Hook to api.GitHook
func ToGitHook(h *git.Hook) *api.GitHook {
	return &api.GitHook{
		Name:     h.Name(),
		IsActive: h.IsActive,
		Content:  h.Content,
	}
}

// ToDeployKey convert models.DeployKey to api.DeployKey
func ToDeployKey(apiLink string, key *models.DeployKey) *api.DeployKey {
	return &api.DeployKey{
		ID:          key.ID,
		KeyID:       key.KeyID,
		Key:         key.Content,
		Fingerprint: key.Fingerprint,
		URL:         fmt.Sprintf("%s%d", apiLink, key.ID),
		Title:       key.Name,
		Created:     key.CreatedUnix.AsTime(),
		ReadOnly:    key.Mode == models.AccessModeRead, // All deploy keys are read-only.
	}
}

// ToOrganization convert models.User to api.Organization
func ToOrganization(org *models.User) *api.Organization {
	return &api.Organization{
		ID:                        org.ID,
		AvatarURL:                 org.AvatarLink(),
		UserName:                  org.Name,
		FullName:                  org.FullName,
		Description:               org.Description,
		Website:                   org.Website,
		Location:                  org.Location,
		Visibility:                org.Visibility.String(),
		RepoAdminChangeTeamAccess: org.RepoAdminChangeTeamAccess,
	}
}

// ToTeam convert models.Team to api.Team
func ToTeam(team *models.Team) *api.Team {
	if team == nil {
		return nil
	}

	return &api.Team{
		ID:                      team.ID,
		Name:                    team.Name,
		Description:             team.Description,
		IncludesAllRepositories: team.IncludesAllRepositories,
		CanCreateOrgRepo:        team.CanCreateOrgRepo,
		Permission:              team.Authorize.String(),
		Units:                   team.GetUnitNames(),
	}
}

// ToAnnotatedTag convert git.Tag to api.AnnotatedTag
func ToAnnotatedTag(repo *models.Repository, t *git.Tag, c *git.Commit) *api.AnnotatedTag {
	return &api.AnnotatedTag{
		Tag:          t.Name,
		SHA:          t.ID.String(),
		Object:       ToAnnotatedTagObject(repo, c),
		Message:      t.Message,
		URL:          util.URLJoin(repo.APIURL(), "git/tags", t.ID.String()),
		Tagger:       ToCommitUser(t.Tagger),
		Verification: ToVerification(c),
	}
}

// ToAnnotatedTagObject convert a git.Commit to an api.AnnotatedTagObject
func ToAnnotatedTagObject(repo *models.Repository, commit *git.Commit) *api.AnnotatedTagObject {
	return &api.AnnotatedTagObject{
		SHA:  commit.ID.String(),
		Type: string(git.ObjectCommit),
		URL:  util.URLJoin(repo.APIURL(), "git/commits", commit.ID.String()),
	}
}

// ToTopicResponse convert from models.Topic to api.TopicResponse
func ToTopicResponse(topic *models.Topic) *api.TopicResponse {
	return &api.TopicResponse{
		ID:        topic.ID,
		Name:      topic.Name,
		RepoCount: topic.RepoCount,
		Created:   topic.CreatedUnix.AsTime(),
		Updated:   topic.UpdatedUnix.AsTime(),
	}
}

// ToOAuth2Application convert from models.OAuth2Application to api.OAuth2Application
func ToOAuth2Application(app *models.OAuth2Application) *api.OAuth2Application {
	return &api.OAuth2Application{
		ID:           app.ID,
		Name:         app.Name,
		ClientID:     app.ClientID,
		ClientSecret: app.ClientSecret,
		RedirectURIs: app.RedirectURIs,
		Created:      app.CreatedUnix.AsTime(),
	}
}

// ToLFSLock convert a LFSLock to api.LFSLock
func ToLFSLock(l *models.LFSLock) *api.LFSLock {
	return &api.LFSLock{
		ID:       strconv.FormatInt(l.ID, 10),
		Path:     l.Path,
		LockedAt: l.Created.Round(time.Second),
		Owner: &api.LFSLockOwner{
			Name: l.Owner.DisplayName(),
		},
	}
}
