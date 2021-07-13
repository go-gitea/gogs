// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

import "errors"

// TopMetricsAggregation selects metrics from the document with the largest or smallest "sort" value.
// top_metrics is fairly similar to top_hits in spirit but because it is more limited it is able to do
// its job using less memory and is often faster.
//
// See: https://www.elastic.co/guide/en/elasticsearch/reference/7.0/search-aggregations-metrics-top-metrics.html
type TopMetricsAggregation struct {
	fields []string
	sorter Sorter
	size   int
}

func NewTopMetricsAggregation() *TopMetricsAggregation {
	return &TopMetricsAggregation{}
}

// Field adds a field to run aggregation against.
func (a *TopMetricsAggregation) Field(field string) *TopMetricsAggregation {
	a.fields = append(a.fields, field)
	return a
}

// Sort adds a sort order.
func (a *TopMetricsAggregation) Sort(field string, ascending bool) *TopMetricsAggregation {
	a.sorter = SortInfo{Field: field, Ascending: ascending}
	return a
}

// SortWithInfo adds a sort order.
func (a *TopMetricsAggregation) SortWithInfo(info SortInfo) *TopMetricsAggregation {
	a.sorter = info
	return a
}

// SortBy adds a sort order.
func (a *TopMetricsAggregation) SortBy(sorter Sorter) *TopMetricsAggregation {
	a.sorter = sorter
	return a
}

// Size sets the number of top documents returned by the aggregation. The default size is 1.
func (a *TopMetricsAggregation) Size(size int) *TopMetricsAggregation {
	a.size = size
	return a
}

func (a *TopMetricsAggregation) Source() (interface{}, error) {
	params := make(map[string]interface{})

	if len(a.fields) == 0 {
		return nil, errors.New("field list is required for the top metrics aggregation")
	}
	metrics := make([]interface{}, len(a.fields))
	for idx, field := range a.fields {
		metrics[idx] = map[string]string{"field": field}
	}
	params["metrics"] = metrics

	if a.sorter == nil {
		return nil, errors.New("sorter is required for the top metrics aggregation")
	}
	sortSource, err := a.sorter.Source()
	if err != nil {
		return nil, err
	}
	params["sort"] = sortSource

	if a.size > 1 {
		params["size"] = a.size
	}

	source := map[string]interface{}{
		"top_metrics": params,
	}
	return source, nil
}
