// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"github.com/Unknwon/com"

	api "code.gitea.io/sdk/gitea"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
)

// Search search users
func Search(ctx *context.APIContext) {
	opts := &models.SearchUserOptions{
		Keyword:  ctx.Query("q"),
		Type:     models.UserTypeIndividual,
		PageSize: com.StrTo(ctx.Query("limit")).MustInt(),
	}
	if opts.PageSize == 0 {
		opts.PageSize = 10
	}

	users, _, err := models.SearchUserByName(opts)
	if err != nil {
		ctx.JSON(500, map[string]interface{}{
			"ok":    false,
			"error": err.Error(),
		})
		return
	}

	results := make([]*api.User, len(users))
	for i := range users {
		results[i] = &api.User{
			ID:        users[i].ID,
			UserName:  users[i].Name,
			AvatarUrl: users[i].AvatarLink(),
			FullName:  users[i].FullName,
		}
		if ctx.IsSigned {
			results[i].Email = users[i].Email
		}
	}

	ctx.JSON(200, map[string]interface{}{
		"ok":   true,
		"data": results,
	})
}

// GetInfo get user's information
func GetInfo(ctx *context.APIContext) {
	u, err := models.GetUserByName(ctx.Params(":username"))
	if err != nil {
		if models.IsErrUserNotExist(err) {
			ctx.Status(404)
		} else {
			ctx.Error(500, "GetUserByName", err)
		}
		return
	}

	// Hide user e-mail when API caller isn't signed in.
	if !ctx.IsSigned {
		u.Email = ""
	}
	ctx.JSON(200, u.APIFormat())
}

// GetAuthenticatedUser get curent user's information
func GetAuthenticatedUser(ctx *context.APIContext) {
	ctx.JSON(200, ctx.User.APIFormat())
}
