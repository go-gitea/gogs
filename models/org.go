// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"

	"github.com/unknwon/com"
	"xorm.io/builder"
	"xorm.io/xorm"
)

// IsOwnedBy returns true if given user is in the owner team.
func (org *User) IsOwnedBy(uid int64) (bool, error) {
	return IsOrganizationOwner(org.ID, uid)
}

// IsOrgMember returns true if given user is member of organization.
func (org *User) IsOrgMember(uid int64) (bool, error) {
	return IsOrganizationMember(org.ID, uid)
}

// CanCreateOrgRepo returns true if given user can create repo in organization
func (org *User) CanCreateOrgRepo(uid int64) (bool, error) {
	return CanCreateOrgRepo(org.ID, uid)
}

func (org *User) getTeam(e Engine, name string) (*Team, error) {
	return getTeam(e, org.ID, name)
}

// GetTeam returns named team of organization.
func (org *User) GetTeam(name string) (*Team, error) {
	return org.getTeam(x, name)
}

func (org *User) getOwnerTeam(e Engine) (*Team, error) {
	return org.getTeam(e, ownerTeamName)
}

// GetOwnerTeam returns owner team of organization.
func (org *User) GetOwnerTeam() (*Team, error) {
	return org.getOwnerTeam(x)
}

func (org *User) getTeams(e Engine) error {
	if org.Teams != nil {
		return nil
	}
	return e.
		Where("org_id=?", org.ID).
		OrderBy("CASE WHEN name LIKE '" + ownerTeamName + "' THEN '' ELSE name END").
		Find(&org.Teams)
}

// GetTeams returns paginated teams that belong to organization.
func (org *User) GetTeams(opts *SearchTeamOptions) error {
	if opts.Page != 0 {
		return org.getTeams(opts.getPaginatedSession())
	}

	return org.getTeams(x)
}

// GetMembers returns all members of organization.
func (org *User) GetMembers() (err error) {
	org.Members, org.MembersIsPublic, err = FindOrgMembers(&FindOrgMembersOpts{
		OrgID: org.ID,
	})
	return
}

// FindOrgMembersOpts represensts find org members condtions
type FindOrgMembersOpts struct {
	ListOptions
	OrgID      int64
	PublicOnly bool
}

// CountOrgMembers counts the organization's members
func CountOrgMembers(opts FindOrgMembersOpts) (int64, error) {
	sess := x.Where("org_id=?", opts.OrgID)
	if opts.PublicOnly {
		sess.And("is_public = ?", true)
	}
	return sess.Count(new(OrgUser))
}

// FindOrgMembers loads organization members according conditions
func FindOrgMembers(opts *FindOrgMembersOpts) (UserList, map[int64]bool, error) {
	ous, err := GetOrgUsersByOrgID(opts)
	if err != nil {
		return nil, nil, err
	}

	var ids = make([]int64, len(ous))
	var idsIsPublic = make(map[int64]bool, len(ous))
	for i, ou := range ous {
		ids[i] = ou.UID
		idsIsPublic[ou.UID] = ou.IsPublic
	}

	users, err := GetUsersByIDs(ids)
	if err != nil {
		return nil, nil, err
	}
	return users, idsIsPublic, nil
}

// AddMember adds new member to organization.
func (org *User) AddMember(uid int64) error {
	return AddOrgUser(org.ID, uid)
}

// RemoveMember removes member from organization.
func (org *User) RemoveMember(uid int64) error {
	return RemoveOrgUser(org.ID, uid)
}

func (org *User) removeOrgRepo(e Engine, repoID int64) error {
	return removeOrgRepo(e, org.ID, repoID)
}

// RemoveOrgRepo removes all team-repository relations of organization.
func (org *User) RemoveOrgRepo(repoID int64) error {
	return org.removeOrgRepo(x, repoID)
}

// CreateOrganization creates record of a new organization.
func CreateOrganization(org, owner *User) (err error) {
	if !owner.CanCreateOrganization() {
		return ErrUserNotAllowedCreateOrg{}
	}

	if err = IsUsableUsername(org.Name); err != nil {
		return err
	}

	isExist, err := IsUserExist(0, org.Name)
	if err != nil {
		return err
	} else if isExist {
		return ErrUserAlreadyExist{org.Name}
	}

	org.LowerName = strings.ToLower(org.Name)
	if org.Rands, err = GetUserSalt(); err != nil {
		return err
	}
	if org.Salt, err = GetUserSalt(); err != nil {
		return err
	}
	org.UseCustomAvatar = true
	org.MaxRepoCreation = -1
	org.NumTeams = 1
	org.NumMembers = 1
	org.Type = UserTypeOrganization

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if _, err = sess.Insert(org); err != nil {
		return fmt.Errorf("insert organization: %v", err)
	}
	if err = org.generateRandomAvatar(sess); err != nil {
		return fmt.Errorf("generate random avatar: %v", err)
	}

	// Add initial creator to organization and owner team.
	if _, err = sess.Insert(&OrgUser{
		UID:   owner.ID,
		OrgID: org.ID,
	}); err != nil {
		return fmt.Errorf("insert org-user relation: %v", err)
	}

	// Create default owner team.
	t := &Team{
		OrgID:                   org.ID,
		LowerName:               strings.ToLower(ownerTeamName),
		Name:                    ownerTeamName,
		Authorize:               AccessModeOwner,
		NumMembers:              1,
		IncludesAllRepositories: true,
		CanCreateOrgRepo:        true,
	}
	if _, err = sess.Insert(t); err != nil {
		return fmt.Errorf("insert owner team: %v", err)
	}

	// insert units for team
	var units = make([]TeamUnit, 0, len(AllRepoUnitTypes))
	for _, tp := range AllRepoUnitTypes {
		units = append(units, TeamUnit{
			OrgID:  org.ID,
			TeamID: t.ID,
			Type:   tp,
		})
	}

	if _, err = sess.Insert(&units); err != nil {
		if err := sess.Rollback(); err != nil {
			log.Error("CreateOrganization: sess.Rollback: %v", err)
		}
		return err
	}

	if _, err = sess.Insert(&TeamUser{
		UID:    owner.ID,
		OrgID:  org.ID,
		TeamID: t.ID,
	}); err != nil {
		return fmt.Errorf("insert team-user relation: %v", err)
	}

	return sess.Commit()
}

// GetOrgByName returns organization by given name.
func GetOrgByName(name string) (*User, error) {
	if len(name) == 0 {
		return nil, ErrOrgNotExist{0, name}
	}
	u := &User{
		LowerName: strings.ToLower(name),
		Type:      UserTypeOrganization,
	}
	has, err := x.Get(u)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrOrgNotExist{0, name}
	}
	return u, nil
}

// CountOrganizations returns number of organizations.
func CountOrganizations() int64 {
	count, _ := x.
		Where("type=1").
		Count(new(User))
	return count
}

// DeleteOrganization completely and permanently deletes everything of organization.
func DeleteOrganization(org *User) (err error) {
	if !org.IsOrganization() {
		return fmt.Errorf("%s is a user not an organization", org.Name)
	}

	sess := x.NewSession()
	defer sess.Close()

	if err = sess.Begin(); err != nil {
		return err
	}

	if err = deleteOrg(sess, org); err != nil {
		if IsErrUserOwnRepos(err) {
			return err
		} else if err != nil {
			return fmt.Errorf("deleteOrg: %v", err)
		}
	}

	return sess.Commit()
}

func deleteOrg(e *xorm.Session, u *User) error {
	// Check ownership of repository.
	count, err := getRepositoryCount(e, u)
	if err != nil {
		return fmt.Errorf("GetRepositoryCount: %v", err)
	} else if count > 0 {
		return ErrUserOwnRepos{UID: u.ID}
	}

	if err := deleteBeans(e,
		&Team{OrgID: u.ID},
		&OrgUser{OrgID: u.ID},
		&TeamUser{OrgID: u.ID},
		&TeamUnit{OrgID: u.ID},
	); err != nil {
		return fmt.Errorf("deleteBeans: %v", err)
	}

	if _, err = e.ID(u.ID).Delete(new(User)); err != nil {
		return fmt.Errorf("Delete: %v", err)
	}

	// FIXME: system notice
	// Note: There are something just cannot be roll back,
	//	so just keep error logs of those operations.
	path := UserPath(u.Name)

	if err := util.RemoveAll(path); err != nil {
		return fmt.Errorf("Failed to RemoveAll %s: %v", path, err)
	}

	if len(u.Avatar) > 0 {
		avatarPath := u.CustomAvatarPath()
		if com.IsExist(avatarPath) {
			if err := util.Remove(avatarPath); err != nil {
				return fmt.Errorf("Failed to remove %s: %v", avatarPath, err)
			}
		}
	}

	return nil
}

// ________                ____ ___
// \_____  \_______  ____ |    |   \______ ___________
//  /   |   \_  __ \/ ___\|    |   /  ___// __ \_  __ \
// /    |    \  | \/ /_/  >    |  /\___ \\  ___/|  | \/
// \_______  /__|  \___  /|______//____  >\___  >__|
//         \/     /_____/              \/     \/

// OrgUser represents an organization-user relation.
type OrgUser struct {
	ID       int64 `xorm:"pk autoincr"`
	UID      int64 `xorm:"INDEX UNIQUE(s)"`
	OrgID    int64 `xorm:"INDEX UNIQUE(s)"`
	IsPublic bool  `xorm:"INDEX"`
}

// TableName sets the table name to `org_user`
func (o *OrgUser) TableName() string {
	return tbOrgUser[1 : len(tbOrgUser)-1]
}

func isOrganizationOwner(e Engine, orgID, uid int64) (bool, error) {
	ownerTeam, err := getOwnerTeam(e, orgID)
	if err != nil {
		if IsErrTeamNotExist(err) {
			log.Error("Organization does not have owner team: %d", orgID)
			return false, nil
		}
		return false, err
	}
	return isTeamMember(e, orgID, ownerTeam.ID, uid)
}

// IsOrganizationOwner returns true if given user is in the owner team.
func IsOrganizationOwner(orgID, uid int64) (bool, error) {
	return isOrganizationOwner(x, orgID, uid)
}

// IsOrganizationMember returns true if given user is member of organization.
func IsOrganizationMember(orgID, uid int64) (bool, error) {
	return isOrganizationMember(x, orgID, uid)
}

func isOrganizationMember(e Engine, orgID, uid int64) (bool, error) {
	return e.
		Where("uid=?", uid).
		And("org_id=?", orgID).
		Table(tbOrgUser).
		Exist()
}

// IsPublicMembership returns true if given user public his/her membership.
func IsPublicMembership(orgID, uid int64) (bool, error) {
	return x.
		Where("uid=?", uid).
		And("org_id=?", orgID).
		And("is_public=?", true).
		Table(tbOrgUser).
		Exist()
}

// CanCreateOrgRepo returns true if user can create repo in organization
func CanCreateOrgRepo(orgID, uid int64) (bool, error) {
	if owner, err := IsOrganizationOwner(orgID, uid); owner || err != nil {
		return owner, err
	}
	return x.
		Where(builder.Eq{tbTeam + ".can_create_org_repo": true}).
		Join("INNER", tbTeamUser, tbTeamUser+".team_id = "+tbTeam+".id").
		And(tbTeamUser+".uid = ?", uid).
		And(tbTeamUser+".org_id = ?", orgID).
		Exist(new(Team))
}

func getOrgsByUserID(sess *xorm.Session, userID int64, showAll bool) ([]*User, error) {
	orgs := make([]*User, 0, 10)
	if !showAll {
		sess.And(tbOrgUser+".is_public=?", true)
	}
	return orgs, sess.
		And(tbOrgUser+".uid=?", userID).
		Join("INNER", tbOrgUser, tbOrgUser+".org_id="+tbUser+".id").
		Asc(tbUser + ".name").
		Find(&orgs)
}

// GetOrgsByUserID returns a list of organizations that the given user ID
// has joined.
func GetOrgsByUserID(userID int64, showAll bool) ([]*User, error) {
	sess := x.NewSession()
	defer sess.Close()
	return getOrgsByUserID(sess, userID, showAll)
}

func getOwnedOrgsByUserID(sess *xorm.Session, userID int64) ([]*User, error) {
	orgs := make([]*User, 0, 10)
	return orgs, sess.
		Join("INNER", tbTeamUser, tbTeamUser+".org_id="+tbUser+".id").
		Join("INNER", tbTeam, tbTeam+".id="+tbTeamUser+".team_id").
		Where(tbTeamUser+".uid=?", userID).
		And(tbTeam+".authorize=?", AccessModeOwner).
		Asc(tbUser + ".name").
		Find(&orgs)
}

// HasOrgVisible tells if the given user can see the given org
func HasOrgVisible(org *User, user *User) bool {
	return hasOrgVisible(x, org, user)
}

func hasOrgVisible(e Engine, org *User, user *User) bool {
	// Not SignedUser
	if user == nil {
		return org.Visibility == structs.VisibleTypePublic
	}

	if user.IsAdmin {
		return true
	}

	if (org.Visibility == structs.VisibleTypePrivate || user.IsRestricted) && !org.hasMemberWithUserID(e, user.ID) {
		return false
	}
	return true
}

// HasOrgsVisible tells if the given user can see at least one of the orgs provided
func HasOrgsVisible(orgs []*User, user *User) bool {
	if len(orgs) == 0 {
		return false
	}

	for _, org := range orgs {
		if HasOrgVisible(org, user) {
			return true
		}
	}
	return false
}

// GetOwnedOrgsByUserID returns a list of organizations are owned by given user ID.
func GetOwnedOrgsByUserID(userID int64) ([]*User, error) {
	sess := x.NewSession()
	defer sess.Close()
	return getOwnedOrgsByUserID(sess, userID)
}

// GetOwnedOrgsByUserIDDesc returns a list of organizations are owned by
// given user ID, ordered descending by the given condition.
func GetOwnedOrgsByUserIDDesc(userID int64, desc string) ([]*User, error) {
	return getOwnedOrgsByUserID(x.Desc(desc), userID)
}

// GetOrgsCanCreateRepoByUserID returns a list of organizations where given user ID
// are allowed to create repos.
func GetOrgsCanCreateRepoByUserID(userID int64) ([]*User, error) {
	orgs := make([]*User, 0, 10)
	return orgs, x.Where(builder.In("id", builder.Select(tbUser+".id").From(tbUser).
		Join("INNER", tbTeamUser, tbTeamUser+".org_id = "+tbUser+".id").
		Join("INNER", tbTeam, tbTeam+".id = "+tbTeamUser+".team_id").
		Where(builder.Eq{tbTeamUser + ".uid": userID}).
		And(builder.Eq{tbTeam + ".authorize": AccessModeOwner}.
			Or(builder.Eq{tbTeam + ".can_create_org_repo": true})))).
		Desc(tbUser + ".updated_unix").Find(&orgs)
}

// GetOrgUsersByUserID returns all organization-user relations by user ID.
func GetOrgUsersByUserID(uid int64, opts *SearchOrganizationsOptions) ([]*OrgUser, error) {
	ous := make([]*OrgUser, 0, 10)
	sess := x.
		Join("LEFT", tbUser, tbOrgUser+".org_id="+tbUser+".id").
		Where(tbOrgUser+".uid=?", uid)
	if !opts.All {
		// Only show public organizations
		sess.And("is_public=?", true)
	}

	if opts.PageSize != 0 {
		sess = opts.setSessionPagination(sess)
	}

	err := sess.
		Asc(tbUser + ".name").
		Find(&ous)
	return ous, err
}

// GetOrgUsersByOrgID returns all organization-user relations by organization ID.
func GetOrgUsersByOrgID(opts *FindOrgMembersOpts) ([]*OrgUser, error) {
	return getOrgUsersByOrgID(x, opts)
}

func getOrgUsersByOrgID(e Engine, opts *FindOrgMembersOpts) ([]*OrgUser, error) {
	sess := e.Where("org_id=?", opts.OrgID)
	if opts.PublicOnly {
		sess.And("is_public = ?", true)
	}
	if opts.ListOptions.PageSize > 0 {
		sess = opts.setSessionPagination(sess)

		ous := make([]*OrgUser, 0, opts.PageSize)
		return ous, sess.Find(&ous)
	}

	var ous []*OrgUser
	return ous, sess.Find(&ous)
}

// ChangeOrgUserStatus changes public or private membership status.
func ChangeOrgUserStatus(orgID, uid int64, public bool) error {
	ou := new(OrgUser)
	has, err := x.
		Where("uid=?", uid).
		And("org_id=?", orgID).
		Get(ou)
	if err != nil {
		return err
	} else if !has {
		return nil
	}

	ou.IsPublic = public
	_, err = x.ID(ou.ID).Cols("is_public").Update(ou)
	return err
}

// AddOrgUser adds new user to given organization.
func AddOrgUser(orgID, uid int64) error {
	isAlreadyMember, err := IsOrganizationMember(orgID, uid)
	if err != nil || isAlreadyMember {
		return err
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	ou := &OrgUser{
		UID:      uid,
		OrgID:    orgID,
		IsPublic: setting.Service.DefaultOrgMemberVisible,
	}

	if _, err := sess.Insert(ou); err != nil {
		if err := sess.Rollback(); err != nil {
			log.Error("AddOrgUser: sess.Rollback: %v", err)
		}
		return err
	} else if _, err = sess.Exec("UPDATE "+tbUser+" SET num_members = num_members + 1 WHERE id = ?",
		orgID); err != nil {
		if err := sess.Rollback(); err != nil {
			log.Error("AddOrgUser: sess.Rollback: %v", err)
		}
		return err
	}

	return sess.Commit()
}

func removeOrgUser(sess *xorm.Session, orgID, userID int64) error {
	ou := new(OrgUser)

	has, err := sess.
		Where("uid=?", userID).
		And("org_id=?", orgID).
		Get(ou)
	if err != nil {
		return fmt.Errorf("get org-user: %v", err)
	} else if !has {
		return nil
	}

	org, err := getUserByID(sess, orgID)
	if err != nil {
		return fmt.Errorf("GetUserByID [%d]: %v", orgID, err)
	}

	// Check if the user to delete is the last member in owner team.
	if isOwner, err := isOrganizationOwner(sess, orgID, userID); err != nil {
		return err
	} else if isOwner {
		t, err := org.getOwnerTeam(sess)
		if err != nil {
			return err
		}
		if t.NumMembers == 1 {
			if err := t.getMembers(sess); err != nil {
				return err
			}
			if t.Members[0].ID == userID {
				return ErrLastOrgOwner{UID: userID}
			}
		}
	}

	if _, err := sess.ID(ou.ID).Delete(ou); err != nil {
		return err
	} else if _, err = sess.Exec("UPDATE "+tbUser+" SET num_members=num_members-1 WHERE id=?",
		orgID); err != nil {
		return err
	}

	// Delete all repository accesses and unwatch them.
	env, err := org.accessibleReposEnv(sess, userID)
	if err != nil {
		return fmt.Errorf("AccessibleReposEnv: %v", err)
	}
	repoIDs, err := env.RepoIDs(1, org.NumRepos)
	if err != nil {
		return fmt.Errorf("GetUserRepositories [%d]: %v", userID, err)
	}
	for _, repoID := range repoIDs {
		if err = watchRepo(sess, userID, repoID, false); err != nil {
			return err
		}
	}

	if len(repoIDs) > 0 {
		if _, err = sess.
			Where("user_id = ?", userID).
			In("repo_id", repoIDs).
			Delete(new(Access)); err != nil {
			return err
		}
	}

	// Delete member in his/her teams.
	teams, err := getUserOrgTeams(sess, org.ID, userID)
	if err != nil {
		return err
	}
	for _, t := range teams {
		if err = removeTeamMember(sess, t, userID); err != nil {
			return err
		}
	}

	return nil
}

// RemoveOrgUser removes user from given organization.
func RemoveOrgUser(orgID, userID int64) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	if err := removeOrgUser(sess, orgID, userID); err != nil {
		return err
	}
	return sess.Commit()
}

func removeOrgRepo(e Engine, orgID, repoID int64) error {
	teamRepos := make([]*TeamRepo, 0, 10)
	if err := e.Find(&teamRepos, &TeamRepo{OrgID: orgID, RepoID: repoID}); err != nil {
		return err
	}

	if len(teamRepos) == 0 {
		return nil
	}

	if _, err := e.Delete(&TeamRepo{
		OrgID:  orgID,
		RepoID: repoID,
	}); err != nil {
		return err
	}

	teamIDs := make([]int64, len(teamRepos))
	for i, teamRepo := range teamRepos {
		teamIDs[i] = teamRepo.TeamID
	}

	_, err := e.Decr("num_repos").In("id", teamIDs).Update(new(Team))
	return err
}

func (org *User) getUserTeams(e Engine, userID int64, cols ...string) ([]*Team, error) {
	teams := make([]*Team, 0, org.NumTeams)
	return teams, e.
		Where(tbTeamUser+".org_id = ?", org.ID).
		Join("INNER", tbTeamUser, tbTeamUser+".team_id = "+tbTeam+".id").
		Join("INNER", tbUser, tbUser+".id="+tbTeamUser+".uid").
		And(tbTeamUser+".uid = ?", userID).
		Asc(tbUser + ".name").
		Cols(cols...).
		Find(&teams)
}

func (org *User) getUserTeamIDs(e Engine, userID int64) ([]int64, error) {
	teamIDs := make([]int64, 0, org.NumTeams)
	return teamIDs, e.
		Table(tbTeam).
		Cols(tbTeam+".id").
		Where(tbTeamUser+".org_id = ?", org.ID).
		Join("INNER", tbTeamUser, tbTeamUser+".team_id = "+tbTeam+".id").
		And(tbTeamUser+".uid = ?", userID).
		Find(&teamIDs)
}

// TeamsWithAccessToRepo returns all teams that have given access level to the repository.
func (org *User) TeamsWithAccessToRepo(repoID int64, mode AccessMode) ([]*Team, error) {
	return GetTeamsWithAccessToRepo(org.ID, repoID, mode)
}

// GetUserTeamIDs returns of all team IDs of the organization that user is member of.
func (org *User) GetUserTeamIDs(userID int64) ([]int64, error) {
	return org.getUserTeamIDs(x, userID)
}

// GetUserTeams returns all teams that belong to user,
// and that the user has joined.
func (org *User) GetUserTeams(userID int64) ([]*Team, error) {
	return org.getUserTeams(x, userID)
}

// AccessibleReposEnvironment operations involving the repositories that are
// accessible to a particular user
type AccessibleReposEnvironment interface {
	CountRepos() (int64, error)
	RepoIDs(page, pageSize int) ([]int64, error)
	Repos(page, pageSize int) ([]*Repository, error)
	MirrorRepos() ([]*Repository, error)
	AddKeyword(keyword string)
	SetSort(SearchOrderBy)
}

type accessibleReposEnv struct {
	org     *User
	user    *User
	teamIDs []int64
	e       Engine
	keyword string
	orderBy SearchOrderBy
}

// AccessibleReposEnv an AccessibleReposEnvironment for the repositories in `org`
// that are accessible to the specified user.
func (org *User) AccessibleReposEnv(userID int64) (AccessibleReposEnvironment, error) {
	return org.accessibleReposEnv(x, userID)
}

func (org *User) accessibleReposEnv(e Engine, userID int64) (AccessibleReposEnvironment, error) {
	var user *User

	if userID > 0 {
		u, err := getUserByID(e, userID)
		if err != nil {
			return nil, err
		}
		user = u
	}

	teamIDs, err := org.getUserTeamIDs(e, userID)
	if err != nil {
		return nil, err
	}
	return &accessibleReposEnv{
		org:     org,
		user:    user,
		teamIDs: teamIDs,
		e:       e,
		orderBy: SearchOrderByRecentUpdated,
	}, nil
}

func (env *accessibleReposEnv) cond() builder.Cond {
	var (
		cond = builder.NewCond()
	)

	if env.user == nil || !env.user.IsRestricted {
		cond = cond.Or(builder.Eq{
			tbRepository + ".owner_id":   env.org.ID,
			tbRepository + ".is_private": false,
		})
	}
	if len(env.teamIDs) > 0 {
		cond = cond.Or(builder.In(tbTeamRepo+".team_id", env.teamIDs))
	}
	if env.keyword != "" {
		cond = cond.And(builder.Like{tbRepository + ".lower_name", strings.ToLower(env.keyword)})
	}
	return cond
}

func (env *accessibleReposEnv) CountRepos() (int64, error) {
	repoCount, err := env.e.
		Join("INNER", tbTeamRepo, tbTeamRepo+".repo_id="+tbRepository+".id").
		Where(env.cond()).
		Distinct(tbRepository + ".id").
		Count(&Repository{})
	if err != nil {
		return 0, fmt.Errorf("count user repositories in organization: %v", err)
	}
	return repoCount, nil
}

func (env *accessibleReposEnv) RepoIDs(page, pageSize int) ([]int64, error) {
	if page <= 0 {
		page = 1
	}

	repoIDs := make([]int64, 0, pageSize)
	return repoIDs, env.e.
		Table(tbRepository).
		Join("INNER", tbTeamRepo, tbTeamRepo+".repo_id="+tbRepository+".id").
		Where(env.cond()).
		GroupBy(tbRepository+".id,"+tbRepository+"."+strings.Fields(string(env.orderBy))[0]).
		OrderBy(string(env.orderBy)).
		Limit(pageSize, (page-1)*pageSize).
		Cols(tbRepository + ".id").
		Find(&repoIDs)
}

func (env *accessibleReposEnv) Repos(page, pageSize int) ([]*Repository, error) {
	repoIDs, err := env.RepoIDs(page, pageSize)
	if err != nil {
		return nil, fmt.Errorf("GetUserRepositoryIDs: %v", err)
	}

	repos := make([]*Repository, 0, len(repoIDs))
	if len(repoIDs) == 0 {
		return repos, nil
	}

	return repos, env.e.
		In(tbRepository+".id", repoIDs).
		OrderBy(string(env.orderBy)).
		Find(&repos)
}

func (env *accessibleReposEnv) MirrorRepoIDs() ([]int64, error) {
	repoIDs := make([]int64, 0, 10)
	return repoIDs, env.e.
		Table(tbRepository).
		Join("INNER", tbTeamRepo, tbTeamRepo+".repo_id="+tbRepository+".id AND "+tbRepository+".is_mirror=?", true).
		Where(env.cond()).
		GroupBy(tbRepository + ".id, " + tbRepository + ".updated_unix").
		OrderBy(string(env.orderBy)).
		Cols(tbRepository + ".id").
		Find(&repoIDs)
}

func (env *accessibleReposEnv) MirrorRepos() ([]*Repository, error) {
	repoIDs, err := env.MirrorRepoIDs()
	if err != nil {
		return nil, fmt.Errorf("MirrorRepoIDs: %v", err)
	}

	repos := make([]*Repository, 0, len(repoIDs))
	if len(repoIDs) == 0 {
		return repos, nil
	}

	return repos, env.e.
		In(tbRepository+".id", repoIDs).
		Find(&repos)
}

func (env *accessibleReposEnv) AddKeyword(keyword string) {
	env.keyword = keyword
}

func (env *accessibleReposEnv) SetSort(orderBy SearchOrderBy) {
	env.orderBy = orderBy
}
