// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"encoding/base64"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/auth"
	"code.gitea.io/gitea/services/auth/source/oauth2"
	"code.gitea.io/gitea/services/forms"

	"gitea.com/go-chi/binding"
	"github.com/dgrijalva/jwt-go"
	jsoniter "github.com/json-iterator/go"
)

const (
	tplGrantAccess base.TplName = "user/auth/grant"
	tplGrantError  base.TplName = "user/auth/grant_error"
)

// TODO move error and responses to SDK or models

// AuthorizeErrorCode represents an error code specified in RFC 6749
type AuthorizeErrorCode string

const (
	// ErrorCodeInvalidRequest represents the according error in RFC 6749
	ErrorCodeInvalidRequest AuthorizeErrorCode = "invalid_request"
	// ErrorCodeUnauthorizedClient represents the according error in RFC 6749
	ErrorCodeUnauthorizedClient AuthorizeErrorCode = "unauthorized_client"
	// ErrorCodeAccessDenied represents the according error in RFC 6749
	ErrorCodeAccessDenied AuthorizeErrorCode = "access_denied"
	// ErrorCodeUnsupportedResponseType represents the according error in RFC 6749
	ErrorCodeUnsupportedResponseType AuthorizeErrorCode = "unsupported_response_type"
	// ErrorCodeInvalidScope represents the according error in RFC 6749
	ErrorCodeInvalidScope AuthorizeErrorCode = "invalid_scope"
	// ErrorCodeServerError represents the according error in RFC 6749
	ErrorCodeServerError AuthorizeErrorCode = "server_error"
	// ErrorCodeTemporaryUnavailable represents the according error in RFC 6749
	ErrorCodeTemporaryUnavailable AuthorizeErrorCode = "temporarily_unavailable"
)

// AuthorizeError represents an error type specified in RFC 6749
type AuthorizeError struct {
	ErrorCode        AuthorizeErrorCode `json:"error" form:"error"`
	ErrorDescription string
	State            string
}

// Error returns the error message
func (err AuthorizeError) Error() string {
	return fmt.Sprintf("%s: %s", err.ErrorCode, err.ErrorDescription)
}

// AccessTokenErrorCode represents an error code specified in RFC 6749
type AccessTokenErrorCode string

const (
	// AccessTokenErrorCodeInvalidRequest represents an error code specified in RFC 6749
	AccessTokenErrorCodeInvalidRequest AccessTokenErrorCode = "invalid_request"
	// AccessTokenErrorCodeInvalidClient represents an error code specified in RFC 6749
	AccessTokenErrorCodeInvalidClient = "invalid_client"
	// AccessTokenErrorCodeInvalidGrant represents an error code specified in RFC 6749
	AccessTokenErrorCodeInvalidGrant = "invalid_grant"
	// AccessTokenErrorCodeUnauthorizedClient represents an error code specified in RFC 6749
	AccessTokenErrorCodeUnauthorizedClient = "unauthorized_client"
	// AccessTokenErrorCodeUnsupportedGrantType represents an error code specified in RFC 6749
	AccessTokenErrorCodeUnsupportedGrantType = "unsupported_grant_type"
	// AccessTokenErrorCodeInvalidScope represents an error code specified in RFC 6749
	AccessTokenErrorCodeInvalidScope = "invalid_scope"
)

// AccessTokenError represents an error response specified in RFC 6749
type AccessTokenError struct {
	ErrorCode        AccessTokenErrorCode `json:"error" form:"error"`
	ErrorDescription string               `json:"error_description"`
}

// Error returns the error message
func (err AccessTokenError) Error() string {
	return fmt.Sprintf("%s: %s", err.ErrorCode, err.ErrorDescription)
}

// BearerTokenErrorCode represents an error code specified in RFC 6750
type BearerTokenErrorCode string

const (
	// BearerTokenErrorCodeInvalidRequest represents an error code specified in RFC 6750
	BearerTokenErrorCodeInvalidRequest BearerTokenErrorCode = "invalid_request"
	// BearerTokenErrorCodeInvalidToken represents an error code specified in RFC 6750
	BearerTokenErrorCodeInvalidToken BearerTokenErrorCode = "invalid_token"
	// BearerTokenErrorCodeInsufficientScope represents an error code specified in RFC 6750
	BearerTokenErrorCodeInsufficientScope BearerTokenErrorCode = "insufficient_scope"
)

// BearerTokenError represents an error response specified in RFC 6750
type BearerTokenError struct {
	ErrorCode        BearerTokenErrorCode `json:"error" form:"error"`
	ErrorDescription string               `json:"error_description"`
}

// TokenType specifies the kind of token
type TokenType string

const (
	// TokenTypeBearer represents a token type specified in RFC 6749
	TokenTypeBearer TokenType = "bearer"
	// TokenTypeMAC represents a token type specified in RFC 6749
	TokenTypeMAC = "mac"
)

// AccessTokenResponse represents a successful access token response
type AccessTokenResponse struct {
	AccessToken  string    `json:"access_token"`
	TokenType    TokenType `json:"token_type"`
	ExpiresIn    int64     `json:"expires_in"`
	RefreshToken string    `json:"refresh_token"`
	IDToken      string    `json:"id_token,omitempty"`
}

func newAccessTokenResponse(grant *models.OAuth2Grant, signingKey oauth2.JWTSigningKey) (*AccessTokenResponse, *AccessTokenError) {
	if setting.OAuth2.InvalidateRefreshTokens {
		if err := grant.IncreaseCounter(); err != nil {
			return nil, &AccessTokenError{
				ErrorCode:        AccessTokenErrorCodeInvalidGrant,
				ErrorDescription: "cannot increase the grant counter",
			}
		}
	}
	// generate access token to access the API
	expirationDate := timeutil.TimeStampNow().Add(setting.OAuth2.AccessTokenExpirationTime)
	accessToken := &oauth2.Token{
		GrantID: grant.ID,
		Type:    oauth2.TypeAccessToken,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationDate.AsTime().Unix(),
		},
	}
	signedAccessToken, err := accessToken.SignToken()
	if err != nil {
		return nil, &AccessTokenError{
			ErrorCode:        AccessTokenErrorCodeInvalidRequest,
			ErrorDescription: "cannot sign token",
		}
	}

	// generate refresh token to request an access token after it expired later
	refreshExpirationDate := timeutil.TimeStampNow().Add(setting.OAuth2.RefreshTokenExpirationTime * 60 * 60).AsTime().Unix()
	refreshToken := &oauth2.Token{
		GrantID: grant.ID,
		Counter: grant.Counter,
		Type:    oauth2.TypeRefreshToken,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: refreshExpirationDate,
		},
	}
	signedRefreshToken, err := refreshToken.SignToken()
	if err != nil {
		return nil, &AccessTokenError{
			ErrorCode:        AccessTokenErrorCodeInvalidRequest,
			ErrorDescription: "cannot sign token",
		}
	}

	// generate OpenID Connect id_token
	signedIDToken := ""
	if grant.ScopeContains("openid") {
		app, err := models.GetOAuth2ApplicationByID(grant.ApplicationID)
		if err != nil {
			return nil, &AccessTokenError{
				ErrorCode:        AccessTokenErrorCodeInvalidRequest,
				ErrorDescription: "cannot find application",
			}
		}
		err = app.LoadUser()
		if err != nil {
			if models.IsErrUserNotExist(err) {
				return nil, &AccessTokenError{
					ErrorCode:        AccessTokenErrorCodeInvalidRequest,
					ErrorDescription: "cannot find user",
				}
			}
			log.Error("Error loading user: %v", err)
			return nil, &AccessTokenError{
				ErrorCode:        AccessTokenErrorCodeInvalidRequest,
				ErrorDescription: "server error",
			}
		}

		idToken := &oauth2.OIDCToken{
			StandardClaims: jwt.StandardClaims{
				ExpiresAt: expirationDate.AsTime().Unix(),
				Issuer:    setting.AppURL,
				Audience:  app.ClientID,
				Subject:   fmt.Sprint(grant.UserID),
			},
			Nonce: grant.Nonce,
		}
		if grant.ScopeContains("profile") {
			idToken.Name = app.User.FullName
			idToken.PreferredUsername = app.User.Name
			idToken.Profile = app.User.HTMLURL()
			idToken.Picture = app.User.AvatarLink()
			idToken.Website = app.User.Website
			idToken.Locale = app.User.Language
			idToken.UpdatedAt = app.User.UpdatedUnix
		}
		if grant.ScopeContains("email") {
			idToken.Email = app.User.Email
			idToken.EmailVerified = app.User.IsActive
		}

		signedIDToken, err = idToken.SignToken(signingKey)
		if err != nil {
			return nil, &AccessTokenError{
				ErrorCode:        AccessTokenErrorCodeInvalidRequest,
				ErrorDescription: "cannot sign token",
			}
		}
	}

	return &AccessTokenResponse{
		AccessToken:  signedAccessToken,
		TokenType:    TokenTypeBearer,
		ExpiresIn:    setting.OAuth2.AccessTokenExpirationTime,
		RefreshToken: signedRefreshToken,
		IDToken:      signedIDToken,
	}, nil
}

type userInfoResponse struct {
	Sub      string `json:"sub"`
	Name     string `json:"name"`
	Username string `json:"preferred_username"`
	Email    string `json:"email"`
	Picture  string `json:"picture"`
}

// InfoOAuth manages request for userinfo endpoint
func InfoOAuth(ctx *context.Context) {
	header := ctx.Req.Header.Get("Authorization")
	auths := strings.Fields(header)
	if len(auths) != 2 || auths[0] != "Bearer" {
		ctx.HandleText(http.StatusUnauthorized, "no valid auth token authorization")
		return
	}
	uid := auth.CheckOAuthAccessToken(auths[1])
	if uid == 0 {
		handleBearerTokenError(ctx, BearerTokenError{
			ErrorCode:        BearerTokenErrorCodeInvalidToken,
			ErrorDescription: "Access token not assigned to any user",
		})
		return
	}
	authUser, err := models.GetUserByID(uid)
	if err != nil {
		ctx.ServerError("GetUserByID", err)
		return
	}
	response := &userInfoResponse{
		Sub:      fmt.Sprint(authUser.ID),
		Name:     authUser.FullName,
		Username: authUser.Name,
		Email:    authUser.Email,
		Picture:  authUser.AvatarLink(),
	}
	ctx.JSON(http.StatusOK, response)
}

// AuthorizeOAuth manages authorize requests
func AuthorizeOAuth(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.AuthorizationForm)
	errs := binding.Errors{}
	errs = form.Validate(ctx.Req, errs)
	if len(errs) > 0 {
		errstring := ""
		for _, e := range errs {
			errstring += e.Error() + "\n"
		}
		ctx.ServerError("AuthorizeOAuth: Validate: ", fmt.Errorf("errors occurred during validation: %s", errstring))
		return
	}

	app, err := models.GetOAuth2ApplicationByClientID(form.ClientID)
	if err != nil {
		if models.IsErrOauthClientIDInvalid(err) {
			handleAuthorizeError(ctx, AuthorizeError{
				ErrorCode:        ErrorCodeUnauthorizedClient,
				ErrorDescription: "Client ID not registered",
				State:            form.State,
			}, "")
			return
		}
		ctx.ServerError("GetOAuth2ApplicationByClientID", err)
		return
	}
	if err := app.LoadUser(); err != nil {
		ctx.ServerError("LoadUser", err)
		return
	}

	if !app.ContainsRedirectURI(form.RedirectURI) {
		handleAuthorizeError(ctx, AuthorizeError{
			ErrorCode:        ErrorCodeInvalidRequest,
			ErrorDescription: "Unregistered Redirect URI",
			State:            form.State,
		}, "")
		return
	}

	if form.ResponseType != "code" {
		handleAuthorizeError(ctx, AuthorizeError{
			ErrorCode:        ErrorCodeUnsupportedResponseType,
			ErrorDescription: "Only code response type is supported.",
			State:            form.State,
		}, form.RedirectURI)
		return
	}

	// pkce support
	switch form.CodeChallengeMethod {
	case "S256":
	case "plain":
		if err := ctx.Session.Set("CodeChallengeMethod", form.CodeChallengeMethod); err != nil {
			handleAuthorizeError(ctx, AuthorizeError{
				ErrorCode:        ErrorCodeServerError,
				ErrorDescription: "cannot set code challenge method",
				State:            form.State,
			}, form.RedirectURI)
			return
		}
		if err := ctx.Session.Set("CodeChallengeMethod", form.CodeChallenge); err != nil {
			handleAuthorizeError(ctx, AuthorizeError{
				ErrorCode:        ErrorCodeServerError,
				ErrorDescription: "cannot set code challenge",
				State:            form.State,
			}, form.RedirectURI)
			return
		}
		// Here we're just going to try to release the session early
		if err := ctx.Session.Release(); err != nil {
			// we'll tolerate errors here as they *should* get saved elsewhere
			log.Error("Unable to save changes to the session: %v", err)
		}
	case "":
		break
	default:
		handleAuthorizeError(ctx, AuthorizeError{
			ErrorCode:        ErrorCodeInvalidRequest,
			ErrorDescription: "unsupported code challenge method",
			State:            form.State,
		}, form.RedirectURI)
		return
	}

	grant, err := app.GetGrantByUserID(ctx.User.ID)
	if err != nil {
		handleServerError(ctx, form.State, form.RedirectURI)
		return
	}

	// Redirect if user already granted access
	if grant != nil {
		code, err := grant.GenerateNewAuthorizationCode(form.RedirectURI, form.CodeChallenge, form.CodeChallengeMethod)
		if err != nil {
			handleServerError(ctx, form.State, form.RedirectURI)
			return
		}
		redirect, err := code.GenerateRedirectURI(form.State)
		if err != nil {
			handleServerError(ctx, form.State, form.RedirectURI)
			return
		}
		// Update nonce to reflect the new session
		if len(form.Nonce) > 0 {
			err := grant.SetNonce(form.Nonce)
			if err != nil {
				log.Error("Unable to update nonce: %v", err)
			}
		}
		ctx.Redirect(redirect.String(), 302)
		return
	}

	// show authorize page to grant access
	ctx.Data["Application"] = app
	ctx.Data["RedirectURI"] = form.RedirectURI
	ctx.Data["State"] = form.State
	ctx.Data["Scope"] = form.Scope
	ctx.Data["Nonce"] = form.Nonce
	ctx.Data["ApplicationUserLink"] = "<a href=\"" + html.EscapeString(setting.AppURL) + html.EscapeString(url.PathEscape(app.User.LowerName)) + "\">@" + html.EscapeString(app.User.Name) + "</a>"
	ctx.Data["ApplicationRedirectDomainHTML"] = "<strong>" + html.EscapeString(form.RedirectURI) + "</strong>"
	// TODO document SESSION <=> FORM
	err = ctx.Session.Set("client_id", app.ClientID)
	if err != nil {
		handleServerError(ctx, form.State, form.RedirectURI)
		log.Error(err.Error())
		return
	}
	err = ctx.Session.Set("redirect_uri", form.RedirectURI)
	if err != nil {
		handleServerError(ctx, form.State, form.RedirectURI)
		log.Error(err.Error())
		return
	}
	err = ctx.Session.Set("state", form.State)
	if err != nil {
		handleServerError(ctx, form.State, form.RedirectURI)
		log.Error(err.Error())
		return
	}
	// Here we're just going to try to release the session early
	if err := ctx.Session.Release(); err != nil {
		// we'll tolerate errors here as they *should* get saved elsewhere
		log.Error("Unable to save changes to the session: %v", err)
	}
	ctx.HTML(http.StatusOK, tplGrantAccess)
}

// GrantApplicationOAuth manages the post request submitted when a user grants access to an application
func GrantApplicationOAuth(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.GrantApplicationForm)
	if ctx.Session.Get("client_id") != form.ClientID || ctx.Session.Get("state") != form.State ||
		ctx.Session.Get("redirect_uri") != form.RedirectURI {
		ctx.Error(http.StatusBadRequest)
		return
	}
	app, err := models.GetOAuth2ApplicationByClientID(form.ClientID)
	if err != nil {
		ctx.ServerError("GetOAuth2ApplicationByClientID", err)
		return
	}
	grant, err := app.CreateGrant(ctx.User.ID, form.Scope)
	if err != nil {
		handleAuthorizeError(ctx, AuthorizeError{
			State:            form.State,
			ErrorDescription: "cannot create grant for user",
			ErrorCode:        ErrorCodeServerError,
		}, form.RedirectURI)
		return
	}
	if len(form.Nonce) > 0 {
		err := grant.SetNonce(form.Nonce)
		if err != nil {
			log.Error("Unable to update nonce: %v", err)
		}
	}

	var codeChallenge, codeChallengeMethod string
	codeChallenge, _ = ctx.Session.Get("CodeChallenge").(string)
	codeChallengeMethod, _ = ctx.Session.Get("CodeChallengeMethod").(string)

	code, err := grant.GenerateNewAuthorizationCode(form.RedirectURI, codeChallenge, codeChallengeMethod)
	if err != nil {
		handleServerError(ctx, form.State, form.RedirectURI)
		return
	}
	redirect, err := code.GenerateRedirectURI(form.State)
	if err != nil {
		handleServerError(ctx, form.State, form.RedirectURI)
		return
	}
	ctx.Redirect(redirect.String(), 302)
}

// OIDCWellKnown generates JSON so OIDC clients know Gitea's capabilities
func OIDCWellKnown(ctx *context.Context) {
	t := ctx.Render.TemplateLookup("user/auth/oidc_wellknown")
	ctx.Resp.Header().Set("Content-Type", "application/json")
	ctx.Data["SigningKey"] = oauth2.DefaultSigningKey
	if err := t.Execute(ctx.Resp, ctx.Data); err != nil {
		log.Error("%v", err)
		ctx.Error(http.StatusInternalServerError)
	}
}

// OIDCKeys generates the JSON Web Key Set
func OIDCKeys(ctx *context.Context) {
	jwk, err := oauth2.DefaultSigningKey.ToJWK()
	if err != nil {
		log.Error("Error converting signing key to JWK: %v", err)
		ctx.Error(http.StatusInternalServerError)
		return
	}

	jwk["use"] = "sig"

	jwks := map[string][]map[string]string{
		"keys": {
			jwk,
		},
	}

	ctx.Resp.Header().Set("Content-Type", "application/json")
	enc := jsoniter.NewEncoder(ctx.Resp)
	if err := enc.Encode(jwks); err != nil {
		log.Error("Failed to encode representation as json. Error: %v", err)
	}
}

// AccessTokenOAuth manages all access token requests by the client
func AccessTokenOAuth(ctx *context.Context) {
	form := *web.GetForm(ctx).(*forms.AccessTokenForm)
	if form.ClientID == "" {
		authHeader := ctx.Req.Header.Get("Authorization")
		authContent := strings.SplitN(authHeader, " ", 2)
		if len(authContent) == 2 && authContent[0] == "Basic" {
			payload, err := base64.StdEncoding.DecodeString(authContent[1])
			if err != nil {
				handleAccessTokenError(ctx, AccessTokenError{
					ErrorCode:        AccessTokenErrorCodeInvalidRequest,
					ErrorDescription: "cannot parse basic auth header",
				})
				return
			}
			pair := strings.SplitN(string(payload), ":", 2)
			if len(pair) != 2 {
				handleAccessTokenError(ctx, AccessTokenError{
					ErrorCode:        AccessTokenErrorCodeInvalidRequest,
					ErrorDescription: "cannot parse basic auth header",
				})
				return
			}
			form.ClientID = pair[0]
			form.ClientSecret = pair[1]
		}
	}

	signingKey := oauth2.DefaultSigningKey
	if signingKey.IsSymmetric() {
		clientKey, err := oauth2.CreateJWTSingingKey(signingKey.SigningMethod().Alg(), []byte(form.ClientSecret))
		if err != nil {
			handleAccessTokenError(ctx, AccessTokenError{
				ErrorCode:        AccessTokenErrorCodeInvalidRequest,
				ErrorDescription: "Error creating signing key",
			})
			return
		}
		signingKey = clientKey
	}

	switch form.GrantType {
	case "refresh_token":
		handleRefreshToken(ctx, form, signingKey)
	case "authorization_code":
		handleAuthorizationCode(ctx, form, signingKey)
	default:
		handleAccessTokenError(ctx, AccessTokenError{
			ErrorCode:        AccessTokenErrorCodeUnsupportedGrantType,
			ErrorDescription: "Only refresh_token or authorization_code grant type is supported",
		})
	}
}

func handleRefreshToken(ctx *context.Context, form forms.AccessTokenForm, signingKey oauth2.JWTSigningKey) {
	token, err := oauth2.ParseToken(form.RefreshToken)
	if err != nil {
		handleAccessTokenError(ctx, AccessTokenError{
			ErrorCode:        AccessTokenErrorCodeUnauthorizedClient,
			ErrorDescription: "client is not authorized",
		})
		return
	}
	// get grant before increasing counter
	grant, err := models.GetOAuth2GrantByID(token.GrantID)
	if err != nil || grant == nil {
		handleAccessTokenError(ctx, AccessTokenError{
			ErrorCode:        AccessTokenErrorCodeInvalidGrant,
			ErrorDescription: "grant does not exist",
		})
		return
	}

	// check if token got already used
	if setting.OAuth2.InvalidateRefreshTokens && (grant.Counter != token.Counter || token.Counter == 0) {
		handleAccessTokenError(ctx, AccessTokenError{
			ErrorCode:        AccessTokenErrorCodeUnauthorizedClient,
			ErrorDescription: "token was already used",
		})
		log.Warn("A client tried to use a refresh token for grant_id = %d was used twice!", grant.ID)
		return
	}
	accessToken, tokenErr := newAccessTokenResponse(grant, signingKey)
	if tokenErr != nil {
		handleAccessTokenError(ctx, *tokenErr)
		return
	}
	ctx.JSON(http.StatusOK, accessToken)
}

func handleAuthorizationCode(ctx *context.Context, form forms.AccessTokenForm, signingKey oauth2.JWTSigningKey) {
	app, err := models.GetOAuth2ApplicationByClientID(form.ClientID)
	if err != nil {
		handleAccessTokenError(ctx, AccessTokenError{
			ErrorCode:        AccessTokenErrorCodeInvalidClient,
			ErrorDescription: fmt.Sprintf("cannot load client with client id: '%s'", form.ClientID),
		})
		return
	}
	if !app.ValidateClientSecret([]byte(form.ClientSecret)) {
		handleAccessTokenError(ctx, AccessTokenError{
			ErrorCode:        AccessTokenErrorCodeUnauthorizedClient,
			ErrorDescription: "client is not authorized",
		})
		return
	}
	if form.RedirectURI != "" && !app.ContainsRedirectURI(form.RedirectURI) {
		handleAccessTokenError(ctx, AccessTokenError{
			ErrorCode:        AccessTokenErrorCodeUnauthorizedClient,
			ErrorDescription: "client is not authorized",
		})
		return
	}
	authorizationCode, err := models.GetOAuth2AuthorizationByCode(form.Code)
	if err != nil || authorizationCode == nil {
		handleAccessTokenError(ctx, AccessTokenError{
			ErrorCode:        AccessTokenErrorCodeUnauthorizedClient,
			ErrorDescription: "client is not authorized",
		})
		return
	}
	// check if code verifier authorizes the client, PKCE support
	if !authorizationCode.ValidateCodeChallenge(form.CodeVerifier) {
		handleAccessTokenError(ctx, AccessTokenError{
			ErrorCode:        AccessTokenErrorCodeUnauthorizedClient,
			ErrorDescription: "client is not authorized",
		})
		return
	}
	// check if granted for this application
	if authorizationCode.Grant.ApplicationID != app.ID {
		handleAccessTokenError(ctx, AccessTokenError{
			ErrorCode:        AccessTokenErrorCodeInvalidGrant,
			ErrorDescription: "invalid grant",
		})
		return
	}
	// remove token from database to deny duplicate usage
	if err := authorizationCode.Invalidate(); err != nil {
		handleAccessTokenError(ctx, AccessTokenError{
			ErrorCode:        AccessTokenErrorCodeInvalidRequest,
			ErrorDescription: "cannot proceed your request",
		})
	}
	resp, tokenErr := newAccessTokenResponse(authorizationCode.Grant, signingKey)
	if tokenErr != nil {
		handleAccessTokenError(ctx, *tokenErr)
		return
	}
	// send successful response
	ctx.JSON(http.StatusOK, resp)
}

func handleAccessTokenError(ctx *context.Context, acErr AccessTokenError) {
	ctx.JSON(http.StatusBadRequest, acErr)
}

func handleServerError(ctx *context.Context, state string, redirectURI string) {
	handleAuthorizeError(ctx, AuthorizeError{
		ErrorCode:        ErrorCodeServerError,
		ErrorDescription: "A server error occurred",
		State:            state,
	}, redirectURI)
}

func handleAuthorizeError(ctx *context.Context, authErr AuthorizeError, redirectURI string) {
	if redirectURI == "" {
		log.Warn("Authorization failed: %v", authErr.ErrorDescription)
		ctx.Data["Error"] = authErr
		ctx.HTML(400, tplGrantError)
		return
	}
	redirect, err := url.Parse(redirectURI)
	if err != nil {
		ctx.ServerError("url.Parse", err)
		return
	}
	q := redirect.Query()
	q.Set("error", string(authErr.ErrorCode))
	q.Set("error_description", authErr.ErrorDescription)
	q.Set("state", authErr.State)
	redirect.RawQuery = q.Encode()
	ctx.Redirect(redirect.String(), 302)
}

func handleBearerTokenError(ctx *context.Context, beErr BearerTokenError) {
	ctx.Resp.Header().Set("WWW-Authenticate", fmt.Sprintf("Bearer realm=\"\", error=\"%s\", error_description=\"%s\"", beErr.ErrorCode, beErr.ErrorDescription))
	switch beErr.ErrorCode {
	case BearerTokenErrorCodeInvalidRequest:
		ctx.JSON(http.StatusBadRequest, beErr)
	case BearerTokenErrorCodeInvalidToken:
		ctx.JSON(http.StatusUnauthorized, beErr)
	case BearerTokenErrorCodeInsufficientScope:
		ctx.JSON(http.StatusForbidden, beErr)
	default:
		log.Error("Invalid BearerTokenErrorCode: %v", beErr.ErrorCode)
		ctx.ServerError("Unhandled BearerTokenError", fmt.Errorf("BearerTokenError: error=\"%v\", error_description=\"%v\"", beErr.ErrorCode, beErr.ErrorDescription))
	}
}
