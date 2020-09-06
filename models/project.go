// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"errors"
	"fmt"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

type (
	// ProjectsConfig is used to identify the type of board that is being created
	ProjectsConfig struct {
		BoardType   ProjectBoardType
		Translation string
	}

	// ProjectType is used to identify the type of project in question and ownership
	ProjectType uint8
)

const (
	// ProjectTypeIndividual is a type of project board that is owned by an individual
	ProjectTypeIndividual ProjectType = iota + 1

	// ProjectTypeRepository is a project that is tied to a repository
	ProjectTypeRepository

	// ProjectTypeOrganization is a project that is tied to an organisation
	ProjectTypeOrganization
)

// Project represents a project board
type Project struct {
	ID          int64  `xorm:"pk autoincr"`
	Title       string `xorm:"INDEX NOT NULL"`
	Description string `xorm:"TEXT"`
	RepoID      int64  `xorm:"INDEX"`
	CreatorID   int64  `xorm:"NOT NULL"`
	IsClosed    bool   `xorm:"INDEX"`
	BoardType   ProjectBoardType
	Type        ProjectType

	RenderedContent string `xorm:"-"`

	CreatedUnix    timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix    timeutil.TimeStamp `xorm:"INDEX updated"`
	ClosedDateUnix timeutil.TimeStamp
}

//TableName sets the table name to `project`
func (project *Project) TableName() string {
	return tbProject[1 : len(tbProject)-1]
}

// GetProjectsConfig retrieves the types of configurations projects could have
func GetProjectsConfig() []ProjectsConfig {
	return []ProjectsConfig{
		{ProjectBoardTypeNone, "repo.projects.type.none"},
		{ProjectBoardTypeBasicKanban, "repo.projects.type.basic_kanban"},
		{ProjectBoardTypeBugTriage, "repo.projects.type.bug_triage"},
	}
}

// IsProjectTypeValid checks if a project type is valid
func IsProjectTypeValid(p ProjectType) bool {
	switch p {
	case ProjectTypeRepository:
		return true
	default:
		return false
	}
}

// ProjectSearchOptions are options for GetProjects
type ProjectSearchOptions struct {
	RepoID   int64
	Page     int
	IsClosed util.OptionalBool
	SortType string
	Type     ProjectType
}

// GetProjects returns a list of all projects that have been created in the repository
func GetProjects(opts ProjectSearchOptions) ([]*Project, int64, error) {
	return getProjects(x, opts)
}

func getProjects(e Engine, opts ProjectSearchOptions) ([]*Project, int64, error) {

	projects := make([]*Project, 0, setting.UI.IssuePagingNum)

	var cond builder.Cond = builder.Eq{"repo_id": opts.RepoID}
	switch opts.IsClosed {
	case util.OptionalBoolTrue:
		cond = cond.And(builder.Eq{"is_closed": true})
	case util.OptionalBoolFalse:
		cond = cond.And(builder.Eq{"is_closed": false})
	}

	if opts.Type > 0 {
		cond = cond.And(builder.Eq{"type": opts.Type})
	}

	count, err := e.Where(cond).Count(new(Project))
	if err != nil {
		return nil, 0, fmt.Errorf("Count: %v", err)
	}

	e = e.Where(cond)

	if opts.Page > 0 {
		e = e.Limit(setting.UI.IssuePagingNum, (opts.Page-1)*setting.UI.IssuePagingNum)
	}

	switch opts.SortType {
	case "oldest":
		e.Desc("created_unix")
	case "recentupdate":
		e.Desc("updated_unix")
	case "leastupdate":
		e.Asc("updated_unix")
	default:
		e.Asc("created_unix")
	}

	return projects, count, e.Find(&projects)
}

// NewProject creates a new Project
func NewProject(p *Project) error {
	if !IsProjectBoardTypeValid(p.BoardType) {
		p.BoardType = ProjectBoardTypeNone
	}

	if !IsProjectTypeValid(p.Type) {
		return errors.New("project type is not valid")
	}

	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	if _, err := sess.Insert(p); err != nil {
		return err
	}

	if _, err := sess.Exec("UPDATE "+tbRepository+" SET num_projects = num_projects + 1 WHERE id = ?", p.RepoID); err != nil {
		return err
	}

	if err := createBoardsForProjectsType(sess, p); err != nil {
		return err
	}

	return sess.Commit()
}

// GetProjectByID returns the projects in a repository
func GetProjectByID(id int64) (*Project, error) {
	return getProjectByID(x, id)
}

func getProjectByID(e Engine, id int64) (*Project, error) {
	p := new(Project)

	has, err := e.ID(id).Get(p)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrProjectNotExist{ID: id}
	}

	return p, nil
}

// UpdateProject updates project properties
func UpdateProject(p *Project) error {
	return updateProject(x, p)
}

func updateProject(e Engine, p *Project) error {
	_, err := e.ID(p.ID).Cols(
		"title",
		"description",
	).Update(p)
	return err
}

func updateRepositoryProjectCount(e Engine, repoID int64) error {
	if _, err := e.Exec(builder.Update(
		builder.Eq{
			"`num_projects`": builder.Select("count(*)").From(tbProject).
				Where(builder.Eq{tbProject + ".`repo_id`": repoID}.
					And(builder.Eq{tbProject + ".`type`": ProjectTypeRepository})),
		}).From(tbRepository).Where(builder.Eq{"id": repoID})); err != nil {
		return err
	}

	if _, err := e.Exec(builder.Update(
		builder.Eq{
			"`num_closed_projects`": builder.Select("count(*)").From(tbProject).
				Where(builder.Eq{tbProject + ".`repo_id`": repoID}.
					And(builder.Eq{tbProject + ".`type`": ProjectTypeRepository}).
					And(builder.Eq{tbProject + ".`is_closed`": true})),
		}).From(tbRepository).Where(builder.Eq{"id": repoID})); err != nil {
		return err
	}
	return nil
}

// ChangeProjectStatusByRepoIDAndID toggles a project between opened and closed
func ChangeProjectStatusByRepoIDAndID(repoID, projectID int64, isClosed bool) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	p := new(Project)

	has, err := sess.ID(projectID).Where("repo_id = ?", repoID).Get(p)
	if err != nil {
		return err
	} else if !has {
		return ErrProjectNotExist{ID: projectID, RepoID: repoID}
	}

	if err := changeProjectStatus(sess, p, isClosed); err != nil {
		return err
	}

	return sess.Commit()
}

// ChangeProjectStatus toggle a project between opened and closed
func ChangeProjectStatus(p *Project, isClosed bool) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if err := changeProjectStatus(sess, p, isClosed); err != nil {
		return err
	}

	return sess.Commit()
}

func changeProjectStatus(e Engine, p *Project, isClosed bool) error {
	p.IsClosed = isClosed
	p.ClosedDateUnix = timeutil.TimeStampNow()
	count, err := e.ID(p.ID).Where("repo_id = ? AND is_closed = ?", p.RepoID, !isClosed).Cols("is_closed", "closed_date_unix").Update(p)
	if err != nil {
		return err
	}
	if count < 1 {
		return nil
	}

	return updateRepositoryProjectCount(e, p.RepoID)
}

// DeleteProjectByID deletes a project from a repository.
func DeleteProjectByID(id int64) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if err := deleteProjectByID(sess, id); err != nil {
		return err
	}

	return sess.Commit()
}

func deleteProjectByID(e Engine, id int64) error {
	p, err := getProjectByID(e, id)
	if err != nil {
		if IsErrProjectNotExist(err) {
			return nil
		}
		return err
	}

	if err := deleteProjectIssuesByProjectID(e, id); err != nil {
		return err
	}

	if err := deleteProjectBoardByProjectID(e, id); err != nil {
		return err
	}

	if _, err = e.ID(p.ID).Delete(new(Project)); err != nil {
		return err
	}

	return updateRepositoryProjectCount(e, p.RepoID)
}
