// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"xorm.io/builder"
)

// consistencyCheckable a type that can be tested for database consistency
type consistencyCheckable interface {
	checkForConsistency(t *testing.T)
}

// CheckConsistencyForAll test that the entire database is consistent
func CheckConsistencyForAll(t *testing.T) {
	CheckConsistencyFor(t,
		&User{},
		&Repository{},
		&Issue{},
		&PullRequest{},
		&Milestone{},
		&Label{},
		&Team{},
		&Action{})
}

// CheckConsistencyFor test that all matching database entries are consistent
func CheckConsistencyFor(t *testing.T, beansToCheck ...interface{}) {
	for _, bean := range beansToCheck {
		sliceType := reflect.SliceOf(reflect.TypeOf(bean))
		sliceValue := reflect.MakeSlice(sliceType, 0, 10)

		ptrToSliceValue := reflect.New(sliceType)
		ptrToSliceValue.Elem().Set(sliceValue)

		assert.NoError(t, x.Table(bean).Find(ptrToSliceValue.Interface()))
		sliceValue = ptrToSliceValue.Elem()

		for i := 0; i < sliceValue.Len(); i++ {
			entity := sliceValue.Index(i).Interface()
			checkable, ok := entity.(consistencyCheckable)
			if !ok {
				t.Errorf("Expected %+v (of type %T) to be checkable for consistency",
					entity, entity)
			} else {
				checkable.checkForConsistency(t)
			}
		}
	}
}

// getCount get the count of database entries matching bean
func getCount(t *testing.T, e Engine, bean interface{}) int64 {
	count, err := e.Count(bean)
	assert.NoError(t, err)
	return count
}

// assertCount test the count of database entries matching bean
func assertCount(t *testing.T, bean interface{}, expected int) {
	assert.EqualValues(t, expected, getCount(t, x, bean),
		"Failed consistency test, the counted bean (of type %T) was %+v", bean, bean)
}

func (user *User) checkForConsistency(t *testing.T) {
	assertCount(t, &Repository{OwnerID: user.ID}, user.NumRepos)
	assertCount(t, &Star{UID: user.ID}, user.NumStars)
	assertCount(t, &OrgUser{OrgID: user.ID}, user.NumMembers)
	assertCount(t, &Team{OrgID: user.ID}, user.NumTeams)
	assertCount(t, &Follow{UserID: user.ID}, user.NumFollowing)
	assertCount(t, &Follow{FollowID: user.ID}, user.NumFollowers)
	if user.Type != UserTypeOrganization {
		assert.EqualValues(t, 0, user.NumMembers)
		assert.EqualValues(t, 0, user.NumTeams)
	}
}

func (repo *Repository) checkForConsistency(t *testing.T) {
	assert.Equal(t, repo.LowerName, strings.ToLower(repo.Name), "repo: %+v", repo)
	assertCount(t, &Star{RepoID: repo.ID}, repo.NumStars)
	assertCount(t, &Milestone{RepoID: repo.ID}, repo.NumMilestones)
	assertCount(t, &Repository{ForkID: repo.ID}, repo.NumForks)
	if repo.IsFork {
		AssertExistsAndLoadBean(t, &Repository{ID: repo.ForkID})
	}

	actual := getCount(t, x.Where("Mode<>?", RepoWatchModeDont), &Watch{RepoID: repo.ID})
	assert.EqualValues(t, repo.NumWatches, actual,
		"Unexpected number of watches for repo %+v", repo)

	actual = getCount(t, x.Where("is_pull=?", false), &Issue{RepoID: repo.ID})
	assert.EqualValues(t, repo.NumIssues, actual,
		"Unexpected number of issues for repo %+v", repo)

	actual = getCount(t, x.Where("is_pull=? AND is_closed=?", false, true), &Issue{RepoID: repo.ID})
	assert.EqualValues(t, repo.NumClosedIssues, actual,
		"Unexpected number of closed issues for repo %+v", repo)

	actual = getCount(t, x.Where("is_pull=?", true), &Issue{RepoID: repo.ID})
	assert.EqualValues(t, repo.NumPulls, actual,
		"Unexpected number of pulls for repo %+v", repo)

	actual = getCount(t, x.Where("is_pull=? AND is_closed=?", true, true), &Issue{RepoID: repo.ID})
	assert.EqualValues(t, repo.NumClosedPulls, actual,
		"Unexpected number of closed pulls for repo %+v", repo)

	actual = getCount(t, x.Where("is_closed=?", true), &Milestone{RepoID: repo.ID})
	assert.EqualValues(t, repo.NumClosedMilestones, actual,
		"Unexpected number of closed milestones for repo %+v", repo)
}

func (issue *Issue) checkForConsistency(t *testing.T) {
	actual := getCount(t, x.Where("type=?", CommentTypeComment), &Comment{IssueID: issue.ID})
	assert.EqualValues(t, issue.NumComments, actual,
		"Unexpected number of comments for issue %+v", issue)
	if issue.IsPull {
		pr := AssertExistsAndLoadBean(t, &PullRequest{IssueID: issue.ID}).(*PullRequest)
		assert.EqualValues(t, pr.Index, issue.Index)
	}
}

func (pr *PullRequest) checkForConsistency(t *testing.T) {
	issue := AssertExistsAndLoadBean(t, &Issue{ID: pr.IssueID}).(*Issue)
	assert.True(t, issue.IsPull)
	assert.EqualValues(t, issue.Index, pr.Index)
}

func (milestone *Milestone) checkForConsistency(t *testing.T) {
	assertCount(t, &Issue{MilestoneID: milestone.ID}, milestone.NumIssues)

	actual := getCount(t, x.Where("is_closed=?", true), &Issue{MilestoneID: milestone.ID})
	assert.EqualValues(t, milestone.NumClosedIssues, actual,
		"Unexpected number of closed issues for milestone %+v", milestone)
}

func (label *Label) checkForConsistency(t *testing.T) {
	issueLabels := make([]*IssueLabel, 0, 10)
	assert.NoError(t, x.Find(&issueLabels, &IssueLabel{LabelID: label.ID}))
	assert.EqualValues(t, label.NumIssues, len(issueLabels),
		"Unexpected number of issue for label %+v", label)

	issueIDs := make([]int64, len(issueLabels))
	for i, issueLabel := range issueLabels {
		issueIDs[i] = issueLabel.IssueID
	}

	expected := int64(0)
	if len(issueIDs) > 0 {
		expected = getCount(t, x.In("id", issueIDs).Where("is_closed=?", true), &Issue{})
	}
	assert.EqualValues(t, expected, label.NumClosedIssues,
		"Unexpected number of closed issues for label %+v", label)
}

func (team *Team) checkForConsistency(t *testing.T) {
	assertCount(t, &TeamUser{TeamID: team.ID}, team.NumMembers)
	assertCount(t, &TeamRepo{TeamID: team.ID}, team.NumRepos)
}

func (action *Action) checkForConsistency(t *testing.T) {
	repo := AssertExistsAndLoadBean(t, &Repository{ID: action.RepoID}).(*Repository)
	assert.Equal(t, repo.IsPrivate, action.IsPrivate, "action: %+v", action)
}

// CountOrphanedLabels return count of labels witch are broken and not accessible via ui anymore
func CountOrphanedLabels() (int64, error) {
	noref, err := x.Table(tbLabel).Where("repo_id=? AND org_id=?", 0, 0).Count(tbLabel + ".id")
	if err != nil {
		return 0, err
	}

	norepo, err := x.Table(tbLabel).
		Join("LEFT", tbRepository, tbLabel+".repo_id="+tbRepository+".id").
		Where(builder.IsNull{tbRepository + ".id"}).And(builder.Gt{tbLabel + ".repo_id": 0}).
		Count("id")
	if err != nil {
		return 0, err
	}

	noorg, err := x.Table(tbLabel).
		Join("LEFT", tbUser, tbLabel+".org_id="+tbUser+".id").
		Where(builder.IsNull{tbUser + ".id"}).And(builder.Gt{tbLabel + ".org_id": 0}).
		Count("id")
	if err != nil {
		return 0, err
	}

	return noref + norepo + noorg, nil
}

// DeleteOrphanedLabels delete labels witch are broken and not accessible via ui anymore
func DeleteOrphanedLabels() error {
	// delete labels with no reference
	if _, err := x.Table(tbLabel).Where("repo_id=? AND org_id=?", 0, 0).Delete(new(Label)); err != nil {
		return err
	}

	// delete labels with none existing repos
	if _, err := x.In("id", builder.Select(tbLabel+".id").From(tbLabel).
		Join("LEFT", tbRepository, tbLabel+".repo_id="+tbRepository+".id").
		Where(builder.IsNull{tbRepository + "id"}).And(builder.Gt{tbLabel + ".repo_id": 0})).
		Delete(Label{}); err != nil {
		return err
	}

	// delete labels with none existing orgs
	if _, err := x.In("id", builder.Select(tbLabel+".id").From(tbLabel).
		Join("LEFT", tbUser, tbLabel+".org_id="+tbUser+".id").
		Where(builder.IsNull{tbUser + ".id"}).And(builder.Gt{tbLabel + ".org_id": 0})).
		Delete(Label{}); err != nil {
		return err
	}

	return nil
}

// CountOrphanedIssues count issues without a repo
func CountOrphanedIssues() (int64, error) {
	return x.Table(tbIssue).
		Join("LEFT", tbRepository, tbIssue+".repo_id="+tbRepository+".id").
		Where(builder.IsNull{tbRepository + ".id"}).
		Count("id")
}

// DeleteOrphanedIssues delete issues without a repo
func DeleteOrphanedIssues() error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	var ids []int64

	if err := sess.Table(tbIssue).Distinct(tbIssue+".repo_id").
		Join("LEFT", tbRepository, tbIssue+".repo_id="+tbRepository+".id").
		Where(builder.IsNull{tbRepository + ".id"}).GroupBy(tbIssue + ".repo_id").
		Find(&ids); err != nil {
		return err
	}

	var attachmentPaths []string
	for i := range ids {
		paths, err := deleteIssuesByRepoID(sess, ids[i])
		if err != nil {
			return err
		}
		attachmentPaths = append(attachmentPaths, paths...)
	}

	if err := sess.Commit(); err != nil {
		return err
	}

	// Remove issue attachment files.
	for i := range attachmentPaths {
		removeAllWithNotice(x, "Delete issue attachment", attachmentPaths[i])
	}
	return nil
}

// countOrphanedObjects count subjects with have no existing refobject anymore
func countOrphanedObjects(subject, refobject, joinCond string) (int64, error) {
	return x.Table("`"+subject+"`").
		Join("LEFT", refobject, joinCond).
		Where(builder.IsNull{"`" + refobject + "`.id"}).
		Count("id")
}

// CountOrphanedTrackedTime count tracked_time without existing issues/pulls
func CountOrphanedTrackedTime() (int64, error) {
	return countOrphanedObjects(tbTrackedTime, tbIssue, tbTrackedTime+".issue_id="+tbIssue+".id")
}

// CountPullsWithoutExistingIssues count pulls without existing issues
func CountPullsWithoutExistingIssues() (int64, error) {
	return countOrphanedObjects(tbPullRequest, tbIssue, tbPullRequest+".issue_id="+tbIssue+".id")
}

// deleteOrphanedObjects delete subjects with have no existing refobject anymore
func deleteOrphanedObjects(subject, refobject, joinCond string) error {
	_, err := x.In("id", builder.Select("`"+subject+"`.id").
		From("`"+subject+"`").
		Join("LEFT", "`"+refobject+"`", joinCond).
		Where(builder.IsNull{"`" + refobject + "`.id"})).
		Delete("`" + subject + "`")
	return err
}

// DeleteOrphanedTrackedTime delete tracked_time without existing issues/pulls
func DeleteOrphanedTrackedTime() error {
	return deleteOrphanedObjects(tbTrackedTime, tbIssue, tbTrackedTime+".issue_id="+tbIssue+".id")
}

// DeletePullsWithoutExistingIssues delete pulls without existing issues
func DeletePullsWithoutExistingIssues() error {
	return deleteOrphanedObjects(tbPullRequest, tbIssue, tbPullRequest+".issue_id="+tbIssue+".id")
}

// CountNullArchivedRepository counts the number of repositories with is_archived is null
func CountNullArchivedRepository() (int64, error) {
	return x.Where(builder.IsNull{"is_archived"}).Count(new(Repository))
}

// FixNullArchivedRepository sets is_archived to false where it is null
func FixNullArchivedRepository() (int64, error) {
	return x.Where(builder.IsNull{"is_archived"}).Cols("is_archived").Update(&Repository{
		IsArchived: false,
	})
}
