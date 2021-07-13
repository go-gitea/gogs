// Copyright 2015 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dialects

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"xorm.io/xorm/core"
	"xorm.io/xorm/schemas"
)

var (
	mysqlReservedWords = map[string]bool{
		"ADD":               true,
		"ALL":               true,
		"ALTER":             true,
		"ANALYZE":           true,
		"AND":               true,
		"AS":                true,
		"ASC":               true,
		"ASENSITIVE":        true,
		"BEFORE":            true,
		"BETWEEN":           true,
		"BIGINT":            true,
		"BINARY":            true,
		"BLOB":              true,
		"BOTH":              true,
		"BY":                true,
		"CALL":              true,
		"CASCADE":           true,
		"CASE":              true,
		"CHANGE":            true,
		"CHAR":              true,
		"CHARACTER":         true,
		"CHECK":             true,
		"COLLATE":           true,
		"COLUMN":            true,
		"CONDITION":         true,
		"CONNECTION":        true,
		"CONSTRAINT":        true,
		"CONTINUE":          true,
		"CONVERT":           true,
		"CREATE":            true,
		"CROSS":             true,
		"CURRENT_DATE":      true,
		"CURRENT_TIME":      true,
		"CURRENT_TIMESTAMP": true,
		"CURRENT_USER":      true,
		"CURSOR":            true,
		"DATABASE":          true,
		"DATABASES":         true,
		"DAY_HOUR":          true,
		"DAY_MICROSECOND":   true,
		"DAY_MINUTE":        true,
		"DAY_SECOND":        true,
		"DEC":               true,
		"DECIMAL":           true,
		"DECLARE":           true,
		"DEFAULT":           true,
		"DELAYED":           true,
		"DELETE":            true,
		"DESC":              true,
		"DESCRIBE":          true,
		"DETERMINISTIC":     true,
		"DISTINCT":          true,
		"DISTINCTROW":       true,
		"DIV":               true,
		"DOUBLE":            true,
		"DROP":              true,
		"DUAL":              true,
		"EACH":              true,
		"ELSE":              true,
		"ELSEIF":            true,
		"ENCLOSED":          true,
		"ESCAPED":           true,
		"EXISTS":            true,
		"EXIT":              true,
		"EXPLAIN":           true,
		"FALSE":             true,
		"FETCH":             true,
		"FLOAT":             true,
		"FLOAT4":            true,
		"FLOAT8":            true,
		"FOR":               true,
		"FORCE":             true,
		"FOREIGN":           true,
		"FROM":              true,
		"FULLTEXT":          true,
		"GOTO":              true,
		"GRANT":             true,
		"GROUP":             true,
		"HAVING":            true,
		"HIGH_PRIORITY":     true,
		"HOUR_MICROSECOND":  true,
		"HOUR_MINUTE":       true,
		"HOUR_SECOND":       true,
		"IF":                true,
		"IGNORE":            true,
		"IN":                true, "INDEX": true,
		"INFILE": true, "INNER": true, "INOUT": true,
		"INSENSITIVE": true, "INSERT": true, "INT": true,
		"INT1": true, "INT2": true, "INT3": true,
		"INT4": true, "INT8": true, "INTEGER": true,
		"INTERVAL": true, "INTO": true, "IS": true,
		"ITERATE": true, "JOIN": true, "KEY": true,
		"KEYS": true, "KILL": true, "LABEL": true,
		"LEADING": true, "LEAVE": true, "LEFT": true,
		"LIKE": true, "LIMIT": true, "LINEAR": true,
		"LINES": true, "LOAD": true, "LOCALTIME": true,
		"LOCALTIMESTAMP": true, "LOCK": true, "LONG": true,
		"LONGBLOB": true, "LONGTEXT": true, "LOOP": true,
		"LOW_PRIORITY": true, "MATCH": true, "MEDIUMBLOB": true,
		"MEDIUMINT": true, "MEDIUMTEXT": true, "MIDDLEINT": true,
		"MINUTE_MICROSECOND": true, "MINUTE_SECOND": true, "MOD": true,
		"MODIFIES": true, "NATURAL": true, "NOT": true,
		"NO_WRITE_TO_BINLOG": true, "NULL": true, "NUMERIC": true,
		"ON	OPTIMIZE": true, "OPTION": true,
		"OPTIONALLY": true, "OR": true, "ORDER": true,
		"OUT": true, "OUTER": true, "OUTFILE": true,
		"PRECISION": true, "PRIMARY": true, "PROCEDURE": true,
		"PURGE": true, "RAID0": true, "RANGE": true,
		"READ": true, "READS": true, "REAL": true,
		"REFERENCES": true, "REGEXP": true, "RELEASE": true,
		"RENAME": true, "REPEAT": true, "REPLACE": true,
		"REQUIRE": true, "RESTRICT": true, "RETURN": true,
		"REVOKE": true, "RIGHT": true, "RLIKE": true,
		"SCHEMA": true, "SCHEMAS": true, "SECOND_MICROSECOND": true,
		"SELECT": true, "SENSITIVE": true, "SEPARATOR": true,
		"SET": true, "SHOW": true, "SMALLINT": true,
		"SPATIAL": true, "SPECIFIC": true, "SQL": true,
		"SQLEXCEPTION": true, "SQLSTATE": true, "SQLWARNING": true,
		"SQL_BIG_RESULT": true, "SQL_CALC_FOUND_ROWS": true, "SQL_SMALL_RESULT": true,
		"SSL": true, "STARTING": true, "STRAIGHT_JOIN": true,
		"TABLE": true, "TERMINATED": true, "THEN": true,
		"TINYBLOB": true, "TINYINT": true, "TINYTEXT": true,
		"TO": true, "TRAILING": true, "TRIGGER": true,
		"TRUE": true, "UNDO": true, "UNION": true,
		"UNIQUE": true, "UNLOCK": true, "UNSIGNED": true,
		"UPDATE": true, "USAGE": true, "USE": true,
		"USING": true, "UTC_DATE": true, "UTC_TIME": true,
		"UTC_TIMESTAMP": true, "VALUES": true, "VARBINARY": true,
		"VARCHAR":      true,
		"VARCHARACTER": true,
		"VARYING":      true,
		"WHEN":         true,
		"WHERE":        true,
		"WHILE":        true,
		"WITH":         true,
		"WRITE":        true,
		"X509":         true,
		"XOR":          true,
		"YEAR_MONTH":   true,
		"ZEROFILL":     true,
	}

	mysqlQuoter = schemas.Quoter{
		Prefix:     '`',
		Suffix:     '`',
		IsReserved: schemas.AlwaysReserve,
	}
)

type mysql struct {
	Base
	net               string
	addr              string
	params            map[string]string
	loc               *time.Location
	timeout           time.Duration
	tls               *tls.Config
	allowAllFiles     bool
	allowOldPasswords bool
	clientFoundRows   bool
	rowFormat         string
}

func (db *mysql) Init(uri *URI) error {
	db.quoter = mysqlQuoter
	return db.Base.Init(db, uri)
}

func (db *mysql) Version(ctx context.Context, queryer core.Queryer) (*schemas.Version, error) {
	rows, err := queryer.QueryContext(ctx, "SELECT @@VERSION")
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

	fields := strings.Split(version, "-")
	if len(fields) == 3 && fields[1] == "TiDB" {
		// 5.7.25-TiDB-v3.0.3
		return &schemas.Version{
			Number:  strings.TrimPrefix(fields[2], "v"),
			Level:   fields[0],
			Edition: fields[1],
		}, nil
	}

	var edition string
	if len(fields) == 2 {
		edition = fields[1]
	}

	return &schemas.Version{
		Number:  fields[0],
		Edition: edition,
	}, nil
}

func (db *mysql) SetParams(params map[string]string) {
	rowFormat, ok := params["rowFormat"]
	if ok {
		var t = strings.ToUpper(rowFormat)
		switch t {
		case "COMPACT":
			fallthrough
		case "REDUNDANT":
			fallthrough
		case "DYNAMIC":
			fallthrough
		case "COMPRESSED":
			db.rowFormat = t
			break
		default:
			break
		}
	}
}

func (db *mysql) SQLType(c *schemas.Column) string {
	var res string
	switch t := c.SQLType.Name; t {
	case schemas.Bool:
		res = schemas.TinyInt
		c.Length = 1
	case schemas.Serial:
		c.IsAutoIncrement = true
		c.IsPrimaryKey = true
		c.Nullable = false
		res = schemas.Int
	case schemas.BigSerial:
		c.IsAutoIncrement = true
		c.IsPrimaryKey = true
		c.Nullable = false
		res = schemas.BigInt
	case schemas.Bytea:
		res = schemas.Blob
	case schemas.TimeStampz:
		res = schemas.Char
		c.Length = 64
	case schemas.Enum: // mysql enum
		res = schemas.Enum
		res += "("
		opts := ""
		for v := range c.EnumOptions {
			opts += fmt.Sprintf(",'%v'", v)
		}
		res += strings.TrimLeft(opts, ",")
		res += ")"
	case schemas.Set: // mysql set
		res = schemas.Set
		res += "("
		opts := ""
		for v := range c.SetOptions {
			opts += fmt.Sprintf(",'%v'", v)
		}
		res += strings.TrimLeft(opts, ",")
		res += ")"
	case schemas.NVarchar:
		res = schemas.Varchar
	case schemas.Uuid:
		res = schemas.Varchar
		c.Length = 40
	case schemas.Json:
		res = schemas.Text
	case schemas.UnsignedInt:
		res = schemas.Int
	case schemas.UnsignedBigInt:
		res = schemas.BigInt
	default:
		res = t
	}

	hasLen1 := (c.Length > 0)
	hasLen2 := (c.Length2 > 0)

	if res == schemas.BigInt && !hasLen1 && !hasLen2 {
		c.Length = 20
		hasLen1 = true
	}

	if hasLen2 {
		res += "(" + strconv.Itoa(c.Length) + "," + strconv.Itoa(c.Length2) + ")"
	} else if hasLen1 {
		res += "(" + strconv.Itoa(c.Length) + ")"
	}

	if c.SQLType.Name == schemas.UnsignedBigInt || c.SQLType.Name == schemas.UnsignedInt {
		res += " UNSIGNED"
	}

	return res
}

func (db *mysql) IsReserved(name string) bool {
	_, ok := mysqlReservedWords[strings.ToUpper(name)]
	return ok
}

func (db *mysql) AutoIncrStr() string {
	return "AUTO_INCREMENT"
}

func (db *mysql) IndexCheckSQL(tableName, idxName string) (string, []interface{}) {
	args := []interface{}{db.uri.DBName, tableName, idxName}
	sql := "SELECT `INDEX_NAME` FROM `INFORMATION_SCHEMA`.`STATISTICS`"
	sql += " WHERE `TABLE_SCHEMA` = ? AND `TABLE_NAME` = ? AND `INDEX_NAME`=?"
	return sql, args
}

func (db *mysql) IsTableExist(queryer core.Queryer, ctx context.Context, tableName string) (bool, error) {
	sql := "SELECT `TABLE_NAME` from `INFORMATION_SCHEMA`.`TABLES` WHERE `TABLE_SCHEMA`=? and `TABLE_NAME`=?"
	return db.HasRecords(queryer, ctx, sql, db.uri.DBName, tableName)
}

func (db *mysql) AddColumnSQL(tableName string, col *schemas.Column) string {
	quoter := db.dialect.Quoter()
	s, _ := ColumnString(db, col, true)
	sql := fmt.Sprintf("ALTER TABLE %v ADD %v", quoter.Quote(tableName), s)
	if len(col.Comment) > 0 {
		sql += " COMMENT '" + col.Comment + "'"
	}
	return sql
}

func (db *mysql) GetColumns(queryer core.Queryer, ctx context.Context, tableName string) ([]string, map[string]*schemas.Column, error) {
	args := []interface{}{db.uri.DBName, tableName}
	alreadyQuoted := "(INSTR(VERSION(), 'maria') > 0 && " +
		"(SUBSTRING_INDEX(VERSION(), '.', 1) > 10 || " +
		"(SUBSTRING_INDEX(VERSION(), '.', 1) = 10 && " +
		"(SUBSTRING_INDEX(SUBSTRING(VERSION(), 4), '.', 1) > 2 || " +
		"(SUBSTRING_INDEX(SUBSTRING(VERSION(), 4), '.', 1) = 2 && " +
		"SUBSTRING_INDEX(SUBSTRING(VERSION(), 6), '-', 1) >= 7)))))"
	s := "SELECT `COLUMN_NAME`, `IS_NULLABLE`, `COLUMN_DEFAULT`, `COLUMN_TYPE`," +
		" `COLUMN_KEY`, `EXTRA`, `COLUMN_COMMENT`, " +
		alreadyQuoted + " AS NEEDS_QUOTE " +
		"FROM `INFORMATION_SCHEMA`.`COLUMNS` WHERE `TABLE_SCHEMA` = ? AND `TABLE_NAME` = ?" +
		" ORDER BY `COLUMNS`.ORDINAL_POSITION"

	rows, err := queryer.QueryContext(ctx, s, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	cols := make(map[string]*schemas.Column)
	colSeq := make([]string, 0)
	for rows.Next() {
		col := new(schemas.Column)
		col.Indexes = make(map[string]int)

		var columnName, nullableStr, colType, colKey, extra, comment string
		var alreadyQuoted, isUnsigned bool
		var colDefault *string
		err = rows.Scan(&columnName, &nullableStr, &colDefault, &colType, &colKey, &extra, &comment, &alreadyQuoted)
		if err != nil {
			return nil, nil, err
		}
		col.Name = strings.Trim(columnName, "` ")
		col.Comment = comment
		if nullableStr == "YES" {
			col.Nullable = true
		}

		if colDefault != nil && (!alreadyQuoted || *colDefault != "NULL") {
			col.Default = *colDefault
			col.DefaultIsEmpty = false
		} else {
			col.DefaultIsEmpty = true
		}

		fields := strings.Fields(colType)
		if len(fields) == 2 && fields[1] == "unsigned" {
			isUnsigned = true
		}
		colType = fields[0]
		cts := strings.Split(colType, "(")
		colName := cts[0]
		// Remove the /* mariadb-5.3 */ suffix from coltypes
		colName = strings.TrimSuffix(colName, "/* mariadb-5.3 */")
		colType = strings.ToUpper(colName)
		var len1, len2 int
		if len(cts) == 2 {
			idx := strings.Index(cts[1], ")")
			if colType == schemas.Enum && cts[1][0] == '\'' { // enum
				options := strings.Split(cts[1][0:idx], ",")
				col.EnumOptions = make(map[string]int)
				for k, v := range options {
					v = strings.TrimSpace(v)
					v = strings.Trim(v, "'")
					col.EnumOptions[v] = k
				}
			} else if colType == schemas.Set && cts[1][0] == '\'' {
				options := strings.Split(cts[1][0:idx], ",")
				col.SetOptions = make(map[string]int)
				for k, v := range options {
					v = strings.TrimSpace(v)
					v = strings.Trim(v, "'")
					col.SetOptions[v] = k
				}
			} else {
				lens := strings.Split(cts[1][0:idx], ",")
				len1, err = strconv.Atoi(strings.TrimSpace(lens[0]))
				if err != nil {
					return nil, nil, err
				}
				if len(lens) == 2 {
					len2, err = strconv.Atoi(lens[1])
					if err != nil {
						return nil, nil, err
					}
				}
			}
		}
		if isUnsigned {
			colType = "UNSIGNED " + colType
		}
		col.Length = len1
		col.Length2 = len2
		if _, ok := schemas.SqlTypes[colType]; ok {
			col.SQLType = schemas.SQLType{Name: colType, DefaultLength: len1, DefaultLength2: len2}
		} else {
			return nil, nil, fmt.Errorf("Unknown colType %v", colType)
		}

		if colKey == "PRI" {
			col.IsPrimaryKey = true
		}
		if colKey == "UNI" {
			// col.is
		}

		if extra == "auto_increment" {
			col.IsAutoIncrement = true
		}

		if !col.DefaultIsEmpty {
			if !alreadyQuoted && col.SQLType.IsText() {
				col.Default = "'" + col.Default + "'"
			} else if col.SQLType.IsTime() && !alreadyQuoted && col.Default != "CURRENT_TIMESTAMP" {
				col.Default = "'" + col.Default + "'"
			}
		}
		cols[col.Name] = col
		colSeq = append(colSeq, col.Name)
	}
	return colSeq, cols, nil
}

func (db *mysql) GetTables(queryer core.Queryer, ctx context.Context) ([]*schemas.Table, error) {
	args := []interface{}{db.uri.DBName}
	s := "SELECT `TABLE_NAME`, `ENGINE`, `AUTO_INCREMENT`, `TABLE_COMMENT` from " +
		"`INFORMATION_SCHEMA`.`TABLES` WHERE `TABLE_SCHEMA`=? AND (`ENGINE`='MyISAM' OR `ENGINE` = 'InnoDB' OR `ENGINE` = 'TokuDB')"

	rows, err := queryer.QueryContext(ctx, s, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tables := make([]*schemas.Table, 0)
	for rows.Next() {
		table := schemas.NewEmptyTable()
		var name, engine string
		var autoIncr, comment *string
		err = rows.Scan(&name, &engine, &autoIncr, &comment)
		if err != nil {
			return nil, err
		}

		table.Name = name
		if comment != nil {
			table.Comment = *comment
		}
		table.StoreEngine = engine
		tables = append(tables, table)
	}
	return tables, nil
}

func (db *mysql) SetQuotePolicy(quotePolicy QuotePolicy) {
	switch quotePolicy {
	case QuotePolicyNone:
		var q = mysqlQuoter
		q.IsReserved = schemas.AlwaysNoReserve
		db.quoter = q
	case QuotePolicyReserved:
		var q = mysqlQuoter
		q.IsReserved = db.IsReserved
		db.quoter = q
	case QuotePolicyAlways:
		fallthrough
	default:
		db.quoter = mysqlQuoter
	}
}

func (db *mysql) GetIndexes(queryer core.Queryer, ctx context.Context, tableName string) (map[string]*schemas.Index, error) {
	args := []interface{}{db.uri.DBName, tableName}
	s := "SELECT `INDEX_NAME`, `NON_UNIQUE`, `COLUMN_NAME` FROM `INFORMATION_SCHEMA`.`STATISTICS` WHERE `TABLE_SCHEMA` = ? AND `TABLE_NAME` = ?"

	rows, err := queryer.QueryContext(ctx, s, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	indexes := make(map[string]*schemas.Index, 0)
	for rows.Next() {
		var indexType int
		var indexName, colName, nonUnique string
		err = rows.Scan(&indexName, &nonUnique, &colName)
		if err != nil {
			return nil, err
		}

		if indexName == "PRIMARY" {
			continue
		}

		if "YES" == nonUnique || nonUnique == "1" {
			indexType = schemas.IndexType
		} else {
			indexType = schemas.UniqueType
		}

		colName = strings.Trim(colName, "` ")
		var isRegular bool
		if strings.HasPrefix(indexName, "IDX_"+tableName) || strings.HasPrefix(indexName, "UQE_"+tableName) {
			indexName = indexName[5+len(tableName):]
			isRegular = true
		}

		var index *schemas.Index
		var ok bool
		if index, ok = indexes[indexName]; !ok {
			index = new(schemas.Index)
			index.IsRegular = isRegular
			index.Type = indexType
			index.Name = indexName
			indexes[indexName] = index
		}
		index.AddColumn(colName)
	}
	return indexes, nil
}

func (db *mysql) CreateTableSQL(table *schemas.Table, tableName string) ([]string, bool) {
	var sql = "CREATE TABLE IF NOT EXISTS "
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
			if len(col.Comment) > 0 {
				sql += " COMMENT '" + col.Comment + "'"
			}
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

	if table.StoreEngine != "" {
		sql += " ENGINE=" + table.StoreEngine
	}

	var charset = table.Charset
	if len(charset) == 0 {
		charset = db.URI().Charset
	}
	if len(charset) != 0 {
		sql += " DEFAULT CHARSET " + charset
	}

	if db.rowFormat != "" {
		sql += " ROW_FORMAT=" + db.rowFormat
	}
	return []string{sql}, true
}

func (db *mysql) Filters() []Filter {
	return []Filter{}
}

type mymysqlDriver struct {
}

func (p *mymysqlDriver) Parse(driverName, dataSourceName string) (*URI, error) {
	uri := &URI{DBType: schemas.MYSQL}

	pd := strings.SplitN(dataSourceName, "*", 2)
	if len(pd) == 2 {
		// Parse protocol part of URI
		p := strings.SplitN(pd[0], ":", 2)
		if len(p) != 2 {
			return nil, errors.New("Wrong protocol part of URI")
		}
		uri.Proto = p[0]
		options := strings.Split(p[1], ",")
		uri.Raddr = options[0]
		for _, o := range options[1:] {
			kv := strings.SplitN(o, "=", 2)
			var k, v string
			if len(kv) == 2 {
				k, v = kv[0], kv[1]
			} else {
				k, v = o, "true"
			}
			switch k {
			case "laddr":
				uri.Laddr = v
			case "timeout":
				to, err := time.ParseDuration(v)
				if err != nil {
					return nil, err
				}
				uri.Timeout = to
			default:
				return nil, errors.New("Unknown option: " + k)
			}
		}
		// Remove protocol part
		pd = pd[1:]
	}
	// Parse database part of URI
	dup := strings.SplitN(pd[0], "/", 3)
	if len(dup) != 3 {
		return nil, errors.New("Wrong database part of URI")
	}
	uri.DBName = dup[0]
	uri.User = dup[1]
	uri.Passwd = dup[2]

	return uri, nil
}

type mysqlDriver struct {
}

func (p *mysqlDriver) Parse(driverName, dataSourceName string) (*URI, error) {
	dsnPattern := regexp.MustCompile(
		`^(?:(?P<user>.*?)(?::(?P<passwd>.*))?@)?` + // [user[:password]@]
			`(?:(?P<net>[^\(]*)(?:\((?P<addr>[^\)]*)\))?)?` + // [net[(addr)]]
			`\/(?P<dbname>.*?)` + // /dbname
			`(?:\?(?P<params>[^\?]*))?$`) // [?param1=value1&paramN=valueN]
	matches := dsnPattern.FindStringSubmatch(dataSourceName)
	// tlsConfigRegister := make(map[string]*tls.Config)
	names := dsnPattern.SubexpNames()

	uri := &URI{DBType: schemas.MYSQL}

	for i, match := range matches {
		switch names[i] {
		case "dbname":
			uri.DBName = match
		case "params":
			if len(match) > 0 {
				kvs := strings.Split(match, "&")
				for _, kv := range kvs {
					splits := strings.Split(kv, "=")
					if len(splits) == 2 {
						switch splits[0] {
						case "charset":
							uri.Charset = splits[1]
						}
					}
				}
			}

		}
	}
	return uri, nil
}
