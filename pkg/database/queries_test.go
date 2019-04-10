package database

import (
	"database/sql"
	"fmt"
	"os"
	"testing"

	"github.com/knative/serving/pkg/queue"
	"github.com/lib/pq"
)

var (
	databaseConnectionString = os.Getenv("DB_CONN")
	databaseDriver           = os.Getenv("DB_DRIVER")
)

func TestQueries(t *testing.T) {
	_, err := pq.ParseURL(databaseConnectionString)
	if err != nil {
		t.Fatalf("parse-connection-failed %s", err.Error())
	}

	sqlConn, err := sql.Open(databaseDriver, databaseConnectionString)
	if err != nil {
		t.Fatalf("connection-failed %s", err.Error())
	}

	err = sqlConn.Ping()
	if err != nil {
		t.Fatalf("connection-failed %s", err.Error())
	}

	queryMonitor := NewMonitor()
	monitoredDB := NewMonitoredDB(sqlConn, queryMonitor)
	sqlDB := NewSQLDB(monitoredDB, databaseDriver)
	err = sqlDB.CreateAsyncTable()
	if err != nil {
		t.Fatalf("create-table %s", err.Error())
	}

	reqGuid := "some-guid"
	pod := "some-pod"
	err = sqlDB.CreateAsyncReq(reqGuid, pod)
	if err != nil {
		t.Fatalf("create-async-req %s", err.Error())
	}

	body := []byte("some-binary-data")
	statusCode := 200
	err = sqlDB.UpdateAsyncReq(reqGuid, queue.Ready, body, statusCode)
	if err != nil {
		t.Fatalf("update-async-req %s", err.Error())
	}

	record, err := sqlDB.FetchRecord(reqGuid)
	if err != nil {
		t.Fatalf("fetch-async-req %s", err.Error())
	}

	if string(record.Body) != string(body) {
		t.Errorf("wanted %s - got %s", string(record.Body), string(body))
	}

	if record.StatusCode != statusCode {
		t.Errorf("wanted %d - got %d", record.StatusCode, statusCode)
	}

	err = sqlDB.DeleteRecord(reqGuid)
	if err != nil {
		t.Fatalf("update-async-req %s", err.Error())
	}
}

func TestMultipleRequests(t *testing.T) {
	_, err := pq.ParseURL(databaseConnectionString)
	if err != nil {
		t.Fatalf("parse-connection-failed %s", err.Error())
	}

	sqlConn, err := sql.Open(databaseDriver, databaseConnectionString)
	if err != nil {
		t.Fatalf("connection-failed %s", err.Error())
	}

	err = sqlConn.Ping()
	if err != nil {
		t.Fatalf("connection-failed %s", err.Error())
	}

	queryMonitor := NewMonitor()
	monitoredDB := NewMonitoredDB(sqlConn, queryMonitor)
	sqlDB := NewSQLDB(monitoredDB, databaseDriver)
	err = sqlDB.CreateAsyncTable()
	if err != nil {
		t.Fatalf("create-table %s", err.Error())
	}

	for i := 0; i < 5; i++ {
		reqGuid := fmt.Sprintf("some-guid-%d", i)
		pod := "some-pod"
		err = sqlDB.CreateAsyncReq(reqGuid, pod)
		if err != nil {
			t.Errorf("create-async-req %s", err.Error())
		}
	}

	for i := 0; i < 5; i++ {
		reqGuid := fmt.Sprintf("some-guid-%d", i)
		_, err := sqlDB.FetchRecord(reqGuid)
		if err != nil {
			t.Errorf("fetch-async-req %s", err.Error())
		}
	}

	for i := 0; i < 5; i++ {
		reqGuid := fmt.Sprintf("some-guid-%d", i)
		err = sqlDB.DeleteRecord(reqGuid)
		if err != nil {
			t.Fatalf("update-async-req %s", err.Error())
		}
	}
}
