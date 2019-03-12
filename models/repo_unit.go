// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"encoding/json"

	"code.gitea.io/gitea/modules/util"

	"github.com/Unknwon/com"
	"github.com/go-xorm/core"
	"github.com/go-xorm/xorm"
)

// RepoUnit describes all units of a repository
type RepoUnit struct {
	ID            int64
	RepoID        int64    `xorm:"INDEX(s)"`
	Type          UnitType `xorm:"INDEX(s)"`
	AllowWriteAll bool
	Config        core.Conversion `xorm:"TEXT"`
	CreatedUnix   util.TimeStamp  `xorm:"INDEX CREATED"`
}

// UnitConfig describes common unit config
type UnitConfig struct {
}

// FromDB fills up a UnitConfig from serialized format.
func (cfg *UnitConfig) FromDB(bs []byte) error {
	return json.Unmarshal(bs, &cfg)
}

// ToDB exports a UnitConfig to a serialized format.
func (cfg *UnitConfig) ToDB() ([]byte, error) {
	return json.Marshal(cfg)
}

// ExternalWikiConfig describes external wiki config
type ExternalWikiConfig struct {
	ExternalWikiURL string
}

// FromDB fills up a ExternalWikiConfig from serialized format.
func (cfg *ExternalWikiConfig) FromDB(bs []byte) error {
	return json.Unmarshal(bs, &cfg)
}

// ToDB exports a ExternalWikiConfig to a serialized format.
func (cfg *ExternalWikiConfig) ToDB() ([]byte, error) {
	return json.Marshal(cfg)
}

// ExternalTrackerConfig describes external tracker config
type ExternalTrackerConfig struct {
	ExternalTrackerURL    string
	ExternalTrackerFormat string
	ExternalTrackerStyle  string
}

// FromDB fills up a ExternalTrackerConfig from serialized format.
func (cfg *ExternalTrackerConfig) FromDB(bs []byte) error {
	return json.Unmarshal(bs, &cfg)
}

// ToDB exports a ExternalTrackerConfig to a serialized format.
func (cfg *ExternalTrackerConfig) ToDB() ([]byte, error) {
	return json.Marshal(cfg)
}

// IssuesConfig describes issues config
type IssuesConfig struct {
	EnableTimetracker                bool
	AllowOnlyContributorsToTrackTime bool
	EnableDependencies               bool
}

// FromDB fills up a IssuesConfig from serialized format.
func (cfg *IssuesConfig) FromDB(bs []byte) error {
	return json.Unmarshal(bs, &cfg)
}

// ToDB exports a IssuesConfig to a serialized format.
func (cfg *IssuesConfig) ToDB() ([]byte, error) {
	return json.Marshal(cfg)
}

// PullRequestsConfig describes pull requests config
type PullRequestsConfig struct {
	IgnoreWhitespaceConflicts bool
	AllowMerge                bool
	AllowRebase               bool
	AllowRebaseMerge          bool
	AllowSquash               bool
}

// FromDB fills up a PullRequestsConfig from serialized format.
func (cfg *PullRequestsConfig) FromDB(bs []byte) error {
	return json.Unmarshal(bs, &cfg)
}

// ToDB exports a PullRequestsConfig to a serialized format.
func (cfg *PullRequestsConfig) ToDB() ([]byte, error) {
	return json.Marshal(cfg)
}

// IsMergeStyleAllowed returns if merge style is allowed
func (cfg *PullRequestsConfig) IsMergeStyleAllowed(mergeStyle MergeStyle) bool {
	return mergeStyle == MergeStyleMerge && cfg.AllowMerge ||
		mergeStyle == MergeStyleRebase && cfg.AllowRebase ||
		mergeStyle == MergeStyleRebaseMerge && cfg.AllowRebaseMerge ||
		mergeStyle == MergeStyleSquash && cfg.AllowSquash
}

// BeforeSet is invoked from XORM before setting the value of a field of this object.
func (r *RepoUnit) BeforeSet(colName string, val xorm.Cell) {
	switch colName {
	case "type":
		switch UnitType(Cell2Int64(val)) {
		case UnitTypeCode, UnitTypeReleases, UnitTypeWiki:
			r.Config = new(UnitConfig)
		case UnitTypeExternalWiki:
			r.Config = new(ExternalWikiConfig)
		case UnitTypeExternalTracker:
			r.Config = new(ExternalTrackerConfig)
		case UnitTypePullRequests:
			r.Config = new(PullRequestsConfig)
		case UnitTypeIssues:
			r.Config = new(IssuesConfig)
		default:
			panic("unrecognized repo unit type: " + com.ToStr(*val))
		}
	}
}

// Unit returns Unit
func (r *RepoUnit) Unit() Unit {
	return Units[r.Type]
}

// CodeConfig returns config for UnitTypeCode
func (r *RepoUnit) CodeConfig() *UnitConfig {
	return r.Config.(*UnitConfig)
}

// PullRequestsConfig returns config for UnitTypePullRequests
func (r *RepoUnit) PullRequestsConfig() *PullRequestsConfig {
	return r.Config.(*PullRequestsConfig)
}

// ReleasesConfig returns config for UnitTypeReleases
func (r *RepoUnit) ReleasesConfig() *UnitConfig {
	return r.Config.(*UnitConfig)
}

// ExternalWikiConfig returns config for UnitTypeExternalWiki
func (r *RepoUnit) ExternalWikiConfig() *ExternalWikiConfig {
	return r.Config.(*ExternalWikiConfig)
}

// IssuesConfig returns config for UnitTypeIssues
func (r *RepoUnit) IssuesConfig() *IssuesConfig {
	return r.Config.(*IssuesConfig)
}

// ExternalTrackerConfig returns config for UnitTypeExternalTracker
func (r *RepoUnit) ExternalTrackerConfig() *ExternalTrackerConfig {
	return r.Config.(*ExternalTrackerConfig)
}

func getUnitsByRepoID(e Engine, repoID int64) (units []*RepoUnit, err error) {
	return units, e.Where("repo_id = ?", repoID).Find(&units)
}
