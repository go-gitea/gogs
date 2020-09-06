// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"html/template"
	"unicode/utf8"

	// Needed for jpeg support
	_ "image/jpeg"
	"image/png"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/avatar"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/options"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"github.com/unknwon/com"
	"xorm.io/builder"
)

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

	// LabelTemplates contains the label template files and the list of labels for each file
	LabelTemplates map[string]string

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
	LabelTemplatesFiles := typeFiles[3]
	sort.Strings(Gitignores)
	sort.Strings(Licenses)
	sort.Strings(Readmes)
	sort.Strings(LabelTemplatesFiles)

	// Load label templates
	LabelTemplates = make(map[string]string)
	for _, templateFile := range LabelTemplatesFiles {
		labels, err := LoadLabelsFormatted(templateFile)
		if err != nil {
			log.Error("Failed to load labels: %v", err)
		}
		LabelTemplates[templateFile] = labels
	}

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
	loadUnitConfig()

	RemoveAllWithNotice("Clean up repository temporary data", filepath.Join(setting.AppDataPath, "tmp"))
}

// RepositoryStatus defines the status of repository
type RepositoryStatus int

// all kinds of RepositoryStatus
const (
	RepositoryReady         RepositoryStatus = iota // a normal repository
	RepositoryBeingMigrated                         // repository is migrating
)

// Repository represents a git repository.
type Repository struct {
	ID                  int64 `xorm:"pk autoincr"`
	OwnerID             int64 `xorm:"UNIQUE(s) index"`
	OwnerName           string
	Owner               *User              `xorm:"-"`
	LowerName           string             `xorm:"UNIQUE(s) INDEX NOT NULL"`
	Name                string             `xorm:"INDEX NOT NULL"`
	Description         string             `xorm:"TEXT"`
	Website             string             `xorm:"VARCHAR(2048)"`
	OriginalServiceType api.GitServiceType `xorm:"index"`
	OriginalURL         string             `xorm:"VARCHAR(2048)"`
	DefaultBranch       string

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
	NumProjects         int `xorm:"NOT NULL DEFAULT 0"`
	NumClosedProjects   int `xorm:"NOT NULL DEFAULT 0"`
	NumOpenProjects     int `xorm:"-"`

	IsPrivate  bool `xorm:"INDEX"`
	IsEmpty    bool `xorm:"INDEX"`
	IsArchived bool `xorm:"INDEX"`
	IsMirror   bool `xorm:"INDEX"`
	*Mirror    `xorm:"-"`
	Status     RepositoryStatus `xorm:"NOT NULL DEFAULT 0"`

	RenderingMetas         map[string]string `xorm:"-"`
	DocumentRenderingMetas map[string]string `xorm:"-"`
	Units                  []*RepoUnit       `xorm:"-"`
	PrimaryLanguage        *LanguageStat     `xorm:"-"`

	IsFork                          bool               `xorm:"INDEX NOT NULL DEFAULT false"`
	ForkID                          int64              `xorm:"INDEX"`
	BaseRepo                        *Repository        `xorm:"-"`
	IsTemplate                      bool               `xorm:"INDEX NOT NULL DEFAULT false"`
	TemplateID                      int64              `xorm:"INDEX"`
	TemplateRepo                    *Repository        `xorm:"-"`
	Size                            int64              `xorm:"NOT NULL DEFAULT 0"`
	CodeIndexerStatus               *RepoIndexerStatus `xorm:"-"`
	StatsIndexerStatus              *RepoIndexerStatus `xorm:"-"`
	IsFsckEnabled                   bool               `xorm:"NOT NULL DEFAULT true"`
	CloseIssuesViaCommitInAnyBranch bool               `xorm:"NOT NULL DEFAULT false"`
	Topics                          []string           `xorm:"TEXT JSON"`

	// Avatar: ID(10-20)-md5(32) - must fit into 64 symbols
	Avatar string `xorm:"VARCHAR(64)"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

// TableName sets the table name to `repository`
func (repo *Repository) TableName() string {
	return tbRepository[1 : len(tbRepository)-1]
}

// SanitizedOriginalURL returns a sanitized OriginalURL
func (repo *Repository) SanitizedOriginalURL() string {
	if repo.OriginalURL == "" {
		return ""
	}
	return util.SanitizeURLCredentials(repo.OriginalURL, false)
}

// ColorFormat returns a colored string to represent this repo
func (repo *Repository) ColorFormat(s fmt.State) {
	log.ColorFprintf(s, "%d:%s/%s",
		log.NewColoredIDValue(repo.ID),
		repo.OwnerName,
		repo.Name)
}

// IsBeingMigrated indicates that repository is being migtated
func (repo *Repository) IsBeingMigrated() bool {
	return repo.Status == RepositoryBeingMigrated
}

// IsBeingCreated indicates that repository is being migrated or forked
func (repo *Repository) IsBeingCreated() bool {
	return repo.IsBeingMigrated()
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
	repo.NumOpenProjects = repo.NumProjects - repo.NumClosedProjects
}

// MustOwner always returns a valid *User object to avoid
// conceptually impossible error handling.
// It creates a fake object that contains error details
// when error occurs.
func (repo *Repository) MustOwner() *User {
	return repo.mustOwner(x)
}

// FullName returns the repository full name
func (repo *Repository) FullName() string {
	return repo.OwnerName + "/" + repo.Name
}

// HTMLURL returns the repository HTML URL
func (repo *Repository) HTMLURL() string {
	return setting.AppURL + repo.FullName()
}

// CommitLink make link to by commit full ID
// note: won't check whether it's an right id
func (repo *Repository) CommitLink(commitID string) (result string) {
	if commitID == "" || commitID == "0000000000000000000000000000000000000000" {
		result = ""
	} else {
		result = repo.HTMLURL() + "/commit/" + commitID
	}
	return
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

	cloneLink := repo.cloneLink(false)
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

	//check enabled/disabled units
	hasIssues := false
	var externalTracker *api.ExternalTracker
	var internalTracker *api.InternalTracker
	if unit, err := repo.getUnit(e, UnitTypeIssues); err == nil {
		config := unit.IssuesConfig()
		hasIssues = true
		internalTracker = &api.InternalTracker{
			EnableTimeTracker:                config.EnableTimetracker,
			AllowOnlyContributorsToTrackTime: config.AllowOnlyContributorsToTrackTime,
			EnableIssueDependencies:          config.EnableDependencies,
		}
	} else if unit, err := repo.getUnit(e, UnitTypeExternalTracker); err == nil {
		config := unit.ExternalTrackerConfig()
		hasIssues = true
		externalTracker = &api.ExternalTracker{
			ExternalTrackerURL:    config.ExternalTrackerURL,
			ExternalTrackerFormat: config.ExternalTrackerFormat,
			ExternalTrackerStyle:  config.ExternalTrackerStyle,
		}
	}
	hasWiki := false
	var externalWiki *api.ExternalWiki
	if _, err := repo.getUnit(e, UnitTypeWiki); err == nil {
		hasWiki = true
	} else if unit, err := repo.getUnit(e, UnitTypeExternalWiki); err == nil {
		hasWiki = true
		config := unit.ExternalWikiConfig()
		externalWiki = &api.ExternalWiki{
			ExternalWikiURL: config.ExternalWikiURL,
		}
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
	hasProjects := false
	if _, err := repo.getUnit(e, UnitTypeProjects); err == nil {
		hasProjects = true
	}

	repo.mustOwner(e)

	numReleases, _ := GetReleaseCountByRepoID(repo.ID, FindReleasesOptions{IncludeDrafts: false, IncludeTags: true})

	return &api.Repository{
		ID:                        repo.ID,
		Owner:                     repo.Owner.APIFormat(),
		Name:                      repo.Name,
		FullName:                  repo.FullName(),
		Description:               repo.Description,
		Private:                   repo.IsPrivate,
		Template:                  repo.IsTemplate,
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
		OpenPulls:                 repo.NumOpenPulls,
		Releases:                  int(numReleases),
		DefaultBranch:             repo.DefaultBranch,
		Created:                   repo.CreatedUnix.AsTime(),
		Updated:                   repo.UpdatedUnix.AsTime(),
		Permissions:               permission,
		HasIssues:                 hasIssues,
		ExternalTracker:           externalTracker,
		InternalTracker:           internalTracker,
		HasWiki:                   hasWiki,
		HasProjects:               hasProjects,
		ExternalWiki:              externalWiki,
		HasPullRequests:           hasPullRequests,
		IgnoreWhitespaceConflicts: ignoreWhitespaceConflicts,
		AllowMerge:                allowMerge,
		AllowRebase:               allowRebase,
		AllowRebaseMerge:          allowRebaseMerge,
		AllowSquash:               allowSquash,
		AvatarURL:                 repo.avatarLink(e),
		Internal:                  !repo.IsPrivate && repo.Owner.Visibility == api.VisibleTypePrivate,
	}
}

func (repo *Repository) getUnits(e Engine) (err error) {
	if repo.Units != nil {
		return nil
	}

	repo.Units, err = getUnitsByRepoID(e, repo.ID)
	log.Trace("repo.Units: %-+v", repo.Units)
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

// ComposeMetas composes a map of metas for properly rendering issue links and external issue trackers.
func (repo *Repository) ComposeMetas() map[string]string {
	if len(repo.RenderingMetas) == 0 {
		metas := map[string]string{
			"user":     repo.OwnerName,
			"repo":     repo.Name,
			"repoPath": repo.RepoPath(),
			"mode":     "comment",
		}

		unit, err := repo.GetUnit(UnitTypeExternalTracker)
		if err == nil {
			metas["format"] = unit.ExternalTrackerConfig().ExternalTrackerFormat
			switch unit.ExternalTrackerConfig().ExternalTrackerStyle {
			case markup.IssueNameStyleAlphanumeric:
				metas["style"] = markup.IssueNameStyleAlphanumeric
			default:
				metas["style"] = markup.IssueNameStyleNumeric
			}
		}

		repo.MustOwner()
		if repo.Owner.IsOrganization() {
			teams := make([]string, 0, 5)
			_ = x.Table(tbTeamRepo).
				Join("INNER", tbTeam, tbTeam+".id = "+tbTeamRepo+".team_id").
				Where(tbTeamRepo+".repo_id = ?", repo.ID).
				Select(tbTeam + ".lower_name").
				OrderBy(tbTeam + ".lower_name").
				Find(&teams)
			metas["teams"] = "," + strings.Join(teams, ",") + ","
			metas["org"] = strings.ToLower(repo.OwnerName)
		}

		repo.RenderingMetas = metas
	}
	return repo.RenderingMetas
}

// ComposeDocumentMetas composes a map of metas for properly rendering documents
func (repo *Repository) ComposeDocumentMetas() map[string]string {
	if len(repo.DocumentRenderingMetas) == 0 {
		metas := map[string]string{}
		for k, v := range repo.ComposeMetas() {
			metas[k] = v
		}
		metas["mode"] = "document"
		repo.DocumentRenderingMetas = metas
	}
	return repo.DocumentRenderingMetas
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

func (repo *Repository) getReviewersPrivate(e Engine, doerID, posterID int64) (users []*User, err error) {
	users = make([]*User, 0, 20)

	if err = e.
		SQL("SELECT * FROM "+tbUser+" WHERE id in (SELECT user_id FROM "+tbAccess+" WHERE repo_id = ? AND mode >= ? AND user_id NOT IN ( ?, ?)) ORDER BY name",
			repo.ID, AccessModeRead,
			doerID, posterID).
		Find(&users); err != nil {
		return nil, err
	}

	return users, nil
}

func (repo *Repository) getReviewersPublic(e Engine, doerID, posterID int64) (_ []*User, err error) {

	users := make([]*User, 0)

	SQLCmd := "SELECT * FROM " + tbUser + " WHERE id IN ( " +
		"SELECT user_id FROM " + tbAccess +
		" WHERE repo_id = ? AND mode >= ? AND user_id NOT IN ( ?, ?) " +
		"UNION " +
		"SELECT user_id FROM " + tbWatch +
		" WHERE repo_id = ? AND user_id NOT IN ( ?, ?) AND mode IN (?, ?) " +
		") ORDER BY name"

	if err = e.
		SQL(SQLCmd,
			repo.ID, AccessModeRead, doerID, posterID,
			repo.ID, doerID, posterID, RepoWatchModeNormal, RepoWatchModeAuto).
		Find(&users); err != nil {
		return nil, err
	}

	return users, nil
}

func (repo *Repository) getReviewers(e Engine, doerID, posterID int64) (users []*User, err error) {
	if err = repo.getOwner(e); err != nil {
		return nil, err
	}

	if repo.IsPrivate ||
		(repo.Owner.IsOrganization() && repo.Owner.Visibility == api.VisibleTypePrivate) {
		users, err = repo.getReviewersPrivate(x, doerID, posterID)
	} else {
		users, err = repo.getReviewersPublic(x, doerID, posterID)
	}
	return
}

// GetReviewers get all users can be requested to review
// for private rpo , that return all users that have read access or higher to the repository.
// but for public rpo, that return all users that have write access or higher to the repository,
// and all repo watchers.
// TODO: may be we should hava a busy choice for users to block review request to them.
func (repo *Repository) GetReviewers(doerID, posterID int64) (_ []*User, err error) {
	return repo.getReviewers(x, doerID, posterID)
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

// IsGenerated returns whether _this_ repository was generated from a template
func (repo *Repository) IsGenerated() bool {
	return repo.TemplateID != 0
}

// GetTemplateRepo populates repo.TemplateRepo for a generated repository and
// returns an error on failure (NOTE: no error is returned for
// non-generated repositories, and TemplateRepo will be left untouched)
func (repo *Repository) GetTemplateRepo() (err error) {
	return repo.getTemplateRepo(x)
}

func (repo *Repository) getTemplateRepo(e Engine) (err error) {
	if !repo.IsGenerated() {
		return nil
	}

	repo.TemplateRepo, err = getRepositoryByID(e, repo.TemplateID)
	return err
}

// RepoPath returns the repository path
func (repo *Repository) RepoPath() string {
	return RepoPath(repo.OwnerName, repo.Name)
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
	return fmt.Sprintf("%s/compare/%s...%s", repo.FullName(), oldCommitID, newCommitID)
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
	size, err := util.GetDirectorySize(repo.RepoPath())
	if err != nil {
		return fmt.Errorf("updateSize: %v", err)
	}

	objs, err := repo.GetLFSMetaObjects(-1, 0)
	if err != nil {
		return fmt.Errorf("updateSize: GetLFSMetaObjects: %v", err)
	}
	for _, obj := range objs {
		size += obj.Size
	}

	repo.Size = size
	_, err = e.ID(repo.ID).Cols("size").Update(repo)
	return err
}

// UpdateSize updates the repository size, calculating it using util.GetDirectorySize
func (repo *Repository) UpdateSize(ctx DBContext) error {
	return repo.updateSize(ctx.e)
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

// CanUserDelete returns true if user could delete the repository
func (repo *Repository) CanUserDelete(user *User) (bool, error) {
	if user.IsAdmin || user.ID == repo.OwnerID {
		return true, nil
	}

	if err := repo.GetOwner(); err != nil {
		return false, err
	}

	if repo.Owner.IsOrganization() {
		isOwner, err := repo.Owner.IsOwnedBy(user.ID)
		if err != nil {
			return false, err
		} else if isOwner {
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

// GetReaders returns all users that have explicit read access or higher to the repository.
func (repo *Repository) GetReaders() (_ []*User, err error) {
	return repo.getUsersWithAccessMode(x, AccessModeRead)
}

// GetWriters returns all users that have write access to the repository.
func (repo *Repository) GetWriters() (_ []*User, err error) {
	return repo.getUsersWithAccessMode(x, AccessModeWrite)
}

// IsReader returns true if user has explicit read access or higher to the repository.
func (repo *Repository) IsReader(userID int64) (bool, error) {
	if repo.OwnerID == userID {
		return true, nil
	}
	return x.Where("repo_id = ? AND user_id = ? AND mode >= ?", repo.ID, userID, AccessModeRead).Get(&Access{})
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

func (repo *Repository) cloneLink(isWiki bool) *CloneLink {
	repoName := repo.Name
	if isWiki {
		repoName += ".wiki"
	}

	sshUser := setting.RunUser
	if setting.SSH.StartBuiltinServer {
		sshUser = setting.SSH.BuiltinServerUser
	}

	cl := new(CloneLink)

	// if we have a ipv6 literal we need to put brackets around it
	// for the git cloning to work.
	sshDomain := setting.SSH.Domain
	ip := net.ParseIP(setting.SSH.Domain)
	if ip != nil && ip.To4() == nil {
		sshDomain = "[" + setting.SSH.Domain + "]"
	}

	if setting.SSH.Port != 22 {
		cl.SSH = fmt.Sprintf("ssh://%s@%s/%s/%s.git", sshUser, net.JoinHostPort(setting.SSH.Domain, strconv.Itoa(setting.SSH.Port)), repo.OwnerName, repoName)
	} else if setting.Repository.UseCompatSSHURI {
		cl.SSH = fmt.Sprintf("ssh://%s@%s/%s/%s.git", sshUser, sshDomain, repo.OwnerName, repoName)
	} else {
		cl.SSH = fmt.Sprintf("%s@%s:%s/%s.git", sshUser, sshDomain, repo.OwnerName, repoName)
	}
	cl.HTTPS = ComposeHTTPSCloneURL(repo.OwnerName, repoName)
	return cl
}

// CloneLink returns clone URLs of repository.
func (repo *Repository) CloneLink() (cl *CloneLink) {
	return repo.cloneLink(false)
}

// CheckCreateRepository check if could created a repository
func CheckCreateRepository(doer, u *User, name string) error {
	if !doer.CanCreateRepo() {
		return ErrReachLimitOfRepo{u.MaxRepoCreation}
	}

	if err := IsUsableRepoName(name); err != nil {
		return err
	}

	has, err := isRepositoryExist(x, u, name)
	if err != nil {
		return fmt.Errorf("IsRepositoryExist: %v", err)
	} else if has {
		return ErrRepoAlreadyExist{u.Name, name}
	}
	return nil
}

// CreateRepoOptions contains the create repository options
type CreateRepoOptions struct {
	Name           string
	Description    string
	OriginalURL    string
	GitServiceType api.GitServiceType
	Gitignores     string
	IssueLabels    string
	License        string
	Readme         string
	DefaultBranch  string
	IsPrivate      bool
	IsMirror       bool
	AutoInit       bool
	Status         RepositoryStatus
}

// GetRepoInitFile returns repository init files
func GetRepoInitFile(tp, name string) ([]byte, error) {
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

var (
	reservedRepoNames    = []string{".", ".."}
	reservedRepoPatterns = []string{"*.git", "*.wiki"}
)

// IsUsableRepoName returns true when repository is usable
func IsUsableRepoName(name string) error {
	return isUsableName(reservedRepoNames, reservedRepoPatterns, name)
}

// CreateRepository creates a repository for the user/organization.
func CreateRepository(ctx DBContext, doer, u *User, repo *Repository) (err error) {
	if err = IsUsableRepoName(repo.Name); err != nil {
		return err
	}

	has, err := isRepositoryExist(ctx.e, u, repo.Name)
	if err != nil {
		return fmt.Errorf("IsRepositoryExist: %v", err)
	} else if has {
		return ErrRepoAlreadyExist{u.Name, repo.Name}
	}

	if _, err = ctx.e.Insert(repo); err != nil {
		return err
	}
	if err = deleteRepoRedirect(ctx.e, u.ID, repo.Name); err != nil {
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

	if _, err = ctx.e.Insert(&units); err != nil {
		return err
	}

	// Remember visibility preference.
	u.LastRepoVisibility = repo.IsPrivate
	if err = updateUserCols(ctx.e, u, "last_repo_visibility"); err != nil {
		return fmt.Errorf("updateUser: %v", err)
	}

	if _, err = ctx.e.Incr("num_repos").ID(u.ID).Update(new(User)); err != nil {
		return fmt.Errorf("increment user total_repos: %v", err)
	}
	u.NumRepos++

	// Give access to all members in teams with access to all repositories.
	if u.IsOrganization() {
		if err := u.GetTeams(&SearchTeamOptions{}); err != nil {
			return fmt.Errorf("GetTeams: %v", err)
		}
		for _, t := range u.Teams {
			if t.IncludesAllRepositories {
				if err := t.addRepository(ctx.e, repo); err != nil {
					return fmt.Errorf("addRepository: %v", err)
				}
			}
		}

		if isAdmin, err := isUserRepoAdmin(ctx.e, repo, doer); err != nil {
			return fmt.Errorf("isUserRepoAdmin: %v", err)
		} else if !isAdmin {
			// Make creator repo admin if it wan't assigned automatically
			if err = repo.addCollaborator(ctx.e, doer); err != nil {
				return fmt.Errorf("AddCollaborator: %v", err)
			}
			if err = repo.changeCollaborationAccessMode(ctx.e, doer.ID, AccessModeAdmin); err != nil {
				return fmt.Errorf("ChangeCollaborationAccessMode: %v", err)
			}
		}
	} else if err = repo.recalculateAccesses(ctx.e); err != nil {
		// Organization automatically called this in addRepository method.
		return fmt.Errorf("recalculateAccesses: %v", err)
	}

	if setting.Service.AutoWatchNewRepos {
		if err = watchRepo(ctx.e, doer.ID, repo.ID, true); err != nil {
			return fmt.Errorf("watchRepo: %v", err)
		}
	}

	if err = copyDefaultWebhooksToRepo(ctx.e, repo.ID); err != nil {
		return fmt.Errorf("copyDefaultWebhooksToRepo: %v", err)
	}

	return nil
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

// IncrementRepoForkNum increment repository fork number
func IncrementRepoForkNum(ctx DBContext, repoID int64) error {
	_, err := ctx.e.Exec("UPDATE "+tbRepository+" SET num_forks=num_forks+1 WHERE id=?", repoID)
	return err
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

	oldOwner := repo.Owner

	// Note: we have to set value here to make sure recalculate accesses is based on
	// new owner.
	repo.OwnerID = newOwner.ID
	repo.Owner = newOwner
	repo.OwnerName = newOwner.Name

	// Update repository.
	if _, err := sess.ID(repo.ID).Update(repo); err != nil {
		return fmt.Errorf("update owner: %v", err)
	}

	// Remove redundant collaborators.
	collaborators, err := repo.getCollaborators(sess, ListOptions{})
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
	if oldOwner.IsOrganization() {
		if err = oldOwner.removeOrgRepo(sess, repo.ID); err != nil {
			return fmt.Errorf("removeOrgRepo: %v", err)
		}
	}

	if newOwner.IsOrganization() {
		if err := newOwner.getTeams(sess); err != nil {
			return fmt.Errorf("GetTeams: %v", err)
		}
		for _, t := range newOwner.Teams {
			if t.IncludesAllRepositories {
				if err := t.addRepository(sess, repo); err != nil {
					return fmt.Errorf("addRepository: %v", err)
				}
			}
		}
	} else if err = repo.recalculateAccesses(sess); err != nil {
		// Organization called this in addRepository method.
		return fmt.Errorf("recalculateAccesses: %v", err)
	}

	// Update repository count.
	if _, err = sess.Exec("UPDATE "+tbUser+" SET num_repos=num_repos+1 WHERE id=?", newOwner.ID); err != nil {
		return fmt.Errorf("increase new owner repository count: %v", err)
	} else if _, err = sess.Exec("UPDATE "+tbUser+" SET num_repos=num_repos-1 WHERE id=?", oldOwner.ID); err != nil {
		return fmt.Errorf("decrease old owner repository count: %v", err)
	}

	if err = watchRepo(sess, doer.ID, repo.ID, true); err != nil {
		return fmt.Errorf("watchRepo: %v", err)
	}

	// Remove watch for organization.
	if oldOwner.IsOrganization() {
		if err = watchRepo(sess, oldOwner.ID, repo.ID, false); err != nil {
			return fmt.Errorf("watchRepo [false]: %v", err)
		}
	}

	// Rename remote repository to new path and delete local copy.
	dir := UserPath(newOwner.Name)

	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return fmt.Errorf("Failed to create dir %s: %v", dir, err)
	}

	if err = os.Rename(RepoPath(oldOwner.Name, repo.Name), RepoPath(newOwner.Name, repo.Name)); err != nil {
		return fmt.Errorf("rename repository directory: %v", err)
	}

	// Rename remote wiki repository to new path and delete local copy.
	wikiPath := WikiPath(oldOwner.Name, repo.Name)
	if com.IsExist(wikiPath) {
		if err = os.Rename(wikiPath, WikiPath(newOwner.Name, repo.Name)); err != nil {
			return fmt.Errorf("rename repository wiki: %v", err)
		}
	}

	// If there was previously a redirect at this location, remove it.
	if err = deleteRepoRedirect(sess, newOwner.ID, repo.Name); err != nil {
		return fmt.Errorf("delete repo redirect: %v", err)
	}

	if err := NewRepoRedirect(DBContext{sess}, oldOwner.ID, repo.ID, repo.Name, repo.Name); err != nil {
		return fmt.Errorf("NewRepoRedirect: %v", err)
	}

	return sess.Commit()
}

// ChangeRepositoryName changes all corresponding setting from old repository name to new one.
func ChangeRepositoryName(doer *User, repo *Repository, newRepoName string) (err error) {
	oldRepoName := repo.Name
	newRepoName = strings.ToLower(newRepoName)
	if err = IsUsableRepoName(newRepoName); err != nil {
		return err
	}

	if err := repo.GetOwner(); err != nil {
		return err
	}

	has, err := IsRepositoryExist(repo.Owner, newRepoName)
	if err != nil {
		return fmt.Errorf("IsRepositoryExist: %v", err)
	} else if has {
		return ErrRepoAlreadyExist{repo.Owner.Name, newRepoName}
	}

	newRepoPath := RepoPath(repo.Owner.Name, newRepoName)
	if err = os.Rename(repo.RepoPath(), newRepoPath); err != nil {
		return fmt.Errorf("rename repository directory: %v", err)
	}

	wikiPath := repo.WikiPath()
	if com.IsExist(wikiPath) {
		if err = os.Rename(wikiPath, WikiPath(repo.Owner.Name, newRepoName)); err != nil {
			return fmt.Errorf("rename repository wiki: %v", err)
		}
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return fmt.Errorf("sess.Begin: %v", err)
	}

	// If there was previously a redirect at this location, remove it.
	if err = deleteRepoRedirect(sess, repo.OwnerID, newRepoName); err != nil {
		return fmt.Errorf("delete repo redirect: %v", err)
	}

	if err := NewRepoRedirect(DBContext{sess}, repo.Owner.ID, repo.ID, oldRepoName, newRepoName); err != nil {
		return err
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

	if utf8.RuneCountInString(repo.Description) > 255 {
		repo.Description = string([]rune(repo.Description)[:255])
	}
	if utf8.RuneCountInString(repo.Website) > 255 {
		repo.Website = string([]rune(repo.Website)[:255])
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
		daemonExportFile := path.Join(repo.RepoPath(), `git-daemon-export-ok`)
		if repo.IsPrivate && com.IsExist(daemonExportFile) {
			if err = util.Remove(daemonExportFile); err != nil {
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
			forkRepos[i].IsPrivate = repo.IsPrivate || repo.Owner.Visibility == api.VisibleTypePrivate
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

// UpdateRepositoryCtx updates a repository with db context
func UpdateRepositoryCtx(ctx DBContext, repo *Repository, visibilityChanged bool) error {
	return updateRepository(ctx.e, repo, visibilityChanged)
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

// UpdateRepositoryUpdatedTime updates a repository's updated time
func UpdateRepositoryUpdatedTime(repoID int64, updateTime time.Time) error {
	_, err := x.Exec("UPDATE "+tbRepository+" SET updated_unix = ? WHERE id = ?", updateTime.Unix(), repoID)
	return err
}

// UpdateRepositoryUnits updates a repository's units
func UpdateRepositoryUnits(repo *Repository, units []RepoUnit, deleteUnitTypes []UnitType) (err error) {
	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	// Delete existing settings of units before adding again
	for _, u := range units {
		deleteUnitTypes = append(deleteUnitTypes, u.Type)
	}

	if _, err = sess.Where("repo_id = ?", repo.ID).In("type", deleteUnitTypes).Delete(new(RepoUnit)); err != nil {
		return err
	}

	if len(units) > 0 {
		if _, err = sess.Insert(units); err != nil {
			return err
		}
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
		if err = org.GetTeams(&SearchTeamOptions{}); err != nil {
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
	deployKeys, err := listDeployKeys(sess, repo.ID, ListOptions{})
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

	attachments := make([]*Attachment, 0, 20)
	if err = sess.Join("INNER", tbRelease, tbRelease+".id = "+tbAttachment+".release_id").
		Where(tbRelease+".repo_id = ?", repoID).
		Find(&attachments); err != nil {
		return err
	}
	releaseAttachments := make([]string, 0, len(attachments))
	for i := 0; i < len(attachments); i++ {
		releaseAttachments = append(releaseAttachments, attachments[i].RelativePath())
	}

	if _, err = sess.Exec("UPDATE "+tbUser+" SET num_stars=num_stars-1 WHERE id IN (SELECT `uid` FROM "+tbStar+" WHERE repo_id = ?)", repo.ID); err != nil {
		return err
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
		&LanguageStat{RepoID: repoID},
		&Comment{RefRepoID: repoID},
		&Task{RepoID: repoID},
	); err != nil {
		return fmt.Errorf("deleteBeans: %v", err)
	}

	// Delete Issues and related objects
	var attachmentPaths []string
	if attachmentPaths, err = deleteIssuesByRepoID(sess, repoID); err != nil {
		return err
	}

	if _, err = sess.Where("repo_id = ?", repoID).Delete(new(RepoUnit)); err != nil {
		return err
	}

	if repo.IsFork {
		if _, err = sess.Exec("UPDATE "+tbRepository+" SET num_forks=num_forks-1 WHERE id=?", repo.ForkID); err != nil {
			return fmt.Errorf("decrease fork count: %v", err)
		}
	}

	if _, err = sess.Exec("UPDATE "+tbUser+" SET num_repos=num_repos-1 WHERE id=?", uid); err != nil {
		return err
	}

	if len(repo.Topics) > 0 {
		if err = removeTopicsFromRepo(sess, repo.ID); err != nil {
			return err
		}
	}

	projects, _, err := getProjects(sess, ProjectSearchOptions{
		RepoID: repoID,
	})
	if err != nil {
		return fmt.Errorf("get projects: %v", err)
	}
	for i := range projects {
		if err := deleteProjectByID(sess, projects[i].ID); err != nil {
			return fmt.Errorf("delete project [%d]: %v", projects[i].ID, err)
		}
	}

	// FIXME: Remove repository files should be executed after transaction succeed.
	repoPath := repo.RepoPath()
	removeAllWithNotice(sess, "Delete repository files", repoPath)

	err = repo.deleteWiki(sess)
	if err != nil {
		return err
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

		oidPath := filepath.Join(setting.LFS.ContentPath, v.Oid[0:2], v.Oid[2:4], v.Oid[4:len(v.Oid)])
		removeAllWithNotice(sess, "Delete orphaned LFS file", oidPath)
	}

	if _, err := sess.Delete(&LFSMetaObject{RepositoryID: repoID}); err != nil {
		return err
	}

	if repo.NumForks > 0 {
		if _, err = sess.Exec("UPDATE "+tbRepository+" SET fork_id=0,is_fork=? WHERE fork_id=?", false, repo.ID); err != nil {
			log.Error("reset 'fork_id' and 'is_fork': %v", err)
		}
	}

	if err = sess.Commit(); err != nil {
		sess.Close()
		if len(deployKeys) > 0 {
			// We need to rewrite the public keys because the commit failed
			if err2 := RewriteAllPublicKeys(); err2 != nil {
				return fmt.Errorf("Commit: %v SSH Keys: %v", err, err2)
			}
		}
		return fmt.Errorf("Commit: %v", err)
	}

	sess.Close()

	// We should always delete the files after the database transaction succeed. If
	// we delete the file but the database rollback, the repository will be borken.

	// Remove issue attachment files.
	for i := range attachmentPaths {
		RemoveStorageWithNotice(storage.Attachments, "Delete issue attachment", attachmentPaths[i])
	}

	// Remove release attachment files.
	for i := range releaseAttachments {
		RemoveStorageWithNotice(storage.Attachments, "Delete release attachment", releaseAttachments[i])
	}

	if len(repo.Avatar) > 0 {
		avatarPath := repo.CustomAvatarPath()
		if com.IsExist(avatarPath) {
			if err := util.Remove(avatarPath); err != nil {
				return fmt.Errorf("Failed to remove %s: %v", avatarPath, err)
			}
		}
	}

	return nil
}

// GetRepositoryByOwnerAndName returns the repository by given ownername and reponame.
func GetRepositoryByOwnerAndName(ownerName, repoName string) (*Repository, error) {
	return getRepositoryByOwnerAndName(x, ownerName, repoName)
}

func getRepositoryByOwnerAndName(e Engine, ownerName, repoName string) (*Repository, error) {
	var repo Repository
	has, err := e.Table(tbRepository).Select(tbRepository+".*").
		Join("INNER", tbUser, tbUser+".id = "+tbRepository+".owner_id").
		Where(tbRepository+".lower_name = ?", strings.ToLower(repoName)).
		And(tbUser+".lower_name = ?", strings.ToLower(ownerName)).
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

// GetRepositoryByIDCtx returns the repository by given id if exists.
func GetRepositoryByIDCtx(ctx DBContext, id int64) (*Repository, error) {
	return getRepositoryByID(ctx.e, id)
}

// GetRepositoriesMapByIDs returns the repositories by given id slice.
func GetRepositoriesMapByIDs(ids []int64) (map[int64]*Repository, error) {
	var repos = make(map[int64]*Repository, len(ids))
	return repos, x.In("id", ids).Find(&repos)
}

// GetUserRepositories returns a list of repositories of given user.
func GetUserRepositories(opts *SearchRepoOptions) ([]*Repository, int64, error) {
	if len(opts.OrderBy) == 0 {
		opts.OrderBy = "updated_unix DESC"
	}

	var cond = builder.NewCond()
	cond = cond.And(builder.Eq{"owner_id": opts.Actor.ID})
	if !opts.Private {
		cond = cond.And(builder.Eq{"is_private": false})
	}

	sess := x.NewSession()
	defer sess.Close()

	count, err := sess.Where(cond).Count(new(Repository))
	if err != nil {
		return nil, 0, fmt.Errorf("Count: %v", err)
	}

	sess.Where(cond).OrderBy(opts.OrderBy.String())
	repos := make([]*Repository, 0, opts.PageSize)
	return repos, count, opts.setSessionPagination(sess).Find(&repos)
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
func DeleteRepositoryArchives(ctx context.Context) error {
	return x.
		Where("id > 0").
		Iterate(new(Repository),
			func(idx int, bean interface{}) error {
				repo := bean.(*Repository)
				select {
				case <-ctx.Done():
					return ErrCancelledf("before deleting repository archives for %s", repo.FullName())
				default:
				}
				return util.RemoveAll(filepath.Join(repo.RepoPath(), "archives"))
			})
}

// DeleteOldRepositoryArchives deletes old repository archives.
func DeleteOldRepositoryArchives(ctx context.Context, olderThan time.Duration) error {
	log.Trace("Doing: ArchiveCleanup")

	if err := x.Where("id > 0").Iterate(new(Repository), func(idx int, bean interface{}) error {
		return deleteOldRepositoryArchives(ctx, olderThan, idx, bean)
	}); err != nil {
		log.Trace("Error: ArchiveClean: %v", err)
		return err
	}

	log.Trace("Finished: ArchiveCleanup")
	return nil
}

func deleteOldRepositoryArchives(ctx context.Context, olderThan time.Duration, idx int, bean interface{}) error {
	repo := bean.(*Repository)
	basePath := filepath.Join(repo.RepoPath(), "archives")

	for _, ty := range []string{"zip", "targz"} {
		select {
		case <-ctx.Done():
			return ErrCancelledf("before deleting old repository archives with filetype %s for %s", ty, repo.FullName())
		default:
		}

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

		minimumOldestTime := time.Now().Add(-olderThan)
		for _, info := range files {
			if info.ModTime().Before(minimumOldestTime) && !info.IsDir() {
				select {
				case <-ctx.Done():
					return ErrCancelledf("before deleting old repository archive file %s with filetype %s for %s", info.Name(), ty, repo.FullName())
				default:
				}
				toDelete := filepath.Join(path, info.Name())
				// This is a best-effort purge, so we do not check error codes to confirm removal.
				if err = util.Remove(toDelete); err != nil {
					log.Trace("Unable to delete %s, but proceeding: %v", toDelete, err)
				}
			}
		}
	}

	return nil
}

type repoChecker struct {
	querySQL, correctSQL string
	desc                 string
}

func repoStatsCheck(ctx context.Context, checker *repoChecker) {
	results, err := x.Query(checker.querySQL)
	if err != nil {
		log.Error("Select %s: %v", checker.desc, err)
		return
	}
	for _, result := range results {
		id := com.StrTo(result["id"]).MustInt64()
		select {
		case <-ctx.Done():
			log.Warn("CheckRepoStats: Cancelled before checking %s for Repo[%d]", checker.desc, id)
			return
		default:
		}
		log.Trace("Updating %s: %d", checker.desc, id)
		_, err = x.Exec(checker.correctSQL, id, id)
		if err != nil {
			log.Error("Update %s[%d]: %v", checker.desc, id, err)
		}
	}
}

// CheckRepoStats checks the repository stats
func CheckRepoStats(ctx context.Context) error {
	log.Trace("Doing: CheckRepoStats")

	checkers := []*repoChecker{
		// Repository.NumWatches
		{
			"SELECT repo.id FROM " + tbRepository + " repo WHERE repo.num_watches!=(SELECT COUNT(*) FROM " + tbWatch + " WHERE repo_id=repo.id AND mode<>2)",
			"UPDATE " + tbRepository + " SET num_watches=(SELECT COUNT(*) FROM " + tbWatch + " WHERE repo_id=? AND mode<>2) WHERE id=?",
			"repository count 'num_watches'",
		},
		// Repository.NumStars
		{
			"SELECT repo.id FROM " + tbRepository + " repo WHERE repo.num_stars!=(SELECT COUNT(*) FROM " + tbStar + " WHERE repo_id=repo.id)",
			"UPDATE " + tbRepository + " SET num_stars=(SELECT COUNT(*) FROM " + tbStar + " WHERE repo_id=?) WHERE id=?",
			"repository count 'num_stars'",
		},
		// Label.NumIssues
		{
			"SELECT " + tbLabel + ".id FROM " + tbLabel + " WHERE " + tbLabel + ".num_issues!=(SELECT COUNT(*) FROM " + tbIssueLabel + " WHERE label_id=" + tbLabel + ".id)",
			"UPDATE " + tbLabel + " SET num_issues=(SELECT COUNT(*) FROM " + tbIssueLabel + " WHERE label_id=?) WHERE id=?",
			"label count 'num_issues'",
		},
		// User.NumRepos
		{
			"SELECT " + tbUser + ".id FROM " + tbUser + " WHERE " + tbUser + ".num_repos!=(SELECT COUNT(*) FROM `" + tbRepository + "` WHERE owner_id=" + tbUser + ".id)",
			"UPDATE " + tbUser + " SET num_repos=(SELECT COUNT(*) FROM " + tbRepository + " WHERE owner_id=?) WHERE id=?",
			"user count 'num_repos'",
		},
		// Issue.NumComments
		{
			"SELECT " + tbIssue + ".id FROM " + tbIssue + " WHERE " + tbIssue + ".num_comments!=(SELECT COUNT(*) FROM " + tbComment + " WHERE issue_id=" + tbIssue + ".id AND type=0)",
			"UPDATE " + tbIssue + " SET num_comments=(SELECT COUNT(*) FROM " + tbComment + " WHERE issue_id=? AND type=0) WHERE id=?",
			"issue count 'num_comments'",
		},
	}
	for _, checker := range checkers {
		select {
		case <-ctx.Done():
			log.Warn("CheckRepoStats: Cancelled before %s", checker.desc)
			return ErrCancelledf("before checking %s", checker.desc)
		default:
			repoStatsCheck(ctx, checker)
		}
	}

	// ***** START: Repository.NumClosedIssues *****
	desc := "repository count 'num_closed_issues'"
	results, err := x.Query("SELECT repo.id FROM "+tbRepository+" repo WHERE repo.num_closed_issues!=(SELECT COUNT(*) FROM "+tbIssue+" WHERE repo_id=repo.id AND is_closed=? AND is_pull=?)", true, false)
	if err != nil {
		log.Error("Select %s: %v", desc, err)
	} else {
		for _, result := range results {
			id := com.StrTo(result["id"]).MustInt64()
			select {
			case <-ctx.Done():
				log.Warn("CheckRepoStats: Cancelled during %s for repo ID %d", desc, id)
				return ErrCancelledf("during %s for repo ID %d", desc, id)
			default:
			}
			log.Trace("Updating %s: %d", desc, id)
			_, err = x.Exec("UPDATE "+tbRepository+" SET num_closed_issues=(SELECT COUNT(*) FROM "+tbIssue+" WHERE repo_id=? AND is_closed=? AND is_pull=?) WHERE id=?", id, true, false, id)
			if err != nil {
				log.Error("Update %s[%d]: %v", desc, id, err)
			}
		}
	}
	// ***** END: Repository.NumClosedIssues *****

	// ***** START: Repository.NumClosedPulls *****
	desc = "repository count 'num_closed_pulls'"
	results, err = x.Query("SELECT repo.id FROM "+tbRepository+" repo WHERE repo.num_closed_pulls!=(SELECT COUNT(*) FROM "+tbIssue+" WHERE repo_id=repo.id AND is_closed=? AND is_pull=?)", true, true)
	if err != nil {
		log.Error("Select %s: %v", desc, err)
	} else {
		for _, result := range results {
			id := com.StrTo(result["id"]).MustInt64()
			select {
			case <-ctx.Done():
				log.Warn("CheckRepoStats: Cancelled")
				return ErrCancelledf("during %s for repo ID %d", desc, id)
			default:
			}
			log.Trace("Updating %s: %d", desc, id)
			_, err = x.Exec("UPDATE "+tbRepository+" SET num_closed_pulls=(SELECT COUNT(*) FROM "+tbIssue+" WHERE repo_id=? AND is_closed=? AND is_pull=?) WHERE id=?", id, true, true, id)
			if err != nil {
				log.Error("Update %s[%d]: %v", desc, id, err)
			}
		}
	}
	// ***** END: Repository.NumClosedPulls *****

	// FIXME: use checker when stop supporting old fork repo format.
	// ***** START: Repository.NumForks *****
	results, err = x.Query("SELECT repo.id FROM " + tbRepository + " repo WHERE repo.num_forks!=(SELECT COUNT(*) FROM " + tbRepository + " WHERE fork_id=repo.id)")
	if err != nil {
		log.Error("Select repository count 'num_forks': %v", err)
	} else {
		for _, result := range results {
			id := com.StrTo(result["id"]).MustInt64()
			select {
			case <-ctx.Done():
				log.Warn("CheckRepoStats: Cancelled")
				return ErrCancelledf("during %s for repo ID %d", desc, id)
			default:
			}
			log.Trace("Updating repository count 'num_forks': %d", id)

			repo, err := GetRepositoryByID(id)
			if err != nil {
				log.Error("GetRepositoryByID[%d]: %v", id, err)
				continue
			}

			rawResult, err := x.Query("SELECT COUNT(*) FROM "+tbRepository+" WHERE fork_id=?", repo.ID)
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
	return nil
}

// SetArchiveRepoState sets if a repo is archived
func (repo *Repository) SetArchiveRepoState(isArchived bool) (err error) {
	repo.IsArchived = isArchived
	_, err = x.Where("id = ?", repo.ID).Cols("is_archived").NoAutoTime().Update(repo)
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

// CopyLFS copies LFS data from one repo to another
func CopyLFS(ctx DBContext, newRepo, oldRepo *Repository) error {
	var lfsObjects []*LFSMetaObject
	if err := ctx.e.Where("repository_id=?", oldRepo.ID).Find(&lfsObjects); err != nil {
		return err
	}

	for _, v := range lfsObjects {
		v.ID = 0
		v.RepositoryID = newRepo.ID
		if _, err := ctx.e.Insert(v); err != nil {
			return err
		}
	}

	return nil
}

// GetForks returns all the forks of the repository
func (repo *Repository) GetForks(listOptions ListOptions) ([]*Repository, error) {
	if listOptions.Page == 0 {
		forks := make([]*Repository, 0, repo.NumForks)
		return forks, x.Find(&forks, &Repository{ForkID: repo.ID})
	}

	sess := listOptions.getPaginatedSession()
	forks := make([]*Repository, 0, listOptions.PageSize)
	return forks, sess.Find(&forks, &Repository{ForkID: repo.ID})
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
func RemoveRandomAvatars(ctx context.Context) error {
	return x.
		Where("id > 0").BufferSize(setting.Database.IterateBufferSize).
		Iterate(new(Repository),
			func(idx int, bean interface{}) error {
				repository := bean.(*Repository)
				select {
				case <-ctx.Done():
					return ErrCancelledf("before random avatars removed for %s", repository.FullName())
				default:
				}
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

// AvatarLink returns a link to the repository's avatar.
func (repo *Repository) AvatarLink() string {
	return repo.avatarLink(x)
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
		if err := util.Remove(oldAvatarPath); err != nil {
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
		if err := util.Remove(avatarPath); err != nil {
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

// GetTreePathLock returns LSF lock for the treePath
func (repo *Repository) GetTreePathLock(treePath string) (*LFSLock, error) {
	if setting.LFS.StartServer {
		locks, err := GetLFSLockByRepoID(repo.ID, 0, 0)
		if err != nil {
			return nil, err
		}
		for _, lock := range locks {
			if lock.Path == treePath {
				return lock, nil
			}
		}
	}
	return nil, nil
}

func updateRepositoryCols(e Engine, repo *Repository, cols ...string) error {
	_, err := e.ID(repo.ID).Cols(cols...).Update(repo)
	return err
}

// UpdateRepositoryCols updates repository's columns
func UpdateRepositoryCols(repo *Repository, cols ...string) error {
	return updateRepositoryCols(x, repo, cols...)
}

// DoctorUserStarNum recalculate Stars number for all user
func DoctorUserStarNum() (err error) {
	const batchSize = 100
	sess := x.NewSession()
	defer sess.Close()

	for start := 0; ; start += batchSize {
		users := make([]User, 0, batchSize)
		if err = sess.Limit(batchSize, start).Where("type = ?", 0).Cols("id").Find(&users); err != nil {
			return
		}
		if len(users) == 0 {
			break
		}

		if err = sess.Begin(); err != nil {
			return
		}

		for _, user := range users {
			if _, err = sess.Exec("UPDATE "+tbUser+" SET num_stars=(SELECT COUNT(*) FROM "+tbStar+" WHERE uid=?) WHERE id=?", user.ID, user.ID); err != nil {
				return
			}
		}

		if err = sess.Commit(); err != nil {
			return
		}
	}

	log.Debug("recalculate Stars number for all user finished")

	return
}
