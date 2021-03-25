// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

const (
	privateActivityTestAdmin = "user1"
	privateActivityTestUser  = "user2"
)

// user3 is an organization so it is not usable here
const privateActivityTestOtherUser = "user4"

// activity helpers

func testPrivateActivityDoSomethingForActionEntries(t *testing.T) {
	repoBefore := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: repoBefore.OwnerID}).(*models.User)

	session := loginUser(t, privateActivityTestUser)
	token := getTokenForLoggedInUser(t, session)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues?state=all&token=%s", owner.Name, repoBefore.Name, token)
	req := NewRequestWithJSON(t, "POST", urlStr, &api.CreateIssueOption{
		Body:  "test",
		Title: "test",
	})
	session.MakeRequest(t, req, http.StatusCreated)
}

// private activity helpers

func testPrivateActivityHelperEnablePrivateActivity(t *testing.T) {
	session := loginUser(t, privateActivityTestUser)
	req := NewRequestWithValues(t, "POST", "/user/settings", map[string]string{
		"_csrf":                 GetCSRF(t, session, "/user/settings"),
		"name":                  privateActivityTestUser,
		"email":                 privateActivityTestUser + "@example.com",
		"language":              "en-US",
		"keep_activity_private": "1",
	})
	session.MakeRequest(t, req, http.StatusFound)
}

func testPrivateActivityHelperHasVisibleActivitiesInHTMLDoc(htmlDoc *HTMLDoc) bool {
	return htmlDoc.doc.Find(".feeds").Find(".news").Length() > 0
}

func testPrivateActivityHelperHasVisibleActivitiesFromSession(t *testing.T, session *TestSession) bool {
	req := NewRequestf(t, "GET", "/%s?tab=activity", privateActivityTestUser)
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)

	return testPrivateActivityHelperHasVisibleActivitiesInHTMLDoc(htmlDoc)
}

func testPrivateActivityHelperHasVisibleActivitiesFromPublic(t *testing.T) bool {
	req := NewRequestf(t, "GET", "/%s?tab=activity", privateActivityTestUser)
	resp := MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)

	return testPrivateActivityHelperHasVisibleActivitiesInHTMLDoc(htmlDoc)
}

// heatmap UI helpers

func testPrivateActivityHelperHasVisibleHeatmapInHTMLDoc(htmlDoc *HTMLDoc) bool {
	return htmlDoc.doc.Find("#user-heatmap").Length() > 0
}

func testPrivateActivityHelperHasVisibleProfileHeatmapFromSession(t *testing.T, session *TestSession) bool {
	req := NewRequestf(t, "GET", "/%s?tab=activity", privateActivityTestUser)
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)

	return testPrivateActivityHelperHasVisibleHeatmapInHTMLDoc(htmlDoc)
}

func testPrivateActivityHelperHasVisibleDashboardHeatmapFromSession(t *testing.T, session *TestSession) bool {
	req := NewRequest(t, "GET", "/")
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)

	return testPrivateActivityHelperHasVisibleHeatmapInHTMLDoc(htmlDoc)
}

func testPrivateActivityHelperHasVisibleHeatmapFromPublic(t *testing.T) bool {
	req := NewRequestf(t, "GET", "/%s?tab=activity", privateActivityTestUser)
	resp := MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)

	return testPrivateActivityHelperHasVisibleHeatmapInHTMLDoc(htmlDoc)
}

// heatmap API helpers

func testPrivateActivityHelperHasHeatmapContentFromPublic(t *testing.T) bool {
	req := NewRequestf(t, "GET", "/api/v1/users/%s/heatmap", privateActivityTestUser)
	resp := MakeRequest(t, req, http.StatusOK)

	var items []*models.UserHeatmapData
	DecodeJSON(t, resp, &items)

	return len(items) != 0
}

func testPrivateActivityHelperHasHeatmapContentFromSession(t *testing.T, session *TestSession) bool {
	token := getTokenForLoggedInUser(t, session)

	req := NewRequestf(t, "GET", "/api/v1/users/%s/heatmap?token=%s", privateActivityTestUser, token)
	resp := session.MakeRequest(t, req, http.StatusOK)

	var items []*models.UserHeatmapData
	DecodeJSON(t, resp, &items)

	return len(items) != 0
}

// check activity visibility if the visibility is enabled

func TestPrivateActivityNoVisibleForPublic(t *testing.T) {
	defer prepareTestEnv(t)()
	testPrivateActivityDoSomethingForActionEntries(t)

	visible := testPrivateActivityHelperHasVisibleActivitiesFromPublic(t)

	assert.True(t, visible, "user should have visible activities")
}

func TestPrivateActivityNoVisibleForUserItself(t *testing.T) {
	defer prepareTestEnv(t)()
	testPrivateActivityDoSomethingForActionEntries(t)

	session := loginUser(t, privateActivityTestUser)
	visible := testPrivateActivityHelperHasVisibleActivitiesFromSession(t, session)

	assert.True(t, visible, "user should have visible activities")
}

func TestPrivateActivityNoVisibleForOtherUser(t *testing.T) {
	defer prepareTestEnv(t)()
	testPrivateActivityDoSomethingForActionEntries(t)

	session := loginUser(t, privateActivityTestOtherUser)
	visible := testPrivateActivityHelperHasVisibleActivitiesFromSession(t, session)

	assert.True(t, visible, "user should have visible activities")
}

func TestPrivateActivityNoVisibleForAdmin(t *testing.T) {
	defer prepareTestEnv(t)()
	testPrivateActivityDoSomethingForActionEntries(t)

	session := loginUser(t, privateActivityTestAdmin)
	visible := testPrivateActivityHelperHasVisibleActivitiesFromSession(t, session)

	assert.True(t, visible, "user should have visible activities")
}

// check activity visibility if the visibility is disabled

func TestPrivateActivityYesInvisibleForPublic(t *testing.T) {
	defer prepareTestEnv(t)()
	testPrivateActivityDoSomethingForActionEntries(t)
	testPrivateActivityHelperEnablePrivateActivity(t)

	visible := testPrivateActivityHelperHasVisibleActivitiesFromPublic(t)

	assert.False(t, visible, "user should have no visible activities")
}

func TestPrivateActivityYesVisibleForUserItself(t *testing.T) {
	defer prepareTestEnv(t)()
	testPrivateActivityDoSomethingForActionEntries(t)
	testPrivateActivityHelperEnablePrivateActivity(t)

	session := loginUser(t, privateActivityTestUser)
	visible := testPrivateActivityHelperHasVisibleActivitiesFromSession(t, session)

	assert.True(t, visible, "user should have visible activities")
}

func TestPrivateActivityYesInvisibleForOtherUser(t *testing.T) {
	defer prepareTestEnv(t)()
	testPrivateActivityDoSomethingForActionEntries(t)
	testPrivateActivityHelperEnablePrivateActivity(t)

	session := loginUser(t, privateActivityTestOtherUser)
	visible := testPrivateActivityHelperHasVisibleActivitiesFromSession(t, session)

	assert.False(t, visible, "user should have no visible activities")
}

func TestPrivateActivityYesVisibleForAdmin(t *testing.T) {
	defer prepareTestEnv(t)()
	testPrivateActivityDoSomethingForActionEntries(t)
	testPrivateActivityHelperEnablePrivateActivity(t)

	session := loginUser(t, privateActivityTestAdmin)
	visible := testPrivateActivityHelperHasVisibleActivitiesFromSession(t, session)

	assert.True(t, visible, "user should have visible activities")
}

// check heatmap visibility if the visibility is enabled

func TestPrivateActivityNoHeatmapVisibleForPublic(t *testing.T) {
	defer prepareTestEnv(t)()
	testPrivateActivityDoSomethingForActionEntries(t)

	visible := testPrivateActivityHelperHasVisibleHeatmapFromPublic(t)

	assert.True(t, visible, "user should have visible heatmap")
}

func TestPrivateActivityNoHeatmapVisibleForUserItselfAtProfile(t *testing.T) {
	defer prepareTestEnv(t)()
	testPrivateActivityDoSomethingForActionEntries(t)

	session := loginUser(t, privateActivityTestUser)
	visible := testPrivateActivityHelperHasVisibleProfileHeatmapFromSession(t, session)

	assert.True(t, visible, "user should have visible heatmap")
}

func TestPrivateActivityNoHeatmapVisibleForUserItselfAtDashboard(t *testing.T) {
	defer prepareTestEnv(t)()
	testPrivateActivityDoSomethingForActionEntries(t)

	session := loginUser(t, privateActivityTestUser)
	visible := testPrivateActivityHelperHasVisibleDashboardHeatmapFromSession(t, session)

	assert.True(t, visible, "user should have visible heatmap")
}

func TestPrivateActivityNoHeatmapVisibleForOtherUser(t *testing.T) {
	defer prepareTestEnv(t)()
	testPrivateActivityDoSomethingForActionEntries(t)

	session := loginUser(t, privateActivityTestOtherUser)
	visible := testPrivateActivityHelperHasVisibleProfileHeatmapFromSession(t, session)

	assert.True(t, visible, "user should have visible heatmap")
}

func TestPrivateActivityNoHeatmapVisibleForAdmin(t *testing.T) {
	defer prepareTestEnv(t)()
	testPrivateActivityDoSomethingForActionEntries(t)

	session := loginUser(t, privateActivityTestAdmin)
	visible := testPrivateActivityHelperHasVisibleProfileHeatmapFromSession(t, session)

	assert.True(t, visible, "user should have visible heatmap")
}

// check heatmap visibility if the visibility is disabled

func TestPrivateActivityYesHeatmapInvisibleForPublic(t *testing.T) {
	defer prepareTestEnv(t)()
	testPrivateActivityDoSomethingForActionEntries(t)
	testPrivateActivityHelperEnablePrivateActivity(t)

	visible := testPrivateActivityHelperHasVisibleHeatmapFromPublic(t)

	assert.False(t, visible, "user should have no visible heatmap")
}

func TestPrivateActivityYesHeatmapVisibleForUserItselfAtProfile(t *testing.T) {
	defer prepareTestEnv(t)()
	testPrivateActivityDoSomethingForActionEntries(t)
	testPrivateActivityHelperEnablePrivateActivity(t)

	session := loginUser(t, privateActivityTestUser)
	visible := testPrivateActivityHelperHasVisibleProfileHeatmapFromSession(t, session)

	assert.True(t, visible, "user should have visible heatmap")
}

func TestPrivateActivityYesHeatmapVisibleForUserItselfAtDashboard(t *testing.T) {
	defer prepareTestEnv(t)()
	testPrivateActivityDoSomethingForActionEntries(t)
	testPrivateActivityHelperEnablePrivateActivity(t)

	session := loginUser(t, privateActivityTestUser)
	visible := testPrivateActivityHelperHasVisibleDashboardHeatmapFromSession(t, session)

	assert.True(t, visible, "user should have visible heatmap")
}

func TestPrivateActivityYesHeatmapInvisibleForOtherUser(t *testing.T) {
	defer prepareTestEnv(t)()
	testPrivateActivityDoSomethingForActionEntries(t)
	testPrivateActivityHelperEnablePrivateActivity(t)

	session := loginUser(t, privateActivityTestOtherUser)
	visible := testPrivateActivityHelperHasVisibleProfileHeatmapFromSession(t, session)

	assert.False(t, visible, "user should have no visible heatmap")
}

func TestPrivateActivityYesHeatmapVisibleForAdmin(t *testing.T) {
	defer prepareTestEnv(t)()
	testPrivateActivityDoSomethingForActionEntries(t)
	testPrivateActivityHelperEnablePrivateActivity(t)

	session := loginUser(t, privateActivityTestAdmin)
	visible := testPrivateActivityHelperHasVisibleProfileHeatmapFromSession(t, session)

	assert.True(t, visible, "user should have visible heatmap")
}

// check heatmap api provides content if the visibility is enabled

func TestPrivateActivityNoHeatmapHasContentForPublic(t *testing.T) {
	defer prepareTestEnv(t)()
	testPrivateActivityDoSomethingForActionEntries(t)

	hasContent := testPrivateActivityHelperHasHeatmapContentFromPublic(t)

	assert.True(t, hasContent, "user should have heatmap content")
}

func TestPrivateActivityNoHeatmapHasContentForUserItself(t *testing.T) {
	defer prepareTestEnv(t)()
	testPrivateActivityDoSomethingForActionEntries(t)

	session := loginUser(t, privateActivityTestUser)
	hasContent := testPrivateActivityHelperHasHeatmapContentFromSession(t, session)

	assert.True(t, hasContent, "user should have heatmap content")
}

func TestPrivateActivityNoHeatmapHasContentForOtherUser(t *testing.T) {
	defer prepareTestEnv(t)()
	testPrivateActivityDoSomethingForActionEntries(t)

	session := loginUser(t, privateActivityTestOtherUser)
	hasContent := testPrivateActivityHelperHasHeatmapContentFromSession(t, session)

	assert.True(t, hasContent, "user should have heatmap content")
}

func TestPrivateActivityNoHeatmapHasContentForAdmin(t *testing.T) {
	defer prepareTestEnv(t)()
	testPrivateActivityDoSomethingForActionEntries(t)

	session := loginUser(t, privateActivityTestAdmin)
	hasContent := testPrivateActivityHelperHasHeatmapContentFromSession(t, session)

	assert.True(t, hasContent, "user should have heatmap content")
}

// check heatmap api provides no content if the visibility is disabled
// this should be equal to the hidden heatmap at the UI

func TestPrivateActivityYesHeatmapHasNoContentForPublic(t *testing.T) {
	defer prepareTestEnv(t)()
	testPrivateActivityDoSomethingForActionEntries(t)
	testPrivateActivityHelperEnablePrivateActivity(t)

	hasContent := testPrivateActivityHelperHasHeatmapContentFromPublic(t)

	assert.False(t, hasContent, "user should have no heatmap content")
}

func TestPrivateActivityYesHeatmapHasNoContentForUserItself(t *testing.T) {
	defer prepareTestEnv(t)()
	testPrivateActivityDoSomethingForActionEntries(t)
	testPrivateActivityHelperEnablePrivateActivity(t)

	session := loginUser(t, privateActivityTestUser)
	hasContent := testPrivateActivityHelperHasHeatmapContentFromSession(t, session)

	assert.True(t, hasContent, "user should see their own heatmap content")
}

func TestPrivateActivityYesHeatmapHasNoContentForOtherUser(t *testing.T) {
	defer prepareTestEnv(t)()
	testPrivateActivityDoSomethingForActionEntries(t)
	testPrivateActivityHelperEnablePrivateActivity(t)

	session := loginUser(t, privateActivityTestOtherUser)
	hasContent := testPrivateActivityHelperHasHeatmapContentFromSession(t, session)

	assert.False(t, hasContent, "other user should not see heatmap content")
}

func TestPrivateActivityYesHeatmapHasNoContentForAdmin(t *testing.T) {
	defer prepareTestEnv(t)()
	testPrivateActivityDoSomethingForActionEntries(t)
	testPrivateActivityHelperEnablePrivateActivity(t)

	session := loginUser(t, privateActivityTestAdmin)
	hasContent := testPrivateActivityHelperHasHeatmapContentFromSession(t, session)

	assert.True(t, hasContent, "heatmap should show content for admin")
}
