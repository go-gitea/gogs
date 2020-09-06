// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

var (
	// SupportedDatabases includes all supported databases type
	SupportedDatabases = []string{"MySQL", "PostgreSQL", "MSSQL"}
	dbTypes            = map[string]string{"MySQL": "mysql", "PostgreSQL": "postgres", "MSSQL": "mssql", "SQLite3": "sqlite3"}

	// EnableSQLite3 use SQLite3, set by build flag
	EnableSQLite3 bool

	// Database holds the database settings
	Database = struct {
		Type              string
		Host              string
		Name              string
		User              string
		Passwd            string
		Schema            string
		SSLMode           string
		Path              string
		LogSQL            bool
		Charset           string
		Timeout           int // seconds
		UseSQLite3        bool
		UseMySQL          bool
		UseMSSQL          bool
		UsePostgreSQL     bool
		DBConnectRetries  int
		DBConnectBackoff  time.Duration
		MaxIdleConns      int
		MaxOpenConns      int
		ConnMaxLifetime   time.Duration
		IterateBufferSize int
		TableNamePrefix   string
	}{
		Timeout: 500,
	}
)

// GetDBTypeByName returns the database type as it defined on XORM according the given name
func GetDBTypeByName(name string) string {
	return dbTypes[name]
}

// InitDBConfig loads the database settings
func InitDBConfig() {
	sec := Cfg.Section("database")
	Database.Type = sec.Key("DB_TYPE").String()
	defaultCharset := "utf8"
	switch Database.Type {
	case "sqlite3":
		Database.UseSQLite3 = true
	case "mysql":
		Database.UseMySQL = true
		defaultCharset = "utf8mb4"
	case "postgres":
		Database.UsePostgreSQL = true
	case "mssql":
		Database.UseMSSQL = true
	}
	Database.Host = sec.Key("HOST").String()
	Database.Name = sec.Key("NAME").String()
	Database.User = sec.Key("USER").String()
	if len(Database.Passwd) == 0 {
		Database.Passwd = sec.Key("PASSWD").String()
	}
	Database.Schema = sec.Key("SCHEMA").String()
	Database.SSLMode = sec.Key("SSL_MODE").MustString("disable")
	Database.Charset = sec.Key("CHARSET").In(defaultCharset, []string{"utf8", "utf8mb4"})
	Database.Path = sec.Key("PATH").MustString(filepath.Join(AppDataPath, "gitea.db"))
	Database.Timeout = sec.Key("SQLITE_TIMEOUT").MustInt(500)
	Database.MaxIdleConns = sec.Key("MAX_IDLE_CONNS").MustInt(2)
	if Database.UseMySQL {
		Database.ConnMaxLifetime = sec.Key("CONN_MAX_LIFE_TIME").MustDuration(3 * time.Second)
	} else {
		Database.ConnMaxLifetime = sec.Key("CONN_MAX_LIFE_TIME").MustDuration(0)
	}
	Database.MaxOpenConns = sec.Key("MAX_OPEN_CONNS").MustInt(0)

	Database.IterateBufferSize = sec.Key("ITERATE_BUFFER_SIZE").MustInt(50)
	Database.LogSQL = sec.Key("LOG_SQL").MustBool(true)
	Database.DBConnectRetries = sec.Key("DB_RETRIES").MustInt(10)
	Database.DBConnectBackoff = sec.Key("DB_RETRY_BACKOFF").MustDuration(3 * time.Second)
	Database.TableNamePrefix = sec.Key("DB_TABLE_NAME_PREFIX").MustString("")
}

// DBConnStr returns database connection string
func DBConnStr() (string, error) {
	connStr := ""
	var Param = "?"
	if strings.Contains(Database.Name, Param) {
		Param = "&"
	}
	switch Database.Type {
	case "mysql":
		connType := "tcp"
		if len(Database.Host) > 0 && Database.Host[0] == '/' { // looks like a unix socket
			connType = "unix"
		}
		tls := Database.SSLMode
		if tls == "disable" { // allow (Postgres-inspired) default value to work in MySQL
			tls = "false"
		}
		connStr = fmt.Sprintf("%s:%s@%s(%s)/%s%scharset=%s&parseTime=true&tls=%s",
			Database.User, Database.Passwd, connType, Database.Host, Database.Name, Param, Database.Charset, tls)
	case "postgres":
		connStr = getPostgreSQLConnectionString(Database.Host, Database.User, Database.Passwd, Database.Name, Param, Database.SSLMode)
	case "mssql":
		host, port := ParseMSSQLHostPort(Database.Host)
		connStr = fmt.Sprintf("server=%s; port=%s; database=%s; user id=%s; password=%s;", host, port, Database.Name, Database.User, Database.Passwd)
	case "sqlite3":
		if !EnableSQLite3 {
			return "", errors.New("this binary version does not build support for SQLite3")
		}
		if err := os.MkdirAll(path.Dir(Database.Path), os.ModePerm); err != nil {
			return "", fmt.Errorf("Failed to create directories: %v", err)
		}
		connStr = fmt.Sprintf("file:%s?cache=shared&mode=rwc&_busy_timeout=%d&_txlock=immediate", Database.Path, Database.Timeout)
	default:
		return "", fmt.Errorf("Unknown database type: %s", Database.Type)
	}

	return connStr, nil
}

// parsePostgreSQLHostPort parses given input in various forms defined in
// https://www.postgresql.org/docs/current/static/libpq-connect.html#LIBPQ-CONNSTRING
// and returns proper host and port number.
func parsePostgreSQLHostPort(info string) (string, string) {
	host, port := "127.0.0.1", "5432"
	if strings.Contains(info, ":") && !strings.HasSuffix(info, "]") {
		idx := strings.LastIndex(info, ":")
		host = info[:idx]
		port = info[idx+1:]
	} else if len(info) > 0 {
		host = info
	}
	return host, port
}

func getPostgreSQLConnectionString(dbHost, dbUser, dbPasswd, dbName, dbParam, dbsslMode string) (connStr string) {
	host, port := parsePostgreSQLHostPort(dbHost)
	if host[0] == '/' { // looks like a unix socket
		connStr = fmt.Sprintf("postgres://%s:%s@:%s/%s%ssslmode=%s&host=%s",
			url.PathEscape(dbUser), url.PathEscape(dbPasswd), port, dbName, dbParam, dbsslMode, host)
	} else {
		connStr = fmt.Sprintf("postgres://%s:%s@%s:%s/%s%ssslmode=%s",
			url.PathEscape(dbUser), url.PathEscape(dbPasswd), host, port, dbName, dbParam, dbsslMode)
	}
	return
}

// ParseMSSQLHostPort splits the host into host and port
func ParseMSSQLHostPort(info string) (string, string) {
	host, port := "127.0.0.1", "0"
	if strings.Contains(info, ":") {
		host = strings.Split(info, ":")[0]
		port = strings.Split(info, ":")[1]
	} else if strings.Contains(info, ",") {
		host = strings.Split(info, ",")[0]
		port = strings.TrimSpace(strings.Split(info, ",")[1])
	} else if len(info) > 0 {
		host = info
	}
	return host, port
}
