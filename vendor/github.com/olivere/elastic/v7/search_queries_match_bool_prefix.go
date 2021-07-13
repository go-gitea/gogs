// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

// MatchBoolPrefixQuery query analyzes its input and constructs a bool query from the terms.
// Each term except the last is used in a term query. The last term is used in a prefix query.
//
// For more details, see
// https://www.elastic.co/guide/en/elasticsearch/reference/7.0/query-dsl-match-bool-prefix-query.html
type MatchBoolPrefixQuery struct {
	name                string
	queryText           interface{}
	analyzer            string
	minimumShouldMatch  string
	operator            string
	fuzziness           string
	prefixLength        *int
	maxExpansions       *int
	fuzzyTranspositions *bool
	fuzzyRewrite        string
	boost               *float64
}

// NewMatchBoolPrefixQuery creates and initializes a new MatchBoolPrefixQuery.
func NewMatchBoolPrefixQuery(name string, queryText interface{}) *MatchBoolPrefixQuery {
	return &MatchBoolPrefixQuery{name: name, queryText: queryText}
}

// Analyzer explicitly sets the analyzer to use. It defaults to use explicit
// mapping config for the field, or, if not set, the default search analyzer.
func (q *MatchBoolPrefixQuery) Analyzer(analyzer string) *MatchBoolPrefixQuery {
	q.analyzer = analyzer
	return q
}

// MinimumShouldMatch sets the optional minimumShouldMatch value to apply to the query.
func (q *MatchBoolPrefixQuery) MinimumShouldMatch(minimumShouldMatch string) *MatchBoolPrefixQuery {
	q.minimumShouldMatch = minimumShouldMatch
	return q
}

// Operator sets the operator to use when using a boolean query.
// Can be "AND" or "OR" (default).
func (q *MatchBoolPrefixQuery) Operator(operator string) *MatchBoolPrefixQuery {
	q.operator = operator
	return q
}

// Fuzziness sets the edit distance for fuzzy queries. Default is "AUTO".
func (q *MatchBoolPrefixQuery) Fuzziness(fuzziness string) *MatchBoolPrefixQuery {
	q.fuzziness = fuzziness
	return q
}

// PrefixLength is the number of beginning characters left unchanged for fuzzy matching. Defaults to 0.
func (q *MatchBoolPrefixQuery) PrefixLength(prefixLength int) *MatchBoolPrefixQuery {
	q.prefixLength = &prefixLength
	return q
}

// MaxExpansions sets the number of term expansions to use.
func (q *MatchBoolPrefixQuery) MaxExpansions(n int) *MatchBoolPrefixQuery {
	q.maxExpansions = &n
	return q
}

// FuzzyTranspositions if true, edits for fuzzy matching include transpositions of two adjacent
// characters (ab → ba). Defaults to true.
func (q *MatchBoolPrefixQuery) FuzzyTranspositions(fuzzyTranspositions bool) *MatchBoolPrefixQuery {
	q.fuzzyTranspositions = &fuzzyTranspositions
	return q
}

// FuzzyRewrite sets the fuzzy_rewrite parameter controlling how the
// fuzzy query will get rewritten.
func (q *MatchBoolPrefixQuery) FuzzyRewrite(fuzzyRewrite string) *MatchBoolPrefixQuery {
	q.fuzzyRewrite = fuzzyRewrite
	return q
}

// Boost sets the boost to apply to this query.
func (q *MatchBoolPrefixQuery) Boost(boost float64) *MatchBoolPrefixQuery {
	q.boost = &boost
	return q
}

// Source returns JSON for the function score query.
func (q *MatchBoolPrefixQuery) Source() (interface{}, error) {
	source := make(map[string]interface{})

	match := make(map[string]interface{})
	source["match_bool_prefix"] = match

	query := make(map[string]interface{})
	match[q.name] = query

	query["query"] = q.queryText

	if q.analyzer != "" {
		query["analyzer"] = q.analyzer
	}
	if q.minimumShouldMatch != "" {
		query["minimum_should_match"] = q.minimumShouldMatch
	}
	if q.operator != "" {
		query["operator"] = q.operator
	}
	if q.fuzziness != "" {
		query["fuzziness"] = q.fuzziness
	}
	if q.prefixLength != nil {
		query["prefix_length"] = *q.prefixLength
	}
	if q.maxExpansions != nil {
		query["max_expansions"] = *q.maxExpansions
	}
	if q.fuzzyTranspositions != nil {
		query["fuzzy_transpositions"] = *q.fuzzyTranspositions
	}
	if q.fuzzyRewrite != "" {
		query["fuzzy_rewrite"] = q.fuzzyRewrite
	}
	if q.boost != nil {
		query["boost"] = *q.boost
	}

	return source, nil
}
