// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// UnitType is Unit's Type
type UnitType int

// Enumerate all the unit types
const (
	UnitTypeCode            UnitType = iota + 1 // 1 code
	UnitTypeIssues                              // 2 issues
	UnitTypePullRequests                        // 3 PRs
	UnitTypeReleases                            // 4 Releases
	UnitTypeWiki                                // 5 Wiki
	UnitTypeExternalWiki                        // 6 ExternalWiki
	UnitTypeExternalTracker                     // 7 ExternalTracker
	UnitTypeProjects                            // 8 Kanban board
	UnitTypePackages                            // 9 Packages
)

// Value returns integer value for unit type
func (u UnitType) Value() int {
	return int(u)
}

func (u UnitType) String() string {
	switch u {
	case UnitTypeCode:
		return "UnitTypeCode"
	case UnitTypeIssues:
		return "UnitTypeIssues"
	case UnitTypePullRequests:
		return "UnitTypePullRequests"
	case UnitTypeReleases:
		return "UnitTypeReleases"
	case UnitTypeWiki:
		return "UnitTypeWiki"
	case UnitTypeExternalWiki:
		return "UnitTypeExternalWiki"
	case UnitTypeExternalTracker:
		return "UnitTypeExternalTracker"
	case UnitTypeProjects:
		return "UnitTypeProjects"
	case UnitTypePackages:
		return "UnitTypePackages"
	}
	return fmt.Sprintf("Unknown UnitType %d", u)
}

// ColorFormat provides a ColorFormatted version of this UnitType
func (u UnitType) ColorFormat(s fmt.State) {
	log.ColorFprintf(s, "%d:%s",
		log.NewColoredIDValue(u),
		u)
}

var (
	// AllRepoUnitTypes contains all the unit types
	AllRepoUnitTypes = []UnitType{
		UnitTypeCode,
		UnitTypeIssues,
		UnitTypePullRequests,
		UnitTypeReleases,
		UnitTypeWiki,
		UnitTypeExternalWiki,
		UnitTypeExternalTracker,
		UnitTypeProjects,
		UnitTypePackages,
	}

	// DefaultRepoUnits contains the default unit types
	DefaultRepoUnits = []UnitType{
		UnitTypeCode,
		UnitTypeIssues,
		UnitTypePullRequests,
		UnitTypeReleases,
		UnitTypeWiki,
		UnitTypeProjects,
	}

	// NotAllowedDefaultRepoUnits contains units that can't be default
	NotAllowedDefaultRepoUnits = []UnitType{
		UnitTypeExternalWiki,
		UnitTypeExternalTracker,
		UnitTypePackages,
	}

	// MustRepoUnits contains the units could not be disabled currently
	MustRepoUnits = []UnitType{
		UnitTypeCode,
		UnitTypeReleases,
	}

	// DisabledRepoUnits contains the units that have been globally disabled
	DisabledRepoUnits = []UnitType{}
)

func loadUnitConfig() {
	setDefaultRepoUnits := FindUnitTypes(setting.Repository.DefaultRepoUnits...)
	// Default repo units set if setting is not empty
	if len(setDefaultRepoUnits) > 0 {
		// MustRepoUnits required as default
		DefaultRepoUnits = make([]UnitType, len(MustRepoUnits))
		copy(DefaultRepoUnits, MustRepoUnits)
		for _, defaultU := range setDefaultRepoUnits {
			if !defaultU.CanBeDefault() {
				log.Warn("Not allowed as default unit: %s", defaultU.String())
				continue
			}
			// MustRepoUnits already added
			if defaultU.CanDisable() {
				DefaultRepoUnits = append(DefaultRepoUnits, defaultU)
			}
		}
	}

	DisabledRepoUnits = FindUnitTypes(setting.Repository.DisabledRepoUnits...)
	// Check that must units are not disabled
	for i, disabledU := range DisabledRepoUnits {
		if !disabledU.CanDisable() {
			log.Warn("Not allowed to global disable unit %s", disabledU.String())
			DisabledRepoUnits = append(DisabledRepoUnits[:i], DisabledRepoUnits[i+1:]...)
		}
	}
	// Remove disabled units from default units
	for _, disabledU := range DisabledRepoUnits {
		for i, defaultU := range DefaultRepoUnits {
			if defaultU == disabledU {
				DefaultRepoUnits = append(DefaultRepoUnits[:i], DefaultRepoUnits[i+1:]...)
			}
		}
	}
}

// UnitGlobalDisabled checks if unit type is global disabled
func (u UnitType) UnitGlobalDisabled() bool {
	for _, ud := range DisabledRepoUnits {
		if u == ud {
			return true
		}
	}
	return false
}

// CanDisable checks if this unit type can be disabled.
func (u *UnitType) CanDisable() bool {
	for _, mu := range MustRepoUnits {
		if *u == mu {
			return false
		}
	}
	return true
}

// CanBeDefault checks if the unit type can be a default repo unit
func (u *UnitType) CanBeDefault() bool {
	for _, nadU := range NotAllowedDefaultRepoUnits {
		if *u == nadU {
			return false
		}
	}
	return true
}

// Unit is a section of one repository
type Unit struct {
	Type    UnitType
	NameKey string
	URI     string
	DescKey string
	Idx     int
}

// CanDisable returns if this unit could be disabled.
func (u *Unit) CanDisable() bool {
	return u.Type.CanDisable()
}

// IsLessThan compares order of two units
func (u Unit) IsLessThan(unit Unit) bool {
	if (u.Type == UnitTypeExternalTracker || u.Type == UnitTypeExternalWiki) && unit.Type != UnitTypeExternalTracker && unit.Type != UnitTypeExternalWiki {
		return false
	}
	return u.Idx < unit.Idx
}

// Enumerate all the units
var (
	UnitCode = Unit{
		UnitTypeCode,
		"repo.code",
		"/",
		"repo.code.desc",
		0,
	}

	UnitIssues = Unit{
		UnitTypeIssues,
		"repo.issues",
		"/issues",
		"repo.issues.desc",
		1,
	}

	UnitExternalTracker = Unit{
		UnitTypeExternalTracker,
		"repo.ext_issues",
		"/issues",
		"repo.ext_issues.desc",
		1,
	}

	UnitPullRequests = Unit{
		UnitTypePullRequests,
		"repo.pulls",
		"/pulls",
		"repo.pulls.desc",
		2,
	}

	UnitReleases = Unit{
		UnitTypeReleases,
		"repo.releases",
		"/releases",
		"repo.releases.desc",
		3,
	}

	UnitWiki = Unit{
		UnitTypeWiki,
		"repo.wiki",
		"/wiki",
		"repo.wiki.desc",
		4,
	}

	UnitExternalWiki = Unit{
		UnitTypeExternalWiki,
		"repo.ext_wiki",
		"/wiki",
		"repo.ext_wiki.desc",
		4,
	}

	UnitProjects = Unit{
		UnitTypeProjects,
		"repo.projects",
		"/projects",
		"repo.projects.desc",
		5,
	}

	UnitPackages = Unit{
		UnitTypePackages,
		"repo.packages",
		"/packages",
		"repo.packages.desc",
		6,
	}

	// Units contains all the units
	Units = map[UnitType]Unit{
		UnitTypeCode:            UnitCode,
		UnitTypeIssues:          UnitIssues,
		UnitTypeExternalTracker: UnitExternalTracker,
		UnitTypePullRequests:    UnitPullRequests,
		UnitTypeReleases:        UnitReleases,
		UnitTypeWiki:            UnitWiki,
		UnitTypeExternalWiki:    UnitExternalWiki,
		UnitTypeProjects:        UnitProjects,
		UnitTypePackages:        UnitPackages,
	}
)

// FindUnitTypes give the unit key name and return unit
func FindUnitTypes(nameKeys ...string) (res []UnitType) {
	for _, key := range nameKeys {
		for t, u := range Units {
			if strings.EqualFold(key, u.NameKey) {
				res = append(res, t)
				break
			}
		}
	}
	return
}
