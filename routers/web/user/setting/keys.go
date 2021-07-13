// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/forms"
)

const (
	tplSettingsKeys base.TplName = "user/settings/keys"
)

// Keys render user's SSH/GPG public keys page
func Keys(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsKeys"] = true
	ctx.Data["DisableSSH"] = setting.SSH.Disabled
	ctx.Data["BuiltinSSH"] = setting.SSH.StartBuiltinServer
	ctx.Data["AllowPrincipals"] = setting.SSH.AuthorizedPrincipalsEnabled

	loadKeysData(ctx)

	ctx.HTML(http.StatusOK, tplSettingsKeys)
}

// KeysPost response for change user's SSH/GPG keys
func KeysPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.AddKeyForm)
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsKeys"] = true
	ctx.Data["DisableSSH"] = setting.SSH.Disabled
	ctx.Data["BuiltinSSH"] = setting.SSH.StartBuiltinServer
	ctx.Data["AllowPrincipals"] = setting.SSH.AuthorizedPrincipalsEnabled

	if ctx.HasError() {
		loadKeysData(ctx)

		ctx.HTML(http.StatusOK, tplSettingsKeys)
		return
	}
	switch form.Type {
	case "principal":
		content, err := models.CheckPrincipalKeyString(ctx.User, form.Content)
		if err != nil {
			if models.IsErrSSHDisabled(err) {
				ctx.Flash.Info(ctx.Tr("settings.ssh_disabled"))
			} else {
				ctx.Flash.Error(ctx.Tr("form.invalid_ssh_principal", err.Error()))
			}
			ctx.Redirect(setting.AppSubURL + "/user/settings/keys")
			return
		}
		if _, err = models.AddPrincipalKey(ctx.User.ID, content, 0); err != nil {
			ctx.Data["HasPrincipalError"] = true
			switch {
			case models.IsErrKeyAlreadyExist(err), models.IsErrKeyNameAlreadyUsed(err):
				loadKeysData(ctx)

				ctx.Data["Err_Content"] = true
				ctx.RenderWithErr(ctx.Tr("settings.ssh_principal_been_used"), tplSettingsKeys, &form)
			default:
				ctx.ServerError("AddPrincipalKey", err)
			}
			return
		}
		ctx.Flash.Success(ctx.Tr("settings.add_principal_success", form.Content))
		ctx.Redirect(setting.AppSubURL + "/user/settings/keys")
	case "gpg":
		token := models.VerificationToken(ctx.User, 1)
		lastToken := models.VerificationToken(ctx.User, 0)

		keys, err := models.AddGPGKey(ctx.User.ID, form.Content, token, form.Signature)
		if err != nil && models.IsErrGPGInvalidTokenSignature(err) {
			keys, err = models.AddGPGKey(ctx.User.ID, form.Content, lastToken, form.Signature)
		}
		if err != nil {
			ctx.Data["HasGPGError"] = true
			switch {
			case models.IsErrGPGKeyParsing(err):
				ctx.Flash.Error(ctx.Tr("form.invalid_gpg_key", err.Error()))
				ctx.Redirect(setting.AppSubURL + "/user/settings/keys")
			case models.IsErrGPGKeyIDAlreadyUsed(err):
				loadKeysData(ctx)

				ctx.Data["Err_Content"] = true
				ctx.RenderWithErr(ctx.Tr("settings.gpg_key_id_used"), tplSettingsKeys, &form)
			case models.IsErrGPGInvalidTokenSignature(err):
				loadKeysData(ctx)
				ctx.Data["Err_Content"] = true
				ctx.Data["Err_Signature"] = true
				ctx.Data["KeyID"] = err.(models.ErrGPGInvalidTokenSignature).ID
				ctx.RenderWithErr(ctx.Tr("settings.gpg_invalid_token_signature"), tplSettingsKeys, &form)
			case models.IsErrGPGNoEmailFound(err):
				loadKeysData(ctx)

				ctx.Data["Err_Content"] = true
				ctx.Data["Err_Signature"] = true
				ctx.Data["KeyID"] = err.(models.ErrGPGNoEmailFound).ID
				ctx.RenderWithErr(ctx.Tr("settings.gpg_no_key_email_found"), tplSettingsKeys, &form)
			default:
				ctx.ServerError("AddPublicKey", err)
			}
			return
		}
		keyIDs := ""
		for _, key := range keys {
			keyIDs += key.KeyID
			keyIDs += ", "
		}
		if len(keyIDs) > 0 {
			keyIDs = keyIDs[:len(keyIDs)-2]
		}
		ctx.Flash.Success(ctx.Tr("settings.add_gpg_key_success", keyIDs))
		ctx.Redirect(setting.AppSubURL + "/user/settings/keys")
	case "verify_gpg":
		token := models.VerificationToken(ctx.User, 1)
		lastToken := models.VerificationToken(ctx.User, 0)

		keyID, err := models.VerifyGPGKey(ctx.User.ID, form.KeyID, token, form.Signature)
		if err != nil && models.IsErrGPGInvalidTokenSignature(err) {
			keyID, err = models.VerifyGPGKey(ctx.User.ID, form.KeyID, lastToken, form.Signature)
		}
		if err != nil {
			ctx.Data["HasGPGVerifyError"] = true
			switch {
			case models.IsErrGPGInvalidTokenSignature(err):
				loadKeysData(ctx)
				ctx.Data["VerifyingID"] = form.KeyID
				ctx.Data["Err_Signature"] = true
				ctx.Data["KeyID"] = err.(models.ErrGPGInvalidTokenSignature).ID
				ctx.RenderWithErr(ctx.Tr("settings.gpg_invalid_token_signature"), tplSettingsKeys, &form)
			default:
				ctx.ServerError("VerifyGPG", err)
			}
		}
		ctx.Flash.Success(ctx.Tr("settings.verify_gpg_key_success", keyID))
		ctx.Redirect(setting.AppSubURL + "/user/settings/keys")
	case "ssh":
		content, err := models.CheckPublicKeyString(form.Content)
		if err != nil {
			if models.IsErrSSHDisabled(err) {
				ctx.Flash.Info(ctx.Tr("settings.ssh_disabled"))
			} else if models.IsErrKeyUnableVerify(err) {
				ctx.Flash.Info(ctx.Tr("form.unable_verify_ssh_key"))
			} else {
				ctx.Flash.Error(ctx.Tr("form.invalid_ssh_key", err.Error()))
			}
			ctx.Redirect(setting.AppSubURL + "/user/settings/keys")
			return
		}

		if _, err = models.AddPublicKey(ctx.User.ID, form.Title, content, 0); err != nil {
			ctx.Data["HasSSHError"] = true
			switch {
			case models.IsErrKeyAlreadyExist(err):
				loadKeysData(ctx)

				ctx.Data["Err_Content"] = true
				ctx.RenderWithErr(ctx.Tr("settings.ssh_key_been_used"), tplSettingsKeys, &form)
			case models.IsErrKeyNameAlreadyUsed(err):
				loadKeysData(ctx)

				ctx.Data["Err_Title"] = true
				ctx.RenderWithErr(ctx.Tr("settings.ssh_key_name_used"), tplSettingsKeys, &form)
			case models.IsErrKeyUnableVerify(err):
				ctx.Flash.Info(ctx.Tr("form.unable_verify_ssh_key"))
				ctx.Redirect(setting.AppSubURL + "/user/settings/keys")
			default:
				ctx.ServerError("AddPublicKey", err)
			}
			return
		}
		ctx.Flash.Success(ctx.Tr("settings.add_key_success", form.Title))
		ctx.Redirect(setting.AppSubURL + "/user/settings/keys")

	default:
		ctx.Flash.Warning("Function not implemented")
		ctx.Redirect(setting.AppSubURL + "/user/settings/keys")
	}

}

// DeleteKey response for delete user's SSH/GPG key
func DeleteKey(ctx *context.Context) {

	switch ctx.Query("type") {
	case "gpg":
		if err := models.DeleteGPGKey(ctx.User, ctx.QueryInt64("id")); err != nil {
			ctx.Flash.Error("DeleteGPGKey: " + err.Error())
		} else {
			ctx.Flash.Success(ctx.Tr("settings.gpg_key_deletion_success"))
		}
	case "ssh":
		keyID := ctx.QueryInt64("id")
		external, err := models.PublicKeyIsExternallyManaged(keyID)
		if err != nil {
			ctx.ServerError("sshKeysExternalManaged", err)
			return
		}
		if external {
			ctx.Flash.Error(ctx.Tr("setting.ssh_externally_managed"))
			ctx.Redirect(setting.AppSubURL + "/user/settings/keys")
			return
		}
		if err := models.DeletePublicKey(ctx.User, keyID); err != nil {
			ctx.Flash.Error("DeletePublicKey: " + err.Error())
		} else {
			ctx.Flash.Success(ctx.Tr("settings.ssh_key_deletion_success"))
		}
	case "principal":
		if err := models.DeletePublicKey(ctx.User, ctx.QueryInt64("id")); err != nil {
			ctx.Flash.Error("DeletePublicKey: " + err.Error())
		} else {
			ctx.Flash.Success(ctx.Tr("settings.ssh_principal_deletion_success"))
		}
	default:
		ctx.Flash.Warning("Function not implemented")
		ctx.Redirect(setting.AppSubURL + "/user/settings/keys")
	}
	ctx.JSON(http.StatusOK, map[string]interface{}{
		"redirect": setting.AppSubURL + "/user/settings/keys",
	})
}

func loadKeysData(ctx *context.Context) {
	keys, err := models.ListPublicKeys(ctx.User.ID, models.ListOptions{})
	if err != nil {
		ctx.ServerError("ListPublicKeys", err)
		return
	}
	ctx.Data["Keys"] = keys

	externalKeys, err := models.PublicKeysAreExternallyManaged(keys)
	if err != nil {
		ctx.ServerError("ListPublicKeys", err)
		return
	}
	ctx.Data["ExternalKeys"] = externalKeys

	gpgkeys, err := models.ListGPGKeys(ctx.User.ID, models.ListOptions{})
	if err != nil {
		ctx.ServerError("ListGPGKeys", err)
		return
	}
	ctx.Data["GPGKeys"] = gpgkeys
	tokenToSign := models.VerificationToken(ctx.User, 1)

	// generate a new aes cipher using the csrfToken
	ctx.Data["TokenToSign"] = tokenToSign

	principals, err := models.ListPrincipalKeys(ctx.User.ID, models.ListOptions{})
	if err != nil {
		ctx.ServerError("ListPrincipalKeys", err)
		return
	}
	ctx.Data["Principals"] = principals

	ctx.Data["VerifyingID"] = ctx.Query("verify_gpg")
}
