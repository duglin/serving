package database

import (
	"github.com/knative/serving/pkg/queue"
)

type PostgresSQLDB struct {
	db     QueryableDB
	flavor string
	helper SQLHelper
}

func NewSQLDB(
	db QueryableDB,
	flavor string,
) queue.SQLDB {
	helper := NewSQLHelper(flavor)
	return &PostgresSQLDB{
		db:     db,
		flavor: flavor,
		helper: helper,
	}
}
