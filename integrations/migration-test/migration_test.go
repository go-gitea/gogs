// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"compress/gzip"
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"code.gitea.io/gitea/integrations"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/migrations"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
	"xorm.io/xorm"
)

var currentEngine *xorm.Engine

func initMigrationTest(t *testing.T) func() {
	deferFn := integrations.PrintCurrentTest(t, 2)
	giteaRoot := base.SetupGiteaRoot()
	if giteaRoot == "" {
		integrations.Printf("Environment variable $GITEA_ROOT not set\n")
		os.Exit(1)
	}
	setting.AppPath = path.Join(giteaRoot, "gitea")
	if _, err := os.Stat(setting.AppPath); err != nil {
		integrations.Printf("Could not find gitea binary at %s\n", setting.AppPath)
		os.Exit(1)
	}

	giteaConf := os.Getenv("GITEA_CONF")
	if giteaConf == "" {
		integrations.Printf("Environment variable $GITEA_CONF not set\n")
		os.Exit(1)
	} else if !path.IsAbs(giteaConf) {
		setting.CustomConf = path.Join(giteaRoot, giteaConf)
	} else {
		setting.CustomConf = giteaConf
	}

	setting.NewContext()

	assert.True(t, len(setting.RepoRootPath) != 0)
	assert.NoError(t, util.RemoveAll(setting.RepoRootPath))
	assert.NoError(t, util.CopyDir(path.Join(filepath.Dir(setting.AppPath), "integrations/gitea-repositories-meta"), setting.RepoRootPath))

	git.CheckLFSVersion()
	setting.InitDBConfig()
	setting.NewLogServices(true)
	return deferFn
}

func availableVersions() ([]string, error) {
	migrationsDir, err := os.Open("integrations/migration-test")
	if err != nil {
		return nil, err
	}
	defer migrationsDir.Close()
	versionRE, err := regexp.Compile("gitea-v(?P<version>.+)\\." + regexp.QuoteMeta(setting.Database.Type) + "\\.sql.gz")
	if err != nil {
		return nil, err
	}

	filenames, err := migrationsDir.Readdirnames(-1)
	if err != nil {
		return nil, err
	}
	versions := []string{}
	for _, filename := range filenames {
		if versionRE.MatchString(filename) {
			substrings := versionRE.FindStringSubmatch(filename)
			versions = append(versions, substrings[1])
		}
	}
	sort.Strings(versions)
	return versions, nil
}

func readSQLFromFile(version string) (string, error) {
	filename := fmt.Sprintf("integrations/migration-test/gitea-v%s.%s.sql.gz", version, setting.Database.Type)

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return "", nil
	}

	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	gr, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer gr.Close()

	bytes, err := ioutil.ReadAll(gr)
	if err != nil {
		return "", err
	}
	return string(charset.RemoveBOMIfPresent(bytes)), nil
}

func restoreOldDB(t *testing.T, version string) bool {
	data, err := readSQLFromFile(version)
	assert.NoError(t, err)
	if len(data) == 0 {
		integrations.Printf("No db found to restore for %s version: %s\n", setting.Database.Type, version)
		return false
	}

	switch {
	case setting.Database.UseSQLite3:
		util.Remove(setting.Database.Path)
		err := os.MkdirAll(path.Dir(setting.Database.Path), os.ModePerm)
		assert.NoError(t, err)

		db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?cache=shared&mode=rwc&_busy_timeout=%d&_txlock=immediate", setting.Database.Path, setting.Database.Timeout))
		assert.NoError(t, err)
		defer db.Close()

		_, err = db.Exec(data)
		assert.NoError(t, err)
		db.Close()

	case setting.Database.UseMySQL:
		db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s)/",
			setting.Database.User, setting.Database.Passwd, setting.Database.Host))
		assert.NoError(t, err)
		defer db.Close()

		_, err = db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", setting.Database.Name))
		assert.NoError(t, err)

		_, err = db.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", setting.Database.Name))
		assert.NoError(t, err)
		db.Close()

		db, err = sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s)/%s?multiStatements=true",
			setting.Database.User, setting.Database.Passwd, setting.Database.Host, setting.Database.Name))
		assert.NoError(t, err)
		defer db.Close()

		_, err = db.Exec(data)
		assert.NoError(t, err)
		db.Close()

	case setting.Database.UsePostgreSQL:
		db, err := sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@%s/?sslmode=%s",
			setting.Database.User, setting.Database.Passwd, setting.Database.Host, setting.Database.SSLMode))
		assert.NoError(t, err)
		defer db.Close()

		_, err = db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", setting.Database.Name))
		assert.NoError(t, err)

		_, err = db.Exec(fmt.Sprintf("CREATE DATABASE %s", setting.Database.Name))
		assert.NoError(t, err)
		db.Close()

		// Check if we need to setup a specific schema
		if len(setting.Database.Schema) != 0 {
			db, err = sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=%s",
				setting.Database.User, setting.Database.Passwd, setting.Database.Host, setting.Database.Name, setting.Database.SSLMode))
			if !assert.NoError(t, err) {
				return false
			}
			defer db.Close()

			schrows, err := db.Query(fmt.Sprintf("SELECT 1 FROM information_schema.schemata WHERE schema_name = '%s'", setting.Database.Schema))
			if !assert.NoError(t, err) || !assert.NotEmpty(t, schrows) {
				return false
			}

			if !schrows.Next() {
				// Create and setup a DB schema
				_, err = db.Exec(fmt.Sprintf("CREATE SCHEMA %s", setting.Database.Schema))
				assert.NoError(t, err)
			}
			schrows.Close()

			// Make the user's default search path the created schema; this will affect new connections
			_, err = db.Exec(fmt.Sprintf(`ALTER USER "%s" SET search_path = %s`, setting.Database.User, setting.Database.Schema))
			assert.NoError(t, err)

			db.Close()
		}

		db, err = sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=%s",
			setting.Database.User, setting.Database.Passwd, setting.Database.Host, setting.Database.Name, setting.Database.SSLMode))
		assert.NoError(t, err)
		defer db.Close()

		_, err = db.Exec(data)
		assert.NoError(t, err)
		db.Close()

	case setting.Database.UseMSSQL:
		host, port := setting.ParseMSSQLHostPort(setting.Database.Host)
		db, err := sql.Open("mssql", fmt.Sprintf("server=%s; port=%s; database=%s; user id=%s; password=%s;",
			host, port, "master", setting.Database.User, setting.Database.Passwd))
		assert.NoError(t, err)
		defer db.Close()

		_, err = db.Exec("DROP DATABASE IF EXISTS [gitea]")
		assert.NoError(t, err)

		statements := strings.Split(data, "\nGO\n")
		for _, statement := range statements {
			if len(statement) > 5 && statement[:5] == "USE [" {
				dbname := statement[5 : len(statement)-1]
				db.Close()
				db, err = sql.Open("mssql", fmt.Sprintf("server=%s; port=%s; database=%s; user id=%s; password=%s;",
					host, port, dbname, setting.Database.User, setting.Database.Passwd))
				assert.NoError(t, err)
				defer db.Close()
			}
			_, err = db.Exec(statement)
			assert.NoError(t, err, "Failure whilst running: %s\nError: %v", statement, err)
		}
		db.Close()
	}
	return true
}

func wrappedMigrate(x *xorm.Engine) error {
	currentEngine = x
	return migrations.Migrate(x)
}

func doMigrationTest(t *testing.T, version string) {
	defer integrations.PrintCurrentTest(t)()
	integrations.Printf("Performing migration test for %s version: %s\n", setting.Database.Type, version)
	if !restoreOldDB(t, version) {
		return
	}

	setting.NewXORMLogService(false)

	err := models.NewEngine(context.Background(), wrappedMigrate)
	assert.NoError(t, err)
	currentEngine.Close()

	beans, _ := models.NamesToBean()

	err = models.NewEngine(context.Background(), func(x *xorm.Engine) error {
		currentEngine = x
		return migrations.RecreateTables(beans...)(x)
	})
	assert.NoError(t, err)
	currentEngine.Close()

	// We do this a second time to ensure that there is not a problem with retained indices
	err = models.NewEngine(context.Background(), func(x *xorm.Engine) error {
		currentEngine = x
		return migrations.RecreateTables(beans...)(x)
	})
	assert.NoError(t, err)

	currentEngine.Close()
}

func TestMigrations(t *testing.T) {
	defer initMigrationTest(t)()

	dialect := setting.Database.Type
	versions, err := availableVersions()
	assert.NoError(t, err)

	if len(versions) == 0 {
		integrations.Printf("No old database versions available to migration test for %s\n", dialect)
		return
	}

	integrations.Printf("Preparing to test %d migrations for %s\n", len(versions), dialect)
	for _, version := range versions {
		t.Run(fmt.Sprintf("Migrate-%s-%s", dialect, version), func(t *testing.T) {
			doMigrationTest(t, version)
		})

	}
}
