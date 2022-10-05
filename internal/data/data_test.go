package data

import (
    "context"
    "fmt"
    "testing"
    "time"
    "regexp"

    "github.com/DATA-DOG/go-sqlmock"
    "github.com/go-redis/redismock/v8"
    "github.com/stretchr/testify/assert"
)

func TestCreateUsersTableSuccess(t *testing.T) {
    db, dbMock, err := sqlmock.New()
    if err != nil {
        t.Fatalf("Error \"%v\" was not expected while opening a mock database connection", err)
    }
    dbMock.ExpectExec(regexp.QuoteMeta(`CREATE TABLE IF NOT EXISTS Users (nickname TEXT, email TEXT);`)).WillReturnError(nil).WillReturnResult(sqlmock.NewResult(1, 1))
    dbMock.ExpectExec(regexp.QuoteMeta(`CREATE UNIQUE INDEX IF NOT EXISTS nickname_idx ON Users(nickname);`)).WillReturnError(nil).WillReturnResult(sqlmock.NewResult(1, 1))
    d := &postgresRedisData{}
    d.db = db
    assert.Nil(t, d.createUsersTable())
}

func TestCreateUsersTableCreateTableFailed(t *testing.T) {
    db, dbMock, err := sqlmock.New()
    if err != nil {
        t.Fatalf("Error \"%v\" was not expected while opening a mock database connection", err)
    }
    mockError := fmt.Errorf("Failed to create table.")
    dbMock.ExpectExec(regexp.QuoteMeta(`CREATE TABLE IF NOT EXISTS Users (nickname TEXT, email TEXT);`)).WillReturnError(mockError).WillReturnResult(sqlmock.NewResult(1, 1))
    d := &postgresRedisData{}
    d.db = db
    assert.NotNil(t, d.createUsersTable())
}

func TestCreateUsersTableCreateIndexFailed(t *testing.T) {
    db, dbMock, err := sqlmock.New()
    if err != nil {
        t.Fatalf("Error \"%v\" was not expected while opening a mock database connection", err)
    }
    dbMock.ExpectExec(regexp.QuoteMeta(`CREATE TABLE IF NOT EXISTS Users (nickname TEXT, email TEXT);`)).WillReturnError(nil).WillReturnResult(sqlmock.NewResult(1, 1))
    mockError := fmt.Errorf("Failed to create index.")
    dbMock.ExpectExec(regexp.QuoteMeta(`CREATE UNIQUE INDEX IF NOT EXISTS nickname_idx ON Users(nickname);`)).WillReturnError(mockError).WillReturnResult(sqlmock.NewResult(1, 1))
    d := &postgresRedisData{}
    d.db = db
    assert.NotNil(t, d.createUsersTable)
}

func TestGetOperationExistingKey(t *testing.T) {
    cache, cacheMock := redismock.NewClientMock()
    key := "lUQAbb59alonQuW4p3sqTxA_d9Fq5Jk7dkfd_GR43IdPPCxFzUvWmT4vy4GMkjj5udkSoJrZ5_NEqdeFN6aCRKuPvbZwdF3IQ-KRgRgwpfBIo0XVWBkKv1R5ZwL3CjPLJqzL8_VeeZx4ae0fHz5uqXjZ3vaExLZ6j3ZaL7-Kt9Y"
    cacheMock.ExpectGet(key).SetVal(`{"user":{"nickname":"arbuz","email":"arbuz@gmail.com"},"method":"DELETE"}`)
    d := &postgresRedisData{}
    d.cache = cache
    ctx, cancel := context.WithTimeout(context.Background(), 1 * time.Second)
    defer cancel()
    operation, err := d.GetOperation(ctx, key)
    if assert.Nil(t, err) {
        assert.Equal(t, Operation{User{"arbuz", "arbuz@gmail.com"}, "DELETE"}, *operation)
    }
}

func TestGetOperationNonexistingKey(t *testing.T) {
    cache, cacheMock := redismock.NewClientMock()
    key := "Idonotexist"
    cacheMock.ExpectGet(key).RedisNil()
    d := &postgresRedisData{}
    d.cache = cache
    ctx, cancel := context.WithTimeout(context.Background(), 1 * time.Second)
    defer cancel()
    operation, err := d.GetOperation(ctx, key)
    if assert.Nil(t, err) {
        assert.Equal(t, Operation{}, *operation)
    }
}

func TestSetOperationSuccess(t *testing.T) {
    cache, cacheMock := redismock.NewC

