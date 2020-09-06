// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// RepositoryListDefaultPageSize is the default number of repositories
// to load in memory when running administrative tasks on all (or almost
// all) of them.
// The number should be low enough to avoid filling up all RAM with
// repository data...
const RepositoryListDefaultPageSize = 64

// RepositoryList contains a list of repositories
type RepositoryList []*Repository

func (repos RepositoryList) Len() int {
	return len(repos)
}

func (repos RepositoryList) Less(i, j int) bool {
	return repos[i].FullName() < repos[j].FullName()
}

func (repos RepositoryList) Swap(i, j int) {
	repos[i], repos[j] = repos[j], repos[i]
}

// RepositoryListOfMap make list from values of map
func RepositoryListOfMap(repoMap map[int64]*Repository) RepositoryList {
	return RepositoryList(valuesRepository(repoMap))
}

func (repos RepositoryList) loadAttributes(e Engine) error {
	if len(repos) == 0 {
		return nil
	}

	set := make(map[int64]struct{})
	repoIDs := make([]int64, len(repos))
	for i := range repos {
		set[repos[i].OwnerID] = struct{}{}
		repoIDs[i] = repos[i].ID
	}

	// Load owners.
	users := make(map[int64]*User, len(set))
	if err := e.
		Where("id > 0").
		In("id", keysInt64(set)).
		Find(&users); err != nil {
		return fmt.Errorf("find users: %v", err)
	}
	for i := range repos {
		repos[i].Owner = users[repos[i].OwnerID]
	}

	// Load primary language.
	stats := make(LanguageStatList, 0, len(repos))
	if err := e.
		Where("`is_primary` = ? AND `language` != ?", true, "other").
		In("`repo_id`", repoIDs).
		Find(&stats); err != nil {
		return fmt.Errorf("find primary languages: %v", err)
	}
	stats.loadAttributes()
	for i := range repos {
		for _, st := range stats {
			if st.RepoID == repos[i].ID {
				repos[i].PrimaryLanguage = st
				break
			}
		}
	}

	return nil
}

// LoadAttributes loads the attributes for the given RepositoryList
func (repos RepositoryList) LoadAttributes() error {
	return repos.loadAttributes(x)
}

// MirrorRepositoryList contains the mirror repositories
type MirrorRepositoryList []*Repository

func (repos MirrorRepositoryList) loadAttributes(e Engine) error {
	if len(repos) == 0 {
		return nil
	}

	// Load mirrors.
	repoIDs := make([]int64, 0, len(repos))
	for i := range repos {
		if !repos[i].IsMirror {
			continue
		}

		repoIDs = append(repoIDs, repos[i].ID)
	}
	mirrors := make([]*Mirror, 0, len(repoIDs))
	if err := e.
		Where("id > 0").
		In("repo_id", repoIDs).
		Find(&mirrors); err != nil {
		return fmt.Errorf("find mirrors: %v", err)
	}

	set := make(map[int64]*Mirror)
	for i := range mirrors {
		set[mirrors[i].RepoID] = mirrors[i]
	}
	for i := range repos {
		repos[i].Mirror = set[repos[i].ID]
	}
	return nil
}

// LoadAttributes loads the attributes for the given MirrorRepositoryList
func (repos MirrorRepositoryList) LoadAttributes() error {
	return repos.loadAttributes(x)
}

// SearchRepoOptions holds the search options
type SearchRepoOptions struct {
	ListOptions
	Actor           *User
	Keyword         string
	OwnerID         int64
	PriorityOwnerID int64
	OrderBy         SearchOrderBy
	Private         bool // Include private repositories in results
	StarredByID     int64
	AllPublic       bool // Include also all public repositories of users and public organisations
	AllLimited      bool // Include also all public repositories of limited organisations
	// None -> include public and private
	// True -> include just private
	// False -> incude just public
	IsPrivate util.OptionalBool
	// None -> include collaborative AND non-collaborative
	// True -> include just collaborative
	// False -> incude just non-collaborative
	Collaborate util.OptionalBool
	// None -> include forks AND non-forks
	// True -> include just forks
	// False -> include just non-forks
	Fork util.OptionalBool
	// None -> include templates AND non-templates
	// True -> include just templates
	// False -> include just non-templates
	Template util.OptionalBool
	// None -> include mirrors AND non-mirrors
	// True -> include just mirrors
	// False -> include just non-mirrors
	Mirror util.OptionalBool
	// None -> include archived AND non-archived
	// True -> include just archived
	// False -> include just non-archived
	Archived util.OptionalBool
	// only search topic name
	TopicOnly bool
	// include description in keyword search
	IncludeDescription bool
	// None -> include has milestones AND has no milestone
	// True -> include just has milestones
	// False -> include just has no milestone
	HasMilestones util.OptionalBool
}

//SearchOrderBy is used to sort the result
type SearchOrderBy string

func (s SearchOrderBy) String() string {
	return string(s)
}

// Strings for sorting result
const (
	SearchOrderByAlphabetically        SearchOrderBy = "name ASC"
	SearchOrderByAlphabeticallyReverse SearchOrderBy = "name DESC"
	SearchOrderByLeastUpdated          SearchOrderBy = "updated_unix ASC"
	SearchOrderByRecentUpdated         SearchOrderBy = "updated_unix DESC"
	SearchOrderByOldest                SearchOrderBy = "created_unix ASC"
	SearchOrderByNewest                SearchOrderBy = "created_unix DESC"
	SearchOrderBySize                  SearchOrderBy = "size ASC"
	SearchOrderBySizeReverse           SearchOrderBy = "size DESC"
	SearchOrderByID                    SearchOrderBy = "id ASC"
	SearchOrderByIDReverse             SearchOrderBy = "id DESC"
	SearchOrderByStars                 SearchOrderBy = "num_stars ASC"
	SearchOrderByStarsReverse          SearchOrderBy = "num_stars DESC"
	SearchOrderByForks                 SearchOrderBy = "num_forks ASC"
	SearchOrderByForksReverse          SearchOrderBy = "num_forks DESC"
)

// SearchRepositoryCondition creates a query condition according search repository options
func SearchRepositoryCondition(opts *SearchRepoOptions) builder.Cond {
	var cond = builder.NewCond()
	if opts.Private {
		if opts.Actor != nil && !opts.Actor.IsAdmin && opts.Actor.ID != opts.OwnerID {
			// OK we're in the context of a User
			cond = cond.And(accessibleRepositoryCondition(opts.Actor))
		}
	} else {
		// Not looking at private organisations
		// We should be able to see all non-private repositories that
		// isn't in a private or limited organisation.
		cond = cond.And(
			builder.Eq{"is_private": false},
			builder.NotIn("owner_id", builder.Select("id").From(tbUser).Where(
				builder.And(
					builder.Eq{"type": UserTypeOrganization},
					builder.Or(builder.Eq{"visibility": structs.VisibleTypeLimited},
						builder.Eq{"visibility": structs.VisibleTypePrivate}),
				))))
	}

	if opts.IsPrivate != util.OptionalBoolNone {
		cond = cond.And(builder.Eq{"is_private": opts.IsPrivate.IsTrue()})
	}

	if opts.Template != util.OptionalBoolNone {
		cond = cond.And(builder.Eq{"is_template": opts.Template == util.OptionalBoolTrue})
	}

	// Restrict to starred repositories
	if opts.StarredByID > 0 {
		cond = cond.And(builder.In("id", builder.Select("repo_id").
			From(tbStar).
			Where(builder.Eq{"uid": opts.StarredByID})))
	}

	// Restrict repositories to those the OwnerID owns or contributes to as per opts.Collaborate
	if opts.OwnerID > 0 {
		var accessCond = builder.NewCond()
		if opts.Collaborate != util.OptionalBoolTrue {
			accessCond = builder.Eq{"owner_id": opts.OwnerID}
		}

		if opts.Collaborate != util.OptionalBoolFalse {
			// A Collaboration is:
			collaborateCond := builder.And(
				// 1. Repository we don't own
				builder.Neq{"owner_id": opts.OwnerID},
				// 2. But we can see because of:
				builder.Or(
					// A. We have access
					builder.In(tbRepository+".id",
						builder.Select(tbAccess+".repo_id").
							From(tbAccess).
							Where(builder.Eq{tbAccess + ".user_id": opts.OwnerID})),
					// B. We are in a team for
					builder.In(tbRepository+".id", builder.Select(tbTeamRepo+".repo_id").
						From(tbTeamRepo).
						Where(builder.Eq{tbTeamUser + ".uid": opts.OwnerID}).
						Join("INNER", tbTeamUser, tbTeamUser+".team_id = "+tbTeamRepo+".team_id")),
					// C. Public repositories in private organizations that we are member of
					builder.And(
						builder.Eq{tbRepository + ".is_private": false},
						builder.In(tbRepository+".owner_id",
							builder.Select(tbOrgUser+".org_id").
								From(tbOrgUser).
								Join("INNER", tbUser, tbUser+".id = "+tbOrgUser+".org_id").
								Where(builder.Eq{
									tbOrgUser + ".uid":     opts.OwnerID,
									tbUser + ".type":       UserTypeOrganization,
									tbUser + ".visibility": structs.VisibleTypePrivate,
								})))),
			)
			if !opts.Private {
				collaborateCond = collaborateCond.And(
					builder.Expr("owner_id NOT IN (SELECT org_id FROM "+tbOrgUser+" WHERE "+
						tbOrgUser+".uid = ? AND "+tbOrgUser+".is_public = ?)",
						opts.OwnerID, false))
			}

			accessCond = accessCond.Or(collaborateCond)
		}

		if opts.AllPublic {
			accessCond = accessCond.Or(builder.Eq{"is_private": false}.
				And(builder.In("owner_id", builder.Select(tbUser+".id").
					From(tbUser).Where(builder.Eq{tbUser + ".visibility": structs.VisibleTypePublic}))))
		}

		if opts.AllLimited {
			accessCond = accessCond.Or(builder.Eq{"is_private": false}.
				And(builder.In("owner_id",
					builder.Select(tbUser+".id").
						From(tbUser).
						Where(builder.Eq{tbUser + ".visibility": structs.VisibleTypeLimited}))))
		}

		cond = cond.And(accessCond)
	}

	if opts.Keyword != "" {
		// separate keyword
		var subQueryCond = builder.NewCond()
		for _, v := range strings.Split(opts.Keyword, ",") {
			if opts.TopicOnly {
				subQueryCond = subQueryCond.Or(builder.Eq{tbTopic + ".name": strings.ToLower(v)})
			} else {
				subQueryCond = subQueryCond.Or(builder.Like{tbTopic + ".name", strings.ToLower(v)})
			}
		}
		subQuery := builder.Select(tbRepoTopic+".repo_id").From(tbRepoTopic).
			Join("INNER", tbTopic, tbTopic+".id = "+tbRepoTopic+".topic_id").
			Where(subQueryCond).
			GroupBy(tbRepoTopic + ".repo_id")

		var keywordCond = builder.In("id", subQuery)
		if !opts.TopicOnly {
			var likes = builder.NewCond()
			for _, v := range strings.Split(opts.Keyword, ",") {
				likes = likes.Or(builder.Like{"lower_name", strings.ToLower(v)})
				if opts.IncludeDescription {
					likes = likes.Or(builder.Like{"LOWER(description)", strings.ToLower(v)})
				}
			}
			keywordCond = keywordCond.Or(likes)
		}
		cond = cond.And(keywordCond)
	}

	if opts.Fork != util.OptionalBoolNone {
		cond = cond.And(builder.Eq{"is_fork": opts.Fork == util.OptionalBoolTrue})
	}

	if opts.Mirror != util.OptionalBoolNone {
		cond = cond.And(builder.Eq{"is_mirror": opts.Mirror == util.OptionalBoolTrue})
	}

	if opts.Actor != nil && opts.Actor.IsRestricted {
		cond = cond.And(accessibleRepositoryCondition(opts.Actor))
	}

	if opts.Archived != util.OptionalBoolNone {
		cond = cond.And(builder.Eq{"is_archived": opts.Archived == util.OptionalBoolTrue})
	}

	switch opts.HasMilestones {
	case util.OptionalBoolTrue:
		cond = cond.And(builder.Gt{"num_milestones": 0})
	case util.OptionalBoolFalse:
		cond = cond.And(builder.Eq{"num_milestones": 0}.Or(builder.IsNull{"num_milestones"}))
	}

	return cond
}

// SearchRepository returns repositories based on search options,
// it returns results in given range and number of total results.
func SearchRepository(opts *SearchRepoOptions) (RepositoryList, int64, error) {
	cond := SearchRepositoryCondition(opts)
	return SearchRepositoryByCondition(opts, cond, true)
}

// SearchRepositoryByCondition search repositories by condition
func SearchRepositoryByCondition(opts *SearchRepoOptions, cond builder.Cond, loadAttributes bool) (RepositoryList, int64, error) {
	if opts.Page <= 0 {
		opts.Page = 1
	}

	if len(opts.OrderBy) == 0 {
		opts.OrderBy = SearchOrderByAlphabetically
	}

	if opts.PriorityOwnerID > 0 {
		opts.OrderBy = SearchOrderBy(fmt.Sprintf("CASE WHEN owner_id = %d THEN 0 ELSE owner_id END, %s", opts.PriorityOwnerID, opts.OrderBy))
	}

	sess := x.NewSession()
	defer sess.Close()

	count, err := sess.
		Where(cond).
		Count(new(Repository))

	if err != nil {
		return nil, 0, fmt.Errorf("Count: %v", err)
	}

	repos := make(RepositoryList, 0, opts.PageSize)
	sess.Where(cond).OrderBy(opts.OrderBy.String())
	if opts.PageSize > 0 {
		sess.Limit(opts.PageSize, (opts.Page-1)*opts.PageSize)
	}
	if err = sess.Find(&repos); err != nil {
		return nil, 0, fmt.Errorf("Repo: %v", err)
	}

	if loadAttributes {
		if err = repos.loadAttributes(sess); err != nil {
			return nil, 0, fmt.Errorf("LoadAttributes: %v", err)
		}
	}

	return repos, count, nil
}

// accessibleRepositoryCondition takes a user a returns a condition for checking if a repository is accessible
func accessibleRepositoryCondition(user *User) builder.Cond {
	var cond = builder.NewCond()
	if user == nil || !user.IsRestricted || user.ID <= 0 {
		orgVisibilityLimit := []structs.VisibleType{structs.VisibleTypePrivate}
		if user == nil || user.ID <= 0 {
			orgVisibilityLimit = append(orgVisibilityLimit, structs.VisibleTypeLimited)
		}
		// 1. Be able to see all non-private repositories that either:
		cond = cond.Or(builder.And(
			builder.Eq{tbRepository + ".is_private": false},
			// 2. Aren't in an private organisation or limited organisation if we're not logged in
			builder.NotIn(tbRepository+".owner_id", builder.Select("id").From(tbUser).
				Where(
					builder.And(
						builder.Eq{"type": UserTypeOrganization},
						builder.In("visibility", orgVisibilityLimit)),
				))))
	}

	if user != nil {
		cond = cond.Or(
			// 2. Be able to see all repositories that we have access to
			builder.In(tbRepository+".id", builder.Select("repo_id").
				From(tbAccess).
				Where(builder.And(
					builder.Eq{"user_id": user.ID},
					builder.Gt{"mode": int(AccessModeNone)}))),
			// 3. Repositories that we directly own
			builder.Eq{tbRepository + ".owner_id": user.ID},
			// 4. Be able to see all repositories that we are in a team
			builder.In(tbRepository+".id", builder.Select(tbTeamRepo+".repo_id").
				From(tbTeamRepo).
				Where(builder.Eq{tbTeamUser + ".uid": user.ID}).
				Join("INNER", tbTeamUser, tbTeamUser+".team_id = "+tbTeamRepo+".team_id")),
			// 5. Be able to see all public repos in private organizations that we are an org_user of
			builder.And(builder.Eq{tbRepository + ".is_private": false},
				builder.In(tbRepository+".owner_id",
					builder.Select(tbOrgUser+".org_id").
						From(tbOrgUser).
						Where(builder.Eq{tbOrgUser + ".uid": user.ID}))))
	}

	return cond
}

// SearchRepositoryByName takes keyword and part of repository name to search,
// it returns results in given range and number of total results.
func SearchRepositoryByName(opts *SearchRepoOptions) (RepositoryList, int64, error) {
	opts.IncludeDescription = false
	return SearchRepository(opts)
}

// AccessibleRepoIDsQuery queries accessible repository ids. Usable as a subquery wherever repo ids need to be filtered.
func AccessibleRepoIDsQuery(user *User) *builder.Builder {
	// NB: Please note this code needs to still work if user is nil
	return builder.Select("id").From(tbRepository).Where(accessibleRepositoryCondition(user))
}

// FindUserAccessibleRepoIDs find all accessible repositories' ID by user's id
func FindUserAccessibleRepoIDs(user *User) ([]int64, error) {
	repoIDs := make([]int64, 0, 10)
	if err := x.
		Table(tbRepository).
		Cols("id").
		Where(accessibleRepositoryCondition(user)).
		Find(&repoIDs); err != nil {
		return nil, fmt.Errorf("FindUserAccesibleRepoIDs: %v", err)
	}
	return repoIDs, nil
}
