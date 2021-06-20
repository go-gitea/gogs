// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/url"
	"strings"
	"testing"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"
	"github.com/stretchr/testify/assert"
)

func TestOrgCounts(t *testing.T) {
	onGiteaRun(t, testOrgCounts)
}

func testOrgCounts(t *testing.T, u *url.URL) {
	orgOwner := "user2"
	orgName := "testOrg"
	orgCollaborator := "user4"
	ctx := NewAPITestContext(t, orgOwner, "repo1")

	var ownerCountRepos map[string]int
	var collabCountRepos map[string]int

	t.Run("GetTheOwnersNumRepos", doCheckOrgCounts(orgOwner, map[string]int{},
		false,
		func(_ *testing.T, calcOrgCounts map[string]int) {
			ownerCountRepos = calcOrgCounts
		},
	))
	t.Run("GetTheCollaboratorsNumRepos", doCheckOrgCounts(orgCollaborator, map[string]int{},
		false,
		func(_ *testing.T, calcOrgCounts map[string]int) {
			collabCountRepos = calcOrgCounts
		},
	))

	t.Run("CreatePublicTestOrganization", doAPICreateOrganization(ctx, &api.CreateOrgOption{
		UserName:   orgName,
		Visibility: "public",
	}))

	// Following the creation of the organization, the orgName must appear in the counts with 0 repos
	ownerCountRepos[orgName] = 0

	t.Run("AssertNumRepos0ForTestOrg", doCheckOrgCounts(orgOwner, ownerCountRepos, true))

	// the collaborator is not a collaborator yet
	t.Run("AssertNoTestOrgReposForCollaborator", doCheckOrgCounts(orgCollaborator, collabCountRepos, true))

	t.Run("CreateOrganizationPrivateRepo", doAPICreateOrganizationRepository(ctx, orgName, &api.CreateRepoOption{
		Name:     "privateTestRepo",
		AutoInit: true,
		Private:  true,
	}))

	ownerCountRepos[orgName] = 1
	t.Run("AssertNumRepos1ForTestOrg", doCheckOrgCounts(orgOwner, ownerCountRepos, true))

	t.Run("AssertNoTestOrgReposForCollaborator", doCheckOrgCounts(orgCollaborator, collabCountRepos, true))

	var testTeam api.Team

	t.Run("CreateTeamForPublicTestOrganization", doAPICreateOrganizationTeam(ctx, orgName, &api.CreateTeamOption{
		Name:             "test",
		Permission:       "read",
		Units:            []string{"repo.code", "repo.issues", "repo.wiki", "repo.pulls", "repo.releases"},
		CanCreateOrgRepo: true,
	}, func(_ *testing.T, team api.Team) {
		testTeam = team
	}))

	t.Run("AssertNoTestOrgReposForCollaborator", doCheckOrgCounts(orgCollaborator, collabCountRepos, true))

	t.Run("AddCollboratorToTeam", doAPIAddUserToOrganizationTeam(ctx, testTeam.ID, orgCollaborator))

	collabCountRepos[orgName] = 0
	t.Run("AssertNumRepos0ForTestOrgForCollaborator", doCheckOrgCounts(orgOwner, ownerCountRepos, true))

	// Now create a Public Repo
	t.Run("CreateOrganizationPublicRepo", doAPICreateOrganizationRepository(ctx, orgName, &api.CreateRepoOption{
		Name:     "publicTestRepo",
		AutoInit: true,
	}))

	ownerCountRepos[orgName] = 2
	t.Run("AssertNumRepos2ForTestOrg", doCheckOrgCounts(orgOwner, ownerCountRepos, true))
	collabCountRepos[orgName] = 1
	t.Run("AssertNumRepos1ForTestOrgForCollaborator", doCheckOrgCounts(orgOwner, ownerCountRepos, true))

	// Now add the testTeam to the privateRepo
	t.Run("AddTestTeamToPrivateRepo", doAPIAddRepoToOrganizationTeam(ctx, testTeam.ID, orgName, "privateTestRepo"))

	t.Run("AssertNumRepos2ForTestOrg", doCheckOrgCounts(orgOwner, ownerCountRepos, true))
	collabCountRepos[orgName] = 2
	t.Run("AssertNumRepos2ForTestOrgForCollaborator", doCheckOrgCounts(orgOwner, ownerCountRepos, true))
}

func doCheckOrgCounts(username string, orgCounts map[string]int, strict bool, callback ...func(*testing.T, map[string]int)) func(t *testing.T) {
	canonicalCounts := make(map[string]int, len(orgCounts))

	for key, value := range orgCounts {
		newKey := strings.TrimSpace(strings.ToLower(key))
		canonicalCounts[newKey] = value
	}

	return func(t *testing.T) {
		user := models.AssertExistsAndLoadBean(t, &models.User{
			Name: username,
		}).(*models.User)

		orgs, err := models.GetOrgsByUserID(user.ID, true)
		assert.NoError(t, err)

		calcOrgCounts := map[string]int{}

		for _, org := range orgs {
			calcOrgCounts[org.LowerName] = org.NumRepos
			count, ok := canonicalCounts[org.LowerName]
			if ok {
				assert.True(t, count == org.NumRepos, "Number of Repos in %s is %d when we expected %d", org.Name, org.NumRepos, count)
			} else {
				assert.False(t, strict, "Did not expect to see %s with count %d", org.Name, org.NumRepos)
			}
		}

		for key, value := range orgCounts {
			_, seen := calcOrgCounts[strings.TrimSpace(strings.ToLower(key))]
			assert.True(t, seen, "Expected to see %s with %d but did not", key, value)
		}

		if len(callback) > 0 {
			callback[0](t, calcOrgCounts)
		}
	}
}
