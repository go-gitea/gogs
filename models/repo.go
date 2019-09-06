// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"bytes"
	"crypto/md5"
	"errors"
	"fmt"
	"html/template"

	// Needed for jpeg support
	_ "image/jpeg"
	"image/png"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/avatar"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/options"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/sync"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/go-xorm/xorm"
	"github.com/unknwon/com"
	ini "gopkg.in/ini.v1"
	"xorm.io/builder"
)

var repoWorkingPool = sync.NewExclusivePool()

var (
	// ErrMirrorNotExist mirror does not exist error
	ErrMirrorNotExist = errors.New("Mirror does not exist")

	// ErrNameEmpty name is empty error
	ErrNameEmpty = errors.New("Name is empty")
)

var (
	// Gitignores contains the gitiginore files
	Gitignores []string

	// Licenses contains the license files
	Licenses []string

	// Readmes contains the readme files
	Readmes []string

	// LabelTemplates contains the label template files
	LabelTemplates []string

	// ItemsPerPage maximum items per page in forks, watchers and stars of a repo
	ItemsPerPage = 40
)

// loadRepoConfig loads the repository config
func loadRepoConfig() {
	// Load .gitignore and license files and readme templates.
	types := []string{"gitignore", "license", "readme", "label"}
	typeFiles := make([][]string, 4)
	for i, t := range types {
		files, err := options.Dir(t)
		if err != nil {
			log.Fatal("Failed to get %s files: %v", t, err)
		}
		customPath := path.Join(setting.CustomPath, "options", t)
		if com.IsDir(customPath) {
			customFiles, err := com.StatDir(customPath)
			if err != nil {
				log.Fatal("Failed to get custom %s files: %v", t, err)
			}

			for _, f := range customFiles {
				if !com.IsSliceContainsStr(files, f) {
					files = append(files, f)
				}
			}
		}
		typeFiles[i] = files
	}

	Gitignores = typeFiles[0]
	Licenses = typeFiles[1]
	Readmes = typeFiles[2]
	LabelTemplates = typeFiles[3]
	sort.Strings(Gitignores)
	sort.Strings(Licenses)
	sort.Strings(Readmes)
	sort.Strings(LabelTemplates)

	// Filter out invalid names and promote preferred licenses.
	sortedLicenses := make([]string, 0, len(Licenses))
	for _, name := range setting.Repository.PreferredLicenses {
		if com.IsSliceContainsStr(Licenses, name) {
			sortedLicenses = append(sortedLicenses, name)
		}
	}
	for _, name := range Licenses {
		if !com.IsSliceContainsStr(setting.Repository.PreferredLicenses, name) {
			sortedLicenses = append(sortedLicenses, name)
		}
	}
	Licenses = sortedLicenses
}

// NewRepoContext creates a new repository context
func NewRepoContext() {
	loadRepoConfig()

	RemoveAllWithNotice("Clean up repository temporary data", filepath.Join(setting.AppDataPath, "tmp"))
}

// Repository represents a git repository.
type Repository struct {
	ID            int64  `xorm:"pk autoincr"`
	OwnerID       int64  `xorm:"UNIQUE(s) index"`
	OwnerName     string `xorm:"-"`
	Owner         *User  `xorm:"-"`
	LowerName     string `xorm:"UNIQUE(s) INDEX NOT NULL"`
	Name          string `xorm:"INDEX NOT NULL"`
	Description   string `xorm:"TEXT"`
	Website       string `xorm:"VARCHAR(2048)"`
	OriginalURL   string `xorm:"VARCHAR(2048)"`
	DefaultBranch string

	NumWatches          int
	NumStars            int
	NumForks            int
	NumIssues           int
	NumClosedIssues     int
	NumOpenIssues       int `xorm:"-"`
	NumPulls            int
	NumClosedPulls      int
	NumOpenPulls        int `xorm:"-"`
	NumMilestones       int `xorm:"NOT NULL DEFAULT 0"`
	NumClosedMilestones int `xorm:"NOT NULL DEFAULT 0"`
	NumOpenMilestones   int `xorm:"-"`
	NumReleases         int `xorm:"-"`

	IsPrivate  bool `xorm:"INDEX"`
	IsEmpty    bool `xorm:"INDEX"`
	IsArchived bool `xorm:"INDEX"`

	IsMirror bool `xorm:"INDEX"`
	*Mirror  `xorm:"-"`

	ExternalMetas map[string]string `xorm:"-"`
	Units         []*RepoUnit       `xorm:"-"`

	IsFork                          bool               `xorm:"INDEX NOT NULL DEFAULT false"`
	ForkID                          int64              `xorm:"INDEX"`
	BaseRepo                        *Repository        `xorm:"-"`
	Size                            int64              `xorm:"NOT NULL DEFAULT 0"`
	IndexerStatus                   *RepoIndexerStatus `xorm:"-"`
	IsFsckEnabled                   bool               `xorm:"NOT NULL DEFAULT true"`
	CloseIssuesViaCommitInAnyBranch bool               `xorm:"NOT NULL DEFAULT false"`
	Topics                          []string           `xorm:"TEXT JSON"`

	// Avatar: ID(10-20)-md5(32) - must fit into 64 symbols
	Avatar string `xorm:"VARCHAR(64)"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

// ColorFormat returns a colored string to represent this repo
func (repo *Repository) ColorFormat(s fmt.State) {
	var ownerName interface{}

	if repo.OwnerName != "" {
		ownerName = repo.OwnerName
	} else if repo.Owner != nil {
		ownerName = repo.Owner.Name
	} else {
		ownerName = log.NewColoredIDValue(strconv.FormatInt(repo.OwnerID, 10))
	}

	log.ColorFprintf(s, "%d:%s/%s",
		log.NewColoredIDValue(repo.ID),
		ownerName,
		repo.Name)
}

// AfterLoad is invoked from XORM after setting the values of all fields of this object.
func (repo *Repository) AfterLoad() {
	// FIXME: use models migration to solve all at once.
	if len(repo.DefaultBranch) == 0 {
		repo.DefaultBranch = "master"
	}

	repo.NumOpenIssues = repo.NumIssues - repo.NumClosedIssues
	repo.NumOpenPulls = repo.NumPulls - repo.NumClosedPulls
	repo.NumOpenMilestones = repo.NumMilestones - repo.NumClosedMilestones
}

// MustOwner always returns a valid *User object to avoid
// conceptually impossible error handling.
// It creates a fake object that contains error details
// when error occurs.
func (repo *Repository) MustOwner() *User {
	return repo.mustOwner(x)
}

// MustOwnerName always returns valid owner name to avoid
// conceptually impossible error handling.
// It returns "error" and logs error details when error
// occurs.
func (repo *Repository) MustOwnerName() string {
	return repo.mustOwnerName(x)
}

// FullName returns the repository full name
func (repo *Repository) FullName() string {
	return repo.MustOwnerName() + "/" + repo.Name
}

// HTMLURL returns the repository HTML URL
func (repo *Repository) HTMLURL() string {
	return setting.AppURL + repo.FullName()
}

// APIURL returns the repository API URL
func (repo *Repository) APIURL() string {
	return setting.AppURL + path.Join("api/v1/repos", repo.FullName())
}

// APIFormat converts a Repository to api.Repository
func (repo *Repository) APIFormat(mode AccessMode) *api.Repository {
	return repo.innerAPIFormat(x, mode, false)
}

// GetCommitsCountCacheKey returns cache key used for commits count caching.
func (repo *Repository) GetCommitsCountCacheKey(contextName string, isRef bool) string {
	var prefix string
	if isRef {
		prefix = "ref"
	} else {
		prefix = "commit"
	}
	return fmt.Sprintf("commits-count-%d-%s-%s", repo.ID, prefix, contextName)
}

func (repo *Repository) innerAPIFormat(e Engine, mode AccessMode, isParent bool) *api.Repository {
	var parent *api.Repository

	cloneLink := repo.cloneLink(e, false)
	permission := &api.Permission{
		Admin: mode >= AccessModeAdmin,
		Push:  mode >= AccessModeWrite,
		Pull:  mode >= AccessModeRead,
	}
	if !isParent {
		err := repo.getBaseRepo(e)
		if err != nil {
			log.Error("APIFormat: %v", err)
		}
		if repo.BaseRepo != nil {
			parent = repo.BaseRepo.innerAPIFormat(e, mode, true)
		}
	}
	hasIssues := false
	if _, err := repo.getUnit(e, UnitTypeIssues); err == nil {
		hasIssues = true
	}
	hasWiki := false
	if _, err := repo.getUnit(e, UnitTypeWiki); err == nil {
		hasWiki = true
	}
	hasPullRequests := false
	ignoreWhitespaceConflicts := false
	allowMerge := false
	allowRebase := false
	allowRebaseMerge := false
	allowSquash := false
	if unit, err := repo.getUnit(e, UnitTypePullRequests); err == nil {
		config := unit.PullRequestsConfig()
		hasPullRequests = true
		ignoreWhitespaceConflicts = config.IgnoreWhitespaceConflicts
		allowMerge = config.AllowMerge
		allowRebase = config.AllowRebase
		allowRebaseMerge = config.AllowRebaseMerge
		allowSquash = config.AllowSquash
	}

	return &api.Repository{
		ID:                        repo.ID,
		Owner:                     repo.Owner.APIFormat(),
		Name:                      repo.Name,
		FullName:                  repo.FullName(),
		Description:               repo.Description,
		Private:                   repo.IsPrivate,
		Empty:                     repo.IsEmpty,
		Archived:                  repo.IsArchived,
		Size:                      int(repo.Size / 1024),
		Fork:                      repo.IsFork,
		Parent:                    parent,
		Mirror:                    repo.IsMirror,
		HTMLURL:                   repo.HTMLURL(),
		SSHURL:                    cloneLink.SSH,
		CloneURL:                  cloneLink.HTTPS,
		Website:                   repo.Website,
		Stars:                     repo.NumStars,
		Forks:                     repo.NumForks,
		Watchers:                  repo.NumWatches,
		OpenIssues:                repo.NumOpenIssues,
		DefaultBranch:             repo.DefaultBranch,
		Created:                   repo.CreatedUnix.AsTime(),
		Updated:                   repo.UpdatedUnix.AsTime(),
		Permissions:               permission,
		HasIssues:                 hasIssues,
		HasWiki:                   hasWiki,
		HasPullRequests:           hasPullRequests,
		IgnoreWhitespaceConflicts: ignoreWhitespaceConflicts,
		AllowMerge:                allowMerge,
		AllowRebase:               allowRebase,
		AllowRebaseMerge:          allowRebaseMerge,
		AllowSquash:               allowSquash,
		AvatarURL:                 repo.avatarLink(e),
	}
}

func (repo *Repository) getUnits(e Engine) (err error) {
	if repo.Units != nil {
		return nil
	}

	repo.Units, err = getUnitsByRepoID(e, repo.ID)
	return err
}

// CheckUnitUser check whether user could visit the unit of this repository
func (repo *Repository) CheckUnitUser(userID int64, isAdmin bool, unitType UnitType) bool {
	return repo.checkUnitUser(x, userID, isAdmin, unitType)
}

func (repo *Repository) checkUnitUser(e Engine, userID int64, isAdmin bool, unitType UnitType) bool {
	if isAdmin {
		return true
	}
	user, err := getUserByID(e, userID)
	if err != nil {
		return false
	}
	perm, err := getUserRepoPermission(e, repo, user)
	if err != nil {
		return false
	}

	return perm.CanRead(unitType)
}

// UnitEnabled if this repository has the given unit enabled
func (repo *Repository) UnitEnabled(tp UnitType) bool {
	if err := repo.getUnits(x); err != nil {
		log.Warn("Error loading repository (ID: %d) units: %s", repo.ID, err.Error())
	}
	for _, unit := range repo.Units {
		if unit.Type == tp {
			return true
		}
	}
	return false
}

// ErrUnitTypeNotExist represents a "UnitTypeNotExist" kind of error.
type ErrUnitTypeNotExist struct {
	UT UnitType
}

// IsErrUnitTypeNotExist checks if an error is a ErrUnitNotExist.
func IsErrUnitTypeNotExist(err error) bool {
	_, ok := err.(ErrUnitTypeNotExist)
	return ok
}

func (err ErrUnitTypeNotExist) Error() string {
	return fmt.Sprintf("Unit type does not exist: %s", err.UT.String())
}

// MustGetUnit always returns a RepoUnit object
func (repo *Repository) MustGetUnit(tp UnitType) *RepoUnit {
	ru, err := repo.GetUnit(tp)
	if err == nil {
		return ru
	}

	if tp == UnitTypeExternalWiki {
		return &RepoUnit{
			Type:   tp,
			Config: new(ExternalWikiConfig),
		}
	} else if tp == UnitTypeExternalTracker {
		return &RepoUnit{
			Type:   tp,
			Config: new(ExternalTrackerConfig),
		}
	} else if tp == UnitTypePullRequests {
		return &RepoUnit{
			Type:   tp,
			Config: new(PullRequestsConfig),
		}
	} else if tp == UnitTypeIssues {
		return &RepoUnit{
			Type:   tp,
			Config: new(IssuesConfig),
		}
	}
	return &RepoUnit{
		Type:   tp,
		Config: new(UnitConfig),
	}
}

// GetUnit returns a RepoUnit object
func (repo *Repository) GetUnit(tp UnitType) (*RepoUnit, error) {
	return repo.getUnit(x, tp)
}

func (repo *Repository) getUnit(e Engine, tp UnitType) (*RepoUnit, error) {
	if err := repo.getUnits(e); err != nil {
		return nil, err
	}
	for _, unit := range repo.Units {
		if unit.Type == tp {
			return unit, nil
		}
	}
	return nil, ErrUnitTypeNotExist{tp}
}

func (repo *Repository) getOwner(e Engine) (err error) {
	if repo.Owner != nil {
		return nil
	}

	repo.Owner, err = getUserByID(e, repo.OwnerID)
	return err
}

// GetOwner returns the repository owner
func (repo *Repository) GetOwner() error {
	return repo.getOwner(x)
}

func (repo *Repository) mustOwner(e Engine) *User {
	if err := repo.getOwner(e); err != nil {
		return &User{
			Name:     "error",
			FullName: err.Error(),
		}
	}

	return repo.Owner
}

func (repo *Repository) getOwnerName(e Engine) error {
	if len(repo.OwnerName) > 0 {
		return nil
	}

	if repo.Owner != nil {
		repo.OwnerName = repo.Owner.Name
		return nil
	}

	u := new(User)
	has, err := e.ID(repo.OwnerID).Cols("name").Get(u)
	if err != nil {
		return err
	} else if !has {
		return ErrUserNotExist{repo.OwnerID, "", 0}
	}
	repo.OwnerName = u.Name
	return nil
}

// GetOwnerName returns the repository owner name
func (repo *Repository) GetOwnerName() error {
	return repo.getOwnerName(x)
}

func (repo *Repository) mustOwnerName(e Engine) string {
	if err := repo.getOwnerName(e); err != nil {
		log.Error("Error loading repository owner name: %v", err)
		return "error"
	}

	return repo.OwnerName
}

// ComposeMetas composes a map of metas for properly rendering issue links and external issue trackers.
func (repo *Repository) ComposeMetas() map[string]string {
	if repo.ExternalMetas == nil {
		repo.ExternalMetas = map[string]string{
			"user":     repo.MustOwner().Name,
			"repo":     repo.Name,
			"repoPath": repo.RepoPath(),
		}
		unit, err := repo.GetUnit(UnitTypeExternalTracker)
		if err != nil {
			return repo.ExternalMetas
		}

		repo.ExternalMetas["format"] = unit.ExternalTrackerConfig().ExternalTrackerFormat
		switch unit.ExternalTrackerConfig().ExternalTrackerStyle {
		case markup.IssueNameStyleAlphanumeric:
			repo.ExternalMetas["style"] = markup.IssueNameStyleAlphanumeric
		case markup.IssueNameStyleRegexp:
			repo.ExternalMetas["style"] = markup.IssueNameStyleRegexp
		default:
			repo.ExternalMetas["style"] = markup.IssueNameStyleNumeric
		}
		repo.ExternalMetas["format"] = unit.ExternalTrackerConfig().ExternalTrackerFormat
		repo.ExternalMetas["regexp"] = unit.ExternalTrackerConfig().ExternalTrackerRegexpPattern

	}
	return repo.ExternalMetas
}

// DeleteWiki removes the actual and local copy of repository wiki.
func (repo *Repository) DeleteWiki() error {
	return repo.deleteWiki(x)
}

func (repo *Repository) deleteWiki(e Engine) error {
	wikiPaths := []string{repo.WikiPath()}
	for _, wikiPath := range wikiPaths {
		removeAllWithNotice(e, "Delete repository wiki", wikiPath)
	}

	_, err := e.Where("repo_id = ?", repo.ID).And("type = ?", UnitTypeWiki).Delete(new(RepoUnit))
	return err
}

func (repo *Repository) getAssignees(e Engine) (_ []*User, err error) {
	if err = repo.getOwner(e); err != nil {
		return nil, err
	}

	accesses := make([]*Access, 0, 10)
	if err = e.
		Where("repo_id = ? AND mode >= ?", repo.ID, AccessModeWrite).
		Find(&accesses); err != nil {
		return nil, err
	}

	// Leave a seat for owner itself to append later, but if owner is an organization
	// and just waste 1 unit is cheaper than re-allocate memory once.
	users := make([]*User, 0, len(accesses)+1)
	if len(accesses) > 0 {
		userIDs := make([]int64, len(accesses))
		for i := 0; i < len(accesses); i++ {
			userIDs[i] = accesses[i].UserID
		}

		if err = e.In("id", userIDs).Find(&users); err != nil {
			return nil, err
		}
	}
	if !repo.Owner.IsOrganization() {
		users = append(users, repo.Owner)
	}

	return users, nil
}

// GetAssignees returns all users that have write access and can be assigned to issues
// of the repository,
func (repo *Repository) GetAssignees() (_ []*User, err error) {
	return repo.getAssignees(x)
}

// GetMilestoneByID returns the milestone belongs to repository by given ID.
func (repo *Repository) GetMilestoneByID(milestoneID int64) (*Milestone, error) {
	return GetMilestoneByRepoID(repo.ID, milestoneID)
}

// IssueStats returns number of open and closed repository issues by given filter mode.
func (repo *Repository) IssueStats(uid int64, filterMode int, isPull bool) (int64, int64) {
	return GetRepoIssueStats(repo.ID, uid, filterMode, isPull)
}

// GetMirror sets the repository mirror, returns an error upon failure
func (repo *Repository) GetMirror() (err error) {
	repo.Mirror, err = GetMirrorByRepoID(repo.ID)
	return err
}

// GetBaseRepo populates repo.BaseRepo for a fork repository and
// returns an error on failure (NOTE: no error is returned for
// non-fork repositories, and BaseRepo will be left untouched)
func (repo *Repository) GetBaseRepo() (err error) {
	return repo.getBaseRepo(x)
}

func (repo *Repository) getBaseRepo(e Engine) (err error) {
	if !repo.IsFork {
		return nil
	}

	repo.BaseRepo, err = getRepositoryByID(e, repo.ForkID)
	return err
}

func (repo *Repository) repoPath(e Engine) string {
	return RepoPath(repo.mustOwnerName(e), repo.Name)
}

// RepoPath returns the repository path
func (repo *Repository) RepoPath() string {
	return repo.repoPath(x)
}

// GitConfigPath returns the path to a repository's git config/ directory
func GitConfigPath(repoPath string) string {
	return filepath.Join(repoPath, "config")
}

// GitConfigPath returns the repository git config path
func (repo *Repository) GitConfigPath() string {
	return GitConfigPath(repo.RepoPath())
}

// RelLink returns the repository relative link
func (repo *Repository) RelLink() string {
	return "/" + repo.FullName()
}

// Link returns the repository link
func (repo *Repository) Link() string {
	return setting.AppSubURL + "/" + repo.FullName()
}

// ComposeCompareURL returns the repository comparison URL
func (repo *Repository) ComposeCompareURL(oldCommitID, newCommitID string) string {
	return fmt.Sprintf("%s/%s/compare/%s...%s", repo.MustOwner().Name, repo.Name, oldCommitID, newCommitID)
}

// UpdateDefaultBranch updates the default branch
func (repo *Repository) UpdateDefaultBranch() error {
	_, err := x.ID(repo.ID).Cols("default_branch").Update(repo)
	return err
}

// IsOwnedBy returns true when user owns this repository
func (repo *Repository) IsOwnedBy(userID int64) bool {
	return repo.OwnerID == userID
}

func (repo *Repository) updateSize(e Engine) error {
	repoInfoSize, err := git.GetRepoSize(repo.repoPath(e))
	if err != nil {
		return fmt.Errorf("UpdateSize: %v", err)
	}

	repo.Size = repoInfoSize.Size + repoInfoSize.SizePack
	_, err = e.ID(repo.ID).Cols("size").Update(repo)
	return err
}

// UpdateSize updates the repository size, calculating it using git.GetRepoSize
func (repo *Repository) UpdateSize() error {
	return repo.updateSize(x)
}

// CanUserFork returns true if specified user can fork repository.
func (repo *Repository) CanUserFork(user *User) (bool, error) {
	if user == nil {
		return false, nil
	}
	if repo.OwnerID != user.ID && !user.HasForkedRepo(repo.ID) {
		return true, nil
	}
	if err := user.GetOwnedOrganizations(); err != nil {
		return false, err
	}
	for _, org := range user.OwnedOrgs {
		if repo.OwnerID != org.ID && !org.HasForkedRepo(repo.ID) {
			return true, nil
		}
	}
	return false, nil
}

// CanEnablePulls returns true if repository meets the requirements of accepting pulls.
func (repo *Repository) CanEnablePulls() bool {
	return !repo.IsMirror && !repo.IsEmpty
}

// AllowsPulls returns true if repository meets the requirements of accepting pulls and has them enabled.
func (repo *Repository) AllowsPulls() bool {
	return repo.CanEnablePulls() && repo.UnitEnabled(UnitTypePullRequests)
}

// CanEnableEditor returns true if repository meets the requirements of web editor.
func (repo *Repository) CanEnableEditor() bool {
	return !repo.IsMirror
}

// GetWriters returns all users that have write access to the repository.
func (repo *Repository) GetWriters() (_ []*User, err error) {
	return repo.getUsersWithAccessMode(x, AccessModeWrite)
}

// getUsersWithAccessMode returns users that have at least given access mode to the repository.
func (repo *Repository) getUsersWithAccessMode(e Engine, mode AccessMode) (_ []*User, err error) {
	if err = repo.getOwner(e); err != nil {
		return nil, err
	}

	accesses := make([]*Access, 0, 10)
	if err = e.Where("repo_id = ? AND mode >= ?", repo.ID, mode).Find(&accesses); err != nil {
		return nil, err
	}

	// Leave a seat for owner itself to append later, but if owner is an organization
	// and just waste 1 unit is cheaper than re-allocate memory once.
	users := make([]*User, 0, len(accesses)+1)
	if len(accesses) > 0 {
		userIDs := make([]int64, len(accesses))
		for i := 0; i < len(accesses); i++ {
			userIDs[i] = accesses[i].UserID
		}

		if err = e.In("id", userIDs).Find(&users); err != nil {
			return nil, err
		}
	}
	if !repo.Owner.IsOrganization() {
		users = append(users, repo.Owner)
	}

	return users, nil
}

// DescriptionHTML does special handles to description and return HTML string.
func (repo *Repository) DescriptionHTML() template.HTML {
	desc, err := markup.RenderDescriptionHTML([]byte(repo.Description), repo.HTMLURL(), repo.ComposeMetas())
	if err != nil {
		log.Error("Failed to render description for %s (ID: %d): %v", repo.Name, repo.ID, err)
		return template.HTML(markup.Sanitize(repo.Description))
	}
	return template.HTML(markup.Sanitize(string(desc)))
}

// PatchPath returns corresponding patch file path of repository by given issue ID.
func (repo *Repository) PatchPath(index int64) (string, error) {
	return repo.patchPath(x, index)
}

func (repo *Repository) patchPath(e Engine, index int64) (string, error) {
	if err := repo.getOwner(e); err != nil {
		return "", err
	}

	return filepath.Join(RepoPath(repo.Owner.Name, repo.Name), "pulls", com.ToStr(index)+".patch"), nil
}

// SavePatch saves patch data to corresponding location by given issue ID.
func (repo *Repository) SavePatch(index int64, patch []byte) error {
	return repo.savePatch(x, index, patch)
}

func (repo *Repository) savePatch(e Engine, index int64, patch []byte) error {
	patchPath, err := repo.patchPath(e, index)
	if err != nil {
		return fmt.Errorf("PatchPath: %v", err)
	}
	dir := filepath.Dir(patchPath)

	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return fmt.Errorf("Failed to create dir %s: %v", dir, err)
	}

	if err = ioutil.WriteFile(patchPath, patch, 0644); err != nil {
		return fmt.Errorf("WriteFile: %v", err)
	}

	return nil
}

func isRepositoryExist(e Engine, u *User, repoName string) (bool, error) {
	has, err := e.Get(&Repository{
		OwnerID:   u.ID,
		LowerName: strings.ToLower(repoName),
	})
	return has && com.IsDir(RepoPath(u.Name, repoName)), err
}

// IsRepositoryExist returns true if the repository with given name under user has already existed.
func IsRepositoryExist(u *User, repoName string) (bool, error) {
	return isRepositoryExist(x, u, repoName)
}

// CloneLink represents different types of clone URLs of repository.
type CloneLink struct {
	SSH   string
	HTTPS string
	Git   string
}

// ComposeHTTPSCloneURL returns HTTPS clone URL based on given owner and repository name.
func ComposeHTTPSCloneURL(owner, repo string) string {
	return fmt.Sprintf("%s%s/%s.git", setting.AppURL, url.PathEscape(owner), url.PathEscape(repo))
}

func (repo *Repository) cloneLink(e Engine, isWiki bool) *CloneLink {
	repoName := repo.Name
	if isWiki {
		repoName += ".wiki"
	}

	sshUser := setting.RunUser
	if setting.SSH.StartBuiltinServer {
		sshUser = setting.SSH.BuiltinServerUser
	}

	repo.Owner = repo.mustOwner(e)
	cl := new(CloneLink)
	if setting.SSH.Port != 22 {
		cl.SSH = fmt.Sprintf("ssh://%s@%s:%d/%s/%s.git", sshUser, setting.SSH.Domain, setting.SSH.Port, repo.Owner.Name, repoName)
	} else if setting.Repository.UseCompatSSHURI {
		cl.SSH = fmt.Sprintf("ssh://%s@%s/%s/%s.git", sshUser, setting.SSH.Domain, repo.Owner.Name, repoName)
	} else {
		cl.SSH = fmt.Sprintf("%s@%s:%s/%s.git", sshUser, setting.SSH.Domain, repo.Owner.Name, repoName)
	}
	cl.HTTPS = ComposeHTTPSCloneURL(repo.Owner.Name, repoName)
	return cl
}

// CloneLink returns clone URLs of repository.
func (repo *Repository) CloneLink() (cl *CloneLink) {
	return repo.cloneLink(x, false)
}

// MigrateRepoOptions contains the repository migrate options
type MigrateRepoOptions struct {
	Name                 string
	Description          string
	OriginalURL          string
	IsPrivate            bool
	IsMirror             bool
	RemoteAddr           string
	Wiki                 bool // include wiki repository
	SyncReleasesWithTags bool // sync releases from tags
}

/*
	GitHub, GitLab, Gogs: *.wiki.git
	BitBucket: *.git/wiki
*/
var commonWikiURLSuffixes = []string{".wiki.git", ".git/wiki"}

// wikiRemoteURL returns accessible repository URL for wiki if exists.
// Otherwise, it returns an empty string.
func wikiRemoteURL(remote string) string {
	remote = strings.TrimSuffix(remote, ".git")
	for _, suffix := range commonWikiURLSuffixes {
		wikiURL := remote + suffix
		if git.IsRepoURLAccessible(wikiURL) {
			return wikiURL
		}
	}
	return ""
}

// MigrateRepository migrates an existing repository from other project hosting.
func MigrateRepository(doer, u *User, opts MigrateRepoOptions) (*Repository, error) {
	repo, err := CreateRepository(doer, u, CreateRepoOptions{
		Name:        opts.Name,
		Description: opts.Description,
		OriginalURL: opts.OriginalURL,
		IsPrivate:   opts.IsPrivate,
		IsMirror:    opts.IsMirror,
	})
	if err != nil {
		return nil, err
	}

	repoPath := RepoPath(u.Name, opts.Name)

	if u.IsOrganization() {
		t, err := u.GetOwnerTeam()
		if err != nil {
			return nil, err
		}
		repo.NumWatches = t.NumMembers
	} else {
		repo.NumWatches = 1
	}

	migrateTimeout := time.Duration(setting.Git.Timeout.Migrate) * time.Second

	if err := os.RemoveAll(repoPath); err != nil {
		return repo, fmt.Errorf("Failed to remove %s: %v", repoPath, err)
	}

	if err = git.Clone(opts.RemoteAddr, repoPath, git.CloneRepoOptions{
		Mirror:  true,
		Quiet:   true,
		Timeout: migrateTimeout,
	}); err != nil {
		return repo, fmt.Errorf("Clone: %v", err)
	}

	if opts.Wiki {
		wikiPath := WikiPath(u.Name, opts.Name)
		wikiRemotePath := wikiRemoteURL(opts.RemoteAddr)
		if len(wikiRemotePath) > 0 {
			if err := os.RemoveAll(wikiPath); err != nil {
				return repo, fmt.Errorf("Failed to remove %s: %v", wikiPath, err)
			}

			if err = git.Clone(wikiRemotePath, wikiPath, git.CloneRepoOptions{
				Mirror:  true,
				Quiet:   true,
				Timeout: migrateTimeout,
				Branch:  "master",
			}); err != nil {
				log.Warn("Clone wiki: %v", err)
				if err := os.RemoveAll(wikiPath); err != nil {
					return repo, fmt.Errorf("Failed to remove %s: %v", wikiPath, err)
				}
			}
		}
	}

	gitRepo, err := git.OpenRepository(repoPath)
	if err != nil {
		return repo, fmt.Errorf("OpenRepository: %v", err)
	}

	repo.IsEmpty, err = gitRepo.IsEmpty()
	if err != nil {
		return repo, fmt.Errorf("git.IsEmpty: %v", err)
	}

	if opts.SyncReleasesWithTags && !repo.IsEmpty {
		// Try to get HEAD branch and set it as default branch.
		headBranch, err := gitRepo.GetHEADBranch()
		if err != nil {
			return repo, fmt.Errorf("GetHEADBranch: %v", err)
		}
		if headBranch != nil {
			repo.DefaultBranch = headBranch.Name
		}

		if err = SyncReleasesWithTags(repo, gitRepo); err != nil {
			log.Error("Failed to synchronize tags to releases for repository: %v", err)
		}
	}

	if err = repo.UpdateSize(); err != nil {
		log.Error("Failed to update size for repository: %v", err)
	}

	if opts.IsMirror {
		if _, err = x.InsertOne(&Mirror{
			RepoID:         repo.ID,
			Interval:       setting.Mirror.DefaultInterval,
			EnablePrune:    true,
			NextUpdateUnix: timeutil.TimeStampNow().AddDuration(setting.Mirror.DefaultInterval),
		}); err != nil {
			return repo, fmt.Errorf("InsertOne: %v", err)
		}

		repo.IsMirror = true
		err = UpdateRepository(repo, false)
	} else {
		repo, err = CleanUpMigrateInfo(repo)
	}

	if err != nil && !repo.IsEmpty {
		UpdateRepoIndexer(repo)
	}

	return repo, err
}

// cleanUpMigrateGitConfig removes mirror info which prevents "push --all".
// This also removes possible user credentials.
func cleanUpMigrateGitConfig(configPath string) error {
	cfg, err := ini.Load(configPath)
	if err != nil {
		return fmt.Errorf("open config file: %v", err)
	}
	cfg.DeleteSection("remote \"origin\"")
	if err = cfg.SaveToIndent(configPath, "\t"); err != nil {
		return fmt.Errorf("save config file: %v", err)
	}
	return nil
}

// createDelegateHooks creates all the hooks scripts for the repo
func createDelegateHooks(repoPath string) (err error) {
	var (
		hookNames = []string{"pre-receive", "update", "post-receive"}
		hookTpls  = []string{
			fmt.Sprintf("#!/usr/bin/env %s\ndata=$(cat)\nexitcodes=\"\"\nhookname=$(basename $0)\nGIT_DIR=${GIT_DIR:-$(dirname $0)}\n\nfor hook in ${GIT_DIR}/hooks/${hookname}.d/*; do\ntest -x \"${hook}\" || continue\necho \"${data}\" | \"${hook}\"\nexitcodes=\"${exitcodes} $?\"\ndone\n\nfor i in ${exitcodes}; do\n[ ${i} -eq 0 ] || exit ${i}\ndone\n", setting.ScriptType),
			fmt.Sprintf("#!/usr/bin/env %s\nexitcodes=\"\"\nhookname=$(basename $0)\nGIT_DIR=${GIT_DIR:-$(dirname $0)}\n\nfor hook in ${GIT_DIR}/hooks/${hookname}.d/*; do\ntest -x \"${hook}\" || continue\n\"${hook}\" $1 $2 $3\nexitcodes=\"${exitcodes} $?\"\ndone\n\nfor i in ${exitcodes}; do\n[ ${i} -eq 0 ] || exit ${i}\ndone\n", setting.ScriptType),
			fmt.Sprintf("#!/usr/bin/env %s\ndata=$(cat)\nexitcodes=\"\"\nhookname=$(basename $0)\nGIT_DIR=${GIT_DIR:-$(dirname $0)}\n\nfor hook in ${GIT_DIR}/hooks/${hookname}.d/*; do\ntest -x \"${hook}\" || continue\necho \"${data}\" | \"${hook}\"\nexitcodes=\"${exitcodes} $?\"\ndone\n\nfor i in ${exitcodes}; do\n[ ${i} -eq 0 ] || exit ${i}\ndone\n", setting.ScriptType),
		}
		giteaHookTpls = []string{
			fmt.Sprintf("#!/usr/bin/env %s\n\"%s\" hook --config='%s' pre-receive\n", setting.ScriptType, setting.AppPath, setting.CustomConf),
			fmt.Sprintf("#!/usr/bin/env %s\n\"%s\" hook --config='%s' update $1 $2 $3\n", setting.ScriptType, setting.AppPath, setting.CustomConf),
			fmt.Sprintf("#!/usr/bin/env %s\n\"%s\" hook --config='%s' post-receive\n", setting.ScriptType, setting.AppPath, setting.CustomConf),
		}
	)

	hookDir := filepath.Join(repoPath, "hooks")

	for i, hookName := range hookNames {
		oldHookPath := filepath.Join(hookDir, hookName)
		newHookPath := filepath.Join(hookDir, hookName+".d", "gitea")

		if err := os.MkdirAll(filepath.Join(hookDir, hookName+".d"), os.ModePerm); err != nil {
			return fmt.Errorf("create hooks dir '%s': %v", filepath.Join(hookDir, hookName+".d"), err)
		}

		// WARNING: This will override all old server-side hooks
		if err = ioutil.WriteFile(oldHookPath, []byte(hookTpls[i]), 0777); err != nil {
			return fmt.Errorf("write old hook file '%s': %v", oldHookPath, err)
		}

		if err = ioutil.WriteFile(newHookPath, []byte(giteaHookTpls[i]), 0777); err != nil {
			return fmt.Errorf("write new hook file '%s': %v", newHookPath, err)
		}
	}

	return nil
}

// CleanUpMigrateInfo finishes migrating repository and/or wiki with things that don't need to be done for mirrors.
func CleanUpMigrateInfo(repo *Repository) (*Repository, error) {
	repoPath := repo.RepoPath()
	if err := createDelegateHooks(repoPath); err != nil {
		return repo, fmt.Errorf("createDelegateHooks: %v", err)
	}
	if repo.HasWiki() {
		if err := createDelegateHooks(repo.WikiPath()); err != nil {
			return repo, fmt.Errorf("createDelegateHooks.(wiki): %v", err)
		}
	}

	_, err := git.NewCommand("remote", "remove", "origin").RunInDir(repoPath)
	if err != nil && !strings.HasPrefix(err.Error(), "exit status 128 - fatal: No such remote ") {
		return repo, fmt.Errorf("CleanUpMigrateInfo: %v", err)
	}

	if repo.HasWiki() {
		if err := cleanUpMigrateGitConfig(path.Join(repo.WikiPath(), "config")); err != nil {
			return repo, fmt.Errorf("cleanUpMigrateGitConfig (wiki): %v", err)
		}
	}

	return repo, UpdateRepository(repo, false)
}

// initRepoCommit temporarily changes with work directory.
func initRepoCommit(tmpPath string, sig *git.Signature) (err error) {
	var stderr string
	if _, stderr, err = process.GetManager().ExecDir(-1,
		tmpPath, fmt.Sprintf("initRepoCommit (git add): %s", tmpPath),
		git.GitExecutable, "add", "--all"); err != nil {
		return fmt.Errorf("git add: %s", stderr)
	}

	if _, stderr, err = process.GetManager().ExecDir(-1,
		tmpPath, fmt.Sprintf("initRepoCommit (git commit): %s", tmpPath),
		git.GitExecutable, "commit", fmt.Sprintf("--author='%s <%s>'", sig.Name, sig.Email),
		"-m", "Initial commit"); err != nil {
		return fmt.Errorf("git commit: %s", stderr)
	}

	if _, stderr, err = process.GetManager().ExecDir(-1,
		tmpPath, fmt.Sprintf("initRepoCommit (git push): %s", tmpPath),
		git.GitExecutable, "push", "origin", "master"); err != nil {
		return fmt.Errorf("git push: %s", stderr)
	}
	return nil
}

// CreateRepoOptions contains the create repository options
type CreateRepoOptions struct {
	Name        string
	Description string
	OriginalURL string
	Gitignores  string
	License     string
	Readme      string
	IsPrivate   bool
	IsMirror    bool
	AutoInit    bool
}

func getRepoInitFile(tp, name string) ([]byte, error) {
	cleanedName := strings.TrimLeft(path.Clean("/"+name), "/")
	relPath := path.Join("options", tp, cleanedName)

	// Use custom file when available.
	customPath := path.Join(setting.CustomPath, relPath)
	if com.IsFile(customPath) {
		return ioutil.ReadFile(customPath)
	}

	switch tp {
	case "readme":
		return options.Readme(cleanedName)
	case "gitignore":
		return options.Gitignore(cleanedName)
	case "license":
		return options.License(cleanedName)
	case "label":
		return options.Labels(cleanedName)
	default:
		return []byte{}, fmt.Errorf("Invalid init file type")
	}
}

func prepareRepoCommit(e Engine, repo *Repository, tmpDir, repoPath string, opts CreateRepoOptions) error {
	// Clone to temporary path and do the init commit.
	_, stderr, err := process.GetManager().Exec(
		fmt.Sprintf("initRepository(git clone): %s", repoPath),
		git.GitExecutable, "clone", repoPath, tmpDir,
	)
	if err != nil {
		return fmt.Errorf("git clone: %v - %s", err, stderr)
	}

	// README
	data, err := getRepoInitFile("readme", opts.Readme)
	if err != nil {
		return fmt.Errorf("getRepoInitFile[%s]: %v", opts.Readme, err)
	}

	cloneLink := repo.cloneLink(e, false)
	match := map[string]string{
		"Name":           repo.Name,
		"Description":    repo.Description,
		"CloneURL.SSH":   cloneLink.SSH,
		"CloneURL.HTTPS": cloneLink.HTTPS,
	}
	if err = ioutil.WriteFile(filepath.Join(tmpDir, "README.md"),
		[]byte(com.Expand(string(data), match)), 0644); err != nil {
		return fmt.Errorf("write README.md: %v", err)
	}

	// .gitignore
	if len(opts.Gitignores) > 0 {
		var buf bytes.Buffer
		names := strings.Split(opts.Gitignores, ",")
		for _, name := range names {
			data, err = getRepoInitFile("gitignore", name)
			if err != nil {
				return fmt.Errorf("getRepoInitFile[%s]: %v", name, err)
			}
			buf.WriteString("# ---> " + name + "\n")
			buf.Write(data)
			buf.WriteString("\n")
		}

		if buf.Len() > 0 {
			if err = ioutil.WriteFile(filepath.Join(tmpDir, ".gitignore"), buf.Bytes(), 0644); err != nil {
				return fmt.Errorf("write .gitignore: %v", err)
			}
		}
	}

	// LICENSE
	if len(opts.License) > 0 {
		data, err = getRepoInitFile("license", opts.License)
		if err != nil {
			return fmt.Errorf("getRepoInitFile[%s]: %v", opts.License, err)
		}

		if err = ioutil.WriteFile(filepath.Join(tmpDir, "LICENSE"), data, 0644); err != nil {
			return fmt.Errorf("write LICENSE: %v", err)
		}
	}

	return nil
}

// InitRepository initializes README and .gitignore if needed.
func initRepository(e Engine, repoPath string, u *User, repo *Repository, opts CreateRepoOptions) (err error) {
	// Somehow the directory could exist.
	if com.IsExist(repoPath) {
		return fmt.Errorf("initRepository: path already exists: %s", repoPath)
	}

	// Init git bare new repository.
	if err = git.InitRepository(repoPath, true); err != nil {
		return fmt.Errorf("InitRepository: %v", err)
	} else if err = createDelegateHooks(repoPath); err != nil {
		return fmt.Errorf("createDelegateHooks: %v", err)
	}

	tmpDir := filepath.Join(os.TempDir(), "gitea-"+repo.Name+"-"+com.ToStr(time.Now().Nanosecond()))

	// Initialize repository according to user's choice.
	if opts.AutoInit {

		if err := os.MkdirAll(tmpDir, os.ModePerm); err != nil {
			return fmt.Errorf("Failed to create dir %s: %v", tmpDir, err)
		}

		defer os.RemoveAll(tmpDir)

		if err = prepareRepoCommit(e, repo, tmpDir, repoPath, opts); err != nil {
			return fmt.Errorf("prepareRepoCommit: %v", err)
		}

		// Apply changes and commit.
		if err = initRepoCommit(tmpDir, u.NewGitSig()); err != nil {
			return fmt.Errorf("initRepoCommit: %v", err)
		}
	}

	// Re-fetch the repository from database before updating it (else it would
	// override changes that were done earlier with sql)
	if repo, err = getRepositoryByID(e, repo.ID); err != nil {
		return fmt.Errorf("getRepositoryByID: %v", err)
	}

	if !opts.AutoInit {
		repo.IsEmpty = true
	}

	repo.DefaultBranch = "master"
	if err = updateRepository(e, repo, false); err != nil {
		return fmt.Errorf("updateRepository: %v", err)
	}

	return nil
}

var (
	reservedRepoNames    = []string{".", ".."}
	reservedRepoPatterns = []string{"*.git", "*.wiki"}
)

// IsUsableRepoName returns true when repository is usable
func IsUsableRepoName(name string) error {
	return isUsableName(reservedRepoNames, reservedRepoPatterns, name)
}

func createRepository(e *xorm.Session, doer, u *User, repo *Repository) (err error) {
	if err = IsUsableRepoName(repo.Name); err != nil {
		return err
	}

	has, err := isRepositoryExist(e, u, repo.Name)
	if err != nil {
		return fmt.Errorf("IsRepositoryExist: %v", err)
	} else if has {
		return ErrRepoAlreadyExist{u.Name, repo.Name}
	}

	if _, err = e.Insert(repo); err != nil {
		return err
	}
	if err = deleteRepoRedirect(e, u.ID, repo.Name); err != nil {
		return err
	}

	// insert units for repo
	var units = make([]RepoUnit, 0, len(DefaultRepoUnits))
	for _, tp := range DefaultRepoUnits {
		if tp == UnitTypeIssues {
			units = append(units, RepoUnit{
				RepoID: repo.ID,
				Type:   tp,
				Config: &IssuesConfig{
					EnableTimetracker:                setting.Service.DefaultEnableTimetracking,
					AllowOnlyContributorsToTrackTime: setting.Service.DefaultAllowOnlyContributorsToTrackTime,
					EnableDependencies:               setting.Service.DefaultEnableDependencies,
				},
			})
		} else if tp == UnitTypePullRequests {
			units = append(units, RepoUnit{
				RepoID: repo.ID,
				Type:   tp,
				Config: &PullRequestsConfig{AllowMerge: true, AllowRebase: true, AllowRebaseMerge: true, AllowSquash: true},
			})
		} else {
			units = append(units, RepoUnit{
				RepoID: repo.ID,
				Type:   tp,
			})
		}

	}

	if _, err = e.Insert(&units); err != nil {
		return err
	}

	// Remember visibility preference.
	u.LastRepoVisibility = repo.IsPrivate
	if err = updateUserCols(e, u, "last_repo_visibility"); err != nil {
		return fmt.Errorf("updateUser: %v", err)
	}

	if _, err = e.Incr("num_repos").ID(u.ID).Update(new(User)); err != nil {
		return fmt.Errorf("increment user total_repos: %v", err)
	}
	u.NumRepos++

	// Give access to all members in owner team.
	if u.IsOrganization() {
		t, err := u.getOwnerTeam(e)
		if err != nil {
			return fmt.Errorf("getOwnerTeam: %v", err)
		} else if err = t.addRepository(e, repo); err != nil {
			return fmt.Errorf("addRepository: %v", err)
		} else if err = prepareWebhooks(e, repo, HookEventRepository, &api.RepositoryPayload{
			Action:       api.HookRepoCreated,
			Repository:   repo.innerAPIFormat(e, AccessModeOwner, false),
			Organization: u.APIFormat(),
			Sender:       doer.APIFormat(),
		}); err != nil {
			return fmt.Errorf("prepareWebhooks: %v", err)
		}
	} else if err = repo.recalculateAccesses(e); err != nil {
		// Organization automatically called this in addRepository method.
		return fmt.Errorf("recalculateAccesses: %v", err)
	}

	if setting.Service.AutoWatchNewRepos {
		if err = watchRepo(e, doer.ID, repo.ID, true); err != nil {
			return fmt.Errorf("watchRepo: %v", err)
		}
	}
	if err = newRepoAction(e, doer, repo); err != nil {
		return fmt.Errorf("newRepoAction: %v", err)
	}

	if err = copyDefaultWebhooksToRepo(e, repo.ID); err != nil {
		return fmt.Errorf("copyDefaultWebhooksToRepo: %v", err)
	}

	return nil
}

// CreateRepository creates a repository for the user/organization.
func CreateRepository(doer, u *User, opts CreateRepoOptions) (_ *Repository, err error) {
	if !doer.IsAdmin && !u.CanCreateRepo() {
		return nil, ErrReachLimitOfRepo{u.MaxRepoCreation}
	}

	repo := &Repository{
		OwnerID:                         u.ID,
		Owner:                           u,
		Name:                            opts.Name,
		LowerName:                       strings.ToLower(opts.Name),
		Description:                     opts.Description,
		OriginalURL:                     opts.OriginalURL,
		IsPrivate:                       opts.IsPrivate,
		IsFsckEnabled:                   !opts.IsMirror,
		CloseIssuesViaCommitInAnyBranch: setting.Repository.DefaultCloseIssuesViaCommitsInAnyBranch,
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return nil, err
	}

	if err = createRepository(sess, doer, u, repo); err != nil {
		return nil, err
	}

	// No need for init mirror.
	if !opts.IsMirror {
		repoPath := RepoPath(u.Name, repo.Name)
		if err = initRepository(sess, repoPath, u, repo, opts); err != nil {
			if err2 := os.RemoveAll(repoPath); err2 != nil {
				log.Error("initRepository: %v", err)
				return nil, fmt.Errorf(
					"delete repo directory %s/%s failed(2): %v", u.Name, repo.Name, err2)
			}
			return nil, fmt.Errorf("initRepository: %v", err)
		}

		_, stderr, err := process.GetManager().ExecDir(-1,
			repoPath, fmt.Sprintf("CreateRepository(git update-server-info): %s", repoPath),
			git.GitExecutable, "update-server-info")
		if err != nil {
			return nil, errors.New("CreateRepository(git update-server-info): " + stderr)
		}
	}

	if err = sess.Commit(); err != nil {
		return nil, err
	}

	// Add to hook queue for created repo after session commit.
	if u.IsOrganization() {
		go HookQueue.Add(repo.ID)
	}

	return repo, err
}

func countRepositories(userID int64, private bool) int64 {
	sess := x.Where("id > 0")

	if userID > 0 {
		sess.And("owner_id = ?", userID)
	}
	if !private {
		sess.And("is_private=?", false)
	}

	count, err := sess.Count(new(Repository))
	if err != nil {
		log.Error("countRepositories: %v", err)
	}
	return count
}

// CountRepositories returns number of repositories.
// Argument private only takes effect when it is false,
// set it true to count all repositories.
func CountRepositories(private bool) int64 {
	return countRepositories(-1, private)
}

// CountUserRepositories returns number of repositories user owns.
// Argument private only takes effect when it is false,
// set it true to count all repositories.
func CountUserRepositories(userID int64, private bool) int64 {
	return countRepositories(userID, private)
}

// RepoPath returns repository path by given user and repository name.
func RepoPath(userName, repoName string) string {
	return filepath.Join(UserPath(userName), strings.ToLower(repoName)+".git")
}

// TransferOwnership transfers all corresponding setting from old user to new one.
func TransferOwnership(doer *User, newOwnerName string, repo *Repository) error {
	newOwner, err := GetUserByName(newOwnerName)
	if err != nil {
		return fmt.Errorf("get new owner '%s': %v", newOwnerName, err)
	}

	// Check if new owner has repository with same name.
	has, err := IsRepositoryExist(newOwner, repo.Name)
	if err != nil {
		return fmt.Errorf("IsRepositoryExist: %v", err)
	} else if has {
		return ErrRepoAlreadyExist{newOwnerName, repo.Name}
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return fmt.Errorf("sess.Begin: %v", err)
	}

	owner := repo.Owner

	// Note: we have to set value here to make sure recalculate accesses is based on
	// new owner.
	repo.OwnerID = newOwner.ID
	repo.Owner = newOwner

	// Update repository.
	if _, err := sess.ID(repo.ID).Update(repo); err != nil {
		return fmt.Errorf("update owner: %v", err)
	}

	// Remove redundant collaborators.
	collaborators, err := repo.getCollaborators(sess)
	if err != nil {
		return fmt.Errorf("getCollaborators: %v", err)
	}

	// Dummy object.
	collaboration := &Collaboration{RepoID: repo.ID}
	for _, c := range collaborators {
		if c.ID != newOwner.ID {
			isMember, err := isOrganizationMember(sess, newOwner.ID, c.ID)
			if err != nil {
				return fmt.Errorf("IsOrgMember: %v", err)
			} else if !isMember {
				continue
			}
		}
		collaboration.UserID = c.ID
		if _, err = sess.Delete(collaboration); err != nil {
			return fmt.Errorf("remove collaborator '%d': %v", c.ID, err)
		}
	}

	// Remove old team-repository relations.
	if owner.IsOrganization() {
		if err = owner.removeOrgRepo(sess, repo.ID); err != nil {
			return fmt.Errorf("removeOrgRepo: %v", err)
		}
	}

	if newOwner.IsOrganization() {
		t, err := newOwner.getOwnerTeam(sess)
		if err != nil {
			return fmt.Errorf("getOwnerTeam: %v", err)
		} else if err = t.addRepository(sess, repo); err != nil {
			return fmt.Errorf("add to owner team: %v", err)
		}
	} else if err = repo.recalculateAccesses(sess); err != nil {
		// Organization called this in addRepository method.
		return fmt.Errorf("recalculateAccesses: %v", err)
	}

	// Update repository count.
	if _, err = sess.Exec("UPDATE `user` SET num_repos=num_repos+1 WHERE id=?", newOwner.ID); err != nil {
		return fmt.Errorf("increase new owner repository count: %v", err)
	} else if _, err = sess.Exec("UPDATE `user` SET num_repos=num_repos-1 WHERE id=?", owner.ID); err != nil {
		return fmt.Errorf("decrease old owner repository count: %v", err)
	}

	if err = watchRepo(sess, doer.ID, repo.ID, true); err != nil {
		return fmt.Errorf("watchRepo: %v", err)
	} else if err = transferRepoAction(sess, doer, owner, repo); err != nil {
		return fmt.Errorf("transferRepoAction: %v", err)
	}

	// Rename remote repository to new path and delete local copy.
	dir := UserPath(newOwner.Name)

	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return fmt.Errorf("Failed to create dir %s: %v", dir, err)
	}

	if err = os.Rename(RepoPath(owner.Name, repo.Name), RepoPath(newOwner.Name, repo.Name)); err != nil {
		return fmt.Errorf("rename repository directory: %v", err)
	}

	// Rename remote wiki repository to new path and delete local copy.
	wikiPath := WikiPath(owner.Name, repo.Name)
	if com.IsExist(wikiPath) {
		if err = os.Rename(wikiPath, WikiPath(newOwner.Name, repo.Name)); err != nil {
			return fmt.Errorf("rename repository wiki: %v", err)
		}
	}

	// If there was previously a redirect at this location, remove it.
	if err = deleteRepoRedirect(sess, newOwner.ID, repo.Name); err != nil {
		return fmt.Errorf("delete repo redirect: %v", err)
	}

	return sess.Commit()
}

// ChangeRepositoryName changes all corresponding setting from old repository name to new one.
func ChangeRepositoryName(u *User, oldRepoName, newRepoName string) (err error) {
	oldRepoName = strings.ToLower(oldRepoName)
	newRepoName = strings.ToLower(newRepoName)
	if err = IsUsableRepoName(newRepoName); err != nil {
		return err
	}

	has, err := IsRepositoryExist(u, newRepoName)
	if err != nil {
		return fmt.Errorf("IsRepositoryExist: %v", err)
	} else if has {
		return ErrRepoAlreadyExist{u.Name, newRepoName}
	}

	repo, err := GetRepositoryByName(u.ID, oldRepoName)
	if err != nil {
		return fmt.Errorf("GetRepositoryByName: %v", err)
	}

	// Change repository directory name. We must lock the local copy of the
	// repo so that we can atomically rename the repo path and updates the
	// local copy's origin accordingly.
	repoWorkingPool.CheckIn(com.ToStr(repo.ID))
	defer repoWorkingPool.CheckOut(com.ToStr(repo.ID))

	newRepoPath := RepoPath(u.Name, newRepoName)
	if err = os.Rename(repo.RepoPath(), newRepoPath); err != nil {
		return fmt.Errorf("rename repository directory: %v", err)
	}

	wikiPath := repo.WikiPath()
	if com.IsExist(wikiPath) {
		if err = os.Rename(wikiPath, WikiPath(u.Name, newRepoName)); err != nil {
			return fmt.Errorf("rename repository wiki: %v", err)
		}
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return fmt.Errorf("sess.Begin: %v", err)
	}

	// If there was previously a redirect at this location, remove it.
	if err = deleteRepoRedirect(sess, u.ID, newRepoName); err != nil {
		return fmt.Errorf("delete repo redirect: %v", err)
	}

	return sess.Commit()
}

func getRepositoriesByForkID(e Engine, forkID int64) ([]*Repository, error) {
	repos := make([]*Repository, 0, 10)
	return repos, e.
		Where("fork_id=?", forkID).
		Find(&repos)
}

// GetRepositoriesByForkID returns all repositories with given fork ID.
func GetRepositoriesByForkID(forkID int64) ([]*Repository, error) {
	return getRepositoriesByForkID(x, forkID)
}

func updateRepository(e Engine, repo *Repository, visibilityChanged bool) (err error) {
	repo.LowerName = strings.ToLower(repo.Name)

	if len(repo.Description) > 255 {
		repo.Description = repo.Description[:255]
	}
	if len(repo.Website) > 255 {
		repo.Website = repo.Website[:255]
	}

	if _, err = e.ID(repo.ID).AllCols().Update(repo); err != nil {
		return fmt.Errorf("update: %v", err)
	}

	if visibilityChanged {
		if err = repo.getOwner(e); err != nil {
			return fmt.Errorf("getOwner: %v", err)
		}
		if repo.Owner.IsOrganization() {
			// Organization repository need to recalculate access table when visibility is changed.
			if err = repo.recalculateTeamAccesses(e, 0); err != nil {
				return fmt.Errorf("recalculateTeamAccesses: %v", err)
			}
		}

		// If repo has become private, we need to set its actions to private.
		if repo.IsPrivate {
			_, err = e.Where("repo_id = ?", repo.ID).Cols("is_private").Update(&Action{
				IsPrivate: true,
			})
			if err != nil {
				return err
			}
		}

		// Create/Remove git-daemon-export-ok for git-daemon...
		daemonExportFile := path.Join(repo.repoPath(e), `git-daemon-export-ok`)
		if repo.IsPrivate && com.IsExist(daemonExportFile) {
			if err = os.Remove(daemonExportFile); err != nil {
				log.Error("Failed to remove %s: %v", daemonExportFile, err)
			}
		} else if !repo.IsPrivate && !com.IsExist(daemonExportFile) {
			if f, err := os.Create(daemonExportFile); err != nil {
				log.Error("Failed to create %s: %v", daemonExportFile, err)
			} else {
				f.Close()
			}
		}

		forkRepos, err := getRepositoriesByForkID(e, repo.ID)
		if err != nil {
			return fmt.Errorf("getRepositoriesByForkID: %v", err)
		}
		for i := range forkRepos {
			forkRepos[i].IsPrivate = repo.IsPrivate
			if err = updateRepository(e, forkRepos[i], true); err != nil {
				return fmt.Errorf("updateRepository[%d]: %v", forkRepos[i].ID, err)
			}
		}

		if err = repo.updateSize(e); err != nil {
			log.Error("Failed to update size for repository: %v", err)
		}
	}

	return nil
}

// UpdateRepository updates a repository
func UpdateRepository(repo *Repository, visibilityChanged bool) (err error) {
	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if err = updateRepository(sess, repo, visibilityChanged); err != nil {
		return fmt.Errorf("updateRepository: %v", err)
	}

	return sess.Commit()
}

// UpdateRepositoryUnits updates a repository's units
func UpdateRepositoryUnits(repo *Repository, units []RepoUnit) (err error) {
	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if _, err = sess.Where("repo_id = ?", repo.ID).Delete(new(RepoUnit)); err != nil {
		return err
	}

	if _, err = sess.Insert(units); err != nil {
		return err
	}

	return sess.Commit()
}

// DeleteRepository deletes a repository for a user or organization.
func DeleteRepository(doer *User, uid, repoID int64) error {
	// In case is a organization.
	org, err := GetUserByID(uid)
	if err != nil {
		return err
	}
	if org.IsOrganization() {
		if err = org.GetTeams(); err != nil {
			return err
		}
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	repo := &Repository{ID: repoID, OwnerID: uid}
	has, err := sess.Get(repo)
	if err != nil {
		return err
	} else if !has {
		return ErrRepoNotExist{repoID, uid, "", ""}
	}

	// Delete Deploy Keys
	deployKeys, err := listDeployKeys(sess, repo.ID)
	if err != nil {
		return fmt.Errorf("listDeployKeys: %v", err)
	}
	for _, dKey := range deployKeys {
		if err := deleteDeployKey(sess, doer, dKey.ID); err != nil {
			return fmt.Errorf("deleteDeployKeys: %v", err)
		}
	}

	if cnt, err := sess.ID(repoID).Delete(&Repository{}); err != nil {
		return err
	} else if cnt != 1 {
		return ErrRepoNotExist{repoID, uid, "", ""}
	}

	if org.IsOrganization() {
		for _, t := range org.Teams {
			if !t.hasRepository(sess, repoID) {
				continue
			} else if err = t.removeRepository(sess, repo, false); err != nil {
				return err
			}
		}
	}

	if err = deleteBeans(sess,
		&Access{RepoID: repo.ID},
		&Action{RepoID: repo.ID},
		&Watch{RepoID: repoID},
		&Star{RepoID: repoID},
		&Mirror{RepoID: repoID},
		&Milestone{RepoID: repoID},
		&Release{RepoID: repoID},
		&Collaboration{RepoID: repoID},
		&PullRequest{BaseRepoID: repoID},
		&RepoUnit{RepoID: repoID},
		&RepoRedirect{RedirectRepoID: repoID},
		&Webhook{RepoID: repoID},
		&HookTask{RepoID: repoID},
		&Notification{RepoID: repoID},
		&CommitStatus{RepoID: repoID},
		&RepoIndexerStatus{RepoID: repoID},
	); err != nil {
		return fmt.Errorf("deleteBeans: %v", err)
	}

	deleteCond := builder.Select("id").From("issue").Where(builder.Eq{"repo_id": repoID})
	// Delete comments and attachments
	if _, err = sess.In("issue_id", deleteCond).
		Delete(&Comment{}); err != nil {
		return err
	}

	if _, err = sess.In("issue_id", deleteCond).
		Delete(&IssueUser{}); err != nil {
		return err
	}

	if _, err = sess.In("issue_id", deleteCond).
		Delete(&Reaction{}); err != nil {
		return err
	}

	if _, err = sess.In("issue_id", deleteCond).
		Delete(&IssueWatch{}); err != nil {
		return err
	}

	if _, err = sess.In("issue_id", deleteCond).
		Delete(&Stopwatch{}); err != nil {
		return err
	}

	attachmentPaths := make([]string, 0, 20)
	attachments := make([]*Attachment, 0, len(attachmentPaths))
	if err = sess.Join("INNER", "issue", "issue.id = attachment.issue_id").
		Where("issue.repo_id = ?", repoID).
		Find(&attachments); err != nil {
		return err
	}
	for j := range attachments {
		attachmentPaths = append(attachmentPaths, attachments[j].LocalPath())
	}

	if _, err = sess.In("issue_id", deleteCond).
		Delete(&Attachment{}); err != nil {
		return err
	}

	if _, err = sess.Delete(&Issue{RepoID: repoID}); err != nil {
		return err
	}

	if _, err = sess.Where("repo_id = ?", repoID).Delete(new(RepoUnit)); err != nil {
		return err
	}

	if repo.IsFork {
		if _, err = sess.Exec("UPDATE `repository` SET num_forks=num_forks-1 WHERE id=?", repo.ForkID); err != nil {
			return fmt.Errorf("decrease fork count: %v", err)
		}
	}

	if _, err = sess.Exec("UPDATE `user` SET num_repos=num_repos-1 WHERE id=?", uid); err != nil {
		return err
	}

	// FIXME: Remove repository files should be executed after transaction succeed.
	repoPath := repo.repoPath(sess)
	removeAllWithNotice(sess, "Delete repository files", repoPath)

	err = repo.deleteWiki(sess)
	if err != nil {
		return err
	}

	// Remove attachment files.
	for i := range attachmentPaths {
		removeAllWithNotice(sess, "Delete attachment", attachmentPaths[i])
	}

	// Remove LFS objects
	var lfsObjects []*LFSMetaObject
	if err = sess.Where("repository_id=?", repoID).Find(&lfsObjects); err != nil {
		return err
	}

	for _, v := range lfsObjects {
		count, err := sess.Count(&LFSMetaObject{Oid: v.Oid})
		if err != nil {
			return err
		}

		if count > 1 {
			continue
		}

		oidPath := filepath.Join(v.Oid[0:2], v.Oid[2:4], v.Oid[4:len(v.Oid)])
		removeAllWithNotice(sess, "Delete orphaned LFS file", oidPath)
	}

	if _, err := sess.Delete(&LFSMetaObject{RepositoryID: repoID}); err != nil {
		return err
	}

	if repo.NumForks > 0 {
		if _, err = sess.Exec("UPDATE `repository` SET fork_id=0,is_fork=? WHERE fork_id=?", false, repo.ID); err != nil {
			log.Error("reset 'fork_id' and 'is_fork': %v", err)
		}
	}

	if err = sess.Commit(); err != nil {
		if len(deployKeys) > 0 {
			// We need to rewrite the public keys because the commit failed
			if err2 := RewriteAllPublicKeys(); err2 != nil {
				return fmt.Errorf("Commit: %v SSH Keys: %v", err, err2)
			}
		}
		return fmt.Errorf("Commit: %v", err)
	}

	if org.IsOrganization() {
		if err = PrepareWebhooks(repo, HookEventRepository, &api.RepositoryPayload{
			Action:       api.HookRepoDeleted,
			Repository:   repo.APIFormat(AccessModeOwner),
			Organization: org.APIFormat(),
			Sender:       doer.APIFormat(),
		}); err != nil {
			return err
		}
		go HookQueue.Add(repo.ID)
	}

	if len(repo.Avatar) > 0 {
		avatarPath := repo.CustomAvatarPath()
		if com.IsExist(avatarPath) {
			if err := os.Remove(avatarPath); err != nil {
				return fmt.Errorf("Failed to remove %s: %v", avatarPath, err)
			}
		}
	}

	DeleteRepoFromIndexer(repo)
	return nil
}

// GetRepositoryByOwnerAndName returns the repository by given ownername and reponame.
func GetRepositoryByOwnerAndName(ownerName, repoName string) (*Repository, error) {
	var repo Repository
	has, err := x.Select("repository.*").
		Join("INNER", "`user`", "`user`.id = repository.owner_id").
		Where("repository.lower_name = ?", strings.ToLower(repoName)).
		And("`user`.lower_name = ?", strings.ToLower(ownerName)).
		Get(&repo)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrRepoNotExist{0, 0, ownerName, repoName}
	}
	return &repo, nil
}

// GetRepositoryByName returns the repository by given name under user if exists.
func GetRepositoryByName(ownerID int64, name string) (*Repository, error) {
	repo := &Repository{
		OwnerID:   ownerID,
		LowerName: strings.ToLower(name),
	}
	has, err := x.Get(repo)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrRepoNotExist{0, ownerID, "", name}
	}
	return repo, err
}

func getRepositoryByID(e Engine, id int64) (*Repository, error) {
	repo := new(Repository)
	has, err := e.ID(id).Get(repo)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrRepoNotExist{id, 0, "", ""}
	}
	return repo, nil
}

// GetRepositoryByID returns the repository by given id if exists.
func GetRepositoryByID(id int64) (*Repository, error) {
	return getRepositoryByID(x, id)
}

// GetRepositoriesMapByIDs returns the repositories by given id slice.
func GetRepositoriesMapByIDs(ids []int64) (map[int64]*Repository, error) {
	var repos = make(map[int64]*Repository, len(ids))
	return repos, x.In("id", ids).Find(&repos)
}

// GetUserRepositories returns a list of repositories of given user.
func GetUserRepositories(userID int64, private bool, page, pageSize int, orderBy string) ([]*Repository, error) {
	if len(orderBy) == 0 {
		orderBy = "updated_unix DESC"
	}

	sess := x.
		Where("owner_id = ?", userID).
		OrderBy(orderBy)
	if !private {
		sess.And("is_private=?", false)
	}

	if page <= 0 {
		page = 1
	}
	sess.Limit(pageSize, (page-1)*pageSize)

	repos := make([]*Repository, 0, pageSize)
	return repos, sess.Find(&repos)
}

// GetUserMirrorRepositories returns a list of mirror repositories of given user.
func GetUserMirrorRepositories(userID int64) ([]*Repository, error) {
	repos := make([]*Repository, 0, 10)
	return repos, x.
		Where("owner_id = ?", userID).
		And("is_mirror = ?", true).
		Find(&repos)
}

func getRepositoryCount(e Engine, u *User) (int64, error) {
	return e.Count(&Repository{OwnerID: u.ID})
}

func getPublicRepositoryCount(e Engine, u *User) (int64, error) {
	return e.Where("is_private = ?", false).Count(&Repository{OwnerID: u.ID})
}

func getPrivateRepositoryCount(e Engine, u *User) (int64, error) {
	return e.Where("is_private = ?", true).Count(&Repository{OwnerID: u.ID})
}

// GetRepositoryCount returns the total number of repositories of user.
func GetRepositoryCount(u *User) (int64, error) {
	return getRepositoryCount(x, u)
}

// GetPublicRepositoryCount returns the total number of public repositories of user.
func GetPublicRepositoryCount(u *User) (int64, error) {
	return getPublicRepositoryCount(x, u)
}

// GetPrivateRepositoryCount returns the total number of private repositories of user.
func GetPrivateRepositoryCount(u *User) (int64, error) {
	return getPrivateRepositoryCount(x, u)
}

// DeleteRepositoryArchives deletes all repositories' archives.
func DeleteRepositoryArchives() error {
	return x.
		Where("id > 0").
		Iterate(new(Repository),
			func(idx int, bean interface{}) error {
				repo := bean.(*Repository)
				return os.RemoveAll(filepath.Join(repo.RepoPath(), "archives"))
			})
}

// DeleteOldRepositoryArchives deletes old repository archives.
func DeleteOldRepositoryArchives() {
	log.Trace("Doing: ArchiveCleanup")

	if err := x.Where("id > 0").Iterate(new(Repository), deleteOldRepositoryArchives); err != nil {
		log.Error("ArchiveClean: %v", err)
	}
}

func deleteOldRepositoryArchives(idx int, bean interface{}) error {
	repo := bean.(*Repository)
	basePath := filepath.Join(repo.RepoPath(), "archives")

	for _, ty := range []string{"zip", "targz"} {
		path := filepath.Join(basePath, ty)
		file, err := os.Open(path)
		if err != nil {
			if !os.IsNotExist(err) {
				log.Warn("Unable to open directory %s: %v", path, err)
				return err
			}

			// If the directory doesn't exist, that's okay.
			continue
		}

		files, err := file.Readdir(0)
		file.Close()
		if err != nil {
			log.Warn("Unable to read directory %s: %v", path, err)
			return err
		}

		minimumOldestTime := time.Now().Add(-setting.Cron.ArchiveCleanup.OlderThan)
		for _, info := range files {
			if info.ModTime().Before(minimumOldestTime) && !info.IsDir() {
				toDelete := filepath.Join(path, info.Name())
				// This is a best-effort purge, so we do not check error codes to confirm removal.
				if err = os.Remove(toDelete); err != nil {
					log.Trace("Unable to delete %s, but proceeding: %v", toDelete, err)
				}
			}
		}
	}

	return nil
}

func gatherMissingRepoRecords() ([]*Repository, error) {
	repos := make([]*Repository, 0, 10)
	if err := x.
		Where("id > 0").
		Iterate(new(Repository),
			func(idx int, bean interface{}) error {
				repo := bean.(*Repository)
				if !com.IsDir(repo.RepoPath()) {
					repos = append(repos, repo)
				}
				return nil
			}); err != nil {
		if err2 := CreateRepositoryNotice(fmt.Sprintf("gatherMissingRepoRecords: %v", err)); err2 != nil {
			return nil, fmt.Errorf("CreateRepositoryNotice: %v", err)
		}
	}
	return repos, nil
}

// DeleteMissingRepositories deletes all repository records that lost Git files.
func DeleteMissingRepositories(doer *User) error {
	repos, err := gatherMissingRepoRecords()
	if err != nil {
		return fmt.Errorf("gatherMissingRepoRecords: %v", err)
	}

	if len(repos) == 0 {
		return nil
	}

	for _, repo := range repos {
		log.Trace("Deleting %d/%d...", repo.OwnerID, repo.ID)
		if err := DeleteRepository(doer, repo.OwnerID, repo.ID); err != nil {
			if err2 := CreateRepositoryNotice(fmt.Sprintf("DeleteRepository [%d]: %v", repo.ID, err)); err2 != nil {
				return fmt.Errorf("CreateRepositoryNotice: %v", err)
			}
		}
	}
	return nil
}

// ReinitMissingRepositories reinitializes all repository records that lost Git files.
func ReinitMissingRepositories() error {
	repos, err := gatherMissingRepoRecords()
	if err != nil {
		return fmt.Errorf("gatherMissingRepoRecords: %v", err)
	}

	if len(repos) == 0 {
		return nil
	}

	for _, repo := range repos {
		log.Trace("Initializing %d/%d...", repo.OwnerID, repo.ID)
		if err := git.InitRepository(repo.RepoPath(), true); err != nil {
			if err2 := CreateRepositoryNotice(fmt.Sprintf("InitRepository [%d]: %v", repo.ID, err)); err2 != nil {
				return fmt.Errorf("CreateRepositoryNotice: %v", err)
			}
		}
	}
	return nil
}

// SyncRepositoryHooks rewrites all repositories' pre-receive, update and post-receive hooks
// to make sure the binary and custom conf path are up-to-date.
func SyncRepositoryHooks() error {
	return x.Cols("owner_id", "name").Where("id > 0").Iterate(new(Repository),
		func(idx int, bean interface{}) error {
			if err := createDelegateHooks(bean.(*Repository).RepoPath()); err != nil {
				return fmt.Errorf("SyncRepositoryHook: %v", err)
			}
			if bean.(*Repository).HasWiki() {
				if err := createDelegateHooks(bean.(*Repository).WikiPath()); err != nil {
					return fmt.Errorf("SyncRepositoryHook: %v", err)
				}
			}
			return nil
		})
}

// GitFsck calls 'git fsck' to check repository health.
func GitFsck() {
	log.Trace("Doing: GitFsck")

	if err := x.
		Where("id>0 AND is_fsck_enabled=?", true).BufferSize(setting.Database.IterateBufferSize).
		Iterate(new(Repository),
			func(idx int, bean interface{}) error {
				repo := bean.(*Repository)
				repoPath := repo.RepoPath()
				log.Trace("Running health check on repository %s", repoPath)
				if err := git.Fsck(repoPath, setting.Cron.RepoHealthCheck.Timeout, setting.Cron.RepoHealthCheck.Args...); err != nil {
					desc := fmt.Sprintf("Failed to health check repository (%s): %v", repoPath, err)
					log.Warn(desc)
					if err = CreateRepositoryNotice(desc); err != nil {
						log.Error("CreateRepositoryNotice: %v", err)
					}
				}
				return nil
			}); err != nil {
		log.Error("GitFsck: %v", err)
	}
	log.Trace("Finished: GitFsck")
}

// GitGcRepos calls 'git gc' to remove unnecessary files and optimize the local repository
func GitGcRepos() error {
	args := append([]string{"gc"}, setting.Git.GCArgs...)
	return x.
		Where("id > 0").BufferSize(setting.Database.IterateBufferSize).
		Iterate(new(Repository),
			func(idx int, bean interface{}) error {
				repo := bean.(*Repository)
				if err := repo.GetOwner(); err != nil {
					return err
				}
				_, stderr, err := process.GetManager().ExecDir(
					time.Duration(setting.Git.Timeout.GC)*time.Second,
					RepoPath(repo.Owner.Name, repo.Name), "Repository garbage collection",
					git.GitExecutable, args...)
				if err != nil {
					return fmt.Errorf("%v: %v", err, stderr)
				}
				return nil
			})
}

type repoChecker struct {
	querySQL, correctSQL string
	desc                 string
}

func repoStatsCheck(checker *repoChecker) {
	results, err := x.Query(checker.querySQL)
	if err != nil {
		log.Error("Select %s: %v", checker.desc, err)
		return
	}
	for _, result := range results {
		id := com.StrTo(result["id"]).MustInt64()
		log.Trace("Updating %s: %d", checker.desc, id)
		_, err = x.Exec(checker.correctSQL, id, id)
		if err != nil {
			log.Error("Update %s[%d]: %v", checker.desc, id, err)
		}
	}
}

// CheckRepoStats checks the repository stats
func CheckRepoStats() {
	log.Trace("Doing: CheckRepoStats")

	checkers := []*repoChecker{
		// Repository.NumWatches
		{
			"SELECT repo.id FROM `repository` repo WHERE repo.num_watches!=(SELECT COUNT(*) FROM `watch` WHERE repo_id=repo.id)",
			"UPDATE `repository` SET num_watches=(SELECT COUNT(*) FROM `watch` WHERE repo_id=?) WHERE id=?",
			"repository count 'num_watches'",
		},
		// Repository.NumStars
		{
			"SELECT repo.id FROM `repository` repo WHERE repo.num_stars!=(SELECT COUNT(*) FROM `star` WHERE repo_id=repo.id)",
			"UPDATE `repository` SET num_stars=(SELECT COUNT(*) FROM `star` WHERE repo_id=?) WHERE id=?",
			"repository count 'num_stars'",
		},
		// Label.NumIssues
		{
			"SELECT label.id FROM `label` WHERE label.num_issues!=(SELECT COUNT(*) FROM `issue_label` WHERE label_id=label.id)",
			"UPDATE `label` SET num_issues=(SELECT COUNT(*) FROM `issue_label` WHERE label_id=?) WHERE id=?",
			"label count 'num_issues'",
		},
		// User.NumRepos
		{
			"SELECT `user`.id FROM `user` WHERE `user`.num_repos!=(SELECT COUNT(*) FROM `repository` WHERE owner_id=`user`.id)",
			"UPDATE `user` SET num_repos=(SELECT COUNT(*) FROM `repository` WHERE owner_id=?) WHERE id=?",
			"user count 'num_repos'",
		},
		// Issue.NumComments
		{
			"SELECT `issue`.id FROM `issue` WHERE `issue`.num_comments!=(SELECT COUNT(*) FROM `comment` WHERE issue_id=`issue`.id AND type=0)",
			"UPDATE `issue` SET num_comments=(SELECT COUNT(*) FROM `comment` WHERE issue_id=? AND type=0) WHERE id=?",
			"issue count 'num_comments'",
		},
	}
	for i := range checkers {
		repoStatsCheck(checkers[i])
	}

	// ***** START: Repository.NumClosedIssues *****
	desc := "repository count 'num_closed_issues'"
	results, err := x.Query("SELECT repo.id FROM `repository` repo WHERE repo.num_closed_issues!=(SELECT COUNT(*) FROM `issue` WHERE repo_id=repo.id AND is_closed=? AND is_pull=?)", true, false)
	if err != nil {
		log.Error("Select %s: %v", desc, err)
	} else {
		for _, result := range results {
			id := com.StrTo(result["id"]).MustInt64()
			log.Trace("Updating %s: %d", desc, id)
			_, err = x.Exec("UPDATE `repository` SET num_closed_issues=(SELECT COUNT(*) FROM `issue` WHERE repo_id=? AND is_closed=? AND is_pull=?) WHERE id=?", id, true, false, id)
			if err != nil {
				log.Error("Update %s[%d]: %v", desc, id, err)
			}
		}
	}
	// ***** END: Repository.NumClosedIssues *****

	// ***** START: Repository.NumClosedPulls *****
	desc = "repository count 'num_closed_pulls'"
	results, err = x.Query("SELECT repo.id FROM `repository` repo WHERE repo.num_closed_pulls!=(SELECT COUNT(*) FROM `issue` WHERE repo_id=repo.id AND is_closed=? AND is_pull=?)", true, true)
	if err != nil {
		log.Error("Select %s: %v", desc, err)
	} else {
		for _, result := range results {
			id := com.StrTo(result["id"]).MustInt64()
			log.Trace("Updating %s: %d", desc, id)
			_, err = x.Exec("UPDATE `repository` SET num_closed_pulls=(SELECT COUNT(*) FROM `issue` WHERE repo_id=? AND is_closed=? AND is_pull=?) WHERE id=?", id, true, true, id)
			if err != nil {
				log.Error("Update %s[%d]: %v", desc, id, err)
			}
		}
	}
	// ***** END: Repository.NumClosedPulls *****

	// FIXME: use checker when stop supporting old fork repo format.
	// ***** START: Repository.NumForks *****
	results, err = x.Query("SELECT repo.id FROM `repository` repo WHERE repo.num_forks!=(SELECT COUNT(*) FROM `repository` WHERE fork_id=repo.id)")
	if err != nil {
		log.Error("Select repository count 'num_forks': %v", err)
	} else {
		for _, result := range results {
			id := com.StrTo(result["id"]).MustInt64()
			log.Trace("Updating repository count 'num_forks': %d", id)

			repo, err := GetRepositoryByID(id)
			if err != nil {
				log.Error("GetRepositoryByID[%d]: %v", id, err)
				continue
			}

			rawResult, err := x.Query("SELECT COUNT(*) FROM `repository` WHERE fork_id=?", repo.ID)
			if err != nil {
				log.Error("Select count of forks[%d]: %v", repo.ID, err)
				continue
			}
			repo.NumForks = int(parseCountResult(rawResult))

			if err = UpdateRepository(repo, false); err != nil {
				log.Error("UpdateRepository[%d]: %v", id, err)
				continue
			}
		}
	}
	// ***** END: Repository.NumForks *****
}

// SetArchiveRepoState sets if a repo is archived
func (repo *Repository) SetArchiveRepoState(isArchived bool) (err error) {
	repo.IsArchived = isArchived
	_, err = x.Where("id = ?", repo.ID).Cols("is_archived").Update(repo)
	return
}

// ___________           __
// \_   _____/__________|  | __
//  |    __)/  _ \_  __ \  |/ /
//  |     \(  <_> )  | \/    <
//  \___  / \____/|__|  |__|_ \
//      \/                   \/

// HasForkedRepo checks if given user has already forked a repository with given ID.
func HasForkedRepo(ownerID, repoID int64) (*Repository, bool) {
	repo := new(Repository)
	has, _ := x.
		Where("owner_id=? AND fork_id=?", ownerID, repoID).
		Get(repo)
	return repo, has
}

// ForkRepository forks a repository
func ForkRepository(doer, u *User, oldRepo *Repository, name, desc string) (_ *Repository, err error) {
	forkedRepo, err := oldRepo.GetUserFork(u.ID)
	if err != nil {
		return nil, err
	}
	if forkedRepo != nil {
		return nil, ErrForkAlreadyExist{
			Uname:    u.Name,
			RepoName: oldRepo.FullName(),
			ForkName: forkedRepo.FullName(),
		}
	}

	repo := &Repository{
		OwnerID:       u.ID,
		Owner:         u,
		Name:          name,
		LowerName:     strings.ToLower(name),
		Description:   desc,
		DefaultBranch: oldRepo.DefaultBranch,
		IsPrivate:     oldRepo.IsPrivate,
		IsEmpty:       oldRepo.IsEmpty,
		IsFork:        true,
		ForkID:        oldRepo.ID,
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return nil, err
	}

	if err = createRepository(sess, doer, u, repo); err != nil {
		return nil, err
	}

	if _, err = sess.Exec("UPDATE `repository` SET num_forks=num_forks+1 WHERE id=?", oldRepo.ID); err != nil {
		return nil, err
	}

	repoPath := RepoPath(u.Name, repo.Name)
	_, stderr, err := process.GetManager().ExecTimeout(10*time.Minute,
		fmt.Sprintf("ForkRepository(git clone): %s/%s", u.Name, repo.Name),
		git.GitExecutable, "clone", "--bare", oldRepo.repoPath(sess), repoPath)
	if err != nil {
		return nil, fmt.Errorf("git clone: %v", stderr)
	}

	_, stderr, err = process.GetManager().ExecDir(-1,
		repoPath, fmt.Sprintf("ForkRepository(git update-server-info): %s", repoPath),
		git.GitExecutable, "update-server-info")
	if err != nil {
		return nil, fmt.Errorf("git update-server-info: %v", stderr)
	}

	if err = createDelegateHooks(repoPath); err != nil {
		return nil, fmt.Errorf("createDelegateHooks: %v", err)
	}

	//Commit repo to get Fork ID
	err = sess.Commit()
	if err != nil {
		return nil, err
	}

	oldMode, _ := AccessLevel(doer, oldRepo)
	mode, _ := AccessLevel(doer, repo)

	if err = PrepareWebhooks(oldRepo, HookEventFork, &api.ForkPayload{
		Forkee: oldRepo.APIFormat(oldMode),
		Repo:   repo.APIFormat(mode),
		Sender: doer.APIFormat(),
	}); err != nil {
		log.Error("PrepareWebhooks [repo_id: %d]: %v", oldRepo.ID, err)
	} else {
		go HookQueue.Add(oldRepo.ID)
	}

	// Add to hook queue for created repo after session commit.
	if u.IsOrganization() {
		go HookQueue.Add(repo.ID)
	}

	if err = repo.UpdateSize(); err != nil {
		log.Error("Failed to update size for repository: %v", err)
	}

	// Copy LFS meta objects in new session
	sess2 := x.NewSession()
	defer sess2.Close()
	if err = sess2.Begin(); err != nil {
		return nil, err
	}

	var lfsObjects []*LFSMetaObject

	if err = sess2.Where("repository_id=?", oldRepo.ID).Find(&lfsObjects); err != nil {
		return nil, err
	}

	for _, v := range lfsObjects {
		v.ID = 0
		v.RepositoryID = repo.ID
		if _, err = sess2.Insert(v); err != nil {
			return nil, err
		}
	}

	return repo, sess2.Commit()
}

// GetForks returns all the forks of the repository
func (repo *Repository) GetForks() ([]*Repository, error) {
	forks := make([]*Repository, 0, repo.NumForks)
	return forks, x.Find(&forks, &Repository{ForkID: repo.ID})
}

// GetUserFork return user forked repository from this repository, if not forked return nil
func (repo *Repository) GetUserFork(userID int64) (*Repository, error) {
	var forkedRepo Repository
	has, err := x.Where("fork_id = ?", repo.ID).And("owner_id = ?", userID).Get(&forkedRepo)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, nil
	}
	return &forkedRepo, nil
}

// CustomAvatarPath returns repository custom avatar file path.
func (repo *Repository) CustomAvatarPath() string {
	// Avatar empty by default
	if len(repo.Avatar) == 0 {
		return ""
	}
	return filepath.Join(setting.RepositoryAvatarUploadPath, repo.Avatar)
}

// generateRandomAvatar generates a random avatar for repository.
func (repo *Repository) generateRandomAvatar(e Engine) error {
	idToString := fmt.Sprintf("%d", repo.ID)

	seed := idToString
	img, err := avatar.RandomImage([]byte(seed))
	if err != nil {
		return fmt.Errorf("RandomImage: %v", err)
	}

	repo.Avatar = idToString
	if err = os.MkdirAll(filepath.Dir(repo.CustomAvatarPath()), os.ModePerm); err != nil {
		return fmt.Errorf("MkdirAll: %v", err)
	}
	fw, err := os.Create(repo.CustomAvatarPath())
	if err != nil {
		return fmt.Errorf("Create: %v", err)
	}
	defer fw.Close()

	if err = png.Encode(fw, img); err != nil {
		return fmt.Errorf("Encode: %v", err)
	}
	log.Info("New random avatar created for repository: %d", repo.ID)

	if _, err := e.ID(repo.ID).Cols("avatar").NoAutoTime().Update(repo); err != nil {
		return err
	}

	return nil
}

// RemoveRandomAvatars removes the randomly generated avatars that were created for repositories
func RemoveRandomAvatars() error {
	return x.
		Where("id > 0").BufferSize(setting.Database.IterateBufferSize).
		Iterate(new(Repository),
			func(idx int, bean interface{}) error {
				repository := bean.(*Repository)
				stringifiedID := strconv.FormatInt(repository.ID, 10)
				if repository.Avatar == stringifiedID {
					return repository.DeleteAvatar()
				}
				return nil
			})
}

// RelAvatarLink returns a relative link to the repository's avatar.
func (repo *Repository) RelAvatarLink() string {
	return repo.relAvatarLink(x)
}

func (repo *Repository) relAvatarLink(e Engine) string {
	// If no avatar - path is empty
	avatarPath := repo.CustomAvatarPath()
	if len(avatarPath) == 0 || !com.IsFile(avatarPath) {
		switch mode := setting.RepositoryAvatarFallback; mode {
		case "image":
			return setting.RepositoryAvatarFallbackImage
		case "random":
			if err := repo.generateRandomAvatar(e); err != nil {
				log.Error("generateRandomAvatar: %v", err)
			}
		default:
			// default behaviour: do not display avatar
			return ""
		}
	}
	return setting.AppSubURL + "/repo-avatars/" + repo.Avatar
}

// avatarLink returns user avatar absolute link.
func (repo *Repository) avatarLink(e Engine) string {
	link := repo.relAvatarLink(e)
	// link may be empty!
	if len(link) > 0 {
		if link[0] == '/' && link[1] != '/' {
			return setting.AppURL + strings.TrimPrefix(link, setting.AppSubURL)[1:]
		}
	}
	return link
}

// UploadAvatar saves custom avatar for repository.
// FIXME: split uploads to different subdirs in case we have massive number of repos.
func (repo *Repository) UploadAvatar(data []byte) error {
	m, err := avatar.Prepare(data)
	if err != nil {
		return err
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	oldAvatarPath := repo.CustomAvatarPath()

	// Users can upload the same image to other repo - prefix it with ID
	// Then repo will be removed - only it avatar file will be removed
	repo.Avatar = fmt.Sprintf("%d-%x", repo.ID, md5.Sum(data))
	if _, err := sess.ID(repo.ID).Cols("avatar").Update(repo); err != nil {
		return fmt.Errorf("UploadAvatar: Update repository avatar: %v", err)
	}

	if err := os.MkdirAll(setting.RepositoryAvatarUploadPath, os.ModePerm); err != nil {
		return fmt.Errorf("UploadAvatar: Failed to create dir %s: %v", setting.RepositoryAvatarUploadPath, err)
	}

	fw, err := os.Create(repo.CustomAvatarPath())
	if err != nil {
		return fmt.Errorf("UploadAvatar: Create file: %v", err)
	}
	defer fw.Close()

	if err = png.Encode(fw, *m); err != nil {
		return fmt.Errorf("UploadAvatar: Encode png: %v", err)
	}

	if len(oldAvatarPath) > 0 && oldAvatarPath != repo.CustomAvatarPath() {
		if err := os.Remove(oldAvatarPath); err != nil {
			return fmt.Errorf("UploadAvatar: Failed to remove old repo avatar %s: %v", oldAvatarPath, err)
		}
	}

	return sess.Commit()
}

// DeleteAvatar deletes the repos's custom avatar.
func (repo *Repository) DeleteAvatar() error {

	// Avatar not exists
	if len(repo.Avatar) == 0 {
		return nil
	}

	avatarPath := repo.CustomAvatarPath()
	log.Trace("DeleteAvatar[%d]: %s", repo.ID, avatarPath)

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	repo.Avatar = ""
	if _, err := sess.ID(repo.ID).Cols("avatar").Update(repo); err != nil {
		return fmt.Errorf("DeleteAvatar: Update repository avatar: %v", err)
	}

	if _, err := os.Stat(avatarPath); err == nil {
		if err := os.Remove(avatarPath); err != nil {
			return fmt.Errorf("DeleteAvatar: Failed to remove %s: %v", avatarPath, err)
		}
	} else {
		// // Schrodinger: file may or may not exist. See err for details.
		log.Trace("DeleteAvatar[%d]: %v", err)
	}
	return sess.Commit()
}

// GetOriginalURLHostname returns the hostname of a URL or the URL
func (repo *Repository) GetOriginalURLHostname() string {
	u, err := url.Parse(repo.OriginalURL)
	if err != nil {
		return repo.OriginalURL
	}

	return u.Host
}
