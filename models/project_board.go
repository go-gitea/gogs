// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

type (
	// ProjectBoardType is used to represent a project board type
	ProjectBoardType uint8

	// ProjectBoardList is a list of all project boards in a repository
	ProjectBoardList []*ProjectBoard
)

const (
	// ProjectBoardTypeNone is a project board type that has no predefined columns
	ProjectBoardTypeNone ProjectBoardType = iota

	// ProjectBoardTypeBasicKanban is a project board type that has basic predefined columns
	ProjectBoardTypeBasicKanban

	// ProjectBoardTypeBugTriage is a project board type that has predefined columns suited to hunting down bugs
	ProjectBoardTypeBugTriage
)

// ProjectBoard is used to represent boards on a project
type ProjectBoard struct {
	ID      int64 `xorm:"pk autoincr"`
	Title   string
	Default bool `xorm:"NOT NULL DEFAULT false"` // issues not assigned to a specific board will be assigned to this board

	ProjectID int64 `xorm:"INDEX NOT NULL"`
	CreatorID int64 `xorm:"NOT NULL"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`

	Issues []*Issue `xorm:"-"`
}

//TableName sets the table name to `project_board`
func (b *ProjectBoard) TableName() string {
	return tbProjectBoard[1 : len(tbProjectBoard)-1]
}

// IsProjectBoardTypeValid checks if the project board type is valid
func IsProjectBoardTypeValid(p ProjectBoardType) bool {
	switch p {
	case ProjectBoardTypeNone, ProjectBoardTypeBasicKanban, ProjectBoardTypeBugTriage:
		return true
	default:
		return false
	}
}

func createBoardsForProjectsType(sess *xorm.Session, project *Project) error {

	var items []string

	switch project.BoardType {

	case ProjectBoardTypeBugTriage:
		items = setting.Project.ProjectBoardBugTriageType

	case ProjectBoardTypeBasicKanban:
		items = setting.Project.ProjectBoardBasicKanbanType

	case ProjectBoardTypeNone:
		fallthrough
	default:
		return nil
	}

	if len(items) == 0 {
		return nil
	}

	var boards = make([]ProjectBoard, 0, len(items))

	for _, v := range items {
		boards = append(boards, ProjectBoard{
			CreatedUnix: timeutil.TimeStampNow(),
			CreatorID:   project.CreatorID,
			Title:       v,
			ProjectID:   project.ID,
		})
	}

	_, err := sess.Insert(boards)
	return err
}

// NewProjectBoard adds a new project board to a given project
func NewProjectBoard(board *ProjectBoard) error {
	_, err := x.Insert(board)
	return err
}

// DeleteProjectBoardByID removes all issues references to the project board.
func DeleteProjectBoardByID(boardID int64) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	if err := deleteProjectBoardByID(sess, boardID); err != nil {
		return err
	}

	return sess.Commit()
}

func deleteProjectBoardByID(e Engine, boardID int64) error {
	board, err := getProjectBoard(e, boardID)
	if err != nil {
		if IsErrProjectBoardNotExist(err) {
			return nil
		}

		return err
	}

	if err = board.removeIssues(e); err != nil {
		return err
	}

	if _, err := e.ID(board.ID).Delete(board); err != nil {
		return err
	}
	return nil
}

func deleteProjectBoardByProjectID(e Engine, projectID int64) error {
	_, err := e.Where("project_id=?", projectID).Delete(&ProjectBoard{})
	return err
}

// GetProjectBoard fetches the current board of a project
func GetProjectBoard(boardID int64) (*ProjectBoard, error) {
	return getProjectBoard(x, boardID)
}

func getProjectBoard(e Engine, boardID int64) (*ProjectBoard, error) {
	board := new(ProjectBoard)

	has, err := e.ID(boardID).Get(board)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrProjectBoardNotExist{BoardID: boardID}
	}

	return board, nil
}

// UpdateProjectBoard updates the title of a project board
func UpdateProjectBoard(board *ProjectBoard) error {
	return updateProjectBoard(x, board)
}

func updateProjectBoard(e Engine, board *ProjectBoard) error {
	_, err := e.ID(board.ID).Cols(
		"title",
		"default",
	).Update(board)
	return err
}

// GetProjectBoards fetches all boards related to a project
func GetProjectBoards(projectID int64) ([]*ProjectBoard, error) {

	var boards = make([]*ProjectBoard, 0, 5)

	sess := x.Where("project_id=?", projectID)
	return boards, sess.Find(&boards)
}

// GetUncategorizedBoard represents a board for issues not assigned to one
func GetUncategorizedBoard(projectID int64) (*ProjectBoard, error) {
	return &ProjectBoard{
		ProjectID: projectID,
		Title:     "Uncategorized",
		Default:   true,
	}, nil
}

// LoadIssues load issues assigned to this board
func (b *ProjectBoard) LoadIssues() (IssueList, error) {
	var boardID int64
	if !b.Default {
		boardID = b.ID

	} else {
		// Issues without ProjectBoardID
		boardID = -1
	}
	issues, err := Issues(&IssuesOptions{
		ProjectBoardID: boardID,
		ProjectID:      b.ProjectID,
	})
	b.Issues = issues
	return issues, err
}

// LoadIssues load issues assigned to the boards
func (bs ProjectBoardList) LoadIssues() (IssueList, error) {
	issues := make(IssueList, 0, len(bs)*10)
	for i := range bs {
		il, err := bs[i].LoadIssues()
		if err != nil {
			return nil, err
		}
		bs[i].Issues = il
		issues = append(issues, il...)
	}
	return issues, nil
}
