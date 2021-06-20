// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// Release represents a release of repository.
type Release struct {
	ID               int64       `xorm:"pk autoincr"`
	RepoID           int64       `xorm:"INDEX UNIQUE(n)"`
	Repo             *Repository `xorm:"-"`
	PublisherID      int64       `xorm:"INDEX"`
	Publisher        *User       `xorm:"-"`
	TagName          string      `xorm:"INDEX UNIQUE(n)"`
	OriginalAuthor   string
	OriginalAuthorID int64 `xorm:"index"`
	LowerTagName     string
	Target           string
	Title            string
	Sha1             string `xorm:"VARCHAR(40)"`
	NumCommits       int64
	NumCommitsBehind int64              `xorm:"-"`
	Note             string             `xorm:"TEXT"`
	RenderedNote     string             `xorm:"-"`
	IsDraft          bool               `xorm:"NOT NULL DEFAULT false"`
	IsPrerelease     bool               `xorm:"NOT NULL DEFAULT false"`
	IsTag            bool               `xorm:"NOT NULL DEFAULT false"`
	Attachments      []*Attachment      `xorm:"-"`
	CreatedUnix      timeutil.TimeStamp `xorm:"INDEX"`
}

func (r *Release) loadAttributes(e Engine) error {
	var err error
	if r.Repo == nil {
		r.Repo, err = GetRepositoryByID(r.RepoID)
		if err != nil {
			return err
		}
	}
	if r.Publisher == nil {
		r.Publisher, err = getUserByID(e, r.PublisherID)
		if err != nil {
			if IsErrUserNotExist(err) {
				r.Publisher = NewGhostUser()
			} else {
				return err
			}
		}
	}
	return getReleaseAttachments(e, r)
}

// LoadAttributes load repo and publisher attributes for a release
func (r *Release) LoadAttributes() error {
	return r.loadAttributes(x)
}

// APIURL the api url for a release. release must have attributes loaded
func (r *Release) APIURL() string {
	return fmt.Sprintf("%sapi/v1/repos/%s/releases/%d",
		setting.AppURL, r.Repo.FullName(), r.ID)
}

// ZipURL the zip url for a release. release must have attributes loaded
func (r *Release) ZipURL() string {
	return fmt.Sprintf("%s/archive/%s.zip", r.Repo.HTMLURL(), r.TagName)
}

// TarURL the tar.gz url for a release. release must have attributes loaded
func (r *Release) TarURL() string {
	return fmt.Sprintf("%s/archive/%s.tar.gz", r.Repo.HTMLURL(), r.TagName)
}

// HTMLURL the url for a release on the web UI. release must have attributes loaded
func (r *Release) HTMLURL() string {
	return fmt.Sprintf("%s/releases/tag/%s", r.Repo.HTMLURL(), r.TagName)
}

// IsReleaseExist returns true if release with given tag name already exists.
func IsReleaseExist(repoID int64, tagName string) (bool, error) {
	if len(tagName) == 0 {
		return false, nil
	}

	return x.Get(&Release{RepoID: repoID, LowerTagName: strings.ToLower(tagName)})
}

// InsertRelease inserts a release
func InsertRelease(rel *Release) error {
	_, err := x.Insert(rel)
	return err
}

// InsertReleasesContext insert releases
func InsertReleasesContext(ctx DBContext, rels []*Release) error {
	_, err := ctx.e.Insert(rels)
	return err
}

// UpdateRelease updates all columns of a release
func UpdateRelease(ctx DBContext, rel *Release) error {
	_, err := ctx.e.ID(rel.ID).AllCols().Update(rel)
	return err
}

// AddReleaseAttachments adds a release attachments
func AddReleaseAttachments(ctx DBContext, releaseID int64, attachmentUUIDs []string) (err error) {
	// Check attachments
	attachments, err := getAttachmentsByUUIDs(ctx.e, attachmentUUIDs)
	if err != nil {
		return fmt.Errorf("GetAttachmentsByUUIDs [uuids: %v]: %v", attachmentUUIDs, err)
	}

	for i := range attachments {
		if attachments[i].ReleaseID != 0 {
			return errors.New("release permission denied")
		}
		attachments[i].ReleaseID = releaseID
		// No assign value could be 0, so ignore AllCols().
		if _, err = ctx.e.ID(attachments[i].ID).Update(attachments[i]); err != nil {
			return fmt.Errorf("update attachment [%d]: %v", attachments[i].ID, err)
		}
	}

	return
}

// GetRelease returns release by given ID.
func GetRelease(repoID int64, tagName string) (*Release, error) {
	isExist, err := IsReleaseExist(repoID, tagName)
	if err != nil {
		return nil, err
	} else if !isExist {
		return nil, ErrReleaseNotExist{0, tagName}
	}

	rel := &Release{RepoID: repoID, LowerTagName: strings.ToLower(tagName)}
	_, err = x.Get(rel)
	return rel, err
}

// GetReleaseByID returns release with given ID.
func GetReleaseByID(id int64) (*Release, error) {
	rel := new(Release)
	has, err := x.
		ID(id).
		Get(rel)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrReleaseNotExist{id, ""}
	}

	return rel, nil
}

// FindReleasesOptions describes the conditions to Find releases
type FindReleasesOptions struct {
	ListOptions
	IncludeDrafts bool
	IncludeTags   bool
	IsPreRelease  util.OptionalBool
	IsDraft       util.OptionalBool
	TagNames      []string
}

func (opts *FindReleasesOptions) toConds(repoID int64) builder.Cond {
	cond := builder.NewCond()
	cond = cond.And(builder.Eq{"repo_id": repoID})

	if !opts.IncludeDrafts {
		cond = cond.And(builder.Eq{"is_draft": false})
	}
	if !opts.IncludeTags {
		cond = cond.And(builder.Eq{"is_tag": false})
	}
	if len(opts.TagNames) > 0 {
		cond = cond.And(builder.In("tag_name", opts.TagNames))
	}
	if !opts.IsPreRelease.IsNone() {
		cond = cond.And(builder.Eq{"is_prerelease": opts.IsPreRelease.IsTrue()})
	}
	if !opts.IsDraft.IsNone() {
		cond = cond.And(builder.Eq{"is_draft": opts.IsDraft.IsTrue()})
	}
	return cond
}

// GetReleasesByRepoID returns a list of releases of repository.
func GetReleasesByRepoID(repoID int64, opts FindReleasesOptions) ([]*Release, error) {
	sess := x.
		Desc("created_unix", "id").
		Where(opts.toConds(repoID))

	if opts.PageSize != 0 {
		sess = opts.setSessionPagination(sess)
	}

	rels := make([]*Release, 0, opts.PageSize)
	return rels, sess.Find(&rels)
}

// CountReleasesByRepoID returns a number of releases matching FindReleaseOptions and RepoID.
func CountReleasesByRepoID(repoID int64, opts FindReleasesOptions) (int64, error) {
	return x.Where(opts.toConds(repoID)).Count(new(Release))
}

// GetLatestReleaseByRepoID returns the latest release for a repository
func GetLatestReleaseByRepoID(repoID int64) (*Release, error) {
	cond := builder.NewCond().
		And(builder.Eq{"repo_id": repoID}).
		And(builder.Eq{"is_draft": false}).
		And(builder.Eq{"is_prerelease": false}).
		And(builder.Eq{"is_tag": false})

	rel := new(Release)
	has, err := x.
		Desc("created_unix", "id").
		Where(cond).
		Get(rel)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrReleaseNotExist{0, "latest"}
	}

	return rel, nil
}

// GetReleasesByRepoIDAndNames returns a list of releases of repository according repoID and tagNames.
func GetReleasesByRepoIDAndNames(ctx DBContext, repoID int64, tagNames []string) (rels []*Release, err error) {
	err = ctx.e.
		In("tag_name", tagNames).
		Desc("created_unix").
		Find(&rels, Release{RepoID: repoID})
	return rels, err
}

// GetReleaseCountByRepoID returns the count of releases of repository
func GetReleaseCountByRepoID(repoID int64, opts FindReleasesOptions) (int64, error) {
	return x.Where(opts.toConds(repoID)).Count(&Release{})
}

type releaseMetaSearch struct {
	ID  []int64
	Rel []*Release
}

func (s releaseMetaSearch) Len() int {
	return len(s.ID)
}

func (s releaseMetaSearch) Swap(i, j int) {
	s.ID[i], s.ID[j] = s.ID[j], s.ID[i]
	s.Rel[i], s.Rel[j] = s.Rel[j], s.Rel[i]
}

func (s releaseMetaSearch) Less(i, j int) bool {
	return s.ID[i] < s.ID[j]
}

// GetReleaseAttachments retrieves the attachments for releases
func GetReleaseAttachments(rels ...*Release) (err error) {
	return getReleaseAttachments(x, rels...)
}

func getReleaseAttachments(e Engine, rels ...*Release) (err error) {
	if len(rels) == 0 {
		return
	}

	// To keep this efficient as possible sort all releases by id,
	//    select attachments by release id,
	//    then merge join them

	// Sort
	sortedRels := releaseMetaSearch{ID: make([]int64, len(rels)), Rel: make([]*Release, len(rels))}
	var attachments []*Attachment
	for index, element := range rels {
		element.Attachments = []*Attachment{}
		sortedRels.ID[index] = element.ID
		sortedRels.Rel[index] = element
	}
	sort.Sort(sortedRels)

	// Select attachments
	err = e.
		Asc("release_id", "name").
		In("release_id", sortedRels.ID).
		Find(&attachments, Attachment{})
	if err != nil {
		return err
	}

	// merge join
	currentIndex := 0
	for _, attachment := range attachments {
		for sortedRels.ID[currentIndex] < attachment.ReleaseID {
			currentIndex++
		}
		sortedRels.Rel[currentIndex].Attachments = append(sortedRels.Rel[currentIndex].Attachments, attachment)
	}

	return
}

type releaseSorter struct {
	rels []*Release
}

func (rs *releaseSorter) Len() int {
	return len(rs.rels)
}

func (rs *releaseSorter) Less(i, j int) bool {
	diffNum := rs.rels[i].NumCommits - rs.rels[j].NumCommits
	if diffNum != 0 {
		return diffNum > 0
	}
	return rs.rels[i].CreatedUnix > rs.rels[j].CreatedUnix
}

func (rs *releaseSorter) Swap(i, j int) {
	rs.rels[i], rs.rels[j] = rs.rels[j], rs.rels[i]
}

// SortReleases sorts releases by number of commits and created time.
func SortReleases(rels []*Release) {
	sorter := &releaseSorter{rels: rels}
	sort.Sort(sorter)
}

// DeleteReleaseByID deletes a release from database by given ID.
func DeleteReleaseByID(id int64) error {
	_, err := x.ID(id).Delete(new(Release))
	return err
}

// UpdateReleasesMigrationsByType updates all migrated repositories' releases from gitServiceType to replace originalAuthorID to posterID
func UpdateReleasesMigrationsByType(gitServiceType structs.GitServiceType, originalAuthorID string, posterID int64) error {
	_, err := x.Table("release").
		Where("repo_id IN (SELECT id FROM repository WHERE original_service_type = ?)", gitServiceType).
		And("original_author_id = ?", originalAuthorID).
		Update(map[string]interface{}{
			"publisher_id":       posterID,
			"original_author":    "",
			"original_author_id": 0,
		})
	return err
}
