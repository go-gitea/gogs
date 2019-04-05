// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"fmt"
	"path"
	"strings"

	"github.com/Unknwon/paginater"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/repo"
)

const (
	tplFollowers base.TplName = "user/meta/followers"
	tplStars     base.TplName = "user/meta/stars"
)

// GetUserByName get user by name
func GetUserByName(ctx *context.Context, name string) *models.User {
	user, err := models.GetUserByName(name)
	if err != nil {
		if models.IsErrUserNotExist(err) {
			ctx.NotFound("GetUserByName", nil)
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
	} else if strings.HasSuffix(uname, ".png") {
		ctx.Error(404)
		return
	}

	isShowKeys := false
	if strings.HasSuffix(uname, ".keys") {
		isShowKeys = true
	}

	ctxUser := GetUserByName(ctx, strings.TrimSuffix(uname, ".keys"))
	if ctx.Written() {
		return
	}

	// Show SSH keys.
	if isShowKeys {
		ShowSSHKeys(ctx, ctxUser.ID)
		return
	}

	if ctxUser.IsOrganization() {
		showOrgProfile(ctx)
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
	ctx.Data["EnableHeatmap"] = setting.Service.EnableUserHeatmap
	ctx.Data["HeatmapUser"] = ctxUser.Name
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
	case "activity":
		retrieveFeeds(ctx, models.GetFeedsOptions{RequestedUser: ctxUser,
			IncludePrivate:  showPrivate,
			OnlyPerformedBy: true,
			IncludeDeleted:  false,
		})
		if ctx.Written() {
			return
		}
	case "stars":
		ctx.Data["PageIsProfileStarList"] = true
		if len(keyword) == 0 {
			repos, err = ctxUser.GetStarredRepos(showPrivate, page, setting.UI.User.RepoPagingNum, orderBy.String())
			if err != nil {
				ctx.ServerError("GetStarredRepos", err)
				return
			}

			count, err = ctxUser.GetStarredRepoCount(showPrivate)
			if err != nil {
				ctx.ServerError("GetStarredRepoCount", err)
				return
			}
		} else {
			repos, count, err = models.SearchRepositoryByName(&models.SearchRepoOptions{
				Keyword:     keyword,
				OwnerID:     ctxUser.ID,
				OrderBy:     orderBy,
				Private:     showPrivate,
				Page:        page,
				PageSize:    setting.UI.User.RepoPagingNum,
				Starred:     true,
				Collaborate: util.OptionalBoolFalse,
				TopicOnly:   topicOnly,
			})
			if err != nil {
				ctx.ServerError("SearchRepositoryByName", err)
				return
			}
		}

		ctx.Data["Repos"] = repos
		ctx.Data["Page"] = paginater.New(int(count), setting.UI.User.RepoPagingNum, page, 5)
		ctx.Data["Total"] = count
	default:
		if len(keyword) == 0 {
			var total int
			repos, err = models.GetUserRepositories(ctxUser.ID, showPrivate, page, setting.UI.User.RepoPagingNum, orderBy.String())
			if err != nil {
				ctx.ServerError("GetRepositories", err)
				return
			}
			models.RepositoryList(repos).LoadAttributes()
			ctx.Data["Repos"] = repos

			if showPrivate {
				total = ctxUser.NumRepos
			} else {
				count, err := models.GetPublicRepositoryCount(ctxUser)
				if err != nil {
					ctx.ServerError("GetPublicRepositoryCount", err)
					return
				}
				total = int(count)
			}

			ctx.Data["Page"] = paginater.New(total, setting.UI.User.RepoPagingNum, page, 5)
			ctx.Data["Total"] = total
		} else {
			repos, count, err = models.SearchRepositoryByName(&models.SearchRepoOptions{
				Keyword:     keyword,
				OwnerID:     ctxUser.ID,
				OrderBy:     orderBy,
				Private:     showPrivate,
				Page:        page,
				IsProfile:   true,
				PageSize:    setting.UI.User.RepoPagingNum,
				Collaborate: util.OptionalBoolFalse,
				TopicOnly:   topicOnly,
			})
			if err != nil {
				ctx.ServerError("SearchRepositoryByName", err)
				return
			}

			ctx.Data["Repos"] = repos
			ctx.Data["Page"] = paginater.New(int(count), setting.UI.User.RepoPagingNum, page, 5)
			ctx.Data["Total"] = count
		}
	}

	ctx.Data["ShowUserEmail"] = len(ctxUser.Email) > 0 && ctx.IsSigned && (!ctxUser.KeepEmailPrivate || ctxUser.ID == ctx.User.ID)

	ctx.HTML(200, tplProfile)
}

// Followers render user's followers page
func Followers(ctx *context.Context) {
	u := GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	ctx.Data["Title"] = u.DisplayName()
	ctx.Data["CardsTitle"] = ctx.Tr("user.followers")
	ctx.Data["PageIsFollowers"] = true
	ctx.Data["Owner"] = u
	repo.RenderUserCards(ctx, u.NumFollowers, u.GetFollowers, tplFollowers)
}

// Following render user's followering page
func Following(ctx *context.Context) {
	u := GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	ctx.Data["Title"] = u.DisplayName()
	ctx.Data["CardsTitle"] = ctx.Tr("user.following")
	ctx.Data["PageIsFollowing"] = true
	ctx.Data["Owner"] = u
	repo.RenderUserCards(ctx, u.NumFollowing, u.GetFollowing, tplFollowers)
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
