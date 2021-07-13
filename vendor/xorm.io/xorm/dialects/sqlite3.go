// Copyright 2015 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dialects

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"xorm.io/xorm/core"
	"xorm.io/xorm/schemas"
)

var (
	sqlite3ReservedWords = map[string]bool{
		"ABORT":             true,
		"ACTION":            true,
		"ADD":               true,
		"AFTER":             true,
		"ALL":               true,
		"ALTER":             true,
		"ANALYZE":           true,
		"AND":               true,
		"AS":                true,
		"ASC":               true,
		"ATTACH":            true,
		"AUTOINCREMENT":     true,
		"BEFORE":            true,
		"BEGIN":             true,
		"BETWEEN":           true,
		"BY":                true,
		"CASCADE":           true,
		"CASE":              true,
		"CAST":              true,
		"CHECK":             true,
		"COLLATE":           true,
		"COLUMN":            true,
		"COMMIT":            true,
		"CONFLICT":          true,
		"CONSTRAINT":        true,
		"CREATE":            true,
		"CROSS":             true,
		"CURRENT_DATE":      true,
		"CURRENT_TIME":      true,
		"CURRENT_TIMESTAMP": true,
		"DATABASE":          true,
		"DEFAULT":           true,
		"DEFERRABLE":        true,
		"DEFERRED":          true,
		"DELETE":            true,
		"DESC":              true,
		"DETACH":            true,
		"DISTINCT":          true,
		"DROP":              true,
		"EACH":              true,
		"ELSE":              true,
		"END":               true,
		"ESCAPE":            true,
		"EXCEPT":            true,
		"EXCLUSIVE":         true,
		"EXISTS":            true,
		"EXPLAIN":           true,
		"FAIL":              true,
		"FOR":               true,
		"FOREIGN":           true,
		"FROM":              true,
		"FULL":              true,
		"GLOB":              true,
		"GROUP":             true,
		"HAVING":            true,
		"IF":                true,
		"IGNORE":            true,
		"IMMEDIATE":         true,
		"IN":                true,
		"INDEX":             true,
		"INDEXED":           true,
		"INITIALLY":         true,
		"INNER":             true,
		"INSERT":            true,
		"INSTEAD":           true,
		"INTERSECT":         true,
		"INTO":              true,
		"IS":                true,
		"ISNULL":            true,
		"JOIN":              true,
		"KEY":               true,
		"LEFT":              true,
		"LIKE":              true,
		"LIMIT":             true,
		"MATCH":             true,
		"NATURAL":           true,
		"NO":                true,
		"NOT":               true,
		"NOTNULL":           true,
		"NULL":              true,
		"OF":                true,
		"OFFSET":            true,
		"ON":                true,
		"OR":                true,
		"ORDER":             true,
		"OUTER":             true,
		"PLAN":              true,
		"PRAGMA":            true,
		"PRIMARY":           true,
		"QUERY":             true,
		"RAISE":             true,
		"RECURSIVE":         true,
		"REFERENCES":        true,
		"REGEXP":            true,
		"REINDEX":           true,
		"RELEASE":           true,
		"RENAME":            true,
		"REPLACE":           true,
		"RESTRICT":          true,
		"RIGHT":             true,
		"ROLLBACK":          true,
		"ROW":               true,
		"SAVEPOINT":         true,
		"SELECT":            true,
		"SET":               true,
		"TABLE":             true,
		"TEMP":              true,
		"TEMPORARY":         true,
		"THEN":              true,
		"TO":                true,
		"TRANSACTI":         true,
		"TRIGGER":           true,
		"UNION":             true,
		"UNIQUE":            true,
		"UPDATE":            true,
		"USING":             true,
		"VACUUM":            true,
		"VALUES":            true,
		"VIEW":              true,
		"VIRTUAL":           true,
		"WHEN":              true,
		"WHERE":             true,
		"WITH":              true,
		"WITHOUT":           true,
	}

	sqlite3Quoter = schemas.Quoter{
		Prefix:     '`',
		Suffix:     '`',
		IsReserved: schemas.AlwaysReserve,
	}
)

type sqlite3 struct {
	Base
}

func (db *sqlite3) Init(uri *URI) error {
	db.quoter = sqlite3Quoter
	return db.Base.Init(db, uri)
}

func (db *sqlite3) Version(ctx context.Context, queryer core.Queryer) (*schemas.Version, error) {
	rows, err := queryer.QueryContext(ctx, "SELECT sqlite_version()")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var version string
	if !rows.Next() {
		return nil, errors.New("Unknow version")
	}

	if err := rows.Scan(&version); err != nil {
		return nil, err
	}
	return &schemas.Version{
		Number:  version,
		Edition: "sqlite",
	}, nil
}

func (db *sqlite3) SetQuotePolicy(quotePolicy QuotePolicy) {
	switch quotePolicy {
	case QuotePolicyNone:
		var q = sqlite3Quoter
		q.IsReserved = schemas.AlwaysNoReserve
		db.quoter = q
	case QuotePolicyReserved:
		var q = sqlite3Quoter
		q.IsReserved = db.IsReserved
		db.quoter = q
	case QuotePolicyAlways:
		fallthrough
	default:
		db.quoter = sqlite3Quoter
	}
}

func (db *sqlite3) SQLType(c *schemas.Column) string {
	switch t := c.SQLType.Name; t {
	case schemas.Bool:
		if c.Default == "true" {
			c.Default = "1"
		} else if c.Default == "false" {
			c.Default = "0"
		}
		return schemas.Integer
	case schemas.Date, schemas.DateTime, schemas.TimeStamp, schemas.Time:
		return schemas.DateTime
	case schemas.TimeStampz:
		return schemas.Text
	case schemas.Char, schemas.Varchar, schemas.NVarchar, schemas.TinyText,
		schemas.Text, schemas.MediumText, schemas.LongText, schemas.Json:
		return schemas.Text
	case schemas.Bit, schemas.TinyInt, schemas.SmallInt, schemas.MediumInt, schemas.Int, schemas.Integer, schemas.BigInt,
		schemas.UnsignedBigInt, schemas.UnsignedInt:
		return schemas.Integer
	case schemas.Float, schemas.Double, schemas.Real:
		return schemas.Real
	case schemas.Decimal, schemas.Numeric:
		return schemas.Numeric
	case schemas.TinyBlob, schemas.Blob, schemas.MediumBlob, schemas.LongBlob, schemas.Bytea, schemas.Binary, schemas.VarBinary:
		return schemas.Blob
	case schemas.Serial, schemas.BigSerial:
		c.IsPrimaryKey = true
		c.IsAutoIncrement = true
		c.Nullable = false
		return schemas.Integer
	default:
		return t
	}
}

func (db *sqlite3) FormatBytes(bs []byte) string {
	return fmt.Sprintf("X'%x'", bs)
}

func (db *sqlite3) IsReserved(name string) bool {
	_, ok := sqlite3ReservedWords[strings.ToUpper(name)]
	return ok
}

func (db *sqlite3) AutoIncrStr() string {
	return "AUTOINCREMENT"
}

func (db *sqlite3) IndexCheckSQL(tableName, idxName string) (string, []interface{}) {
	args := []interface{}{idxName}
	return "SELECT name FROM sqlite_master WHERE type='index' and name = ?", args
}

func (db *sqlite3) IsTableExist(queryer core.Queryer, ctx context.Context, tableName string) (bool, error) {
	return db.HasRecords(queryer, ctx, "SELECT name FROM sqlite_master WHERE type='table' and name = ?", tableName)
}

func (db *sqlite3) DropIndexSQL(tableName string, index *schemas.Index) string {
	// var unique string
	idxName := index.Name

	if !strings.HasPrefix(idxName, "UQE_") &&
		!strings.HasPrefix(idxName, "IDX_") {
		if index.Type == schemas.UniqueType {
			idxName = fmt.Sprintf("UQE_%v_%v", tableName, index.Name)
		} else {
			idxName = fmt.Sprintf("IDX_%v_%v", tableName, index.Name)
		}
	}
	return fmt.Sprintf("DROP INDEX %v", db.Quoter().Quote(idxName))
}

func (db *sqlite3) CreateTableSQL(table *schemas.Table, tableName string) ([]string, bool) {
	var sql string
	sql = "CREATE TABLE IF NOT EXISTS "
	if tableName == "" {
		tableName = table.Name
	}

	quoter := db.Quoter()
	sql += quoter.Quote(tableName)
	sql += " ("

	if len(table.ColumnsSeq()) > 0 {
		pkList := table.PrimaryKeys

		for _, colName := range table.ColumnsSeq() {
			col := table.GetColumn(colName)
			s, _ := ColumnString(db, col, col.IsPrimaryKey && len(pkList) == 1)
			sql += s
			sql = strings.TrimSpace(sql)
			sql += ", "
		}

		if len(pkList) > 1 {
			sql += "PRIMARY KEY ( "
			sql += quoter.Join(pkList, ",")
			sql += " ), "
		}

		sql = sql[:len(sql)-2]
	}
	sql += ")"

	return []string{sql}, true
}

func (db *sqlite3) ForUpdateSQL(query string) string {
	return query
}

func (db *sqlite3) IsColumnExist(queryer core.Queryer, ctx context.Context, tableName, colName string) (bool, error) {
	query := "SELECT * FROM " + tableName + " LIMIT 0"
	rows, err := queryer.QueryContext(ctx, query)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return false, err
	}

	for _, col := range cols {
		if strings.EqualFold(col, colName) {
			return true, nil
		}
	}

	return false, nil
}

// splitColStr splits a sqlite col strings as fields
func splitColStr(colStr string) []string {
	colStr = strings.TrimSpace(colStr)
	var results = make([]string, 0, 10)
	var lastIdx int
	var hasC, hasQuote bool
	for i, c := range colStr {
		if c == ' ' && !hasQuote {
			if hasC {
				results = append(results, colStr[lastIdx:i])
				hasC = false
			}
		} else {
			if c == '\'' {
				hasQuote = !hasQuote
			}
			if !hasC {
				lastIdx = i
			}
			hasC = true
			if i == len(colStr)-1 {
				results = append(results, colStr[lastIdx:i+1])
			}
		}
	}
	return results
}

func parseString(colStr string) (*schemas.Column, error) {
	fields := splitColStr(colStr)
	col := new(schemas.Column)
	col.Indexes = make(map[string]int)
	col.Nullable = true
	col.DefaultIsEmpty = true

	for idx, field := range fields {
		if idx == 0 {
			col.Name = strings.Trim(strings.Trim(field, "`[] "), `"`)
			continue
		} else if idx == 1 {
			col.SQLType = schemas.SQLType{Name: field, DefaultLength: 0, DefaultLength2: 0}
			continue
		}
		switch field {
		case "PRIMARY":
			col.IsPrimaryKey = true
		case "AUTOINCREMENT":
			col.IsAutoIncrement = true
		case "NULL":
			if fields[idx-1] == "NOT" {
				col.Nullable = false
			} else {
				col.Nullable = true
			}
		case "DEFAULT":
			col.Default = fields[idx+1]
			col.DefaultIsEmpty = false
		}
	}
	return col, nil
}

func (db *sqlite3) GetColumns(queryer core.Queryer, ctx context.Context, tableName string) ([]string, map[string]*schemas.Column, error) {
	args := []interface{}{tableName}
	s := "SELECT sql FROM sqlite_master WHERE type='table' and name = ?"

	rows, err := queryer.QueryContext(ctx, s, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var name string
	for rows.Next() {
		err = rows.Scan(&name)
		if err != nil {
			return nil, nil, err
		}
		break
	}

	if name == "" {
		return nil, nil, errors.New("no table named " + tableName)
	}

	nStart := strings.Index(name, "(")
	nEnd := strings.LastIndex(name, ")")
	reg := regexp.MustCompile(`[^\(,\)]*(\([^\(]*\))?`)
	colCreates := reg.FindAllString(name[nStart+1:nEnd], -1)
	cols := make(map[string]*schemas.Column)
	colSeq := make([]string, 0)

	for _, colStr := range colCreates {
		reg = regexp.MustCompile(`,\s`)
		colStr = reg.ReplaceAllString(colStr, ",")
		if strings.HasPrefix(strings.TrimSpace(colStr), "PRIMARY KEY") {
			parts := strings.Split(strings.TrimSpace(colStr), "(")
			if len(parts) == 2 {
				pkCols := strings.Split(strings.TrimRight(strings.TrimSpace(parts[1]), ")"), ",")
				for _, pk := range pkCols {
					if col, ok := cols[strings.Trim(strings.TrimSpace(pk), "`")]; ok {
						col.IsPrimaryKey = true
					}
				}
			}
			continue
		}

		col, err := parseString(colStr)
		if err != nil {
			return colSeq, cols, err
		}

		cols[col.Name] = col
		colSeq = append(colSeq, col.Name)
	}
	return colSeq, cols, nil
}

func (db *sqlite3) GetTables(queryer core.Queryer, ctx context.Context) ([]*schemas.Table, error) {
	args := []interface{}{}
	s := "SELECT name FROM sqlite_master WHERE type='table'"

	rows, err := queryer.QueryContext(ctx, s, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tables := make([]*schemas.Table, 0)
	for rows.Next() {
		table := schemas.NewEmptyTable()
		err = rows.Scan(&table.Name)
		if err != nil {
			return nil, err
		}
		if table.Name == "sqlite_sequence" {
			continue
		}
		tables = append(tables, table)
	}
	return tables, nil
}

func (db *sqlite3) GetIndexes(queryer core.Queryer, ctx context.Context, tableName string) (map[string]*schemas.Index, error) {
	args := []interface{}{tableName}
	s := "SELECT sql FROM sqlite_master WHERE type='index' and tbl_name = ?"

	rows, err := queryer.QueryContext(ctx, s, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	indexes := make(map[string]*schemas.Index, 0)
	for rows.Next() {
		var tmpSQL sql.NullString
		err = rows.Scan(&tmpSQL)
		if err != nil {
			return nil, err
		}

		if !tmpSQL.Valid {
			continue
		}
		sql := tmpSQL.String

		index := new(schemas.Index)
		nNStart := strings.Index(sql, "INDEX")
		nNEnd := strings.Index(sql, "ON")
		if nNStart == -1 || nNEnd == -1 {
			continue
		}

		indexName := strings.Trim(strings.TrimSpace(sql[nNStart+6:nNEnd]), "`[]'\"")
		var isRegular bool
		if strings.HasPrefix(indexName, "IDX_"+tableName) || strings.HasPrefix(indexName, "UQE_"+tableName) {
			index.Name = indexName[5+len(tableName):]
			isRegular = true
		} else {
			index.Name = indexName
		}

		if strings.HasPrefix(sql, "CREATE UNIQUE INDEX") {
			index.Type = schemas.UniqueType
		} else {
			index.Type = schemas.IndexType
		}

		nStart := strings.Index(sql, "(")
		nEnd := strings.Index(sql, ")")
		colIndexes := strings.Split(sql[nStart+1:nEnd], ",")

		index.Cols = make([]string, 0)
		for _, col := range colIndexes {
			index.Cols = append(index.Cols, strings.Trim(col, "` []"))
		}
		index.IsRegular = isRegular
		indexes[index.Name] = index
	}

	return indexes, nil
}

func (db *sqlite3) Filters() []Filter {
	return []Filter{}
}

type sqlite3Driver struct {
}

func (p *sqlite3Driver) Parse(driverName, dataSourceName string) (*URI, error) {
	if strings.Contains(dataSourceName, "?") {
		dataSourceName = dataSourceName[:strings.Index(dataSourceName, "?")]
	}

	return &URI{DBType: schemas.SQLITE, DBName: dataSourceName}, nil
}
