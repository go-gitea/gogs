// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"time"

	"code.gitea.io/gitea/modules/setting"

	"xorm.io/builder"
)

// TrackedTime represents a time that was spent for a specific issue.
type TrackedTime struct {
	ID          int64     `xorm:"pk autoincr"`
	IssueID     int64     `xorm:"INDEX"`
	Issue       *Issue    `xorm:"-"`
	UserID      int64     `xorm:"INDEX"`
	User        *User     `xorm:"-"`
	Created     time.Time `xorm:"-"`
	CreatedUnix int64     `xorm:"created"`
	Time        int64     `xorm:"NOT NULL"`
	Deleted     bool      `xorm:"NOT NULL DEFAULT false"`
}

// TableName sets the table name to `tracked_time`
func (t *TrackedTime) TableName() string {
	return tbTrackedTime[1 : len(tbTrackedTime)-1]
}

// TrackedTimeList is a List of TrackedTime's
type TrackedTimeList []*TrackedTime

// AfterLoad is invoked from XORM after setting the values of all fields of this object.
func (t *TrackedTime) AfterLoad() {
	t.Created = time.Unix(t.CreatedUnix, 0).In(setting.DefaultUILocation)
}

// LoadAttributes load Issue, User
func (t *TrackedTime) LoadAttributes() (err error) {
	return t.loadAttributes(x)
}

func (t *TrackedTime) loadAttributes(e Engine) (err error) {
	if t.Issue == nil {
		t.Issue, err = getIssueByID(e, t.IssueID)
		if err != nil {
			return
		}
		err = t.Issue.loadRepo(e)
		if err != nil {
			return
		}
	}
	if t.User == nil {
		t.User, err = getUserByID(e, t.UserID)
		if err != nil {
			return
		}
	}
	return
}

// LoadAttributes load Issue, User
func (tl TrackedTimeList) LoadAttributes() (err error) {
	for _, t := range tl {
		if err = t.LoadAttributes(); err != nil {
			return err
		}
	}
	return
}

// FindTrackedTimesOptions represent the filters for tracked times. If an ID is 0 it will be ignored.
type FindTrackedTimesOptions struct {
	ListOptions
	IssueID           int64
	UserID            int64
	RepositoryID      int64
	MilestoneID       int64
	CreatedAfterUnix  int64
	CreatedBeforeUnix int64
}

// ToCond will convert each condition into a xorm-Cond
func (opts *FindTrackedTimesOptions) ToCond() builder.Cond {
	cond := builder.NewCond().And(builder.Eq{tbTrackedTime + ".deleted": false})
	if opts.IssueID != 0 {
		cond = cond.And(builder.Eq{"issue_id": opts.IssueID})
	}
	if opts.UserID != 0 {
		cond = cond.And(builder.Eq{"user_id": opts.UserID})
	}
	if opts.RepositoryID != 0 {
		cond = cond.And(builder.Eq{tbIssue + ".repo_id": opts.RepositoryID})
	}
	if opts.MilestoneID != 0 {
		cond = cond.And(builder.Eq{tbIssue + ".milestone_id": opts.MilestoneID})
	}
	if opts.CreatedAfterUnix != 0 {
		cond = cond.And(builder.Gte{tbTrackedTime + ".created_unix": opts.CreatedAfterUnix})
	}
	if opts.CreatedBeforeUnix != 0 {
		cond = cond.And(builder.Lte{tbTrackedTime + ".created_unix": opts.CreatedBeforeUnix})
	}
	return cond
}

// ToSession will convert the given options to a xorm Session by using the conditions from ToCond and joining with issue table if required
func (opts *FindTrackedTimesOptions) ToSession(e Engine) Engine {
	sess := e
	if opts.RepositoryID > 0 || opts.MilestoneID > 0 {
		sess = e.Join("INNER", tbIssue, tbIssue+".id = "+tbTrackedTime+".issue_id")
	}

	sess = sess.Where(opts.ToCond())

	if opts.Page != 0 {
		sess = opts.setEnginePagination(sess)
	}

	return sess
}

func getTrackedTimes(e Engine, options FindTrackedTimesOptions) (trackedTimes TrackedTimeList, err error) {
	err = options.ToSession(e).Find(&trackedTimes)
	return
}

// GetTrackedTimes returns all tracked times that fit to the given options.
func GetTrackedTimes(opts FindTrackedTimesOptions) (TrackedTimeList, error) {
	return getTrackedTimes(x, opts)
}

func getTrackedSeconds(e Engine, opts FindTrackedTimesOptions) (trackedSeconds int64, err error) {
	return opts.ToSession(e).SumInt(&TrackedTime{}, "time")
}

// GetTrackedSeconds return sum of seconds
func GetTrackedSeconds(opts FindTrackedTimesOptions) (int64, error) {
	return getTrackedSeconds(x, opts)
}

// AddTime will add the given time (in seconds) to the issue
func AddTime(user *User, issue *Issue, amount int64, created time.Time) (*TrackedTime, error) {
	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return nil, err
	}

	t, err := addTime(sess, user, issue, amount, created)
	if err != nil {
		return nil, err
	}

	if err := issue.loadRepo(sess); err != nil {
		return nil, err
	}

	if _, err := createComment(sess, &CreateCommentOptions{
		Issue:   issue,
		Repo:    issue.Repo,
		Doer:    user,
		Content: SecToTime(amount),
		Type:    CommentTypeAddTimeManual,
	}); err != nil {
		return nil, err
	}

	return t, sess.Commit()
}

func addTime(e Engine, user *User, issue *Issue, amount int64, created time.Time) (*TrackedTime, error) {
	if created.IsZero() {
		created = time.Now()
	}
	tt := &TrackedTime{
		IssueID: issue.ID,
		UserID:  user.ID,
		Time:    amount,
		Created: created,
	}
	if _, err := e.Insert(tt); err != nil {
		return nil, err
	}

	return tt, nil
}

// TotalTimes returns the spent time for each user by an issue
func TotalTimes(options FindTrackedTimesOptions) (map[*User]string, error) {
	trackedTimes, err := GetTrackedTimes(options)
	if err != nil {
		return nil, err
	}
	//Adding total time per user ID
	totalTimesByUser := make(map[int64]int64)
	for _, t := range trackedTimes {
		totalTimesByUser[t.UserID] += t.Time
	}

	totalTimes := make(map[*User]string)
	//Fetching User and making time human readable
	for userID, total := range totalTimesByUser {
		user, err := GetUserByID(userID)
		if err != nil {
			if IsErrUserNotExist(err) {
				continue
			}
			return nil, err
		}
		totalTimes[user] = SecToTime(total)
	}
	return totalTimes, nil
}

// DeleteIssueUserTimes deletes times for issue
func DeleteIssueUserTimes(issue *Issue, user *User) error {
	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	opts := FindTrackedTimesOptions{
		IssueID: issue.ID,
		UserID:  user.ID,
	}

	removedTime, err := deleteTimes(sess, opts)
	if err != nil {
		return err
	}
	if removedTime == 0 {
		return ErrNotExist{}
	}

	if err := issue.loadRepo(sess); err != nil {
		return err
	}
	if _, err := createComment(sess, &CreateCommentOptions{
		Issue:   issue,
		Repo:    issue.Repo,
		Doer:    user,
		Content: "- " + SecToTime(removedTime),
		Type:    CommentTypeDeleteTimeManual,
	}); err != nil {
		return err
	}

	return sess.Commit()
}

// DeleteTime delete a specific Time
func DeleteTime(t *TrackedTime) error {
	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	if err := t.loadAttributes(sess); err != nil {
		return err
	}

	if err := deleteTime(sess, t); err != nil {
		return err
	}

	if _, err := createComment(sess, &CreateCommentOptions{
		Issue:   t.Issue,
		Repo:    t.Issue.Repo,
		Doer:    t.User,
		Content: "- " + SecToTime(t.Time),
		Type:    CommentTypeDeleteTimeManual,
	}); err != nil {
		return err
	}

	return sess.Commit()
}

func deleteTimes(e Engine, opts FindTrackedTimesOptions) (removedTime int64, err error) {

	removedTime, err = getTrackedSeconds(e, opts)
	if err != nil || removedTime == 0 {
		return
	}

	_, err = opts.ToSession(e).Table(tbTrackedTime).Cols("deleted").Update(&TrackedTime{Deleted: true})
	return
}

func deleteTime(e Engine, t *TrackedTime) error {
	if t.Deleted {
		return ErrNotExist{ID: t.ID}
	}
	t.Deleted = true
	_, err := e.ID(t.ID).Cols("deleted").Update(t)
	return err
}

// GetTrackedTimeByID returns raw TrackedTime without loading attributes by id
func GetTrackedTimeByID(id int64) (*TrackedTime, error) {
	time := new(TrackedTime)
	has, err := x.ID(id).Get(time)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrNotExist{ID: id}
	}
	return time, nil
}
