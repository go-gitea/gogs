// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package org

import (
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/user"
	"code.gitea.io/gitea/routers/api/v1/utils"
)

// ListTeams list all the teams of an organization
func ListTeams(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/teams organization orgListTeams
	// ---
	// summary: List an organization's teams
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/TeamList"

	org := ctx.Org.Organization
	if err := org.GetTeams(&models.SearchTeamOptions{
		ListOptions: utils.GetListOptions(ctx),
	}); err != nil {
		ctx.Error(http.StatusInternalServerError, "GetTeams", err)
		return
	}

	apiTeams, err := convert.ToTeams(org.Teams)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "ConvertToTeams", err)
		return
	}

	ctx.JSON(http.StatusOK, apiTeams)
}

// ListUserTeams list all the teams a user belongs to
func ListUserTeams(ctx *context.APIContext) {
	// swagger:operation GET /user/teams user userListTeams
	// ---
	// summary: List all the teams a user belongs to
	// produces:
	// - application/json
	// parameters:
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/TeamList"

	teams, err := models.GetUserTeams(ctx.User.ID, utils.GetListOptions(ctx))
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetUserTeams", err)
		return
	}

	apiTeams, err := convert.ToTeams(teams)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "ConvertToTeams", err)
		return
	}

	ctx.JSON(http.StatusOK, apiTeams)
}

// GetTeam api for get a team
func GetTeam(ctx *context.APIContext) {
	// swagger:operation GET /teams/{id} organization orgGetTeam
	// ---
	// summary: Get a team
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the team to get
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Team"

	apiTeam, err := convert.ToTeam(ctx.Org.Team)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	ctx.JSON(http.StatusOK, apiTeam)
}

// CreateTeam api for create a team
func CreateTeam(ctx *context.APIContext) {
	// swagger:operation POST /orgs/{org}/teams organization orgCreateTeam
	// ---
	// summary: Create a team
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateTeamOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Team"
	//   "422":
	//     "$ref": "#/responses/validationError"
	form := web.GetForm(ctx).(*api.CreateTeamOption)
	team := &models.Team{
		OrgID:                   ctx.Org.Organization.ID,
		Name:                    form.Name,
		Description:             form.Description,
		IncludesAllRepositories: form.IncludesAllRepositories,
		CanCreateOrgRepo:        form.CanCreateOrgRepo,
		Authorize:               models.ParseAccessMode(form.Permission),
	}

	unitTypes := models.FindUnitTypes(form.Units...)

	if team.Authorize < models.AccessModeOwner {
		var units = make([]*models.TeamUnit, 0, len(form.Units))
		for _, tp := range unitTypes {
			units = append(units, &models.TeamUnit{
				OrgID: ctx.Org.Organization.ID,
				Type:  tp,
			})
		}
		team.Units = units
	}

	if err := models.NewTeam(team); err != nil {
		if models.IsErrTeamAlreadyExist(err) {
			ctx.Error(http.StatusUnprocessableEntity, "", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "NewTeam", err)
		}
		return
	}

	apiTeam, err := convert.ToTeam(ctx.Org.Team)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}
	ctx.JSON(http.StatusCreated, apiTeam)
}

// EditTeam api for edit a team
func EditTeam(ctx *context.APIContext) {
	// swagger:operation PATCH /teams/{id} organization orgEditTeam
	// ---
	// summary: Edit a team
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the team to edit
	//   type: integer
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditTeamOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Team"

	form := web.GetForm(ctx).(*api.EditTeamOption)

	team := ctx.Org.Team
	if err := team.GetUnits(); err != nil {
		ctx.InternalServerError(err)
		return
	}

	if form.CanCreateOrgRepo != nil {
		team.CanCreateOrgRepo = *form.CanCreateOrgRepo
	}

	if len(form.Name) > 0 {
		team.Name = form.Name
	}

	if form.Description != nil {
		team.Description = *form.Description
	}

	isAuthChanged := false
	isIncludeAllChanged := false
	if !team.IsOwnerTeam() && len(form.Permission) != 0 {
		// Validate permission level.
		auth := models.ParseAccessMode(form.Permission)

		if team.Authorize != auth {
			isAuthChanged = true
			team.Authorize = auth
		}

		if form.IncludesAllRepositories != nil {
			isIncludeAllChanged = true
			team.IncludesAllRepositories = *form.IncludesAllRepositories
		}
	}

	if team.Authorize < models.AccessModeOwner {
		if len(form.Units) > 0 {
			var units = make([]*models.TeamUnit, 0, len(form.Units))
			unitTypes := models.FindUnitTypes(form.Units...)
			for _, tp := range unitTypes {
				units = append(units, &models.TeamUnit{
					OrgID: ctx.Org.Team.OrgID,
					Type:  tp,
				})
			}
			team.Units = units
		}
	}

	if err := models.UpdateTeam(team, isAuthChanged, isIncludeAllChanged); err != nil {
		ctx.Error(http.StatusInternalServerError, "EditTeam", err)
		return
	}

	apiTeam, err := convert.ToTeam(ctx.Org.Team)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}
	ctx.JSON(http.StatusOK, apiTeam)
}

// DeleteTeam api for delete a team
func DeleteTeam(ctx *context.APIContext) {
	// swagger:operation DELETE /teams/{id} organization orgDeleteTeam
	// ---
	// summary: Delete a team
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the team to delete
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     description: team deleted

	if err := models.DeleteTeam(ctx.Org.Team); err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteTeam", err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// GetTeamMembers api for get a team's members
func GetTeamMembers(ctx *context.APIContext) {
	// swagger:operation GET /teams/{id}/members organization orgListTeamMembers
	// ---
	// summary: List a team's members
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the team
	//   type: integer
	//   format: int64
	//   required: true
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/UserList"

	isMember, err := models.IsOrganizationMember(ctx.Org.Team.OrgID, ctx.User.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "IsOrganizationMember", err)
		return
	} else if !isMember && !ctx.User.IsAdmin {
		ctx.NotFound()
		return
	}
	team := ctx.Org.Team
	if err := team.GetMembers(&models.SearchMembersOptions{
		ListOptions: utils.GetListOptions(ctx),
	}); err != nil {
		ctx.Error(http.StatusInternalServerError, "GetTeamMembers", err)
		return
	}
	members := make([]*api.User, len(team.Members))
	for i, member := range team.Members {
		members[i] = convert.ToUser(member, ctx.User)
	}
	ctx.JSON(http.StatusOK, members)
}

// GetTeamMember api for get a particular member of team
func GetTeamMember(ctx *context.APIContext) {
	// swagger:operation GET /teams/{id}/members/{username} organization orgListTeamMember
	// ---
	// summary: List a particular member of team
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the team
	//   type: integer
	//   format: int64
	//   required: true
	// - name: username
	//   in: path
	//   description: username of the member to list
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/User"
	//   "404":
	//     "$ref": "#/responses/notFound"

	u := user.GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	teamID := ctx.ParamsInt64("teamid")
	isTeamMember, err := models.IsUserInTeams(u.ID, []int64{teamID})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "IsUserInTeams", err)
		return
	} else if !isTeamMember {
		ctx.NotFound()
		return
	}
	ctx.JSON(http.StatusOK, convert.ToUser(u, ctx.User))
}

// AddTeamMember api for add a member to a team
func AddTeamMember(ctx *context.APIContext) {
	// swagger:operation PUT /teams/{id}/members/{username} organization orgAddTeamMember
	// ---
	// summary: Add a team member
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the team
	//   type: integer
	//   format: int64
	//   required: true
	// - name: username
	//   in: path
	//   description: username of the user to add
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"

	u := user.GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	if err := ctx.Org.Team.AddMember(u.ID); err != nil {
		ctx.Error(http.StatusInternalServerError, "AddMember", err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// RemoveTeamMember api for remove one member from a team
func RemoveTeamMember(ctx *context.APIContext) {
	// swagger:operation DELETE /teams/{id}/members/{username} organization orgRemoveTeamMember
	// ---
	// summary: Remove a team member
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the team
	//   type: integer
	//   format: int64
	//   required: true
	// - name: username
	//   in: path
	//   description: username of the user to remove
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"

	u := user.GetUserByParams(ctx)
	if ctx.Written() {
		return
	}

	if err := ctx.Org.Team.RemoveMember(u.ID); err != nil {
		ctx.Error(http.StatusInternalServerError, "RemoveMember", err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// GetTeamRepos api for get a team's repos
func GetTeamRepos(ctx *context.APIContext) {
	// swagger:operation GET /teams/{id}/repos organization orgListTeamRepos
	// ---
	// summary: List a team's repos
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the team
	//   type: integer
	//   format: int64
	//   required: true
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/RepositoryList"

	team := ctx.Org.Team
	if err := team.GetRepositories(&models.SearchTeamOptions{
		ListOptions: utils.GetListOptions(ctx),
	}); err != nil {
		ctx.Error(http.StatusInternalServerError, "GetTeamRepos", err)
	}
	repos := make([]*api.Repository, len(team.Repos))
	for i, repo := range team.Repos {
		access, err := models.AccessLevel(ctx.User, repo)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetTeamRepos", err)
			return
		}
		repos[i] = convert.ToRepo(repo, access)
	}
	ctx.JSON(http.StatusOK, repos)
}

// getRepositoryByParams get repository by a team's organization ID and repo name
func getRepositoryByParams(ctx *context.APIContext) *models.Repository {
	repo, err := models.GetRepositoryByName(ctx.Org.Team.OrgID, ctx.Params(":reponame"))
	if err != nil {
		if models.IsErrRepoNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetRepositoryByName", err)
		}
		return nil
	}
	return repo
}

// AddTeamRepository api for adding a repository to a team
func AddTeamRepository(ctx *context.APIContext) {
	// swagger:operation PUT /teams/{id}/repos/{org}/{repo} organization orgAddTeamRepository
	// ---
	// summary: Add a repository to a team
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the team
	//   type: integer
	//   format: int64
	//   required: true
	// - name: org
	//   in: path
	//   description: organization that owns the repo to add
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo to add
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	repo := getRepositoryByParams(ctx)
	if ctx.Written() {
		return
	}
	if access, err := models.AccessLevel(ctx.User, repo); err != nil {
		ctx.Error(http.StatusInternalServerError, "AccessLevel", err)
		return
	} else if access < models.AccessModeAdmin {
		ctx.Error(http.StatusForbidden, "", "Must have admin-level access to the repository")
		return
	}
	if err := ctx.Org.Team.AddRepository(repo); err != nil {
		ctx.Error(http.StatusInternalServerError, "AddRepository", err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// RemoveTeamRepository api for removing a repository from a team
func RemoveTeamRepository(ctx *context.APIContext) {
	// swagger:operation DELETE /teams/{id}/repos/{org}/{repo} organization orgRemoveTeamRepository
	// ---
	// summary: Remove a repository from a team
	// description: This does not delete the repository, it only removes the
	//              repository from the team.
	// produces:
	// - application/json
	// parameters:
	// - name: id
	//   in: path
	//   description: id of the team
	//   type: integer
	//   format: int64
	//   required: true
	// - name: org
	//   in: path
	//   description: organization that owns the repo to remove
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo to remove
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	repo := getRepositoryByParams(ctx)
	if ctx.Written() {
		return
	}
	if access, err := models.AccessLevel(ctx.User, repo); err != nil {
		ctx.Error(http.StatusInternalServerError, "AccessLevel", err)
		return
	} else if access < models.AccessModeAdmin {
		ctx.Error(http.StatusForbidden, "", "Must have admin-level access to the repository")
		return
	}
	if err := ctx.Org.Team.RemoveRepository(repo.ID); err != nil {
		ctx.Error(http.StatusInternalServerError, "RemoveRepository", err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// SearchTeam api for searching teams
func SearchTeam(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org}/teams/search organization teamSearch
	// ---
	// summary: Search for teams within an organization
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// - name: q
	//   in: query
	//   description: keywords to search
	//   type: string
	// - name: include_desc
	//   in: query
	//   description: include search within team description (defaults to true)
	//   type: boolean
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     description: "SearchResults of a successful search"
	//     schema:
	//       type: object
	//       properties:
	//         ok:
	//           type: boolean
	//         data:
	//           type: array
	//           items:
	//             "$ref": "#/definitions/Team"

	listOptions := utils.GetListOptions(ctx)

	opts := &models.SearchTeamOptions{
		UserID:      ctx.User.ID,
		Keyword:     strings.TrimSpace(ctx.Query("q")),
		OrgID:       ctx.Org.Organization.ID,
		IncludeDesc: ctx.Query("include_desc") == "" || ctx.QueryBool("include_desc"),
		ListOptions: listOptions,
	}

	teams, maxResults, err := models.SearchTeam(opts)
	if err != nil {
		log.Error("SearchTeam failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
			"ok":    false,
			"error": "SearchTeam internal failure",
		})
		return
	}

	apiTeams, err := convert.ToTeams(teams)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	ctx.SetLinkHeader(int(maxResults), listOptions.PageSize)
	ctx.Header().Set("X-Total-Count", fmt.Sprintf("%d", maxResults))
	ctx.Header().Set("Access-Control-Expose-Headers", "X-Total-Count, Link")
	ctx.JSON(http.StatusOK, map[string]interface{}{
		"ok":   true,
		"data": apiTeams,
	})

}
