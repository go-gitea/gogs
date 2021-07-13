// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/pprof"
	"code.gitea.io/gitea/modules/private"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/lfs"

	"github.com/dgrijalva/jwt-go"
	jsoniter "github.com/json-iterator/go"
	"github.com/kballard/go-shellquote"
	"github.com/urfave/cli"
)

const (
	lfsAuthenticateVerb = "git-lfs-authenticate"
)

// CmdServ represents the available serv sub-command.
var CmdServ = cli.Command{
	Name:        "serv",
	Usage:       "This command should only be called by SSH shell",
	Description: `Serv provide access auth for repositories`,
	Action:      runServ,
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name: "enable-pprof",
		},
		cli.BoolFlag{
			Name: "debug",
		},
	},
}

func setup(logPath string, debug bool) {
	_ = log.DelLogger("console")
	if debug {
		_ = log.NewLogger(1000, "console", "console", `{"level":"trace","stacktracelevel":"NONE","stderr":true}`)
	} else {
		_ = log.NewLogger(1000, "console", "console", `{"level":"fatal","stacktracelevel":"NONE","stderr":true}`)
	}
	setting.NewContext()
	if debug {
		setting.RunMode = "dev"
	}
}

var (
	allowedCommands = map[string]models.AccessMode{
		"git-upload-pack":    models.AccessModeRead,
		"git-upload-archive": models.AccessModeRead,
		"git-receive-pack":   models.AccessModeWrite,
		lfsAuthenticateVerb:  models.AccessModeNone,
	}
	alphaDashDotPattern = regexp.MustCompile(`[^\w-\.]`)
)

func fail(userMessage, logMessage string, args ...interface{}) {
	fmt.Fprintln(os.Stderr, "Gitea:", userMessage)

	if len(logMessage) > 0 {
		if !setting.IsProd() {
			fmt.Fprintf(os.Stderr, logMessage+"\n", args...)
		}
	}

	if len(logMessage) > 0 {
		_ = private.SSHLog(true, fmt.Sprintf(logMessage+": ", args...))
	}

	os.Exit(1)
}

func runServ(c *cli.Context) error {
	// FIXME: This needs to internationalised
	setup("serv.log", c.Bool("debug"))

	if setting.SSH.Disabled {
		println("Gitea: SSH has been disabled")
		return nil
	}

	if len(c.Args()) < 1 {
		if err := cli.ShowSubcommandHelp(c); err != nil {
			fmt.Printf("error showing subcommand help: %v\n", err)
		}
		return nil
	}

	keys := strings.Split(c.Args()[0], "-")
	if len(keys) != 2 || keys[0] != "key" {
		fail("Key ID format error", "Invalid key argument: %s", c.Args()[0])
	}
	keyID, err := strconv.ParseInt(keys[1], 10, 64)
	if err != nil {
		fail("Key ID format error", "Invalid key argument: %s", c.Args()[1])
	}

	cmd := os.Getenv("SSH_ORIGINAL_COMMAND")
	if len(cmd) == 0 {
		key, user, err := private.ServNoCommand(keyID)
		if err != nil {
			fail("Internal error", "Failed to check provided key: %v", err)
		}
		switch key.Type {
		case models.KeyTypeDeploy:
			println("Hi there! You've successfully authenticated with the deploy key named " + key.Name + ", but Gitea does not provide shell access.")
		case models.KeyTypePrincipal:
			println("Hi there! You've successfully authenticated with the principal " + key.Content + ", but Gitea does not provide shell access.")
		default:
			println("Hi there, " + user.Name + "! You've successfully authenticated with the key named " + key.Name + ", but Gitea does not provide shell access.")
		}
		println("If this is unexpected, please log in with password and setup Gitea under another user.")
		return nil
	} else if c.Bool("debug") {
		log.Debug("SSH_ORIGINAL_COMMAND: %s", os.Getenv("SSH_ORIGINAL_COMMAND"))
	}

	words, err := shellquote.Split(cmd)
	if err != nil {
		fail("Error parsing arguments", "Failed to parse arguments: %v", err)
	}

	if len(words) < 2 {
		fail("Too few arguments", "Too few arguments in cmd: %s", cmd)
	}

	verb := words[0]
	repoPath := words[1]
	if repoPath[0] == '/' {
		repoPath = repoPath[1:]
	}

	var lfsVerb string
	if verb == lfsAuthenticateVerb {
		if !setting.LFS.StartServer {
			fail("Unknown git command", "LFS authentication request over SSH denied, LFS support is disabled")
		}

		if len(words) > 2 {
			lfsVerb = words[2]
		}
	}

	// LowerCase and trim the repoPath as that's how they are stored.
	repoPath = strings.ToLower(strings.TrimSpace(repoPath))

	rr := strings.SplitN(repoPath, "/", 2)
	if len(rr) != 2 {
		fail("Invalid repository path", "Invalid repository path: %v", repoPath)
	}

	username := strings.ToLower(rr[0])
	reponame := strings.ToLower(strings.TrimSuffix(rr[1], ".git"))

	if alphaDashDotPattern.MatchString(reponame) {
		fail("Invalid repo name", "Invalid repo name: %s", reponame)
	}

	if setting.EnablePprof || c.Bool("enable-pprof") {
		if err := os.MkdirAll(setting.PprofDataPath, os.ModePerm); err != nil {
			fail("Error while trying to create PPROF_DATA_PATH", "Error while trying to create PPROF_DATA_PATH: %v", err)
		}

		stopCPUProfiler, err := pprof.DumpCPUProfileForUsername(setting.PprofDataPath, username)
		if err != nil {
			fail("Internal Server Error", "Unable to start CPU profile: %v", err)
		}
		defer func() {
			stopCPUProfiler()
			err := pprof.DumpMemProfileForUsername(setting.PprofDataPath, username)
			if err != nil {
				fail("Internal Server Error", "Unable to dump Mem Profile: %v", err)
			}
		}()
	}

	requestedMode, has := allowedCommands[verb]
	if !has {
		fail("Unknown git command", "Unknown git command %s", verb)
	}

	if verb == lfsAuthenticateVerb {
		if lfsVerb == "upload" {
			requestedMode = models.AccessModeWrite
		} else if lfsVerb == "download" {
			requestedMode = models.AccessModeRead
		} else {
			fail("Unknown LFS verb", "Unknown lfs verb %s", lfsVerb)
		}
	}

	results, err := private.ServCommand(keyID, username, reponame, requestedMode, verb, lfsVerb)
	if err != nil {
		if private.IsErrServCommand(err) {
			errServCommand := err.(private.ErrServCommand)
			if errServCommand.StatusCode != http.StatusInternalServerError {
				fail("Unauthorized", "%s", errServCommand.Error())
			} else {
				fail("Internal Server Error", "%s", errServCommand.Error())
			}
		}
		fail("Internal Server Error", "%s", err.Error())
	}
	os.Setenv(models.EnvRepoIsWiki, strconv.FormatBool(results.IsWiki))
	os.Setenv(models.EnvRepoName, results.RepoName)
	os.Setenv(models.EnvRepoUsername, results.OwnerName)
	os.Setenv(models.EnvPusherName, results.UserName)
	os.Setenv(models.EnvPusherEmail, results.UserEmail)
	os.Setenv(models.EnvPusherID, strconv.FormatInt(results.UserID, 10))
	os.Setenv(models.EnvRepoID, strconv.FormatInt(results.RepoID, 10))
	os.Setenv(models.EnvPRID, fmt.Sprintf("%d", 0))
	os.Setenv(models.EnvIsDeployKey, fmt.Sprintf("%t", results.IsDeployKey))
	os.Setenv(models.EnvKeyID, fmt.Sprintf("%d", results.KeyID))
	os.Setenv(models.EnvAppURL, setting.AppURL)

	//LFS token authentication
	if verb == lfsAuthenticateVerb {
		url := fmt.Sprintf("%s%s/%s.git/info/lfs", setting.AppURL, url.PathEscape(results.OwnerName), url.PathEscape(results.RepoName))

		now := time.Now()
		claims := lfs.Claims{
			StandardClaims: jwt.StandardClaims{
				ExpiresAt: now.Add(setting.LFS.HTTPAuthExpiry).Unix(),
				NotBefore: now.Unix(),
			},
			RepoID: results.RepoID,
			Op:     lfsVerb,
			UserID: results.UserID,
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

		// Sign and get the complete encoded token as a string using the secret
		tokenString, err := token.SignedString(setting.LFS.JWTSecretBytes)
		if err != nil {
			fail("Internal error", "Failed to sign JWT token: %v", err)
		}

		tokenAuthentication := &models.LFSTokenResponse{
			Header: make(map[string]string),
			Href:   url,
		}
		tokenAuthentication.Header["Authorization"] = fmt.Sprintf("Bearer %s", tokenString)

		json := jsoniter.ConfigCompatibleWithStandardLibrary
		enc := json.NewEncoder(os.Stdout)
		err = enc.Encode(tokenAuthentication)
		if err != nil {
			fail("Internal error", "Failed to encode LFS json response: %v", err)
		}
		return nil
	}

	// Special handle for Windows.
	if setting.IsWindows {
		verb = strings.Replace(verb, "-", " ", 1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		// install notify
		signalChannel := make(chan os.Signal, 1)

		signal.Notify(
			signalChannel,
			syscall.SIGINT,
			syscall.SIGTERM,
		)
		select {
		case <-signalChannel:
		case <-ctx.Done():
		}
		cancel()
		signal.Reset()
	}()

	var gitcmd *exec.Cmd
	verbs := strings.Split(verb, " ")
	if len(verbs) == 2 {
		gitcmd = exec.CommandContext(ctx, verbs[0], verbs[1], repoPath)
	} else {
		gitcmd = exec.CommandContext(ctx, verb, repoPath)
	}

	gitcmd.Dir = setting.RepoRootPath
	gitcmd.Stdout = os.Stdout
	gitcmd.Stdin = os.Stdin
	gitcmd.Stderr = os.Stderr
	if err = gitcmd.Run(); err != nil {
		fail("Internal error", "Failed to execute git command: %v", err)
	}

	// Update user key activity.
	if results.KeyID > 0 {
		if err = private.UpdatePublicKeyInRepo(results.KeyID, results.RepoID); err != nil {
			fail("Internal error", "UpdatePublicKeyInRepo: %v", err)
		}
	}

	return nil
}
