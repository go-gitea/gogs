// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
	"xorm.io/xorm/names"
	"xorm.io/xorm/schemas"
)

const minDBVersion = 70 // Gitea 1.5.3

// Migration describes on migration from lower version to high version
type Migration interface {
	Description() string
	Migrate(*xorm.Engine) error
}

type migration struct {
	description string
	migrate     func(*xorm.Engine) error
}

// NewMigration creates a new migration
func NewMigration(desc string, fn func(*xorm.Engine) error) Migration {
	return &migration{desc, fn}
}

// Description returns the migration's description
func (m *migration) Description() string {
	return m.description
}

// Migrate executes the migration
func (m *migration) Migrate(x *xorm.Engine) error {
	return m.migrate(x)
}

// Version describes the version table. Should have only one row with id==1
type Version struct {
	ID      int64 `xorm:"pk autoincr"`
	Version int64
}

// This is a sequence of migrations. Add new migrations to the bottom of the list.
// If you want to "retire" a migration, remove it from the top of the list and
// update minDBVersion accordingly
var migrations = []Migration{

	// Gitea 1.5.0 ends at v69

	// v70 -> v71
	NewMigration("add issue_dependencies", addIssueDependencies),
	// v71 -> v72
	NewMigration("protect each scratch token", addScratchHash),
	// v72 -> v73
	NewMigration("add review", addReview),

	// Gitea 1.6.0 ends at v73

	// v73 -> v74
	NewMigration("add must_change_password column for users table", addMustChangePassword),
	// v74 -> v75
	NewMigration("add approval whitelists to protected branches", addApprovalWhitelistsToProtectedBranches),
	// v75 -> v76
	NewMigration("clear nonused data which not deleted when user was deleted", clearNonusedData),

	// Gitea 1.7.0 ends at v76

	// v76 -> v77
	NewMigration("add pull request rebase with merge commit", addPullRequestRebaseWithMerge),
	// v77 -> v78
	NewMigration("add theme to users", addUserDefaultTheme),
	// v78 -> v79
	NewMigration("rename repo is_bare to repo is_empty", renameRepoIsBareToIsEmpty),
	// v79 -> v80
	NewMigration("add can close issues via commit in any branch", addCanCloseIssuesViaCommitInAnyBranch),
	// v80 -> v81
	NewMigration("add is locked to issues", addIsLockedToIssues),
	// v81 -> v82
	NewMigration("update U2F counter type", changeU2FCounterType),

	// Gitea 1.8.0 ends at v82

	// v82 -> v83
	NewMigration("hot fix for wrong release sha1 on release table", fixReleaseSha1OnReleaseTable),
	// v83 -> v84
	NewMigration("add uploader id for table attachment", addUploaderIDForAttachment),
	// v84 -> v85
	NewMigration("add table to store original imported gpg keys", addGPGKeyImport),
	// v85 -> v86
	NewMigration("hash application token", hashAppToken),
	// v86 -> v87
	NewMigration("add http method to webhook", addHTTPMethodToWebhook),
	// v87 -> v88
	NewMigration("add avatar field to repository", addAvatarFieldToRepository),

	// Gitea 1.9.0 ends at v88

	// v88 -> v89
	NewMigration("add commit status context field to commit_status", addCommitStatusContext),
	// v89 -> v90
	NewMigration("add original author/url migration info to issues, comments, and repo ", addOriginalMigrationInfo),
	// v90 -> v91
	NewMigration("change length of some repository columns", changeSomeColumnsLengthOfRepo),
	// v91 -> v92
	NewMigration("add index on owner_id of repository and type, review_id of comment", addIndexOnRepositoryAndComment),
	// v92 -> v93
	NewMigration("remove orphaned repository index statuses", removeLingeringIndexStatus),
	// v93 -> v94
	NewMigration("add email notification enabled preference to user", addEmailNotificationEnabledToUser),
	// v94 -> v95
	NewMigration("add enable_status_check, status_check_contexts to protected_branch", addStatusCheckColumnsForProtectedBranches),
	// v95 -> v96
	NewMigration("add table columns for cross referencing issues", addCrossReferenceColumns),
	// v96 -> v97
	NewMigration("delete orphaned attachments", deleteOrphanedAttachments),
	// v97 -> v98
	NewMigration("add repo_admin_change_team_access to user", addRepoAdminChangeTeamAccessColumnForUser),
	// v98 -> v99
	NewMigration("add original author name and id on migrated release", addOriginalAuthorOnMigratedReleases),
	// v99 -> v100
	NewMigration("add task table and status column for repository table", addTaskTable),
	// v100 -> v101
	NewMigration("update migration repositories' service type", updateMigrationServiceTypes),
	// v101 -> v102
	NewMigration("change length of some external login users columns", changeSomeColumnsLengthOfExternalLoginUser),

	// Gitea 1.10.0 ends at v102

	// v102 -> v103
	NewMigration("update migration repositories' service type", dropColumnHeadUserNameOnPullRequest),
	// v103 -> v104
	NewMigration("Add WhitelistDeployKeys to protected branch", addWhitelistDeployKeysToBranches),
	// v104 -> v105
	NewMigration("remove unnecessary columns from label", removeLabelUneededCols),
	// v105 -> v106
	NewMigration("add includes_all_repositories to teams", addTeamIncludesAllRepositories),
	// v106 -> v107
	NewMigration("add column `mode` to table watch", addModeColumnToWatch),
	// v107 -> v108
	NewMigration("Add template options to repository", addTemplateToRepo),
	// v108 -> v109
	NewMigration("Add comment_id on table notification", addCommentIDOnNotification),
	// v109 -> v110
	NewMigration("add can_create_org_repo to team", addCanCreateOrgRepoColumnForTeam),
	// v110 -> v111
	NewMigration("change review content type to text", changeReviewContentToText),
	// v111 -> v112
	NewMigration("update branch protection for can push and whitelist enable", addBranchProtectionCanPushAndEnableWhitelist),
	// v112 -> v113
	NewMigration("remove release attachments which repository deleted", removeAttachmentMissedRepo),
	// v113 -> v114
	NewMigration("new feature: change target branch of pull requests", featureChangeTargetBranch),
	// v114 -> v115
	NewMigration("Remove authentication credentials from stored URL", sanitizeOriginalURL),
	// v115 -> v116
	NewMigration("add user_id prefix to existing user avatar name", renameExistingUserAvatarName),
	// v116 -> v117
	NewMigration("Extend TrackedTimes", extendTrackedTimes),

	// Gitea 1.11.0 ends at v117

	// v117 -> v118
	NewMigration("Add block on rejected reviews branch protection", addBlockOnRejectedReviews),
	// v118 -> v119
	NewMigration("Add commit id and stale to reviews", addReviewCommitAndStale),
	// v119 -> v120
	NewMigration("Fix migrated repositories' git service type", fixMigratedRepositoryServiceType),
	// v120 -> v121
	NewMigration("Add owner_name on table repository", addOwnerNameOnRepository),
	// v121 -> v122
	NewMigration("add is_restricted column for users table", addIsRestricted),
	// v122 -> v123
	NewMigration("Add Require Signed Commits to ProtectedBranch", addRequireSignedCommits),
	// v123 -> v124
	NewMigration("Add original information for reactions", addReactionOriginals),
	// v124 -> v125
	NewMigration("Add columns to user and repository", addUserRepoMissingColumns),
	// v125 -> v126
	NewMigration("Add some columns on review for migration", addReviewMigrateInfo),
	// v126 -> v127
	NewMigration("Fix topic repository count", fixTopicRepositoryCount),
	// v127 -> v128
	NewMigration("add repository code language statistics", addLanguageStats),
	// v128 -> v129
	NewMigration("fix merge base for pull requests", fixMergeBase),
	// v129 -> v130
	NewMigration("remove dependencies from deleted repositories", purgeUnusedDependencies),
	// v130 -> v131
	NewMigration("Expand webhooks for more granularity", expandWebhooks),
	// v131 -> v132
	NewMigration("Add IsSystemWebhook column to webhooks table", addSystemWebhookColumn),
	// v132 -> v133
	NewMigration("Add Branch Protection Protected Files Column", addBranchProtectionProtectedFilesColumn),
	// v133 -> v134
	NewMigration("Add EmailHash Table", addEmailHashTable),
	// v134 -> v135
	NewMigration("Refix merge base for merged pull requests", refixMergeBase),
	// v135 -> v136
	NewMigration("Add OrgID column to Labels table", addOrgIDLabelColumn),
	// v136 -> v137
	NewMigration("Add CommitsAhead and CommitsBehind Column to PullRequest Table", addCommitDivergenceToPulls),
	// v137 -> v138
	NewMigration("Add Branch Protection Block Outdated Branch", addBlockOnOutdatedBranch),
	// v138 -> v139
	NewMigration("Add ResolveDoerID to Comment table", addResolveDoerIDCommentColumn),
	// v139 -> v140
	NewMigration("prepend refs/heads/ to issue refs", prependRefsHeadsToIssueRefs),

	// Gitea 1.12.0 ends at v140

	// v140 -> v141
	NewMigration("Save detected language file size to database instead of percent", fixLanguageStatsToSaveSize),
	// v141 -> v142
	NewMigration("Add KeepActivityPrivate to User table", addKeepActivityPrivateUserColumn),
	// v142 -> v143
	NewMigration("Ensure Repository.IsArchived is not null", setIsArchivedToFalse),
	// v143 -> v144
	NewMigration("recalculate Stars number for all user", recalculateStars),
	// v144 -> v145
	NewMigration("update Matrix Webhook http method to 'PUT'", updateMatrixWebhookHTTPMethod),
	// v145 -> v146
	NewMigration("Increase Language field to 50 in LanguageStats", increaseLanguageField),
	// v146 -> v147
	NewMigration("Add projects info to repository table", addProjectsInfo),
	// v147 -> v148
	NewMigration("create review for 0 review id code comments", createReviewsForCodeComments),
	// v148 -> v149
	NewMigration("remove issue dependency comments who refer to non existing issues", purgeInvalidDependenciesComments),
	// v149 -> v150
	NewMigration("Add Created and Updated to Milestone table", addCreatedAndUpdatedToMilestones),
	// v150 -> v151
	NewMigration("add primary key to repo_topic", addPrimaryKeyToRepoTopic),
	// v151 -> v152
	NewMigration("set default password algorithm to Argon2", setDefaultPasswordToArgon2),
	// v152 -> v153
	NewMigration("add TrustModel field to Repository", addTrustModelToRepository),
	// v153 > v154
	NewMigration("add Team review request support", addTeamReviewRequestSupport),
	// v154 > v155
	NewMigration("add timestamps to Star, Label, Follow, Watch and Collaboration", addTimeStamps),

	// Gitea 1.13.0 ends at v155

	// v155 -> v156
	NewMigration("add changed_protected_files column for pull_request table", addChangedProtectedFilesPullRequestColumn),
	// v156 -> v157
	NewMigration("fix publisher ID for tag releases", fixPublisherIDforTagReleases),
	// v157 -> v158
	NewMigration("ensure repo topics are up-to-date", fixRepoTopics),
	// v158 -> v159
	NewMigration("code comment replies should have the commitID of the review they are replying to", updateCodeCommentReplies),
	// v159 -> v160
	NewMigration("update reactions constraint", updateReactionConstraint),
	// v160 -> v161
	NewMigration("Add block on official review requests branch protection", addBlockOnOfficialReviewRequests),
	// v161 -> v162
	NewMigration("Convert task type from int to string", convertTaskTypeToString),
	// v162 -> v163
	NewMigration("Convert webhook task type from int to string", convertWebhookTaskTypeToString),
	// v163 -> v164
	NewMigration("Convert topic name from 25 to 50", convertTopicNameFrom25To50),
	// v164 -> v165
	NewMigration("Add scope and nonce columns to oauth2_grant table", addScopeAndNonceColumnsToOAuth2Grant),
	// v165 -> v166
	NewMigration("Convert hook task type from char(16) to varchar(16) and trim the column", convertHookTaskTypeToVarcharAndTrim),
	// v166 -> v167
	NewMigration("Where Password is Valid with Empty String delete it", recalculateUserEmptyPWD),
	// v167 -> v168
	NewMigration("Add user redirect", addUserRedirect),
	// v168 -> v169
	NewMigration("Recreate user table to fix default values", recreateUserTableToFixDefaultValues),
	// v169 -> v170
	NewMigration("Update DeleteBranch comments to set the old_ref to the commit_sha", commentTypeDeleteBranchUseOldRef),
	// v170 -> v171
	NewMigration("Add Dismissed to Review table", addDismissedReviewColumn),
	// v171 -> v172
	NewMigration("Add Sorting to ProjectBoard table", addSortingColToProjectBoard),
	// v172 -> v173
	NewMigration("Add sessions table for go-chi/session", addSessionTable),
	// v173 -> v174
	NewMigration("Add time_id column to Comment", addTimeIDCommentColumn),
	// v174 -> v175
	NewMigration("Create repo transfer table", addRepoTransfer),
	// v175 -> v176
	NewMigration("Fix Postgres ID Sequences broken by recreate-table", fixPostgresIDSequences),
	// v176 -> v177
	NewMigration("Remove invalid labels from comments", removeInvalidLabels),
	// v177 -> v178
	NewMigration("Delete orphaned IssueLabels", deleteOrphanedIssueLabels),

	// Gitea 1.14.0 ends at v178

	// v178 -> v179
	NewMigration("Add LFS columns to Mirror", addLFSMirrorColumns),
	// v179 -> v180
	NewMigration("Convert avatar url to text", convertAvatarURLToText),
	// v180 -> v181
	NewMigration("Delete credentials from past migrations", deleteMigrationCredentials),
	// v181 -> v182
	NewMigration("Always save primary email on email address table", addPrimaryEmail2EmailAddress),
	// v182 -> v183
	NewMigration("Add issue resource index table", addIssueResourceIndexTable),
	// v183 -> v184
	NewMigration("Create PushMirror table", createPushMirrorTable),
	// v184 -> v185
	NewMigration("Rename Task errors to message", renameTaskErrorsToMessage),
	// v185 -> v186
	NewMigration("Add new table repo_archiver", addRepoArchiver),
	// v186 -> v187
	NewMigration("Create protected tag table", createProtectedTagTable),
	// v187 -> v188
	NewMigration("Drop unneeded webhook related columns", dropWebhookColumns),
	// v188 -> v189
	NewMigration("Add key is verified to gpg key", addKeyIsVerified),
	// v189 -> v190
	NewMigration("Unwrap ldap.Sources", unwrapLDAPSourceCfg),
}

// GetCurrentDBVersion returns the current db version
func GetCurrentDBVersion(x *xorm.Engine) (int64, error) {
	if err := x.Sync(new(Version)); err != nil {
		return -1, fmt.Errorf("sync: %v", err)
	}

	currentVersion := &Version{ID: 1}
	has, err := x.Get(currentVersion)
	if err != nil {
		return -1, fmt.Errorf("get: %v", err)
	}
	if !has {
		return -1, nil
	}
	return currentVersion.Version, nil
}

// ExpectedVersion returns the expected db version
func ExpectedVersion() int64 {
	return int64(minDBVersion + len(migrations))
}

// EnsureUpToDate will check if the db is at the correct version
func EnsureUpToDate(x *xorm.Engine) error {
	currentDB, err := GetCurrentDBVersion(x)
	if err != nil {
		return err
	}

	if currentDB < 0 {
		return fmt.Errorf("Database has not been initialised")
	}

	if minDBVersion > currentDB {
		return fmt.Errorf("DB version %d (<= %d) is too old for auto-migration. Upgrade to Gitea 1.6.4 first then upgrade to this version", currentDB, minDBVersion)
	}

	expected := ExpectedVersion()

	if currentDB != expected {
		return fmt.Errorf(`Current database version %d is not equal to the expected version %d. Please run "gitea [--config /path/to/app.ini] migrate" to update the database version`, currentDB, expected)
	}

	return nil
}

// Migrate database to current version
func Migrate(x *xorm.Engine) error {
	// Set a new clean the default mapper to GonicMapper as that is the default for Gitea.
	x.SetMapper(names.GonicMapper{})
	if err := x.Sync(new(Version)); err != nil {
		return fmt.Errorf("sync: %v", err)
	}

	currentVersion := &Version{ID: 1}
	has, err := x.Get(currentVersion)
	if err != nil {
		return fmt.Errorf("get: %v", err)
	} else if !has {
		// If the version record does not exist we think
		// it is a fresh installation and we can skip all migrations.
		currentVersion.ID = 0
		currentVersion.Version = int64(minDBVersion + len(migrations))

		if _, err = x.InsertOne(currentVersion); err != nil {
			return fmt.Errorf("insert: %v", err)
		}
	}

	v := currentVersion.Version
	if minDBVersion > v {
		log.Fatal(`Gitea no longer supports auto-migration from your previously installed version.
Please try upgrading to a lower version first (suggested v1.6.4), then upgrade to this version.`)
		return nil
	}

	// Downgrading Gitea's database version not supported
	if int(v-minDBVersion) > len(migrations) {
		msg := fmt.Sprintf("Downgrading database version from '%d' to '%d' is not supported and may result in loss of data integrity.\nIf you really know what you're doing, execute `UPDATE version SET version=%d WHERE id=1;`\n",
			v, minDBVersion+len(migrations), minDBVersion+len(migrations))
		fmt.Fprint(os.Stderr, msg)
		log.Fatal(msg)
		return nil
	}

	// Migrate
	for i, m := range migrations[v-minDBVersion:] {
		log.Info("Migration[%d]: %s", v+int64(i), m.Description())
		// Reset the mapper between each migration - migrations are not supposed to depend on each other
		x.SetMapper(names.GonicMapper{})
		if err = m.Migrate(x); err != nil {
			return fmt.Errorf("do migrate: %v", err)
		}
		currentVersion.Version = v + int64(i) + 1
		if _, err = x.ID(1).Update(currentVersion); err != nil {
			return err
		}
	}
	return nil
}

// RecreateTables will recreate the tables for the provided beans using the newly provided bean definition and move all data to that new table
// WARNING: YOU MUST PROVIDE THE FULL BEAN DEFINITION
func RecreateTables(beans ...interface{}) func(*xorm.Engine) error {
	return func(x *xorm.Engine) error {
		sess := x.NewSession()
		defer sess.Close()
		if err := sess.Begin(); err != nil {
			return err
		}
		sess = sess.StoreEngine("InnoDB")
		for _, bean := range beans {
			log.Info("Recreating Table: %s for Bean: %s", x.TableName(bean), reflect.Indirect(reflect.ValueOf(bean)).Type().Name())
			if err := recreateTable(sess, bean); err != nil {
				return err
			}
		}
		return sess.Commit()
	}
}

// recreateTable will recreate the table using the newly provided bean definition and move all data to that new table
// WARNING: YOU MUST PROVIDE THE FULL BEAN DEFINITION
// WARNING: YOU MUST COMMIT THE SESSION AT THE END
func recreateTable(sess *xorm.Session, bean interface{}) error {
	// TODO: This will not work if there are foreign keys

	tableName := sess.Engine().TableName(bean)
	tempTableName := fmt.Sprintf("tmp_recreate__%s", tableName)

	// We need to move the old table away and create a new one with the correct columns
	// We will need to do this in stages to prevent data loss
	//
	// First create the temporary table
	if err := sess.Table(tempTableName).CreateTable(bean); err != nil {
		log.Error("Unable to create table %s. Error: %v", tempTableName, err)
		return err
	}

	if err := sess.Table(tempTableName).CreateUniques(bean); err != nil {
		log.Error("Unable to create uniques for table %s. Error: %v", tempTableName, err)
		return err
	}

	if err := sess.Table(tempTableName).CreateIndexes(bean); err != nil {
		log.Error("Unable to create indexes for table %s. Error: %v", tempTableName, err)
		return err
	}

	// Work out the column names from the bean - these are the columns to select from the old table and install into the new table
	table, err := sess.Engine().TableInfo(bean)
	if err != nil {
		log.Error("Unable to get table info. Error: %v", err)

		return err
	}
	newTableColumns := table.Columns()
	if len(newTableColumns) == 0 {
		return fmt.Errorf("no columns in new table")
	}
	hasID := false
	for _, column := range newTableColumns {
		hasID = hasID || (column.IsPrimaryKey && column.IsAutoIncrement)
	}

	if hasID && setting.Database.UseMSSQL {
		if _, err := sess.Exec(fmt.Sprintf("SET IDENTITY_INSERT `%s` ON", tempTableName)); err != nil {
			log.Error("Unable to set identity insert for table %s. Error: %v", tempTableName, err)
			return err
		}
	}

	sqlStringBuilder := &strings.Builder{}
	_, _ = sqlStringBuilder.WriteString("INSERT INTO `")
	_, _ = sqlStringBuilder.WriteString(tempTableName)
	_, _ = sqlStringBuilder.WriteString("` (`")
	_, _ = sqlStringBuilder.WriteString(newTableColumns[0].Name)
	_, _ = sqlStringBuilder.WriteString("`")
	for _, column := range newTableColumns[1:] {
		_, _ = sqlStringBuilder.WriteString(", `")
		_, _ = sqlStringBuilder.WriteString(column.Name)
		_, _ = sqlStringBuilder.WriteString("`")
	}
	_, _ = sqlStringBuilder.WriteString(")")
	_, _ = sqlStringBuilder.WriteString(" SELECT ")
	if newTableColumns[0].Default != "" {
		_, _ = sqlStringBuilder.WriteString("COALESCE(`")
		_, _ = sqlStringBuilder.WriteString(newTableColumns[0].Name)
		_, _ = sqlStringBuilder.WriteString("`, ")
		_, _ = sqlStringBuilder.WriteString(newTableColumns[0].Default)
		_, _ = sqlStringBuilder.WriteString(")")
	} else {
		_, _ = sqlStringBuilder.WriteString("`")
		_, _ = sqlStringBuilder.WriteString(newTableColumns[0].Name)
		_, _ = sqlStringBuilder.WriteString("`")
	}

	for _, column := range newTableColumns[1:] {
		if column.Default != "" {
			_, _ = sqlStringBuilder.WriteString(", COALESCE(`")
			_, _ = sqlStringBuilder.WriteString(column.Name)
			_, _ = sqlStringBuilder.WriteString("`, ")
			_, _ = sqlStringBuilder.WriteString(column.Default)
			_, _ = sqlStringBuilder.WriteString(")")
		} else {
			_, _ = sqlStringBuilder.WriteString(", `")
			_, _ = sqlStringBuilder.WriteString(column.Name)
			_, _ = sqlStringBuilder.WriteString("`")
		}
	}
	_, _ = sqlStringBuilder.WriteString(" FROM `")
	_, _ = sqlStringBuilder.WriteString(tableName)
	_, _ = sqlStringBuilder.WriteString("`")

	if _, err := sess.Exec(sqlStringBuilder.String()); err != nil {
		log.Error("Unable to set copy data in to temp table %s. Error: %v", tempTableName, err)
		return err
	}

	if hasID && setting.Database.UseMSSQL {
		if _, err := sess.Exec(fmt.Sprintf("SET IDENTITY_INSERT `%s` OFF", tempTableName)); err != nil {
			log.Error("Unable to switch off identity insert for table %s. Error: %v", tempTableName, err)
			return err
		}
	}

	switch {
	case setting.Database.UseSQLite3:
		// SQLite will drop all the constraints on the old table
		if _, err := sess.Exec(fmt.Sprintf("DROP TABLE `%s`", tableName)); err != nil {
			log.Error("Unable to drop old table %s. Error: %v", tableName, err)
			return err
		}

		if err := sess.Table(tempTableName).DropIndexes(bean); err != nil {
			log.Error("Unable to drop indexes on temporary table %s. Error: %v", tempTableName, err)
			return err
		}

		if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE `%s` RENAME TO `%s`", tempTableName, tableName)); err != nil {
			log.Error("Unable to rename %s to %s. Error: %v", tempTableName, tableName, err)
			return err
		}

		if err := sess.Table(tableName).CreateIndexes(bean); err != nil {
			log.Error("Unable to recreate indexes on table %s. Error: %v", tableName, err)
			return err
		}

		if err := sess.Table(tableName).CreateUniques(bean); err != nil {
			log.Error("Unable to recreate uniques on table %s. Error: %v", tableName, err)
			return err
		}

	case setting.Database.UseMySQL:
		// MySQL will drop all the constraints on the old table
		if _, err := sess.Exec(fmt.Sprintf("DROP TABLE `%s`", tableName)); err != nil {
			log.Error("Unable to drop old table %s. Error: %v", tableName, err)
			return err
		}

		// SQLite and MySQL will move all the constraints from the temporary table to the new table
		if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE `%s` RENAME TO `%s`", tempTableName, tableName)); err != nil {
			log.Error("Unable to rename %s to %s. Error: %v", tempTableName, tableName, err)
			return err
		}
	case setting.Database.UsePostgreSQL:
		var originalSequences []string
		type sequenceData struct {
			LastValue int  `xorm:"'last_value'"`
			IsCalled  bool `xorm:"'is_called'"`
		}
		sequenceMap := map[string]sequenceData{}

		schema := sess.Engine().Dialect().URI().Schema
		sess.Engine().SetSchema("")
		if err := sess.Table("information_schema.sequences").Cols("sequence_name").Where("sequence_name LIKE ? || '_%' AND sequence_catalog = ?", tableName, setting.Database.Name).Find(&originalSequences); err != nil {
			log.Error("Unable to rename %s to %s. Error: %v", tempTableName, tableName, err)
			return err
		}
		sess.Engine().SetSchema(schema)

		for _, sequence := range originalSequences {
			sequenceData := sequenceData{}
			if _, err := sess.Table(sequence).Cols("last_value", "is_called").Get(&sequenceData); err != nil {
				log.Error("Unable to get last_value and is_called from %s. Error: %v", sequence, err)
				return err
			}
			sequenceMap[sequence] = sequenceData

		}

		// CASCADE causes postgres to drop all the constraints on the old table
		if _, err := sess.Exec(fmt.Sprintf("DROP TABLE `%s` CASCADE", tableName)); err != nil {
			log.Error("Unable to drop old table %s. Error: %v", tableName, err)
			return err
		}

		// CASCADE causes postgres to move all the constraints from the temporary table to the new table
		if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE `%s` RENAME TO `%s`", tempTableName, tableName)); err != nil {
			log.Error("Unable to rename %s to %s. Error: %v", tempTableName, tableName, err)
			return err
		}

		var indices []string
		sess.Engine().SetSchema("")
		if err := sess.Table("pg_indexes").Cols("indexname").Where("tablename = ? ", tableName).Find(&indices); err != nil {
			log.Error("Unable to rename %s to %s. Error: %v", tempTableName, tableName, err)
			return err
		}
		sess.Engine().SetSchema(schema)

		for _, index := range indices {
			newIndexName := strings.Replace(index, "tmp_recreate__", "", 1)
			if _, err := sess.Exec(fmt.Sprintf("ALTER INDEX `%s` RENAME TO `%s`", index, newIndexName)); err != nil {
				log.Error("Unable to rename %s to %s. Error: %v", index, newIndexName, err)
				return err
			}
		}

		var sequences []string
		sess.Engine().SetSchema("")
		if err := sess.Table("information_schema.sequences").Cols("sequence_name").Where("sequence_name LIKE 'tmp_recreate__' || ? || '_%' AND sequence_catalog = ?", tableName, setting.Database.Name).Find(&sequences); err != nil {
			log.Error("Unable to rename %s to %s. Error: %v", tempTableName, tableName, err)
			return err
		}
		sess.Engine().SetSchema(schema)

		for _, sequence := range sequences {
			newSequenceName := strings.Replace(sequence, "tmp_recreate__", "", 1)
			if _, err := sess.Exec(fmt.Sprintf("ALTER SEQUENCE `%s` RENAME TO `%s`", sequence, newSequenceName)); err != nil {
				log.Error("Unable to rename %s sequence to %s. Error: %v", sequence, newSequenceName, err)
				return err
			}
			val, ok := sequenceMap[newSequenceName]
			if newSequenceName == tableName+"_id_seq" {
				if ok && val.LastValue != 0 {
					if _, err := sess.Exec(fmt.Sprintf("SELECT setval('%s', %d, %t)", newSequenceName, val.LastValue, val.IsCalled)); err != nil {
						log.Error("Unable to reset %s to %d. Error: %v", newSequenceName, val, err)
						return err
					}
				} else {
					// We're going to try to guess this
					if _, err := sess.Exec(fmt.Sprintf("SELECT setval('%s', COALESCE((SELECT MAX(id)+1 FROM `%s`), 1), false)", newSequenceName, tableName)); err != nil {
						log.Error("Unable to reset %s. Error: %v", newSequenceName, err)
						return err
					}
				}
			} else if ok {
				if _, err := sess.Exec(fmt.Sprintf("SELECT setval('%s', %d, %t)", newSequenceName, val.LastValue, val.IsCalled)); err != nil {
					log.Error("Unable to reset %s to %d. Error: %v", newSequenceName, val, err)
					return err
				}
			}

		}

	case setting.Database.UseMSSQL:
		// MSSQL will drop all the constraints on the old table
		if _, err := sess.Exec(fmt.Sprintf("DROP TABLE `%s`", tableName)); err != nil {
			log.Error("Unable to drop old table %s. Error: %v", tableName, err)
			return err
		}

		// MSSQL sp_rename will move all the constraints from the temporary table to the new table
		if _, err := sess.Exec(fmt.Sprintf("sp_rename `%s`,`%s`", tempTableName, tableName)); err != nil {
			log.Error("Unable to rename %s to %s. Error: %v", tempTableName, tableName, err)
			return err
		}

	default:
		log.Fatal("Unrecognized DB")
	}
	return nil
}

// WARNING: YOU MUST COMMIT THE SESSION AT THE END
func dropTableColumns(sess *xorm.Session, tableName string, columnNames ...string) (err error) {
	if tableName == "" || len(columnNames) == 0 {
		return nil
	}
	// TODO: This will not work if there are foreign keys

	switch {
	case setting.Database.UseSQLite3:
		// First drop the indexes on the columns
		res, errIndex := sess.Query(fmt.Sprintf("PRAGMA index_list(`%s`)", tableName))
		if errIndex != nil {
			return errIndex
		}
		for _, row := range res {
			indexName := row["name"]
			indexRes, err := sess.Query(fmt.Sprintf("PRAGMA index_info(`%s`)", indexName))
			if err != nil {
				return err
			}
			if len(indexRes) != 1 {
				continue
			}
			indexColumn := string(indexRes[0]["name"])
			for _, name := range columnNames {
				if name == indexColumn {
					_, err := sess.Exec(fmt.Sprintf("DROP INDEX `%s`", indexName))
					if err != nil {
						return err
					}
				}
			}
		}

		// Here we need to get the columns from the original table
		sql := fmt.Sprintf("SELECT sql FROM sqlite_master WHERE tbl_name='%s' and type='table'", tableName)
		res, err := sess.Query(sql)
		if err != nil {
			return err
		}
		tableSQL := string(res[0]["sql"])

		// Separate out the column definitions
		tableSQL = tableSQL[strings.Index(tableSQL, "("):]

		// Remove the required columnNames
		for _, name := range columnNames {
			tableSQL = regexp.MustCompile(regexp.QuoteMeta("`"+name+"`")+"[^`,)]*?[,)]").ReplaceAllString(tableSQL, "")
		}

		// Ensure the query is ended properly
		tableSQL = strings.TrimSpace(tableSQL)
		if tableSQL[len(tableSQL)-1] != ')' {
			if tableSQL[len(tableSQL)-1] == ',' {
				tableSQL = tableSQL[:len(tableSQL)-1]
			}
			tableSQL += ")"
		}

		// Find all the columns in the table
		columns := regexp.MustCompile("`([^`]*)`").FindAllString(tableSQL, -1)

		tableSQL = fmt.Sprintf("CREATE TABLE `new_%s_new` ", tableName) + tableSQL
		if _, err := sess.Exec(tableSQL); err != nil {
			return err
		}

		// Now restore the data
		columnsSeparated := strings.Join(columns, ",")
		insertSQL := fmt.Sprintf("INSERT INTO `new_%s_new` (%s) SELECT %s FROM %s", tableName, columnsSeparated, columnsSeparated, tableName)
		if _, err := sess.Exec(insertSQL); err != nil {
			return err
		}

		// Now drop the old table
		if _, err := sess.Exec(fmt.Sprintf("DROP TABLE `%s`", tableName)); err != nil {
			return err
		}

		// Rename the table
		if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE `new_%s_new` RENAME TO `%s`", tableName, tableName)); err != nil {
			return err
		}

	case setting.Database.UsePostgreSQL:
		cols := ""
		for _, col := range columnNames {
			if cols != "" {
				cols += ", "
			}
			cols += "DROP COLUMN `" + col + "` CASCADE"
		}
		if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE `%s` %s", tableName, cols)); err != nil {
			return fmt.Errorf("Drop table `%s` columns %v: %v", tableName, columnNames, err)
		}
	case setting.Database.UseMySQL:
		// Drop indexes on columns first
		sql := fmt.Sprintf("SHOW INDEX FROM %s WHERE column_name IN ('%s')", tableName, strings.Join(columnNames, "','"))
		res, err := sess.Query(sql)
		if err != nil {
			return err
		}
		for _, index := range res {
			indexName := index["column_name"]
			if len(indexName) > 0 {
				_, err := sess.Exec(fmt.Sprintf("DROP INDEX `%s` ON `%s`", indexName, tableName))
				if err != nil {
					return err
				}
			}
		}

		// Now drop the columns
		cols := ""
		for _, col := range columnNames {
			if cols != "" {
				cols += ", "
			}
			cols += "DROP COLUMN `" + col + "`"
		}
		if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE `%s` %s", tableName, cols)); err != nil {
			return fmt.Errorf("Drop table `%s` columns %v: %v", tableName, columnNames, err)
		}
	case setting.Database.UseMSSQL:
		cols := ""
		for _, col := range columnNames {
			if cols != "" {
				cols += ", "
			}
			cols += "`" + strings.ToLower(col) + "`"
		}
		sql := fmt.Sprintf("SELECT Name FROM SYS.DEFAULT_CONSTRAINTS WHERE PARENT_OBJECT_ID = OBJECT_ID('%[1]s') AND PARENT_COLUMN_ID IN (SELECT column_id FROM sys.columns WHERE lower(NAME) IN (%[2]s) AND object_id = OBJECT_ID('%[1]s'))",
			tableName, strings.ReplaceAll(cols, "`", "'"))
		constraints := make([]string, 0)
		if err := sess.SQL(sql).Find(&constraints); err != nil {
			return fmt.Errorf("Find constraints: %v", err)
		}
		for _, constraint := range constraints {
			if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE `%s` DROP CONSTRAINT `%s`", tableName, constraint)); err != nil {
				return fmt.Errorf("Drop table `%s` default constraint `%s`: %v", tableName, constraint, err)
			}
		}
		sql = fmt.Sprintf("SELECT DISTINCT Name FROM SYS.INDEXES INNER JOIN SYS.INDEX_COLUMNS ON INDEXES.INDEX_ID = INDEX_COLUMNS.INDEX_ID AND INDEXES.OBJECT_ID = INDEX_COLUMNS.OBJECT_ID WHERE INDEXES.OBJECT_ID = OBJECT_ID('%[1]s') AND INDEX_COLUMNS.COLUMN_ID IN (SELECT column_id FROM sys.columns WHERE lower(NAME) IN (%[2]s) AND object_id = OBJECT_ID('%[1]s'))",
			tableName, strings.ReplaceAll(cols, "`", "'"))
		constraints = make([]string, 0)
		if err := sess.SQL(sql).Find(&constraints); err != nil {
			return fmt.Errorf("Find constraints: %v", err)
		}
		for _, constraint := range constraints {
			if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE `%s` DROP CONSTRAINT IF EXISTS `%s`", tableName, constraint)); err != nil {
				return fmt.Errorf("Drop table `%s` index constraint `%s`: %v", tableName, constraint, err)
			}
			if _, err := sess.Exec(fmt.Sprintf("DROP INDEX IF EXISTS `%[2]s` ON `%[1]s`", tableName, constraint)); err != nil {
				return fmt.Errorf("Drop index `%[2]s` on `%[1]s`: %v", tableName, constraint, err)
			}
		}

		if _, err := sess.Exec(fmt.Sprintf("ALTER TABLE `%s` DROP COLUMN %s", tableName, cols)); err != nil {
			return fmt.Errorf("Drop table `%s` columns %v: %v", tableName, columnNames, err)
		}
	default:
		log.Fatal("Unrecognized DB")
	}

	return nil
}

// modifyColumn will modify column's type or other propertity. SQLITE is not supported
func modifyColumn(x *xorm.Engine, tableName string, col *schemas.Column) error {
	var indexes map[string]*schemas.Index
	var err error
	// MSSQL have to remove index at first, otherwise alter column will fail
	// ref. https://sqlzealots.com/2018/05/09/error-message-the-index-is-dependent-on-column-alter-table-alter-column-failed-because-one-or-more-objects-access-this-column/
	if x.Dialect().URI().DBType == schemas.MSSQL {
		indexes, err = x.Dialect().GetIndexes(x.DB(), context.Background(), tableName)
		if err != nil {
			return err
		}

		for _, index := range indexes {
			_, err = x.Exec(x.Dialect().DropIndexSQL(tableName, index))
			if err != nil {
				return err
			}
		}
	}

	defer func() {
		for _, index := range indexes {
			_, err = x.Exec(x.Dialect().CreateIndexSQL(tableName, index))
			if err != nil {
				log.Error("Create index %s on table %s failed: %v", index.Name, tableName, err)
			}
		}
	}()

	alterSQL := x.Dialect().ModifyColumnSQL(tableName, col)
	if _, err := x.Exec(alterSQL); err != nil {
		return err
	}
	return nil
}
