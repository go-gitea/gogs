# Elastic

**This is a development branch that is actively being worked on. DO NOT USE IN PRODUCTION! If you want to use stable versions of Elastic, please use Go modules for the 7.x release (or later) or a dependency manager like [dep](https://github.com/golang/dep) for earlier releases.**

Elastic is an [Elasticsearch](http://www.elasticsearch.org/) client for the
[Go](http://www.golang.org/) programming language.

[![Build Status](https://github.com/olivere/elastic/workflows/Test/badge.svg)](https://github.com/olivere/elastic/actions)
[![Godoc](http://img.shields.io/badge/godoc-reference-blue.svg?style=flat)](https://pkg.go.dev/github.com/olivere/elastic/v7?tab=doc)
[![license](http://img.shields.io/badge/license-MIT-red.svg?style=flat)](https://raw.githubusercontent.com/olivere/elastic/master/LICENSE)

See the [wiki](https://github.com/olivere/elastic/wiki) for additional information about Elastic.

<a href="https://www.buymeacoffee.com/Bjd96U8fm" target="_blank"><img src="https://www.buymeacoffee.com/assets/img/custom_images/orange_img.png" alt="Buy Me A Coffee" style="height: 41px !important;width: 174px !important;box-shadow: 0px 3px 2px 0px rgba(190, 190, 190, 0.5) !important;-webkit-box-shadow: 0px 3px 2px 0px rgba(190, 190, 190, 0.5) !important;" ></a>

## Releases

**The release branches (e.g. [`release-branch.v7`](https://github.com/olivere/elastic/tree/release-branch.v7))
are actively being worked on and can break at any time.
If you want to use stable versions of Elastic, please use Go modules.**

Here's the version matrix:

Elasticsearch version | Elastic version  | Package URL | Remarks |
----------------------|------------------|-------------|---------|
7.x                   | 7.0              | [`github.com/olivere/elastic/v7`](https://github.com/olivere/elastic) ([source](https://github.com/olivere/elastic/tree/release-branch.v7) [doc](http://godoc.org/github.com/olivere/elastic)) | Use Go modules.
6.x                   | 6.0              | [`github.com/olivere/elastic`](https://github.com/olivere/elastic) ([source](https://github.com/olivere/elastic/tree/release-branch.v6) [doc](http://godoc.org/github.com/olivere/elastic)) | Use a dependency manager (see below).
5.x                   | 5.0              | [`gopkg.in/olivere/elastic.v5`](https://gopkg.in/olivere/elastic.v5) ([source](https://github.com/olivere/elastic/tree/release-branch.v5) [doc](http://godoc.org/gopkg.in/olivere/elastic.v5)) | Actively maintained.
2.x                   | 3.0              | [`gopkg.in/olivere/elastic.v3`](https://gopkg.in/olivere/elastic.v3) ([source](https://github.com/olivere/elastic/tree/release-branch.v3) [doc](http://godoc.org/gopkg.in/olivere/elastic.v3)) | Deprecated. Please update.
1.x                   | 2.0              | [`gopkg.in/olivere/elastic.v2`](https://gopkg.in/olivere/elastic.v2) ([source](https://github.com/olivere/elastic/tree/release-branch.v2) [doc](http://godoc.org/gopkg.in/olivere/elastic.v2)) | Deprecated. Please update.
0.9-1.3               | 1.0              | [`gopkg.in/olivere/elastic.v1`](https://gopkg.in/olivere/elastic.v1) ([source](https://github.com/olivere/elastic/tree/release-branch.v1) [doc](http://godoc.org/gopkg.in/olivere/elastic.v1)) | Deprecated. Please update.

**Example:**

You have installed Elasticsearch 7.0.0 and want to use Elastic.
As listed above, you should use Elastic 7.0 (code is in `release-branch.v7`).

To use the required version of Elastic in your application, you
should use [Go modules](https://github.com/golang/go/wiki/Modules)
to manage dependencies. Make sure to use a version such as `7.0.0` or later.

To use Elastic, import:

```go
import "github.com/olivere/elastic/v7"
```

### Elastic 7.0

Elastic 7.0 targets Elasticsearch 7.x which [was released on April 10th 2019](https://www.elastic.co/guide/en/elasticsearch/reference/7.0/release-notes-7.0.0.html).

As always with major version, there are a lot of [breaking changes](https://www.elastic.co/guide/en/elasticsearch/reference/7.0/release-notes-7.0.0.html#breaking-7.0.0).
We will use this as an opportunity to [clean up and refactor Elastic](https://github.com/olivere/elastic/blob/release-branch.v7/CHANGELOG-7.0.md),
as we already did in earlier (major) releases.

### Elastic 6.0

Elastic 6.0 targets Elasticsearch 6.x which was [released on 14th November 2017](https://www.elastic.co/blog/elasticsearch-6-0-0-released).

Notice that there are a lot of [breaking changes in Elasticsearch 6.0](https://www.elastic.co/guide/en/elasticsearch/reference/6.7/breaking-changes-6.0.html)
and we used this as an opportunity to [clean up and refactor Elastic](https://github.com/olivere/elastic/blob/release-branch.v6/CHANGELOG-6.0.md)
as we did in the transition from earlier versions of Elastic.

### Elastic 5.0

Elastic 5.0 targets Elasticsearch 5.0.0 and later. Elasticsearch 5.0.0 was
[released on 26th October 2016](https://www.elastic.co/blog/elasticsearch-5-0-0-released).

Notice that there are will be a lot of [breaking changes in Elasticsearch 5.0](https://www.elastic.co/guide/en/elasticsearch/reference/5.0/breaking-changes-5.0.html)
and we used this as an opportunity to [clean up and refactor Elastic](https://github.com/olivere/elastic/blob/release-branch.v5/CHANGELOG-5.0.md)
as we did in the transition from Elastic 2.0 (for Elasticsearch 1.x) to Elastic 3.0 (for Elasticsearch 2.x).

Furthermore, the jump in version numbers will give us a chance to be in sync with the Elastic Stack.

### Elastic 3.0

Elastic 3.0 targets Elasticsearch 2.x and is published via [`gopkg.in/olivere/elastic.v3`](https://gopkg.in/olivere/elastic.v3).

Elastic 3.0 will only get critical bug fixes. You should update to a recent version.

### Elastic 2.0

Elastic 2.0 targets Elasticsearch 1.x and is published via [`gopkg.in/olivere/elastic.v2`](https://gopkg.in/olivere/elastic.v2).

Elastic 2.0 will only get critical bug fixes. You should update to a recent version.

### Elastic 1.0

Elastic 1.0 is deprecated. You should really update Elasticsearch and Elastic
to a recent version.

However, if you cannot update for some reason, don't worry. Version 1.0 is
still available. All you need to do is go-get it and change your import path
as described above.


## Status

We use Elastic in production since 2012. Elastic is stable but the API changes
now and then. We strive for API compatibility.
However, Elasticsearch sometimes introduces [breaking changes](https://www.elastic.co/guide/en/elasticsearch/reference/master/breaking-changes.html)
and we sometimes have to adapt.

Having said that, there have been no big API changes that required you
to rewrite your application big time. More often than not it's renaming APIs
and adding/removing features so that Elastic is in sync with Elasticsearch.

Elastic has been used in production starting with Elasticsearch 0.90 up to recent 7.x
versions.
We recently switched to [GitHub Actions for testing](https://github.com/olivere/elastic/actions).
Before that, we used [Travis CI](https://travis-ci.org/olivere/elastic) successfully for years).

Elasticsearch has quite a few features. Most of them are implemented
by Elastic. I add features and APIs as required. It's straightforward
to implement missing pieces. I'm accepting pull requests :-)

Having said that, I hope you find the project useful.


## Getting Started

The first thing you do is to create a [Client](https://github.com/olivere/elastic/blob/master/client.go).
The client connects to Elasticsearch on `http://127.0.0.1:9200` by default.

You typically create one client for your app. Here's a complete example of
creating a client, creating an index, adding a document, executing a search etc.

An example is available [here](https://olivere.github.io/elastic/).

Here's a [link to a complete working example for v6](https://gist.github.com/olivere/e4a376b4783c0914e44ea4f745ce2ebf).

Here are a few tips on how to get used to Elastic:

1. Head over to the [Wiki](https://github.com/olivere/elastic/wiki) for detailed information and
   topics like e.g. [how to add a middleware](https://github.com/olivere/elastic/wiki/HttpTransport)
   or how to [connect to AWS](https://github.com/olivere/elastic/wiki/Using-with-AWS-Elasticsearch-Service).
2. If you are unsure how to implement something, read the tests (all `_test.go` files).
   They not only serve as a guard against changes, but also as a reference.
3. The [recipes](https://github.com/olivere/elastic/tree/release-branch.v6/recipes)
   contains small examples on how to implement something, e.g. bulk indexing, scrolling etc.


## API Status

### Document APIs

- [x] Index API
- [x] Get API
- [x] Delete API
- [x] Delete By Query API
- [x] Update API
- [x] Update By Query API
- [x] Multi Get API
- [x] Bulk API
- [x] Reindex API
- [x] Term Vectors
- [x] Multi termvectors API

### Search APIs

- [x] Search
- [x] Search Template
- [ ] Multi Search Template
- [x] Search Shards API
- [x] Suggesters
  - [x] Term Suggester
  - [x] Phrase Suggester
  - [x] Completion Suggester
  - [x] Context Suggester
- [x] Multi Search API
- [x] Count API
- [x] Validate API
- [x] Explain API
- [x] Profile API
- [x] Field Capabilities API

### Aggregations

- Metrics Aggregations
  - [x] Avg
  - [ ] Boxplot (X-pack)
  - [x] Cardinality
  - [x] Extended Stats
  - [x] Geo Bounds
  - [x] Geo Centroid
  - [x] Matrix stats
  - [x] Max
  - [x] Median absolute deviation
  - [x] Min
  - [x] Percentile Ranks
  - [x] Percentiles
  - [ ] Rate (X-pack)
  - [ ] Scripted Metric
  - [x] Stats
  - [ ] String stats (X-pack)
  - [x] Sum
  - [ ] T-test (X-pack)
  - [x] Top Hits
  - [x] Top metrics (X-pack)
  - [x] Value Count
  - [x] Weighted avg
- Bucket Aggregations
  - [x] Adjacency Matrix
  - [x] Auto-interval Date Histogram
  - [x] Children
  - [x] Composite
  - [x] Date Histogram
  - [x] Date Range
  - [x] Diversified Sampler
  - [x] Filter
  - [x] Filters
  - [x] Geo Distance
  - [x] Geohash Grid
  - [x] Geotile grid
  - [x] Global
  - [x] Histogram
  - [x] IP Range
  - [x] Missing
  - [x] Nested
  - [ ] Parent
  - [x] Range
  - [ ] Rare terms
  - [x] Reverse Nested
  - [x] Sampler
  - [x] Significant Terms
  - [x] Significant Text
  - [x] Terms
  - [ ] Variable width histogram
- Pipeline Aggregations
  - [x] Avg Bucket
  - [x] Bucket Script
  - [x] Bucket Selector
  - [x] Bucket Sort
  - [ ] Cumulative cardinality (X-pack)
  - [x] Cumulative Sum
  - [x] Derivative
  - [ ] Extended Stats Bucket
  - [ ] Inference bucket (X-pack)
  - [x] Max Bucket
  - [x] Min Bucket
  - [x] Moving Average
  - [x] Moving function
  - [ ] Moving percentiles (X-pack)
  - [ ] Normalize (X-pack)
  - [x] Percentiles Bucket
  - [x] Serial Differencing
  - [x] Stats Bucket
  - [x] Sum Bucket
- [x] Aggregation Metadata

### Indices APIs

- [x] Create Index
- [x] Delete Index
- [x] Get Index
- [x] Indices Exists
- [x] Open / Close Index
- [x] Shrink Index
- [x] Rollover Index
- [x] Put Mapping
- [x] Get Mapping
- [x] Get Field Mapping
- [x] Types Exists
- [x] Index Aliases
- [x] Update Indices Settings
- [x] Get Settings
- [x] Analyze
  - [x] Explain Analyze
- [x] Index Templates
- [x] Indices Stats
- [x] Indices Segments
- [ ] Indices Recovery
- [ ] Indices Shard Stores
- [x] Clear Cache
- [x] Flush
  - [x] Synced Flush
- [x] Refresh
- [x] Force Merge

### Index Lifecycle Management APIs

- [x] Create Policy
- [x] Get Policy
- [x] Delete Policy
- [ ] Move to Step
- [ ] Remove Policy
- [ ] Retry Policy
- [ ] Get Ilm Status
- [ ] Explain Lifecycle
- [ ] Start Ilm
- [ ] Stop Ilm

### cat APIs

- [X] cat aliases
- [X] cat allocation
- [X] cat count
- [ ] cat fielddata
- [X] cat health
- [X] cat indices
- [ ] cat master
- [ ] cat nodeattrs
- [ ] cat nodes
- [ ] cat pending tasks
- [ ] cat plugins
- [ ] cat recovery
- [ ] cat repositories
- [ ] cat thread pool
- [ ] cat shards
- [ ] cat segments
- [ ] cat snapshots
- [ ] cat templates

### Cluster APIs

- [x] Cluster Health
- [x] Cluster State
- [x] Cluster Stats
- [ ] Pending Cluster Tasks
- [x] Cluster Reroute
- [ ] Cluster Update Settings
- [x] Nodes Stats
- [x] Nodes Info
- [ ] Nodes Feature Usage
- [ ] Remote Cluster Info
- [x] Task Management API
- [ ] Nodes hot_threads
- [ ] Cluster Allocation Explain API

### Query DSL

- [x] Match All Query
- [x] Inner hits
- Full text queries
  - [x] Match Query
  - [x] Match Boolean Prefix Query
  - [x] Match Phrase Query
  - [x] Match Phrase Prefix Query
  - [x] Multi Match Query
  - [x] Common Terms Query
  - [x] Query String Query
  - [x] Simple Query String Query
- Term level queries
  - [x] Term Query
  - [x] Terms Query
  - [x] Terms Set Query
  - [x] Range Query
  - [x] Exists Query
  - [x] Prefix Query
  - [x] Wildcard Query
  - [x] Regexp Query
  - [x] Fuzzy Query
  - [x] Type Query
  - [x] Ids Query
- Compound queries
  - [x] Constant Score Query
  - [x] Bool Query
  - [x] Dis Max Query
  - [x] Function Score Query
  - [x] Boosting Query
- Joining queries
  - [x] Nested Query
  - [x] Has Child Query
  - [x] Has Parent Query
  - [x] Parent Id Query
- Geo queries
  - [ ] GeoShape Query
  - [x] Geo Bounding Box Query
  - [x] Geo Distance Query
  - [x] Geo Polygon Query
- Specialized queries
  - [x] Distance Feature Query
  - [x] More Like This Query
  - [x] Script Query
  - [x] Script Score Query
  - [x] Percolate Query
- Span queries
  - [x] Span Term Query
  - [ ] Span Multi Term Query
  - [x] Span First Query
  - [x] Span Near Query
  - [ ] Span Or Query
  - [ ] Span Not Query
  - [ ] Span Containing Query
  - [ ] Span Within Query
  - [ ] Span Field Masking Query
- [ ] Minimum Should Match
- [ ] Multi Term Query Rewrite

### Modules

- Snapshot and Restore
  - [x] Repositories
  - [x] Snapshot get
  - [x] Snapshot create
  - [x] Snapshot delete
  - [ ] Restore
  - [ ] Snapshot status
  - [ ] Monitoring snapshot/restore status
  - [ ] Stopping currently running snapshot and restore
- Scripting
  - [x] GetScript
  - [x] PutScript
  - [x] DeleteScript

### Sorting

- [x] Sort by score
- [x] Sort by field
- [x] Sort by geo distance
- [x] Sort by script
- [x] Sort by doc

### Scrolling

Scrolling is supported via a  `ScrollService`. It supports an iterator-like interface.
The `ClearScroll` API is implemented as well.

A pattern for [efficiently scrolling in parallel](https://github.com/olivere/elastic/wiki/ScrollParallel)
is described in the [Wiki](https://github.com/olivere/elastic/wiki).

## How to contribute

Read [the contribution guidelines](https://github.com/olivere/elastic/blob/master/CONTRIBUTING.md).

## Credits

Thanks a lot for the great folks working hard on
[Elasticsearch](https://www.elastic.co/products/elasticsearch)
and
[Go](https://golang.org/).

Elastic uses portions of the
[uritemplates](https://github.com/jtacoma/uritemplates) library
by Joshua Tacoma,
[backoff](https://github.com/cenkalti/backoff) by Cenk Altı and
[leaktest](https://github.com/fortytw2/leaktest) by Ian Chiles.

## LICENSE

MIT-LICENSE. See [LICENSE](http://olivere.mit-license.org/)
or the LICENSE file provided in the repository for details.
