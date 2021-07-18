// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/web/org"
	"code.gitea.io/gitea/routers/web/rss"

	"github.com/gorilla/feeds"
)

// GetUserByName get user by name
func GetUserByName(ctx *context.Context, name string) *models.User {
	user, err := models.GetUserByName(name)
	if err != nil {
		if models.IsErrUserNotExist(err) {
			if redirectUserID, err := models.LookupUserRedirect(name); err == nil {
				context.RedirectToUser(ctx, name, redirectUserID)
			} else {
				ctx.NotFound("GetUserByName", err)
			}
		} else {
			ctx.ServerError("GetUserByName", err)
		}
		return nil
	}
	return user
}

// GetUserByParams returns user whose name is presented in URL paramenter.
func GetUserByParams(ctx *context.Context) *models.User {
	return GetUserByName(ctx, ctx.Params(":username"))
}

// Profile render user's profile page
func Profile(ctx *context.Context) {
	uname := ctx.Params(":username")

	// Special handle for FireFox requests favicon.ico.
	if uname == "favicon.ico" {
		ctx.ServeFile(path.Join(setting.StaticRootPath, "public/img/favicon.png"))
		return
	}

	if strings.HasSuffix(uname, ".png") {
		ctx.Error(http.StatusNotFound)
		return
	}

	isShowKeys := false
	if strings.HasSuffix(uname, ".keys") {
		isShowKeys = true
		uname = strings.TrimSuffix(uname, ".keys")
	}

	isShowGPG := false
	if strings.HasSuffix(uname, ".gpg") {
		isShowGPG = true
		uname = strings.TrimSuffix(uname, ".gpg")
	}

	isShowRSS := false
	if strings.HasSuffix(uname, ".rss") {
		isShowRSS = true
		uname = strings.TrimSuffix(uname, ".rss")
	}

	ctxUser := GetUserByName(ctx, uname)
	if ctx.Written() {
		return
	}

	if ctxUser.IsOrganization() {
		org.Home(ctx)
		return
	}

	// check view permissions
	if !ctxUser.IsVisibleToUser(ctx.User) {
		ctx.NotFound("user", fmt.Errorf(uname))
		return
	}

	// Show SSH keys.
	if isShowKeys {
		ShowSSHKeys(ctx, ctxUser.ID)
		return
	}

	// Show GPG keys.
	if isShowGPG {
		ShowGPGKeys(ctx, ctxUser.ID)
		return
	}

	// Show User RSS feed
	if isShowRSS {
		ShowRSS(ctx, ctxUser)
		return
	}

	// Show OpenID URIs
	openIDs, err := models.GetUserOpenIDs(ctxUser.ID)
	if err != nil {
		ctx.ServerError("GetUserOpenIDs", err)
		return
	}

	ctx.Data["Title"] = ctxUser.DisplayName()
	ctx.Data["PageIsUserProfile"] = true
	ctx.Data["Owner"] = ctxUser
	ctx.Data["OpenIDs"] = openIDs

	if setting.Service.EnableUserHeatmap {
		data, err := models.GetUserHeatmapDataByUser(ctxUser, ctx.User)
		if err != nil {
			ctx.ServerError("GetUserHeatmapDataByUser", err)
			return
		}
		ctx.Data["HeatmapData"] = data
	}

	if len(ctxUser.Description) != 0 {
		content, err := markdown.RenderString(&markup.RenderContext{
			URLPrefix: ctx.Repo.RepoLink,
			Metas:     map[string]string{"mode": "document"},
			GitRepo:   ctx.Repo.GitRepo,
		}, ctxUser.Description)
		if err != nil {
			ctx.ServerError("RenderString", err)
			return
		}
		ctx.Data["RenderedDescription"] = content
	}

	showPrivate := ctx.IsSigned && (ctx.User.IsAdmin || ctx.User.ID == ctxUser.ID)

	orgs, err := models.GetOrgsByUserID(ctxUser.ID, showPrivate)
	if err != nil {
		ctx.ServerError("GetOrgsByUserIDDesc", err)
		return
	}

	ctx.Data["Orgs"] = orgs
	ctx.Data["HasOrgsVisible"] = models.HasOrgsVisible(orgs, ctx.User)

	tab := ctx.Query("tab")
	ctx.Data["TabName"] = tab

	page := ctx.QueryInt("page")
	if page <= 0 {
		page = 1
	}

	topicOnly := ctx.QueryBool("topic")

	var (
		repos   []*models.Repository
		count   int64
		total   int
		orderBy models.SearchOrderBy
	)

	ctx.Data["SortType"] = ctx.Query("sort")
	switch ctx.Query("sort") {
	case "newest":
		orderBy = models.SearchOrderByNewest
	case "oldest":
		orderBy = models.SearchOrderByOldest
	case "recentupdate":
		orderBy = models.SearchOrderByRecentUpdated
	case "leastupdate":
		orderBy = models.SearchOrderByLeastUpdated
	case "reversealphabetically":
		orderBy = models.SearchOrderByAlphabeticallyReverse
	case "alphabetically":
		orderBy = models.SearchOrderByAlphabetically
	case "moststars":
		orderBy = models.SearchOrderByStarsReverse
	case "feweststars":
		orderBy = models.SearchOrderByStars
	case "mostforks":
		orderBy = models.SearchOrderByForksReverse
	case "fewestforks":
		orderBy = models.SearchOrderByForks
	default:
		ctx.Data["SortType"] = "recentupdate"
		orderBy = models.SearchOrderByRecentUpdated
	}

	keyword := strings.Trim(ctx.Query("q"), " ")
	ctx.Data["Keyword"] = keyword
	switch tab {
	case "followers":
		items, err := ctxUser.GetFollowers(models.ListOptions{
			PageSize: setting.UI.User.RepoPagingNum,
			Page:     page,
		})
		if err != nil {
			ctx.ServerError("GetFollowers", err)
			return
		}
		ctx.Data["Cards"] = items

		total = ctxUser.NumFollowers
	case "following":
		items, err := ctxUser.GetFollowing(models.ListOptions{
			PageSize: setting.UI.User.RepoPagingNum,
			Page:     page,
		})
		if err != nil {
			ctx.ServerError("GetFollowing", err)
			return
		}
		ctx.Data["Cards"] = items

		total = ctxUser.NumFollowing
	case "activity":
		actions := retrieveFeeds(ctx, models.GetFeedsOptions{RequestedUser: ctxUser,
			Actor:           ctx.User,
			IncludePrivate:  showPrivate,
			OnlyPerformedBy: true,
			IncludeDeleted:  false,
			Date:            ctx.Query("date"),
		})
		if ctx.Written() {
			return
		}
		ctx.Data["Feeds"] = actions
	case "stars":
		ctx.Data["PageIsProfileStarList"] = true
		repos, count, err = models.SearchRepository(&models.SearchRepoOptions{
			ListOptions: models.ListOptions{
				PageSize: setting.UI.User.RepoPagingNum,
				Page:     page,
			},
			Actor:              ctx.User,
			Keyword:            keyword,
			OrderBy:            orderBy,
			Private:            ctx.IsSigned,
			StarredByID:        ctxUser.ID,
			Collaborate:        util.OptionalBoolFalse,
			TopicOnly:          topicOnly,
			IncludeDescription: setting.UI.SearchRepoDescription,
		})
		if err != nil {
			ctx.ServerError("SearchRepository", err)
			return
		}

		total = int(count)
	case "projects":
		ctx.Data["OpenProjects"], _, err = models.GetProjects(models.ProjectSearchOptions{
			Page:     -1,
			IsClosed: util.OptionalBoolFalse,
			Type:     models.ProjectTypeIndividual,
		})
		if err != nil {
			ctx.ServerError("GetProjects", err)
			return
		}
	case "watching":
		repos, count, err = models.SearchRepository(&models.SearchRepoOptions{
			ListOptions: models.ListOptions{
				PageSize: setting.UI.User.RepoPagingNum,
				Page:     page,
			},
			Actor:              ctx.User,
			Keyword:            keyword,
			OrderBy:            orderBy,
			Private:            ctx.IsSigned,
			WatchedByID:        ctxUser.ID,
			Collaborate:        util.OptionalBoolFalse,
			TopicOnly:          topicOnly,
			IncludeDescription: setting.UI.SearchRepoDescription,
		})
		if err != nil {
			ctx.ServerError("SearchRepository", err)
			return
		}

		total = int(count)
	default:
		repos, count, err = models.SearchRepository(&models.SearchRepoOptions{
			ListOptions: models.ListOptions{
				PageSize: setting.UI.User.RepoPagingNum,
				Page:     page,
			},
			Actor:              ctx.User,
			Keyword:            keyword,
			OwnerID:            ctxUser.ID,
			OrderBy:            orderBy,
			Private:            ctx.IsSigned,
			Collaborate:        util.OptionalBoolFalse,
			TopicOnly:          topicOnly,
			IncludeDescription: setting.UI.SearchRepoDescription,
		})
		if err != nil {
			ctx.ServerError("SearchRepository", err)
			return
		}

		total = int(count)
	}
	ctx.Data["Repos"] = repos
	ctx.Data["Total"] = total

	pager := context.NewPagination(total, setting.UI.User.RepoPagingNum, page, 5)
	pager.SetDefaultParams(ctx)
	ctx.Data["Page"] = pager

	ctx.Data["ShowUserEmail"] = len(ctxUser.Email) > 0 && ctx.IsSigned && (!ctxUser.KeepEmailPrivate || ctxUser.ID == ctx.User.ID)

	ctx.HTML(http.StatusOK, tplProfile)
}

// ShowRSS show user activity as RSS feed
func ShowRSS(ctx *context.Context, ctxUser *models.User) {
	actions := retrieveFeeds(ctx, models.GetFeedsOptions{RequestedUser: ctxUser,
		Actor:           ctx.User,
		IncludePrivate:  false,
		OnlyPerformedBy: true,
		IncludeDeleted:  false,
		Date:            ctx.Query("date"),
	})
	if ctx.Written() {
		return
	}

	now := time.Now()
	feed := &feeds.Feed{
		Title:       ctx.Tr("home.profile_of", ctxUser.DisplayName()),
		Link:        &feeds.Link{Href: ctxUser.HTMLURL()},
		Description: ctxUser.Description,
		Created:     now,
	}

	feed.Items = rss.FeedActionsToFeedItems(ctx, actions)

	//atom, err := feed.ToAtom()
	if rss, err := feed.ToRss(); err != nil {
		ctx.ServerError("ToRss", err)
	} else {
		ctx.PlainText(http.StatusOK, []byte(rss))
	}

}

// Action response for follow/unfollow user request
func Action(ctx *context.Context) {
	u := GetUserByParams(ctx)
	if ctx.Written() {
		return
	}

	var err error
	switch ctx.Params(":action") {
	case "follow":
		err = models.FollowUser(ctx.User.ID, u.ID)
	case "unfollow":
		err = models.UnfollowUser(ctx.User.ID, u.ID)
	}

	if err != nil {
		ctx.ServerError(fmt.Sprintf("Action (%s)", ctx.Params(":action")), err)
		return
	}

	ctx.RedirectToFirst(ctx.Query("redirect_to"), u.HomeLink())
}
