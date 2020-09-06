// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	golog "log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/migrations"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/options"
	"code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"xorm.io/builder"
	"xorm.io/xorm"

	"github.com/urfave/cli"
)

// CmdDoctor represents the available doctor sub-command.
var CmdDoctor = cli.Command{
	Name:        "doctor",
	Usage:       "Diagnose problems",
	Description: "A command to diagnose problems with the current Gitea instance according to the given configuration.",
	Action:      runDoctor,
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "list",
			Usage: "List the available checks",
		},
		cli.BoolFlag{
			Name:  "default",
			Usage: "Run the default checks (if neither --run or --all is set, this is the default behaviour)",
		},
		cli.StringSliceFlag{
			Name:  "run",
			Usage: "Run the provided checks - (if --default is set, the default checks will also run)",
		},
		cli.BoolFlag{
			Name:  "all",
			Usage: "Run all the available checks",
		},
		cli.BoolFlag{
			Name:  "fix",
			Usage: "Automatically fix what we can",
		},
		cli.StringFlag{
			Name:  "log-file",
			Usage: `Name of the log file (default: "doctor.log"). Set to "-" to output to stdout, set to "" to disable`,
		},
	},
	Subcommands: []cli.Command{
		cmdRecreateTable,
	},
}

var cmdRecreateTable = cli.Command{
	Name:      "recreate-table",
	Usage:     "Recreate tables from XORM definitions and copy the data.",
	ArgsUsage: "[TABLE]... : (TABLEs to recreate - leave blank for all)",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "debug",
			Usage: "Print SQL commands sent",
		},
	},
	Description: `The database definitions Gitea uses change across versions, sometimes changing default values and leaving old unused columns.

This command will cause Xorm to recreate tables, copying over the data and deleting the old table.

You should back-up your database before doing this and ensure that your database is up-to-date first.`,
	Action: runRecreateTable,
}

type check struct {
	title            string
	name             string
	isDefault        bool
	f                func(ctx *cli.Context) ([]string, error)
	abortIfFailed    bool
	skipDatabaseInit bool
}

// checklist represents list for all checks
var checklist = []check{
	{
		// NOTE: this check should be the first in the list
		title:            "Check paths and basic configuration",
		name:             "paths",
		isDefault:        true,
		f:                runDoctorPathInfo,
		abortIfFailed:    true,
		skipDatabaseInit: true,
	},
	{
		title:         "Check Database Version",
		name:          "check-db-version",
		isDefault:     true,
		f:             runDoctorCheckDBVersion,
		abortIfFailed: false,
	},
	{
		title:     "Check consistency of database",
		name:      "check-db-consistency",
		isDefault: false,
		f:         runDoctorCheckDBConsistency,
	},
	{
		title:     "Check if OpenSSH authorized_keys file is up-to-date",
		name:      "authorized_keys",
		isDefault: true,
		f:         runDoctorAuthorizedKeys,
	},
	{
		title:     "Check if SCRIPT_TYPE is available",
		name:      "script-type",
		isDefault: false,
		f:         runDoctorScriptType,
	},
	{
		title:     "Check if hook files are up-to-date and executable",
		name:      "hooks",
		isDefault: false,
		f:         runDoctorHooks,
	},
	{
		title:     "Recalculate merge bases",
		name:      "recalculate_merge_bases",
		isDefault: false,
		f:         runDoctorPRMergeBase,
	},
	{
		title:     "Recalculate Stars number for all user",
		name:      "recalculate_stars_number",
		isDefault: false,
		f:         runDoctorUserStarNum,
	},
	{
		title:     "Enable push options",
		name:      "enable-push-options",
		isDefault: false,
		f:         runDoctorEnablePushOptions,
	},
	// more checks please append here
}

func runRecreateTable(ctx *cli.Context) error {
	// Redirect the default golog to here
	golog.SetFlags(0)
	golog.SetPrefix("")
	golog.SetOutput(log.NewLoggerAsWriter("INFO", log.GetLogger(log.DEFAULT)))

	setting.NewContext()
	setting.InitDBConfig()

	setting.EnableXORMLog = ctx.Bool("debug")
	setting.Database.LogSQL = ctx.Bool("debug")
	setting.Cfg.Section("log").Key("XORM").SetValue(",")

	setting.NewXORMLogService(!ctx.Bool("debug"))
	if err := models.SetEngine(); err != nil {
		fmt.Println(err)
		fmt.Println("Check if you are using the right config file. You can use a --config directive to specify one.")
		return nil
	}

	args := ctx.Args()
	names := make([]string, 0, ctx.NArg())
	for i := 0; i < ctx.NArg(); i++ {
		names = append(names, args.Get(i))
	}

	beans, err := models.NamesToBean(names...)
	if err != nil {
		return err
	}
	recreateTables := migrations.RecreateTables(beans...)

	return models.NewEngine(context.Background(), func(x *xorm.Engine) error {
		if err := migrations.EnsureUpToDate(x); err != nil {
			return err
		}
		return recreateTables(x)
	})

}

func runDoctor(ctx *cli.Context) error {

	// Silence the default loggers
	log.DelNamedLogger("console")
	log.DelNamedLogger(log.DEFAULT)

	// Now setup our own
	logFile := ctx.String("log-file")
	if !ctx.IsSet("log-file") {
		logFile = "doctor.log"
	}

	if len(logFile) == 0 {
		log.NewLogger(1000, "doctor", "console", `{"level":"NONE","stacktracelevel":"NONE","colorize":"%t"}`)
	} else if logFile == "-" {
		log.NewLogger(1000, "doctor", "console", `{"level":"trace","stacktracelevel":"NONE"}`)
	} else {
		log.NewLogger(1000, "doctor", "file", fmt.Sprintf(`{"filename":%q,"level":"trace","stacktracelevel":"NONE"}`, logFile))
	}

	// Finally redirect the default golog to here
	golog.SetFlags(0)
	golog.SetPrefix("")
	golog.SetOutput(log.NewLoggerAsWriter("INFO", log.GetLogger(log.DEFAULT)))

	if ctx.IsSet("list") {
		w := tabwriter.NewWriter(os.Stdout, 0, 8, 0, '\t', 0)
		_, _ = w.Write([]byte("Default\tName\tTitle\n"))
		for _, check := range checklist {
			if check.isDefault {
				_, _ = w.Write([]byte{'*'})
			}
			_, _ = w.Write([]byte{'\t'})
			_, _ = w.Write([]byte(check.name))
			_, _ = w.Write([]byte{'\t'})
			_, _ = w.Write([]byte(check.title))
			_, _ = w.Write([]byte{'\n'})
		}
		return w.Flush()
	}

	var checks []check
	if ctx.Bool("all") {
		checks = checklist
	} else if ctx.IsSet("run") {
		addDefault := ctx.Bool("default")
		names := ctx.StringSlice("run")
		for i, name := range names {
			names[i] = strings.ToLower(strings.TrimSpace(name))
		}

		for _, check := range checklist {
			if addDefault && check.isDefault {
				checks = append(checks, check)
				continue
			}
			for _, name := range names {
				if name == check.name {
					checks = append(checks, check)
					break
				}
			}
		}
	} else {
		for _, check := range checklist {
			if check.isDefault {
				checks = append(checks, check)
			}
		}
	}

	dbIsInit := false

	for i, check := range checks {
		if !dbIsInit && !check.skipDatabaseInit {
			// Only open database after the most basic configuration check
			setting.EnableXORMLog = false
			if err := initDBDisableConsole(true); err != nil {
				fmt.Println(err)
				fmt.Println("Check if you are using the right config file. You can use a --config directive to specify one.")
				return nil
			}
			dbIsInit = true
		}
		fmt.Println("[", i+1, "]", check.title)
		messages, err := check.f(ctx)
		for _, message := range messages {
			fmt.Println("-", message)
		}
		if err != nil {
			fmt.Println("Error:", err)
			if check.abortIfFailed {
				return nil
			}
		} else {
			fmt.Println("OK.")
		}
		fmt.Println()
	}
	return nil
}

func runDoctorPathInfo(ctx *cli.Context) ([]string, error) {

	res := make([]string, 0, 10)

	if fi, err := os.Stat(setting.CustomConf); err != nil || !fi.Mode().IsRegular() {
		res = append(res, fmt.Sprintf("Failed to find configuration file at '%s'.", setting.CustomConf))
		res = append(res, fmt.Sprintf("If you've never ran Gitea yet, this is normal and '%s' will be created for you on first run.", setting.CustomConf))
		res = append(res, "Otherwise check that you are running this command from the correct path and/or provide a `--config` parameter.")
		return res, fmt.Errorf("can't proceed without a configuration file")
	}

	setting.NewContext()

	fail := false

	check := func(name, path string, is_dir, required, is_write bool) {
		res = append(res, fmt.Sprintf("%-25s  '%s'", name+":", path))
		fi, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) && ctx.Bool("fix") && is_dir {
				if err := os.MkdirAll(path, 0777); err != nil {
					res = append(res, fmt.Sprintf("    ERROR: %v", err))
					fail = true
					return
				}
				fi, err = os.Stat(path)
			}
		}
		if err != nil {
			if required {
				res = append(res, fmt.Sprintf("    ERROR: %v", err))
				fail = true
				return
			}
			res = append(res, fmt.Sprintf("    NOTICE: not accessible (%v)", err))
			return
		}

		if is_dir && !fi.IsDir() {
			res = append(res, "    ERROR: not a directory")
			fail = true
			return
		} else if !is_dir && !fi.Mode().IsRegular() {
			res = append(res, "    ERROR: not a regular file")
			fail = true
		} else if is_write {
			if err := runDoctorWritableDir(path); err != nil {
				res = append(res, fmt.Sprintf("    ERROR: not writable: %v", err))
				fail = true
			}
		}
	}

	// Note print paths inside quotes to make any leading/trailing spaces evident
	check("Configuration File Path", setting.CustomConf, false, true, false)
	check("Repository Root Path", setting.RepoRootPath, true, true, true)
	check("Data Root Path", setting.AppDataPath, true, true, true)
	check("Custom File Root Path", setting.CustomPath, true, false, false)
	check("Work directory", setting.AppWorkPath, true, true, false)
	check("Log Root Path", setting.LogRootPath, true, true, true)

	if options.IsDynamic() {
		// Do not check/report on StaticRootPath if data is embedded in Gitea (-tags bindata)
		check("Static File Root Path", setting.StaticRootPath, true, true, false)
	}

	if fail {
		return res, fmt.Errorf("please check your configuration file and try again")
	}

	return res, nil
}

func runDoctorWritableDir(path string) error {
	// There's no platform-independent way of checking if a directory is writable
	// https://stackoverflow.com/questions/20026320/how-to-tell-if-folder-exists-and-is-writable

	tmpFile, err := ioutil.TempFile(path, "doctors-order")
	if err != nil {
		return err
	}
	if err := util.Remove(tmpFile.Name()); err != nil {
		fmt.Printf("Warning: can't remove temporary file: '%s'\n", tmpFile.Name())
	}
	tmpFile.Close()
	return nil
}

const tplCommentPrefix = `# gitea public key`

func runDoctorAuthorizedKeys(ctx *cli.Context) ([]string, error) {
	if setting.SSH.StartBuiltinServer || !setting.SSH.CreateAuthorizedKeysFile {
		return nil, nil
	}

	fPath := filepath.Join(setting.SSH.RootPath, "authorized_keys")
	f, err := os.Open(fPath)
	if err != nil {
		if ctx.Bool("fix") {
			return []string{fmt.Sprintf("Error whilst opening authorized_keys: %v. Attempting regeneration", err)}, models.RewriteAllPublicKeys()
		}
		return nil, err
	}
	defer f.Close()

	linesInAuthorizedKeys := map[string]bool{}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, tplCommentPrefix) {
			continue
		}
		linesInAuthorizedKeys[line] = true
	}
	f.Close()

	// now we regenerate and check if there are any lines missing
	regenerated := &bytes.Buffer{}
	if err := models.RegeneratePublicKeys(regenerated); err != nil {
		return nil, err
	}
	scanner = bufio.NewScanner(regenerated)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, tplCommentPrefix) {
			continue
		}
		if ok := linesInAuthorizedKeys[line]; ok {
			continue
		}
		if ctx.Bool("fix") {
			return []string{"authorized_keys is out of date, attempting regeneration"}, models.RewriteAllPublicKeys()
		}
		return nil, fmt.Errorf(`authorized_keys is out of date and should be regenerated with "gitea admin regenerate keys" or "gitea doctor --run authorized_keys --fix"`)
	}
	return nil, nil
}

func runDoctorCheckDBVersion(ctx *cli.Context) ([]string, error) {
	if err := models.NewEngine(context.Background(), migrations.EnsureUpToDate); err != nil {
		if ctx.Bool("fix") {
			return []string{fmt.Sprintf("WARN: Got Error %v during ensure up to date", err), "Attempting to migrate to the latest DB version to fix this."}, models.NewEngine(context.Background(), migrations.Migrate)
		}
		return nil, err
	}
	return nil, nil
}

func iterateRepositories(each func(*models.Repository) ([]string, error)) ([]string, error) {
	results := []string{}
	err := models.Iterate(
		models.DefaultDBContext(),
		new(models.Repository),
		builder.Gt{"id": 0},
		func(idx int, bean interface{}) error {
			res, err := each(bean.(*models.Repository))
			results = append(results, res...)
			return err
		},
	)
	return results, err
}

func iteratePRs(repo *models.Repository, each func(*models.Repository, *models.PullRequest) ([]string, error)) ([]string, error) {
	results := []string{}
	err := models.Iterate(
		models.DefaultDBContext(),
		new(models.PullRequest),
		builder.Eq{"base_repo_id": repo.ID},
		func(idx int, bean interface{}) error {
			res, err := each(repo, bean.(*models.PullRequest))
			results = append(results, res...)
			return err
		},
	)
	return results, err
}

func runDoctorHooks(ctx *cli.Context) ([]string, error) {
	// Need to iterate across all of the repositories
	return iterateRepositories(func(repo *models.Repository) ([]string, error) {
		results, err := repository.CheckDelegateHooks(repo.RepoPath())
		if err != nil {
			return nil, err
		}
		if len(results) > 0 && ctx.Bool("fix") {
			return []string{fmt.Sprintf("regenerated hooks for %s", repo.FullName())}, repository.CreateDelegateHooks(repo.RepoPath())
		}

		return results, nil
	})
}

func runDoctorPRMergeBase(ctx *cli.Context) ([]string, error) {
	numRepos := 0
	numPRs := 0
	numPRsUpdated := 0
	results, err := iterateRepositories(func(repo *models.Repository) ([]string, error) {
		numRepos++
		return iteratePRs(repo, func(repo *models.Repository, pr *models.PullRequest) ([]string, error) {
			numPRs++
			results := []string{}
			pr.BaseRepo = repo
			repoPath := repo.RepoPath()

			oldMergeBase := pr.MergeBase

			if !pr.HasMerged {
				var err error
				pr.MergeBase, err = git.NewCommand("merge-base", "--", pr.BaseBranch, pr.GetGitRefName()).RunInDir(repoPath)
				if err != nil {
					var err2 error
					pr.MergeBase, err2 = git.NewCommand("rev-parse", git.BranchPrefix+pr.BaseBranch).RunInDir(repoPath)
					if err2 != nil {
						results = append(results, fmt.Sprintf("WARN: Unable to get merge base for PR ID %d, #%d onto %s in %s/%s", pr.ID, pr.Index, pr.BaseBranch, pr.BaseRepo.OwnerName, pr.BaseRepo.Name))
						log.Error("Unable to get merge base for PR ID %d, Index %d in %s/%s. Error: %v & %v", pr.ID, pr.Index, pr.BaseRepo.OwnerName, pr.BaseRepo.Name, err, err2)
						return results, nil
					}
				}
			} else {
				parentsString, err := git.NewCommand("rev-list", "--parents", "-n", "1", pr.MergedCommitID).RunInDir(repoPath)
				if err != nil {
					results = append(results, fmt.Sprintf("WARN: Unable to get parents for merged PR ID %d, #%d onto %s in %s/%s", pr.ID, pr.Index, pr.BaseBranch, pr.BaseRepo.OwnerName, pr.BaseRepo.Name))
					log.Error("Unable to get parents for merged PR ID %d, Index %d in %s/%s. Error: %v", pr.ID, pr.Index, pr.BaseRepo.OwnerName, pr.BaseRepo.Name, err)
					return results, nil
				}
				parents := strings.Split(strings.TrimSpace(parentsString), " ")
				if len(parents) < 2 {
					return results, nil
				}

				args := append([]string{"merge-base", "--"}, parents[1:]...)
				args = append(args, pr.GetGitRefName())

				pr.MergeBase, err = git.NewCommand(args...).RunInDir(repoPath)
				if err != nil {
					results = append(results, fmt.Sprintf("WARN: Unable to get merge base for merged PR ID %d, #%d onto %s in %s/%s", pr.ID, pr.Index, pr.BaseBranch, pr.BaseRepo.OwnerName, pr.BaseRepo.Name))
					log.Error("Unable to get merge base for merged PR ID %d, Index %d in %s/%s. Error: %v", pr.ID, pr.Index, pr.BaseRepo.OwnerName, pr.BaseRepo.Name, err)
					return results, nil
				}
			}
			pr.MergeBase = strings.TrimSpace(pr.MergeBase)
			if pr.MergeBase != oldMergeBase {
				if ctx.Bool("fix") {
					if err := pr.UpdateCols("merge_base"); err != nil {
						return results, err
					}
				} else {
					results = append(results, fmt.Sprintf("#%d onto %s in %s/%s: MergeBase should be %s but is %s", pr.Index, pr.BaseBranch, pr.BaseRepo.OwnerName, pr.BaseRepo.Name, oldMergeBase, pr.MergeBase))
				}
				numPRsUpdated++
			}
			return results, nil
		})
	})

	if ctx.Bool("fix") {
		results = append(results, fmt.Sprintf("%d PR mergebases updated of %d PRs total in %d repos", numPRsUpdated, numPRs, numRepos))
	} else {
		if numPRsUpdated > 0 && err == nil {
			return results, fmt.Errorf("%d PRs with incorrect mergebases of %d PRs total in %d repos", numPRsUpdated, numPRs, numRepos)
		}
		results = append(results, fmt.Sprintf("%d PRs with incorrect mergebases of %d PRs total in %d repos", numPRsUpdated, numPRs, numRepos))
	}

	return results, err
}

func runDoctorUserStarNum(ctx *cli.Context) ([]string, error) {
	return nil, models.DoctorUserStarNum()
}

func runDoctorScriptType(ctx *cli.Context) ([]string, error) {
	path, err := exec.LookPath(setting.ScriptType)
	if err != nil {
		return []string{fmt.Sprintf("ScriptType %s is not on the current PATH", setting.ScriptType)}, err
	}
	return []string{fmt.Sprintf("ScriptType %s is on the current PATH at %s", setting.ScriptType, path)}, nil
}

func runDoctorCheckDBConsistency(ctx *cli.Context) ([]string, error) {
	var results []string

	// make sure DB version is uptodate
	if err := models.NewEngine(context.Background(), migrations.EnsureUpToDate); err != nil {
		return nil, fmt.Errorf("model version on the database does not match the current Gitea version. Model consistency will not be checked until the database is upgraded")
	}

	//find labels without existing repo or org
	count, err := models.CountOrphanedLabels()
	if err != nil {
		return nil, err
	}
	if count > 0 {
		if ctx.Bool("fix") {
			if err = models.DeleteOrphanedLabels(); err != nil {
				return nil, err
			}
			results = append(results, fmt.Sprintf("%d labels without existing repository/organisation deleted", count))
		} else {
			results = append(results, fmt.Sprintf("%d labels without existing repository/organisation", count))
		}
	}

	//find issues without existing repository
	count, err = models.CountOrphanedIssues()
	if err != nil {
		return nil, err
	}
	if count > 0 {
		if ctx.Bool("fix") {
			if err = models.DeleteOrphanedIssues(); err != nil {
				return nil, err
			}
			results = append(results, fmt.Sprintf("%d issues without existing repository deleted", count))
		} else {
			results = append(results, fmt.Sprintf("%d issues without existing repository", count))
		}
	}

	//find pulls without existing issues
	count, err = models.CountPullsWithoutExistingIssues()
	if err != nil {
		return nil, err
	}
	if count > 0 {
		if ctx.Bool("fix") {
			if err = models.DeletePullsWithoutExistingIssues(); err != nil {
				return nil, err
			}
			results = append(results, fmt.Sprintf("%d pull requests without existing issue deleted", count))
		} else {
			results = append(results, fmt.Sprintf("%d pull requests without existing issue", count))
		}
	}

	//find tracked times without existing issues/pulls
	count, err = models.CountOrphanedTrackedTime()
	if err != nil {
		return nil, err
	}
	if count > 0 {
		if ctx.Bool("fix") {
			if err = models.DeleteOrphanedTrackedTime(); err != nil {
				return nil, err
			}
			results = append(results, fmt.Sprintf("%d tracked times without existing issue deleted", count))
		} else {
			results = append(results, fmt.Sprintf("%d tracked times without existing issue", count))
		}
	}

	count, err = models.CountNullArchivedRepository()
	if err != nil {
		return nil, err
	}
	if count > 0 {
		if ctx.Bool("fix") {
			updatedCount, err := models.FixNullArchivedRepository()
			if err != nil {
				return nil, err
			}
			results = append(results, fmt.Sprintf("%d repositories with null is_archived updated", updatedCount))
		} else {
			results = append(results, fmt.Sprintf("%d repositories with null is_archived", count))
		}
	}

	//ToDo: function to recalc all counters

	return results, nil
}

func runDoctorEnablePushOptions(ctx *cli.Context) ([]string, error) {
	numRepos := 0
	_, err := iterateRepositories(func(repo *models.Repository) ([]string, error) {
		numRepos++
		r, err := git.OpenRepository(repo.RepoPath())
		if err != nil {
			return nil, err
		}
		defer r.Close()

		if ctx.Bool("fix") {
			_, err := git.NewCommand("config", "receive.advertisePushOptions", "true").RunInDir(r.Path)
			return nil, err
		}

		return nil, nil
	})

	var prefix string
	if !ctx.Bool("fix") {
		prefix = "DRY RUN: "
	}
	return []string{fmt.Sprintf("%sEnabled push options for %d repositories.", prefix, numRepos)}, err
}
