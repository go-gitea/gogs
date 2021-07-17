// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package auth

import (
	"errors"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/web/middleware"
	"code.gitea.io/gitea/services/auth/source/sspi"

	gouuid "github.com/google/uuid"
	"github.com/quasoft/websspi"
	"github.com/unrolled/render"
)

const (
	tplSignIn base.TplName = "user/auth/signin"
)

var (
	// sspiAuth is a global instance of the websspi authentication package,
	// which is used to avoid acquiring the server credential handle on
	// every request
	sspiAuth *websspi.Authenticator

	// Ensure the struct implements the interface.
	_ Method        = &SSPI{}
	_ Named         = &SSPI{}
	_ Initializable = &SSPI{}
	_ Freeable      = &SSPI{}
)

// SSPI implements the SingleSignOn interface and authenticates requests
// via the built-in SSPI module in Windows for SPNEGO authentication.
// On successful authentication returns a valid user object.
// Returns nil if authentication fails.
type SSPI struct {
	rnd *render.Render
}

// Init creates a new global websspi.Authenticator object
func (s *SSPI) Init() error {
	config := websspi.NewConfig()
	var err error
	sspiAuth, err = websspi.New(config)
	if err != nil {
		return err
	}
	s.rnd = render.New(render.Options{
		Extensions:    []string{".tmpl"},
		Directory:     "templates",
		Funcs:         templates.NewFuncMap(),
		Asset:         templates.GetAsset,
		AssetNames:    templates.GetAssetNames,
		IsDevelopment: !setting.IsProd(),
	})
	return nil
}

// Name represents the name of auth method
func (s *SSPI) Name() string {
	return "sspi"
}

// Free releases resources used by the global websspi.Authenticator object
func (s *SSPI) Free() error {
	return sspiAuth.Free()
}

// Verify uses SSPI (Windows implementation of SPNEGO) to authenticate the request.
// If authentication is successful, returns the corresponding user object.
// If negotiation should continue or authentication fails, immediately returns a 401 HTTP
// response code, as required by the SPNEGO protocol.
func (s *SSPI) Verify(req *http.Request, w http.ResponseWriter, store DataStore, sess SessionStore) *models.User {
	if !s.shouldAuthenticate(req) {
		return nil
	}

	cfg, err := s.getConfig()
	if err != nil {
		log.Error("could not get SSPI config: %v", err)
		return nil
	}

	log.Trace("SSPI Authorization: Attempting to authenticate")
	userInfo, outToken, err := sspiAuth.Authenticate(req, w)
	if err != nil {
		log.Warn("Authentication failed with error: %v\n", err)
		sspiAuth.AppendAuthenticateHeader(w, outToken)

		// Include the user login page in the 401 response to allow the user
		// to login with another authentication method if SSPI authentication
		// fails
		store.GetData()["Flash"] = map[string]string{
			"ErrorMsg": err.Error(),
		}
		store.GetData()["EnableOpenIDSignIn"] = setting.Service.EnableOpenIDSignIn
		store.GetData()["EnableSSPI"] = true

		err := s.rnd.HTML(w, 401, string(tplSignIn), templates.BaseVars().Merge(store.GetData()))
		if err != nil {
			log.Error("%v", err)
		}

		return nil
	}
	if outToken != "" {
		sspiAuth.AppendAuthenticateHeader(w, outToken)
	}

	username := sanitizeUsername(userInfo.Username, cfg)
	if len(username) == 0 {
		return nil
	}
	log.Info("Authenticated as %s\n", username)

	user, err := models.GetUserByName(username)
	if err != nil {
		if !models.IsErrUserNotExist(err) {
			log.Error("GetUserByName: %v", err)
			return nil
		}
		if !cfg.AutoCreateUsers {
			log.Error("User '%s' not found", username)
			return nil
		}
		user, err = s.newUser(username, cfg)
		if err != nil {
			log.Error("CreateUser: %v", err)
			return nil
		}
	}

	// Make sure requests to API paths and PWA resources do not create a new session
	if !middleware.IsAPIPath(req) && !isAttachmentDownload(req) {
		handleSignIn(w, req, sess, user)
	}

	log.Trace("SSPI Authorization: Logged in user %-v", user)
	return user
}

// getConfig retrieves the SSPI configuration from login sources
func (s *SSPI) getConfig() (*sspi.Source, error) {
	sources, err := models.ActiveLoginSources(models.LoginSSPI)
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return nil, errors.New("no active login sources of type SSPI found")
	}
	if len(sources) > 1 {
		return nil, errors.New("more than one active login source of type SSPI found")
	}
	return sources[0].Cfg.(*sspi.Source), nil
}

func (s *SSPI) shouldAuthenticate(req *http.Request) (shouldAuth bool) {
	shouldAuth = false
	path := strings.TrimSuffix(req.URL.Path, "/")
	if path == "/user/login" {
		if req.FormValue("user_name") != "" && req.FormValue("password") != "" {
			shouldAuth = false
		} else if req.FormValue("auth_with_sspi") == "1" {
			shouldAuth = true
		}
	} else if middleware.IsAPIPath(req) || isAttachmentDownload(req) {
		shouldAuth = true
	}
	return
}

// newUser creates a new user object for the purpose of automatic registration
// and populates its name and email with the information present in request headers.
func (s *SSPI) newUser(username string, cfg *sspi.Source) (*models.User, error) {
	email := gouuid.New().String() + "@localhost.localdomain"
	user := &models.User{
		Name:                         username,
		Email:                        email,
		KeepEmailPrivate:             true,
		Passwd:                       gouuid.New().String(),
		IsActive:                     cfg.AutoActivateUsers,
		Language:                     cfg.DefaultLanguage,
		UseCustomAvatar:              true,
		Avatar:                       models.DefaultAvatarLink(),
		EmailNotificationsPreference: models.EmailNotificationsDisabled,
	}
	if err := models.CreateUser(user); err != nil {
		return nil, err
	}
	return user, nil
}

// stripDomainNames removes NETBIOS domain name and separator from down-level logon names
// (eg. "DOMAIN\user" becomes "user"), and removes the UPN suffix (domain name) and separator
// from UPNs (eg. "user@domain.local" becomes "user")
func stripDomainNames(username string) string {
	if strings.Contains(username, "\\") {
		parts := strings.SplitN(username, "\\", 2)
		if len(parts) > 1 {
			username = parts[1]
		}
	} else if strings.Contains(username, "@") {
		parts := strings.Split(username, "@")
		if len(parts) > 1 {
			username = parts[0]
		}
	}
	return username
}

func replaceSeparators(username string, cfg *sspi.Source) string {
	newSep := cfg.SeparatorReplacement
	username = strings.ReplaceAll(username, "\\", newSep)
	username = strings.ReplaceAll(username, "/", newSep)
	username = strings.ReplaceAll(username, "@", newSep)
	return username
}

func sanitizeUsername(username string, cfg *sspi.Source) string {
	if len(username) == 0 {
		return ""
	}
	if cfg.StripDomainNames {
		username = stripDomainNames(username)
	}
	// Replace separators even if we have already stripped the domain name part,
	// as the username can contain several separators: eg. "MICROSOFT\useremail@live.com"
	username = replaceSeparators(username, cfg)
	return username
}

// specialInit registers the SSPI auth method as the last method in the list.
// The SSPI plugin is expected to be executed last, as it returns 401 status code if negotiation
// fails (or if negotiation should continue), which would prevent other authentication methods
// to execute at all.
func specialInit() {
	if models.IsSSPIEnabled() {
		Register(&SSPI{})
	}
}
