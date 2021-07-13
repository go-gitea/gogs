// Copyright 2019 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dialects

import (
	"context"
	"fmt"
	"strings"
	"time"

	"xorm.io/xorm/core"
	"xorm.io/xorm/schemas"
)

// URI represents an uri to visit database
type URI struct {
	DBType  schemas.DBType
	Proto   string
	Host    string
	Port    string
	DBName  string
	User    string
	Passwd  string
	Charset string
	Laddr   string
	Raddr   string
	Timeout time.Duration
	Schema  string
}

// SetSchema set schema
func (uri *URI) SetSchema(schema string) {
	// hack me
	if uri.DBType == schemas.POSTGRES {
		uri.Schema = strings.TrimSpace(schema)
	}
}

// Dialect represents a kind of database
type Dialect interface {
	Init(*URI) error
	URI() *URI
	SQLType(*schemas.Column) string
	FormatBytes(b []byte) string
	Version(ctx context.Context, queryer core.Queryer) (*schemas.Version, error)

	IsReserved(string) bool
	Quoter() schemas.Quoter
	SetQuotePolicy(quotePolicy QuotePolicy)

	AutoIncrStr() string

	GetIndexes(queryer core.Queryer, ctx context.Context, tableName string) (map[string]*schemas.Index, error)
	IndexCheckSQL(tableName, idxName string) (string, []interface{})
	CreateIndexSQL(tableName string, index *schemas.Index) string
	DropIndexSQL(tableName string, index *schemas.Index) string

	GetTables(queryer core.Queryer, ctx context.Context) ([]*schemas.Table, error)
	IsTableExist(queryer core.Queryer, ctx context.Context, tableName string) (bool, error)
	CreateTableSQL(table *schemas.Table, tableName string) ([]string, bool)
	DropTableSQL(tableName string) (string, bool)

	GetColumns(queryer core.Queryer, ctx context.Context, tableName string) ([]string, map[string]*schemas.Column, error)
	IsColumnExist(queryer core.Queryer, ctx context.Context, tableName string, colName string) (bool, error)
	AddColumnSQL(tableName string, col *schemas.Column) string
	ModifyColumnSQL(tableName string, col *schemas.Column) string

	ForUpdateSQL(query string) string

	Filters() []Filter
	SetParams(params map[string]string)
}

// Base represents a basic dialect and all real dialects could embed this struct
type Base struct {
	dialect Dialect
	uri     *URI
	quoter  schemas.Quoter
}

// Quoter returns the current database Quoter
func (db *Base) Quoter() schemas.Quoter {
	return db.quoter
}

// Init initialize the dialect
func (db *Base) Init(dialect Dialect, uri *URI) error {
	db.dialect, db.uri = dialect, uri
	return nil
}

// URI returns the uri of database
func (db *Base) URI() *URI {
	return db.uri
}

// FormatBytes formats bytes
func (db *Base) FormatBytes(bs []byte) string {
	return fmt.Sprintf("0x%x", bs)
}

// DropTableSQL returns drop table SQL
func (db *Base) DropTableSQL(tableName string) (string, bool) {
	quote := db.dialect.Quoter().Quote
	return fmt.Sprintf("DROP TABLE IF EXISTS %s", quote(tableName)), true
}

// HasRecords returns true if the SQL has records returned
func (db *Base) HasRecords(queryer core.Queryer, ctx context.Context, query string, args ...interface{}) (bool, error) {
	rows, err := queryer.QueryContext(ctx, query, args...)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	if rows.Next() {
		return true, nil
	}
	return false, nil
}

// IsColumnExist returns true if the column of the table exist
func (db *Base) IsColumnExist(queryer core.Queryer, ctx context.Context, tableName, colName string) (bool, error) {
	quote := db.dialect.Quoter().Quote
	query := fmt.Sprintf(
		"SELECT %v FROM %v.%v WHERE %v = ? AND %v = ? AND %v = ?",
		quote("COLUMN_NAME"),
		quote("INFORMATION_SCHEMA"),
		quote("COLUMNS"),
		quote("TABLE_SCHEMA"),
		quote("TABLE_NAME"),
		quote("COLUMN_NAME"),
	)
	return db.HasRecords(queryer, ctx, query, db.uri.DBName, tableName, colName)
}

// AddColumnSQL returns a SQL to add a column
func (db *Base) AddColumnSQL(tableName string, col *schemas.Column) string {
	s, _ := ColumnString(db.dialect, col, true)
	return fmt.Sprintf("ALTER TABLE %v ADD %v", db.dialect.Quoter().Quote(tableName), s)
}

// CreateIndexSQL returns a SQL to create index
func (db *Base) CreateIndexSQL(tableName string, index *schemas.Index) string {
	quoter := db.dialect.Quoter()
	var unique string
	var idxName string
	if index.Type == schemas.UniqueType {
		unique = " UNIQUE"
	}
	idxName = index.XName(tableName)
	return fmt.Sprintf("CREATE%s INDEX %v ON %v (%v)", unique,
		quoter.Quote(idxName), quoter.Quote(tableName),
		quoter.Join(index.Cols, ","))
}

// DropIndexSQL returns a SQL to drop index
func (db *Base) DropIndexSQL(tableName string, index *schemas.Index) string {
	quote := db.dialect.Quoter().Quote
	var name string
	if index.IsRegular {
		name = index.XName(tableName)
	} else {
		name = index.Name
	}
	return fmt.Sprintf("DROP INDEX %v ON %s", quote(name), quote(tableName))
}

// ModifyColumnSQL returns a SQL to modify SQL
func (db *Base) ModifyColumnSQL(tableName string, col *schemas.Column) string {
	s, _ := ColumnString(db.dialect, col, false)
	return fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s", tableName, s)
}

// ForUpdateSQL returns for updateSQL
func (db *Base) ForUpdateSQL(query string) string {
	return query + " FOR UPDATE"
}

// SetParams set params
func (db *Base) SetParams(params map[string]string) {
}

var (
	dialects = map[string]func() Dialect{}
)

// RegisterDialect register database dialect
func RegisterDialect(dbName schemas.DBType, dialectFunc func() Dialect) {
	if dialectFunc == nil {
		panic("core: Register dialect is nil")
	}
	dialects[strings.ToLower(string(dbName))] = dialectFunc // !nashtsai! allow override dialect
}

// QueryDialect query if registered database dialect
func QueryDialect(dbName schemas.DBType) Dialect {
	if d, ok := dialects[strings.ToLower(string(dbName))]; ok {
		return d()
	}
	return nil
}

func regDrvsNDialects() bool {
	providedDrvsNDialects := map[string]struct {
		dbType     schemas.DBType
		getDriver  func() Driver
		getDialect func() Dialect
	}{
		"mssql":    {"mssql", func() Driver { return &odbcDriver{} }, func() Dialect { return &mssql{} }},
		"odbc":     {"mssql", func() Driver { return &odbcDriver{} }, func() Dialect { return &mssql{} }}, // !nashtsai! TODO change this when supporting MS Access
		"mysql":    {"mysql", func() Driver { return &mysqlDriver{} }, func() Dialect { return &mysql{} }},
		"mymysql":  {"mysql", func() Driver { return &mymysqlDriver{} }, func() Dialect { return &mysql{} }},
		"postgres": {"postgres", func() Driver { return &pqDriver{} }, func() Dialect { return &postgres{} }},
		"pgx":      {"postgres", func() Driver { return &pqDriverPgx{} }, func() Dialect { return &postgres{} }},
		"sqlite3":  {"sqlite3", func() Driver { return &sqlite3Driver{} }, func() Dialect { return &sqlite3{} }},
		"sqlite":   {"sqlite3", func() Driver { return &sqlite3Driver{} }, func() Dialect { return &sqlite3{} }},
		"oci8":     {"oracle", func() Driver { return &oci8Driver{} }, func() Dialect { return &oracle{} }},
		"godror":   {"oracle", func() Driver { return &godrorDriver{} }, func() Dialect { return &oracle{} }},
	}

	for driverName, v := range providedDrvsNDialects {
		if driver := QueryDriver(driverName); driver == nil {
			RegisterDriver(driverName, v.getDriver())
			RegisterDialect(v.dbType, v.getDialect)
		}
	}
	return true
}

func init() {
	regDrvsNDialects()
}

// ColumnString generate column description string according dialect
func ColumnString(dialect Dialect, col *schemas.Column, includePrimaryKey bool) (string, error) {
	bd := strings.Builder{}

	if err := dialect.Quoter().QuoteTo(&bd, col.Name); err != nil {
		return "", err
	}

	if err := bd.WriteByte(' '); err != nil {
		return "", err
	}

	if _, err := bd.WriteString(dialect.SQLType(col)); err != nil {
		return "", err
	}

	if err := bd.WriteByte(' '); err != nil {
		return "", err
	}

	if includePrimaryKey && col.IsPrimaryKey {
		if _, err := bd.WriteString("PRIMARY KEY "); err != nil {
			return "", err
		}

		if col.IsAutoIncrement {
			if _, err := bd.WriteString(dialect.AutoIncrStr()); err != nil {
				return "", err
			}
			if err := bd.WriteByte(' '); err != nil {
				return "", err
			}
		}
	}

	if col.Default != "" {
		if _, err := bd.WriteString("DEFAULT "); err != nil {
			return "", err
		}
		if _, err := bd.WriteString(col.Default); err != nil {
			return "", err
		}
		if err := bd.WriteByte(' '); err != nil {
			return "", err
		}
	}

	if col.Nullable {
		if _, err := bd.WriteString("NULL "); err != nil {
			return "", err
		}
	} else {
		if _, err := bd.WriteString("NOT NULL "); err != nil {
			return "", err
		}
	}

	return bd.String(), nil
}
