// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"text/template"
	"time"

	"code.gitea.io/gitea/modules/generate"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/user"
	"code.gitea.io/gitea/modules/util"

	jsoniter "github.com/json-iterator/go"
	shellquote "github.com/kballard/go-shellquote"
	"github.com/unknwon/com"
	gossh "golang.org/x/crypto/ssh"
	ini "gopkg.in/ini.v1"
)

// Scheme describes protocol types
type Scheme string

// enumerates all the scheme types
const (
	HTTP       Scheme = "http"
	HTTPS      Scheme = "https"
	FCGI       Scheme = "fcgi"
	FCGIUnix   Scheme = "fcgi+unix"
	UnixSocket Scheme = "unix"
)

// LandingPage describes the default page
type LandingPage string

// enumerates all the landing page types
const (
	LandingPageHome          LandingPage = "/"
	LandingPageExplore       LandingPage = "/explore"
	LandingPageOrganizations LandingPage = "/explore/organizations"
	LandingPageLogin         LandingPage = "/user/login"
)

// enumerates all the types of captchas
const (
	ImageCaptcha = "image"
	ReCaptcha    = "recaptcha"
	HCaptcha     = "hcaptcha"
)

// settings
var (
	// AppVer is the version of the current build of Gitea. It is set in main.go from main.Version.
	AppVer string
	// AppBuiltWith represents a human readable version go runtime build version and build tags. (See main.go formatBuiltWith().)
	AppBuiltWith string
	// AppStartTime store time gitea has started
	AppStartTime time.Time
	// AppName is the Application name, used in the page title.
	// It maps to ini:"APP_NAME"
	AppName string
	// AppURL is the Application ROOT_URL. It always has a '/' suffix
	// It maps to ini:"ROOT_URL"
	AppURL string
	// AppSubURL represents the sub-url mounting point for gitea. It is either "" or starts with '/' and ends without '/', such as '/{subpath}'.
	// This value is empty if site does not have sub-url.
	AppSubURL string
	// AppPath represents the path to the gitea binary
	AppPath string
	// AppWorkPath is the "working directory" of Gitea. It maps to the environment variable GITEA_WORK_DIR.
	// If that is not set it is the default set here by the linker or failing that the directory of AppPath.
	//
	// AppWorkPath is used as the base path for several other paths.
	AppWorkPath string
	// AppDataPath is the default path for storing data.
	// It maps to ini:"APP_DATA_PATH" and defaults to AppWorkPath + "/data"
	AppDataPath string

	// Server settings
	Protocol             Scheme
	Domain               string
	HTTPAddr             string
	HTTPPort             string
	LocalURL             string
	RedirectOtherPort    bool
	PortToRedirect       string
	OfflineMode          bool
	CertFile             string
	KeyFile              string
	StaticRootPath       string
	StaticCacheTime      time.Duration
	EnableGzip           bool
	LandingPageURL       LandingPage
	UnixSocketPermission uint32
	EnablePprof          bool
	PprofDataPath        string
	EnableLetsEncrypt    bool
	LetsEncryptTOS       bool
	LetsEncryptDirectory string
	LetsEncryptEmail     string
	GracefulRestartable  bool
	GracefulHammerTime   time.Duration
	StartupTimeout       time.Duration
	PerWriteTimeout      = 30 * time.Second
	PerWritePerKbTimeout = 10 * time.Second
	StaticURLPrefix      string
	AbsoluteAssetURL     string

	SSH = struct {
		Disabled                              bool               `ini:"DISABLE_SSH"`
		StartBuiltinServer                    bool               `ini:"START_SSH_SERVER"`
		BuiltinServerUser                     string             `ini:"BUILTIN_SSH_SERVER_USER"`
		Domain                                string             `ini:"SSH_DOMAIN"`
		Port                                  int                `ini:"SSH_PORT"`
		ListenHost                            string             `ini:"SSH_LISTEN_HOST"`
		ListenPort                            int                `ini:"SSH_LISTEN_PORT"`
		RootPath                              string             `ini:"SSH_ROOT_PATH"`
		ServerCiphers                         []string           `ini:"SSH_SERVER_CIPHERS"`
		ServerKeyExchanges                    []string           `ini:"SSH_SERVER_KEY_EXCHANGES"`
		ServerMACs                            []string           `ini:"SSH_SERVER_MACS"`
		ServerHostKeys                        []string           `ini:"SSH_SERVER_HOST_KEYS"`
		KeyTestPath                           string             `ini:"SSH_KEY_TEST_PATH"`
		KeygenPath                            string             `ini:"SSH_KEYGEN_PATH"`
		AuthorizedKeysBackup                  bool               `ini:"SSH_AUTHORIZED_KEYS_BACKUP"`
		AuthorizedPrincipalsBackup            bool               `ini:"SSH_AUTHORIZED_PRINCIPALS_BACKUP"`
		AuthorizedKeysCommandTemplate         string             `ini:"SSH_AUTHORIZED_KEYS_COMMAND_TEMPLATE"`
		AuthorizedKeysCommandTemplateTemplate *template.Template `ini:"-"`
		MinimumKeySizeCheck                   bool               `ini:"-"`
		MinimumKeySizes                       map[string]int     `ini:"-"`
		CreateAuthorizedKeysFile              bool               `ini:"SSH_CREATE_AUTHORIZED_KEYS_FILE"`
		CreateAuthorizedPrincipalsFile        bool               `ini:"SSH_CREATE_AUTHORIZED_PRINCIPALS_FILE"`
		ExposeAnonymous                       bool               `ini:"SSH_EXPOSE_ANONYMOUS"`
		AuthorizedPrincipalsAllow             []string           `ini:"SSH_AUTHORIZED_PRINCIPALS_ALLOW"`
		AuthorizedPrincipalsEnabled           bool               `ini:"-"`
		TrustedUserCAKeys                     []string           `ini:"SSH_TRUSTED_USER_CA_KEYS"`
		TrustedUserCAKeysFile                 string             `ini:"SSH_TRUSTED_USER_CA_KEYS_FILENAME"`
		TrustedUserCAKeysParsed               []gossh.PublicKey  `ini:"-"`
		PerWriteTimeout                       time.Duration      `ini:"SSH_PER_WRITE_TIMEOUT"`
		PerWritePerKbTimeout                  time.Duration      `ini:"SSH_PER_WRITE_PER_KB_TIMEOUT"`
	}{
		Disabled:                      false,
		StartBuiltinServer:            false,
		Domain:                        "",
		Port:                          22,
		ServerCiphers:                 []string{"aes128-ctr", "aes192-ctr", "aes256-ctr", "aes128-gcm@openssh.com", "arcfour256", "arcfour128"},
		ServerKeyExchanges:            []string{"diffie-hellman-group1-sha1", "diffie-hellman-group14-sha1", "ecdh-sha2-nistp256", "ecdh-sha2-nistp384", "ecdh-sha2-nistp521", "curve25519-sha256@libssh.org"},
		ServerMACs:                    []string{"hmac-sha2-256-etm@openssh.com", "hmac-sha2-256", "hmac-sha1", "hmac-sha1-96"},
		KeygenPath:                    "ssh-keygen",
		MinimumKeySizeCheck:           true,
		MinimumKeySizes:               map[string]int{"ed25519": 256, "ed25519-sk": 256, "ecdsa": 256, "ecdsa-sk": 256, "rsa": 2048},
		ServerHostKeys:                []string{"ssh/gitea.rsa", "ssh/gogs.rsa"},
		AuthorizedKeysCommandTemplate: "{{.AppPath}} --config={{.CustomConf}} serv key-{{.Key.ID}}",
		PerWriteTimeout:               PerWriteTimeout,
		PerWritePerKbTimeout:          PerWritePerKbTimeout,
	}

	// Security settings
	InstallLock                        bool
	SecretKey                          string
	LogInRememberDays                  int
	CookieUserName                     string
	CookieRememberName                 string
	ReverseProxyAuthUser               string
	ReverseProxyAuthEmail              string
	ReverseProxyLimit                  int
	ReverseProxyTrustedProxies         []string
	MinPasswordLength                  int
	ImportLocalPaths                   bool
	DisableGitHooks                    bool
	DisableWebhooks                    bool
	OnlyAllowPushIfGiteaEnvironmentSet bool
	PasswordComplexity                 []string
	PasswordHashAlgo                   string
	PasswordCheckPwn                   bool

	// UI settings
	UI = struct {
		ExplorePagingNum      int
		IssuePagingNum        int
		RepoSearchPagingNum   int
		MembersPagingNum      int
		FeedMaxCommitNum      int
		FeedPagingNum         int
		GraphMaxCommitNum     int
		CodeCommentLines      int
		ReactionMaxUserNum    int
		ThemeColorMetaTag     string
		MaxDisplayFileSize    int64
		ShowUserEmail         bool
		DefaultShowFullName   bool
		DefaultTheme          string
		Themes                []string
		Reactions             []string
		ReactionsMap          map[string]bool
		SearchRepoDescription bool
		UseServiceWorker      bool

		Notification struct {
			MinTimeout            time.Duration
			TimeoutStep           time.Duration
			MaxTimeout            time.Duration
			EventSourceUpdateTime time.Duration
		} `ini:"ui.notification"`

		SVG struct {
			Enabled bool `ini:"ENABLE_RENDER"`
		} `ini:"ui.svg"`

		CSV struct {
			MaxFileSize int64
		} `ini:"ui.csv"`

		Admin struct {
			UserPagingNum   int
			RepoPagingNum   int
			NoticePagingNum int
			OrgPagingNum    int
		} `ini:"ui.admin"`
		User struct {
			RepoPagingNum int
		} `ini:"ui.user"`
		Meta struct {
			Author      string
			Description string
			Keywords    string
		} `ini:"ui.meta"`
	}{
		ExplorePagingNum:    20,
		IssuePagingNum:      10,
		RepoSearchPagingNum: 10,
		MembersPagingNum:    20,
		FeedMaxCommitNum:    5,
		FeedPagingNum:       20,
		GraphMaxCommitNum:   100,
		CodeCommentLines:    4,
		ReactionMaxUserNum:  10,
		ThemeColorMetaTag:   `#6cc644`,
		MaxDisplayFileSize:  8388608,
		DefaultTheme:        `gitea`,
		Themes:              []string{`gitea`, `arc-green`},
		Reactions:           []string{`+1`, `-1`, `laugh`, `hooray`, `confused`, `heart`, `rocket`, `eyes`},
		Notification: struct {
			MinTimeout            time.Duration
			TimeoutStep           time.Duration
			MaxTimeout            time.Duration
			EventSourceUpdateTime time.Duration
		}{
			MinTimeout:            10 * time.Second,
			TimeoutStep:           10 * time.Second,
			MaxTimeout:            60 * time.Second,
			EventSourceUpdateTime: 10 * time.Second,
		},
		SVG: struct {
			Enabled bool `ini:"ENABLE_RENDER"`
		}{
			Enabled: true,
		},
		CSV: struct {
			MaxFileSize int64
		}{
			MaxFileSize: 524288,
		},
		Admin: struct {
			UserPagingNum   int
			RepoPagingNum   int
			NoticePagingNum int
			OrgPagingNum    int
		}{
			UserPagingNum:   50,
			RepoPagingNum:   50,
			NoticePagingNum: 25,
			OrgPagingNum:    50,
		},
		User: struct {
			RepoPagingNum int
		}{
			RepoPagingNum: 15,
		},
		Meta: struct {
			Author      string
			Description string
			Keywords    string
		}{
			Author:      "Gitea - Git with a cup of tea",
			Description: "Gitea (Git with a cup of tea) is a painless self-hosted Git service written in Go",
			Keywords:    "go,git,self-hosted,gitea",
		},
	}

	// Markdown settings
	Markdown = struct {
		EnableHardLineBreakInComments  bool
		EnableHardLineBreakInDocuments bool
		CustomURLSchemes               []string `ini:"CUSTOM_URL_SCHEMES"`
		FileExtensions                 []string
	}{
		EnableHardLineBreakInComments:  true,
		EnableHardLineBreakInDocuments: false,
		FileExtensions:                 strings.Split(".md,.markdown,.mdown,.mkd", ","),
	}

	// Admin settings
	Admin struct {
		DisableRegularOrgCreation bool
		DefaultEmailNotification  string
	}

	// Log settings
	LogLevel           log.Level
	StacktraceLogLevel string
	LogRootPath        string
	DisableRouterLog   bool
	RouterLogLevel     log.Level
	EnableAccessLog    bool
	EnableSSHLog       bool
	AccessLogTemplate  string
	EnableXORMLog      bool

	// Time settings
	TimeFormat string
	// UILocation is the location on the UI, so that we can display the time on UI.
	DefaultUILocation = time.Local

	CSRFCookieName     = "_csrf"
	CSRFCookieHTTPOnly = true

	ManifestData string

	// Mirror settings
	Mirror struct {
		DefaultInterval time.Duration
		MinInterval     time.Duration
	}

	// API settings
	API = struct {
		EnableSwagger          bool
		SwaggerURL             string
		MaxResponseItems       int
		DefaultPagingNum       int
		DefaultGitTreesPerPage int
		DefaultMaxBlobSize     int64
	}{
		EnableSwagger:          true,
		SwaggerURL:             "",
		MaxResponseItems:       50,
		DefaultPagingNum:       30,
		DefaultGitTreesPerPage: 1000,
		DefaultMaxBlobSize:     10485760,
	}

	OAuth2 = struct {
		Enable                     bool
		AccessTokenExpirationTime  int64
		RefreshTokenExpirationTime int64
		InvalidateRefreshTokens    bool
		JWTSigningAlgorithm        string `ini:"JWT_SIGNING_ALGORITHM"`
		JWTSecretBase64            string `ini:"JWT_SECRET"`
		JWTSigningPrivateKeyFile   string `ini:"JWT_SIGNING_PRIVATE_KEY_FILE"`
		MaxTokenLength             int
	}{
		Enable:                     true,
		AccessTokenExpirationTime:  3600,
		RefreshTokenExpirationTime: 730,
		InvalidateRefreshTokens:    false,
		JWTSigningAlgorithm:        "RS256",
		JWTSigningPrivateKeyFile:   "jwt/private.pem",
		MaxTokenLength:             math.MaxInt16,
	}

	U2F = struct {
		AppID         string
		TrustedFacets []string
	}{}

	// Metrics settings
	Metrics = struct {
		Enabled bool
		Token   string
	}{
		Enabled: false,
		Token:   "",
	}

	// I18n settings
	Langs []string
	Names []string

	// Highlight settings are loaded in modules/template/highlight.go

	// Other settings
	ShowFooterBranding         bool
	ShowFooterVersion          bool
	ShowFooterTemplateLoadTime bool

	// Global setting objects
	Cfg           *ini.File
	CustomPath    string // Custom directory path
	CustomConf    string
	PIDFile       = "/run/gitea.pid"
	WritePIDFile  bool
	RunMode       string
	RunUser       string
	IsWindows     bool
	HasRobotsTxt  bool
	InternalToken string // internal access token
)

// IsProd if it's a production mode
func IsProd() bool {
	return strings.EqualFold(RunMode, "prod")
}

func getAppPath() (string, error) {
	var appPath string
	var err error
	if IsWindows && filepath.IsAbs(os.Args[0]) {
		appPath = filepath.Clean(os.Args[0])
	} else {
		appPath, err = exec.LookPath(os.Args[0])
	}

	if err != nil {
		return "", err
	}
	appPath, err = filepath.Abs(appPath)
	if err != nil {
		return "", err
	}
	// Note: we don't use path.Dir here because it does not handle case
	//	which path starts with two "/" in Windows: "//psf/Home/..."
	return strings.ReplaceAll(appPath, "\\", "/"), err
}

func getWorkPath(appPath string) string {
	workPath := AppWorkPath

	if giteaWorkPath, ok := os.LookupEnv("GITEA_WORK_DIR"); ok {
		workPath = giteaWorkPath
	}
	if len(workPath) == 0 {
		i := strings.LastIndex(appPath, "/")
		if i == -1 {
			workPath = appPath
		} else {
			workPath = appPath[:i]
		}
	}
	return strings.ReplaceAll(workPath, "\\", "/")
}

func init() {
	IsWindows = runtime.GOOS == "windows"
	// We can rely on log.CanColorStdout being set properly because modules/log/console_windows.go comes before modules/setting/setting.go lexicographically
	log.NewLogger(0, "console", "console", fmt.Sprintf(`{"level": "trace", "colorize": %t, "stacktraceLevel": "none"}`, log.CanColorStdout))

	var err error
	if AppPath, err = getAppPath(); err != nil {
		log.Fatal("Failed to get app path: %v", err)
	}
	AppWorkPath = getWorkPath(AppPath)
}

func forcePathSeparator(path string) {
	if strings.Contains(path, "\\") {
		log.Fatal("Do not use '\\' or '\\\\' in paths, instead, please use '/' in all places")
	}
}

// IsRunUserMatchCurrentUser returns false if configured run user does not match
// actual user that runs the app. The first return value is the actual user name.
// This check is ignored under Windows since SSH remote login is not the main
// method to login on Windows.
func IsRunUserMatchCurrentUser(runUser string) (string, bool) {
	if IsWindows || SSH.StartBuiltinServer {
		return "", true
	}

	currentUser := user.CurrentUsername()
	return currentUser, runUser == currentUser
}

func createPIDFile(pidPath string) {
	currentPid := os.Getpid()
	if err := os.MkdirAll(filepath.Dir(pidPath), os.ModePerm); err != nil {
		log.Fatal("Failed to create PID folder: %v", err)
	}

	file, err := os.Create(pidPath)
	if err != nil {
		log.Fatal("Failed to create PID file: %v", err)
	}
	defer file.Close()
	if _, err := file.WriteString(strconv.FormatInt(int64(currentPid), 10)); err != nil {
		log.Fatal("Failed to write PID information: %v", err)
	}
}

// SetCustomPathAndConf will set CustomPath and CustomConf with reference to the
// GITEA_CUSTOM environment variable and with provided overrides before stepping
// back to the default
func SetCustomPathAndConf(providedCustom, providedConf, providedWorkPath string) {
	if len(providedWorkPath) != 0 {
		AppWorkPath = filepath.ToSlash(providedWorkPath)
	}
	if giteaCustom, ok := os.LookupEnv("GITEA_CUSTOM"); ok {
		CustomPath = giteaCustom
	}
	if len(providedCustom) != 0 {
		CustomPath = providedCustom
	}
	if len(CustomPath) == 0 {
		CustomPath = path.Join(AppWorkPath, "custom")
	} else if !filepath.IsAbs(CustomPath) {
		CustomPath = path.Join(AppWorkPath, CustomPath)
	}

	if len(providedConf) != 0 {
		CustomConf = providedConf
	}
	if len(CustomConf) == 0 {
		CustomConf = path.Join(CustomPath, "conf/app.ini")
	} else if !filepath.IsAbs(CustomConf) {
		CustomConf = path.Join(CustomPath, CustomConf)
		log.Warn("Using 'custom' directory as relative origin for configuration file: '%s'", CustomConf)
	}
}

// NewContext initializes configuration context.
// NOTE: do not print any log except error.
func NewContext() {
	Cfg = ini.Empty()

	if WritePIDFile && len(PIDFile) > 0 {
		createPIDFile(PIDFile)
	}

	isFile, err := util.IsFile(CustomConf)
	if err != nil {
		log.Error("Unable to check if %s is a file. Error: %v", CustomConf, err)
	}
	if isFile {
		if err := Cfg.Append(CustomConf); err != nil {
			log.Fatal("Failed to load custom conf '%s': %v", CustomConf, err)
		}
	} else {
		log.Warn("Custom config '%s' not found, ignore this if you're running first time", CustomConf)
	}
	Cfg.NameMapper = ini.SnackCase

	homeDir, err := com.HomeDir()
	if err != nil {
		log.Fatal("Failed to get home directory: %v", err)
	}
	homeDir = strings.ReplaceAll(homeDir, "\\", "/")

	LogLevel = getLogLevel(Cfg.Section("log"), "LEVEL", log.INFO)
	StacktraceLogLevel = getStacktraceLogLevel(Cfg.Section("log"), "STACKTRACE_LEVEL", "None")
	LogRootPath = Cfg.Section("log").Key("ROOT_PATH").MustString(path.Join(AppWorkPath, "log"))
	forcePathSeparator(LogRootPath)
	RouterLogLevel = log.FromString(Cfg.Section("log").Key("ROUTER_LOG_LEVEL").MustString("Info"))

	sec := Cfg.Section("server")
	AppName = Cfg.Section("").Key("APP_NAME").MustString("Gitea: Git with a cup of tea")

	Protocol = HTTP
	switch sec.Key("PROTOCOL").String() {
	case "https":
		Protocol = HTTPS
		CertFile = sec.Key("CERT_FILE").String()
		KeyFile = sec.Key("KEY_FILE").String()
		if !filepath.IsAbs(CertFile) && len(CertFile) > 0 {
			CertFile = filepath.Join(CustomPath, CertFile)
		}
		if !filepath.IsAbs(KeyFile) && len(KeyFile) > 0 {
			KeyFile = filepath.Join(CustomPath, KeyFile)
		}
	case "fcgi":
		Protocol = FCGI
	case "fcgi+unix":
		Protocol = FCGIUnix
		UnixSocketPermissionRaw := sec.Key("UNIX_SOCKET_PERMISSION").MustString("666")
		UnixSocketPermissionParsed, err := strconv.ParseUint(UnixSocketPermissionRaw, 8, 32)
		if err != nil || UnixSocketPermissionParsed > 0777 {
			log.Fatal("Failed to parse unixSocketPermission: %s", UnixSocketPermissionRaw)
		}
		UnixSocketPermission = uint32(UnixSocketPermissionParsed)
	case "unix":
		Protocol = UnixSocket
		UnixSocketPermissionRaw := sec.Key("UNIX_SOCKET_PERMISSION").MustString("666")
		UnixSocketPermissionParsed, err := strconv.ParseUint(UnixSocketPermissionRaw, 8, 32)
		if err != nil || UnixSocketPermissionParsed > 0777 {
			log.Fatal("Failed to parse unixSocketPermission: %s", UnixSocketPermissionRaw)
		}
		UnixSocketPermission = uint32(UnixSocketPermissionParsed)
	}
	EnableLetsEncrypt = sec.Key("ENABLE_LETSENCRYPT").MustBool(false)
	LetsEncryptTOS = sec.Key("LETSENCRYPT_ACCEPTTOS").MustBool(false)
	if !LetsEncryptTOS && EnableLetsEncrypt {
		log.Warn("Failed to enable Let's Encrypt due to Let's Encrypt TOS not being accepted")
		EnableLetsEncrypt = false
	}
	LetsEncryptDirectory = sec.Key("LETSENCRYPT_DIRECTORY").MustString("https")
	LetsEncryptEmail = sec.Key("LETSENCRYPT_EMAIL").MustString("")
	Domain = sec.Key("DOMAIN").MustString("localhost")
	HTTPAddr = sec.Key("HTTP_ADDR").MustString("0.0.0.0")
	HTTPPort = sec.Key("HTTP_PORT").MustString("3000")
	GracefulRestartable = sec.Key("ALLOW_GRACEFUL_RESTARTS").MustBool(true)
	GracefulHammerTime = sec.Key("GRACEFUL_HAMMER_TIME").MustDuration(60 * time.Second)
	StartupTimeout = sec.Key("STARTUP_TIMEOUT").MustDuration(0 * time.Second)
	PerWriteTimeout = sec.Key("PER_WRITE_TIMEOUT").MustDuration(PerWriteTimeout)
	PerWritePerKbTimeout = sec.Key("PER_WRITE_PER_KB_TIMEOUT").MustDuration(PerWritePerKbTimeout)

	defaultAppURL := string(Protocol) + "://" + Domain
	if (Protocol == HTTP && HTTPPort != "80") || (Protocol == HTTPS && HTTPPort != "443") {
		defaultAppURL += ":" + HTTPPort
	}
	AppURL = sec.Key("ROOT_URL").MustString(defaultAppURL + "/")
	// This should be TrimRight to ensure that there is only a single '/' at the end of AppURL.
	AppURL = strings.TrimRight(AppURL, "/") + "/"

	// Check if has app suburl.
	appURL, err := url.Parse(AppURL)
	if err != nil {
		log.Fatal("Invalid ROOT_URL '%s': %s", AppURL, err)
	}
	// Suburl should start with '/' and end without '/', such as '/{subpath}'.
	// This value is empty if site does not have sub-url.
	AppSubURL = strings.TrimSuffix(appURL.Path, "/")
	StaticURLPrefix = strings.TrimSuffix(sec.Key("STATIC_URL_PREFIX").MustString(AppSubURL), "/")

	// Check if Domain differs from AppURL domain than update it to AppURL's domain
	urlHostname := appURL.Hostname()
	if urlHostname != Domain && net.ParseIP(urlHostname) == nil && urlHostname != "" {
		Domain = urlHostname
	}

	AbsoluteAssetURL = MakeAbsoluteAssetURL(AppURL, StaticURLPrefix)

	manifestBytes := MakeManifestData(AppName, AppURL, AbsoluteAssetURL)
	ManifestData = `application/json;base64,` + base64.StdEncoding.EncodeToString(manifestBytes)

	var defaultLocalURL string
	switch Protocol {
	case UnixSocket:
		defaultLocalURL = "http://unix/"
	case FCGI:
		defaultLocalURL = AppURL
	case FCGIUnix:
		defaultLocalURL = AppURL
	default:
		defaultLocalURL = string(Protocol) + "://"
		if HTTPAddr == "0.0.0.0" {
			defaultLocalURL += net.JoinHostPort("localhost", HTTPPort) + "/"
		} else {
			defaultLocalURL += net.JoinHostPort(HTTPAddr, HTTPPort) + "/"
		}
	}
	LocalURL = sec.Key("LOCAL_ROOT_URL").MustString(defaultLocalURL)
	RedirectOtherPort = sec.Key("REDIRECT_OTHER_PORT").MustBool(false)
	PortToRedirect = sec.Key("PORT_TO_REDIRECT").MustString("80")
	OfflineMode = sec.Key("OFFLINE_MODE").MustBool()
	DisableRouterLog = sec.Key("DISABLE_ROUTER_LOG").MustBool()
	if len(StaticRootPath) == 0 {
		StaticRootPath = AppWorkPath
	}
	StaticRootPath = sec.Key("STATIC_ROOT_PATH").MustString(StaticRootPath)
	StaticCacheTime = sec.Key("STATIC_CACHE_TIME").MustDuration(6 * time.Hour)
	AppDataPath = sec.Key("APP_DATA_PATH").MustString(path.Join(AppWorkPath, "data"))
	EnableGzip = sec.Key("ENABLE_GZIP").MustBool()
	EnablePprof = sec.Key("ENABLE_PPROF").MustBool(false)
	PprofDataPath = sec.Key("PPROF_DATA_PATH").MustString(path.Join(AppWorkPath, "data/tmp/pprof"))
	if !filepath.IsAbs(PprofDataPath) {
		PprofDataPath = filepath.Join(AppWorkPath, PprofDataPath)
	}

	switch sec.Key("LANDING_PAGE").MustString("home") {
	case "explore":
		LandingPageURL = LandingPageExplore
	case "organizations":
		LandingPageURL = LandingPageOrganizations
	case "login":
		LandingPageURL = LandingPageLogin
	default:
		LandingPageURL = LandingPageHome
	}

	if len(SSH.Domain) == 0 {
		SSH.Domain = Domain
	}
	SSH.RootPath = path.Join(homeDir, ".ssh")
	serverCiphers := sec.Key("SSH_SERVER_CIPHERS").Strings(",")
	if len(serverCiphers) > 0 {
		SSH.ServerCiphers = serverCiphers
	}
	serverKeyExchanges := sec.Key("SSH_SERVER_KEY_EXCHANGES").Strings(",")
	if len(serverKeyExchanges) > 0 {
		SSH.ServerKeyExchanges = serverKeyExchanges
	}
	serverMACs := sec.Key("SSH_SERVER_MACS").Strings(",")
	if len(serverMACs) > 0 {
		SSH.ServerMACs = serverMACs
	}
	SSH.KeyTestPath = os.TempDir()
	if err = Cfg.Section("server").MapTo(&SSH); err != nil {
		log.Fatal("Failed to map SSH settings: %v", err)
	}
	for i, key := range SSH.ServerHostKeys {
		if !filepath.IsAbs(key) {
			SSH.ServerHostKeys[i] = filepath.Join(AppDataPath, key)
		}
	}

	SSH.KeygenPath = sec.Key("SSH_KEYGEN_PATH").MustString("ssh-keygen")
	SSH.Port = sec.Key("SSH_PORT").MustInt(22)
	SSH.ListenPort = sec.Key("SSH_LISTEN_PORT").MustInt(SSH.Port)

	// When disable SSH, start builtin server value is ignored.
	if SSH.Disabled {
		SSH.StartBuiltinServer = false
	}

	trustedUserCaKeys := sec.Key("SSH_TRUSTED_USER_CA_KEYS").Strings(",")
	for _, caKey := range trustedUserCaKeys {
		pubKey, _, _, _, err := gossh.ParseAuthorizedKey([]byte(caKey))
		if err != nil {
			log.Fatal("Failed to parse TrustedUserCaKeys: %s %v", caKey, err)
		}

		SSH.TrustedUserCAKeysParsed = append(SSH.TrustedUserCAKeysParsed, pubKey)
	}
	if len(trustedUserCaKeys) > 0 {
		// Set the default as email,username otherwise we can leave it empty
		sec.Key("SSH_AUTHORIZED_PRINCIPALS_ALLOW").MustString("username,email")
	} else {
		sec.Key("SSH_AUTHORIZED_PRINCIPALS_ALLOW").MustString("off")
	}

	SSH.AuthorizedPrincipalsAllow, SSH.AuthorizedPrincipalsEnabled = parseAuthorizedPrincipalsAllow(sec.Key("SSH_AUTHORIZED_PRINCIPALS_ALLOW").Strings(","))

	if !SSH.Disabled && !SSH.StartBuiltinServer {
		if err := os.MkdirAll(SSH.RootPath, 0700); err != nil {
			log.Fatal("Failed to create '%s': %v", SSH.RootPath, err)
		} else if err = os.MkdirAll(SSH.KeyTestPath, 0644); err != nil {
			log.Fatal("Failed to create '%s': %v", SSH.KeyTestPath, err)
		}

		if len(trustedUserCaKeys) > 0 && SSH.AuthorizedPrincipalsEnabled {
			fname := sec.Key("SSH_TRUSTED_USER_CA_KEYS_FILENAME").MustString(filepath.Join(SSH.RootPath, "gitea-trusted-user-ca-keys.pem"))
			if err := ioutil.WriteFile(fname,
				[]byte(strings.Join(trustedUserCaKeys, "\n")), 0600); err != nil {
				log.Fatal("Failed to create '%s': %v", fname, err)
			}
		}
	}

	SSH.MinimumKeySizeCheck = sec.Key("MINIMUM_KEY_SIZE_CHECK").MustBool(SSH.MinimumKeySizeCheck)
	minimumKeySizes := Cfg.Section("ssh.minimum_key_sizes").Keys()
	for _, key := range minimumKeySizes {
		if key.MustInt() != -1 {
			SSH.MinimumKeySizes[strings.ToLower(key.Name())] = key.MustInt()
		} else {
			delete(SSH.MinimumKeySizes, strings.ToLower(key.Name()))
		}
	}

	SSH.AuthorizedKeysBackup = sec.Key("SSH_AUTHORIZED_KEYS_BACKUP").MustBool(true)
	SSH.CreateAuthorizedKeysFile = sec.Key("SSH_CREATE_AUTHORIZED_KEYS_FILE").MustBool(true)

	SSH.AuthorizedPrincipalsBackup = false
	SSH.CreateAuthorizedPrincipalsFile = false
	if SSH.AuthorizedPrincipalsEnabled {
		SSH.AuthorizedPrincipalsBackup = sec.Key("SSH_AUTHORIZED_PRINCIPALS_BACKUP").MustBool(true)
		SSH.CreateAuthorizedPrincipalsFile = sec.Key("SSH_CREATE_AUTHORIZED_PRINCIPALS_FILE").MustBool(true)
	}

	SSH.ExposeAnonymous = sec.Key("SSH_EXPOSE_ANONYMOUS").MustBool(false)
	SSH.AuthorizedKeysCommandTemplate = sec.Key("SSH_AUTHORIZED_KEYS_COMMAND_TEMPLATE").MustString(SSH.AuthorizedKeysCommandTemplate)

	SSH.AuthorizedKeysCommandTemplateTemplate = template.Must(template.New("").Parse(SSH.AuthorizedKeysCommandTemplate))

	SSH.PerWriteTimeout = sec.Key("SSH_PER_WRITE_TIMEOUT").MustDuration(PerWriteTimeout)
	SSH.PerWritePerKbTimeout = sec.Key("SSH_PER_WRITE_PER_KB_TIMEOUT").MustDuration(PerWritePerKbTimeout)

	if err = Cfg.Section("oauth2").MapTo(&OAuth2); err != nil {
		log.Fatal("Failed to OAuth2 settings: %v", err)
		return
	}

	if !filepath.IsAbs(OAuth2.JWTSigningPrivateKeyFile) {
		OAuth2.JWTSigningPrivateKeyFile = filepath.Join(CustomPath, OAuth2.JWTSigningPrivateKeyFile)
	}

	sec = Cfg.Section("admin")
	Admin.DefaultEmailNotification = sec.Key("DEFAULT_EMAIL_NOTIFICATIONS").MustString("enabled")

	sec = Cfg.Section("security")
	InstallLock = sec.Key("INSTALL_LOCK").MustBool(false)
	SecretKey = sec.Key("SECRET_KEY").MustString("!#@FDEWREWR&*(")
	LogInRememberDays = sec.Key("LOGIN_REMEMBER_DAYS").MustInt(7)
	CookieUserName = sec.Key("COOKIE_USERNAME").MustString("gitea_awesome")
	CookieRememberName = sec.Key("COOKIE_REMEMBER_NAME").MustString("gitea_incredible")

	ReverseProxyAuthUser = sec.Key("REVERSE_PROXY_AUTHENTICATION_USER").MustString("X-WEBAUTH-USER")
	ReverseProxyAuthEmail = sec.Key("REVERSE_PROXY_AUTHENTICATION_EMAIL").MustString("X-WEBAUTH-EMAIL")

	ReverseProxyLimit = sec.Key("REVERSE_PROXY_LIMIT").MustInt(1)
	ReverseProxyTrustedProxies = sec.Key("REVERSE_PROXY_TRUSTED_PROXIES").Strings(",")
	if len(ReverseProxyTrustedProxies) == 0 {
		ReverseProxyTrustedProxies = []string{"127.0.0.0/8", "::1/128"}
	}

	MinPasswordLength = sec.Key("MIN_PASSWORD_LENGTH").MustInt(6)
	ImportLocalPaths = sec.Key("IMPORT_LOCAL_PATHS").MustBool(false)
	DisableGitHooks = sec.Key("DISABLE_GIT_HOOKS").MustBool(true)
	DisableWebhooks = sec.Key("DISABLE_WEBHOOKS").MustBool(false)
	OnlyAllowPushIfGiteaEnvironmentSet = sec.Key("ONLY_ALLOW_PUSH_IF_GITEA_ENVIRONMENT_SET").MustBool(true)
	PasswordHashAlgo = sec.Key("PASSWORD_HASH_ALGO").MustString("pbkdf2")
	CSRFCookieHTTPOnly = sec.Key("CSRF_COOKIE_HTTP_ONLY").MustBool(true)
	PasswordCheckPwn = sec.Key("PASSWORD_CHECK_PWN").MustBool(false)

	InternalToken = loadInternalToken(sec)

	cfgdata := sec.Key("PASSWORD_COMPLEXITY").Strings(",")
	if len(cfgdata) == 0 {
		cfgdata = []string{"off"}
	}
	PasswordComplexity = make([]string, 0, len(cfgdata))
	for _, name := range cfgdata {
		name := strings.ToLower(strings.Trim(name, `"`))
		if name != "" {
			PasswordComplexity = append(PasswordComplexity, name)
		}
	}

	newAttachmentService()
	newLFSService()

	timeFormatKey := Cfg.Section("time").Key("FORMAT").MustString("")
	if timeFormatKey != "" {
		TimeFormat = map[string]string{
			"ANSIC":       time.ANSIC,
			"UnixDate":    time.UnixDate,
			"RubyDate":    time.RubyDate,
			"RFC822":      time.RFC822,
			"RFC822Z":     time.RFC822Z,
			"RFC850":      time.RFC850,
			"RFC1123":     time.RFC1123,
			"RFC1123Z":    time.RFC1123Z,
			"RFC3339":     time.RFC3339,
			"RFC3339Nano": time.RFC3339Nano,
			"Kitchen":     time.Kitchen,
			"Stamp":       time.Stamp,
			"StampMilli":  time.StampMilli,
			"StampMicro":  time.StampMicro,
			"StampNano":   time.StampNano,
		}[timeFormatKey]
		// When the TimeFormatKey does not exist in the previous map e.g.'2006-01-02 15:04:05'
		if len(TimeFormat) == 0 {
			TimeFormat = timeFormatKey
			TestTimeFormat, _ := time.Parse(TimeFormat, TimeFormat)
			if TestTimeFormat.Format(time.RFC3339) != "2006-01-02T15:04:05Z" {
				log.Warn("Provided TimeFormat: %s does not create a fully specified date and time.", TimeFormat)
				log.Warn("In order to display dates and times correctly please check your time format has 2006, 01, 02, 15, 04 and 05")
			}
			log.Trace("Custom TimeFormat: %s", TimeFormat)
		}
	}

	zone := Cfg.Section("time").Key("DEFAULT_UI_LOCATION").String()
	if zone != "" {
		DefaultUILocation, err = time.LoadLocation(zone)
		if err != nil {
			log.Fatal("Load time zone failed: %v", err)
		} else {
			log.Info("Default UI Location is %v", zone)
		}
	}
	if DefaultUILocation == nil {
		DefaultUILocation = time.Local
	}

	RunUser = Cfg.Section("").Key("RUN_USER").MustString(user.CurrentUsername())
	RunMode = Cfg.Section("").Key("RUN_MODE").MustString("prod")
	// Does not check run user when the install lock is off.
	if InstallLock {
		currentUser, match := IsRunUserMatchCurrentUser(RunUser)
		if !match {
			log.Fatal("Expect user '%s' but current user is: %s", RunUser, currentUser)
		}
	}

	SSH.BuiltinServerUser = Cfg.Section("server").Key("BUILTIN_SSH_SERVER_USER").MustString(RunUser)

	newRepository()

	newPictureService()

	if err = Cfg.Section("ui").MapTo(&UI); err != nil {
		log.Fatal("Failed to map UI settings: %v", err)
	} else if err = Cfg.Section("markdown").MapTo(&Markdown); err != nil {
		log.Fatal("Failed to map Markdown settings: %v", err)
	} else if err = Cfg.Section("admin").MapTo(&Admin); err != nil {
		log.Fatal("Fail to map Admin settings: %v", err)
	} else if err = Cfg.Section("api").MapTo(&API); err != nil {
		log.Fatal("Failed to map API settings: %v", err)
	} else if err = Cfg.Section("metrics").MapTo(&Metrics); err != nil {
		log.Fatal("Failed to map Metrics settings: %v", err)
	}

	u := *appURL
	u.Path = path.Join(u.Path, "api", "swagger")
	API.SwaggerURL = u.String()

	newGit()

	sec = Cfg.Section("mirror")
	Mirror.MinInterval = sec.Key("MIN_INTERVAL").MustDuration(10 * time.Minute)
	Mirror.DefaultInterval = sec.Key("DEFAULT_INTERVAL").MustDuration(8 * time.Hour)
	if Mirror.MinInterval.Minutes() < 1 {
		log.Warn("Mirror.MinInterval is too low")
		Mirror.MinInterval = 1 * time.Minute
	}
	if Mirror.DefaultInterval < Mirror.MinInterval {
		log.Warn("Mirror.DefaultInterval is less than Mirror.MinInterval")
		Mirror.DefaultInterval = time.Hour * 8
	}

	Langs = Cfg.Section("i18n").Key("LANGS").Strings(",")
	if len(Langs) == 0 {
		Langs = []string{
			"en-US", "zh-CN", "zh-HK", "zh-TW", "de-DE", "fr-FR", "nl-NL", "lv-LV",
			"ru-RU", "uk-UA", "ja-JP", "es-ES", "pt-BR", "pt-PT", "pl-PL", "bg-BG",
			"it-IT", "fi-FI", "tr-TR", "cs-CZ", "sr-SP", "sv-SE", "ko-KR"}
	}
	Names = Cfg.Section("i18n").Key("NAMES").Strings(",")
	if len(Names) == 0 {
		Names = []string{"English", "简体中文", "繁體中文（香港）", "繁體中文（台灣）", "Deutsch",
			"français", "Nederlands", "latviešu", "русский", "Українська", "日本語",
			"español", "português do Brasil", "Português de Portugal", "polski", "български",
			"italiano", "suomi", "Türkçe", "čeština", "српски", "svenska", "한국어"}
	}

	ShowFooterBranding = Cfg.Section("other").Key("SHOW_FOOTER_BRANDING").MustBool(false)
	ShowFooterVersion = Cfg.Section("other").Key("SHOW_FOOTER_VERSION").MustBool(true)
	ShowFooterTemplateLoadTime = Cfg.Section("other").Key("SHOW_FOOTER_TEMPLATE_LOAD_TIME").MustBool(true)

	UI.ShowUserEmail = Cfg.Section("ui").Key("SHOW_USER_EMAIL").MustBool(true)
	UI.DefaultShowFullName = Cfg.Section("ui").Key("DEFAULT_SHOW_FULL_NAME").MustBool(false)
	UI.SearchRepoDescription = Cfg.Section("ui").Key("SEARCH_REPO_DESCRIPTION").MustBool(true)
	UI.UseServiceWorker = Cfg.Section("ui").Key("USE_SERVICE_WORKER").MustBool(true)

	HasRobotsTxt, err = util.IsFile(path.Join(CustomPath, "robots.txt"))
	if err != nil {
		log.Error("Unable to check if %s is a file. Error: %v", path.Join(CustomPath, "robots.txt"), err)
	}

	newMarkup()

	sec = Cfg.Section("U2F")
	U2F.TrustedFacets, _ = shellquote.Split(sec.Key("TRUSTED_FACETS").MustString(strings.TrimSuffix(AppURL, AppSubURL+"/")))
	U2F.AppID = sec.Key("APP_ID").MustString(strings.TrimSuffix(AppURL, "/"))

	UI.ReactionsMap = make(map[string]bool)
	for _, reaction := range UI.Reactions {
		UI.ReactionsMap[reaction] = true
	}
}

func parseAuthorizedPrincipalsAllow(values []string) ([]string, bool) {
	anything := false
	email := false
	username := false
	for _, value := range values {
		v := strings.ToLower(strings.TrimSpace(value))
		switch v {
		case "off":
			return []string{"off"}, false
		case "email":
			email = true
		case "username":
			username = true
		case "anything":
			anything = true
		}
	}
	if anything {
		return []string{"anything"}, true
	}

	authorizedPrincipalsAllow := []string{}
	if username {
		authorizedPrincipalsAllow = append(authorizedPrincipalsAllow, "username")
	}
	if email {
		authorizedPrincipalsAllow = append(authorizedPrincipalsAllow, "email")
	}

	return authorizedPrincipalsAllow, true
}

func loadInternalToken(sec *ini.Section) string {
	uri := sec.Key("INTERNAL_TOKEN_URI").String()
	if len(uri) == 0 {
		return loadOrGenerateInternalToken(sec)
	}
	tempURI, err := url.Parse(uri)
	if err != nil {
		log.Fatal("Failed to parse INTERNAL_TOKEN_URI (%s): %v", uri, err)
	}
	switch tempURI.Scheme {
	case "file":
		fp, err := os.OpenFile(tempURI.RequestURI(), os.O_RDWR, 0600)
		if err != nil {
			log.Fatal("Failed to open InternalTokenURI (%s): %v", uri, err)
		}
		defer fp.Close()

		buf, err := ioutil.ReadAll(fp)
		if err != nil {
			log.Fatal("Failed to read InternalTokenURI (%s): %v", uri, err)
		}
		// No token in the file, generate one and store it.
		if len(buf) == 0 {
			token, err := generate.NewInternalToken()
			if err != nil {
				log.Fatal("Error generate internal token: %v", err)
			}
			if _, err := io.WriteString(fp, token); err != nil {
				log.Fatal("Error writing to InternalTokenURI (%s): %v", uri, err)
			}
			return token
		}

		return strings.TrimSpace(string(buf))
	default:
		log.Fatal("Unsupported URI-Scheme %q (INTERNAL_TOKEN_URI = %q)", tempURI.Scheme, uri)
	}
	return ""
}

func loadOrGenerateInternalToken(sec *ini.Section) string {
	var err error
	token := sec.Key("INTERNAL_TOKEN").String()
	if len(token) == 0 {
		token, err = generate.NewInternalToken()
		if err != nil {
			log.Fatal("Error generate internal token: %v", err)
		}

		// Save secret
		CreateOrAppendToCustomConf(func(cfg *ini.File) {
			cfg.Section("security").Key("INTERNAL_TOKEN").SetValue(token)
		})
	}
	return token
}

// MakeAbsoluteAssetURL returns the absolute asset url prefix without a trailing slash
func MakeAbsoluteAssetURL(appURL string, staticURLPrefix string) string {
	parsedPrefix, err := url.Parse(strings.TrimSuffix(staticURLPrefix, "/"))
	if err != nil {
		log.Fatal("Unable to parse STATIC_URL_PREFIX: %v", err)
	}

	if err == nil && parsedPrefix.Hostname() == "" {
		if staticURLPrefix == "" {
			return strings.TrimSuffix(appURL, "/")
		}

		// StaticURLPrefix is just a path
		return util.URLJoin(appURL, strings.TrimSuffix(staticURLPrefix, "/"))
	}

	return strings.TrimSuffix(staticURLPrefix, "/")
}

// MakeManifestData generates web app manifest JSON
func MakeManifestData(appName string, appURL string, absoluteAssetURL string) []byte {
	type manifestIcon struct {
		Src   string `json:"src"`
		Type  string `json:"type"`
		Sizes string `json:"sizes"`
	}

	type manifestJSON struct {
		Name      string         `json:"name"`
		ShortName string         `json:"short_name"`
		StartURL  string         `json:"start_url"`
		Icons     []manifestIcon `json:"icons"`
	}

	json := jsoniter.ConfigCompatibleWithStandardLibrary
	bytes, err := json.Marshal(&manifestJSON{
		Name:      appName,
		ShortName: appName,
		StartURL:  appURL,
		Icons: []manifestIcon{
			{
				Src:   absoluteAssetURL + "/assets/img/logo.png",
				Type:  "image/png",
				Sizes: "512x512",
			},
			{
				Src:   absoluteAssetURL + "/assets/img/logo.svg",
				Type:  "image/svg+xml",
				Sizes: "512x512",
			},
		},
	})

	if err != nil {
		log.Error("unable to marshal manifest JSON. Error: %v", err)
		return make([]byte, 0)
	}

	return bytes
}

// CreateOrAppendToCustomConf creates or updates the custom config.
// Use the callback to set individual values.
func CreateOrAppendToCustomConf(callback func(cfg *ini.File)) {
	cfg := ini.Empty()
	isFile, err := util.IsFile(CustomConf)
	if err != nil {
		log.Error("Unable to check if %s is a file. Error: %v", CustomConf, err)
	}
	if isFile {
		if err := cfg.Append(CustomConf); err != nil {
			log.Error("failed to load custom conf %s: %v", CustomConf, err)
			return
		}
	}

	callback(cfg)

	if err := os.MkdirAll(filepath.Dir(CustomConf), os.ModePerm); err != nil {
		log.Fatal("failed to create '%s': %v", CustomConf, err)
		return
	}
	if err := cfg.SaveTo(CustomConf); err != nil {
		log.Fatal("error saving to custom config: %v", err)
	}
}

// NewServices initializes the services
func NewServices() {
	InitDBConfig()
	newService()
	newOAuth2Client()
	NewLogServices(false)
	newCacheService()
	newSessionService()
	newCORSService()
	newMailService()
	newRegisterMailService()
	newNotifyMailService()
	newWebhookService()
	newMigrationsService()
	newIndexerService()
	newTaskService()
	NewQueueService()
	newProject()
	newMimeTypeMap()
}

// NewServicesForInstall initializes the services for install
func NewServicesForInstall() {
	newService()
	newMailService()
}
