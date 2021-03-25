// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issues

import (
	"fmt"
	"os"
	"strconv"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/custom"
	"github.com/blevesearch/bleve/v2/analysis/token/lowercase"
	"github.com/blevesearch/bleve/v2/analysis/token/unicodenorm"
	"github.com/blevesearch/bleve/v2/analysis/tokenizer/unicode"
	"github.com/blevesearch/bleve/v2/index/upsidedown"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/ethantkoenig/rupture"
)

const (
	issueIndexerAnalyzer      = "issueIndexer"
	issueIndexerDocType       = "issueIndexerDocType"
	issueIndexerLatestVersion = 1
)

// indexerID a bleve-compatible unique identifier for an integer id
func indexerID(id int64) string {
	return strconv.FormatInt(id, 36)
}

// idOfIndexerID the integer id associated with an indexer id
func idOfIndexerID(indexerID string) (int64, error) {
	id, err := strconv.ParseInt(indexerID, 36, 64)
	if err != nil {
		return 0, fmt.Errorf("Unexpected indexer ID %s: %v", indexerID, err)
	}
	return id, nil
}

// numericEqualityQuery a numeric equality query for the given value and field
func numericEqualityQuery(value int64, field string) *query.NumericRangeQuery {
	f := float64(value)
	tru := true
	q := bleve.NewNumericRangeInclusiveQuery(&f, &f, &tru, &tru)
	q.SetField(field)
	return q
}

func newMatchPhraseQuery(matchPhrase, field, analyzer string) *query.MatchPhraseQuery {
	q := bleve.NewMatchPhraseQuery(matchPhrase)
	q.FieldVal = field
	q.Analyzer = analyzer
	return q
}

const unicodeNormalizeName = "unicodeNormalize"

func addUnicodeNormalizeTokenFilter(m *mapping.IndexMappingImpl) error {
	return m.AddCustomTokenFilter(unicodeNormalizeName, map[string]interface{}{
		"type": unicodenorm.Name,
		"form": unicodenorm.NFC,
	})
}

const maxBatchSize = 16

// openIndexer open the index at the specified path, checking for metadata
// updates and bleve version updates.  If index needs to be created (or
// re-created), returns (nil, nil)
func openIndexer(path string, latestVersion int) (bleve.Index, error) {
	_, err := os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	metadata, err := rupture.ReadIndexMetadata(path)
	if err != nil {
		return nil, err
	}
	if metadata.Version < latestVersion {
		// the indexer is using a previous version, so we should delete it and
		// re-populate
		return nil, util.RemoveAll(path)
	}

	index, err := bleve.Open(path)
	if err != nil && err == upsidedown.IncompatibleVersion {
		// the indexer was built with a previous version of bleve, so we should
		// delete it and re-populate
		return nil, util.RemoveAll(path)
	} else if err != nil {
		return nil, err
	}

	return index, nil
}

// BleveIndexerData an update to the issue indexer
type BleveIndexerData IndexerData

// Type returns the document type, for bleve's mapping.Classifier interface.
func (i *BleveIndexerData) Type() string {
	return issueIndexerDocType
}

// createIssueIndexer create an issue indexer if one does not already exist
func createIssueIndexer(path string, latestVersion int) (bleve.Index, error) {
	mapping := bleve.NewIndexMapping()
	docMapping := bleve.NewDocumentMapping()

	numericFieldMapping := bleve.NewNumericFieldMapping()
	numericFieldMapping.IncludeInAll = false
	docMapping.AddFieldMappingsAt("RepoID", numericFieldMapping)

	textFieldMapping := bleve.NewTextFieldMapping()
	textFieldMapping.Store = false
	textFieldMapping.IncludeInAll = false
	docMapping.AddFieldMappingsAt("Title", textFieldMapping)
	docMapping.AddFieldMappingsAt("Content", textFieldMapping)
	docMapping.AddFieldMappingsAt("Comments", textFieldMapping)

	if err := addUnicodeNormalizeTokenFilter(mapping); err != nil {
		return nil, err
	} else if err = mapping.AddCustomAnalyzer(issueIndexerAnalyzer, map[string]interface{}{
		"type":          custom.Name,
		"char_filters":  []string{},
		"tokenizer":     unicode.Name,
		"token_filters": []string{unicodeNormalizeName, lowercase.Name},
	}); err != nil {
		return nil, err
	}

	mapping.DefaultAnalyzer = issueIndexerAnalyzer
	mapping.AddDocumentMapping(issueIndexerDocType, docMapping)
	mapping.AddDocumentMapping("_all", bleve.NewDocumentDisabledMapping())

	index, err := bleve.New(path, mapping)
	if err != nil {
		return nil, err
	}

	if err = rupture.WriteIndexMetadata(path, &rupture.IndexMetadata{
		Version: latestVersion,
	}); err != nil {
		return nil, err
	}
	return index, nil
}

var _ Indexer = &BleveIndexer{}

// BleveIndexer implements Indexer interface
type BleveIndexer struct {
	indexDir string
	indexer  bleve.Index
}

// NewBleveIndexer creates a new bleve local indexer
func NewBleveIndexer(indexDir string) *BleveIndexer {
	return &BleveIndexer{
		indexDir: indexDir,
	}
}

// Init will initialize the indexer
func (b *BleveIndexer) Init() (bool, error) {
	var err error
	b.indexer, err = openIndexer(b.indexDir, issueIndexerLatestVersion)
	if err != nil {
		return false, err
	}
	if b.indexer != nil {
		return true, nil
	}

	b.indexer, err = createIssueIndexer(b.indexDir, issueIndexerLatestVersion)
	return false, err
}

// Close will close the bleve indexer
func (b *BleveIndexer) Close() {
	if b.indexer != nil {
		if err := b.indexer.Close(); err != nil {
			log.Error("Error whilst closing indexer: %v", err)
		}
	}
}

// Index will save the index data
func (b *BleveIndexer) Index(issues []*IndexerData) error {
	batch := rupture.NewFlushingBatch(b.indexer, maxBatchSize)
	for _, issue := range issues {
		if err := batch.Index(indexerID(issue.ID), struct {
			RepoID   int64
			Title    string
			Content  string
			Comments []string
		}{
			RepoID:   issue.RepoID,
			Title:    issue.Title,
			Content:  issue.Content,
			Comments: issue.Comments,
		}); err != nil {
			return err
		}
	}
	return batch.Flush()
}

// Delete deletes indexes by ids
func (b *BleveIndexer) Delete(ids ...int64) error {
	batch := rupture.NewFlushingBatch(b.indexer, maxBatchSize)
	for _, id := range ids {
		if err := batch.Delete(indexerID(id)); err != nil {
			return err
		}
	}
	return batch.Flush()
}

// Search searches for issues by given conditions.
// Returns the matching issue IDs
func (b *BleveIndexer) Search(keyword string, repoIDs []int64, limit, start int) (*SearchResult, error) {
	var repoQueriesP []*query.NumericRangeQuery
	for _, repoID := range repoIDs {
		repoQueriesP = append(repoQueriesP, numericEqualityQuery(repoID, "RepoID"))
	}
	repoQueries := make([]query.Query, len(repoQueriesP))
	for i, v := range repoQueriesP {
		repoQueries[i] = query.Query(v)
	}

	indexerQuery := bleve.NewConjunctionQuery(
		bleve.NewDisjunctionQuery(repoQueries...),
		bleve.NewDisjunctionQuery(
			newMatchPhraseQuery(keyword, "Title", issueIndexerAnalyzer),
			newMatchPhraseQuery(keyword, "Content", issueIndexerAnalyzer),
			newMatchPhraseQuery(keyword, "Comments", issueIndexerAnalyzer),
		))
	search := bleve.NewSearchRequestOptions(indexerQuery, limit, start, false)
	search.SortBy([]string{"-_score"})

	result, err := b.indexer.Search(search)
	if err != nil {
		return nil, err
	}

	ret := SearchResult{
		Hits: make([]Match, 0, len(result.Hits)),
	}
	for _, hit := range result.Hits {
		id, err := idOfIndexerID(hit.ID)
		if err != nil {
			return nil, err
		}
		ret.Hits = append(ret.Hits, Match{
			ID: id,
		})
	}
	return &ret, nil
}
