// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issues

import "code.gitea.io/gitea/models"

// DBIndexer implements Indexer inteface to use database's like search
type DBIndexer struct {
}

func (db *DBIndexer) Init() (bool, error) {
	return false, nil
}

func (db *DBIndexer) Index(issue []*IndexerData) error {
	return nil
}

func (db *DBIndexer) Delete(ids ...int64) error {
	return nil
}

func (db *DBIndexer) Search(kw string, repoID int64, limit, start int) (*SearchResult, error) {
	total, ids, err := models.SearchIssueIDsByKeyword(kw, repoID, limit, start)
	if err != nil {
		return nil, err
	}
	var result = SearchResult{
		Total: total,
		Hits:  make([]Match, 0, limit),
	}
	for _, id := range ids {
		result.Hits = append(result.Hits, Match{
			ID:     id,
			RepoID: repoID,
		})
	}
	return &result, nil
}
