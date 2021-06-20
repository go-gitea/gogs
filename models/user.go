// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"container/list"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	_ "image/jpeg" // Needed for jpeg support
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/crypto/scrypt"
	"golang.org/x/crypto/ssh"
	"xorm.io/builder"
)

// UserType defines the user type
type UserType int

const (
	// UserTypeIndividual defines an individual user
	UserTypeIndividual UserType = iota // Historic reason to make it starts at 0.

	// UserTypeOrganization defines an organization
	UserTypeOrganization
)

const (
	algoBcrypt = "bcrypt"
	algoScrypt = "scrypt"
	algoArgon2 = "argon2"
	algoPbkdf2 = "pbkdf2"
)

// AvailableHashAlgorithms represents the available password hashing algorithms
var AvailableHashAlgorithms = []string{
	algoPbkdf2,
	algoArgon2,
	algoScrypt,
	algoBcrypt,
}

const (
	// EmailNotificationsEnabled indicates that the user would like to receive all email notifications
	EmailNotificationsEnabled = "enabled"
	// EmailNotificationsOnMention indicates that the user would like to be notified via email when mentioned.
	EmailNotificationsOnMention = "onmention"
	// EmailNotificationsDisabled indicates that the user would not like to be notified via email.
	EmailNotificationsDisabled = "disabled"
)

var (
	// ErrEmailNotActivated e-mail address has not been activated error
	ErrEmailNotActivated = errors.New("E-mail address has not been activated")

	// ErrUserNameIllegal user name contains illegal characters error
	ErrUserNameIllegal = errors.New("User name contains illegal characters")

	// ErrLoginSourceNotActived login source is not actived error
	ErrLoginSourceNotActived = errors.New("Login source is not actived")

	// ErrUnsupportedLoginType login source is unknown error
	ErrUnsupportedLoginType = errors.New("Login source is unknown")

	// Characters prohibited in a user name (anything except A-Za-z0-9_.-)
	alphaDashDotPattern = regexp.MustCompile(`[^\w-\.]`)
)

// User represents the object of individual and member of organization.
type User struct {
	ID        int64  `xorm:"pk autoincr"`
	LowerName string `xorm:"UNIQUE NOT NULL"`
	Name      string `xorm:"UNIQUE NOT NULL"`
	FullName  string
	// Email is the primary email address (to be used for communication)
	Email                        string `xorm:"NOT NULL"`
	KeepEmailPrivate             bool
	EmailNotificationsPreference string `xorm:"VARCHAR(20) NOT NULL DEFAULT 'enabled'"`
	Passwd                       string `xorm:"NOT NULL"`
	PasswdHashAlgo               string `xorm:"NOT NULL DEFAULT 'argon2'"`

	// MustChangePassword is an attribute that determines if a user
	// is to change his/her password after registration.
	MustChangePassword bool `xorm:"NOT NULL DEFAULT false"`

	LoginType   LoginType
	LoginSource int64 `xorm:"NOT NULL DEFAULT 0"`
	LoginName   string
	Type        UserType
	OwnedOrgs   []*User       `xorm:"-"`
	Repos       []*Repository `xorm:"-"`
	Location    string
	Website     string
	Rands       string `xorm:"VARCHAR(10)"`
	Salt        string `xorm:"VARCHAR(10)"`
	Language    string `xorm:"VARCHAR(5)"`
	Description string

	CreatedUnix   timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix   timeutil.TimeStamp `xorm:"INDEX updated"`
	LastLoginUnix timeutil.TimeStamp `xorm:"INDEX"`

	// Remember visibility choice for convenience, true for private
	LastRepoVisibility bool
	// Maximum repository creation limit, -1 means use global default
	MaxRepoCreation int `xorm:"NOT NULL DEFAULT -1"`

	// Permissions
	IsActive                bool `xorm:"INDEX"` // Activate primary email
	IsAdmin                 bool
	IsRestricted            bool `xorm:"NOT NULL DEFAULT false"`
	AllowGitHook            bool
	AllowImportLocal        bool // Allow migrate repository by local path
	AllowCreateOrganization bool `xorm:"DEFAULT true"`
	ProhibitLogin           bool `xorm:"NOT NULL DEFAULT false"`

	// Avatar
	Avatar          string `xorm:"VARCHAR(2048) NOT NULL"`
	AvatarEmail     string `xorm:"NOT NULL"`
	UseCustomAvatar bool

	// Counters
	NumFollowers int
	NumFollowing int `xorm:"NOT NULL DEFAULT 0"`
	NumStars     int
	NumRepos     int

	// For organization
	NumTeams                  int
	NumMembers                int
	Teams                     []*Team             `xorm:"-"`
	Members                   UserList            `xorm:"-"`
	MembersIsPublic           map[int64]bool      `xorm:"-"`
	Visibility                structs.VisibleType `xorm:"NOT NULL DEFAULT 0"`
	RepoAdminChangeTeamAccess bool                `xorm:"NOT NULL DEFAULT false"`

	// Preferences
	DiffViewStyle       string `xorm:"NOT NULL DEFAULT ''"`
	Theme               string `xorm:"NOT NULL DEFAULT ''"`
	KeepActivityPrivate bool   `xorm:"NOT NULL DEFAULT false"`
}

// SearchOrganizationsOptions options to filter organizations
type SearchOrganizationsOptions struct {
	ListOptions
	All bool
}

// ColorFormat writes a colored string to identify this struct
func (u *User) ColorFormat(s fmt.State) {
	log.ColorFprintf(s, "%d:%s",
		log.NewColoredIDValue(u.ID),
		log.NewColoredValue(u.Name))
}

// BeforeUpdate is invoked from XORM before updating this object.
func (u *User) BeforeUpdate() {
	if u.MaxRepoCreation < -1 {
		u.MaxRepoCreation = -1
	}

	// Organization does not need email
	u.Email = strings.ToLower(u.Email)
	if !u.IsOrganization() {
		if len(u.AvatarEmail) == 0 {
			u.AvatarEmail = u.Email
		}
	}

	u.LowerName = strings.ToLower(u.Name)
	u.Location = base.TruncateString(u.Location, 255)
	u.Website = base.TruncateString(u.Website, 255)
	u.Description = base.TruncateString(u.Description, 255)
}

// AfterLoad is invoked from XORM after filling all the fields of this object.
func (u *User) AfterLoad() {
	if u.Theme == "" {
		u.Theme = setting.UI.DefaultTheme
	}
}

// SetLastLogin set time to last login
func (u *User) SetLastLogin() {
	u.LastLoginUnix = timeutil.TimeStampNow()
}

// UpdateDiffViewStyle updates the users diff view style
func (u *User) UpdateDiffViewStyle(style string) error {
	u.DiffViewStyle = style
	return UpdateUserCols(u, "diff_view_style")
}

// UpdateTheme updates a users' theme irrespective of the site wide theme
func (u *User) UpdateTheme(themeName string) error {
	u.Theme = themeName
	return UpdateUserCols(u, "theme")
}

// GetEmail returns an noreply email, if the user has set to keep his
// email address private, otherwise the primary email address.
func (u *User) GetEmail() string {
	if u.KeepEmailPrivate {
		return fmt.Sprintf("%s@%s", u.LowerName, setting.Service.NoReplyAddress)
	}
	return u.Email
}

// GetAllUsers returns a slice of all individual users found in DB.
func GetAllUsers() ([]*User, error) {
	users := make([]*User, 0)
	return users, x.OrderBy("id").Where("type = ?", UserTypeIndividual).Find(&users)
}

// IsLocal returns true if user login type is LoginPlain.
func (u *User) IsLocal() bool {
	return u.LoginType <= LoginPlain
}

// IsOAuth2 returns true if user login type is LoginOAuth2.
func (u *User) IsOAuth2() bool {
	return u.LoginType == LoginOAuth2
}

// HasForkedRepo checks if user has already forked a repository with given ID.
func (u *User) HasForkedRepo(repoID int64) bool {
	_, has := HasForkedRepo(u.ID, repoID)
	return has
}

// MaxCreationLimit returns the number of repositories a user is allowed to create
func (u *User) MaxCreationLimit() int {
	if u.MaxRepoCreation <= -1 {
		return setting.Repository.MaxCreationLimit
	}
	return u.MaxRepoCreation
}

// CanCreateRepo returns if user login can create a repository
// NOTE: functions calling this assume a failure due to repository count limit; if new checks are added, those functions should be revised
func (u *User) CanCreateRepo() bool {
	if u.IsAdmin {
		return true
	}
	if u.MaxRepoCreation <= -1 {
		if setting.Repository.MaxCreationLimit <= -1 {
			return true
		}
		return u.NumRepos < setting.Repository.MaxCreationLimit
	}
	return u.NumRepos < u.MaxRepoCreation
}

// CanCreateOrganization returns true if user can create organisation.
func (u *User) CanCreateOrganization() bool {
	return u.IsAdmin || (u.AllowCreateOrganization && !setting.Admin.DisableRegularOrgCreation)
}

// CanEditGitHook returns true if user can edit Git hooks.
func (u *User) CanEditGitHook() bool {
	return !setting.DisableGitHooks && (u.IsAdmin || u.AllowGitHook)
}

// CanImportLocal returns true if user can migrate repository by local path.
func (u *User) CanImportLocal() bool {
	if !setting.ImportLocalPaths || u == nil {
		return false
	}
	return u.IsAdmin || u.AllowImportLocal
}

// DashboardLink returns the user dashboard page link.
func (u *User) DashboardLink() string {
	if u.IsOrganization() {
		return u.OrganisationLink() + "/dashboard/"
	}
	return setting.AppSubURL + "/"
}

// HomeLink returns the user or organization home page link.
func (u *User) HomeLink() string {
	return setting.AppSubURL + "/" + u.Name
}

// HTMLURL returns the user or organization's full link.
func (u *User) HTMLURL() string {
	return setting.AppURL + u.Name
}

// OrganisationLink returns the organization sub page link.
func (u *User) OrganisationLink() string {
	return setting.AppSubURL + "/org/" + u.Name
}

// GenerateEmailActivateCode generates an activate code based on user information and given e-mail.
func (u *User) GenerateEmailActivateCode(email string) string {
	code := base.CreateTimeLimitCode(
		fmt.Sprintf("%d%s%s%s%s", u.ID, email, u.LowerName, u.Passwd, u.Rands),
		setting.Service.ActiveCodeLives, nil)

	// Add tail hex username
	code += hex.EncodeToString([]byte(u.LowerName))
	return code
}

// GetFollowers returns range of user's followers.
func (u *User) GetFollowers(listOptions ListOptions) ([]*User, error) {
	sess := x.
		Where("follow.follow_id=?", u.ID).
		Join("LEFT", "follow", "`user`.id=follow.user_id")

	if listOptions.Page != 0 {
		sess = listOptions.setSessionPagination(sess)

		users := make([]*User, 0, listOptions.PageSize)
		return users, sess.Find(&users)
	}

	users := make([]*User, 0, 8)
	return users, sess.Find(&users)
}

// IsFollowing returns true if user is following followID.
func (u *User) IsFollowing(followID int64) bool {
	return IsFollowing(u.ID, followID)
}

// GetFollowing returns range of user's following.
func (u *User) GetFollowing(listOptions ListOptions) ([]*User, error) {
	sess := x.
		Where("follow.user_id=?", u.ID).
		Join("LEFT", "follow", "`user`.id=follow.follow_id")

	if listOptions.Page != 0 {
		sess = listOptions.setSessionPagination(sess)

		users := make([]*User, 0, listOptions.PageSize)
		return users, sess.Find(&users)
	}

	users := make([]*User, 0, 8)
	return users, sess.Find(&users)
}

// NewGitSig generates and returns the signature of given user.
func (u *User) NewGitSig() *git.Signature {
	return &git.Signature{
		Name:  u.GitName(),
		Email: u.GetEmail(),
		When:  time.Now(),
	}
}

func hashPassword(passwd, salt, algo string) string {
	var tempPasswd []byte

	switch algo {
	case algoBcrypt:
		tempPasswd, _ = bcrypt.GenerateFromPassword([]byte(passwd), bcrypt.DefaultCost)
		return string(tempPasswd)
	case algoScrypt:
		tempPasswd, _ = scrypt.Key([]byte(passwd), []byte(salt), 65536, 16, 2, 50)
	case algoArgon2:
		tempPasswd = argon2.IDKey([]byte(passwd), []byte(salt), 2, 65536, 8, 50)
	case algoPbkdf2:
		fallthrough
	default:
		tempPasswd = pbkdf2.Key([]byte(passwd), []byte(salt), 10000, 50, sha256.New)
	}

	return fmt.Sprintf("%x", tempPasswd)
}

// SetPassword hashes a password using the algorithm defined in the config value of PASSWORD_HASH_ALGO
// change passwd, salt and passwd_hash_algo fields
func (u *User) SetPassword(passwd string) (err error) {
	if len(passwd) == 0 {
		u.Passwd = ""
		u.Salt = ""
		u.PasswdHashAlgo = ""
		return nil
	}

	if u.Salt, err = GetUserSalt(); err != nil {
		return err
	}
	u.PasswdHashAlgo = setting.PasswordHashAlgo
	u.Passwd = hashPassword(passwd, u.Salt, setting.PasswordHashAlgo)

	return nil
}

// ValidatePassword checks if given password matches the one belongs to the user.
func (u *User) ValidatePassword(passwd string) bool {
	tempHash := hashPassword(passwd, u.Salt, u.PasswdHashAlgo)

	if u.PasswdHashAlgo != algoBcrypt && subtle.ConstantTimeCompare([]byte(u.Passwd), []byte(tempHash)) == 1 {
		return true
	}
	if u.PasswdHashAlgo == algoBcrypt && bcrypt.CompareHashAndPassword([]byte(u.Passwd), []byte(passwd)) == nil {
		return true
	}
	return false
}

// IsPasswordSet checks if the password is set or left empty
func (u *User) IsPasswordSet() bool {
	return len(u.Passwd) != 0
}

// IsOrganization returns true if user is actually a organization.
func (u *User) IsOrganization() bool {
	return u.Type == UserTypeOrganization
}

// IsUserOrgOwner returns true if user is in the owner team of given organization.
func (u *User) IsUserOrgOwner(orgID int64) bool {
	isOwner, err := IsOrganizationOwner(orgID, u.ID)
	if err != nil {
		log.Error("IsOrganizationOwner: %v", err)
		return false
	}
	return isOwner
}

// HasMemberWithUserID returns true if user with userID is part of the u organisation.
func (u *User) HasMemberWithUserID(userID int64) bool {
	return u.hasMemberWithUserID(x, userID)
}

func (u *User) hasMemberWithUserID(e Engine, userID int64) bool {
	isMember, err := isOrganizationMember(e, u.ID, userID)
	if err != nil {
		log.Error("IsOrganizationMember: %v", err)
		return false
	}
	return isMember
}

// IsPublicMember returns true if user public his/her membership in given organization.
func (u *User) IsPublicMember(orgID int64) bool {
	isMember, err := IsPublicMembership(orgID, u.ID)
	if err != nil {
		log.Error("IsPublicMembership: %v", err)
		return false
	}
	return isMember
}

func (u *User) getOrganizationCount(e Engine) (int64, error) {
	return e.
		Where("uid=?", u.ID).
		Count(new(OrgUser))
}

// GetOrganizationCount returns count of membership of organization of user.
func (u *User) GetOrganizationCount() (int64, error) {
	return u.getOrganizationCount(x)
}

// GetRepositories returns repositories that user owns, including private repositories.
func (u *User) GetRepositories(listOpts ListOptions, names ...string) (err error) {
	u.Repos, _, err = GetUserRepositories(&SearchRepoOptions{Actor: u, Private: true, ListOptions: listOpts, LowerNames: names})
	return err
}

// GetRepositoryIDs returns repositories IDs where user owned and has unittypes
// Caller shall check that units is not globally disabled
func (u *User) GetRepositoryIDs(units ...UnitType) ([]int64, error) {
	var ids []int64

	sess := x.Table("repository").Cols("repository.id")

	if len(units) > 0 {
		sess = sess.Join("INNER", "repo_unit", "repository.id = repo_unit.repo_id")
		sess = sess.In("repo_unit.type", units)
	}

	return ids, sess.Where("owner_id = ?", u.ID).Find(&ids)
}

// GetActiveRepositoryIDs returns non-archived repositories IDs where user owned and has unittypes
// Caller shall check that units is not globally disabled
func (u *User) GetActiveRepositoryIDs(units ...UnitType) ([]int64, error) {
	var ids []int64

	sess := x.Table("repository").Cols("repository.id")

	if len(units) > 0 {
		sess = sess.Join("INNER", "repo_unit", "repository.id = repo_unit.repo_id")
		sess = sess.In("repo_unit.type", units)
	}

	sess.Where(builder.Eq{"is_archived": false})

	return ids, sess.Where("owner_id = ?", u.ID).GroupBy("repository.id").Find(&ids)
}

// GetOrgRepositoryIDs returns repositories IDs where user's team owned and has unittypes
// Caller shall check that units is not globally disabled
func (u *User) GetOrgRepositoryIDs(units ...UnitType) ([]int64, error) {
	var ids []int64

	if err := x.Table("repository").
		Cols("repository.id").
		Join("INNER", "team_user", "repository.owner_id = team_user.org_id").
		Join("INNER", "team_repo", "(? != ? and repository.is_private != ?) OR (team_user.team_id = team_repo.team_id AND repository.id = team_repo.repo_id)", true, u.IsRestricted, true).
		Where("team_user.uid = ?", u.ID).
		GroupBy("repository.id").Find(&ids); err != nil {
		return nil, err
	}

	if len(units) > 0 {
		return FilterOutRepoIdsWithoutUnitAccess(u, ids, units...)
	}

	return ids, nil
}

// GetActiveOrgRepositoryIDs returns non-archived repositories IDs where user's team owned and has unittypes
// Caller shall check that units is not globally disabled
func (u *User) GetActiveOrgRepositoryIDs(units ...UnitType) ([]int64, error) {
	var ids []int64

	if err := x.Table("repository").
		Cols("repository.id").
		Join("INNER", "team_user", "repository.owner_id = team_user.org_id").
		Join("INNER", "team_repo", "(? != ? and repository.is_private != ?) OR (team_user.team_id = team_repo.team_id AND repository.id = team_repo.repo_id)", true, u.IsRestricted, true).
		Where("team_user.uid = ?", u.ID).
		Where(builder.Eq{"is_archived": false}).
		GroupBy("repository.id").Find(&ids); err != nil {
		return nil, err
	}

	if len(units) > 0 {
		return FilterOutRepoIdsWithoutUnitAccess(u, ids, units...)
	}

	return ids, nil
}

// GetAccessRepoIDs returns all repositories IDs where user's or user is a team member organizations
// Caller shall check that units is not globally disabled
func (u *User) GetAccessRepoIDs(units ...UnitType) ([]int64, error) {
	ids, err := u.GetRepositoryIDs(units...)
	if err != nil {
		return nil, err
	}
	ids2, err := u.GetOrgRepositoryIDs(units...)
	if err != nil {
		return nil, err
	}
	return append(ids, ids2...), nil
}

// GetActiveAccessRepoIDs returns all non-archived repositories IDs where user's or user is a team member organizations
// Caller shall check that units is not globally disabled
func (u *User) GetActiveAccessRepoIDs(units ...UnitType) ([]int64, error) {
	ids, err := u.GetActiveRepositoryIDs(units...)
	if err != nil {
		return nil, err
	}
	ids2, err := u.GetActiveOrgRepositoryIDs(units...)
	if err != nil {
		return nil, err
	}
	return append(ids, ids2...), nil
}

// GetMirrorRepositories returns mirror repositories that user owns, including private repositories.
func (u *User) GetMirrorRepositories() ([]*Repository, error) {
	return GetUserMirrorRepositories(u.ID)
}

// GetOwnedOrganizations returns all organizations that user owns.
func (u *User) GetOwnedOrganizations() (err error) {
	u.OwnedOrgs, err = GetOwnedOrgsByUserID(u.ID)
	return err
}

// DisplayName returns full name if it's not empty,
// returns username otherwise.
func (u *User) DisplayName() string {
	trimmed := strings.TrimSpace(u.FullName)
	if len(trimmed) > 0 {
		return trimmed
	}
	return u.Name
}

// GetDisplayName returns full name if it's not empty and DEFAULT_SHOW_FULL_NAME is set,
// returns username otherwise.
func (u *User) GetDisplayName() string {
	if setting.UI.DefaultShowFullName {
		trimmed := strings.TrimSpace(u.FullName)
		if len(trimmed) > 0 {
			return trimmed
		}
	}
	return u.Name
}

func gitSafeName(name string) string {
	return strings.TrimSpace(strings.NewReplacer("\n", "", "<", "", ">", "").Replace(name))
}

// GitName returns a git safe name
func (u *User) GitName() string {
	gitName := gitSafeName(u.FullName)
	if len(gitName) > 0 {
		return gitName
	}
	// Although u.Name should be safe if created in our system
	// LDAP users may have bad names
	gitName = gitSafeName(u.Name)
	if len(gitName) > 0 {
		return gitName
	}
	// Totally pathological name so it's got to be:
	return fmt.Sprintf("user-%d", u.ID)
}

// ShortName ellipses username to length
func (u *User) ShortName(length int) string {
	return base.EllipsisString(u.Name, length)
}

// IsMailable checks if a user is eligible
// to receive emails.
func (u *User) IsMailable() bool {
	return u.IsActive
}

// EmailNotifications returns the User's email notification preference
func (u *User) EmailNotifications() string {
	return u.EmailNotificationsPreference
}

// SetEmailNotifications sets the user's email notification preference
func (u *User) SetEmailNotifications(set string) error {
	u.EmailNotificationsPreference = set
	if err := UpdateUserCols(u, "email_notifications_preference"); err != nil {
		log.Error("SetEmailNotifications: %v", err)
		return err
	}
	return nil
}

func isUserExist(e Engine, uid int64, name string) (bool, error) {
	if len(name) == 0 {
		return false, nil
	}
	return e.
		Where("id!=?", uid).
		Get(&User{LowerName: strings.ToLower(name)})
}

// IsUserExist checks if given user name exist,
// the user name should be noncased unique.
// If uid is presented, then check will rule out that one,
// it is used when update a user name in settings page.
func IsUserExist(uid int64, name string) (bool, error) {
	return isUserExist(x, uid, name)
}

// GetUserSalt returns a random user salt token.
func GetUserSalt() (string, error) {
	return util.RandomString(10)
}

// NewGhostUser creates and returns a fake user for someone has deleted his/her account.
func NewGhostUser() *User {
	return &User{
		ID:        -1,
		Name:      "Ghost",
		LowerName: "ghost",
	}
}

// NewReplaceUser creates and returns a fake user for external user
func NewReplaceUser(name string) *User {
	return &User{
		ID:        -1,
		Name:      name,
		LowerName: strings.ToLower(name),
	}
}

// IsGhost check if user is fake user for a deleted account
func (u *User) IsGhost() bool {
	if u == nil {
		return false
	}
	return u.ID == -1 && u.Name == "Ghost"
}

var (
	reservedUsernames = []string{
		".",
		"..",
		".well-known",
		"admin",
		"api",
		"assets",
		"attachments",
		"avatars",
		"captcha",
		"commits",
		"debug",
		"error",
		"explore",
		"favicon.ico",
		"ghost",
		"help",
		"install",
		"issues",
		"less",
		"login",
		"manifest.json",
		"metrics",
		"milestones",
		"new",
		"notifications",
		"org",
		"plugins",
		"pulls",
		"raw",
		"repo",
		"robots.txt",
		"search",
		"serviceworker.js",
		"stars",
		"template",
		"user",
	}

	reservedUserPatterns = []string{"*.keys", "*.gpg"}
)

// isUsableName checks if name is reserved or pattern of name is not allowed
// based on given reserved names and patterns.
// Names are exact match, patterns can be prefix or suffix match with placeholder '*'.
func isUsableName(names, patterns []string, name string) error {
	name = strings.TrimSpace(strings.ToLower(name))
	if utf8.RuneCountInString(name) == 0 {
		return ErrNameEmpty
	}

	for i := range names {
		if name == names[i] {
			return ErrNameReserved{name}
		}
	}

	for _, pat := range patterns {
		if pat[0] == '*' && strings.HasSuffix(name, pat[1:]) ||
			(pat[len(pat)-1] == '*' && strings.HasPrefix(name, pat[:len(pat)-1])) {
			return ErrNamePatternNotAllowed{pat}
		}
	}

	return nil
}

// IsUsableUsername returns an error when a username is reserved
func IsUsableUsername(name string) error {
	// Validate username make sure it satisfies requirement.
	if alphaDashDotPattern.MatchString(name) {
		// Note: usually this error is normally caught up earlier in the UI
		return ErrNameCharsNotAllowed{Name: name}
	}
	return isUsableName(reservedUsernames, reservedUserPatterns, name)
}

// CreateUser creates record of a new user.
func CreateUser(u *User) (err error) {
	if err = IsUsableUsername(u.Name); err != nil {
		return err
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	isExist, err := isUserExist(sess, 0, u.Name)
	if err != nil {
		return err
	} else if isExist {
		return ErrUserAlreadyExist{u.Name}
	}

	if err = deleteUserRedirect(sess, u.Name); err != nil {
		return err
	}

	u.Email = strings.ToLower(u.Email)
	if err = ValidateEmail(u.Email); err != nil {
		return err
	}

	isExist, err = isEmailUsed(sess, u.Email)
	if err != nil {
		return err
	} else if isExist {
		return ErrEmailAlreadyUsed{u.Email}
	}

	u.KeepEmailPrivate = setting.Service.DefaultKeepEmailPrivate

	u.LowerName = strings.ToLower(u.Name)
	u.AvatarEmail = u.Email
	if u.Rands, err = GetUserSalt(); err != nil {
		return err
	}
	if err = u.SetPassword(u.Passwd); err != nil {
		return err
	}
	u.AllowCreateOrganization = setting.Service.DefaultAllowCreateOrganization && !setting.Admin.DisableRegularOrgCreation
	u.EmailNotificationsPreference = setting.Admin.DefaultEmailNotification
	u.MaxRepoCreation = -1
	u.Theme = setting.UI.DefaultTheme

	if _, err = sess.Insert(u); err != nil {
		return err
	}

	// insert email address
	if _, err := sess.Insert(&EmailAddress{
		UID:         u.ID,
		Email:       u.Email,
		LowerEmail:  strings.ToLower(u.Email),
		IsActivated: u.IsActive,
		IsPrimary:   true,
	}); err != nil {
		return err
	}

	return sess.Commit()
}

func countUsers(e Engine) int64 {
	count, _ := e.
		Where("type=0").
		Count(new(User))
	return count
}

// CountUsers returns number of users.
func CountUsers() int64 {
	return countUsers(x)
}

// get user by verify code
func getVerifyUser(code string) (user *User) {
	if len(code) <= base.TimeLimitCodeLength {
		return nil
	}

	// use tail hex username query user
	hexStr := code[base.TimeLimitCodeLength:]
	if b, err := hex.DecodeString(hexStr); err == nil {
		if user, err = GetUserByName(string(b)); user != nil {
			return user
		}
		log.Error("user.getVerifyUser: %v", err)
	}

	return nil
}

// VerifyUserActiveCode verifies active code when active account
func VerifyUserActiveCode(code string) (user *User) {
	minutes := setting.Service.ActiveCodeLives

	if user = getVerifyUser(code); user != nil {
		// time limit code
		prefix := code[:base.TimeLimitCodeLength]
		data := fmt.Sprintf("%d%s%s%s%s", user.ID, user.Email, user.LowerName, user.Passwd, user.Rands)

		if base.VerifyTimeLimitCode(data, minutes, prefix) {
			return user
		}
	}
	return nil
}

// VerifyActiveEmailCode verifies active email code when active account
func VerifyActiveEmailCode(code, email string) *EmailAddress {
	minutes := setting.Service.ActiveCodeLives

	if user := getVerifyUser(code); user != nil {
		// time limit code
		prefix := code[:base.TimeLimitCodeLength]
		data := fmt.Sprintf("%d%s%s%s%s", user.ID, email, user.LowerName, user.Passwd, user.Rands)

		if base.VerifyTimeLimitCode(data, minutes, prefix) {
			emailAddress := &EmailAddress{UID: user.ID, Email: email}
			if has, _ := x.Get(emailAddress); has {
				return emailAddress
			}
		}
	}
	return nil
}

// ChangeUserName changes all corresponding setting from old user name to new one.
func ChangeUserName(u *User, newUserName string) (err error) {
	oldUserName := u.Name
	if err = IsUsableUsername(newUserName); err != nil {
		return err
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	isExist, err := isUserExist(sess, 0, newUserName)
	if err != nil {
		return err
	} else if isExist {
		return ErrUserAlreadyExist{newUserName}
	}

	if _, err = sess.Exec("UPDATE `repository` SET owner_name=? WHERE owner_name=?", newUserName, oldUserName); err != nil {
		return fmt.Errorf("Change repo owner name: %v", err)
	}

	// Do not fail if directory does not exist
	if err = os.Rename(UserPath(oldUserName), UserPath(newUserName)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("Rename user directory: %v", err)
	}

	if err = newUserRedirect(sess, u.ID, oldUserName, newUserName); err != nil {
		return err
	}

	if err = sess.Commit(); err != nil {
		if err2 := os.Rename(UserPath(newUserName), UserPath(oldUserName)); err2 != nil && !os.IsNotExist(err2) {
			log.Critical("Unable to rollback directory change during failed username change from: %s to: %s. DB Error: %v. Filesystem Error: %v", oldUserName, newUserName, err, err2)
			return fmt.Errorf("failed to rollback directory change during failed username change from: %s to: %s. DB Error: %w. Filesystem Error: %v", oldUserName, newUserName, err, err2)
		}
		return err
	}

	return nil
}

// checkDupEmail checks whether there are the same email with the user
func checkDupEmail(e Engine, u *User) error {
	u.Email = strings.ToLower(u.Email)
	has, err := e.
		Where("id!=?", u.ID).
		And("type=?", u.Type).
		And("email=?", u.Email).
		Get(new(User))
	if err != nil {
		return err
	} else if has {
		return ErrEmailAlreadyUsed{u.Email}
	}
	return nil
}

func updateUser(e Engine, u *User) (err error) {
	u.Email = strings.ToLower(u.Email)
	if err = ValidateEmail(u.Email); err != nil {
		return err
	}
	_, err = e.ID(u.ID).AllCols().Update(u)
	return err
}

// UpdateUser updates user's information.
func UpdateUser(u *User) error {
	return updateUser(x, u)
}

// UpdateUserCols update user according special columns
func UpdateUserCols(u *User, cols ...string) error {
	return updateUserCols(x, u, cols...)
}

func updateUserCols(e Engine, u *User, cols ...string) error {
	_, err := e.ID(u.ID).Cols(cols...).Update(u)
	return err
}

// UpdateUserSetting updates user's settings.
func UpdateUserSetting(u *User) (err error) {
	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}
	if !u.IsOrganization() {
		if err = checkDupEmail(sess, u); err != nil {
			return err
		}
	}
	if err = updateUser(sess, u); err != nil {
		return err
	}
	return sess.Commit()
}

// deleteBeans deletes all given beans, beans should contain delete conditions.
func deleteBeans(e Engine, beans ...interface{}) (err error) {
	for i := range beans {
		if _, err = e.Delete(beans[i]); err != nil {
			return err
		}
	}
	return nil
}

func deleteUser(e Engine, u *User) error {
	// Note: A user owns any repository or belongs to any organization
	//	cannot perform delete operation.

	// Check ownership of repository.
	count, err := getRepositoryCount(e, u)
	if err != nil {
		return fmt.Errorf("GetRepositoryCount: %v", err)
	} else if count > 0 {
		return ErrUserOwnRepos{UID: u.ID}
	}

	// Check membership of organization.
	count, err = u.getOrganizationCount(e)
	if err != nil {
		return fmt.Errorf("GetOrganizationCount: %v", err)
	} else if count > 0 {
		return ErrUserHasOrgs{UID: u.ID}
	}

	// ***** START: Watch *****
	watchedRepoIDs := make([]int64, 0, 10)
	if err = e.Table("watch").Cols("watch.repo_id").
		Where("watch.user_id = ?", u.ID).And("watch.mode <>?", RepoWatchModeDont).Find(&watchedRepoIDs); err != nil {
		return fmt.Errorf("get all watches: %v", err)
	}
	if _, err = e.Decr("num_watches").In("id", watchedRepoIDs).NoAutoTime().Update(new(Repository)); err != nil {
		return fmt.Errorf("decrease repository num_watches: %v", err)
	}
	// ***** END: Watch *****

	// ***** START: Star *****
	starredRepoIDs := make([]int64, 0, 10)
	if err = e.Table("star").Cols("star.repo_id").
		Where("star.uid = ?", u.ID).Find(&starredRepoIDs); err != nil {
		return fmt.Errorf("get all stars: %v", err)
	} else if _, err = e.Decr("num_stars").In("id", starredRepoIDs).NoAutoTime().Update(new(Repository)); err != nil {
		return fmt.Errorf("decrease repository num_stars: %v", err)
	}
	// ***** END: Star *****

	// ***** START: Follow *****
	followeeIDs := make([]int64, 0, 10)
	if err = e.Table("follow").Cols("follow.follow_id").
		Where("follow.user_id = ?", u.ID).Find(&followeeIDs); err != nil {
		return fmt.Errorf("get all followees: %v", err)
	} else if _, err = e.Decr("num_followers").In("id", followeeIDs).Update(new(User)); err != nil {
		return fmt.Errorf("decrease user num_followers: %v", err)
	}

	followerIDs := make([]int64, 0, 10)
	if err = e.Table("follow").Cols("follow.user_id").
		Where("follow.follow_id = ?", u.ID).Find(&followerIDs); err != nil {
		return fmt.Errorf("get all followers: %v", err)
	} else if _, err = e.Decr("num_following").In("id", followerIDs).Update(new(User)); err != nil {
		return fmt.Errorf("decrease user num_following: %v", err)
	}
	// ***** END: Follow *****

	if err = deleteBeans(e,
		&AccessToken{UID: u.ID},
		&Collaboration{UserID: u.ID},
		&Access{UserID: u.ID},
		&Watch{UserID: u.ID},
		&Star{UID: u.ID},
		&Follow{UserID: u.ID},
		&Follow{FollowID: u.ID},
		&Action{UserID: u.ID},
		&IssueUser{UID: u.ID},
		&EmailAddress{UID: u.ID},
		&UserOpenID{UID: u.ID},
		&Reaction{UserID: u.ID},
		&TeamUser{UID: u.ID},
		&Collaboration{UserID: u.ID},
		&Stopwatch{UserID: u.ID},
	); err != nil {
		return fmt.Errorf("deleteBeans: %v", err)
	}

	if setting.Service.UserDeleteWithCommentsMaxTime != 0 &&
		u.CreatedUnix.AsTime().Add(setting.Service.UserDeleteWithCommentsMaxTime).After(time.Now()) {

		// Delete Comments
		const batchSize = 50
		for start := 0; ; start += batchSize {
			comments := make([]*Comment, 0, batchSize)
			if err = e.Where("type=? AND poster_id=?", CommentTypeComment, u.ID).Limit(batchSize, start).Find(&comments); err != nil {
				return err
			}
			if len(comments) == 0 {
				break
			}

			for _, comment := range comments {
				if err = deleteComment(e, comment); err != nil {
					return err
				}
			}
		}

		// Delete Reactions
		if err = deleteReaction(e, &ReactionOptions{Doer: u}); err != nil {
			return err
		}
	}

	// ***** START: PublicKey *****
	if _, err = e.Delete(&PublicKey{OwnerID: u.ID}); err != nil {
		return fmt.Errorf("deletePublicKeys: %v", err)
	}
	err = rewriteAllPublicKeys(e)
	if err != nil {
		return err
	}
	err = rewriteAllPrincipalKeys(e)
	if err != nil {
		return err
	}
	// ***** END: PublicKey *****

	// ***** START: GPGPublicKey *****
	keys, err := listGPGKeys(e, u.ID, ListOptions{})
	if err != nil {
		return fmt.Errorf("ListGPGKeys: %v", err)
	}
	// Delete GPGKeyImport(s).
	for _, key := range keys {
		if _, err = e.Delete(&GPGKeyImport{KeyID: key.KeyID}); err != nil {
			return fmt.Errorf("deleteGPGKeyImports: %v", err)
		}
	}
	if _, err = e.Delete(&GPGKey{OwnerID: u.ID}); err != nil {
		return fmt.Errorf("deleteGPGKeys: %v", err)
	}
	// ***** END: GPGPublicKey *****

	// Clear assignee.
	if err = clearAssigneeByUserID(e, u.ID); err != nil {
		return fmt.Errorf("clear assignee: %v", err)
	}

	// ***** START: ExternalLoginUser *****
	if err = removeAllAccountLinks(e, u); err != nil {
		return fmt.Errorf("ExternalLoginUser: %v", err)
	}
	// ***** END: ExternalLoginUser *****

	if _, err = e.ID(u.ID).Delete(new(User)); err != nil {
		return fmt.Errorf("Delete: %v", err)
	}

	// Note: There are something just cannot be roll back,
	//	so just keep error logs of those operations.
	path := UserPath(u.Name)
	if err = util.RemoveAll(path); err != nil {
		err = fmt.Errorf("Failed to RemoveAll %s: %v", path, err)
		_ = createNotice(e, NoticeTask, fmt.Sprintf("delete user '%s': %v", u.Name, err))
		return err
	}

	if len(u.Avatar) > 0 {
		avatarPath := u.CustomAvatarRelativePath()
		if err = storage.Avatars.Delete(avatarPath); err != nil {
			err = fmt.Errorf("Failed to remove %s: %v", avatarPath, err)
			_ = createNotice(e, NoticeTask, fmt.Sprintf("delete user '%s': %v", u.Name, err))
			return err
		}
	}

	return nil
}

// DeleteUser completely and permanently deletes everything of a user,
// but issues/comments/pulls will be kept and shown as someone has been deleted,
// unless the user is younger than USER_DELETE_WITH_COMMENTS_MAX_DAYS.
func DeleteUser(u *User) (err error) {
	if u.IsOrganization() {
		return fmt.Errorf("%s is an organization not a user", u.Name)
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if err = deleteUser(sess, u); err != nil {
		// Note: don't wrapper error here.
		return err
	}

	return sess.Commit()
}

// DeleteInactiveUsers deletes all inactive users and email addresses.
func DeleteInactiveUsers(ctx context.Context, olderThan time.Duration) (err error) {
	users := make([]*User, 0, 10)
	if olderThan > 0 {
		if err = x.
			Where("is_active = ? and created_unix < ?", false, time.Now().Add(-olderThan).Unix()).
			Find(&users); err != nil {
			return fmt.Errorf("get all inactive users: %v", err)
		}
	} else {
		if err = x.
			Where("is_active = ?", false).
			Find(&users); err != nil {
			return fmt.Errorf("get all inactive users: %v", err)
		}
	}
	// FIXME: should only update authorized_keys file once after all deletions.
	for _, u := range users {
		select {
		case <-ctx.Done():
			return ErrCancelledf("Before delete inactive user %s", u.Name)
		default:
		}
		if err = DeleteUser(u); err != nil {
			// Ignore users that were set inactive by admin.
			if IsErrUserOwnRepos(err) || IsErrUserHasOrgs(err) {
				continue
			}
			return err
		}
	}

	_, err = x.
		Where("is_activated = ?", false).
		Delete(new(EmailAddress))
	return err
}

// UserPath returns the path absolute path of user repositories.
func UserPath(userName string) string {
	return filepath.Join(setting.RepoRootPath, strings.ToLower(userName))
}

func getUserByID(e Engine, id int64) (*User, error) {
	u := new(User)
	has, err := e.ID(id).Get(u)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrUserNotExist{id, "", 0}
	}
	return u, nil
}

// GetUserByID returns the user object by given ID if exists.
func GetUserByID(id int64) (*User, error) {
	return getUserByID(x, id)
}

// GetUserByName returns user by given name.
func GetUserByName(name string) (*User, error) {
	return getUserByName(x, name)
}

func getUserByName(e Engine, name string) (*User, error) {
	if len(name) == 0 {
		return nil, ErrUserNotExist{0, name, 0}
	}
	u := &User{LowerName: strings.ToLower(name)}
	has, err := e.Get(u)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrUserNotExist{0, name, 0}
	}
	return u, nil
}

// GetUserEmailsByNames returns a list of e-mails corresponds to names of users
// that have their email notifications set to enabled or onmention.
func GetUserEmailsByNames(names []string) []string {
	return getUserEmailsByNames(x, names)
}

func getUserEmailsByNames(e Engine, names []string) []string {
	mails := make([]string, 0, len(names))
	for _, name := range names {
		u, err := getUserByName(e, name)
		if err != nil {
			continue
		}
		if u.IsMailable() && u.EmailNotifications() != EmailNotificationsDisabled {
			mails = append(mails, u.Email)
		}
	}
	return mails
}

// GetMaileableUsersByIDs gets users from ids, but only if they can receive mails
func GetMaileableUsersByIDs(ids []int64, isMention bool) ([]*User, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	ous := make([]*User, 0, len(ids))

	if isMention {
		return ous, x.In("id", ids).
			Where("`type` = ?", UserTypeIndividual).
			And("`prohibit_login` = ?", false).
			And("`is_active` = ?", true).
			And("`email_notifications_preference` IN ( ?, ?)", EmailNotificationsEnabled, EmailNotificationsOnMention).
			Find(&ous)
	}

	return ous, x.In("id", ids).
		Where("`type` = ?", UserTypeIndividual).
		And("`prohibit_login` = ?", false).
		And("`is_active` = ?", true).
		And("`email_notifications_preference` = ?", EmailNotificationsEnabled).
		Find(&ous)
}

// GetUserNamesByIDs returns usernames for all resolved users from a list of Ids.
func GetUserNamesByIDs(ids []int64) ([]string, error) {
	unames := make([]string, 0, len(ids))
	err := x.In("id", ids).
		Table("user").
		Asc("name").
		Cols("name").
		Find(&unames)
	return unames, err
}

// GetUsersByIDs returns all resolved users from a list of Ids.
func GetUsersByIDs(ids []int64) (UserList, error) {
	ous := make([]*User, 0, len(ids))
	if len(ids) == 0 {
		return ous, nil
	}
	err := x.In("id", ids).
		Asc("name").
		Find(&ous)
	return ous, err
}

// GetUserIDsByNames returns a slice of ids corresponds to names.
func GetUserIDsByNames(names []string, ignoreNonExistent bool) ([]int64, error) {
	ids := make([]int64, 0, len(names))
	for _, name := range names {
		u, err := GetUserByName(name)
		if err != nil {
			if ignoreNonExistent {
				continue
			} else {
				return nil, err
			}
		}
		ids = append(ids, u.ID)
	}
	return ids, nil
}

// UserCommit represents a commit with validation of user.
type UserCommit struct {
	User *User
	*git.Commit
}

// ValidateCommitWithEmail check if author's e-mail of commit is corresponding to a user.
func ValidateCommitWithEmail(c *git.Commit) *User {
	if c.Author == nil {
		return nil
	}
	u, err := GetUserByEmail(c.Author.Email)
	if err != nil {
		return nil
	}
	return u
}

// ValidateCommitsWithEmails checks if authors' e-mails of commits are corresponding to users.
func ValidateCommitsWithEmails(oldCommits *list.List) *list.List {
	var (
		u          *User
		emails     = map[string]*User{}
		newCommits = list.New()
		e          = oldCommits.Front()
	)
	for e != nil {
		c := e.Value.(*git.Commit)

		if c.Author != nil {
			if v, ok := emails[c.Author.Email]; !ok {
				u, _ = GetUserByEmail(c.Author.Email)
				emails[c.Author.Email] = u
			} else {
				u = v
			}
		} else {
			u = nil
		}

		newCommits.PushBack(UserCommit{
			User:   u,
			Commit: c,
		})
		e = e.Next()
	}
	return newCommits
}

// GetUserByEmail returns the user object by given e-mail if exists.
func GetUserByEmail(email string) (*User, error) {
	return GetUserByEmailContext(DefaultDBContext(), email)
}

// GetUserByEmailContext returns the user object by given e-mail if exists with db context
func GetUserByEmailContext(ctx DBContext, email string) (*User, error) {
	if len(email) == 0 {
		return nil, ErrUserNotExist{0, email, 0}
	}

	email = strings.ToLower(email)
	// First try to find the user by primary email
	user := &User{Email: email}
	has, err := ctx.e.Get(user)
	if err != nil {
		return nil, err
	}
	if has {
		return user, nil
	}

	// Otherwise, check in alternative list for activated email addresses
	emailAddress := &EmailAddress{Email: email, IsActivated: true}
	has, err = ctx.e.Get(emailAddress)
	if err != nil {
		return nil, err
	}
	if has {
		return getUserByID(ctx.e, emailAddress.UID)
	}

	// Finally, if email address is the protected email address:
	if strings.HasSuffix(email, fmt.Sprintf("@%s", setting.Service.NoReplyAddress)) {
		username := strings.TrimSuffix(email, fmt.Sprintf("@%s", setting.Service.NoReplyAddress))
		user := &User{}
		has, err := ctx.e.Where("lower_name=?", username).Get(user)
		if err != nil {
			return nil, err
		}
		if has {
			return user, nil
		}
	}

	return nil, ErrUserNotExist{0, email, 0}
}

// GetUser checks if a user already exists
func GetUser(user *User) (bool, error) {
	return x.Get(user)
}

// SearchUserOptions contains the options for searching
type SearchUserOptions struct {
	ListOptions
	Keyword       string
	Type          UserType
	UID           int64
	OrderBy       SearchOrderBy
	Visible       []structs.VisibleType
	Actor         *User // The user doing the search
	IsActive      util.OptionalBool
	SearchByEmail bool // Search by email as well as username/full name
}

func (opts *SearchUserOptions) toConds() builder.Cond {
	var cond builder.Cond = builder.Eq{"type": opts.Type}
	if len(opts.Keyword) > 0 {
		lowerKeyword := strings.ToLower(opts.Keyword)
		keywordCond := builder.Or(
			builder.Like{"lower_name", lowerKeyword},
			builder.Like{"LOWER(full_name)", lowerKeyword},
		)
		if opts.SearchByEmail {
			keywordCond = keywordCond.Or(builder.Like{"LOWER(email)", lowerKeyword})
		}

		cond = cond.And(keywordCond)
	}

	if len(opts.Visible) > 0 {
		cond = cond.And(builder.In("visibility", opts.Visible))
	} else {
		cond = cond.And(builder.In("visibility", structs.VisibleTypePublic))
	}

	if opts.Actor != nil {
		var exprCond builder.Cond
		if setting.Database.UseMySQL {
			exprCond = builder.Expr("org_user.org_id = user.id")
		} else if setting.Database.UseMSSQL {
			exprCond = builder.Expr("org_user.org_id = [user].id")
		} else {
			exprCond = builder.Expr("org_user.org_id = \"user\".id")
		}

		var accessCond builder.Cond
		if !opts.Actor.IsRestricted {
			accessCond = builder.Or(
				builder.In("id", builder.Select("org_id").From("org_user").LeftJoin("`user`", exprCond).Where(builder.And(builder.Eq{"uid": opts.Actor.ID}, builder.Eq{"visibility": structs.VisibleTypePrivate}))),
				builder.In("visibility", structs.VisibleTypePublic, structs.VisibleTypeLimited))
		} else {
			// restricted users only see orgs they are a member of
			accessCond = builder.In("id", builder.Select("org_id").From("org_user").LeftJoin("`user`", exprCond).Where(builder.And(builder.Eq{"uid": opts.Actor.ID})))
		}
		cond = cond.And(accessCond)
	}

	if opts.UID > 0 {
		cond = cond.And(builder.Eq{"id": opts.UID})
	}

	if !opts.IsActive.IsNone() {
		cond = cond.And(builder.Eq{"is_active": opts.IsActive.IsTrue()})
	}

	return cond
}

// SearchUsers takes options i.e. keyword and part of user name to search,
// it returns results in given range and number of total results.
func SearchUsers(opts *SearchUserOptions) (users []*User, _ int64, _ error) {
	cond := opts.toConds()
	count, err := x.Where(cond).Count(new(User))
	if err != nil {
		return nil, 0, fmt.Errorf("Count: %v", err)
	}

	if len(opts.OrderBy) == 0 {
		opts.OrderBy = SearchOrderByAlphabetically
	}

	sess := x.Where(cond).OrderBy(opts.OrderBy.String())
	if opts.Page != 0 {
		sess = opts.setSessionPagination(sess)
	}

	users = make([]*User, 0, opts.PageSize)
	return users, count, sess.Find(&users)
}

// GetStarredRepos returns the repos starred by a particular user
func GetStarredRepos(userID int64, private bool, listOptions ListOptions) ([]*Repository, error) {
	sess := x.Where("star.uid=?", userID).
		Join("LEFT", "star", "`repository`.id=`star`.repo_id")
	if !private {
		sess = sess.And("is_private=?", false)
	}

	if listOptions.Page != 0 {
		sess = listOptions.setSessionPagination(sess)

		repos := make([]*Repository, 0, listOptions.PageSize)
		return repos, sess.Find(&repos)
	}

	repos := make([]*Repository, 0, 10)
	return repos, sess.Find(&repos)
}

// GetWatchedRepos returns the repos watched by a particular user
func GetWatchedRepos(userID int64, private bool, listOptions ListOptions) ([]*Repository, error) {
	sess := x.Where("watch.user_id=?", userID).
		And("`watch`.mode<>?", RepoWatchModeDont).
		Join("LEFT", "watch", "`repository`.id=`watch`.repo_id")
	if !private {
		sess = sess.And("is_private=?", false)
	}

	if listOptions.Page != 0 {
		sess = listOptions.setSessionPagination(sess)

		repos := make([]*Repository, 0, listOptions.PageSize)
		return repos, sess.Find(&repos)
	}

	repos := make([]*Repository, 0, 10)
	return repos, sess.Find(&repos)
}

// deleteKeysMarkedForDeletion returns true if ssh keys needs update
func deleteKeysMarkedForDeletion(keys []string) (bool, error) {
	// Start session
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return false, err
	}

	// Delete keys marked for deletion
	var sshKeysNeedUpdate bool
	for _, KeyToDelete := range keys {
		key, err := searchPublicKeyByContentWithEngine(sess, KeyToDelete)
		if err != nil {
			log.Error("SearchPublicKeyByContent: %v", err)
			continue
		}
		if err = deletePublicKeys(sess, key.ID); err != nil {
			log.Error("deletePublicKeys: %v", err)
			continue
		}
		sshKeysNeedUpdate = true
	}

	if err := sess.Commit(); err != nil {
		return false, err
	}

	return sshKeysNeedUpdate, nil
}

// addLdapSSHPublicKeys add a users public keys. Returns true if there are changes.
func addLdapSSHPublicKeys(usr *User, s *LoginSource, sshPublicKeys []string) bool {
	var sshKeysNeedUpdate bool
	for _, sshKey := range sshPublicKeys {
		var err error
		found := false
		keys := []byte(sshKey)
	loop:
		for len(keys) > 0 && err == nil {
			var out ssh.PublicKey
			// We ignore options as they are not relevant to Gitea
			out, _, _, keys, err = ssh.ParseAuthorizedKey(keys)
			if err != nil {
				break loop
			}
			found = true
			marshalled := string(ssh.MarshalAuthorizedKey(out))
			marshalled = marshalled[:len(marshalled)-1]
			sshKeyName := fmt.Sprintf("%s-%s", s.Name, ssh.FingerprintSHA256(out))

			if _, err := AddPublicKey(usr.ID, sshKeyName, marshalled, s.ID); err != nil {
				if IsErrKeyAlreadyExist(err) {
					log.Trace("addLdapSSHPublicKeys[%s]: LDAP Public SSH Key %s already exists for user", sshKeyName, usr.Name)
				} else {
					log.Error("addLdapSSHPublicKeys[%s]: Error adding LDAP Public SSH Key for user %s: %v", sshKeyName, usr.Name, err)
				}
			} else {
				log.Trace("addLdapSSHPublicKeys[%s]: Added LDAP Public SSH Key for user %s", sshKeyName, usr.Name)
				sshKeysNeedUpdate = true
			}
		}
		if !found && err != nil {
			log.Warn("addLdapSSHPublicKeys[%s]: Skipping invalid LDAP Public SSH Key for user %s: %v", s.Name, usr.Name, sshKey)
		}
	}
	return sshKeysNeedUpdate
}

// synchronizeLdapSSHPublicKeys updates a users public keys. Returns true if there are changes.
func synchronizeLdapSSHPublicKeys(usr *User, s *LoginSource, sshPublicKeys []string) bool {
	var sshKeysNeedUpdate bool

	log.Trace("synchronizeLdapSSHPublicKeys[%s]: Handling LDAP Public SSH Key synchronization for user %s", s.Name, usr.Name)

	// Get Public Keys from DB with current LDAP source
	var giteaKeys []string
	keys, err := ListPublicLdapSSHKeys(usr.ID, s.ID)
	if err != nil {
		log.Error("synchronizeLdapSSHPublicKeys[%s]: Error listing LDAP Public SSH Keys for user %s: %v", s.Name, usr.Name, err)
	}

	for _, v := range keys {
		giteaKeys = append(giteaKeys, v.OmitEmail())
	}

	// Get Public Keys from LDAP and skip duplicate keys
	var ldapKeys []string
	for _, v := range sshPublicKeys {
		sshKeySplit := strings.Split(v, " ")
		if len(sshKeySplit) > 1 {
			ldapKey := strings.Join(sshKeySplit[:2], " ")
			if !util.ExistsInSlice(ldapKey, ldapKeys) {
				ldapKeys = append(ldapKeys, ldapKey)
			}
		}
	}

	// Check if Public Key sync is needed
	if util.IsEqualSlice(giteaKeys, ldapKeys) {
		log.Trace("synchronizeLdapSSHPublicKeys[%s]: LDAP Public Keys are already in sync for %s (LDAP:%v/DB:%v)", s.Name, usr.Name, len(ldapKeys), len(giteaKeys))
		return false
	}
	log.Trace("synchronizeLdapSSHPublicKeys[%s]: LDAP Public Key needs update for user %s (LDAP:%v/DB:%v)", s.Name, usr.Name, len(ldapKeys), len(giteaKeys))

	// Add LDAP Public SSH Keys that doesn't already exist in DB
	var newLdapSSHKeys []string
	for _, LDAPPublicSSHKey := range ldapKeys {
		if !util.ExistsInSlice(LDAPPublicSSHKey, giteaKeys) {
			newLdapSSHKeys = append(newLdapSSHKeys, LDAPPublicSSHKey)
		}
	}
	if addLdapSSHPublicKeys(usr, s, newLdapSSHKeys) {
		sshKeysNeedUpdate = true
	}

	// Mark LDAP keys from DB that doesn't exist in LDAP for deletion
	var giteaKeysToDelete []string
	for _, giteaKey := range giteaKeys {
		if !util.ExistsInSlice(giteaKey, ldapKeys) {
			log.Trace("synchronizeLdapSSHPublicKeys[%s]: Marking LDAP Public SSH Key for deletion for user %s: %v", s.Name, usr.Name, giteaKey)
			giteaKeysToDelete = append(giteaKeysToDelete, giteaKey)
		}
	}

	// Delete LDAP keys from DB that doesn't exist in LDAP
	needUpd, err := deleteKeysMarkedForDeletion(giteaKeysToDelete)
	if err != nil {
		log.Error("synchronizeLdapSSHPublicKeys[%s]: Error deleting LDAP Public SSH Keys marked for deletion for user %s: %v", s.Name, usr.Name, err)
	}
	if needUpd {
		sshKeysNeedUpdate = true
	}

	return sshKeysNeedUpdate
}

// SyncExternalUsers is used to synchronize users with external authorization source
func SyncExternalUsers(ctx context.Context, updateExisting bool) error {
	log.Trace("Doing: SyncExternalUsers")

	ls, err := LoginSources()
	if err != nil {
		log.Error("SyncExternalUsers: %v", err)
		return err
	}

	for _, s := range ls {
		if !s.IsActived || !s.IsSyncEnabled {
			continue
		}
		select {
		case <-ctx.Done():
			log.Warn("SyncExternalUsers: Cancelled before update of %s", s.Name)
			return ErrCancelledf("Before update of %s", s.Name)
		default:
		}

		if s.IsLDAP() {
			log.Trace("Doing: SyncExternalUsers[%s]", s.Name)

			var existingUsers []int64
			isAttributeSSHPublicKeySet := len(strings.TrimSpace(s.LDAP().AttributeSSHPublicKey)) > 0
			var sshKeysNeedUpdate bool

			// Find all users with this login type
			var users []*User
			err = x.Where("login_type = ?", LoginLDAP).
				And("login_source = ?", s.ID).
				Find(&users)
			if err != nil {
				log.Error("SyncExternalUsers: %v", err)
				return err
			}
			select {
			case <-ctx.Done():
				log.Warn("SyncExternalUsers: Cancelled before update of %s", s.Name)
				return ErrCancelledf("Before update of %s", s.Name)
			default:
			}

			sr, err := s.LDAP().SearchEntries()
			if err != nil {
				log.Error("SyncExternalUsers LDAP source failure [%s], skipped", s.Name)
				continue
			}

			if len(sr) == 0 {
				if !s.LDAP().AllowDeactivateAll {
					log.Error("LDAP search found no entries but did not report an error. Refusing to deactivate all users")
					continue
				} else {
					log.Warn("LDAP search found no entries but did not report an error. All users will be deactivated as per settings")
				}
			}

			for _, su := range sr {
				select {
				case <-ctx.Done():
					log.Warn("SyncExternalUsers: Cancelled at update of %s before completed update of users", s.Name)
					// Rewrite authorized_keys file if LDAP Public SSH Key attribute is set and any key was added or removed
					if sshKeysNeedUpdate {
						err = RewriteAllPublicKeys()
						if err != nil {
							log.Error("RewriteAllPublicKeys: %v", err)
						}
					}
					return ErrCancelledf("During update of %s before completed update of users", s.Name)
				default:
				}
				if len(su.Username) == 0 {
					continue
				}

				if len(su.Mail) == 0 {
					su.Mail = fmt.Sprintf("%s@localhost", su.Username)
				}

				var usr *User
				// Search for existing user
				for _, du := range users {
					if du.LowerName == strings.ToLower(su.Username) {
						usr = du
						break
					}
				}

				fullName := composeFullName(su.Name, su.Surname, su.Username)
				// If no existing user found, create one
				if usr == nil {
					log.Trace("SyncExternalUsers[%s]: Creating user %s", s.Name, su.Username)

					usr = &User{
						LowerName:    strings.ToLower(su.Username),
						Name:         su.Username,
						FullName:     fullName,
						LoginType:    s.Type,
						LoginSource:  s.ID,
						LoginName:    su.Username,
						Email:        su.Mail,
						IsAdmin:      su.IsAdmin,
						IsRestricted: su.IsRestricted,
						IsActive:     true,
					}

					err = CreateUser(usr)

					if err != nil {
						log.Error("SyncExternalUsers[%s]: Error creating user %s: %v", s.Name, su.Username, err)
					} else if isAttributeSSHPublicKeySet {
						log.Trace("SyncExternalUsers[%s]: Adding LDAP Public SSH Keys for user %s", s.Name, usr.Name)
						if addLdapSSHPublicKeys(usr, s, su.SSHPublicKey) {
							sshKeysNeedUpdate = true
						}
					}
				} else if updateExisting {
					existingUsers = append(existingUsers, usr.ID)

					// Synchronize SSH Public Key if that attribute is set
					if isAttributeSSHPublicKeySet && synchronizeLdapSSHPublicKeys(usr, s, su.SSHPublicKey) {
						sshKeysNeedUpdate = true
					}

					// Check if user data has changed
					if (len(s.LDAP().AdminFilter) > 0 && usr.IsAdmin != su.IsAdmin) ||
						(len(s.LDAP().RestrictedFilter) > 0 && usr.IsRestricted != su.IsRestricted) ||
						!strings.EqualFold(usr.Email, su.Mail) ||
						usr.FullName != fullName ||
						!usr.IsActive {

						log.Trace("SyncExternalUsers[%s]: Updating user %s", s.Name, usr.Name)

						usr.FullName = fullName
						usr.Email = su.Mail
						// Change existing admin flag only if AdminFilter option is set
						if len(s.LDAP().AdminFilter) > 0 {
							usr.IsAdmin = su.IsAdmin
						}
						// Change existing restricted flag only if RestrictedFilter option is set
						if !usr.IsAdmin && len(s.LDAP().RestrictedFilter) > 0 {
							usr.IsRestricted = su.IsRestricted
						}
						usr.IsActive = true

						err = UpdateUserCols(usr, "full_name", "email", "is_admin", "is_restricted", "is_active")
						if err != nil {
							log.Error("SyncExternalUsers[%s]: Error updating user %s: %v", s.Name, usr.Name, err)
						}
					}
				}
			}

			// Rewrite authorized_keys file if LDAP Public SSH Key attribute is set and any key was added or removed
			if sshKeysNeedUpdate {
				err = RewriteAllPublicKeys()
				if err != nil {
					log.Error("RewriteAllPublicKeys: %v", err)
				}
			}

			select {
			case <-ctx.Done():
				log.Warn("SyncExternalUsers: Cancelled during update of %s before delete users", s.Name)
				return ErrCancelledf("During update of %s before delete users", s.Name)
			default:
			}

			// Deactivate users not present in LDAP
			if updateExisting {
				for _, usr := range users {
					found := false
					for _, uid := range existingUsers {
						if usr.ID == uid {
							found = true
							break
						}
					}
					if !found {
						log.Trace("SyncExternalUsers[%s]: Deactivating user %s", s.Name, usr.Name)

						usr.IsActive = false
						err = UpdateUserCols(usr, "is_active")
						if err != nil {
							log.Error("SyncExternalUsers[%s]: Error deactivating user %s: %v", s.Name, usr.Name, err)
						}
					}
				}
			}
		}
	}
	return nil
}

// IterateUser iterate users
func IterateUser(f func(user *User) error) error {
	var start int
	batchSize := setting.Database.IterateBufferSize
	for {
		users := make([]*User, 0, batchSize)
		if err := x.Limit(batchSize, start).Find(&users); err != nil {
			return err
		}
		if len(users) == 0 {
			return nil
		}
		start += len(users)

		for _, user := range users {
			if err := f(user); err != nil {
				return err
			}
		}
	}
}
