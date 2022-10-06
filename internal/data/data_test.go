package data

import (
    "context"
    "encoding/json"
    "fmt"
    "testing"
    "time"
    "strings"
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
    cache, cacheMock := redismock.NewClientMock()
    cacheMock = cacheMock.Regexp()
    user := User{"arbuzich", "myemail@example.com"}
    method := "ADD"
    opn := Operation{user, method}
    buf := new(strings.Builder)
    err := json.NewEncoder(buf).Encode(&opn)
    jsonData := buf.String()
    if err != nil {
        t.Fatalf("Unexpected error while encoding Operation struct: %v", err)
    }
    regexpStr := fmt.Sprintf(`.[%d]`, keySize)
    cacheMock.ExpectSet(regexpStr, jsonData, authExpiration).SetVal("Success")
    d := &postgresRedisData{}
    d.cache = cache
    ctx, cancel := context.WithTimeout(context.Background(), 1 * time.Second)
    defer cancel()
    
    key, err := d.SetOperation(ctx, user, method)
    pattern := regexp.MustCompile(regexpStr)
    if assert.Nil(t, err) {
        assert.True(t, pattern.MatchString(key))
    }
    cacheMock.ExpectGet(key).SetVal(jsonData)
    result := d.cache.Get(ctx, key)
    resultString, err := result.Result()
    if assert.Nil(t, result.Err()) && assert.Nil(t, err) {
        assert.Equal(t, jsonData, resultString)
    }
}

func TestCheckNicknameInDatabaseCacheHit(t *testing.T) {
    cache, cacheMock := redismock.NewClientMock()
    testNickname := "Aboba"
    cacheMock.ExpectGet(testNickname).SetVal("true")
    ctx, cancel := context.WithTimeout(context.Background(), 1 * time.Second)
    defer cancel()
    d := &postgresRedisData{}
    d.cache = cache
    result, err := d.CheckNicknameInDatabase(ctx, testNickname)
    if assert.Nil(t, err) {
        assert.True(t, result)
    }
}

func TestCheckNicknameInDatabaseCacheMiss(t *testing.T) {
    cache, cacheMock := redismock.NewClientMock()
    db, dbMock, err := sqlmock.New()
    if err != nil {
        t.Fatalf("Error \"%v\" was not expected while opening a mock database connection", err)
    }
    d := &postgresRedisData{}
    d.cache = cache
    d.db = db
    testNickname := "ThomasShelby"
    cacheMock.ExpectGet(testNickname).RedisNil()
    rows := sqlmock.NewRows([]string{"nickname"}).AddRow(testNickname)
    dbMock.ExpectQuery(regexp.QuoteMeta(`SELECT nickname FROM Users WHERE nickname = $1`)).WithArgs(testNickname).WillReturnRows(rows).RowsWillBeClosed()
    cacheMock.ExpectSet(testNickname, true, cacheExpiration).SetVal("success")
    ctx, cancel := context.WithTimeout(context.Background(), 1 * time.Second)
    defer cancel()
    result, err := d.CheckNicknameInDatabase(ctx, testNickname)
    if assert.Nil(t, err) {
        assert.True(t, result)
    }
}

func TestCheckNicknameInDatabaseNotExists(t *testing.T) {
    cache, cacheMock := redismock.NewClientMock()
    db, dbMock, err := sqlmock.New()
    if err != nil {
        t.Fatalf("Error \"%v\" was not expected while opening a mock database connection", err)
    }
    d := &postgresRedisData{}
    d.cache = cache
    d.db = db
    testNickname := "WatermelonHater"
    cacheMock.ExpectGet(testNickname).RedisNil()
    rows := sqlmock.NewRows([]string{"nickname"})
    dbMock.ExpectQuery(regexp.QuoteMeta(`SELECT nickname FROM Users WHERE nickname = $1`)).WithArgs(testNickname).WillReturnRows(rows).RowsWillBeClosed()
    cacheMock.ExpectSet(testNickname, false, cacheExpiration).SetVal("success")
    ctx, cancel := context.WithTimeout(context.Background(), 1 * time.Second)
    defer cancel()
    result, err := d.CheckNicknameInDatabase(ctx, testNickname)
    if assert.Nil(t, err) {
        assert.False(t, result)
    }
}

func TestAddUserToDatabase(t *testing.T) {
    cache, cacheMock := redismock.NewClientMock()
    db, dbMock, err := sqlmock.New()
    if err != nil {
        t.Fatalf("Error \"%v\" was not expected while opening a mock database connection", err)
    }
    d := &postgresRedisData{}
    d.cache = cache
    d.db = db
    testUser := User{"Newbie", "nwb@example.com"}
    dbMock.ExpectExec(regexp.QuoteMeta(`INSERT INTO Users VALUES ($1, $2)`)).WithArgs(testUser.Nickname, testUser.Email).WillReturnResult(sqlmock.NewResult(1, 1))
    cacheMock.ExpectDel(ListUsersKey).SetVal(1)
    ctx, cancel := context.WithTimeout(context.Background(), 1 * time.Second)
    defer cancel()
    err = d.AddUserToDatabase(ctx, testUser)
    assert.Nil(t, err)
}

func TestDeleteUserFromDatabase(t *testing.T) {
    cache, cacheMock := redismock.NewClientMock()
    db, dbMock, err := sqlmock.New()
    if err != nil {
        t.Fatalf("Error \"%v\" was not expected while opening a mock database connection", err)
    }
    d := &postgresRedisData{}
    d.cache = cache
    d.db = db
    testUser := User{"Old", "old@example.com"}
    dbMock.ExpectExec(regexp.QuoteMeta(`DELETE FROM Users WHERE nickname=$1 AND email=$2`)).WithArgs(testUser.Nickname, testUser.Email).WillReturnResult(sqlmock.NewResult(1, 1))
    cacheMock.ExpectDel(ListUsersKey).SetVal(0)
    ctx, cancel := context.WithTimeout(context.Background(), 1 * time.Second)
    defer cancel()
    err = d.DeleteUserFromDatabase(ctx, testUser)
    assert.Nil(t, err)
}

func TestGetUsersFromDatabaseCacheHit(t *testing.T) {
    cache, cacheMock := redismock.NewClientMock()
    d := &postgresRedisData{}
    d.cache = cache
    cacheMock.ExpectGet(ListUsersKey).SetVal(`[{"nickname":"pupa","email":"buhga@gmail.com"},
                                               {"nickname":"lupa","email":"lteria@gmail.com"}]`)
    testUsers := []User{{"pupa", "buhga@gmail.com"}, {"lupa", "lteria@gmail.com"}}
    ctx, cancel := context.WithTimeout(context.Background(), 1 * time.Second)
    defer cancel()
    result, err := d.GetUsersFromDatabase(ctx)
    if assert.Nil(t, err) && assert.Equal(t, len(testUsers), len(result)) {
        for i := 0; i < len(testUsers); i++ {
            assert.Equal(t, testUsers[i], result[i])
        }
    }
}

func TestGetUsersFromDatabaseCacheMiss(t *testing.T) {
    cache, cacheMock := redismock.NewClientMock()
    db, dbMock, err := sqlmock.New()
    if err != nil {
        t.Fatalf("Error \"%v\" was not expected while opening a mock database connection", err)
    }
    d := &postgresRedisData{}
    d.cache = cache
    d.db = db
    cacheMock.ExpectGet(ListUsersKey).RedisNil()
    rows := sqlmock.NewRows([]string{"nickname", "email"})
    rows.AddRow("pupa", "buhga@gmail.com").AddRow("lupa", "lteria@gmail.com")
    dbMock.ExpectQuery(regexp.QuoteMeta(`SELECT nickname, email FROM Users`)).WillReturnRows(rows).RowsWillBeClosed()
    testUsers := []User{{"pupa", "buhga@gmail.com"}, {"lupa", "lteria@gmail.com"}}
    buf := new(strings.Builder)
    if err = json.NewEncoder(buf).Encode(&testUsers); err != nil {
        t.Fatalf("Unexpected error while encoding Users slice: %v", err)
    }
    cacheMock.ExpectSet(ListUsersKey, buf.String(), cacheExpiration).SetVal("success")
    ctx, cancel := context.WithTimeout(context.Background(), 1 * time.Second)
    defer cancel()
    result, err := d.GetUsersFromDatabase(ctx)
    if assert.Nil(t, err) && assert.Equal(t, len(testUsers), len(result)) {
        for i := 0; i < len(testUsers); i++ {
            assert.Equal(t, testUsers[i], result[i])
        }
    }
}

func TestGetUsersFromDatabaseEmpty(t *testing.T) {
    cache, cacheMock := redismock.NewClientMock()
    db, dbMock, err := sqlmock.New()
    if err != nil {
        t.Fatalf("Error \"%v\" was not expected while opening a mock database connection", err)
    }
    d := &postgresRedisData{}
    d.cache = cache
    d.db = db
    cacheMock.ExpectGet(ListUsersKey).RedisNil()
    rows := sqlmock.NewRows([]string{"nickname", "email"})
    dbMock.ExpectQuery(regexp.QuoteMeta(`SELECT nickname, email FROM Users`)).WillReturnRows(rows).RowsWillBeClosed()
    cacheMock.ExpectSet(ListUsersKey, "null\n", cacheExpiration).SetVal("success")
    ctx, cancel := context.WithTimeout(context.Background(), 1 * time.Second)
    defer cancel()
    result, err := d.GetUsersFromDatabase(ctx)
    if assert.Nil(t, err) {
        assert.Zero(t, len(result))
    }
}
    
    
    
    
