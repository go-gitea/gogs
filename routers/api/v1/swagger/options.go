// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package swagger

import (
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/forms"
)

// not actually a response, just a hack to get go-swagger to include definitions
// of the various XYZOption structs

// parameterBodies
// swagger:response parameterBodies
type swaggerParameterBodies struct {
	// in:body
	AddCollaboratorOption api.AddCollaboratorOption

	// in:body
	CreateEmailOption api.CreateEmailOption
	// in:body
	DeleteEmailOption api.DeleteEmailOption

	// in:body
	CreateHookOption api.CreateHookOption
	// in:body
	EditHookOption api.EditHookOption

	// in:body
	EditGitHookOption api.EditGitHookOption

	// in:body
	CreateIssueOption api.CreateIssueOption
	// in:body
	EditIssueOption api.EditIssueOption
	// in:body
	EditDeadlineOption api.EditDeadlineOption

	// in:body
	CreateIssueCommentOption api.CreateIssueCommentOption
	// in:body
	EditIssueCommentOption api.EditIssueCommentOption

	// in:body
	IssueLabelsOption api.IssueLabelsOption

	// in:body
	CreateKeyOption api.CreateKeyOption

	// in:body
	CreateLabelOption api.CreateLabelOption
	// in:body
	EditLabelOption api.EditLabelOption

	// in:body
	MarkdownOption api.MarkdownOption

	// in:body
	CreateMilestoneOption api.CreateMilestoneOption
	// in:body
	EditMilestoneOption api.EditMilestoneOption

	// in:body
	CreateOrgOption api.CreateOrgOption
	// in:body
	EditOrgOption api.EditOrgOption

	// in:body
	CreatePullRequestOption api.CreatePullRequestOption
	// in:body
	EditPullRequestOption api.EditPullRequestOption
	// in:body
	MergePullRequestOption forms.MergePullRequestForm

	// in:body
	CreateReleaseOption api.CreateReleaseOption
	// in:body
	EditReleaseOption api.EditReleaseOption

	// in:body
	CreateRepoOption api.CreateRepoOption
	// in:body
	EditRepoOption api.EditRepoOption
	// in:body
	TransferRepoOption api.TransferRepoOption
	// in:body
	CreateForkOption api.CreateForkOption
	// in:body
	GenerateRepoOption api.GenerateRepoOption

	// in:body
	CreateStatusOption api.CreateStatusOption

	// in:body
	CreateTeamOption api.CreateTeamOption
	// in:body
	EditTeamOption api.EditTeamOption

	// in:body
	AddTimeOption api.AddTimeOption

	// in:body
	CreateUserOption api.CreateUserOption

	// in:body
	EditUserOption api.EditUserOption

	// in:body
	MigrateRepoForm forms.MigrateRepoForm

	// in:body
	EditAttachmentOptions api.EditAttachmentOptions

	// in:body
	CreateFileOptions api.CreateFileOptions

	// in:body
	UpdateFileOptions api.UpdateFileOptions

	// in:body
	DeleteFileOptions api.DeleteFileOptions

	// in:body
	CommitDateOptions api.CommitDateOptions

	// in:body
	RepoTopicOptions api.RepoTopicOptions

	// in:body
	EditReactionOption api.EditReactionOption

	// in:body
	CreateBranchRepoOption api.CreateBranchRepoOption

	// in:body
	CreateBranchProtectionOption api.CreateBranchProtectionOption

	// in:body
	EditBranchProtectionOption api.EditBranchProtectionOption

	// in:body
	CreateOAuth2ApplicationOptions api.CreateOAuth2ApplicationOptions

	// in:body
	CreatePullReviewOptions api.CreatePullReviewOptions

	// in:body
	CreatePullReviewComment api.CreatePullReviewComment

	// in:body
	SubmitPullReviewOptions api.SubmitPullReviewOptions

	// in:body
	DismissPullReviewOptions api.DismissPullReviewOptions

	// in:body
	MigrateRepoOptions api.MigrateRepoOptions

	// in:body
	PullReviewRequestOptions api.PullReviewRequestOptions

	// in:body
	CreateTagOption api.CreateTagOption

	// in:body
	UserSettingsOptions api.UserSettingsOptions
}
