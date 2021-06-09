// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.package models

package models

import (
	"fmt"
	"testing"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
)

func TestGetUserHeatmapDataByUser(t *testing.T) {
	testCases := []struct {
		userID      int64
		doerID      int64
		CountResult int
		JSONResult  string
	}{
		{2, 2, 1, `[{"timestamp":1603152000,"contributions":1}]`}, // self looks at action in private repo
		{2, 1, 1, `[{"timestamp":1603152000,"contributions":1}]`}, // admin looks at action in private repo
		{2, 3, 0, `[]`}, // other user looks at action in private repo
		{2, 0, 0, `[]`}, // nobody looks at action in private repo
		{16, 15, 1, `[{"timestamp":1603238400,"contributions":1}]`}, // collaborator looks at action in private repo
		{3, 3, 0, `[]`}, // no action action not performed by target user
	}
	// Prepare
	assert.NoError(t, PrepareTestDatabase())

	for i, tc := range testCases {
		user := AssertExistsAndLoadBean(t, &User{ID: tc.userID}).(*User)

		doer := &User{ID: tc.doerID}
		_, err := loadBeanIfExists(doer)
		assert.NoError(t, err)
		if tc.doerID == 0 {
			doer = nil
		}

		// get the action for comparison
		actions, err := GetFeeds(GetFeedsOptions{
			RequestedUser:   user,
			Actor:           doer,
			IncludePrivate:  true,
			OnlyPerformedBy: true,
			IncludeDeleted:  true,
		})
		assert.NoError(t, err)

		// Get the heatmap and compare
		heatmap, err := GetUserHeatmapDataByUser(user, doer)
		assert.NoError(t, err)
		assert.Len(t, heatmap, len(actions), "invalid action count: did the test data became too old?")
		assert.Len(t, heatmap, tc.CountResult, fmt.Sprintf("testcase %d", i))

		//Test JSON rendering
		jsonData, err := json.Marshal(heatmap)
		assert.NoError(t, err)
		assert.Equal(t, tc.JSONResult, string(jsonData))
	}
}
