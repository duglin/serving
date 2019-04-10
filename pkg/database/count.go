package database

import (
	"fmt"
)

// SELECT COUNT(*) FROM <table> WHERE ...
func (h *sqlHelper) Count(
	q Queryable,
	table string,
	wheres string,
	whereBindings ...interface{},
) (int, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s\n", table)

	if len(wheres) > 0 {
		query += "WHERE " + wheres
	}

	var count int
	err := q.QueryRow(h.Rebind(query), whereBindings...).Scan(&count)
	return count, err
}
