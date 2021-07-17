// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	pwd "code.gitea.io/gitea/modules/password"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/services/auth/source/oauth2"

	"github.com/urfave/cli"
)

var (
	// CmdAdmin represents the available admin sub-command.
	CmdAdmin = cli.Command{
		Name:  "admin",
		Usage: "Command line interface to perform common administrative operations",
		Subcommands: []cli.Command{
			subcmdUser,
			subcmdRepoSyncReleases,
			subcmdRegenerate,
			subcmdAuth,
			subcmdSendMail,
		},
	}

	subcmdUser = cli.Command{
		Name:  "user",
		Usage: "Modify users",
		Subcommands: []cli.Command{
			microcmdUserCreate,
			microcmdUserList,
			microcmdUserChangePassword,
			microcmdUserDelete,
		},
	}

	microcmdUserList = cli.Command{
		Name:   "list",
		Usage:  "List users",
		Action: runListUsers,
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "admin",
				Usage: "List only admin users",
			},
		},
	}

	microcmdUserCreate = cli.Command{
		Name:   "create",
		Usage:  "Create a new user in database",
		Action: runCreateUser,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "name",
				Usage: "Username. DEPRECATED: use username instead",
			},
			cli.StringFlag{
				Name:  "username",
				Usage: "Username",
			},
			cli.StringFlag{
				Name:  "password",
				Usage: "User password",
			},
			cli.StringFlag{
				Name:  "email",
				Usage: "User email address",
			},
			cli.BoolFlag{
				Name:  "admin",
				Usage: "User is an admin",
			},
			cli.BoolFlag{
				Name:  "random-password",
				Usage: "Generate a random password for the user",
			},
			cli.BoolFlag{
				Name:  "must-change-password",
				Usage: "Set this option to false to prevent forcing the user to change their password after initial login, (Default: true)",
			},
			cli.IntFlag{
				Name:  "random-password-length",
				Usage: "Length of the random password to be generated",
				Value: 12,
			},
			cli.BoolFlag{
				Name:  "access-token",
				Usage: "Generate access token for the user",
			},
		},
	}

	microcmdUserChangePassword = cli.Command{
		Name:   "change-password",
		Usage:  "Change a user's password",
		Action: runChangePassword,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "username,u",
				Value: "",
				Usage: "The user to change password for",
			},
			cli.StringFlag{
				Name:  "password,p",
				Value: "",
				Usage: "New password to set for user",
			},
		},
	}

	microcmdUserDelete = cli.Command{
		Name:  "delete",
		Usage: "Delete specific user by id, name or email",
		Flags: []cli.Flag{
			cli.Int64Flag{
				Name:  "id",
				Usage: "ID of user of the user to delete",
			},
			cli.StringFlag{
				Name:  "username,u",
				Usage: "Username of the user to delete",
			},
			cli.StringFlag{
				Name:  "email,e",
				Usage: "Email of the user to delete",
			},
		},
		Action: runDeleteUser,
	}

	subcmdRepoSyncReleases = cli.Command{
		Name:   "repo-sync-releases",
		Usage:  "Synchronize repository releases with tags",
		Action: runRepoSyncReleases,
	}

	subcmdRegenerate = cli.Command{
		Name:  "regenerate",
		Usage: "Regenerate specific files",
		Subcommands: []cli.Command{
			microcmdRegenHooks,
			microcmdRegenKeys,
		},
	}

	microcmdRegenHooks = cli.Command{
		Name:   "hooks",
		Usage:  "Regenerate git-hooks",
		Action: runRegenerateHooks,
	}

	microcmdRegenKeys = cli.Command{
		Name:   "keys",
		Usage:  "Regenerate authorized_keys file",
		Action: runRegenerateKeys,
	}

	subcmdAuth = cli.Command{
		Name:  "auth",
		Usage: "Modify external auth providers",
		Subcommands: []cli.Command{
			microcmdAuthAddOauth,
			microcmdAuthUpdateOauth,
			cmdAuthAddLdapBindDn,
			cmdAuthUpdateLdapBindDn,
			cmdAuthAddLdapSimpleAuth,
			cmdAuthUpdateLdapSimpleAuth,
			microcmdAuthList,
			microcmdAuthDelete,
		},
	}

	microcmdAuthList = cli.Command{
		Name:   "list",
		Usage:  "List auth sources",
		Action: runListAuth,
		Flags: []cli.Flag{
			cli.IntFlag{
				Name:  "min-width",
				Usage: "Minimal cell width including any padding for the formatted table",
				Value: 0,
			},
			cli.IntFlag{
				Name:  "tab-width",
				Usage: "width of tab characters in formatted table (equivalent number of spaces)",
				Value: 8,
			},
			cli.IntFlag{
				Name:  "padding",
				Usage: "padding added to a cell before computing its width",
				Value: 1,
			},
			cli.StringFlag{
				Name:  "pad-char",
				Usage: `ASCII char used for padding if padchar == '\\t', the Writer will assume that the width of a '\\t' in the formatted output is tabwidth, and cells are left-aligned independent of align_left (for correct-looking results, tabwidth must correspond to the tab width in the viewer displaying the result)`,
				Value: "\t",
			},
			cli.BoolFlag{
				Name:  "vertical-bars",
				Usage: "Set to true to print vertical bars between columns",
			},
		},
	}

	idFlag = cli.Int64Flag{
		Name:  "id",
		Usage: "ID of authentication source",
	}

	microcmdAuthDelete = cli.Command{
		Name:   "delete",
		Usage:  "Delete specific auth source",
		Flags:  []cli.Flag{idFlag},
		Action: runDeleteAuth,
	}

	oauthCLIFlags = []cli.Flag{
		cli.StringFlag{
			Name:  "name",
			Value: "",
			Usage: "Application Name",
		},
		cli.StringFlag{
			Name:  "provider",
			Value: "",
			Usage: "OAuth2 Provider",
		},
		cli.StringFlag{
			Name:  "key",
			Value: "",
			Usage: "Client ID (Key)",
		},
		cli.StringFlag{
			Name:  "secret",
			Value: "",
			Usage: "Client Secret",
		},
		cli.StringFlag{
			Name:  "auto-discover-url",
			Value: "",
			Usage: "OpenID Connect Auto Discovery URL (only required when using OpenID Connect as provider)",
		},
		cli.StringFlag{
			Name:  "use-custom-urls",
			Value: "false",
			Usage: "Use custom URLs for GitLab/GitHub OAuth endpoints",
		},
		cli.StringFlag{
			Name:  "custom-auth-url",
			Value: "",
			Usage: "Use a custom Authorization URL (option for GitLab/GitHub)",
		},
		cli.StringFlag{
			Name:  "custom-token-url",
			Value: "",
			Usage: "Use a custom Token URL (option for GitLab/GitHub)",
		},
		cli.StringFlag{
			Name:  "custom-profile-url",
			Value: "",
			Usage: "Use a custom Profile URL (option for GitLab/GitHub)",
		},
		cli.StringFlag{
			Name:  "custom-email-url",
			Value: "",
			Usage: "Use a custom Email URL (option for GitHub)",
		},
		cli.StringFlag{
			Name:  "icon-url",
			Value: "",
			Usage: "Custom icon URL for OAuth2 login source",
		},
	}

	microcmdAuthUpdateOauth = cli.Command{
		Name:   "update-oauth",
		Usage:  "Update existing Oauth authentication source",
		Action: runUpdateOauth,
		Flags:  append(oauthCLIFlags[:1], append([]cli.Flag{idFlag}, oauthCLIFlags[1:]...)...),
	}

	microcmdAuthAddOauth = cli.Command{
		Name:   "add-oauth",
		Usage:  "Add new Oauth authentication source",
		Action: runAddOauth,
		Flags:  oauthCLIFlags,
	}

	subcmdSendMail = cli.Command{
		Name:   "sendmail",
		Usage:  "Send a message to all users",
		Action: runSendMail,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "title",
				Usage: `a title of a message`,
				Value: "",
			},
			cli.StringFlag{
				Name:  "content",
				Usage: "a content of a message",
				Value: "",
			},
			cli.BoolFlag{
				Name:  "force,f",
				Usage: "A flag to bypass a confirmation step",
			},
		},
	}
)

func runChangePassword(c *cli.Context) error {
	if err := argsSet(c, "username", "password"); err != nil {
		return err
	}

	if err := initDB(); err != nil {
		return err
	}
	if !pwd.IsComplexEnough(c.String("password")) {
		return errors.New("Password does not meet complexity requirements")
	}
	pwned, err := pwd.IsPwned(context.Background(), c.String("password"))
	if err != nil {
		return err
	}
	if pwned {
		return errors.New("The password you chose is on a list of stolen passwords previously exposed in public data breaches. Please try again with a different password.\nFor more details, see https://haveibeenpwned.com/Passwords")
	}
	uname := c.String("username")
	user, err := models.GetUserByName(uname)
	if err != nil {
		return err
	}
	if err = user.SetPassword(c.String("password")); err != nil {
		return err
	}

	if err = models.UpdateUserCols(user, "passwd", "passwd_hash_algo", "salt"); err != nil {
		return err
	}

	fmt.Printf("%s's password has been successfully updated!\n", user.Name)
	return nil
}

func runCreateUser(c *cli.Context) error {
	if err := argsSet(c, "email"); err != nil {
		return err
	}

	if c.IsSet("name") && c.IsSet("username") {
		return errors.New("Cannot set both --name and --username flags")
	}
	if !c.IsSet("name") && !c.IsSet("username") {
		return errors.New("One of --name or --username flags must be set")
	}

	if c.IsSet("password") && c.IsSet("random-password") {
		return errors.New("cannot set both -random-password and -password flags")
	}

	var username string
	if c.IsSet("username") {
		username = c.String("username")
	} else {
		username = c.String("name")
		fmt.Fprintf(os.Stderr, "--name flag is deprecated. Use --username instead.\n")
	}

	if err := initDB(); err != nil {
		return err
	}

	var password string
	if c.IsSet("password") {
		password = c.String("password")
	} else if c.IsSet("random-password") {
		var err error
		password, err = pwd.Generate(c.Int("random-password-length"))
		if err != nil {
			return err
		}
		fmt.Printf("generated random password is '%s'\n", password)
	} else {
		return errors.New("must set either password or random-password flag")
	}

	// always default to true
	var changePassword = true

	// If this is the first user being created.
	// Take it as the admin and don't force a password update.
	if n := models.CountUsers(); n == 0 {
		changePassword = false
	}

	if c.IsSet("must-change-password") {
		changePassword = c.Bool("must-change-password")
	}

	u := &models.User{
		Name:               username,
		Email:              c.String("email"),
		Passwd:             password,
		IsActive:           true,
		IsAdmin:            c.Bool("admin"),
		MustChangePassword: changePassword,
		Theme:              setting.UI.DefaultTheme,
	}

	if err := models.CreateUser(u); err != nil {
		return fmt.Errorf("CreateUser: %v", err)
	}

	if c.Bool("access-token") {
		t := &models.AccessToken{
			Name: "gitea-admin",
			UID:  u.ID,
		}

		if err := models.NewAccessToken(t); err != nil {
			return err
		}

		fmt.Printf("Access token was successfully created... %s\n", t.Token)
	}

	fmt.Printf("New user '%s' has been successfully created!\n", username)
	return nil
}

func runListUsers(c *cli.Context) error {
	if err := initDB(); err != nil {
		return err
	}

	users, err := models.GetAllUsers()

	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 5, 0, 1, ' ', 0)

	if c.IsSet("admin") {
		fmt.Fprintf(w, "ID\tUsername\tEmail\tIsActive\n")
		for _, u := range users {
			if u.IsAdmin {
				fmt.Fprintf(w, "%d\t%s\t%s\t%t\n", u.ID, u.Name, u.Email, u.IsActive)
			}
		}
	} else {
		fmt.Fprintf(w, "ID\tUsername\tEmail\tIsActive\tIsAdmin\n")
		for _, u := range users {
			fmt.Fprintf(w, "%d\t%s\t%s\t%t\t%t\n", u.ID, u.Name, u.Email, u.IsActive, u.IsAdmin)
		}

	}

	w.Flush()
	return nil

}

func runDeleteUser(c *cli.Context) error {
	if !c.IsSet("id") && !c.IsSet("username") && !c.IsSet("email") {
		return fmt.Errorf("You must provide the id, username or email of a user to delete")
	}

	if err := initDB(); err != nil {
		return err
	}

	if err := storage.Init(); err != nil {
		return err
	}

	var err error
	var user *models.User
	if c.IsSet("email") {
		user, err = models.GetUserByEmail(c.String("email"))
	} else if c.IsSet("username") {
		user, err = models.GetUserByName(c.String("username"))
	} else {
		user, err = models.GetUserByID(c.Int64("id"))
	}
	if err != nil {
		return err
	}
	if c.IsSet("username") && user.LowerName != strings.ToLower(strings.TrimSpace(c.String("username"))) {
		return fmt.Errorf("The user %s who has email %s does not match the provided username %s", user.Name, c.String("email"), c.String("username"))
	}

	if c.IsSet("id") && user.ID != c.Int64("id") {
		return fmt.Errorf("The user %s does not match the provided id %d", user.Name, c.Int64("id"))
	}

	return models.DeleteUser(user)
}

func runRepoSyncReleases(_ *cli.Context) error {
	if err := initDB(); err != nil {
		return err
	}

	log.Trace("Synchronizing repository releases (this may take a while)")
	for page := 1; ; page++ {
		repos, count, err := models.SearchRepositoryByName(&models.SearchRepoOptions{
			ListOptions: models.ListOptions{
				PageSize: models.RepositoryListDefaultPageSize,
				Page:     page,
			},
			Private: true,
		})
		if err != nil {
			return fmt.Errorf("SearchRepositoryByName: %v", err)
		}
		if len(repos) == 0 {
			break
		}
		log.Trace("Processing next %d repos of %d", len(repos), count)
		for _, repo := range repos {
			log.Trace("Synchronizing repo %s with path %s", repo.FullName(), repo.RepoPath())
			gitRepo, err := git.OpenRepository(repo.RepoPath())
			if err != nil {
				log.Warn("OpenRepository: %v", err)
				continue
			}

			oldnum, err := getReleaseCount(repo.ID)
			if err != nil {
				log.Warn(" GetReleaseCountByRepoID: %v", err)
			}
			log.Trace(" currentNumReleases is %d, running SyncReleasesWithTags", oldnum)

			if err = repo_module.SyncReleasesWithTags(repo, gitRepo); err != nil {
				log.Warn(" SyncReleasesWithTags: %v", err)
				gitRepo.Close()
				continue
			}

			count, err = getReleaseCount(repo.ID)
			if err != nil {
				log.Warn(" GetReleaseCountByRepoID: %v", err)
				gitRepo.Close()
				continue
			}

			log.Trace(" repo %s releases synchronized to tags: from %d to %d",
				repo.FullName(), oldnum, count)
			gitRepo.Close()
		}
	}

	return nil
}

func getReleaseCount(id int64) (int64, error) {
	return models.GetReleaseCountByRepoID(
		id,
		models.FindReleasesOptions{
			IncludeTags: true,
		},
	)
}

func runRegenerateHooks(_ *cli.Context) error {
	if err := initDB(); err != nil {
		return err
	}
	return repo_module.SyncRepositoryHooks(graceful.GetManager().ShutdownContext())
}

func runRegenerateKeys(_ *cli.Context) error {
	if err := initDB(); err != nil {
		return err
	}
	return models.RewriteAllPublicKeys()
}

func parseOAuth2Config(c *cli.Context) *oauth2.Source {
	var customURLMapping *oauth2.CustomURLMapping
	if c.IsSet("use-custom-urls") {
		customURLMapping = &oauth2.CustomURLMapping{
			TokenURL:   c.String("custom-token-url"),
			AuthURL:    c.String("custom-auth-url"),
			ProfileURL: c.String("custom-profile-url"),
			EmailURL:   c.String("custom-email-url"),
		}
	} else {
		customURLMapping = nil
	}
	return &oauth2.Source{
		Provider:                      c.String("provider"),
		ClientID:                      c.String("key"),
		ClientSecret:                  c.String("secret"),
		OpenIDConnectAutoDiscoveryURL: c.String("auto-discover-url"),
		CustomURLMapping:              customURLMapping,
		IconURL:                       c.String("icon-url"),
	}
}

func runAddOauth(c *cli.Context) error {
	if err := initDB(); err != nil {
		return err
	}

	return models.CreateLoginSource(&models.LoginSource{
		Type:     models.LoginOAuth2,
		Name:     c.String("name"),
		IsActive: true,
		Cfg:      parseOAuth2Config(c),
	})
}

func runUpdateOauth(c *cli.Context) error {
	if !c.IsSet("id") {
		return fmt.Errorf("--id flag is missing")
	}

	if err := initDB(); err != nil {
		return err
	}

	source, err := models.GetLoginSourceByID(c.Int64("id"))
	if err != nil {
		return err
	}

	oAuth2Config := source.Cfg.(*oauth2.Source)

	if c.IsSet("name") {
		source.Name = c.String("name")
	}

	if c.IsSet("provider") {
		oAuth2Config.Provider = c.String("provider")
	}

	if c.IsSet("key") {
		oAuth2Config.ClientID = c.String("key")
	}

	if c.IsSet("secret") {
		oAuth2Config.ClientSecret = c.String("secret")
	}

	if c.IsSet("auto-discover-url") {
		oAuth2Config.OpenIDConnectAutoDiscoveryURL = c.String("auto-discover-url")
	}

	if c.IsSet("icon-url") {
		oAuth2Config.IconURL = c.String("icon-url")
	}

	// update custom URL mapping
	var customURLMapping = &oauth2.CustomURLMapping{}

	if oAuth2Config.CustomURLMapping != nil {
		customURLMapping.TokenURL = oAuth2Config.CustomURLMapping.TokenURL
		customURLMapping.AuthURL = oAuth2Config.CustomURLMapping.AuthURL
		customURLMapping.ProfileURL = oAuth2Config.CustomURLMapping.ProfileURL
		customURLMapping.EmailURL = oAuth2Config.CustomURLMapping.EmailURL
	}
	if c.IsSet("use-custom-urls") && c.IsSet("custom-token-url") {
		customURLMapping.TokenURL = c.String("custom-token-url")
	}

	if c.IsSet("use-custom-urls") && c.IsSet("custom-auth-url") {
		customURLMapping.AuthURL = c.String("custom-auth-url")
	}

	if c.IsSet("use-custom-urls") && c.IsSet("custom-profile-url") {
		customURLMapping.ProfileURL = c.String("custom-profile-url")
	}

	if c.IsSet("use-custom-urls") && c.IsSet("custom-email-url") {
		customURLMapping.EmailURL = c.String("custom-email-url")
	}

	oAuth2Config.CustomURLMapping = customURLMapping
	source.Cfg = oAuth2Config

	return models.UpdateSource(source)
}

func runListAuth(c *cli.Context) error {
	if err := initDB(); err != nil {
		return err
	}

	loginSources, err := models.LoginSources()

	if err != nil {
		return err
	}

	flags := tabwriter.AlignRight
	if c.Bool("vertical-bars") {
		flags |= tabwriter.Debug
	}

	padChar := byte('\t')
	if len(c.String("pad-char")) > 0 {
		padChar = c.String("pad-char")[0]
	}

	// loop through each source and print
	w := tabwriter.NewWriter(os.Stdout, c.Int("min-width"), c.Int("tab-width"), c.Int("padding"), padChar, flags)
	fmt.Fprintf(w, "ID\tName\tType\tEnabled\n")
	for _, source := range loginSources {
		fmt.Fprintf(w, "%d\t%s\t%s\t%t\n", source.ID, source.Name, models.LoginNames[source.Type], source.IsActive)
	}
	w.Flush()

	return nil
}

func runDeleteAuth(c *cli.Context) error {
	if !c.IsSet("id") {
		return fmt.Errorf("--id flag is missing")
	}

	if err := initDB(); err != nil {
		return err
	}

	source, err := models.GetLoginSourceByID(c.Int64("id"))
	if err != nil {
		return err
	}

	return models.DeleteSource(source)
}
