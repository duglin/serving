package database

import (
	"database/sql"
	"fmt"
	"strings"
)

const (
	MySQL    = "mysql"
	Postgres = "postgres"

	LockRow   RowLock = true
	NoLockRow RowLock = false
)

type SQLHelper interface {
	Transact(db QueryableDB, f func(tx Tx) error) error
	One(q Queryable, table string, columns ColumnList, lockRow RowLock, wheres string, whereBindings ...interface{}) RowScanner
	All(q Queryable, table string, columns ColumnList, lockRow RowLock, wheres string, whereBindings ...interface{}) (*sql.Rows, error)
	Upsert(q Queryable, table string, attributes SQLAttributes, wheres string, whereBindings ...interface{}) (bool, error)
	Insert(q Queryable, table string, attributes SQLAttributes) (sql.Result, error)
	Update(q Queryable, table string, updates SQLAttributes, wheres string, whereBindings ...interface{}) (sql.Result, error)
	Delete(q Queryable, table string, wheres string, whereBindings ...interface{}) (sql.Result, error)
	Count(q Queryable, table string, wheres string, whereBindings ...interface{}) (int, error)

	Rebind(query string) string
}

type sqlHelper struct {
	flavor string
}

func NewSQLHelper(flavor string) *sqlHelper {
	return &sqlHelper{flavor: flavor}
}

type RowLock bool
type SQLAttributes map[string]interface{}
type ColumnList []string

func (h *sqlHelper) Rebind(query string) string {
	return RebindForFlavor(query, h.flavor)
}

func RebindForFlavor(query, flavor string) string {
	if flavor == MySQL {
		return query
	}
	if flavor != Postgres {
		panic(fmt.Sprintf("Unrecognized DB flavor '%s'", flavor))
	}

	strParts := strings.Split(query, "?")
	for i := 1; i < len(strParts); i++ {
		strParts[i-1] = fmt.Sprintf("%s$%d", strParts[i-1], i)
	}
	return strings.Replace(strings.Join(strParts, ""), "MEDIUMTEXT", "TEXT", -1)
}

func QuestionMarks(count int) string {
	if count == 0 {
		return ""
	}
	return strings.Repeat("?, ", count-1) + "?"
}
