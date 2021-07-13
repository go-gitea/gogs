// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/upload"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/forms"
	releaseservice "code.gitea.io/gitea/services/release"
)

const (
	tplReleases   base.TplName = "repo/release/list"
	tplReleaseNew base.TplName = "repo/release/new"
)

// calReleaseNumCommitsBehind calculates given release has how many commits behind release target.
func calReleaseNumCommitsBehind(repoCtx *context.Repository, release *models.Release, countCache map[string]int64) error {
	// Fast return if release target is same as default branch.
	if repoCtx.BranchName == release.Target {
		release.NumCommitsBehind = repoCtx.CommitsCount - release.NumCommits
		return nil
	}

	// Get count if not exists
	if _, ok := countCache[release.Target]; !ok {
		if repoCtx.GitRepo.IsBranchExist(release.Target) {
			commit, err := repoCtx.GitRepo.GetBranchCommit(release.Target)
			if err != nil {
				return fmt.Errorf("GetBranchCommit: %v", err)
			}
			countCache[release.Target], err = commit.CommitsCount()
			if err != nil {
				return fmt.Errorf("CommitsCount: %v", err)
			}
		} else {
			// Use NumCommits of the newest release on that target
			countCache[release.Target] = release.NumCommits
		}
	}
	release.NumCommitsBehind = countCache[release.Target] - release.NumCommits
	return nil
}

// Releases render releases list page
func Releases(ctx *context.Context) {
	releasesOrTags(ctx, false)
}

// TagsList render tags list page
func TagsList(ctx *context.Context) {
	releasesOrTags(ctx, true)
}

func releasesOrTags(ctx *context.Context, isTagList bool) {
	ctx.Data["PageIsReleaseList"] = true
	ctx.Data["DefaultBranch"] = ctx.Repo.Repository.DefaultBranch
	ctx.Data["IsViewBranch"] = false
	ctx.Data["IsViewTag"] = true
	// Disable the showCreateNewBranch form in the dropdown on this page.
	ctx.Data["CanCreateBranch"] = false
	ctx.Data["HideBranchesInDropdown"] = true

	if isTagList {
		ctx.Data["Title"] = ctx.Tr("repo.release.tags")
		ctx.Data["PageIsTagList"] = true
	} else {
		ctx.Data["Title"] = ctx.Tr("repo.release.releases")
		ctx.Data["PageIsTagList"] = false
	}

	tags, err := ctx.Repo.GitRepo.GetTags()
	if err != nil {
		ctx.ServerError("GetTags", err)
		return
	}
	ctx.Data["Tags"] = tags

	writeAccess := ctx.Repo.CanWrite(models.UnitTypeReleases)
	ctx.Data["CanCreateRelease"] = writeAccess && !ctx.Repo.Repository.IsArchived

	opts := models.FindReleasesOptions{
		ListOptions: models.ListOptions{
			Page:     ctx.QueryInt("page"),
			PageSize: convert.ToCorrectPageSize(ctx.QueryInt("limit")),
		},
		IncludeDrafts: writeAccess && !isTagList,
		IncludeTags:   isTagList,
	}

	releases, err := models.GetReleasesByRepoID(ctx.Repo.Repository.ID, opts)
	if err != nil {
		ctx.ServerError("GetReleasesByRepoID", err)
		return
	}

	count, err := models.GetReleaseCountByRepoID(ctx.Repo.Repository.ID, opts)
	if err != nil {
		ctx.ServerError("GetReleaseCountByRepoID", err)
		return
	}

	if err = models.GetReleaseAttachments(releases...); err != nil {
		ctx.ServerError("GetReleaseAttachments", err)
		return
	}

	// Temporary cache commits count of used branches to speed up.
	countCache := make(map[string]int64)
	cacheUsers := make(map[int64]*models.User)
	if ctx.User != nil {
		cacheUsers[ctx.User.ID] = ctx.User
	}
	var ok bool

	for _, r := range releases {
		if r.Publisher, ok = cacheUsers[r.PublisherID]; !ok {
			r.Publisher, err = models.GetUserByID(r.PublisherID)
			if err != nil {
				if models.IsErrUserNotExist(err) {
					r.Publisher = models.NewGhostUser()
				} else {
					ctx.ServerError("GetUserByID", err)
					return
				}
			}
			cacheUsers[r.PublisherID] = r.Publisher
		}

		r.Note, err = markdown.RenderString(&markup.RenderContext{
			URLPrefix: ctx.Repo.RepoLink,
			Metas:     ctx.Repo.Repository.ComposeMetas(),
			GitRepo:   ctx.Repo.GitRepo,
		}, r.Note)
		if err != nil {
			ctx.ServerError("RenderString", err)
			return
		}

		if r.IsDraft {
			continue
		}

		if err := calReleaseNumCommitsBehind(ctx.Repo, r, countCache); err != nil {
			ctx.ServerError("calReleaseNumCommitsBehind", err)
			return
		}
	}

	ctx.Data["Releases"] = releases
	ctx.Data["ReleasesNum"] = len(releases)

	pager := context.NewPagination(int(count), opts.PageSize, opts.Page, 5)
	pager.SetDefaultParams(ctx)
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplReleases)
}

// SingleRelease renders a single release's page
func SingleRelease(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.release.releases")
	ctx.Data["PageIsReleaseList"] = true

	writeAccess := ctx.Repo.CanWrite(models.UnitTypeReleases)
	ctx.Data["CanCreateRelease"] = writeAccess && !ctx.Repo.Repository.IsArchived

	release, err := models.GetRelease(ctx.Repo.Repository.ID, ctx.Params("*"))
	if err != nil {
		if models.IsErrReleaseNotExist(err) {
			ctx.NotFound("GetRelease", err)
			return
		}
		ctx.ServerError("GetReleasesByRepoID", err)
		return
	}

	err = models.GetReleaseAttachments(release)
	if err != nil {
		ctx.ServerError("GetReleaseAttachments", err)
		return
	}

	release.Publisher, err = models.GetUserByID(release.PublisherID)
	if err != nil {
		if models.IsErrUserNotExist(err) {
			release.Publisher = models.NewGhostUser()
		} else {
			ctx.ServerError("GetUserByID", err)
			return
		}
	}
	if !release.IsDraft {
		if err := calReleaseNumCommitsBehind(ctx.Repo, release, make(map[string]int64)); err != nil {
			ctx.ServerError("calReleaseNumCommitsBehind", err)
			return
		}
	}
	release.Note, err = markdown.RenderString(&markup.RenderContext{
		URLPrefix: ctx.Repo.RepoLink,
		Metas:     ctx.Repo.Repository.ComposeMetas(),
		GitRepo:   ctx.Repo.GitRepo,
	}, release.Note)
	if err != nil {
		ctx.ServerError("RenderString", err)
		return
	}

	ctx.Data["Releases"] = []*models.Release{release}
	ctx.HTML(http.StatusOK, tplReleases)
}

// LatestRelease redirects to the latest release
func LatestRelease(ctx *context.Context) {
	release, err := models.GetLatestReleaseByRepoID(ctx.Repo.Repository.ID)
	if err != nil {
		if models.IsErrReleaseNotExist(err) {
			ctx.NotFound("LatestRelease", err)
			return
		}
		ctx.ServerError("GetLatestReleaseByRepoID", err)
		return
	}

	if err := release.LoadAttributes(); err != nil {
		ctx.ServerError("LoadAttributes", err)
		return
	}

	ctx.Redirect(release.HTMLURL())
}

// NewRelease render creating or edit release page
func NewRelease(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.release.new_release")
	ctx.Data["PageIsReleaseList"] = true
	ctx.Data["RequireSimpleMDE"] = true
	ctx.Data["RequireTribute"] = true
	ctx.Data["tag_target"] = ctx.Repo.Repository.DefaultBranch
	if tagName := ctx.Query("tag"); len(tagName) > 0 {
		rel, err := models.GetRelease(ctx.Repo.Repository.ID, tagName)
		if err != nil && !models.IsErrReleaseNotExist(err) {
			ctx.ServerError("GetRelease", err)
			return
		}

		if rel != nil {
			rel.Repo = ctx.Repo.Repository
			if err := rel.LoadAttributes(); err != nil {
				ctx.ServerError("LoadAttributes", err)
				return
			}

			ctx.Data["tag_name"] = rel.TagName
			ctx.Data["tag_target"] = rel.Target
			ctx.Data["title"] = rel.Title
			ctx.Data["content"] = rel.Note
			ctx.Data["attachments"] = rel.Attachments
		}
	}
	ctx.Data["IsAttachmentEnabled"] = setting.Attachment.Enabled
	upload.AddUploadContext(ctx, "release")
	ctx.HTML(http.StatusOK, tplReleaseNew)
}

// NewReleasePost response for creating a release
func NewReleasePost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.NewReleaseForm)
	ctx.Data["Title"] = ctx.Tr("repo.release.new_release")
	ctx.Data["PageIsReleaseList"] = true
	ctx.Data["RequireSimpleMDE"] = true
	ctx.Data["RequireTribute"] = true

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplReleaseNew)
		return
	}

	if !ctx.Repo.GitRepo.IsBranchExist(form.Target) {
		ctx.RenderWithErr(ctx.Tr("form.target_branch_not_exist"), tplReleaseNew, &form)
		return
	}

	var attachmentUUIDs []string
	if setting.Attachment.Enabled {
		attachmentUUIDs = form.Files
	}

	rel, err := models.GetRelease(ctx.Repo.Repository.ID, form.TagName)
	if err != nil {
		if !models.IsErrReleaseNotExist(err) {
			ctx.ServerError("GetRelease", err)
			return
		}

		msg := ""
		if len(form.Title) > 0 && form.AddTagMsg {
			msg = form.Title + "\n\n" + form.Content
		}

		if len(form.TagOnly) > 0 {
			if err = releaseservice.CreateNewTag(ctx.User, ctx.Repo.Repository, form.Target, form.TagName, msg); err != nil {
				if models.IsErrTagAlreadyExists(err) {
					e := err.(models.ErrTagAlreadyExists)
					ctx.Flash.Error(ctx.Tr("repo.branch.tag_collision", e.TagName))
					ctx.Redirect(ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL())
					return
				}

				if models.IsErrInvalidTagName(err) {
					ctx.Flash.Error(ctx.Tr("repo.release.tag_name_invalid"))
					ctx.Redirect(ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL())
					return
				}

				if models.IsErrProtectedTagName(err) {
					ctx.Flash.Error(ctx.Tr("repo.release.tag_name_protected"))
					ctx.Redirect(ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL())
					return
				}

				ctx.ServerError("releaseservice.CreateNewTag", err)
				return
			}

			ctx.Flash.Success(ctx.Tr("repo.tag.create_success", form.TagName))
			ctx.Redirect(ctx.Repo.RepoLink + "/src/tag/" + form.TagName)
			return
		}

		rel = &models.Release{
			RepoID:       ctx.Repo.Repository.ID,
			Repo:         ctx.Repo.Repository,
			PublisherID:  ctx.User.ID,
			Publisher:    ctx.User,
			Title:        form.Title,
			TagName:      form.TagName,
			Target:       form.Target,
			Note:         form.Content,
			IsDraft:      len(form.Draft) > 0,
			IsPrerelease: form.Prerelease,
			IsTag:        false,
		}

		if err = releaseservice.CreateRelease(ctx.Repo.GitRepo, rel, attachmentUUIDs, msg); err != nil {
			ctx.Data["Err_TagName"] = true
			switch {
			case models.IsErrReleaseAlreadyExist(err):
				ctx.RenderWithErr(ctx.Tr("repo.release.tag_name_already_exist"), tplReleaseNew, &form)
			case models.IsErrInvalidTagName(err):
				ctx.RenderWithErr(ctx.Tr("repo.release.tag_name_invalid"), tplReleaseNew, &form)
			case models.IsErrProtectedTagName(err):
				ctx.RenderWithErr(ctx.Tr("repo.release.tag_name_protected"), tplReleaseNew, &form)
			default:
				ctx.ServerError("CreateRelease", err)
			}
			return
		}
	} else {
		if !rel.IsTag {
			ctx.Data["Err_TagName"] = true
			ctx.RenderWithErr(ctx.Tr("repo.release.tag_name_already_exist"), tplReleaseNew, &form)
			return
		}

		rel.Title = form.Title
		rel.Note = form.Content
		rel.Target = form.Target
		rel.IsDraft = len(form.Draft) > 0
		rel.IsPrerelease = form.Prerelease
		rel.PublisherID = ctx.User.ID
		rel.IsTag = false

		if err = releaseservice.UpdateRelease(ctx.User, ctx.Repo.GitRepo, rel, attachmentUUIDs, nil, nil); err != nil {
			ctx.Data["Err_TagName"] = true
			ctx.ServerError("UpdateRelease", err)
			return
		}
	}
	log.Trace("Release created: %s/%s:%s", ctx.User.LowerName, ctx.Repo.Repository.Name, form.TagName)

	ctx.Redirect(ctx.Repo.RepoLink + "/releases")
}

// EditRelease render release edit page
func EditRelease(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.release.edit_release")
	ctx.Data["PageIsReleaseList"] = true
	ctx.Data["PageIsEditRelease"] = true
	ctx.Data["RequireSimpleMDE"] = true
	ctx.Data["RequireTribute"] = true
	ctx.Data["IsAttachmentEnabled"] = setting.Attachment.Enabled
	upload.AddUploadContext(ctx, "release")

	tagName := ctx.Params("*")
	rel, err := models.GetRelease(ctx.Repo.Repository.ID, tagName)
	if err != nil {
		if models.IsErrReleaseNotExist(err) {
			ctx.NotFound("GetRelease", err)
		} else {
			ctx.ServerError("GetRelease", err)
		}
		return
	}
	ctx.Data["ID"] = rel.ID
	ctx.Data["tag_name"] = rel.TagName
	ctx.Data["tag_target"] = rel.Target
	ctx.Data["title"] = rel.Title
	ctx.Data["content"] = rel.Note
	ctx.Data["prerelease"] = rel.IsPrerelease
	ctx.Data["IsDraft"] = rel.IsDraft

	rel.Repo = ctx.Repo.Repository
	if err := rel.LoadAttributes(); err != nil {
		ctx.ServerError("LoadAttributes", err)
		return
	}
	ctx.Data["attachments"] = rel.Attachments

	ctx.HTML(http.StatusOK, tplReleaseNew)
}

// EditReleasePost response for edit release
func EditReleasePost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.EditReleaseForm)
	ctx.Data["Title"] = ctx.Tr("repo.release.edit_release")
	ctx.Data["PageIsReleaseList"] = true
	ctx.Data["PageIsEditRelease"] = true
	ctx.Data["RequireSimpleMDE"] = true
	ctx.Data["RequireTribute"] = true

	tagName := ctx.Params("*")
	rel, err := models.GetRelease(ctx.Repo.Repository.ID, tagName)
	if err != nil {
		if models.IsErrReleaseNotExist(err) {
			ctx.NotFound("GetRelease", err)
		} else {
			ctx.ServerError("GetRelease", err)
		}
		return
	}
	if rel.IsTag {
		ctx.NotFound("GetRelease", err)
		return
	}
	ctx.Data["tag_name"] = rel.TagName
	ctx.Data["tag_target"] = rel.Target
	ctx.Data["title"] = rel.Title
	ctx.Data["content"] = rel.Note
	ctx.Data["prerelease"] = rel.IsPrerelease

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplReleaseNew)
		return
	}

	const delPrefix = "attachment-del-"
	const editPrefix = "attachment-edit-"
	var addAttachmentUUIDs, delAttachmentUUIDs []string
	var editAttachments = make(map[string]string) // uuid -> new name
	if setting.Attachment.Enabled {
		addAttachmentUUIDs = form.Files
		for k, v := range ctx.Req.Form {
			if strings.HasPrefix(k, delPrefix) && v[0] == "true" {
				delAttachmentUUIDs = append(delAttachmentUUIDs, k[len(delPrefix):])
			} else if strings.HasPrefix(k, editPrefix) {
				editAttachments[k[len(editPrefix):]] = v[0]
			}
		}
	}

	rel.Title = form.Title
	rel.Note = form.Content
	rel.IsDraft = len(form.Draft) > 0
	rel.IsPrerelease = form.Prerelease
	if err = releaseservice.UpdateRelease(ctx.User, ctx.Repo.GitRepo,
		rel, addAttachmentUUIDs, delAttachmentUUIDs, editAttachments); err != nil {
		ctx.ServerError("UpdateRelease", err)
		return
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/releases")
}

// DeleteRelease delete a release
func DeleteRelease(ctx *context.Context) {
	deleteReleaseOrTag(ctx, false)
}

// DeleteTag delete a tag
func DeleteTag(ctx *context.Context) {
	deleteReleaseOrTag(ctx, true)
}

func deleteReleaseOrTag(ctx *context.Context, isDelTag bool) {
	if err := releaseservice.DeleteReleaseByID(ctx.QueryInt64("id"), ctx.User, isDelTag); err != nil {
		ctx.Flash.Error("DeleteReleaseByID: " + err.Error())
	} else {
		if isDelTag {
			ctx.Flash.Success(ctx.Tr("repo.release.deletion_tag_success"))
		} else {
			ctx.Flash.Success(ctx.Tr("repo.release.deletion_success"))
		}
	}

	if isDelTag {
		ctx.JSON(http.StatusOK, map[string]interface{}{
			"redirect": ctx.Repo.RepoLink + "/tags",
		})
		return
	}

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"redirect": ctx.Repo.RepoLink + "/releases",
	})
}
