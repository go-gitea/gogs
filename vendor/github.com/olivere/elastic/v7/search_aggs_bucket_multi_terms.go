// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

// MultiTermsAggregation is a multi-bucket value source based aggregation
// where buckets are dynamically built - one per unique set of values.
// The multi terms aggregation is very similar to the terms aggregation,
// however in most cases it will be slower than the terms aggregation and will
// consume more memory. Therefore, if the same set of fields is constantly
// used, it would be more efficient to index a combined key for this fields
// as a separate field and use the terms aggregation on this field.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.13/search-aggregations-bucket-multi-terms-aggregation.html
type MultiTermsAggregation struct {
	multiTerms      []MultiTerm
	subAggregations map[string]Aggregation
	meta            map[string]interface{}

	size                  *int
	shardSize             *int
	minDocCount           *int
	shardMinDocCount      *int
	collectionMode        string
	showTermDocCountError *bool
	order                 []MultiTermsOrder
}

// NewMultiTermsAggregation initializes a new MultiTermsAggregation.
func NewMultiTermsAggregation() *MultiTermsAggregation {
	return &MultiTermsAggregation{
		subAggregations: make(map[string]Aggregation),
	}
}

// Terms adds a slice of field names to return in the aggregation.
//
// Notice that it appends to existing terms, so you can use Terms more than
// once, and mix with MultiTerms method.
func (a *MultiTermsAggregation) Terms(fields ...string) *MultiTermsAggregation {
	for _, field := range fields {
		a.multiTerms = append(a.multiTerms, MultiTerm{Field: field})
	}
	return a
}

// MultiTerms adds a slice of MultiTerm instances to return in the aggregation.
//
// Notice that it appends to existing terms, so you can use MultiTerms more
// than once, and mix with Terms method.
func (a *MultiTermsAggregation) MultiTerms(multiTerms ...MultiTerm) *MultiTermsAggregation {
	a.multiTerms = append(a.multiTerms, multiTerms...)
	return a
}

func (a *MultiTermsAggregation) SubAggregation(name string, subAggregation Aggregation) *MultiTermsAggregation {
	a.subAggregations[name] = subAggregation
	return a
}

// Meta sets the meta data to be included in the aggregation response.
func (a *MultiTermsAggregation) Meta(metaData map[string]interface{}) *MultiTermsAggregation {
	a.meta = metaData
	return a
}

func (a *MultiTermsAggregation) Size(size int) *MultiTermsAggregation {
	a.size = &size
	return a
}

func (a *MultiTermsAggregation) ShardSize(shardSize int) *MultiTermsAggregation {
	a.shardSize = &shardSize
	return a
}

func (a *MultiTermsAggregation) MinDocCount(minDocCount int) *MultiTermsAggregation {
	a.minDocCount = &minDocCount
	return a
}

func (a *MultiTermsAggregation) ShardMinDocCount(shardMinDocCount int) *MultiTermsAggregation {
	a.shardMinDocCount = &shardMinDocCount
	return a
}

func (a *MultiTermsAggregation) Order(order string, asc bool) *MultiTermsAggregation {
	a.order = append(a.order, MultiTermsOrder{Field: order, Ascending: asc})
	return a
}

func (a *MultiTermsAggregation) OrderByCount(asc bool) *MultiTermsAggregation {
	// "order" : { "_count" : "asc" }
	a.order = append(a.order, MultiTermsOrder{Field: "_count", Ascending: asc})
	return a
}

func (a *MultiTermsAggregation) OrderByCountAsc() *MultiTermsAggregation {
	return a.OrderByCount(true)
}

func (a *MultiTermsAggregation) OrderByCountDesc() *MultiTermsAggregation {
	return a.OrderByCount(false)
}

func (a *MultiTermsAggregation) OrderByKey(asc bool) *MultiTermsAggregation {
	// "order" : { "_term" : "asc" }
	a.order = append(a.order, MultiTermsOrder{Field: "_key", Ascending: asc})
	return a
}

func (a *MultiTermsAggregation) OrderByKeyAsc() *MultiTermsAggregation {
	return a.OrderByKey(true)
}

func (a *MultiTermsAggregation) OrderByKeyDesc() *MultiTermsAggregation {
	return a.OrderByKey(false)
}

// OrderByAggregation creates a bucket ordering strategy which sorts buckets
// based on a single-valued calc get.
func (a *MultiTermsAggregation) OrderByAggregation(aggName string, asc bool) *MultiTermsAggregation {
	// {
	// 	"aggs": {
	// 	  "genres_and_products": {
	// 		"multi_terms": {
	// 		  "terms": [
	// 			{
	// 			  "field": "genre"
	// 			},
	// 			{
	// 			  "field": "product"
	// 			}
	// 		  ],
	// 		  "order": {
	// 			"total_quantity": "desc"
	// 		  }
	// 		},
	// 		"aggs": {
	// 		  "total_quantity": {
	// 			"sum": {
	// 			  "field": "quantity"
	// 			}
	// 		  }
	// 		}
	// 	  }
	// 	}
	// }
	a.order = append(a.order, MultiTermsOrder{Field: aggName, Ascending: asc})
	return a
}

// OrderByAggregationAndMetric creates a bucket ordering strategy which
// sorts buckets based on a multi-valued calc get.
func (a *MultiTermsAggregation) OrderByAggregationAndMetric(aggName, metric string, asc bool) *MultiTermsAggregation {
	// {
	// 	"aggs": {
	// 	  "genres_and_products": {
	// 		"multi_terms": {
	// 		  "terms": [
	// 			{
	// 			  "field": "genre"
	// 			},
	// 			{
	// 			  "field": "product"
	// 			}
	// 		  ],
	// 		  "order": {
	// 			"total_quantity": "desc"
	// 		  }
	// 		},
	// 		"aggs": {
	// 		  "total_quantity": {
	// 			"sum": {
	// 			  "field": "quantity"
	// 			}
	// 		  }
	// 		}
	// 	  }
	// 	}
	// }
	a.order = append(a.order, MultiTermsOrder{Field: aggName + "." + metric, Ascending: asc})
	return a
}

// Collection mode can be depth_first or breadth_first as of 1.4.0.
func (a *MultiTermsAggregation) CollectionMode(collectionMode string) *MultiTermsAggregation {
	a.collectionMode = collectionMode
	return a
}

func (a *MultiTermsAggregation) ShowTermDocCountError(showTermDocCountError bool) *MultiTermsAggregation {
	a.showTermDocCountError = &showTermDocCountError
	return a
}

func (a *MultiTermsAggregation) Source() (interface{}, error) {
	// Example:
	// {
	// 	"aggs": {
	// 	  "genres_and_products": {
	// 		"multi_terms": {
	// 		  "terms": [
	// 			{
	// 			  "field": "genre"
	// 			},
	// 			{
	// 			  "field": "product"
	// 			}
	// 		  ]
	// 		}
	// 	  }
	// 	}
	// }
	// This method returns only the "multi_terms": { "terms": [ { "field": "genre" }, { "field": "product" } ] } part.

	source := make(map[string]interface{})
	opts := make(map[string]interface{})
	source["multi_terms"] = opts

	// ValuesSourceAggregationBuilder
	terms := make([]interface{}, len(a.multiTerms))
	for i := range a.multiTerms {
		s, err := a.multiTerms[i].Source()
		if err != nil {
			return nil, err
		}
		terms[i] = s
	}
	opts["terms"] = terms

	// TermsBuilder
	if a.size != nil && *a.size >= 0 {
		opts["size"] = *a.size
	}
	if a.shardSize != nil && *a.shardSize >= 0 {
		opts["shard_size"] = *a.shardSize
	}
	if a.minDocCount != nil && *a.minDocCount >= 0 {
		opts["min_doc_count"] = *a.minDocCount
	}
	if a.shardMinDocCount != nil && *a.shardMinDocCount >= 0 {
		opts["shard_min_doc_count"] = *a.shardMinDocCount
	}
	if a.showTermDocCountError != nil {
		opts["show_term_doc_count_error"] = *a.showTermDocCountError
	}
	if a.collectionMode != "" {
		opts["collect_mode"] = a.collectionMode
	}
	if len(a.order) > 0 {
		var orderSlice []interface{}
		for _, order := range a.order {
			src, err := order.Source()
			if err != nil {
				return nil, err
			}
			orderSlice = append(orderSlice, src)
		}
		opts["order"] = orderSlice
	}

	// AggregationBuilder (SubAggregations)
	if len(a.subAggregations) > 0 {
		aggsMap := make(map[string]interface{})
		source["aggregations"] = aggsMap
		for name, aggregate := range a.subAggregations {
			src, err := aggregate.Source()
			if err != nil {
				return nil, err
			}
			aggsMap[name] = src
		}
	}

	// Add Meta data if available
	if len(a.meta) > 0 {
		source["meta"] = a.meta
	}

	return source, nil
}

// MultiTermsOrder specifies a single order field for a multi terms aggregation.
type MultiTermsOrder struct {
	Field     string
	Ascending bool
}

// Source returns serializable JSON of the MultiTermsOrder.
func (order *MultiTermsOrder) Source() (interface{}, error) {
	source := make(map[string]string)
	if order.Ascending {
		source[order.Field] = "asc"
	} else {
		source[order.Field] = "desc"
	}
	return source, nil
}

// MultiTerm specifies a single term field for a multi terms aggregation.
type MultiTerm struct {
	Field   string
	Missing interface{}
}

// Source returns serializable JSON of the MultiTerm.
func (term *MultiTerm) Source() (interface{}, error) {
	source := make(map[string]interface{})
	source["field"] = term.Field
	if term.Missing != nil {
		source["missing"] = term.Missing
	}
	return source, nil
}
