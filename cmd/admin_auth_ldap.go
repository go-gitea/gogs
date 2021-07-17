// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/services/auth/source/ldap"

	"github.com/urfave/cli"
)

type (
	authService struct {
		initDB             func() error
		createLoginSource  func(loginSource *models.LoginSource) error
		updateLoginSource  func(loginSource *models.LoginSource) error
		getLoginSourceByID func(id int64) (*models.LoginSource, error)
	}
)

var (
	commonLdapCLIFlags = []cli.Flag{
		cli.StringFlag{
			Name:  "name",
			Usage: "Authentication name.",
		},
		cli.BoolFlag{
			Name:  "not-active",
			Usage: "Deactivate the authentication source.",
		},
		cli.StringFlag{
			Name:  "security-protocol",
			Usage: "Security protocol name.",
		},
		cli.BoolFlag{
			Name:  "skip-tls-verify",
			Usage: "Disable TLS verification.",
		},
		cli.StringFlag{
			Name:  "host",
			Usage: "The address where the LDAP server can be reached.",
		},
		cli.IntFlag{
			Name:  "port",
			Usage: "The port to use when connecting to the LDAP server.",
		},
		cli.StringFlag{
			Name:  "user-search-base",
			Usage: "The LDAP base at which user accounts will be searched for.",
		},
		cli.StringFlag{
			Name:  "user-filter",
			Usage: "An LDAP filter declaring how to find the user record that is attempting to authenticate.",
		},
		cli.StringFlag{
			Name:  "admin-filter",
			Usage: "An LDAP filter specifying if a user should be given administrator privileges.",
		},
		cli.StringFlag{
			Name:  "restricted-filter",
			Usage: "An LDAP filter specifying if a user should be given restricted status.",
		},
		cli.BoolFlag{
			Name:  "allow-deactivate-all",
			Usage: "Allow empty search results to deactivate all users.",
		},
		cli.StringFlag{
			Name:  "username-attribute",
			Usage: "The attribute of the user’s LDAP record containing the user name.",
		},
		cli.StringFlag{
			Name:  "firstname-attribute",
			Usage: "The attribute of the user’s LDAP record containing the user’s first name.",
		},
		cli.StringFlag{
			Name:  "surname-attribute",
			Usage: "The attribute of the user’s LDAP record containing the user’s surname.",
		},
		cli.StringFlag{
			Name:  "email-attribute",
			Usage: "The attribute of the user’s LDAP record containing the user’s email address.",
		},
		cli.StringFlag{
			Name:  "public-ssh-key-attribute",
			Usage: "The attribute of the user’s LDAP record containing the user’s public ssh key.",
		},
	}

	ldapBindDnCLIFlags = append(commonLdapCLIFlags,
		cli.StringFlag{
			Name:  "bind-dn",
			Usage: "The DN to bind to the LDAP server with when searching for the user.",
		},
		cli.StringFlag{
			Name:  "bind-password",
			Usage: "The password for the Bind DN, if any.",
		},
		cli.BoolFlag{
			Name:  "attributes-in-bind",
			Usage: "Fetch attributes in bind DN context.",
		},
		cli.BoolFlag{
			Name:  "synchronize-users",
			Usage: "Enable user synchronization.",
		},
		cli.UintFlag{
			Name:  "page-size",
			Usage: "Search page size.",
		})

	ldapSimpleAuthCLIFlags = append(commonLdapCLIFlags,
		cli.StringFlag{
			Name:  "user-dn",
			Usage: "The user’s DN.",
		})

	cmdAuthAddLdapBindDn = cli.Command{
		Name:  "add-ldap",
		Usage: "Add new LDAP (via Bind DN) authentication source",
		Action: func(c *cli.Context) error {
			return newAuthService().addLdapBindDn(c)
		},
		Flags: ldapBindDnCLIFlags,
	}

	cmdAuthUpdateLdapBindDn = cli.Command{
		Name:  "update-ldap",
		Usage: "Update existing LDAP (via Bind DN) authentication source",
		Action: func(c *cli.Context) error {
			return newAuthService().updateLdapBindDn(c)
		},
		Flags: append([]cli.Flag{idFlag}, ldapBindDnCLIFlags...),
	}

	cmdAuthAddLdapSimpleAuth = cli.Command{
		Name:  "add-ldap-simple",
		Usage: "Add new LDAP (simple auth) authentication source",
		Action: func(c *cli.Context) error {
			return newAuthService().addLdapSimpleAuth(c)
		},
		Flags: ldapSimpleAuthCLIFlags,
	}

	cmdAuthUpdateLdapSimpleAuth = cli.Command{
		Name:  "update-ldap-simple",
		Usage: "Update existing LDAP (simple auth) authentication source",
		Action: func(c *cli.Context) error {
			return newAuthService().updateLdapSimpleAuth(c)
		},
		Flags: append([]cli.Flag{idFlag}, ldapSimpleAuthCLIFlags...),
	}
)

// newAuthService creates a service with default functions.
func newAuthService() *authService {
	return &authService{
		initDB:             initDB,
		createLoginSource:  models.CreateLoginSource,
		updateLoginSource:  models.UpdateSource,
		getLoginSourceByID: models.GetLoginSourceByID,
	}
}

// parseLoginSource assigns values on loginSource according to command line flags.
func parseLoginSource(c *cli.Context, loginSource *models.LoginSource) {
	if c.IsSet("name") {
		loginSource.Name = c.String("name")
	}
	if c.IsSet("not-active") {
		loginSource.IsActive = !c.Bool("not-active")
	}
	if c.IsSet("synchronize-users") {
		loginSource.IsSyncEnabled = c.Bool("synchronize-users")
	}
}

// parseLdapConfig assigns values on config according to command line flags.
func parseLdapConfig(c *cli.Context, config *ldap.Source) error {
	if c.IsSet("name") {
		config.Name = c.String("name")
	}
	if c.IsSet("host") {
		config.Host = c.String("host")
	}
	if c.IsSet("port") {
		config.Port = c.Int("port")
	}
	if c.IsSet("security-protocol") {
		p, ok := findLdapSecurityProtocolByName(c.String("security-protocol"))
		if !ok {
			return fmt.Errorf("Unknown security protocol name: %s", c.String("security-protocol"))
		}
		config.SecurityProtocol = p
	}
	if c.IsSet("skip-tls-verify") {
		config.SkipVerify = c.Bool("skip-tls-verify")
	}
	if c.IsSet("bind-dn") {
		config.BindDN = c.String("bind-dn")
	}
	if c.IsSet("user-dn") {
		config.UserDN = c.String("user-dn")
	}
	if c.IsSet("bind-password") {
		config.BindPassword = c.String("bind-password")
	}
	if c.IsSet("user-search-base") {
		config.UserBase = c.String("user-search-base")
	}
	if c.IsSet("username-attribute") {
		config.AttributeUsername = c.String("username-attribute")
	}
	if c.IsSet("firstname-attribute") {
		config.AttributeName = c.String("firstname-attribute")
	}
	if c.IsSet("surname-attribute") {
		config.AttributeSurname = c.String("surname-attribute")
	}
	if c.IsSet("email-attribute") {
		config.AttributeMail = c.String("email-attribute")
	}
	if c.IsSet("attributes-in-bind") {
		config.AttributesInBind = c.Bool("attributes-in-bind")
	}
	if c.IsSet("public-ssh-key-attribute") {
		config.AttributeSSHPublicKey = c.String("public-ssh-key-attribute")
	}
	if c.IsSet("page-size") {
		config.SearchPageSize = uint32(c.Uint("page-size"))
	}
	if c.IsSet("user-filter") {
		config.Filter = c.String("user-filter")
	}
	if c.IsSet("admin-filter") {
		config.AdminFilter = c.String("admin-filter")
	}
	if c.IsSet("restricted-filter") {
		config.RestrictedFilter = c.String("restricted-filter")
	}
	if c.IsSet("allow-deactivate-all") {
		config.AllowDeactivateAll = c.Bool("allow-deactivate-all")
	}
	return nil
}

// findLdapSecurityProtocolByName finds security protocol by its name ignoring case.
// It returns the value of the security protocol and if it was found.
func findLdapSecurityProtocolByName(name string) (ldap.SecurityProtocol, bool) {
	for i, n := range ldap.SecurityProtocolNames {
		if strings.EqualFold(name, n) {
			return i, true
		}
	}
	return 0, false
}

// getLoginSource gets the login source by its id defined in the command line flags.
// It returns an error if the id is not set, does not match any source or if the source is not of expected type.
func (a *authService) getLoginSource(c *cli.Context, loginType models.LoginType) (*models.LoginSource, error) {
	if err := argsSet(c, "id"); err != nil {
		return nil, err
	}

	loginSource, err := a.getLoginSourceByID(c.Int64("id"))
	if err != nil {
		return nil, err
	}

	if loginSource.Type != loginType {
		return nil, fmt.Errorf("Invalid authentication type. expected: %s, actual: %s", models.LoginNames[loginType], models.LoginNames[loginSource.Type])
	}

	return loginSource, nil
}

// addLdapBindDn adds a new LDAP via Bind DN authentication source.
func (a *authService) addLdapBindDn(c *cli.Context) error {
	if err := argsSet(c, "name", "security-protocol", "host", "port", "user-search-base", "user-filter", "email-attribute"); err != nil {
		return err
	}

	if err := a.initDB(); err != nil {
		return err
	}

	loginSource := &models.LoginSource{
		Type:     models.LoginLDAP,
		IsActive: true, // active by default
		Cfg: &ldap.Source{
			Enabled: true, // always true
		},
	}

	parseLoginSource(c, loginSource)
	if err := parseLdapConfig(c, loginSource.Cfg.(*ldap.Source)); err != nil {
		return err
	}

	return a.createLoginSource(loginSource)
}

// updateLdapBindDn updates a new LDAP via Bind DN authentication source.
func (a *authService) updateLdapBindDn(c *cli.Context) error {
	if err := a.initDB(); err != nil {
		return err
	}

	loginSource, err := a.getLoginSource(c, models.LoginLDAP)
	if err != nil {
		return err
	}

	parseLoginSource(c, loginSource)
	if err := parseLdapConfig(c, loginSource.Cfg.(*ldap.Source)); err != nil {
		return err
	}

	return a.updateLoginSource(loginSource)
}

// addLdapSimpleAuth adds a new LDAP (simple auth) authentication source.
func (a *authService) addLdapSimpleAuth(c *cli.Context) error {
	if err := argsSet(c, "name", "security-protocol", "host", "port", "user-dn", "user-filter", "email-attribute"); err != nil {
		return err
	}

	if err := a.initDB(); err != nil {
		return err
	}

	loginSource := &models.LoginSource{
		Type:     models.LoginDLDAP,
		IsActive: true, // active by default
		Cfg: &ldap.Source{
			Enabled: true, // always true
		},
	}

	parseLoginSource(c, loginSource)
	if err := parseLdapConfig(c, loginSource.Cfg.(*ldap.Source)); err != nil {
		return err
	}

	return a.createLoginSource(loginSource)
}

// updateLdapBindDn updates a new LDAP (simple auth) authentication source.
func (a *authService) updateLdapSimpleAuth(c *cli.Context) error {
	if err := a.initDB(); err != nil {
		return err
	}

	loginSource, err := a.getLoginSource(c, models.LoginDLDAP)
	if err != nil {
		return err
	}

	parseLoginSource(c, loginSource)
	if err := parseLdapConfig(c, loginSource.Cfg.(*ldap.Source)); err != nil {
		return err
	}

	return a.updateLoginSource(loginSource)
}
