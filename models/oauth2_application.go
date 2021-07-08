// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/auth/oauth2"
	"code.gitea.io/gitea/modules/secret"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"github.com/dgrijalva/jwt-go"
	uuid "github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"xorm.io/xorm"
)

// OAuth2Application represents an OAuth2 client (RFC 6749)
type OAuth2Application struct {
	ID   int64 `xorm:"pk autoincr"`
	UID  int64 `xorm:"INDEX"`
	User *User `xorm:"-"`

	Name string

	ClientID     string `xorm:"unique"`
	ClientSecret string

	RedirectURIs []string `xorm:"redirect_uris JSON TEXT"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

// TableName sets the table name to `oauth2_application`
func (app *OAuth2Application) TableName() string {
	return "oauth2_application"
}

// PrimaryRedirectURI returns the first redirect uri or an empty string if empty
func (app *OAuth2Application) PrimaryRedirectURI() string {
	if len(app.RedirectURIs) == 0 {
		return ""
	}
	return app.RedirectURIs[0]
}

// LoadUser will load User by UID
func (app *OAuth2Application) LoadUser() (err error) {
	if app.User == nil {
		app.User, err = GetUserByID(app.UID)
	}
	return
}

// ContainsRedirectURI checks if redirectURI is allowed for app
func (app *OAuth2Application) ContainsRedirectURI(redirectURI string) bool {
	return util.IsStringInSlice(redirectURI, app.RedirectURIs, true)
}

// GenerateClientSecret will generate the client secret and returns the plaintext and saves the hash at the database
func (app *OAuth2Application) GenerateClientSecret() (string, error) {
	clientSecret, err := secret.New()
	if err != nil {
		return "", err
	}
	hashedSecret, err := bcrypt.GenerateFromPassword([]byte(clientSecret), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	app.ClientSecret = string(hashedSecret)
	if _, err := x.ID(app.ID).Cols("client_secret").Update(app); err != nil {
		return "", err
	}
	return clientSecret, nil
}

// ValidateClientSecret validates the given secret by the hash saved in database
func (app *OAuth2Application) ValidateClientSecret(secret []byte) bool {
	return bcrypt.CompareHashAndPassword([]byte(app.ClientSecret), secret) == nil
}

// GetGrantByUserID returns a OAuth2Grant by its user and application ID
func (app *OAuth2Application) GetGrantByUserID(userID int64) (*OAuth2Grant, error) {
	return app.getGrantByUserID(x, userID)
}

func (app *OAuth2Application) getGrantByUserID(e Engine, userID int64) (grant *OAuth2Grant, err error) {
	grant = new(OAuth2Grant)
	if has, err := e.Where("user_id = ? AND application_id = ?", userID, app.ID).Get(grant); err != nil {
		return nil, err
	} else if !has {
		return nil, nil
	}
	return grant, nil
}

// CreateGrant generates a grant for an user
func (app *OAuth2Application) CreateGrant(userID int64, scope string) (*OAuth2Grant, error) {
	return app.createGrant(x, userID, scope)
}

func (app *OAuth2Application) createGrant(e Engine, userID int64, scope string) (*OAuth2Grant, error) {
	grant := &OAuth2Grant{
		ApplicationID: app.ID,
		UserID:        userID,
		Scope:         scope,
	}
	_, err := e.Insert(grant)
	if err != nil {
		return nil, err
	}
	return grant, nil
}

// GetOAuth2ApplicationByClientID returns the oauth2 application with the given client_id. Returns an error if not found.
func GetOAuth2ApplicationByClientID(clientID string) (app *OAuth2Application, err error) {
	return getOAuth2ApplicationByClientID(x, clientID)
}

func getOAuth2ApplicationByClientID(e Engine, clientID string) (app *OAuth2Application, err error) {
	app = new(OAuth2Application)
	has, err := e.Where("client_id = ?", clientID).Get(app)
	if !has {
		return nil, ErrOAuthClientIDInvalid{ClientID: clientID}
	}
	return
}

// GetOAuth2ApplicationByID returns the oauth2 application with the given id. Returns an error if not found.
func GetOAuth2ApplicationByID(id int64) (app *OAuth2Application, err error) {
	return getOAuth2ApplicationByID(x, id)
}

func getOAuth2ApplicationByID(e Engine, id int64) (app *OAuth2Application, err error) {
	app = new(OAuth2Application)
	has, err := e.ID(id).Get(app)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrOAuthApplicationNotFound{ID: id}
	}
	return app, nil
}

// GetOAuth2ApplicationsByUserID returns all oauth2 applications owned by the user
func GetOAuth2ApplicationsByUserID(userID int64) (apps []*OAuth2Application, err error) {
	return getOAuth2ApplicationsByUserID(x, userID)
}

func getOAuth2ApplicationsByUserID(e Engine, userID int64) (apps []*OAuth2Application, err error) {
	apps = make([]*OAuth2Application, 0)
	err = e.Where("uid = ?", userID).Find(&apps)
	return
}

// CreateOAuth2ApplicationOptions holds options to create an oauth2 application
type CreateOAuth2ApplicationOptions struct {
	Name         string
	UserID       int64
	RedirectURIs []string
}

// CreateOAuth2Application inserts a new oauth2 application
func CreateOAuth2Application(opts CreateOAuth2ApplicationOptions) (*OAuth2Application, error) {
	return createOAuth2Application(x, opts)
}

func createOAuth2Application(e Engine, opts CreateOAuth2ApplicationOptions) (*OAuth2Application, error) {
	clientID := uuid.New().String()
	app := &OAuth2Application{
		UID:          opts.UserID,
		Name:         opts.Name,
		ClientID:     clientID,
		RedirectURIs: opts.RedirectURIs,
	}
	if _, err := e.Insert(app); err != nil {
		return nil, err
	}
	return app, nil
}

// UpdateOAuth2ApplicationOptions holds options to update an oauth2 application
type UpdateOAuth2ApplicationOptions struct {
	ID           int64
	Name         string
	UserID       int64
	RedirectURIs []string
}

// UpdateOAuth2Application updates an oauth2 application
func UpdateOAuth2Application(opts UpdateOAuth2ApplicationOptions) (*OAuth2Application, error) {
	sess := x.NewSession()
	if err := sess.Begin(); err != nil {
		return nil, err
	}
	defer sess.Close()

	app, err := getOAuth2ApplicationByID(sess, opts.ID)
	if err != nil {
		return nil, err
	}
	if app.UID != opts.UserID {
		return nil, fmt.Errorf("UID mismatch")
	}

	app.Name = opts.Name
	app.RedirectURIs = opts.RedirectURIs

	if err = updateOAuth2Application(sess, app); err != nil {
		return nil, err
	}
	app.ClientSecret = ""

	return app, sess.Commit()
}

func updateOAuth2Application(e Engine, app *OAuth2Application) error {
	if _, err := e.ID(app.ID).Update(app); err != nil {
		return err
	}
	return nil
}

func deleteOAuth2Application(sess *xorm.Session, id, userid int64) error {
	if deleted, err := sess.Delete(&OAuth2Application{ID: id, UID: userid}); err != nil {
		return err
	} else if deleted == 0 {
		return ErrOAuthApplicationNotFound{ID: id}
	}
	codes := make([]*OAuth2AuthorizationCode, 0)
	// delete correlating auth codes
	if err := sess.Join("INNER", "oauth2_grant",
		"oauth2_authorization_code.grant_id = oauth2_grant.id AND oauth2_grant.application_id = ?", id).Find(&codes); err != nil {
		return err
	}
	codeIDs := make([]int64, 0)
	for _, grant := range codes {
		codeIDs = append(codeIDs, grant.ID)
	}

	if _, err := sess.In("id", codeIDs).Delete(new(OAuth2AuthorizationCode)); err != nil {
		return err
	}

	if _, err := sess.Where("application_id = ?", id).Delete(new(OAuth2Grant)); err != nil {
		return err
	}
	return nil
}

// DeleteOAuth2Application deletes the application with the given id and the grants and auth codes related to it. It checks if the userid was the creator of the app.
func DeleteOAuth2Application(id, userid int64) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	if err := deleteOAuth2Application(sess, id, userid); err != nil {
		return err
	}
	return sess.Commit()
}

// ListOAuth2Applications returns a list of oauth2 applications belongs to given user.
func ListOAuth2Applications(uid int64, listOptions ListOptions) ([]*OAuth2Application, error) {
	sess := x.
		Where("uid=?", uid).
		Desc("id")

	if listOptions.Page != 0 {
		sess = listOptions.setSessionPagination(sess)

		apps := make([]*OAuth2Application, 0, listOptions.PageSize)
		return apps, sess.Find(&apps)
	}

	apps := make([]*OAuth2Application, 0, 5)
	return apps, sess.Find(&apps)
}

//////////////////////////////////////////////////////

// OAuth2AuthorizationCode is a code to obtain an access token in combination with the client secret once. It has a limited lifetime.
type OAuth2AuthorizationCode struct {
	ID                  int64        `xorm:"pk autoincr"`
	Grant               *OAuth2Grant `xorm:"-"`
	GrantID             int64
	Code                string `xorm:"INDEX unique"`
	CodeChallenge       string
	CodeChallengeMethod string
	RedirectURI         string
	ValidUntil          timeutil.TimeStamp `xorm:"index"`
}

// TableName sets the table name to `oauth2_authorization_code`
func (code *OAuth2AuthorizationCode) TableName() string {
	return "oauth2_authorization_code"
}

// GenerateRedirectURI generates a redirect URI for a successful authorization request. State will be used if not empty.
func (code *OAuth2AuthorizationCode) GenerateRedirectURI(state string) (redirect *url.URL, err error) {
	if redirect, err = url.Parse(code.RedirectURI); err != nil {
		return
	}
	q := redirect.Query()
	if state != "" {
		q.Set("state", state)
	}
	q.Set("code", code.Code)
	redirect.RawQuery = q.Encode()
	return
}

// Invalidate deletes the auth code from the database to invalidate this code
func (code *OAuth2AuthorizationCode) Invalidate() error {
	return code.invalidate(x)
}

func (code *OAuth2AuthorizationCode) invalidate(e Engine) error {
	_, err := e.Delete(code)
	return err
}

// ValidateCodeChallenge validates the given verifier against the saved code challenge. This is part of the PKCE implementation.
func (code *OAuth2AuthorizationCode) ValidateCodeChallenge(verifier string) bool {
	return code.validateCodeChallenge(verifier)
}

func (code *OAuth2AuthorizationCode) validateCodeChallenge(verifier string) bool {
	switch code.CodeChallengeMethod {
	case "S256":
		// base64url(SHA256(verifier)) see https://tools.ietf.org/html/rfc7636#section-4.6
		h := sha256.Sum256([]byte(verifier))
		hashedVerifier := base64.RawURLEncoding.EncodeToString(h[:])
		return hashedVerifier == code.CodeChallenge
	case "plain":
		return verifier == code.CodeChallenge
	case "":
		return true
	default:
		// unsupported method -> return false
		return false
	}
}

// GetOAuth2AuthorizationByCode returns an authorization by its code
func GetOAuth2AuthorizationByCode(code string) (*OAuth2AuthorizationCode, error) {
	return getOAuth2AuthorizationByCode(x, code)
}

func getOAuth2AuthorizationByCode(e Engine, code string) (auth *OAuth2AuthorizationCode, err error) {
	auth = new(OAuth2AuthorizationCode)
	if has, err := e.Where("code = ?", code).Get(auth); err != nil {
		return nil, err
	} else if !has {
		return nil, nil
	}
	auth.Grant = new(OAuth2Grant)
	if has, err := e.ID(auth.GrantID).Get(auth.Grant); err != nil {
		return nil, err
	} else if !has {
		return nil, nil
	}
	return auth, nil
}

//////////////////////////////////////////////////////

// OAuth2Grant represents the permission of an user for a specific application to access resources
type OAuth2Grant struct {
	ID            int64              `xorm:"pk autoincr"`
	UserID        int64              `xorm:"INDEX unique(user_application)"`
	Application   *OAuth2Application `xorm:"-"`
	ApplicationID int64              `xorm:"INDEX unique(user_application)"`
	Counter       int64              `xorm:"NOT NULL DEFAULT 1"`
	Scope         string             `xorm:"TEXT"`
	Nonce         string             `xorm:"TEXT"`
	CreatedUnix   timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix   timeutil.TimeStamp `xorm:"updated"`
}

// TableName sets the table name to `oauth2_grant`
func (grant *OAuth2Grant) TableName() string {
	return "oauth2_grant"
}

// GenerateNewAuthorizationCode generates a new authorization code for a grant and saves it to the database
func (grant *OAuth2Grant) GenerateNewAuthorizationCode(redirectURI, codeChallenge, codeChallengeMethod string) (*OAuth2AuthorizationCode, error) {
	return grant.generateNewAuthorizationCode(x, redirectURI, codeChallenge, codeChallengeMethod)
}

func (grant *OAuth2Grant) generateNewAuthorizationCode(e Engine, redirectURI, codeChallenge, codeChallengeMethod string) (code *OAuth2AuthorizationCode, err error) {
	var codeSecret string
	if codeSecret, err = secret.New(); err != nil {
		return &OAuth2AuthorizationCode{}, err
	}
	code = &OAuth2AuthorizationCode{
		Grant:               grant,
		GrantID:             grant.ID,
		RedirectURI:         redirectURI,
		Code:                codeSecret,
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: codeChallengeMethod,
	}
	if _, err := e.Insert(code); err != nil {
		return nil, err
	}
	return code, nil
}

// IncreaseCounter increases the counter and updates the grant
func (grant *OAuth2Grant) IncreaseCounter() error {
	return grant.increaseCount(x)
}

func (grant *OAuth2Grant) increaseCount(e Engine) error {
	_, err := e.ID(grant.ID).Incr("counter").Update(new(OAuth2Grant))
	if err != nil {
		return err
	}
	updatedGrant, err := getOAuth2GrantByID(e, grant.ID)
	if err != nil {
		return err
	}
	grant.Counter = updatedGrant.Counter
	return nil
}

// ScopeContains returns true if the grant scope contains the specified scope
func (grant *OAuth2Grant) ScopeContains(scope string) bool {
	for _, currentScope := range strings.Split(grant.Scope, " ") {
		if scope == currentScope {
			return true
		}
	}
	return false
}

// SetNonce updates the current nonce value of a grant
func (grant *OAuth2Grant) SetNonce(nonce string) error {
	return grant.setNonce(x, nonce)
}

func (grant *OAuth2Grant) setNonce(e Engine, nonce string) error {
	grant.Nonce = nonce
	_, err := e.ID(grant.ID).Cols("nonce").Update(grant)
	if err != nil {
		return err
	}
	return nil
}

// GetOAuth2GrantByID returns the grant with the given ID
func GetOAuth2GrantByID(id int64) (*OAuth2Grant, error) {
	return getOAuth2GrantByID(x, id)
}

func getOAuth2GrantByID(e Engine, id int64) (grant *OAuth2Grant, err error) {
	grant = new(OAuth2Grant)
	if has, err := e.ID(id).Get(grant); err != nil {
		return nil, err
	} else if !has {
		return nil, nil
	}
	return
}

// GetOAuth2GrantsByUserID lists all grants of a certain user
func GetOAuth2GrantsByUserID(uid int64) ([]*OAuth2Grant, error) {
	return getOAuth2GrantsByUserID(x, uid)
}

func getOAuth2GrantsByUserID(e Engine, uid int64) ([]*OAuth2Grant, error) {
	type joinedOAuth2Grant struct {
		Grant       *OAuth2Grant       `xorm:"extends"`
		Application *OAuth2Application `xorm:"extends"`
	}
	var results *xorm.Rows
	var err error
	if results, err = e.
		Table("oauth2_grant").
		Where("user_id = ?", uid).
		Join("INNER", "oauth2_application", "application_id = oauth2_application.id").
		Rows(new(joinedOAuth2Grant)); err != nil {
		return nil, err
	}
	defer results.Close()
	grants := make([]*OAuth2Grant, 0)
	for results.Next() {
		joinedGrant := new(joinedOAuth2Grant)
		if err := results.Scan(joinedGrant); err != nil {
			return nil, err
		}
		joinedGrant.Grant.Application = joinedGrant.Application
		grants = append(grants, joinedGrant.Grant)
	}
	return grants, nil
}

// RevokeOAuth2Grant deletes the grant with grantID and userID
func RevokeOAuth2Grant(grantID, userID int64) error {
	return revokeOAuth2Grant(x, grantID, userID)
}

func revokeOAuth2Grant(e Engine, grantID, userID int64) error {
	_, err := e.Delete(&OAuth2Grant{ID: grantID, UserID: userID})
	return err
}

//////////////////////////////////////////////////////////////

// OAuth2TokenType represents the type of token for an oauth application
type OAuth2TokenType int

const (
	// TypeAccessToken is a token with short lifetime to access the api
	TypeAccessToken OAuth2TokenType = 0
	// TypeRefreshToken is token with long lifetime to refresh access tokens obtained by the client
	TypeRefreshToken = iota
)

// OAuth2Token represents a JWT token used to authenticate a client
type OAuth2Token struct {
	GrantID int64           `json:"gnt"`
	Type    OAuth2TokenType `json:"tt"`
	Counter int64           `json:"cnt,omitempty"`
	jwt.StandardClaims
}

// ParseOAuth2Token parses a signed jwt string
func ParseOAuth2Token(jwtToken string) (*OAuth2Token, error) {
	parsedToken, err := jwt.ParseWithClaims(jwtToken, &OAuth2Token{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method == nil || token.Method.Alg() != oauth2.DefaultSigningKey.SigningMethod().Alg() {
			return nil, fmt.Errorf("unexpected signing algo: %v", token.Header["alg"])
		}
		return oauth2.DefaultSigningKey.VerifyKey(), nil
	})
	if err != nil {
		return nil, err
	}
	var token *OAuth2Token
	var ok bool
	if token, ok = parsedToken.Claims.(*OAuth2Token); !ok || !parsedToken.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return token, nil
}

// SignToken signs the token with the JWT secret
func (token *OAuth2Token) SignToken() (string, error) {
	token.IssuedAt = time.Now().Unix()
	jwtToken := jwt.NewWithClaims(oauth2.DefaultSigningKey.SigningMethod(), token)
	oauth2.DefaultSigningKey.PreProcessToken(jwtToken)
	return jwtToken.SignedString(oauth2.DefaultSigningKey.SignKey())
}

// OIDCToken represents an OpenID Connect id_token
type OIDCToken struct {
	jwt.StandardClaims
	Nonce string `json:"nonce,omitempty"`

	// Scope profile
	Name              string             `json:"name,omitempty"`
	PreferredUsername string             `json:"preferred_username,omitempty"`
	Profile           string             `json:"profile,omitempty"`
	Picture           string             `json:"picture,omitempty"`
	Website           string             `json:"website,omitempty"`
	Locale            string             `json:"locale,omitempty"`
	UpdatedAt         timeutil.TimeStamp `json:"updated_at,omitempty"`

	// Scope email
	Email         string `json:"email,omitempty"`
	EmailVerified bool   `json:"email_verified,omitempty"`
}

// SignToken signs an id_token with the (symmetric) client secret key
func (token *OIDCToken) SignToken(signingKey oauth2.JWTSigningKey) (string, error) {
	token.IssuedAt = time.Now().Unix()
	jwtToken := jwt.NewWithClaims(signingKey.SigningMethod(), token)
	signingKey.PreProcessToken(jwtToken)
	return jwtToken.SignedString(signingKey.SignKey())
}
