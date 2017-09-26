// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"github.com/go-xorm/xorm"
)

func removeDuplicateUnitTypes(x *xorm.Engine) error {
	// RepoUnit describes all units of a repository
	type RepoUnit struct {
		RepoID int64
		Type   int
	}

	// Repo describes a repository
	type Repo struct {
		ID int64
	}

	// Enumerate all the unit types
	const (
		UnitTypeCode             = iota + 1 // 1 code
		UnitTypeIssues                      // 2 issues
		UnitTypePullRequests                // 3 PRs
		UnitTypeReleases                    // 4 Releases
		UnitTypeWiki                        // 5 Wiki
		UnitTypeExternalWiki                // 6 ExternalWiki
		UnitTypeExternalTracker             // 7 ExternalTracker
	)

	var externalIssueRepos []Repo
	err := x.Table("repository").Select("`repository`.id").
		Join("INNER", "repo_unit", "`repo_unit`.repo_id = `repository`.id").
		Where("`repo_unit`.type = ?", UnitTypeExternalTracker).
		Find(&externalIssueRepos)
	if err != nil {
		return fmt.Errorf("Query repositories: %v", err)
	}

	var externalWikiRepos []Repo
	err = x.Table("repository").Select("`repository`.id").
		Join("INNER", "repo_unit", "`repo_unit`.repo_id = `repository`.id").
		Where("`repo_unit`.type = ?", UnitTypeExternalWiki).
		Find(&externalWikiRepos)
	if err != nil {
		return fmt.Errorf("Query repositories: %v", err)
	}

	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	for _, repo := range externalIssueRepos {
		if _, err = sess.Delete(&RepoUnit{
			RepoID: repo.ID,
			Type:   UnitTypeIssues,
		}); err != nil {
			return fmt.Errorf("Delete repo unit: %v", err)
		}
	}

	for _, repo := range externalWikiRepos {
		if _, err = sess.Delete(&RepoUnit{
			RepoID: repo.ID,
			Type:   UnitTypeWiki,
		}); err != nil {
			return fmt.Errorf("Delete repo unit: %v", err)
		}
	}

	return sess.Commit()
}
