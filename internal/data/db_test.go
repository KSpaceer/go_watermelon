package data

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestCreateUsersTableSuccess(t *testing.T) {
	db, dbMock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error \"%v\" was not expected while opening a mock database connection", err)
	}
	dbMock.ExpectExec(regexp.QuoteMeta(`CREATE TABLE IF NOT EXISTS Users (nickname TEXT,email TEXT,UNIQUE (nickname));`)).WillReturnError(nil).WillReturnResult(sqlmock.NewResult(1, 1))
	assert.Nil(t, createUsersTable(db))
}

func TestCreateUsersTableCreateTableFailed(t *testing.T) {
	db, dbMock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error \"%v\" was not expected while opening a mock database connection", err)
	}
	mockError := fmt.Errorf("Failed to create table.")
	dbMock.ExpectExec(regexp.QuoteMeta(`CREATE TABLE IF NOT EXISTS Users (nickname TEXT, email TEXT);`)).WillReturnError(mockError).WillReturnResult(sqlmock.NewResult(1, 1))
	assert.NotNil(t, createUsersTable(db))
}
