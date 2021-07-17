// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/eventsource"
	"code.gitea.io/gitea/modules/hcaptcha"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/password"
	"code.gitea.io/gitea/modules/recaptcha"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/modules/web/middleware"
	"code.gitea.io/gitea/routers/utils"
	"code.gitea.io/gitea/services/auth"
	"code.gitea.io/gitea/services/auth/source/oauth2"
	"code.gitea.io/gitea/services/externalaccount"
	"code.gitea.io/gitea/services/forms"
	"code.gitea.io/gitea/services/mailer"

	"github.com/markbates/goth"
	"github.com/tstranex/u2f"
)

const (
	// tplMustChangePassword template for updating a user's password
	tplMustChangePassword = "user/auth/change_passwd"
	// tplSignIn template for sign in page
	tplSignIn base.TplName = "user/auth/signin"
	// tplSignUp template path for sign up page
	tplSignUp base.TplName = "user/auth/signup"
	// TplActivate template path for activate user
	TplActivate       base.TplName = "user/auth/activate"
	tplForgotPassword base.TplName = "user/auth/forgot_passwd"
	tplResetPassword  base.TplName = "user/auth/reset_passwd"
	tplTwofa          base.TplName = "user/auth/twofa"
	tplTwofaScratch   base.TplName = "user/auth/twofa_scratch"
	tplLinkAccount    base.TplName = "user/auth/link_account"
	tplU2F            base.TplName = "user/auth/u2f"
)

// AutoSignIn reads cookie and try to auto-login.
func AutoSignIn(ctx *context.Context) (bool, error) {
	if !models.HasEngine {
		return false, nil
	}

	uname := ctx.GetCookie(setting.CookieUserName)
	if len(uname) == 0 {
		return false, nil
	}

	isSucceed := false
	defer func() {
		if !isSucceed {
			log.Trace("auto-login cookie cleared: %s", uname)
			ctx.DeleteCookie(setting.CookieUserName)
			ctx.DeleteCookie(setting.CookieRememberName)
		}
	}()

	u, err := models.GetUserByName(uname)
	if err != nil {
		if !models.IsErrUserNotExist(err) {
			return false, fmt.Errorf("GetUserByName: %v", err)
		}
		return false, nil
	}

	if val, ok := ctx.GetSuperSecureCookie(
		base.EncodeMD5(u.Rands+u.Passwd), setting.CookieRememberName); !ok || val != u.Name {
		return false, nil
	}

	isSucceed = true

	// Set session IDs
	if err := ctx.Session.Set("uid", u.ID); err != nil {
		return false, err
	}
	if err := ctx.Session.Set("uname", u.Name); err != nil {
		return false, err
	}
	if err := ctx.Session.Release(); err != nil {
		return false, err
	}

	middleware.DeleteCSRFCookie(ctx.Resp)
	return true, nil
}

func checkAutoLogin(ctx *context.Context) bool {
	// Check auto-login.
	isSucceed, err := AutoSignIn(ctx)
	if err != nil {
		ctx.ServerError("AutoSignIn", err)
		return true
	}

	redirectTo := ctx.Query("redirect_to")
	if len(redirectTo) > 0 {
		middleware.SetRedirectToCookie(ctx.Resp, redirectTo)
	} else {
		redirectTo = ctx.GetCookie("redirect_to")
	}

	if isSucceed {
		middleware.DeleteRedirectToCookie(ctx.Resp)
		ctx.RedirectToFirst(redirectTo, setting.AppSubURL+string(setting.LandingPageURL))
		return true
	}

	return false
}

// SignIn render sign in page
func SignIn(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("sign_in")

	// Check auto-login.
	if checkAutoLogin(ctx) {
		return
	}

	orderedOAuth2Names, oauth2Providers, err := oauth2.GetActiveOAuth2Providers()
	if err != nil {
		ctx.ServerError("UserSignIn", err)
		return
	}
	ctx.Data["OrderedOAuth2Names"] = orderedOAuth2Names
	ctx.Data["OAuth2Providers"] = oauth2Providers
	ctx.Data["Title"] = ctx.Tr("sign_in")
	ctx.Data["SignInLink"] = setting.AppSubURL + "/user/login"
	ctx.Data["PageIsSignIn"] = true
	ctx.Data["PageIsLogin"] = true
	ctx.Data["EnableSSPI"] = models.IsSSPIEnabled()

	ctx.HTML(http.StatusOK, tplSignIn)
}

// SignInPost response for sign in request
func SignInPost(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("sign_in")

	orderedOAuth2Names, oauth2Providers, err := oauth2.GetActiveOAuth2Providers()
	if err != nil {
		ctx.ServerError("UserSignIn", err)
		return
	}
	ctx.Data["OrderedOAuth2Names"] = orderedOAuth2Names
	ctx.Data["OAuth2Providers"] = oauth2Providers
	ctx.Data["Title"] = ctx.Tr("sign_in")
	ctx.Data["SignInLink"] = setting.AppSubURL + "/user/login"
	ctx.Data["PageIsSignIn"] = true
	ctx.Data["PageIsLogin"] = true
	ctx.Data["EnableSSPI"] = models.IsSSPIEnabled()

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplSignIn)
		return
	}

	form := web.GetForm(ctx).(*forms.SignInForm)
	u, err := auth.UserSignIn(form.UserName, form.Password)
	if err != nil {
		if models.IsErrUserNotExist(err) {
			ctx.RenderWithErr(ctx.Tr("form.username_password_incorrect"), tplSignIn, &form)
			log.Info("Failed authentication attempt for %s from %s: %v", form.UserName, ctx.RemoteAddr(), err)
		} else if models.IsErrEmailAlreadyUsed(err) {
			ctx.RenderWithErr(ctx.Tr("form.email_been_used"), tplSignIn, &form)
			log.Info("Failed authentication attempt for %s from %s: %v", form.UserName, ctx.RemoteAddr(), err)
		} else if models.IsErrUserProhibitLogin(err) {
			log.Info("Failed authentication attempt for %s from %s: %v", form.UserName, ctx.RemoteAddr(), err)
			ctx.Data["Title"] = ctx.Tr("auth.prohibit_login")
			ctx.HTML(http.StatusOK, "user/auth/prohibit_login")
		} else if models.IsErrUserInactive(err) {
			if setting.Service.RegisterEmailConfirm {
				ctx.Data["Title"] = ctx.Tr("auth.active_your_account")
				ctx.HTML(http.StatusOK, TplActivate)
			} else {
				log.Info("Failed authentication attempt for %s from %s: %v", form.UserName, ctx.RemoteAddr(), err)
				ctx.Data["Title"] = ctx.Tr("auth.prohibit_login")
				ctx.HTML(http.StatusOK, "user/auth/prohibit_login")
			}
		} else {
			ctx.ServerError("UserSignIn", err)
		}
		return
	}
	// If this user is enrolled in 2FA, we can't sign the user in just yet.
	// Instead, redirect them to the 2FA authentication page.
	_, err = models.GetTwoFactorByUID(u.ID)
	if err != nil {
		if models.IsErrTwoFactorNotEnrolled(err) {
			handleSignIn(ctx, u, form.Remember)
		} else {
			ctx.ServerError("UserSignIn", err)
		}
		return
	}

	// User needs to use 2FA, save data and redirect to 2FA page.
	if err := ctx.Session.Set("twofaUid", u.ID); err != nil {
		ctx.ServerError("UserSignIn: Unable to set twofaUid in session", err)
		return
	}
	if err := ctx.Session.Set("twofaRemember", form.Remember); err != nil {
		ctx.ServerError("UserSignIn: Unable to set twofaRemember in session", err)
		return
	}
	if err := ctx.Session.Release(); err != nil {
		ctx.ServerError("UserSignIn: Unable to save session", err)
		return
	}

	regs, err := models.GetU2FRegistrationsByUID(u.ID)
	if err == nil && len(regs) > 0 {
		ctx.Redirect(setting.AppSubURL + "/user/u2f")
		return
	}

	ctx.Redirect(setting.AppSubURL + "/user/two_factor")
}

// TwoFactor shows the user a two-factor authentication page.
func TwoFactor(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("twofa")

	// Check auto-login.
	if checkAutoLogin(ctx) {
		return
	}

	// Ensure user is in a 2FA session.
	if ctx.Session.Get("twofaUid") == nil {
		ctx.ServerError("UserSignIn", errors.New("not in 2FA session"))
		return
	}

	ctx.HTML(http.StatusOK, tplTwofa)
}

// TwoFactorPost validates a user's two-factor authentication token.
func TwoFactorPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.TwoFactorAuthForm)
	ctx.Data["Title"] = ctx.Tr("twofa")

	// Ensure user is in a 2FA session.
	idSess := ctx.Session.Get("twofaUid")
	if idSess == nil {
		ctx.ServerError("UserSignIn", errors.New("not in 2FA session"))
		return
	}

	id := idSess.(int64)
	twofa, err := models.GetTwoFactorByUID(id)
	if err != nil {
		ctx.ServerError("UserSignIn", err)
		return
	}

	// Validate the passcode with the stored TOTP secret.
	ok, err := twofa.ValidateTOTP(form.Passcode)
	if err != nil {
		ctx.ServerError("UserSignIn", err)
		return
	}

	if ok && twofa.LastUsedPasscode != form.Passcode {
		remember := ctx.Session.Get("twofaRemember").(bool)
		u, err := models.GetUserByID(id)
		if err != nil {
			ctx.ServerError("UserSignIn", err)
			return
		}

		if ctx.Session.Get("linkAccount") != nil {
			gothUser := ctx.Session.Get("linkAccountGothUser")
			if gothUser == nil {
				ctx.ServerError("UserSignIn", errors.New("not in LinkAccount session"))
				return
			}

			err = externalaccount.LinkAccountToUser(u, gothUser.(goth.User))
			if err != nil {
				ctx.ServerError("UserSignIn", err)
				return
			}
		}

		twofa.LastUsedPasscode = form.Passcode
		if err = models.UpdateTwoFactor(twofa); err != nil {
			ctx.ServerError("UserSignIn", err)
			return
		}

		handleSignIn(ctx, u, remember)
		return
	}

	ctx.RenderWithErr(ctx.Tr("auth.twofa_passcode_incorrect"), tplTwofa, forms.TwoFactorAuthForm{})
}

// TwoFactorScratch shows the scratch code form for two-factor authentication.
func TwoFactorScratch(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("twofa_scratch")

	// Check auto-login.
	if checkAutoLogin(ctx) {
		return
	}

	// Ensure user is in a 2FA session.
	if ctx.Session.Get("twofaUid") == nil {
		ctx.ServerError("UserSignIn", errors.New("not in 2FA session"))
		return
	}

	ctx.HTML(http.StatusOK, tplTwofaScratch)
}

// TwoFactorScratchPost validates and invalidates a user's two-factor scratch token.
func TwoFactorScratchPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.TwoFactorScratchAuthForm)
	ctx.Data["Title"] = ctx.Tr("twofa_scratch")

	// Ensure user is in a 2FA session.
	idSess := ctx.Session.Get("twofaUid")
	if idSess == nil {
		ctx.ServerError("UserSignIn", errors.New("not in 2FA session"))
		return
	}

	id := idSess.(int64)
	twofa, err := models.GetTwoFactorByUID(id)
	if err != nil {
		ctx.ServerError("UserSignIn", err)
		return
	}

	// Validate the passcode with the stored TOTP secret.
	if twofa.VerifyScratchToken(form.Token) {
		// Invalidate the scratch token.
		_, err = twofa.GenerateScratchToken()
		if err != nil {
			ctx.ServerError("UserSignIn", err)
			return
		}
		if err = models.UpdateTwoFactor(twofa); err != nil {
			ctx.ServerError("UserSignIn", err)
			return
		}

		remember := ctx.Session.Get("twofaRemember").(bool)
		u, err := models.GetUserByID(id)
		if err != nil {
			ctx.ServerError("UserSignIn", err)
			return
		}

		handleSignInFull(ctx, u, remember, false)
		ctx.Flash.Info(ctx.Tr("auth.twofa_scratch_used"))
		ctx.Redirect(setting.AppSubURL + "/user/settings/security")
		return
	}

	ctx.RenderWithErr(ctx.Tr("auth.twofa_scratch_token_incorrect"), tplTwofaScratch, forms.TwoFactorScratchAuthForm{})
}

// U2F shows the U2F login page
func U2F(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("twofa")
	ctx.Data["RequireU2F"] = true
	// Check auto-login.
	if checkAutoLogin(ctx) {
		return
	}

	// Ensure user is in a 2FA session.
	if ctx.Session.Get("twofaUid") == nil {
		ctx.ServerError("UserSignIn", errors.New("not in U2F session"))
		return
	}

	ctx.HTML(http.StatusOK, tplU2F)
}

// U2FChallenge submits a sign challenge to the browser
func U2FChallenge(ctx *context.Context) {
	// Ensure user is in a U2F session.
	idSess := ctx.Session.Get("twofaUid")
	if idSess == nil {
		ctx.ServerError("UserSignIn", errors.New("not in U2F session"))
		return
	}
	id := idSess.(int64)
	regs, err := models.GetU2FRegistrationsByUID(id)
	if err != nil {
		ctx.ServerError("UserSignIn", err)
		return
	}
	if len(regs) == 0 {
		ctx.ServerError("UserSignIn", errors.New("no device registered"))
		return
	}
	challenge, err := u2f.NewChallenge(setting.U2F.AppID, setting.U2F.TrustedFacets)
	if err != nil {
		ctx.ServerError("u2f.NewChallenge", err)
		return
	}
	if err := ctx.Session.Set("u2fChallenge", challenge); err != nil {
		ctx.ServerError("UserSignIn: unable to set u2fChallenge in session", err)
		return
	}
	if err := ctx.Session.Release(); err != nil {
		ctx.ServerError("UserSignIn: unable to store session", err)
	}

	ctx.JSON(http.StatusOK, challenge.SignRequest(regs.ToRegistrations()))
}

// U2FSign authenticates the user by signResp
func U2FSign(ctx *context.Context) {
	signResp := web.GetForm(ctx).(*u2f.SignResponse)
	challSess := ctx.Session.Get("u2fChallenge")
	idSess := ctx.Session.Get("twofaUid")
	if challSess == nil || idSess == nil {
		ctx.ServerError("UserSignIn", errors.New("not in U2F session"))
		return
	}
	challenge := challSess.(*u2f.Challenge)
	id := idSess.(int64)
	regs, err := models.GetU2FRegistrationsByUID(id)
	if err != nil {
		ctx.ServerError("UserSignIn", err)
		return
	}
	for _, reg := range regs {
		r, err := reg.Parse()
		if err != nil {
			log.Fatal("parsing u2f registration: %v", err)
			continue
		}
		newCounter, authErr := r.Authenticate(*signResp, *challenge, reg.Counter)
		if authErr == nil {
			reg.Counter = newCounter
			user, err := models.GetUserByID(id)
			if err != nil {
				ctx.ServerError("UserSignIn", err)
				return
			}
			remember := ctx.Session.Get("twofaRemember").(bool)
			if err := reg.UpdateCounter(); err != nil {
				ctx.ServerError("UserSignIn", err)
				return
			}

			if ctx.Session.Get("linkAccount") != nil {
				gothUser := ctx.Session.Get("linkAccountGothUser")
				if gothUser == nil {
					ctx.ServerError("UserSignIn", errors.New("not in LinkAccount session"))
					return
				}

				err = externalaccount.LinkAccountToUser(user, gothUser.(goth.User))
				if err != nil {
					ctx.ServerError("UserSignIn", err)
					return
				}
			}
			redirect := handleSignInFull(ctx, user, remember, false)
			if redirect == "" {
				redirect = setting.AppSubURL + "/"
			}
			ctx.PlainText(200, []byte(redirect))
			return
		}
	}
	ctx.Error(http.StatusUnauthorized)
}

// This handles the final part of the sign-in process of the user.
func handleSignIn(ctx *context.Context, u *models.User, remember bool) {
	handleSignInFull(ctx, u, remember, true)
}

func handleSignInFull(ctx *context.Context, u *models.User, remember bool, obeyRedirect bool) string {
	if remember {
		days := 86400 * setting.LogInRememberDays
		ctx.SetCookie(setting.CookieUserName, u.Name, days)
		ctx.SetSuperSecureCookie(base.EncodeMD5(u.Rands+u.Passwd),
			setting.CookieRememberName, u.Name, days)
	}

	_ = ctx.Session.Delete("openid_verified_uri")
	_ = ctx.Session.Delete("openid_signin_remember")
	_ = ctx.Session.Delete("openid_determined_email")
	_ = ctx.Session.Delete("openid_determined_username")
	_ = ctx.Session.Delete("twofaUid")
	_ = ctx.Session.Delete("twofaRemember")
	_ = ctx.Session.Delete("u2fChallenge")
	_ = ctx.Session.Delete("linkAccount")
	if err := ctx.Session.Set("uid", u.ID); err != nil {
		log.Error("Error setting uid %d in session: %v", u.ID, err)
	}
	if err := ctx.Session.Set("uname", u.Name); err != nil {
		log.Error("Error setting uname %s session: %v", u.Name, err)
	}
	if err := ctx.Session.Release(); err != nil {
		log.Error("Unable to store session: %v", err)
	}

	// Language setting of the user overwrites the one previously set
	// If the user does not have a locale set, we save the current one.
	if len(u.Language) == 0 {
		u.Language = ctx.Locale.Language()
		if err := models.UpdateUserCols(u, "language"); err != nil {
			log.Error(fmt.Sprintf("Error updating user language [user: %d, locale: %s]", u.ID, u.Language))
			return setting.AppSubURL + "/"
		}
	}

	middleware.SetLocaleCookie(ctx.Resp, u.Language, 0)

	// Clear whatever CSRF has right now, force to generate a new one
	middleware.DeleteCSRFCookie(ctx.Resp)

	// Register last login
	u.SetLastLogin()
	if err := models.UpdateUserCols(u, "last_login_unix"); err != nil {
		ctx.ServerError("UpdateUserCols", err)
		return setting.AppSubURL + "/"
	}

	if redirectTo := ctx.GetCookie("redirect_to"); len(redirectTo) > 0 && !utils.IsExternalURL(redirectTo) {
		middleware.DeleteRedirectToCookie(ctx.Resp)
		if obeyRedirect {
			ctx.RedirectToFirst(redirectTo)
		}
		return redirectTo
	}

	if obeyRedirect {
		ctx.Redirect(setting.AppSubURL + "/")
	}
	return setting.AppSubURL + "/"
}

// SignInOAuth handles the OAuth2 login buttons
func SignInOAuth(ctx *context.Context) {
	provider := ctx.Params(":provider")

	loginSource, err := models.GetActiveOAuth2LoginSourceByName(provider)
	if err != nil {
		ctx.ServerError("SignIn", err)
		return
	}

	// try to do a direct callback flow, so we don't authenticate the user again but use the valid accesstoken to get the user
	user, gothUser, err := oAuth2UserLoginCallback(loginSource, ctx.Req, ctx.Resp)
	if err == nil && user != nil {
		// we got the user without going through the whole OAuth2 authentication flow again
		handleOAuth2SignIn(ctx, user, gothUser)
		return
	}

	if err = loginSource.Cfg.(*oauth2.Source).Callout(ctx.Req, ctx.Resp); err != nil {
		if strings.Contains(err.Error(), "no provider for ") {
			if err = oauth2.ResetOAuth2(); err != nil {
				ctx.ServerError("SignIn", err)
				return
			}
			if err = loginSource.Cfg.(*oauth2.Source).Callout(ctx.Req, ctx.Resp); err != nil {
				ctx.ServerError("SignIn", err)
			}
			return
		}
		ctx.ServerError("SignIn", err)
	}
	// redirect is done in oauth2.Auth
}

// SignInOAuthCallback handles the callback from the given provider
func SignInOAuthCallback(ctx *context.Context) {
	provider := ctx.Params(":provider")

	// first look if the provider is still active
	loginSource, err := models.GetActiveOAuth2LoginSourceByName(provider)
	if err != nil {
		ctx.ServerError("SignIn", err)
		return
	}

	if loginSource == nil {
		ctx.ServerError("SignIn", errors.New("No valid provider found, check configured callback url in provider"))
		return
	}

	u, gothUser, err := oAuth2UserLoginCallback(loginSource, ctx.Req, ctx.Resp)

	if err != nil {
		ctx.ServerError("UserSignIn", err)
		return
	}

	if u == nil {
		if !(setting.Service.DisableRegistration || setting.Service.AllowOnlyInternalRegistration) && setting.OAuth2Client.EnableAutoRegistration {
			// create new user with details from oauth2 provider
			var missingFields []string
			if gothUser.UserID == "" {
				missingFields = append(missingFields, "sub")
			}
			if gothUser.Email == "" {
				missingFields = append(missingFields, "email")
			}
			if setting.OAuth2Client.Username == setting.OAuth2UsernameNickname && gothUser.NickName == "" {
				missingFields = append(missingFields, "nickname")
			}
			if len(missingFields) > 0 {
				log.Error("OAuth2 Provider %s returned empty or missing fields: %s", loginSource.Name, missingFields)
				if loginSource.IsOAuth2() && loginSource.Cfg.(*oauth2.Source).Provider == "openidConnect" {
					log.Error("You may need to change the 'OPENID_CONNECT_SCOPES' setting to request all required fields")
				}
				err = fmt.Errorf("OAuth2 Provider %s returned empty or missing fields: %s", loginSource.Name, missingFields)
				ctx.ServerError("CreateUser", err)
				return
			}
			u = &models.User{
				Name:        getUserName(&gothUser),
				FullName:    gothUser.Name,
				Email:       gothUser.Email,
				IsActive:    !setting.OAuth2Client.RegisterEmailConfirm,
				LoginType:   models.LoginOAuth2,
				LoginSource: loginSource.ID,
				LoginName:   gothUser.UserID,
			}

			if !createAndHandleCreatedUser(ctx, base.TplName(""), nil, u, &gothUser, setting.OAuth2Client.AccountLinking != setting.OAuth2AccountLinkingDisabled) {
				// error already handled
				return
			}
		} else {
			// no existing user is found, request attach or new account
			showLinkingLogin(ctx, gothUser)
			return
		}
	}

	handleOAuth2SignIn(ctx, u, gothUser)
}

func getUserName(gothUser *goth.User) string {
	switch setting.OAuth2Client.Username {
	case setting.OAuth2UsernameEmail:
		return strings.Split(gothUser.Email, "@")[0]
	case setting.OAuth2UsernameNickname:
		return gothUser.NickName
	default: // OAuth2UsernameUserid
		return gothUser.UserID
	}
}

func showLinkingLogin(ctx *context.Context, gothUser goth.User) {
	if err := ctx.Session.Set("linkAccountGothUser", gothUser); err != nil {
		log.Error("Error setting linkAccountGothUser in session: %v", err)
	}
	if err := ctx.Session.Release(); err != nil {
		log.Error("Error storing session: %v", err)
	}
	ctx.Redirect(setting.AppSubURL + "/user/link_account")
}

func updateAvatarIfNeed(url string, u *models.User) {
	if setting.OAuth2Client.UpdateAvatar && len(url) > 0 {
		resp, err := http.Get(url)
		if err == nil {
			defer func() {
				_ = resp.Body.Close()
			}()
		}
		// ignore any error
		if err == nil && resp.StatusCode == http.StatusOK {
			data, err := ioutil.ReadAll(io.LimitReader(resp.Body, setting.Avatar.MaxFileSize+1))
			if err == nil && int64(len(data)) <= setting.Avatar.MaxFileSize {
				_ = u.UploadAvatar(data)
			}
		}
	}
}

func handleOAuth2SignIn(ctx *context.Context, u *models.User, gothUser goth.User) {
	updateAvatarIfNeed(gothUser.AvatarURL, u)

	// If this user is enrolled in 2FA, we can't sign the user in just yet.
	// Instead, redirect them to the 2FA authentication page.
	_, err := models.GetTwoFactorByUID(u.ID)
	if err != nil {
		if !models.IsErrTwoFactorNotEnrolled(err) {
			ctx.ServerError("UserSignIn", err)
			return
		}

		if err := ctx.Session.Set("uid", u.ID); err != nil {
			log.Error("Error setting uid in session: %v", err)
		}
		if err := ctx.Session.Set("uname", u.Name); err != nil {
			log.Error("Error setting uname in session: %v", err)
		}
		if err := ctx.Session.Release(); err != nil {
			log.Error("Error storing session: %v", err)
		}

		// Clear whatever CSRF has right now, force to generate a new one
		middleware.DeleteCSRFCookie(ctx.Resp)

		// Register last login
		u.SetLastLogin()
		if err := models.UpdateUserCols(u, "last_login_unix"); err != nil {
			ctx.ServerError("UpdateUserCols", err)
			return
		}

		// update external user information
		if err := models.UpdateExternalUser(u, gothUser); err != nil {
			log.Error("UpdateExternalUser failed: %v", err)
		}

		if redirectTo := ctx.GetCookie("redirect_to"); len(redirectTo) > 0 {
			middleware.DeleteRedirectToCookie(ctx.Resp)
			ctx.RedirectToFirst(redirectTo)
			return
		}

		ctx.Redirect(setting.AppSubURL + "/")
		return
	}

	// User needs to use 2FA, save data and redirect to 2FA page.
	if err := ctx.Session.Set("twofaUid", u.ID); err != nil {
		log.Error("Error setting twofaUid in session: %v", err)
	}
	if err := ctx.Session.Set("twofaRemember", false); err != nil {
		log.Error("Error setting twofaRemember in session: %v", err)
	}
	if err := ctx.Session.Release(); err != nil {
		log.Error("Error storing session: %v", err)
	}

	// If U2F is enrolled -> Redirect to U2F instead
	regs, err := models.GetU2FRegistrationsByUID(u.ID)
	if err == nil && len(regs) > 0 {
		ctx.Redirect(setting.AppSubURL + "/user/u2f")
		return
	}

	ctx.Redirect(setting.AppSubURL + "/user/two_factor")
}

// OAuth2UserLoginCallback attempts to handle the callback from the OAuth2 provider and if successful
// login the user
func oAuth2UserLoginCallback(loginSource *models.LoginSource, request *http.Request, response http.ResponseWriter) (*models.User, goth.User, error) {
	gothUser, err := loginSource.Cfg.(*oauth2.Source).Callback(request, response)
	if err != nil {
		if err.Error() == "securecookie: the value is too long" {
			log.Error("OAuth2 Provider %s returned too long a token. Current max: %d. Either increase the [OAuth2] MAX_TOKEN_LENGTH or reduce the information returned from the OAuth2 provider", loginSource.Name, setting.OAuth2.MaxTokenLength)
			err = fmt.Errorf("OAuth2 Provider %s returned too long a token. Current max: %d. Either increase the [OAuth2] MAX_TOKEN_LENGTH or reduce the information returned from the OAuth2 provider", loginSource.Name, setting.OAuth2.MaxTokenLength)
		}
		return nil, goth.User{}, err
	}

	user := &models.User{
		LoginName:   gothUser.UserID,
		LoginType:   models.LoginOAuth2,
		LoginSource: loginSource.ID,
	}

	hasUser, err := models.GetUser(user)
	if err != nil {
		return nil, goth.User{}, err
	}

	if hasUser {
		return user, gothUser, nil
	}

	// search in external linked users
	externalLoginUser := &models.ExternalLoginUser{
		ExternalID:    gothUser.UserID,
		LoginSourceID: loginSource.ID,
	}
	hasUser, err = models.GetExternalLogin(externalLoginUser)
	if err != nil {
		return nil, goth.User{}, err
	}
	if hasUser {
		user, err = models.GetUserByID(externalLoginUser.UserID)
		return user, gothUser, err
	}

	// no user found to login
	return nil, gothUser, nil

}

// LinkAccount shows the page where the user can decide to login or create a new account
func LinkAccount(ctx *context.Context) {
	ctx.Data["DisablePassword"] = !setting.Service.RequireExternalRegistrationPassword || setting.Service.AllowOnlyExternalRegistration
	ctx.Data["Title"] = ctx.Tr("link_account")
	ctx.Data["LinkAccountMode"] = true
	ctx.Data["EnableCaptcha"] = setting.Service.EnableCaptcha && setting.Service.RequireExternalRegistrationCaptcha
	ctx.Data["Captcha"] = context.GetImageCaptcha()
	ctx.Data["CaptchaType"] = setting.Service.CaptchaType
	ctx.Data["RecaptchaURL"] = setting.Service.RecaptchaURL
	ctx.Data["RecaptchaSitekey"] = setting.Service.RecaptchaSitekey
	ctx.Data["HcaptchaSitekey"] = setting.Service.HcaptchaSitekey
	ctx.Data["DisableRegistration"] = setting.Service.DisableRegistration
	ctx.Data["AllowOnlyInternalRegistration"] = setting.Service.AllowOnlyInternalRegistration
	ctx.Data["ShowRegistrationButton"] = false

	// use this to set the right link into the signIn and signUp templates in the link_account template
	ctx.Data["SignInLink"] = setting.AppSubURL + "/user/link_account_signin"
	ctx.Data["SignUpLink"] = setting.AppSubURL + "/user/link_account_signup"

	gothUser := ctx.Session.Get("linkAccountGothUser")
	if gothUser == nil {
		ctx.ServerError("UserSignIn", errors.New("not in LinkAccount session"))
		return
	}

	gu, _ := gothUser.(goth.User)
	uname := getUserName(&gu)
	email := gu.Email
	ctx.Data["user_name"] = uname
	ctx.Data["email"] = email

	if len(email) != 0 {
		u, err := models.GetUserByEmail(email)
		if err != nil && !models.IsErrUserNotExist(err) {
			ctx.ServerError("UserSignIn", err)
			return
		}
		if u != nil {
			ctx.Data["user_exists"] = true
		}
	} else if len(uname) != 0 {
		u, err := models.GetUserByName(uname)
		if err != nil && !models.IsErrUserNotExist(err) {
			ctx.ServerError("UserSignIn", err)
			return
		}
		if u != nil {
			ctx.Data["user_exists"] = true
		}
	}

	ctx.HTML(http.StatusOK, tplLinkAccount)
}

// LinkAccountPostSignIn handle the coupling of external account with another account using signIn
func LinkAccountPostSignIn(ctx *context.Context) {
	signInForm := web.GetForm(ctx).(*forms.SignInForm)
	ctx.Data["DisablePassword"] = !setting.Service.RequireExternalRegistrationPassword || setting.Service.AllowOnlyExternalRegistration
	ctx.Data["Title"] = ctx.Tr("link_account")
	ctx.Data["LinkAccountMode"] = true
	ctx.Data["LinkAccountModeSignIn"] = true
	ctx.Data["EnableCaptcha"] = setting.Service.EnableCaptcha && setting.Service.RequireExternalRegistrationCaptcha
	ctx.Data["RecaptchaURL"] = setting.Service.RecaptchaURL
	ctx.Data["Captcha"] = context.GetImageCaptcha()
	ctx.Data["CaptchaType"] = setting.Service.CaptchaType
	ctx.Data["RecaptchaSitekey"] = setting.Service.RecaptchaSitekey
	ctx.Data["HcaptchaSitekey"] = setting.Service.HcaptchaSitekey
	ctx.Data["DisableRegistration"] = setting.Service.DisableRegistration
	ctx.Data["ShowRegistrationButton"] = false

	// use this to set the right link into the signIn and signUp templates in the link_account template
	ctx.Data["SignInLink"] = setting.AppSubURL + "/user/link_account_signin"
	ctx.Data["SignUpLink"] = setting.AppSubURL + "/user/link_account_signup"

	gothUser := ctx.Session.Get("linkAccountGothUser")
	if gothUser == nil {
		ctx.ServerError("UserSignIn", errors.New("not in LinkAccount session"))
		return
	}

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplLinkAccount)
		return
	}

	u, err := auth.UserSignIn(signInForm.UserName, signInForm.Password)
	if err != nil {
		if models.IsErrUserNotExist(err) {
			ctx.Data["user_exists"] = true
			ctx.RenderWithErr(ctx.Tr("form.username_password_incorrect"), tplLinkAccount, &signInForm)
		} else {
			ctx.ServerError("UserLinkAccount", err)
		}
		return
	}

	linkAccount(ctx, u, gothUser.(goth.User), signInForm.Remember)
}

func linkAccount(ctx *context.Context, u *models.User, gothUser goth.User, remember bool) {
	updateAvatarIfNeed(gothUser.AvatarURL, u)

	// If this user is enrolled in 2FA, we can't sign the user in just yet.
	// Instead, redirect them to the 2FA authentication page.
	_, err := models.GetTwoFactorByUID(u.ID)
	if err != nil {
		if !models.IsErrTwoFactorNotEnrolled(err) {
			ctx.ServerError("UserLinkAccount", err)
			return
		}

		err = externalaccount.LinkAccountToUser(u, gothUser)
		if err != nil {
			ctx.ServerError("UserLinkAccount", err)
			return
		}

		handleSignIn(ctx, u, remember)
		return
	}

	// User needs to use 2FA, save data and redirect to 2FA page.
	if err := ctx.Session.Set("twofaUid", u.ID); err != nil {
		log.Error("Error setting twofaUid in session: %v", err)
	}
	if err := ctx.Session.Set("twofaRemember", remember); err != nil {
		log.Error("Error setting twofaRemember in session: %v", err)
	}
	if err := ctx.Session.Set("linkAccount", true); err != nil {
		log.Error("Error setting linkAccount in session: %v", err)
	}
	if err := ctx.Session.Release(); err != nil {
		log.Error("Error storing session: %v", err)
	}

	// If U2F is enrolled -> Redirect to U2F instead
	regs, err := models.GetU2FRegistrationsByUID(u.ID)
	if err == nil && len(regs) > 0 {
		ctx.Redirect(setting.AppSubURL + "/user/u2f")
		return
	}

	ctx.Redirect(setting.AppSubURL + "/user/two_factor")
}

// LinkAccountPostRegister handle the creation of a new account for an external account using signUp
func LinkAccountPostRegister(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.RegisterForm)
	// TODO Make insecure passwords optional for local accounts also,
	//      once email-based Second-Factor Auth is available
	ctx.Data["DisablePassword"] = !setting.Service.RequireExternalRegistrationPassword || setting.Service.AllowOnlyExternalRegistration
	ctx.Data["Title"] = ctx.Tr("link_account")
	ctx.Data["LinkAccountMode"] = true
	ctx.Data["LinkAccountModeRegister"] = true
	ctx.Data["EnableCaptcha"] = setting.Service.EnableCaptcha && setting.Service.RequireExternalRegistrationCaptcha
	ctx.Data["RecaptchaURL"] = setting.Service.RecaptchaURL
	ctx.Data["Captcha"] = context.GetImageCaptcha()
	ctx.Data["CaptchaType"] = setting.Service.CaptchaType
	ctx.Data["RecaptchaSitekey"] = setting.Service.RecaptchaSitekey
	ctx.Data["HcaptchaSitekey"] = setting.Service.HcaptchaSitekey
	ctx.Data["DisableRegistration"] = setting.Service.DisableRegistration
	ctx.Data["ShowRegistrationButton"] = false

	// use this to set the right link into the signIn and signUp templates in the link_account template
	ctx.Data["SignInLink"] = setting.AppSubURL + "/user/link_account_signin"
	ctx.Data["SignUpLink"] = setting.AppSubURL + "/user/link_account_signup"

	gothUserInterface := ctx.Session.Get("linkAccountGothUser")
	if gothUserInterface == nil {
		ctx.ServerError("UserSignUp", errors.New("not in LinkAccount session"))
		return
	}
	gothUser, ok := gothUserInterface.(goth.User)
	if !ok {
		ctx.ServerError("UserSignUp", fmt.Errorf("session linkAccountGothUser type is %t but not goth.User", gothUserInterface))
		return
	}

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplLinkAccount)
		return
	}

	if setting.Service.DisableRegistration || setting.Service.AllowOnlyInternalRegistration {
		ctx.Error(http.StatusForbidden)
		return
	}

	if setting.Service.EnableCaptcha && setting.Service.RequireExternalRegistrationCaptcha {
		var valid bool
		var err error
		switch setting.Service.CaptchaType {
		case setting.ImageCaptcha:
			valid = context.GetImageCaptcha().VerifyReq(ctx.Req)
		case setting.ReCaptcha:
			valid, err = recaptcha.Verify(ctx, form.GRecaptchaResponse)
		case setting.HCaptcha:
			valid, err = hcaptcha.Verify(ctx, form.HcaptchaResponse)
		default:
			ctx.ServerError("Unknown Captcha Type", fmt.Errorf("Unknown Captcha Type: %s", setting.Service.CaptchaType))
			return
		}
		if err != nil {
			log.Debug("%s", err.Error())
		}

		if !valid {
			ctx.Data["Err_Captcha"] = true
			ctx.RenderWithErr(ctx.Tr("form.captcha_incorrect"), tplLinkAccount, &form)
			return
		}
	}

	if !form.IsEmailDomainAllowed() {
		ctx.RenderWithErr(ctx.Tr("auth.email_domain_blacklisted"), tplLinkAccount, &form)
		return
	}

	if setting.Service.AllowOnlyExternalRegistration || !setting.Service.RequireExternalRegistrationPassword {
		// In models.User an empty password is classed as not set, so we set form.Password to empty.
		// Eventually the database should be changed to indicate "Second Factor"-enabled accounts
		// (accounts that do not introduce the security vulnerabilities of a password).
		// If a user decides to circumvent second-factor security, and purposefully create a password,
		// they can still do so using the "Recover Account" option.
		form.Password = ""
	} else {
		if (len(strings.TrimSpace(form.Password)) > 0 || len(strings.TrimSpace(form.Retype)) > 0) && form.Password != form.Retype {
			ctx.Data["Err_Password"] = true
			ctx.RenderWithErr(ctx.Tr("form.password_not_match"), tplLinkAccount, &form)
			return
		}
		if len(strings.TrimSpace(form.Password)) > 0 && len(form.Password) < setting.MinPasswordLength {
			ctx.Data["Err_Password"] = true
			ctx.RenderWithErr(ctx.Tr("auth.password_too_short", setting.MinPasswordLength), tplLinkAccount, &form)
			return
		}
	}

	loginSource, err := models.GetActiveOAuth2LoginSourceByName(gothUser.Provider)
	if err != nil {
		ctx.ServerError("CreateUser", err)
	}

	u := &models.User{
		Name:        form.UserName,
		Email:       form.Email,
		Passwd:      form.Password,
		IsActive:    !(setting.Service.RegisterEmailConfirm || setting.Service.RegisterManualConfirm),
		LoginType:   models.LoginOAuth2,
		LoginSource: loginSource.ID,
		LoginName:   gothUser.UserID,
	}

	if !createAndHandleCreatedUser(ctx, tplLinkAccount, form, u, &gothUser, false) {
		// error already handled
		return
	}

	ctx.Redirect(setting.AppSubURL + "/user/login")
}

// HandleSignOut resets the session and sets the cookies
func HandleSignOut(ctx *context.Context) {
	_ = ctx.Session.Flush()
	_ = ctx.Session.Destroy(ctx.Resp, ctx.Req)
	ctx.DeleteCookie(setting.CookieUserName)
	ctx.DeleteCookie(setting.CookieRememberName)
	middleware.DeleteCSRFCookie(ctx.Resp)
	middleware.DeleteLocaleCookie(ctx.Resp)
	middleware.DeleteRedirectToCookie(ctx.Resp)
}

// SignOut sign out from login status
func SignOut(ctx *context.Context) {
	if ctx.User != nil {
		eventsource.GetManager().SendMessageBlocking(ctx.User.ID, &eventsource.Event{
			Name: "logout",
			Data: ctx.Session.ID(),
		})
	}
	HandleSignOut(ctx)
	ctx.Redirect(setting.AppSubURL + "/")
}

// SignUp render the register page
func SignUp(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("sign_up")

	ctx.Data["SignUpLink"] = setting.AppSubURL + "/user/sign_up"

	ctx.Data["EnableCaptcha"] = setting.Service.EnableCaptcha
	ctx.Data["RecaptchaURL"] = setting.Service.RecaptchaURL
	ctx.Data["Captcha"] = context.GetImageCaptcha()
	ctx.Data["CaptchaType"] = setting.Service.CaptchaType
	ctx.Data["RecaptchaSitekey"] = setting.Service.RecaptchaSitekey
	ctx.Data["HcaptchaSitekey"] = setting.Service.HcaptchaSitekey
	ctx.Data["PageIsSignUp"] = true

	//Show Disabled Registration message if DisableRegistration or AllowOnlyExternalRegistration options are true
	ctx.Data["DisableRegistration"] = setting.Service.DisableRegistration || setting.Service.AllowOnlyExternalRegistration

	ctx.HTML(http.StatusOK, tplSignUp)
}

// SignUpPost response for sign up information submission
func SignUpPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.RegisterForm)
	ctx.Data["Title"] = ctx.Tr("sign_up")

	ctx.Data["SignUpLink"] = setting.AppSubURL + "/user/sign_up"

	ctx.Data["EnableCaptcha"] = setting.Service.EnableCaptcha
	ctx.Data["RecaptchaURL"] = setting.Service.RecaptchaURL
	ctx.Data["Captcha"] = context.GetImageCaptcha()
	ctx.Data["CaptchaType"] = setting.Service.CaptchaType
	ctx.Data["RecaptchaSitekey"] = setting.Service.RecaptchaSitekey
	ctx.Data["HcaptchaSitekey"] = setting.Service.HcaptchaSitekey
	ctx.Data["PageIsSignUp"] = true

	//Permission denied if DisableRegistration or AllowOnlyExternalRegistration options are true
	if setting.Service.DisableRegistration || setting.Service.AllowOnlyExternalRegistration {
		ctx.Error(http.StatusForbidden)
		return
	}

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplSignUp)
		return
	}

	if setting.Service.EnableCaptcha {
		var valid bool
		var err error
		switch setting.Service.CaptchaType {
		case setting.ImageCaptcha:
			valid = context.GetImageCaptcha().VerifyReq(ctx.Req)
		case setting.ReCaptcha:
			valid, err = recaptcha.Verify(ctx, form.GRecaptchaResponse)
		case setting.HCaptcha:
			valid, err = hcaptcha.Verify(ctx, form.HcaptchaResponse)
		default:
			ctx.ServerError("Unknown Captcha Type", fmt.Errorf("Unknown Captcha Type: %s", setting.Service.CaptchaType))
			return
		}
		if err != nil {
			log.Debug("%s", err.Error())
		}

		if !valid {
			ctx.Data["Err_Captcha"] = true
			ctx.RenderWithErr(ctx.Tr("form.captcha_incorrect"), tplSignUp, &form)
			return
		}
	}

	if !form.IsEmailDomainAllowed() {
		ctx.RenderWithErr(ctx.Tr("auth.email_domain_blacklisted"), tplSignUp, &form)
		return
	}

	if form.Password != form.Retype {
		ctx.Data["Err_Password"] = true
		ctx.RenderWithErr(ctx.Tr("form.password_not_match"), tplSignUp, &form)
		return
	}
	if len(form.Password) < setting.MinPasswordLength {
		ctx.Data["Err_Password"] = true
		ctx.RenderWithErr(ctx.Tr("auth.password_too_short", setting.MinPasswordLength), tplSignUp, &form)
		return
	}
	if !password.IsComplexEnough(form.Password) {
		ctx.Data["Err_Password"] = true
		ctx.RenderWithErr(password.BuildComplexityError(ctx), tplSignUp, &form)
		return
	}
	pwned, err := password.IsPwned(ctx, form.Password)
	if pwned {
		errMsg := ctx.Tr("auth.password_pwned")
		if err != nil {
			log.Error(err.Error())
			errMsg = ctx.Tr("auth.password_pwned_err")
		}
		ctx.Data["Err_Password"] = true
		ctx.RenderWithErr(errMsg, tplSignUp, &form)
		return
	}

	u := &models.User{
		Name:         form.UserName,
		Email:        form.Email,
		Passwd:       form.Password,
		IsActive:     !(setting.Service.RegisterEmailConfirm || setting.Service.RegisterManualConfirm),
		IsRestricted: setting.Service.DefaultUserIsRestricted,
	}

	if !createAndHandleCreatedUser(ctx, tplSignUp, form, u, nil, false) {
		// error already handled
		return
	}

	ctx.Flash.Success(ctx.Tr("auth.sign_up_successful"))
	handleSignInFull(ctx, u, false, true)
}

// createAndHandleCreatedUser calls createUserInContext and
// then handleUserCreated.
func createAndHandleCreatedUser(ctx *context.Context, tpl base.TplName, form interface{}, u *models.User, gothUser *goth.User, allowLink bool) bool {
	if !createUserInContext(ctx, tpl, form, u, gothUser, allowLink) {
		return false
	}
	return handleUserCreated(ctx, u, gothUser)
}

// createUserInContext creates a user and handles errors within a given context.
// Optionally a template can be specified.
func createUserInContext(ctx *context.Context, tpl base.TplName, form interface{}, u *models.User, gothUser *goth.User, allowLink bool) (ok bool) {
	if err := models.CreateUser(u); err != nil {
		if allowLink && (models.IsErrUserAlreadyExist(err) || models.IsErrEmailAlreadyUsed(err)) {
			if setting.OAuth2Client.AccountLinking == setting.OAuth2AccountLinkingAuto {
				var user *models.User
				user = &models.User{Name: u.Name}
				hasUser, err := models.GetUser(user)
				if !hasUser || err != nil {
					user = &models.User{Email: u.Email}
					hasUser, err = models.GetUser(user)
					if !hasUser || err != nil {
						ctx.ServerError("UserLinkAccount", err)
						return
					}
				}

				// TODO: probably we should respect 'remember' user's choice...
				linkAccount(ctx, user, *gothUser, true)
				return // user is already created here, all redirects are handled
			} else if setting.OAuth2Client.AccountLinking == setting.OAuth2AccountLinkingLogin {
				showLinkingLogin(ctx, *gothUser)
				return // user will be created only after linking login
			}
		}

		// handle error without template
		if len(tpl) == 0 {
			ctx.ServerError("CreateUser", err)
			return
		}

		// handle error with template
		switch {
		case models.IsErrUserAlreadyExist(err):
			ctx.Data["Err_UserName"] = true
			ctx.RenderWithErr(ctx.Tr("form.username_been_taken"), tpl, form)
		case models.IsErrEmailAlreadyUsed(err):
			ctx.Data["Err_Email"] = true
			ctx.RenderWithErr(ctx.Tr("form.email_been_used"), tpl, form)
		case models.IsErrEmailInvalid(err):
			ctx.Data["Err_Email"] = true
			ctx.RenderWithErr(ctx.Tr("form.email_invalid"), tpl, form)
		case models.IsErrNameReserved(err):
			ctx.Data["Err_UserName"] = true
			ctx.RenderWithErr(ctx.Tr("user.form.name_reserved", err.(models.ErrNameReserved).Name), tpl, form)
		case models.IsErrNamePatternNotAllowed(err):
			ctx.Data["Err_UserName"] = true
			ctx.RenderWithErr(ctx.Tr("user.form.name_pattern_not_allowed", err.(models.ErrNamePatternNotAllowed).Pattern), tpl, form)
		case models.IsErrNameCharsNotAllowed(err):
			ctx.Data["Err_UserName"] = true
			ctx.RenderWithErr(ctx.Tr("user.form.name_chars_not_allowed", err.(models.ErrNameCharsNotAllowed).Name), tpl, form)
		default:
			ctx.ServerError("CreateUser", err)
		}
		return
	}
	log.Trace("Account created: %s", u.Name)
	return true
}

// handleUserCreated does additional steps after a new user is created.
// It auto-sets admin for the only user, updates the optional external user and
// sends a confirmation email if required.
func handleUserCreated(ctx *context.Context, u *models.User, gothUser *goth.User) (ok bool) {
	// Auto-set admin for the only user.
	if models.CountUsers() == 1 {
		u.IsAdmin = true
		u.IsActive = true
		u.SetLastLogin()
		if err := models.UpdateUserCols(u, "is_admin", "is_active", "last_login_unix"); err != nil {
			ctx.ServerError("UpdateUser", err)
			return
		}
	}

	// update external user information
	if gothUser != nil {
		if err := models.UpdateExternalUser(u, *gothUser); err != nil {
			log.Error("UpdateExternalUser failed: %v", err)
		}
	}

	// Send confirmation email
	if !u.IsActive && u.ID > 1 {
		mailer.SendActivateAccountMail(ctx.Locale, u)

		ctx.Data["IsSendRegisterMail"] = true
		ctx.Data["Email"] = u.Email
		ctx.Data["ActiveCodeLives"] = timeutil.MinutesToFriendly(setting.Service.ActiveCodeLives, ctx.Locale.Language())
		ctx.HTML(http.StatusOK, TplActivate)

		if err := ctx.Cache.Put("MailResendLimit_"+u.LowerName, u.LowerName, 180); err != nil {
			log.Error("Set cache(MailResendLimit) fail: %v", err)
		}
		return
	}

	return true
}

// Activate render activate user page
func Activate(ctx *context.Context) {
	code := ctx.Query("code")

	if len(code) == 0 {
		ctx.Data["IsActivatePage"] = true
		if ctx.User == nil || ctx.User.IsActive {
			ctx.NotFound("invalid user", nil)
			return
		}
		// Resend confirmation email.
		if setting.Service.RegisterEmailConfirm {
			if ctx.Cache.IsExist("MailResendLimit_" + ctx.User.LowerName) {
				ctx.Data["ResendLimited"] = true
			} else {
				ctx.Data["ActiveCodeLives"] = timeutil.MinutesToFriendly(setting.Service.ActiveCodeLives, ctx.Locale.Language())
				mailer.SendActivateAccountMail(ctx.Locale, ctx.User)

				if err := ctx.Cache.Put("MailResendLimit_"+ctx.User.LowerName, ctx.User.LowerName, 180); err != nil {
					log.Error("Set cache(MailResendLimit) fail: %v", err)
				}
			}
		} else {
			ctx.Data["ServiceNotEnabled"] = true
		}
		ctx.HTML(http.StatusOK, TplActivate)
		return
	}

	user := models.VerifyUserActiveCode(code)
	// if code is wrong
	if user == nil {
		ctx.Data["IsActivateFailed"] = true
		ctx.HTML(http.StatusOK, TplActivate)
		return
	}

	// if account is local account, verify password
	if user.LoginSource == 0 {
		ctx.Data["Code"] = code
		ctx.Data["NeedsPassword"] = true
		ctx.HTML(http.StatusOK, TplActivate)
		return
	}

	handleAccountActivation(ctx, user)
}

// ActivatePost handles account activation with password check
func ActivatePost(ctx *context.Context) {
	code := ctx.Query("code")
	if len(code) == 0 {
		ctx.Redirect(setting.AppSubURL + "/user/activate")
		return
	}

	user := models.VerifyUserActiveCode(code)
	// if code is wrong
	if user == nil {
		ctx.Data["IsActivateFailed"] = true
		ctx.HTML(http.StatusOK, TplActivate)
		return
	}

	// if account is local account, verify password
	if user.LoginSource == 0 {
		password := ctx.Query("password")
		if len(password) == 0 {
			ctx.Data["Code"] = code
			ctx.Data["NeedsPassword"] = true
			ctx.HTML(http.StatusOK, TplActivate)
			return
		}
		if !user.ValidatePassword(password) {
			ctx.Data["IsActivateFailed"] = true
			ctx.HTML(http.StatusOK, TplActivate)
			return
		}
	}

	handleAccountActivation(ctx, user)
}

func handleAccountActivation(ctx *context.Context, user *models.User) {
	user.IsActive = true
	var err error
	if user.Rands, err = models.GetUserSalt(); err != nil {
		ctx.ServerError("UpdateUser", err)
		return
	}
	if err := models.UpdateUserCols(user, "is_active", "rands"); err != nil {
		if models.IsErrUserNotExist(err) {
			ctx.NotFound("UpdateUserCols", err)
		} else {
			ctx.ServerError("UpdateUser", err)
		}
		return
	}

	if err := models.ActivateUserEmail(user.ID, user.Email, true); err != nil {
		log.Error("Unable to activate email for user: %-v with email: %s: %v", user, user.Email, err)
		ctx.ServerError("ActivateUserEmail", err)
		return
	}

	log.Trace("User activated: %s", user.Name)

	if err := ctx.Session.Set("uid", user.ID); err != nil {
		log.Error("Error setting uid in session[%s]: %v", ctx.Session.ID(), err)
	}
	if err := ctx.Session.Set("uname", user.Name); err != nil {
		log.Error("Error setting uname in session[%s]: %v", ctx.Session.ID(), err)
	}
	if err := ctx.Session.Release(); err != nil {
		log.Error("Error storing session[%s]: %v", ctx.Session.ID(), err)
	}

	ctx.Flash.Success(ctx.Tr("auth.account_activated"))
	ctx.Redirect(setting.AppSubURL + "/")
}

// ActivateEmail render the activate email page
func ActivateEmail(ctx *context.Context) {
	code := ctx.Query("code")
	emailStr := ctx.Query("email")

	// Verify code.
	if email := models.VerifyActiveEmailCode(code, emailStr); email != nil {
		if err := email.Activate(); err != nil {
			ctx.ServerError("ActivateEmail", err)
		}

		log.Trace("Email activated: %s", email.Email)
		ctx.Flash.Success(ctx.Tr("settings.add_email_success"))

		if u, err := models.GetUserByID(email.UID); err != nil {
			log.Warn("GetUserByID: %d", email.UID)
		} else {
			// Allow user to validate more emails
			_ = ctx.Cache.Delete("MailResendLimit_" + u.LowerName)
		}
	}

	// FIXME: e-mail verification does not require the user to be logged in,
	// so this could be redirecting to the login page.
	// Should users be logged in automatically here? (consider 2FA requirements, etc.)
	ctx.Redirect(setting.AppSubURL + "/user/settings/account")
}

// ForgotPasswd render the forget password page
func ForgotPasswd(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("auth.forgot_password_title")

	if setting.MailService == nil {
		log.Warn(ctx.Tr("auth.disable_forgot_password_mail_admin"))
		ctx.Data["IsResetDisable"] = true
		ctx.HTML(http.StatusOK, tplForgotPassword)
		return
	}

	email := ctx.Query("email")
	ctx.Data["Email"] = email

	ctx.Data["IsResetRequest"] = true
	ctx.HTML(http.StatusOK, tplForgotPassword)
}

// ForgotPasswdPost response for forget password request
func ForgotPasswdPost(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("auth.forgot_password_title")

	if setting.MailService == nil {
		ctx.NotFound("ForgotPasswdPost", nil)
		return
	}
	ctx.Data["IsResetRequest"] = true

	email := ctx.Query("email")
	ctx.Data["Email"] = email

	u, err := models.GetUserByEmail(email)
	if err != nil {
		if models.IsErrUserNotExist(err) {
			ctx.Data["ResetPwdCodeLives"] = timeutil.MinutesToFriendly(setting.Service.ResetPwdCodeLives, ctx.Locale.Language())
			ctx.Data["IsResetSent"] = true
			ctx.HTML(http.StatusOK, tplForgotPassword)
			return
		}

		ctx.ServerError("user.ResetPasswd(check existence)", err)
		return
	}

	if !u.IsLocal() && !u.IsOAuth2() {
		ctx.Data["Err_Email"] = true
		ctx.RenderWithErr(ctx.Tr("auth.non_local_account"), tplForgotPassword, nil)
		return
	}

	if ctx.Cache.IsExist("MailResendLimit_" + u.LowerName) {
		ctx.Data["ResendLimited"] = true
		ctx.HTML(http.StatusOK, tplForgotPassword)
		return
	}

	mailer.SendResetPasswordMail(u)

	if err = ctx.Cache.Put("MailResendLimit_"+u.LowerName, u.LowerName, 180); err != nil {
		log.Error("Set cache(MailResendLimit) fail: %v", err)
	}

	ctx.Data["ResetPwdCodeLives"] = timeutil.MinutesToFriendly(setting.Service.ResetPwdCodeLives, ctx.Locale.Language())
	ctx.Data["IsResetSent"] = true
	ctx.HTML(http.StatusOK, tplForgotPassword)
}

func commonResetPassword(ctx *context.Context) (*models.User, *models.TwoFactor) {
	code := ctx.Query("code")

	ctx.Data["Title"] = ctx.Tr("auth.reset_password")
	ctx.Data["Code"] = code

	if nil != ctx.User {
		ctx.Data["user_signed_in"] = true
	}

	if len(code) == 0 {
		ctx.Flash.Error(ctx.Tr("auth.invalid_code"))
		return nil, nil
	}

	// Fail early, don't frustrate the user
	u := models.VerifyUserActiveCode(code)
	if u == nil {
		ctx.Flash.Error(ctx.Tr("auth.invalid_code"))
		return nil, nil
	}

	twofa, err := models.GetTwoFactorByUID(u.ID)
	if err != nil {
		if !models.IsErrTwoFactorNotEnrolled(err) {
			ctx.Error(http.StatusInternalServerError, "CommonResetPassword", err.Error())
			return nil, nil
		}
	} else {
		ctx.Data["has_two_factor"] = true
		ctx.Data["scratch_code"] = ctx.QueryBool("scratch_code")
	}

	// Show the user that they are affecting the account that they intended to
	ctx.Data["user_email"] = u.Email

	if nil != ctx.User && u.ID != ctx.User.ID {
		ctx.Flash.Error(ctx.Tr("auth.reset_password_wrong_user", ctx.User.Email, u.Email))
		return nil, nil
	}

	return u, twofa
}

// ResetPasswd render the account recovery page
func ResetPasswd(ctx *context.Context) {
	ctx.Data["IsResetForm"] = true

	commonResetPassword(ctx)
	if ctx.Written() {
		return
	}

	ctx.HTML(http.StatusOK, tplResetPassword)
}

// ResetPasswdPost response from account recovery request
func ResetPasswdPost(ctx *context.Context) {
	u, twofa := commonResetPassword(ctx)
	if ctx.Written() {
		return
	}

	if u == nil {
		// Flash error has been set
		ctx.HTML(http.StatusOK, tplResetPassword)
		return
	}

	// Validate password length.
	passwd := ctx.Query("password")
	if len(passwd) < setting.MinPasswordLength {
		ctx.Data["IsResetForm"] = true
		ctx.Data["Err_Password"] = true
		ctx.RenderWithErr(ctx.Tr("auth.password_too_short", setting.MinPasswordLength), tplResetPassword, nil)
		return
	} else if !password.IsComplexEnough(passwd) {
		ctx.Data["IsResetForm"] = true
		ctx.Data["Err_Password"] = true
		ctx.RenderWithErr(password.BuildComplexityError(ctx), tplResetPassword, nil)
		return
	} else if pwned, err := password.IsPwned(ctx, passwd); pwned || err != nil {
		errMsg := ctx.Tr("auth.password_pwned")
		if err != nil {
			log.Error(err.Error())
			errMsg = ctx.Tr("auth.password_pwned_err")
		}
		ctx.Data["IsResetForm"] = true
		ctx.Data["Err_Password"] = true
		ctx.RenderWithErr(errMsg, tplResetPassword, nil)
		return
	}

	// Handle two-factor
	regenerateScratchToken := false
	if twofa != nil {
		if ctx.QueryBool("scratch_code") {
			if !twofa.VerifyScratchToken(ctx.Query("token")) {
				ctx.Data["IsResetForm"] = true
				ctx.Data["Err_Token"] = true
				ctx.RenderWithErr(ctx.Tr("auth.twofa_scratch_token_incorrect"), tplResetPassword, nil)
				return
			}
			regenerateScratchToken = true
		} else {
			passcode := ctx.Query("passcode")
			ok, err := twofa.ValidateTOTP(passcode)
			if err != nil {
				ctx.Error(http.StatusInternalServerError, "ValidateTOTP", err.Error())
				return
			}
			if !ok || twofa.LastUsedPasscode == passcode {
				ctx.Data["IsResetForm"] = true
				ctx.Data["Err_Passcode"] = true
				ctx.RenderWithErr(ctx.Tr("auth.twofa_passcode_incorrect"), tplResetPassword, nil)
				return
			}

			twofa.LastUsedPasscode = passcode
			if err = models.UpdateTwoFactor(twofa); err != nil {
				ctx.ServerError("ResetPasswdPost: UpdateTwoFactor", err)
				return
			}
		}
	}
	var err error
	if u.Rands, err = models.GetUserSalt(); err != nil {
		ctx.ServerError("UpdateUser", err)
		return
	}
	if err = u.SetPassword(passwd); err != nil {
		ctx.ServerError("UpdateUser", err)
		return
	}
	u.MustChangePassword = false
	if err := models.UpdateUserCols(u, "must_change_password", "passwd", "passwd_hash_algo", "rands", "salt"); err != nil {
		ctx.ServerError("UpdateUser", err)
		return
	}

	log.Trace("User password reset: %s", u.Name)
	ctx.Data["IsResetFailed"] = true
	remember := len(ctx.Query("remember")) != 0

	if regenerateScratchToken {
		// Invalidate the scratch token.
		_, err = twofa.GenerateScratchToken()
		if err != nil {
			ctx.ServerError("UserSignIn", err)
			return
		}
		if err = models.UpdateTwoFactor(twofa); err != nil {
			ctx.ServerError("UserSignIn", err)
			return
		}

		handleSignInFull(ctx, u, remember, false)
		ctx.Flash.Info(ctx.Tr("auth.twofa_scratch_used"))
		ctx.Redirect(setting.AppSubURL + "/user/settings/security")
		return
	}

	handleSignInFull(ctx, u, remember, true)
}

// MustChangePassword renders the page to change a user's password
func MustChangePassword(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("auth.must_change_password")
	ctx.Data["ChangePasscodeLink"] = setting.AppSubURL + "/user/settings/change_password"
	ctx.Data["MustChangePassword"] = true
	ctx.HTML(http.StatusOK, tplMustChangePassword)
}

// MustChangePasswordPost response for updating a user's password after his/her
// account was created by an admin
func MustChangePasswordPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.MustChangePasswordForm)
	ctx.Data["Title"] = ctx.Tr("auth.must_change_password")
	ctx.Data["ChangePasscodeLink"] = setting.AppSubURL + "/user/settings/change_password"
	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplMustChangePassword)
		return
	}
	u := ctx.User
	// Make sure only requests for users who are eligible to change their password via
	// this method passes through
	if !u.MustChangePassword {
		ctx.ServerError("MustUpdatePassword", errors.New("cannot update password.. Please visit the settings page"))
		return
	}

	if form.Password != form.Retype {
		ctx.Data["Err_Password"] = true
		ctx.RenderWithErr(ctx.Tr("form.password_not_match"), tplMustChangePassword, &form)
		return
	}

	if len(form.Password) < setting.MinPasswordLength {
		ctx.Data["Err_Password"] = true
		ctx.RenderWithErr(ctx.Tr("auth.password_too_short", setting.MinPasswordLength), tplMustChangePassword, &form)
		return
	}

	var err error
	if err = u.SetPassword(form.Password); err != nil {
		ctx.ServerError("UpdateUser", err)
		return
	}

	u.MustChangePassword = false

	if err := models.UpdateUserCols(u, "must_change_password", "passwd", "passwd_hash_algo", "salt"); err != nil {
		ctx.ServerError("UpdateUser", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("settings.change_password_success"))

	log.Trace("User updated password: %s", u.Name)

	if redirectTo := ctx.GetCookie("redirect_to"); len(redirectTo) > 0 && !utils.IsExternalURL(redirectTo) {
		middleware.DeleteRedirectToCookie(ctx.Resp)
		ctx.RedirectToFirst(redirectTo)
		return
	}

	ctx.Redirect(setting.AppSubURL + "/")
}
