// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"regexp"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/structs"
)

// Service settings
var Service = struct {
	DefaultUserVisibility                   string
	DefaultUserVisibilityMode               structs.VisibleType
	AllowedUserVisibilityModes              []string
	AllowedUserVisibilityModesSlice         AllowedVisibility `ini:"-"`
	DefaultOrgVisibility                    string
	DefaultOrgVisibilityMode                structs.VisibleType
	ActiveCodeLives                         int
	ResetPwdCodeLives                       int
	RegisterEmailConfirm                    bool
	RegisterManualConfirm                   bool
	EmailDomainWhitelist                    []string
	EmailDomainBlocklist                    []string
	DisableRegistration                     bool
	AllowOnlyInternalRegistration           bool
	AllowOnlyExternalRegistration           bool
	ShowRegistrationButton                  bool
	ShowMilestonesDashboardPage             bool
	RequireSignInView                       bool
	EnableNotifyMail                        bool
	EnableBasicAuth                         bool
	EnableReverseProxyAuth                  bool
	EnableReverseProxyAutoRegister          bool
	EnableReverseProxyEmail                 bool
	EnableCaptcha                           bool
	RequireExternalRegistrationCaptcha      bool
	RequireExternalRegistrationPassword     bool
	CaptchaType                             string
	RecaptchaSecret                         string
	RecaptchaSitekey                        string
	RecaptchaURL                            string
	HcaptchaSecret                          string
	HcaptchaSitekey                         string
	DefaultKeepEmailPrivate                 bool
	DefaultAllowCreateOrganization          bool
	DefaultUserIsRestricted                 bool
	EnableTimetracking                      bool
	DefaultEnableTimetracking               bool
	DefaultEnableDependencies               bool
	AllowCrossRepositoryDependencies        bool
	DefaultAllowOnlyContributorsToTrackTime bool
	NoReplyAddress                          string
	EnableUserHeatmap                       bool
	AutoWatchNewRepos                       bool
	AutoWatchOnChanges                      bool
	DefaultOrgMemberVisible                 bool
	UserDeleteWithCommentsMaxTime           time.Duration
	ValidSiteURLSchemes                     []string

	// OpenID settings
	EnableOpenIDSignIn bool
	EnableOpenIDSignUp bool
	OpenIDWhitelist    []*regexp.Regexp
	OpenIDBlacklist    []*regexp.Regexp

	// Explore page settings
	Explore struct {
		RequireSigninView bool `ini:"REQUIRE_SIGNIN_VIEW"`
		DisableUsersPage  bool `ini:"DISABLE_USERS_PAGE"`
	} `ini:"service.explore"`
}{
	AllowedUserVisibilityModesSlice: []bool{true, true, true},
}

// AllowedVisibility store in a 3 item bool array what is allowed
type AllowedVisibility []bool

// IsAllowedVisibility check if a AllowedVisibility allow a specific VisibleType
func (a AllowedVisibility) IsAllowedVisibility(t structs.VisibleType) bool {
	if int(t) >= len(a) {
		return false
	}
	return a[t]
}

// ToVisibleTypeSlice convert a AllowedVisibility into a VisibleType slice
func (a AllowedVisibility) ToVisibleTypeSlice() (result []structs.VisibleType) {
	for i, v := range a {
		if v {
			result = append(result, structs.VisibleType(i))
		}
	}
	return
}

func newService() {
	sec := Cfg.Section("service")
	Service.ActiveCodeLives = sec.Key("ACTIVE_CODE_LIVE_MINUTES").MustInt(180)
	Service.ResetPwdCodeLives = sec.Key("RESET_PASSWD_CODE_LIVE_MINUTES").MustInt(180)
	Service.DisableRegistration = sec.Key("DISABLE_REGISTRATION").MustBool()
	Service.AllowOnlyInternalRegistration = sec.Key("ALLOW_ONLY_INTERNAL_REGISTRATION").MustBool()
	Service.AllowOnlyExternalRegistration = sec.Key("ALLOW_ONLY_EXTERNAL_REGISTRATION").MustBool()
	if Service.AllowOnlyExternalRegistration && Service.AllowOnlyInternalRegistration {
		log.Warn("ALLOW_ONLY_INTERNAL_REGISTRATION and ALLOW_ONLY_EXTERNAL_REGISTRATION are true - disabling registration")
		Service.DisableRegistration = true
	}
	if !sec.Key("REGISTER_EMAIL_CONFIRM").MustBool() {
		Service.RegisterManualConfirm = sec.Key("REGISTER_MANUAL_CONFIRM").MustBool(false)
	} else {
		Service.RegisterManualConfirm = false
	}
	Service.EmailDomainWhitelist = sec.Key("EMAIL_DOMAIN_WHITELIST").Strings(",")
	Service.EmailDomainBlocklist = sec.Key("EMAIL_DOMAIN_BLOCKLIST").Strings(",")
	Service.ShowRegistrationButton = sec.Key("SHOW_REGISTRATION_BUTTON").MustBool(!(Service.DisableRegistration || Service.AllowOnlyExternalRegistration))
	Service.ShowMilestonesDashboardPage = sec.Key("SHOW_MILESTONES_DASHBOARD_PAGE").MustBool(true)
	Service.RequireSignInView = sec.Key("REQUIRE_SIGNIN_VIEW").MustBool()
	Service.EnableBasicAuth = sec.Key("ENABLE_BASIC_AUTHENTICATION").MustBool(true)
	Service.EnableReverseProxyAuth = sec.Key("ENABLE_REVERSE_PROXY_AUTHENTICATION").MustBool()
	Service.EnableReverseProxyAutoRegister = sec.Key("ENABLE_REVERSE_PROXY_AUTO_REGISTRATION").MustBool()
	Service.EnableReverseProxyEmail = sec.Key("ENABLE_REVERSE_PROXY_EMAIL").MustBool()
	Service.EnableCaptcha = sec.Key("ENABLE_CAPTCHA").MustBool(false)
	Service.RequireExternalRegistrationCaptcha = sec.Key("REQUIRE_EXTERNAL_REGISTRATION_CAPTCHA").MustBool(Service.EnableCaptcha)
	Service.RequireExternalRegistrationPassword = sec.Key("REQUIRE_EXTERNAL_REGISTRATION_PASSWORD").MustBool()
	Service.CaptchaType = sec.Key("CAPTCHA_TYPE").MustString(ImageCaptcha)
	Service.RecaptchaSecret = sec.Key("RECAPTCHA_SECRET").MustString("")
	Service.RecaptchaSitekey = sec.Key("RECAPTCHA_SITEKEY").MustString("")
	Service.RecaptchaURL = sec.Key("RECAPTCHA_URL").MustString("https://www.google.com/recaptcha/")
	Service.HcaptchaSecret = sec.Key("HCAPTCHA_SECRET").MustString("")
	Service.HcaptchaSitekey = sec.Key("HCAPTCHA_SITEKEY").MustString("")
	Service.DefaultKeepEmailPrivate = sec.Key("DEFAULT_KEEP_EMAIL_PRIVATE").MustBool()
	Service.DefaultAllowCreateOrganization = sec.Key("DEFAULT_ALLOW_CREATE_ORGANIZATION").MustBool(true)
	Service.DefaultUserIsRestricted = sec.Key("DEFAULT_USER_IS_RESTRICTED").MustBool(false)
	Service.EnableTimetracking = sec.Key("ENABLE_TIMETRACKING").MustBool(true)
	if Service.EnableTimetracking {
		Service.DefaultEnableTimetracking = sec.Key("DEFAULT_ENABLE_TIMETRACKING").MustBool(true)
	}
	Service.DefaultEnableDependencies = sec.Key("DEFAULT_ENABLE_DEPENDENCIES").MustBool(true)
	Service.AllowCrossRepositoryDependencies = sec.Key("ALLOW_CROSS_REPOSITORY_DEPENDENCIES").MustBool(true)
	Service.DefaultAllowOnlyContributorsToTrackTime = sec.Key("DEFAULT_ALLOW_ONLY_CONTRIBUTORS_TO_TRACK_TIME").MustBool(true)
	Service.NoReplyAddress = sec.Key("NO_REPLY_ADDRESS").MustString("noreply." + Domain)
	Service.EnableUserHeatmap = sec.Key("ENABLE_USER_HEATMAP").MustBool(true)
	Service.AutoWatchNewRepos = sec.Key("AUTO_WATCH_NEW_REPOS").MustBool(true)
	Service.AutoWatchOnChanges = sec.Key("AUTO_WATCH_ON_CHANGES").MustBool(false)
	Service.DefaultUserVisibility = sec.Key("DEFAULT_USER_VISIBILITY").In("public", structs.ExtractKeysFromMapString(structs.VisibilityModes))
	Service.DefaultUserVisibilityMode = structs.VisibilityModes[Service.DefaultUserVisibility]
	Service.AllowedUserVisibilityModes = sec.Key("ALLOWED_USER_VISIBILITY_MODES").Strings(",")
	if len(Service.AllowedUserVisibilityModes) != 0 {
		Service.AllowedUserVisibilityModesSlice = []bool{false, false, false}
		for _, sMode := range Service.AllowedUserVisibilityModes {
			Service.AllowedUserVisibilityModesSlice[structs.VisibilityModes[sMode]] = true
		}
	}
	Service.DefaultOrgVisibility = sec.Key("DEFAULT_ORG_VISIBILITY").In("public", structs.ExtractKeysFromMapString(structs.VisibilityModes))
	Service.DefaultOrgVisibilityMode = structs.VisibilityModes[Service.DefaultOrgVisibility]
	Service.DefaultOrgMemberVisible = sec.Key("DEFAULT_ORG_MEMBER_VISIBLE").MustBool()
	Service.UserDeleteWithCommentsMaxTime = sec.Key("USER_DELETE_WITH_COMMENTS_MAX_TIME").MustDuration(0)
	sec.Key("VALID_SITE_URL_SCHEMES").MustString("http,https")
	Service.ValidSiteURLSchemes = sec.Key("VALID_SITE_URL_SCHEMES").Strings(",")
	schemes := make([]string, len(Service.ValidSiteURLSchemes))
	for _, scheme := range Service.ValidSiteURLSchemes {
		scheme = strings.ToLower(strings.TrimSpace(scheme))
		if scheme != "" {
			schemes = append(schemes, scheme)
		}
	}
	Service.ValidSiteURLSchemes = schemes

	if err := Cfg.Section("service.explore").MapTo(&Service.Explore); err != nil {
		log.Fatal("Failed to map service.explore settings: %v", err)
	}

	sec = Cfg.Section("openid")
	Service.EnableOpenIDSignIn = sec.Key("ENABLE_OPENID_SIGNIN").MustBool(!InstallLock)
	Service.EnableOpenIDSignUp = sec.Key("ENABLE_OPENID_SIGNUP").MustBool(!Service.DisableRegistration && Service.EnableOpenIDSignIn)
	pats := sec.Key("WHITELISTED_URIS").Strings(" ")
	if len(pats) != 0 {
		Service.OpenIDWhitelist = make([]*regexp.Regexp, len(pats))
		for i, p := range pats {
			Service.OpenIDWhitelist[i] = regexp.MustCompilePOSIX(p)
		}
	}
	pats = sec.Key("BLACKLISTED_URIS").Strings(" ")
	if len(pats) != 0 {
		Service.OpenIDBlacklist = make([]*regexp.Regexp, len(pats))
		for i, p := range pats {
			Service.OpenIDBlacklist[i] = regexp.MustCompilePOSIX(p)
		}
	}
}
