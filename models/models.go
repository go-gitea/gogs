// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"code.gitea.io/gitea/modules/setting"

	// Needed for the MySQL driver
	_ "github.com/go-sql-driver/mysql"
	"xorm.io/xorm"
	"xorm.io/xorm/names"
	"xorm.io/xorm/schemas"

	// Needed for the Postgresql driver
	_ "github.com/lib/pq"

	// Needed for the MSSQL driver
	_ "github.com/denisenkom/go-mssqldb"
)

// Engine represents a xorm engine or session.
type Engine interface {
	Table(tableNameOrBean interface{}) *xorm.Session
	Count(...interface{}) (int64, error)
	Decr(column string, arg ...interface{}) *xorm.Session
	Delete(interface{}) (int64, error)
	Exec(...interface{}) (sql.Result, error)
	Find(interface{}, ...interface{}) error
	Get(interface{}) (bool, error)
	ID(interface{}) *xorm.Session
	In(string, ...interface{}) *xorm.Session
	Incr(column string, arg ...interface{}) *xorm.Session
	Insert(...interface{}) (int64, error)
	InsertOne(interface{}) (int64, error)
	Iterate(interface{}, xorm.IterFunc) error
	Join(joinOperator string, tablename interface{}, condition string, args ...interface{}) *xorm.Session
	SQL(interface{}, ...interface{}) *xorm.Session
	Where(interface{}, ...interface{}) *xorm.Session
	Asc(colNames ...string) *xorm.Session
	Desc(colNames ...string) *xorm.Session
	Limit(limit int, start ...int) *xorm.Session
	SumInt(bean interface{}, columnName string) (res int64, err error)
}

const (
	// When queries are broken down in parts because of the number
	// of parameters, attempt to break by this amount
	maxQueryParameters = 300
)

var (
	x      *xorm.Engine
	tables []interface{}

	// HasEngine specifies if we have a xorm.Engine
	HasEngine bool
)

func init() {
	tables = append(tables,
		new(User),
		new(PublicKey),
		new(AccessToken),
		new(Repository),
		new(DeployKey),
		new(Collaboration),
		new(Access),
		new(Upload),
		new(Watch),
		new(Star),
		new(Follow),
		new(Action),
		new(Issue),
		new(PullRequest),
		new(Comment),
		new(Attachment),
		new(Label),
		new(IssueLabel), //
		new(Milestone),
		new(Mirror),
		new(Release),
		new(LoginSource),
		new(Webhook),
		new(HookTask),
		new(Team),
		new(OrgUser),
		new(TeamUser),
		new(TeamRepo),
		new(Notice),
		new(EmailAddress),
		new(Notification),
		new(IssueUser),
		new(LFSMetaObject),
		new(TwoFactor),
		new(GPGKey),
		new(GPGKeyImport),
		new(RepoUnit),
		new(RepoRedirect), //
		new(ExternalLoginUser),
		new(ProtectedBranch),
		new(UserOpenID),
		new(IssueWatch),
		new(CommitStatus),
		new(Stopwatch),
		new(TrackedTime),
		new(DeletedBranch),
		new(RepoIndexerStatus),
		new(IssueDependency),
		new(LFSLock),
		new(Reaction),
		new(IssueAssignees),
		new(U2FRegistration),
		new(TeamUnit),
		new(Review),
		new(OAuth2Application),
		new(OAuth2AuthorizationCode),
		new(OAuth2Grant),
		new(Task), //
		new(LanguageStat),
		new(EmailHash),
		new(Project),
		new(ProjectBoard),
		new(ProjectIssue),
		new(RepoTopic),
		new(Topic),
	)

	gonicNames := []string{"SSL", "UID"}
	for _, name := range gonicNames {
		names.LintGonicMapper[name] = true
	}
}

// RealTableName get table name with prefix
func realTableName(tableName string) string {
	return setting.Database.TableNamePrefix + tableName
}

// Real table name list
var (
	tbUser                    = "`user`"
	tbPublicKey               = "`public_key`"
	tbAccessToken             = "`access_token`"
	tbRepository              = "`repository`"
	tbDeployKey               = "`deploy_key`"
	tbCollaboration           = "`collaboration`"
	tbAccess                  = "`access`"
	tbUpload                  = "`upload`"
	tbWatch                   = "`watch`"
	tbStar                    = "`star`"
	tbFollow                  = "`follow`"
	tbAction                  = "`action`"
	tbIssue                   = "`issue`"
	tbPullRequest             = "`pull_request`"
	tbComment                 = "`comment`"
	tbAttachment              = "`attachment`"
	tbLabel                   = "`label`"
	tbIssueLabel              = "`issue_label`"
	tbMilestone               = "`milestone`"
	tbMirror                  = "`mirror`"
	tbRelease                 = "`Release`"
	tbLoginSource             = "`login_source`"
	tbWebhook                 = "`webhook`"
	tbHookTask                = "`hook_task`"
	tbTeam                    = "`team`"
	tbOrgUser                 = "`org_user`"
	tbTeamUser                = "`team_user`"
	tbTeamRepo                = "`team_repo`"
	tbNotice                  = "`notice`"
	tbEmailAddress            = "`email_address`"
	tbNotification            = "`notification`"
	tbIssueUser               = "`issue_user`"
	tbLfsMetaObject           = "`lfs_meta_object`"
	tbTwoFactor               = "two_factor"
	tbGPGKey                  = "`gpg_key`"
	tbGPGKeyImport            = "gpg_key_import"
	tbRepoUnit                = "`repo_unit`"
	tbRepoRedirect            = "`repo_redirect`"
	tbExternalLoginUser       = "`external_login_user`"
	tbProtectedBranch         = "`protected_branch`"
	tbUserOpenID              = "`user_open_id`"
	tbIssueWatch              = "`issue_watch`"
	tbCommitStatus            = "`commit_status`"
	tbStopwatch               = "`stopwatch`"
	tbTrackedTime             = "`tracked_time`"
	tbDeletedBranch           = "`deleted_branch`"
	tbRepoIndexerStatus       = "`repo_indexer_status`"
	tbIssueDependency         = "`issue_dependency`"
	tbLFSLock                 = "`lfs_lock`"
	tbReaction                = "`reaction`"
	tbIssueAssignees          = "`issue_assignees`"
	tbU2fRegistration         = "`u2f_registration`"
	tbTeamUnit                = "`team_unit`"
	tbReview                  = "`review`"
	tbOauth2Application       = "`oauth2_application`"
	tbOauth2AuthorizationCode = "`oauth2_authorization_code`"
	tbOauth2Grant             = "`oauth2_grant`"
	tbTask                    = "`task`"
	tbLanguageStat            = "`language_stat`"
	tbEmailHash               = "`email_hash`"
	tbProject                 = "`project`"
	tbProjectBoard            = "`project_board`"
	tbProjectIssue            = "`project_issue`"
	tbRepoTopic               = "`repo_topic`"
	tbTopic                   = "`topic`"
)

// InitRealTableNameList Init the real table name list
func InitRealTableNameList() {
	tbUser = "`" + realTableName("user") + "`"
	tbPublicKey = "`" + realTableName("public_key") + "`"
	tbAccessToken = "`" + realTableName("access_token") + "`"
	tbRepository = "`" + realTableName("repository") + "`"
	tbDeployKey = "`" + realTableName("deploy_key") + "`"
	tbCollaboration = "`" + realTableName("collaboration") + "`"
	tbAccess = "`" + realTableName("access") + "`"
	tbUpload = "`" + realTableName("upload") + "`"
	tbWatch = "`" + realTableName("watch") + "`"
	tbStar = "`" + realTableName("star") + "`"
	tbFollow = "`" + realTableName("follow") + "`"
	tbAction = "`" + realTableName("action") + "`"
	tbIssue = "`" + realTableName("issue") + "`"
	tbPullRequest = "`" + realTableName("pull_request") + "`"
	tbComment = "`" + realTableName("comment") + "`"
	tbAttachment = "`" + realTableName("attachment") + "`"
	tbLabel = "`" + realTableName("label") + "`"
	tbIssueLabel = "`" + realTableName("issue_label") + "`"
	tbMilestone = "`" + realTableName("milestone") + "`"
	tbMirror = "`" + realTableName("mirror") + "`"
	tbRelease = "`" + realTableName("release") + "`"
	tbLoginSource = "`" + realTableName("login_source") + "`"
	tbWebhook = "`" + realTableName("webhook") + "`"
	tbHookTask = "`" + realTableName("hook_task") + "`"
	tbTeam = "`" + realTableName("team") + "`"
	tbOrgUser = "`" + realTableName("org_user") + "`"
	tbTeamUser = "`" + realTableName("team_user") + "`"
	tbTeamRepo = "`" + realTableName("team_repo") + "`"
	tbNotice = "`" + realTableName("notice") + "`"
	tbEmailAddress = "`" + realTableName("email_address") + "`"
	tbNotification = "`" + realTableName("notification") + "`"
	tbIssueUser = "`" + realTableName("issue_user") + "`"
	tbLfsMetaObject = "`" + realTableName("lfs_meta_object") + "`"
	tbTwoFactor = "`" + realTableName("two_factor") + "`"
	tbGPGKey = "`" + realTableName("gpg_key") + "`"
	tbGPGKeyImport = "`" + realTableName("gpg_key_import") + "`"
	tbRepoUnit = "`" + realTableName("repo_unit") + "`"
	tbRepoRedirect = "`" + realTableName("repo_redirect") + "`"
	tbExternalLoginUser = "`" + realTableName("external_login_user") + "`"
	tbProtectedBranch = "`" + realTableName("protected_branch") + "`"
	tbUserOpenID = "`" + realTableName("user_open_id") + "`"
	tbIssueWatch = "`" + realTableName("issue_watch") + "`"
	tbCommitStatus = "`" + realTableName("commit_status") + "`"
	tbStopwatch = "`" + realTableName("stopwatch") + "`"
	tbTrackedTime = "`" + realTableName("tracked_time") + "`"
	tbDeletedBranch = "`" + realTableName("deleted_branch") + "`"
	tbRepoIndexerStatus = "`" + realTableName("repo_indexer_status") + "`"
	tbIssueDependency = "`" + realTableName("issue_dependency") + "`"
	tbLFSLock = "`" + realTableName("lfs_lock") + "`"
	tbReaction = "`" + realTableName("reaction") + "`"
	tbIssueAssignees = "`" + realTableName("issue_assignees") + "`"
	tbU2fRegistration = "`" + realTableName("u2f_registration") + "`"
	tbTeamUnit = "`" + realTableName("team_unit") + "`"
	tbReview = "`" + realTableName("review") + "`"
	tbOauth2Application = "`" + realTableName("oauth2_application") + "`"
	tbOauth2AuthorizationCode = "`" + realTableName("oauth2_authorization_code") + "`"
	tbOauth2Grant = "`" + realTableName("oauth2_grant") + "`"
	tbTask = "`" + realTableName("task") + "`"
	tbLanguageStat = "`" + realTableName("language_stat") + "`"
	tbEmailHash = "`" + realTableName("email_hash") + "`"
	tbProject = "`" + realTableName("project") + "`"
	tbProjectBoard = "`" + realTableName("project_board") + "`"
	tbProjectIssue = "`" + realTableName("project_issue") + "`"
	tbRepoTopic = "`" + realTableName("repo_topic") + "`"
	tbTopic = "`" + realTableName("topic") + "`"
}

func getEngine() (*xorm.Engine, error) {
	connStr, err := setting.DBConnStr()
	if err != nil {
		return nil, err
	}

	engine, err := xorm.NewEngine(setting.Database.Type, connStr)
	if err != nil {
		return nil, err
	}
	if setting.Database.Type == "mysql" {
		engine.Dialect().SetParams(map[string]string{"rowFormat": "DYNAMIC"})
	} else if setting.Database.Type == "mssql" {
		engine.Dialect().SetParams(map[string]string{"DEFAULT_VARCHAR": "nvarchar"})
	}
	engine.SetSchema(setting.Database.Schema)
	return engine, nil
}

// NewTestEngine sets a new test xorm.Engine
func NewTestEngine() (err error) {
	x, err = getEngine()
	if err != nil {
		return fmt.Errorf("Connect to database: %v", err)
	}

	x.SetMapper(names.GonicMapper{})

	if len(setting.Database.TableNamePrefix) > 0 {
		x.SetTableMapper(names.NewPrefixMapper(x.GetTableMapper(), setting.Database.TableNamePrefix))
	}
	InitRealTableNameList()

	x.SetLogger(NewXORMLogger(!setting.ProdMode))
	x.ShowSQL(!setting.ProdMode)
	return x.StoreEngine("InnoDB").Sync2(tables...)
}

// SetEngine sets the xorm.Engine
func SetEngine() (err error) {
	x, err = getEngine()
	if err != nil {
		return fmt.Errorf("Failed to connect to database: %v", err)
	}

	x.SetMapper(names.GonicMapper{})
	if len(setting.Database.TableNamePrefix) > 0 {
		x.SetTableMapper(names.NewPrefixMapper(x.GetTableMapper(), setting.Database.TableNamePrefix))
	}
	InitRealTableNameList()
	// WARNING: for serv command, MUST remove the output to os.stdout,
	// so use log file to instead print to stdout.
	x.SetLogger(NewXORMLogger(setting.Database.LogSQL))
	x.ShowSQL(setting.Database.LogSQL)
	x.SetMaxOpenConns(setting.Database.MaxOpenConns)
	x.SetMaxIdleConns(setting.Database.MaxIdleConns)
	x.SetConnMaxLifetime(setting.Database.ConnMaxLifetime)
	return nil
}

// NewEngine initializes a new xorm.Engine
// This function must never call .Sync2() if the provided migration function fails.
// When called from the "doctor" command, the migration function is a version check
// that prevents the doctor from fixing anything in the database if the migration level
// is different from the expected value.
func NewEngine(ctx context.Context, migrateFunc func(*xorm.Engine) error) (err error) {
	if err = SetEngine(); err != nil {
		return err
	}

	x.SetDefaultContext(ctx)

	if err = x.Ping(); err != nil {
		return err
	}

	if err = migrateFunc(x); err != nil {
		return fmt.Errorf("migrate: %v", err)
	}

	if err = x.StoreEngine("InnoDB").Sync2(tables...); err != nil {
		return fmt.Errorf("sync database struct error: %v", err)
	}

	return nil
}

// NamesToBean return a list of beans or an error
func NamesToBean(names ...string) ([]interface{}, error) {
	beans := []interface{}{}
	if len(names) == 0 {
		beans = append(beans, tables...)
		return beans, nil
	}
	// Need to map provided names to beans...
	beanMap := make(map[string]interface{})
	for _, bean := range tables {

		beanMap[strings.ToLower(reflect.Indirect(reflect.ValueOf(bean)).Type().Name())] = bean
		beanMap[strings.ToLower(x.TableName(bean))] = bean
		beanMap[strings.ToLower(x.TableName(bean, true))] = bean
	}

	gotBean := make(map[interface{}]bool)
	for _, name := range names {
		bean, ok := beanMap[strings.ToLower(strings.TrimSpace(name))]
		if !ok {
			return nil, fmt.Errorf("No table found that matches: %s", name)
		}
		if !gotBean[bean] {
			beans = append(beans, bean)
			gotBean[bean] = true
		}
	}
	return beans, nil
}

// Statistic contains the database statistics
type Statistic struct {
	Counter struct {
		User, Org, PublicKey,
		Repo, Watch, Star, Action, Access,
		Issue, Comment, Oauth, Follow,
		Mirror, Release, LoginSource, Webhook,
		Milestone, Label, HookTask,
		Team, UpdateTask, Attachment int64
	}
}

// GetStatistic returns the database statistics
func GetStatistic() (stats Statistic) {
	stats.Counter.User = CountUsers()
	stats.Counter.Org = CountOrganizations()
	stats.Counter.PublicKey, _ = x.Count(new(PublicKey))
	stats.Counter.Repo = CountRepositories(true)
	stats.Counter.Watch, _ = x.Count(new(Watch))
	stats.Counter.Star, _ = x.Count(new(Star))
	stats.Counter.Action, _ = x.Count(new(Action))
	stats.Counter.Access, _ = x.Count(new(Access))
	stats.Counter.Issue, _ = x.Count(new(Issue))
	stats.Counter.Comment, _ = x.Count(new(Comment))
	stats.Counter.Oauth = 0
	stats.Counter.Follow, _ = x.Count(new(Follow))
	stats.Counter.Mirror, _ = x.Count(new(Mirror))
	stats.Counter.Release, _ = x.Count(new(Release))
	stats.Counter.LoginSource = CountLoginSources()
	stats.Counter.Webhook, _ = x.Count(new(Webhook))
	stats.Counter.Milestone, _ = x.Count(new(Milestone))
	stats.Counter.Label, _ = x.Count(new(Label))
	stats.Counter.HookTask, _ = x.Count(new(HookTask))
	stats.Counter.Team, _ = x.Count(new(Team))
	stats.Counter.Attachment, _ = x.Count(new(Attachment))
	return
}

// Ping tests if database is alive
func Ping() error {
	if x != nil {
		return x.Ping()
	}
	return errors.New("database not configured")
}

// DumpDatabase dumps all data from database according the special database SQL syntax to file system.
func DumpDatabase(filePath string, dbType string) error {
	var tbs []*schemas.Table
	for _, t := range tables {
		t, err := x.TableInfo(t)
		if err != nil {
			return err
		}
		tbs = append(tbs, t)
	}
	if len(dbType) > 0 {
		return x.DumpTablesToFile(tbs, filePath, schemas.DBType(dbType))
	}
	return x.DumpTablesToFile(tbs, filePath)
}

// MaxBatchInsertSize returns the table's max batch insert size
func MaxBatchInsertSize(bean interface{}) int {
	t, err := x.TableInfo(bean)
	if err != nil {
		return 50
	}
	return 999 / len(t.ColumnsSeq())
}

// Count returns records number according struct's fields as database query conditions
func Count(bean interface{}) (int64, error) {
	return x.Count(bean)
}

// IsTableNotEmpty returns true if table has at least one record
func IsTableNotEmpty(tableName string) (bool, error) {
	return x.Table(realTableName(tableName)).Exist()
}

// DeleteAllRecords will delete all the records of this table
func DeleteAllRecords(tableName string) error {
	_, err := x.Exec(fmt.Sprintf("DELETE FROM %s", realTableName(tableName)))
	return err
}

// GetMaxID will return max id of the table
func GetMaxID(beanOrTableName interface{}) (maxID int64, err error) {
	if v, ok := beanOrTableName.(string); ok {
		beanOrTableName = realTableName(v)
	}
	_, err = x.Select("MAX(id)").Table(beanOrTableName).Get(&maxID)
	return
}

// FindByMaxID filled results as the condition from database
func FindByMaxID(maxID int64, limit int, results interface{}) error {
	return x.Where("id <= ?", maxID).
		OrderBy("id DESC").
		Limit(limit).
		Find(results)
}
