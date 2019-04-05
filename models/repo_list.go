// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"

	"github.com/go-xorm/builder"
	"github.com/go-xorm/core"
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

	// Load owners.
	set := make(map[int64]struct{})
	for i := range repos {
		set[repos[i].OwnerID] = struct{}{}
	}
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
	Keyword   string
	OwnerID   int64
	OrderBy   SearchOrderBy
	Private   bool // Include private repositories in results
	Starred   bool
	Page      int
	AllPublic bool // Include also all public repositories
	PageSize  int  // Can be smaller than or equal to setting.ExplorePagingNum
	// None -> include collaborative AND non-collaborative
	// True -> include just collaborative
	// False -> incude just non-collaborative
	Collaborate util.OptionalBool
	// None -> include forks AND non-forks
	// True -> include just forks
	// False -> include just non-forks
	Fork util.OptionalBool
	// None -> include mirrors AND non-mirrors
	// True -> include just mirrors
	// False -> include just non-mirrors
	Mirror util.OptionalBool
	// only search topic name
	TopicOnly bool
}

//SearchOrderBy is used to sort the result
type SearchOrderBy string

func (s SearchOrderBy) String() string {
	return string(s)
}

// Strings for sorting result
const (
	SearchOrderByAlphabetically        SearchOrderBy = "name ASC"
	SearchOrderByAlphabeticallyReverse               = "name DESC"
	SearchOrderByLeastUpdated                        = "updated_unix ASC"
	SearchOrderByRecentUpdated                       = "updated_unix DESC"
	SearchOrderByOldest                              = "created_unix ASC"
	SearchOrderByNewest                              = "created_unix DESC"
	SearchOrderBySize                                = "size ASC"
	SearchOrderBySizeReverse                         = "size DESC"
	SearchOrderByID                                  = "id ASC"
	SearchOrderByIDReverse                           = "id DESC"
	SearchOrderByStars                               = "num_stars ASC"
	SearchOrderByStarsReverse                        = "num_stars DESC"
	SearchOrderByForks                               = "num_forks ASC"
	SearchOrderByForksReverse                        = "num_forks DESC"
)

// SearchRepositoryByName takes keyword and part of repository name to search,
// it returns results in given range and number of total results.
func SearchRepositoryByName(opts *SearchRepoOptions) (RepositoryList, int64, error) {
	if opts.Page <= 0 {
		opts.Page = 1
	}

	var cond = builder.NewCond()

	if !opts.Private {
		cond = cond.And(builder.Eq{"is_private": false})
		accessCond := builder.Or(
			builder.NotIn("owner_id", builder.Select("id").From("`user`").Where(builder.Or(builder.Eq{"visibility": structs.VisibleTypeLimited}, builder.Eq{"visibility": structs.VisibleTypePrivate}))),
			builder.NotIn("owner_id", builder.Select("id").From("`user`").Where(builder.Eq{"type": UserTypeOrganization})))
		cond = cond.And(accessCond)
	}

	if opts.OwnerID > 0 {
		if opts.Starred {
			cond = cond.And(builder.In("id", builder.Select("repo_id").From("star").Where(builder.Eq{"uid": opts.OwnerID})))
		} else {
			var accessCond = builder.NewCond()
			if opts.Collaborate != util.OptionalBoolTrue {
				accessCond = builder.Eq{"owner_id": opts.OwnerID}
			}

			if opts.Collaborate != util.OptionalBoolFalse {
				collaborateCond := builder.And(
					builder.Expr("repository.id IN (SELECT repo_id FROM `access` WHERE access.user_id = ?)", opts.OwnerID),
					builder.Neq{"owner_id": opts.OwnerID})
				if !opts.Private {
					collaborateCond = collaborateCond.And(builder.Expr("owner_id NOT IN (SELECT org_id FROM org_user WHERE org_user.uid = ? AND org_user.is_public = ?)", opts.OwnerID, false))
				}

				accessCond = accessCond.Or(collaborateCond)
			}

			var exprCond builder.Cond
			if DbCfg.Type == core.POSTGRES {
				exprCond = builder.Expr("org_user.org_id = \"user\".id")
			} else if DbCfg.Type == core.MSSQL {
				exprCond = builder.Expr("org_user.org_id = [user].id")
			} else {
				exprCond = builder.Eq{"org_user.org_id": "user.id"}
			}

			visibilityCond := builder.Or(
				builder.In("owner_id",
					builder.Select("org_id").From("org_user").
						LeftJoin("`user`", exprCond).
						Where(
							builder.And(
								builder.Eq{"uid": opts.OwnerID},
								builder.Eq{"visibility": structs.VisibleTypePrivate})),
				),
				builder.In("owner_id",
					builder.Select("id").From("`user`").
						Where(
							builder.Or(
								builder.Eq{"visibility": structs.VisibleTypePublic},
								builder.Eq{"visibility": structs.VisibleTypeLimited})),
				),
				builder.NotIn("owner_id", builder.Select("id").From("`user`").Where(builder.Eq{"type": UserTypeOrganization})),
			)
			cond = cond.And(visibilityCond)

			if opts.AllPublic {
				accessCond = accessCond.Or(builder.Eq{"is_private": false})
			}

			cond = cond.And(accessCond)
		}
	}

	if opts.Keyword != "" {
		// separate keyword
		var subQueryCond = builder.NewCond()
		for _, v := range strings.Split(opts.Keyword, ",") {
			subQueryCond = subQueryCond.Or(builder.Like{"topic.name", strings.ToLower(v)})
		}
		subQuery := builder.Select("repo_topic.repo_id").From("repo_topic").
			Join("INNER", "topic", "topic.id = repo_topic.topic_id").
			Where(subQueryCond).
			GroupBy("repo_topic.repo_id")

		var keywordCond = builder.In("id", subQuery)
		if !opts.TopicOnly {
			var likes = builder.NewCond()
			for _, v := range strings.Split(opts.Keyword, ",") {
				likes = likes.Or(builder.Like{"lower_name", strings.ToLower(v)})
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

	if len(opts.OrderBy) == 0 {
		opts.OrderBy = SearchOrderByAlphabetically
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
	if err = sess.
		Where(cond).
		OrderBy(opts.OrderBy.String()).
		Limit(opts.PageSize, (opts.Page-1)*opts.PageSize).
		Find(&repos); err != nil {
		return nil, 0, fmt.Errorf("Repo: %v", err)
	}

	if err = repos.loadAttributes(sess); err != nil {
		return nil, 0, fmt.Errorf("LoadAttributes: %v", err)
	}

	return repos, count, nil
}

// FindUserAccessibleRepoIDs find all accessible repositories' ID by user's id
func FindUserAccessibleRepoIDs(userID int64) ([]int64, error) {
	var accessCond builder.Cond = builder.Eq{"is_private": false}

	if userID > 0 {
		accessCond = accessCond.Or(
			builder.Eq{"owner_id": userID},
			builder.And(
				builder.Expr("id IN (SELECT repo_id FROM `access` WHERE access.user_id = ?)", userID),
				builder.Neq{"owner_id": userID},
			),
		)
	}

	repoIDs := make([]int64, 0, 10)
	if err := x.
		Table("repository").
		Cols("id").
		Where(accessCond).
		Find(&repoIDs); err != nil {
		return nil, fmt.Errorf("FindUserAccesibleRepoIDs: %v", err)
	}
	return repoIDs, nil
}
