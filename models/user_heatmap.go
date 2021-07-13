// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.package models

package models

import (
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
)

// UserHeatmapData represents the data needed to create a heatmap
type UserHeatmapData struct {
	Timestamp     timeutil.TimeStamp `json:"timestamp"`
	Contributions int64              `json:"contributions"`
}

// GetUserHeatmapDataByUser returns an array of UserHeatmapData
func GetUserHeatmapDataByUser(user, doer *User) ([]*UserHeatmapData, error) {
	return getUserHeatmapData(user, nil, doer)
}

// GetUserHeatmapDataByUserTeam returns an array of UserHeatmapData
func GetUserHeatmapDataByUserTeam(user *User, team *Team, doer *User) ([]*UserHeatmapData, error) {
	return getUserHeatmapData(user, team, doer)
}

func getUserHeatmapData(user *User, team *Team, doer *User) ([]*UserHeatmapData, error) {
	hdata := make([]*UserHeatmapData, 0)

	if !activityReadable(user, doer) {
		return hdata, nil
	}

	// Group by 15 minute intervals which will allow the client to accurately shift the timestamp to their timezone.
	// The interval is based on the fact that there are timezones such as UTC +5:30 and UTC +12:45.
	groupBy := "created_unix / 900 * 900"
	groupByName := "timestamp" // We need this extra case because mssql doesn't allow grouping by alias
	switch {
	case setting.Database.UseMySQL:
		groupBy = "created_unix DIV 900 * 900"
	case setting.Database.UseMSSQL:
		groupByName = groupBy
	}

	cond, err := activityQueryCondition(GetFeedsOptions{
		RequestedUser:  user,
		RequestedTeam:  team,
		Actor:          doer,
		IncludePrivate: true, // don't filter by private, as we already filter by repo access
		IncludeDeleted: true,
		// * Heatmaps for individual users only include actions that the user themself did.
		// * For organizations actions by all users that were made in owned
		//   repositories are counted.
		OnlyPerformedBy: !user.IsOrganization(),
	})
	if err != nil {
		return nil, err
	}

	return hdata, x.
		Select(groupBy+" AS timestamp, count(user_id) as contributions").
		Table("action").
		Where(cond).
		And("created_unix > ?", timeutil.TimeStampNow()-31536000).
		GroupBy(groupByName).
		OrderBy("timestamp").
		Find(&hdata)
}
