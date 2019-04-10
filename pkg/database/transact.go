package database

import (
	"log"
)

// BEGIN TRANSACTION; f ... ; COMMIT; or
// BEGIN TRANSACTION; f ... ; ROLLBACK; if f returns an error.
func (h *sqlHelper) Transact(db QueryableDB, f func(tx Tx) error) error {
	var err error

	err = func() error {
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()

		err = f(tx)
		if err != nil {
			return err
		}

		err = tx.Commit()
		if err != nil {
			log.Printf("failed-committing-transaction %s", err.Error())

		}
		return err
	}()

	return err
}
