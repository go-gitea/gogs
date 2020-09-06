// Copyright 2018 The Gitea Authors. All rights reserved.
// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/builder"
	"xorm.io/xorm"
)

const ownerTeamName = "Owners"

// Team represents a organization team.
type Team struct {
	ID                      int64 `xorm:"pk autoincr"`
	OrgID                   int64 `xorm:"INDEX"`
	LowerName               string
	Name                    string
	Description             string
	Authorize               AccessMode
	Repos                   []*Repository `xorm:"-"`
	Members                 []*User       `xorm:"-"`
	NumRepos                int
	NumMembers              int
	Units                   []*TeamUnit `xorm:"-"`
	IncludesAllRepositories bool        `xorm:"NOT NULL DEFAULT false"`
	CanCreateOrgRepo        bool        `xorm:"NOT NULL DEFAULT false"`
}

// SearchTeamOptions holds the search options
type SearchTeamOptions struct {
	ListOptions
	UserID      int64
	Keyword     string
	OrgID       int64
	IncludeDesc bool
}

// SearchMembersOptions holds the search options
type SearchMembersOptions struct {
	ListOptions
}

// SearchTeam search for teams. Caller is responsible to check permissions.
func SearchTeam(opts *SearchTeamOptions) ([]*Team, int64, error) {
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.PageSize == 0 {
		// Default limit
		opts.PageSize = 10
	}

	var cond = builder.NewCond()

	if len(opts.Keyword) > 0 {
		lowerKeyword := strings.ToLower(opts.Keyword)
		var keywordCond builder.Cond = builder.Like{"lower_name", lowerKeyword}
		if opts.IncludeDesc {
			keywordCond = keywordCond.Or(builder.Like{"LOWER(description)", lowerKeyword})
		}
		cond = cond.And(keywordCond)
	}

	cond = cond.And(builder.Eq{"org_id": opts.OrgID})

	sess := x.NewSession()
	defer sess.Close()

	count, err := sess.
		Where(cond).
		Count(new(Team))

	if err != nil {
		return nil, 0, err
	}

	sess = sess.Where(cond)
	if opts.PageSize == -1 {
		opts.PageSize = int(count)
	} else {
		sess = sess.Limit(opts.PageSize, (opts.Page-1)*opts.PageSize)
	}

	teams := make([]*Team, 0, opts.PageSize)
	if err = sess.
		OrderBy("lower_name").
		Find(&teams); err != nil {
		return nil, 0, err
	}

	return teams, count, nil
}

// TableName sets the table name to `Tteam`
func (t *Team) TableName() string {
	return tbTeam[1 : len(tbTeam)-1]
}

// ColorFormat provides a basic color format for a Team
func (t *Team) ColorFormat(s fmt.State) {
	log.ColorFprintf(s, "%d:%s (OrgID: %d) %-v",
		log.NewColoredIDValue(t.ID),
		t.Name,
		log.NewColoredIDValue(t.OrgID),
		t.Authorize)

}

// GetUnits return a list of available units for a team
func (t *Team) GetUnits() error {
	return t.getUnits(x)
}

func (t *Team) getUnits(e Engine) (err error) {
	if t.Units != nil {
		return nil
	}

	t.Units, err = getUnitsByTeamID(e, t.ID)
	return err
}

// GetUnitNames returns the team units names
func (t *Team) GetUnitNames() (res []string) {
	for _, u := range t.Units {
		res = append(res, Units[u.Type].NameKey)
	}
	return
}

// HasWriteAccess returns true if team has at least write level access mode.
func (t *Team) HasWriteAccess() bool {
	return t.Authorize >= AccessModeWrite
}

// IsOwnerTeam returns true if team is owner team.
func (t *Team) IsOwnerTeam() bool {
	return t.Name == ownerTeamName
}

// IsMember returns true if given user is a member of team.
func (t *Team) IsMember(userID int64) bool {
	isMember, err := IsTeamMember(t.OrgID, t.ID, userID)
	if err != nil {
		log.Error("IsMember: %v", err)
		return false
	}
	return isMember
}

func (t *Team) getRepositories(e Engine) error {
	if t.Repos != nil {
		return nil
	}
	return e.Join("INNER", tbTeamRepo, tbRepository+".id = "+tbTeamRepo+".repo_id").
		Where(tbTeamRepo+".team_id=?", t.ID).
		OrderBy(tbRepository + ".name").
		Find(&t.Repos)
}

// GetRepositories returns paginated repositories in team of organization.
func (t *Team) GetRepositories(opts *SearchTeamOptions) error {
	if opts.Page == 0 {
		return t.getRepositories(x)
	}

	return t.getRepositories(opts.getPaginatedSession())
}

func (t *Team) getMembers(e Engine) (err error) {
	t.Members, err = getTeamMembers(e, t.ID)
	return err
}

// GetMembers returns paginated members in team of organization.
func (t *Team) GetMembers(opts *SearchMembersOptions) (err error) {
	if opts.Page == 0 {
		return t.getMembers(x)
	}

	return t.getMembers(opts.getPaginatedSession())
}

// AddMember adds new membership of the team to the organization,
// the user will have membership to the organization automatically when needed.
func (t *Team) AddMember(userID int64) error {
	return AddTeamMember(t, userID)
}

// RemoveMember removes member from team of organization.
func (t *Team) RemoveMember(userID int64) error {
	return RemoveTeamMember(t, userID)
}

func (t *Team) hasRepository(e Engine, repoID int64) bool {
	return hasTeamRepo(e, t.OrgID, t.ID, repoID)
}

// HasRepository returns true if given repository belong to team.
func (t *Team) HasRepository(repoID int64) bool {
	return t.hasRepository(x, repoID)
}

func (t *Team) addRepository(e Engine, repo *Repository) (err error) {
	if err = addTeamRepo(e, t.OrgID, t.ID, repo.ID); err != nil {
		return err
	}

	if _, err = e.Incr("num_repos").ID(t.ID).Update(new(Team)); err != nil {
		return fmt.Errorf("update team: %v", err)
	}

	t.NumRepos++

	if err = repo.recalculateTeamAccesses(e, 0); err != nil {
		return fmt.Errorf("recalculateAccesses: %v", err)
	}

	// Make all team members watch this repo if enabled in global settings
	if setting.Service.AutoWatchNewRepos {
		if err = t.getMembers(e); err != nil {
			return fmt.Errorf("getMembers: %v", err)
		}
		for _, u := range t.Members {
			if err = watchRepo(e, u.ID, repo.ID, true); err != nil {
				return fmt.Errorf("watchRepo: %v", err)
			}
		}
	}

	return nil
}

// addAllRepositories adds all repositories to the team.
// If the team already has some repositories they will be left unchanged.
func (t *Team) addAllRepositories(e Engine) error {
	var orgRepos []Repository
	if err := e.Where("owner_id = ?", t.OrgID).Find(&orgRepos); err != nil {
		return fmt.Errorf("get org repos: %v", err)
	}

	for _, repo := range orgRepos {
		if !t.hasRepository(e, repo.ID) {
			if err := t.addRepository(e, &repo); err != nil {
				return fmt.Errorf("addRepository: %v", err)
			}
		}
	}

	return nil
}

// AddAllRepositories adds all repositories to the team
func (t *Team) AddAllRepositories() (err error) {
	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if err = t.addAllRepositories(sess); err != nil {
		return err
	}

	return sess.Commit()
}

// AddRepository adds new repository to team of organization.
func (t *Team) AddRepository(repo *Repository) (err error) {
	if repo.OwnerID != t.OrgID {
		return errors.New("Repository does not belong to organization")
	} else if t.HasRepository(repo.ID) {
		return nil
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if err = t.addRepository(sess, repo); err != nil {
		return err
	}

	return sess.Commit()
}

// RemoveAllRepositories removes all repositories from team and recalculates access
func (t *Team) RemoveAllRepositories() (err error) {
	if t.IncludesAllRepositories {
		return nil
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if err = t.removeAllRepositories(sess); err != nil {
		return err
	}

	return sess.Commit()
}

// removeAllRepositories removes all repositories from team and recalculates access
// Note: Shall not be called if team includes all repositories
func (t *Team) removeAllRepositories(e Engine) (err error) {
	// Delete all accesses.
	for _, repo := range t.Repos {
		if err := repo.recalculateTeamAccesses(e, t.ID); err != nil {
			return err
		}

		// Remove watches from all users and now unaccessible repos
		for _, user := range t.Members {
			has, err := hasAccess(e, user.ID, repo)
			if err != nil {
				return err
			} else if has {
				continue
			}

			if err = watchRepo(e, user.ID, repo.ID, false); err != nil {
				return err
			}

			// Remove all IssueWatches a user has subscribed to in the repositories
			if err = removeIssueWatchersByRepoID(e, user.ID, repo.ID); err != nil {
				return err
			}
		}
	}

	// Delete team-repo
	if _, err := e.
		Where("team_id=?", t.ID).
		Delete(new(TeamRepo)); err != nil {
		return err
	}

	t.NumRepos = 0
	if _, err = e.ID(t.ID).Cols("num_repos").Update(t); err != nil {
		return err
	}

	return nil
}

// removeRepository removes a repository from a team and recalculates access
// Note: Repository shall not be removed from team if it includes all repositories (unless the repository is deleted)
func (t *Team) removeRepository(e Engine, repo *Repository, recalculate bool) (err error) {
	if err = removeTeamRepo(e, t.ID, repo.ID); err != nil {
		return err
	}

	t.NumRepos--
	if _, err = e.ID(t.ID).Cols("num_repos").Update(t); err != nil {
		return err
	}

	// Don't need to recalculate when delete a repository from organization.
	if recalculate {
		if err = repo.recalculateTeamAccesses(e, t.ID); err != nil {
			return err
		}
	}

	teamUsers, err := getTeamUsersByTeamID(e, t.ID)
	if err != nil {
		return fmt.Errorf("getTeamUsersByTeamID: %v", err)
	}
	for _, teamUser := range teamUsers {
		has, err := hasAccess(e, teamUser.UID, repo)
		if err != nil {
			return err
		} else if has {
			continue
		}

		if err = watchRepo(e, teamUser.UID, repo.ID, false); err != nil {
			return err
		}

		// Remove all IssueWatches a user has subscribed to in the repositories
		if err := removeIssueWatchersByRepoID(e, teamUser.UID, repo.ID); err != nil {
			return err
		}
	}

	return nil
}

// RemoveRepository removes repository from team of organization.
// If the team shall include all repositories the request is ignored.
func (t *Team) RemoveRepository(repoID int64) error {
	if !t.HasRepository(repoID) {
		return nil
	}

	if t.IncludesAllRepositories {
		return nil
	}

	repo, err := GetRepositoryByID(repoID)
	if err != nil {
		return err
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if err = t.removeRepository(sess, repo, true); err != nil {
		return err
	}

	return sess.Commit()
}

// UnitEnabled returns if the team has the given unit type enabled
func (t *Team) UnitEnabled(tp UnitType) bool {
	return t.unitEnabled(x, tp)
}

func (t *Team) unitEnabled(e Engine, tp UnitType) bool {
	if err := t.getUnits(e); err != nil {
		log.Warn("Error loading team (ID: %d) units: %s", t.ID, err.Error())
	}

	for _, unit := range t.Units {
		if unit.Type == tp {
			return true
		}
	}
	return false
}

// IsUsableTeamName tests if a name could be as team name
func IsUsableTeamName(name string) error {
	switch name {
	case "new":
		return ErrNameReserved{name}
	default:
		return nil
	}
}

// NewTeam creates a record of new team.
// It's caller's responsibility to assign organization ID.
func NewTeam(t *Team) (err error) {
	if len(t.Name) == 0 {
		return errors.New("empty team name")
	}

	if err = IsUsableTeamName(t.Name); err != nil {
		return err
	}

	has, err := x.ID(t.OrgID).Get(new(User))
	if err != nil {
		return err
	}
	if !has {
		return ErrOrgNotExist{t.OrgID, ""}
	}

	t.LowerName = strings.ToLower(t.Name)
	has, err = x.
		Where("org_id=?", t.OrgID).
		And("lower_name=?", t.LowerName).
		Get(new(Team))
	if err != nil {
		return err
	}
	if has {
		return ErrTeamAlreadyExist{t.OrgID, t.LowerName}
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if _, err = sess.Insert(t); err != nil {
		errRollback := sess.Rollback()
		if errRollback != nil {
			log.Error("NewTeam sess.Rollback: %v", errRollback)
		}
		return err
	}

	// insert units for team
	if len(t.Units) > 0 {
		for _, unit := range t.Units {
			unit.TeamID = t.ID
		}
		if _, err = sess.Insert(&t.Units); err != nil {
			errRollback := sess.Rollback()
			if errRollback != nil {
				log.Error("NewTeam sess.Rollback: %v", errRollback)
			}
			return err
		}
	}

	// Add all repositories to the team if it has access to all of them.
	if t.IncludesAllRepositories {
		err = t.addAllRepositories(sess)
		if err != nil {
			return fmt.Errorf("addAllRepositories: %v", err)
		}
	}

	// Update organization number of teams.
	if _, err = sess.Exec("UPDATE "+tbUser+" SET num_teams=num_teams+1 WHERE id = ?", t.OrgID); err != nil {
		errRollback := sess.Rollback()
		if errRollback != nil {
			log.Error("NewTeam sess.Rollback: %v", errRollback)
		}
		return err
	}
	return sess.Commit()
}

func getTeam(e Engine, orgID int64, name string) (*Team, error) {
	t := &Team{
		OrgID:     orgID,
		LowerName: strings.ToLower(name),
	}
	has, err := e.Get(t)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrTeamNotExist{orgID, 0, name}
	}
	return t, nil
}

// GetTeam returns team by given team name and organization.
func GetTeam(orgID int64, name string) (*Team, error) {
	return getTeam(x, orgID, name)
}

// GetTeamIDsByNames returns a slice of team ids corresponds to names.
func GetTeamIDsByNames(orgID int64, names []string, ignoreNonExistent bool) ([]int64, error) {
	ids := make([]int64, 0, len(names))
	for _, name := range names {
		u, err := GetTeam(orgID, name)
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

// getOwnerTeam returns team by given team name and organization.
func getOwnerTeam(e Engine, orgID int64) (*Team, error) {
	return getTeam(e, orgID, ownerTeamName)
}

func getTeamByID(e Engine, teamID int64) (*Team, error) {
	t := new(Team)
	has, err := e.ID(teamID).Get(t)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrTeamNotExist{0, teamID, ""}
	}
	return t, nil
}

// GetTeamByID returns team by given ID.
func GetTeamByID(teamID int64) (*Team, error) {
	return getTeamByID(x, teamID)
}

// GetTeamNamesByID returns team's lower name from a list of team ids.
func GetTeamNamesByID(teamIDs []int64) ([]string, error) {
	if len(teamIDs) == 0 {
		return []string{}, nil
	}

	var teamNames []string
	err := x.Table(tbTeam).
		Select("lower_name").
		In("id", teamIDs).
		Asc("name").
		Find(&teamNames)

	return teamNames, err
}

// UpdateTeam updates information of team.
func UpdateTeam(t *Team, authChanged bool, includeAllChanged bool) (err error) {
	if len(t.Name) == 0 {
		return errors.New("empty team name")
	}

	if len(t.Description) > 255 {
		t.Description = t.Description[:255]
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	t.LowerName = strings.ToLower(t.Name)
	has, err := sess.
		Where("org_id=?", t.OrgID).
		And("lower_name=?", t.LowerName).
		And("id!=?", t.ID).
		Get(new(Team))
	if err != nil {
		return err
	} else if has {
		return ErrTeamAlreadyExist{t.OrgID, t.LowerName}
	}

	if _, err = sess.ID(t.ID).Cols("name", "lower_name", "description",
		"can_create_org_repo", "authorize", "includes_all_repositories").Update(t); err != nil {
		return fmt.Errorf("update: %v", err)
	}

	// update units for team
	if len(t.Units) > 0 {
		for _, unit := range t.Units {
			unit.TeamID = t.ID
		}
		// Delete team-unit.
		if _, err := sess.
			Where("team_id=?", t.ID).
			Delete(new(TeamUnit)); err != nil {
			return err
		}
		if _, err = sess.Cols("org_id", "team_id", "type").Insert(&t.Units); err != nil {
			errRollback := sess.Rollback()
			if errRollback != nil {
				log.Error("UpdateTeam sess.Rollback: %v", errRollback)
			}
			return err
		}
	}

	// Update access for team members if needed.
	if authChanged {
		if err = t.getRepositories(sess); err != nil {
			return fmt.Errorf("getRepositories: %v", err)
		}

		for _, repo := range t.Repos {
			if err = repo.recalculateTeamAccesses(sess, 0); err != nil {
				return fmt.Errorf("recalculateTeamAccesses: %v", err)
			}
		}
	}

	// Add all repositories to the team if it has access to all of them.
	if includeAllChanged && t.IncludesAllRepositories {
		err = t.addAllRepositories(sess)
		if err != nil {
			return fmt.Errorf("addAllRepositories: %v", err)
		}
	}

	return sess.Commit()
}

// DeleteTeam deletes given team.
// It's caller's responsibility to assign organization ID.
func DeleteTeam(t *Team) error {
	if err := t.GetRepositories(&SearchTeamOptions{}); err != nil {
		return err
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if err := t.getMembers(sess); err != nil {
		return err
	}

	if err := t.removeAllRepositories(sess); err != nil {
		return err
	}

	// Delete team-user.
	if _, err := sess.
		Where("org_id=?", t.OrgID).
		Where("team_id=?", t.ID).
		Delete(new(TeamUser)); err != nil {
		return err
	}

	// Delete team-unit.
	if _, err := sess.
		Where("team_id=?", t.ID).
		Delete(new(TeamUnit)); err != nil {
		return err
	}

	// Delete team.
	if _, err := sess.ID(t.ID).Delete(new(Team)); err != nil {
		return err
	}
	// Update organization number of teams.
	if _, err := sess.Exec("UPDATE "+tbUser+" SET num_teams=num_teams-1 WHERE id=?", t.OrgID); err != nil {
		return err
	}

	return sess.Commit()
}

// ___________                    ____ ___
// \__    ___/___ _____    _____ |    |   \______ ___________
//   |    |_/ __ \\__  \  /     \|    |   /  ___// __ \_  __ \
//   |    |\  ___/ / __ \|  Y Y  \    |  /\___ \\  ___/|  | \/
//   |____| \___  >____  /__|_|  /______//____  >\___  >__|
//              \/     \/      \/             \/     \/

// TeamUser represents an team-user relation.
type TeamUser struct {
	ID     int64 `xorm:"pk autoincr"`
	OrgID  int64 `xorm:"INDEX"`
	TeamID int64 `xorm:"UNIQUE(s)"`
	UID    int64 `xorm:"UNIQUE(s)"`
}

// TableName sets the table name to `team_user`
func (t *TeamUser) TableName() string {
	return tbTeamUser[1 : len(tbTeamUser)-1]
}

func isTeamMember(e Engine, orgID, teamID, userID int64) (bool, error) {
	return e.
		Where("org_id=?", orgID).
		And("team_id=?", teamID).
		And("uid=?", userID).
		Table(tbTeamUser).
		Exist()
}

// IsTeamMember returns true if given user is a member of team.
func IsTeamMember(orgID, teamID, userID int64) (bool, error) {
	return isTeamMember(x, orgID, teamID, userID)
}

func getTeamUsersByTeamID(e Engine, teamID int64) ([]*TeamUser, error) {
	teamUsers := make([]*TeamUser, 0, 10)
	return teamUsers, e.
		Where("team_id=?", teamID).
		Find(&teamUsers)
}

func getTeamMembers(e Engine, teamID int64) (_ []*User, err error) {
	teamUsers, err := getTeamUsersByTeamID(e, teamID)
	if err != nil {
		return nil, fmt.Errorf("get team-users: %v", err)
	}
	members := make([]*User, len(teamUsers))
	for i, teamUser := range teamUsers {
		member, err := getUserByID(e, teamUser.UID)
		if err != nil {
			return nil, fmt.Errorf("get user '%d': %v", teamUser.UID, err)
		}
		members[i] = member
	}
	sort.Slice(members, func(i, j int) bool {
		return members[i].DisplayName() < members[j].DisplayName()
	})
	return members, nil
}

// GetTeamMembers returns all members in given team of organization.
func GetTeamMembers(teamID int64) ([]*User, error) {
	return getTeamMembers(x, teamID)
}

func getUserTeams(e Engine, userID int64, listOptions ListOptions) (teams []*Team, err error) {
	sess := e.
		Join("INNER", tbTeamUser, tbTeamUser+".team_id = "+tbTeam+".id").
		Where(tbTeamUser+".uid=?", userID)
	if listOptions.Page != 0 {
		sess = listOptions.setSessionPagination(sess)
	}
	return teams, sess.Find(&teams)
}

func getUserOrgTeams(e Engine, orgID, userID int64) (teams []*Team, err error) {
	return teams, e.
		Join("INNER", tbTeamUser, tbTeamUser+".team_id = "+tbTeam+".id").
		Where(tbTeam+".org_id = ?", orgID).
		And(tbTeamUser+".uid=?", userID).
		Find(&teams)
}

func getUserRepoTeams(e Engine, orgID, userID, repoID int64) (teams []*Team, err error) {
	return teams, e.
		Join("INNER", tbTeamUser, tbTeamUser+".team_id = "+tbTeam+".id").
		Join("INNER", tbTeamRepo, tbTeamRepo+".team_id = "+tbTeam+".id").
		Where(tbTeam+".org_id = ?", orgID).
		And(tbTeamUser+".uid=?", userID).
		And(tbTeamRepo+".repo_id=?", repoID).
		Find(&teams)
}

// GetUserOrgTeams returns all teams that user belongs to in given organization.
func GetUserOrgTeams(orgID, userID int64) ([]*Team, error) {
	return getUserOrgTeams(x, orgID, userID)
}

// GetUserTeams returns all teams that user belongs across all organizations.
func GetUserTeams(userID int64, listOptions ListOptions) ([]*Team, error) {
	return getUserTeams(x, userID, listOptions)
}

// AddTeamMember adds new membership of given team to given organization,
// the user will have membership to given organization automatically when needed.
func AddTeamMember(team *Team, userID int64) error {
	isAlreadyMember, err := IsTeamMember(team.OrgID, team.ID, userID)
	if err != nil || isAlreadyMember {
		return err
	}

	if err := AddOrgUser(team.OrgID, userID); err != nil {
		return err
	}

	// Get team and its repositories.
	if err := team.GetRepositories(&SearchTeamOptions{}); err != nil {
		return err
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if _, err := sess.Insert(&TeamUser{
		UID:    userID,
		OrgID:  team.OrgID,
		TeamID: team.ID,
	}); err != nil {
		return err
	} else if _, err := sess.Incr("num_members").ID(team.ID).Update(new(Team)); err != nil {
		return err
	}

	team.NumMembers++

	// Give access to team repositories.
	for _, repo := range team.Repos {
		if err := repo.recalculateUserAccess(sess, userID); err != nil {
			return err
		}
		if setting.Service.AutoWatchNewRepos {
			if err = watchRepo(sess, userID, repo.ID, true); err != nil {
				return err
			}
		}
	}

	return sess.Commit()
}

func removeTeamMember(e *xorm.Session, team *Team, userID int64) error {
	isMember, err := isTeamMember(e, team.OrgID, team.ID, userID)
	if err != nil || !isMember {
		return err
	}

	// Check if the user to delete is the last member in owner team.
	if team.IsOwnerTeam() && team.NumMembers == 1 {
		return ErrLastOrgOwner{UID: userID}
	}

	team.NumMembers--

	if err := team.getRepositories(e); err != nil {
		return err
	}

	if _, err := e.Delete(&TeamUser{
		UID:    userID,
		OrgID:  team.OrgID,
		TeamID: team.ID,
	}); err != nil {
		return err
	} else if _, err = e.
		ID(team.ID).
		Cols("num_members").
		Update(team); err != nil {
		return err
	}

	// Delete access to team repositories.
	for _, repo := range team.Repos {
		if err := repo.recalculateUserAccess(e, userID); err != nil {
			return err
		}

		// Remove watches from now unaccessible
		if err := repo.reconsiderWatches(e, userID); err != nil {
			return err
		}

		// Remove issue assignments from now unaccessible
		if err := repo.reconsiderIssueAssignees(e, userID); err != nil {
			return err
		}
	}

	// Check if the user is a member of any team in the organization.
	if count, err := e.Count(&TeamUser{
		UID:   userID,
		OrgID: team.OrgID,
	}); err != nil {
		return err
	} else if count == 0 {
		return removeOrgUser(e, team.OrgID, userID)
	}

	return nil
}

// RemoveTeamMember removes member from given team of given organization.
func RemoveTeamMember(team *Team, userID int64) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	if err := removeTeamMember(sess, team, userID); err != nil {
		return err
	}
	return sess.Commit()
}

// IsUserInTeams returns if a user in some teams
func IsUserInTeams(userID int64, teamIDs []int64) (bool, error) {
	return isUserInTeams(x, userID, teamIDs)
}

func isUserInTeams(e Engine, userID int64, teamIDs []int64) (bool, error) {
	return e.Where("uid=?", userID).In("team_id", teamIDs).Exist(new(TeamUser))
}

// UsersInTeamsCount counts the number of users which are in userIDs and teamIDs
func UsersInTeamsCount(userIDs []int64, teamIDs []int64) (int64, error) {
	var ids []int64
	if err := x.In("uid", userIDs).In("team_id", teamIDs).
		Table(tbTeamUser).
		Cols("uid").GroupBy("uid").Find(&ids); err != nil {
		return 0, err
	}
	return int64(len(ids)), nil
}

// ___________                  __________
// \__    ___/___ _____    _____\______   \ ____ ______   ____
//   |    |_/ __ \\__  \  /     \|       _// __ \\____ \ /  _ \
//   |    |\  ___/ / __ \|  Y Y  \    |   \  ___/|  |_> >  <_> )
//   |____| \___  >____  /__|_|  /____|_  /\___  >   __/ \____/
//              \/     \/      \/       \/     \/|__|

// TeamRepo represents an team-repository relation.
type TeamRepo struct {
	ID     int64 `xorm:"pk autoincr"`
	OrgID  int64 `xorm:"INDEX"`
	TeamID int64 `xorm:"UNIQUE(s)"`
	RepoID int64 `xorm:"UNIQUE(s)"`
}

// TableName sets the table name to `team_repo`
func (t *TeamRepo) TableName() string {
	return tbTeamRepo[1 : len(tbTeamRepo)-1]
}

func hasTeamRepo(e Engine, orgID, teamID, repoID int64) bool {
	has, _ := e.
		Where("org_id=?", orgID).
		And("team_id=?", teamID).
		And("repo_id=?", repoID).
		Get(new(TeamRepo))
	return has
}

// HasTeamRepo returns true if given repository belongs to team.
func HasTeamRepo(orgID, teamID, repoID int64) bool {
	return hasTeamRepo(x, orgID, teamID, repoID)
}

func addTeamRepo(e Engine, orgID, teamID, repoID int64) error {
	_, err := e.InsertOne(&TeamRepo{
		OrgID:  orgID,
		TeamID: teamID,
		RepoID: repoID,
	})
	return err
}

func removeTeamRepo(e Engine, teamID, repoID int64) error {
	_, err := e.Delete(&TeamRepo{
		TeamID: teamID,
		RepoID: repoID,
	})
	return err
}

// GetTeamsWithAccessToRepo returns all teams in an organization that have given access level to the repository.
func GetTeamsWithAccessToRepo(orgID, repoID int64, mode AccessMode) ([]*Team, error) {
	teams := make([]*Team, 0, 5)
	return teams, x.Where(tbTeam+".authorize >= ?", mode).
		Join("INNER", tbTeamRepo, tbTeamRepo+".team_id = "+tbTeam+".id").
		And(tbTeamRepo+".org_id = ?", orgID).
		And(tbTeamRepo+".repo_id = ?", repoID).
		Find(&teams)
}

// ___________                    ____ ___      .__  __
// \__    ___/___ _____    _____ |    |   \____ |__|/  |_
//   |    |_/ __ \\__  \  /     \|    |   /    \|  \   __\
//   |    |\  ___/ / __ \|  Y Y  \    |  /   |  \  ||  |
//   |____| \___  >____  /__|_|  /______/|___|  /__||__|
//              \/     \/      \/             \/

// TeamUnit describes all units of a repository
type TeamUnit struct {
	ID     int64    `xorm:"pk autoincr"`
	OrgID  int64    `xorm:"INDEX"`
	TeamID int64    `xorm:"UNIQUE(s)"`
	Type   UnitType `xorm:"UNIQUE(s)"`
}

// TableName sets the table name to `team_unit`
func (t *TeamUnit) TableName() string {
	return tbTeamUnit[1 : len(tbTeamUnit)-1]
}

// Unit returns Unit
func (t *TeamUnit) Unit() Unit {
	return Units[t.Type]
}

func getUnitsByTeamID(e Engine, teamID int64) (units []*TeamUnit, err error) {
	return units, e.Where("team_id = ?", teamID).Find(&units)
}

// UpdateTeamUnits updates a teams's units
func UpdateTeamUnits(team *Team, units []TeamUnit) (err error) {
	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if _, err = sess.Where("team_id = ?", team.ID).Delete(new(TeamUnit)); err != nil {
		return err
	}

	if len(units) > 0 {
		if _, err = sess.Insert(units); err != nil {
			errRollback := sess.Rollback()
			if errRollback != nil {
				log.Error("UpdateTeamUnits sess.Rollback: %v", errRollback)
			}
			return err
		}
	}

	return sess.Commit()
}
