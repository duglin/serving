package database

import (
	"database/sql"
	"errors"
	"log"

	"github.com/knative/serving/pkg/queue"
	"go.uber.org/zap"
)

const (
	AsyncTable = "async_table"
)

var (
	recordColumns = ColumnList{
		AsyncTable + ".guid",
		AsyncTable + ".pod",
		AsyncTable + ".status",
		AsyncTable + ".body",
		AsyncTable + ".status_code",
	}
)

func (db *PostgresSQLDB) CreateAsyncTable() error {
	_, err := db.db.Exec(`
		CREATE TABLE IF NOT EXISTS async_table(
			guid VARCHAR(255) PRIMARY KEY,
			pod VARCHAR(255) NOT NULL,
			status INT NOT NULL,
			body BYTEA,
			status_code INT
		)
	`)
	if err != nil {
		return err
	}

	return nil
}

func (db *PostgresSQLDB) CreateAsyncReq(guid, pod string) error {
	err := db.transact(func(tx Tx) error {
		_, err := db.insert(tx, AsyncTable,
			SQLAttributes{
				"guid":        guid,
				"pod":         pod,
				"status":      queue.InProgress,
				"status_code": 0,
			},
		)
		return err
	})

	if err != nil {
		return err
	}

	return nil
}

func (db *PostgresSQLDB) UpdateAsyncReq(guid string, status queue.Status, body []byte, statusCode int) error {
	err := db.transact(func(tx Tx) error {
		_, err := db.update(tx, AsyncTable, SQLAttributes{
			"status":      status,
			"body":        body,
			"status_code": statusCode,
		}, "guid = ?", guid)

		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

func (db *PostgresSQLDB) FetchRecord(guid string) (*queue.AsyncCallRecord, error) {
	var records []*queue.AsyncCallRecord

	err := db.transact(func(tx Tx) error {
		rows, err := db.all(tx, AsyncTable, recordColumns, LockRow, "guid = ?", guid)
		if err != nil {
			return err
		}

		records, err = db.scanRecord(rows)

		if err != nil {
			return err
		}

		if len(records) != 1 {
			return errors.New("multiple-records-found")
		}

		return nil
	})

	if err != nil {
		return nil, err
	}
	return records[0], nil
}

func (db *PostgresSQLDB) DeleteRecord(guid string) error {
	err := db.transact(func(tx Tx) error {
		_, err := db.delete(
			tx,
			AsyncTable,
			"guid = ?",
			guid,
		)

		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

func (db *PostgresSQLDB) scanRecord(rows *sql.Rows) ([]*queue.AsyncCallRecord, error) {
	result := []*queue.AsyncCallRecord{}

	for rows.Next() {
		var record queue.AsyncCallRecord
		err := rows.Scan(
			&record.Guid,
			&record.Pod,
			&record.Status,
			&record.Body,
			&record.StatusCode,
		)

		if err != nil {
			log.Println("failed-scanning-actual-lrp", zap.Error(err))
			return nil, err
		}

		result = append(result, &record)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return result, nil
}

func (db *PostgresSQLDB) one(q Queryable, table string,
	columns ColumnList, lockRow RowLock,
	wheres string, whereBindings ...interface{},
) RowScanner {
	return db.helper.One(q, table, columns, lockRow, wheres, whereBindings...)
}

func (db *PostgresSQLDB) all(q Queryable, table string,
	columns ColumnList, lockRow RowLock,
	wheres string, whereBindings ...interface{},
) (*sql.Rows, error) {
	return db.helper.All(q, table, columns, lockRow, wheres, whereBindings...)
}

func (db *PostgresSQLDB) upsert(q Queryable, table string, attributes SQLAttributes, wheres string, whereBindings ...interface{}) (bool, error) {
	return db.helper.Upsert(q, table, attributes, wheres, whereBindings...)
}

func (db *PostgresSQLDB) insert(q Queryable, table string, attributes SQLAttributes) (sql.Result, error) {
	return db.helper.Insert(q, table, attributes)
}

func (db *PostgresSQLDB) update(q Queryable, table string, updates SQLAttributes, wheres string, whereBindings ...interface{}) (sql.Result, error) {
	return db.helper.Update(q, table, updates, wheres, whereBindings...)
}

func (db *PostgresSQLDB) delete(q Queryable, table string, wheres string, whereBindings ...interface{}) (sql.Result, error) {
	return db.helper.Delete(q, table, wheres, whereBindings...)
}

func (db *PostgresSQLDB) transact(f func(tx Tx) error) error {
	err := db.helper.Transact(db.db, f)
	if err != nil {
		return err
	}
	return nil
}
