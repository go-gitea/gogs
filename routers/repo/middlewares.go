// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
)

// SetEditorconfigIfExists set editor config as render variable
func SetEditorconfigIfExists(ctx *context.Context) {
	if ctx.Repo.Repository.IsEmpty {
		ctx.Data["Editorconfig"] = nil
		return
	}

	ec, err := ctx.Repo.GetEditorconfig()

	if err != nil && !git.IsErrNotExist(err) {
		description := fmt.Sprintf("Error while getting .editorconfig file: %v", err)
		if err := models.CreateRepositoryNotice(description); err != nil {
			ctx.ServerError("ErrCreatingReporitoryNotice", err)
		}
		return
	}

	ctx.Data["Editorconfig"] = ec
}

// SetDiffViewStyle set diff style as render variable
func SetDiffViewStyle(ctx *context.Context) {
	queryStyle := ctx.Query("style")

	if !ctx.IsSigned {
		ctx.Data["IsSplitStyle"] = queryStyle == "split"
		return
	}

	var (
		userStyle = ctx.User.DiffViewStyle
		style     string
	)

	if queryStyle == "unified" || queryStyle == "split" {
		style = queryStyle
	} else if userStyle == "unified" || userStyle == "split" {
		style = userStyle
	} else {
		style = "unified"
	}

	ctx.Data["IsSplitStyle"] = style == "split"
	if err := ctx.User.UpdateDiffViewStyle(style); err != nil {
		ctx.ServerError("ErrUpdateDiffViewStyle", err)
	}
}

// SetWhitespaceBehavior set whitespace behavior as render variable
func SetWhitespaceBehavior(ctx *context.Context) {
	queryWhitespaceBehavior := ctx.Query("whitespace")
	if !ctx.IsSigned {
		switch queryWhitespaceBehavior {
		case "ignore-all", "ignore-eol", "ignore-change":
			ctx.Data["WhitespaceBehavior"] = queryWhitespaceBehavior
		default:
			ctx.Data["WhitespaceBehavior"] = ""
		}
		return
	}

	var (
		userWhitespaceBehaviour = ctx.User.WhitespaceBehavior
		whitespaceBehavior      string
	)

	if queryWhitespaceBehavior == "ignore-all" || queryWhitespaceBehavior == "ignore-eol" || queryWhitespaceBehavior == "ignore-change" || queryWhitespaceBehavior == "" {
		whitespaceBehavior = queryWhitespaceBehavior
	} else if userWhitespaceBehaviour == "ignore-all" || userWhitespaceBehaviour == "ignore-eol" || userWhitespaceBehaviour == "ignore-change" || userWhitespaceBehaviour == "" {
		whitespaceBehavior = userWhitespaceBehaviour
	} else {
		whitespaceBehavior = ""
	}

	ctx.Data["WhitespaceBehavior"] = whitespaceBehavior
	if err := ctx.User.UpdateWhitespaceBehavior(whitespaceBehavior); err != nil {
		ctx.ServerError("ErrUpdateWhitespaceBehavior", err)
	}
}
