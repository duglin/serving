package database

import (
	"log"
)

// Upsert insert a record if it doesn't exist or update the record if one
// already exists.  Returns true if a new record was inserted in the database.
func (h *sqlHelper) Upsert(
	q Queryable,
	table string,
	attributes SQLAttributes,
	wheres string,
	whereBindings ...interface{},
) (bool, error) {
	res, err := h.Update(
		q,
		table,
		attributes,
		wheres,
		whereBindings...,
	)
	if err != nil {
		return false, err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		// this should never happen
		log.Printf("failed-getting-rows-affected %s", err.Error())
		return false, err
	}

	if rowsAffected > 0 {
		return false, nil
	}

	res, err = h.Insert(
		q,
		table,
		attributes,
	)
	if err != nil {
		return false, err
	}

	return true, nil
}
