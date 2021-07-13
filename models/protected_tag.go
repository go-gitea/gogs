// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/gobwas/glob"
)

// ProtectedTag struct
type ProtectedTag struct {
	ID               int64 `xorm:"pk autoincr"`
	RepoID           int64
	NamePattern      string
	RegexPattern     *regexp.Regexp `xorm:"-"`
	GlobPattern      glob.Glob      `xorm:"-"`
	AllowlistUserIDs []int64        `xorm:"JSON TEXT"`
	AllowlistTeamIDs []int64        `xorm:"JSON TEXT"`

	CreatedUnix timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
}

// InsertProtectedTag inserts a protected tag to database
func InsertProtectedTag(pt *ProtectedTag) error {
	_, err := x.Insert(pt)
	return err
}

// UpdateProtectedTag updates the protected tag
func UpdateProtectedTag(pt *ProtectedTag) error {
	_, err := x.ID(pt.ID).AllCols().Update(pt)
	return err
}

// DeleteProtectedTag deletes a protected tag by ID
func DeleteProtectedTag(pt *ProtectedTag) error {
	_, err := x.ID(pt.ID).Delete(&ProtectedTag{})
	return err
}

// EnsureCompiledPattern ensures the glob pattern is compiled
func (pt *ProtectedTag) EnsureCompiledPattern() error {
	if pt.RegexPattern != nil || pt.GlobPattern != nil {
		return nil
	}

	var err error
	if len(pt.NamePattern) >= 2 && strings.HasPrefix(pt.NamePattern, "/") && strings.HasSuffix(pt.NamePattern, "/") {
		pt.RegexPattern, err = regexp.Compile(pt.NamePattern[1 : len(pt.NamePattern)-1])
	} else {
		pt.GlobPattern, err = glob.Compile(pt.NamePattern)
	}
	return err
}

// IsUserAllowed returns true if the user is allowed to modify the tag
func (pt *ProtectedTag) IsUserAllowed(userID int64) (bool, error) {
	if base.Int64sContains(pt.AllowlistUserIDs, userID) {
		return true, nil
	}

	if len(pt.AllowlistTeamIDs) == 0 {
		return false, nil
	}

	in, err := IsUserInTeams(userID, pt.AllowlistTeamIDs)
	if err != nil {
		return false, err
	}
	return in, nil
}

// GetProtectedTags gets all protected tags of the repository
func (repo *Repository) GetProtectedTags() ([]*ProtectedTag, error) {
	tags := make([]*ProtectedTag, 0)
	return tags, x.Find(&tags, &ProtectedTag{RepoID: repo.ID})
}

// GetProtectedTagByID gets the protected tag with the specific id
func GetProtectedTagByID(id int64) (*ProtectedTag, error) {
	tag := new(ProtectedTag)
	has, err := x.ID(id).Get(tag)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, nil
	}
	return tag, nil
}

// IsUserAllowedToControlTag checks if a user can control the specific tag.
// It returns true if the tag name is not protected or the user is allowed to control it.
func IsUserAllowedToControlTag(tags []*ProtectedTag, tagName string, userID int64) (bool, error) {
	isAllowed := true
	for _, tag := range tags {
		err := tag.EnsureCompiledPattern()
		if err != nil {
			return false, err
		}

		if !tag.matchString(tagName) {
			continue
		}

		isAllowed, err = tag.IsUserAllowed(userID)
		if err != nil {
			return false, err
		}
		if isAllowed {
			break
		}
	}

	return isAllowed, nil
}

func (pt *ProtectedTag) matchString(name string) bool {
	if pt.RegexPattern != nil {
		return pt.RegexPattern.MatchString(name)
	}
	return pt.GlobPattern.Match(name)
}
