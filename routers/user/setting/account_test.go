// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	auth "code.gitea.io/gitea/modules/forms"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/web"

	"github.com/stretchr/testify/assert"
)

func TestChangePassword(t *testing.T) {
	oldPassword := "password"
	setting.MinPasswordLength = 6
	pcALL := []string{"lower", "upper", "digit", "spec"}
	pcLUN := []string{"lower", "upper", "digit"}
	pcLU := []string{"lower", "upper"}

	for _, req := range []struct {
		OldPassword        string
		NewPassword        string
		Retype             string
		Message            string
		PasswordComplexity []string
	}{
		{
			OldPassword:        oldPassword,
			NewPassword:        "Qwerty123456-",
			Retype:             "Qwerty123456-",
			Message:            "",
			PasswordComplexity: pcALL,
		},
		{
			OldPassword:        oldPassword,
			NewPassword:        "12345",
			Retype:             "12345",
			Message:            "auth.password_too_short",
			PasswordComplexity: pcALL,
		},
		{
			OldPassword:        "12334",
			NewPassword:        "123456",
			Retype:             "123456",
			Message:            "settings.password_incorrect",
			PasswordComplexity: pcALL,
		},
		{
			OldPassword:        oldPassword,
			NewPassword:        "123456",
			Retype:             "12345",
			Message:            "form.password_not_match",
			PasswordComplexity: pcALL,
		},
		{
			OldPassword:        oldPassword,
			NewPassword:        "Qwerty",
			Retype:             "Qwerty",
			Message:            "form.password_complexity",
			PasswordComplexity: pcALL,
		},
		{
			OldPassword:        oldPassword,
			NewPassword:        "Qwerty",
			Retype:             "Qwerty",
			Message:            "form.password_complexity",
			PasswordComplexity: pcLUN,
		},
		{
			OldPassword:        oldPassword,
			NewPassword:        "QWERTY",
			Retype:             "QWERTY",
			Message:            "form.password_complexity",
			PasswordComplexity: pcLU,
		},
	} {
		models.PrepareTestEnv(t)
		ctx := test.MockContext(t, "user/settings/security")
		test.LoadUser(t, ctx, 2)
		test.LoadRepo(t, ctx, 1)

		web.SetForm(ctx, &auth.ChangePasswordForm{
			OldPassword: req.OldPassword,
			Password:    req.NewPassword,
			Retype:      req.Retype,
		})
		AccountPost(ctx)

		assert.Contains(t, ctx.Flash.ErrorMsg, req.Message)
		assert.EqualValues(t, http.StatusFound, ctx.Resp.Status())
	}
}
