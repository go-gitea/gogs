// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issues

import "code.gitea.io/gitea/models"

// DBIndexer implements Indexer interface to use database's like search
type DBIndexer struct{}

// Init dummy function
func (db *DBIndexer) Init() (bool, error) {
	return false, nil
}

// Index dummy function
func (db *DBIndexer) Index(issue []*IndexerData) error {
	return nil
}

// Delete dummy function
func (db *DBIndexer) Delete(ids ...int64) error {
	return nil
}

// Close dummy function
func (db *DBIndexer) Close() {
}

// Search dummy function
func (db *DBIndexer) Search(kw string, repoIDs []int64, limit, start int) (*SearchResult, error) {
	total, ids, err := models.SearchIssueIDsByKeyword(kw, repoIDs, limit, start)
	if err != nil {
		return nil, err
	}
	result := SearchResult{
		Total: total,
		Hits:  make([]Match, 0, limit),
	}
	for _, id := range ids {
		result.Hits = append(result.Hits, Match{
			ID: id,
		})
	}
	return &result, nil
}
