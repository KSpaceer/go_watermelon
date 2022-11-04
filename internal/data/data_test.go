package data

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-redis/redismock/v8"
	"github.com/stretchr/testify/assert"
)

func TestGetOperationExistingKey(t *testing.T) {
	cache, cacheMock := redismock.NewClientMock()
	key := "lUQAbb59alonQuW4p3sqTxA_d9Fq5Jk7dkfd_GR43IdPPCxFzUvWmT4vy4GMkjj5udkSoJrZ5_NEqdeFN6aCRKuPvbZwdF3IQ-KRgRgwpfBIo0XVWBkKv1R5ZwL3CjPLJqzL8_VeeZx4ae0fHz5uqXjZ3vaExLZ6j3ZaL7-Kt9Y"
	cacheMock.ExpectGet(key).SetVal(`{"user":{"nickname":"arbuz","email":"arbuz@gmail.com"},"method":"DELETE"}`)
	d := &dataHandler{}
	d.cache = &RedisCache{cache}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
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
	d := &dataHandler{}
	d.cache = &RedisCache{cache}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
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
	d := &dataHandler{}
	d.cache = &RedisCache{cache}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	key, err := d.SetOperation(ctx, user, method)
	pattern := regexp.MustCompile(regexpStr)
	if assert.Nil(t, err) {
		assert.True(t, pattern.MatchString(key))
	}
	cacheMock.ExpectGet(key).SetVal(jsonData)
	resultString, err := d.cache.Get(ctx, key)
	if assert.Nil(t, err) {
		assert.Equal(t, jsonData, resultString)
	}
}

func TestGetEmailByNicknameCacheHit(t *testing.T) {
	cache, cacheMock := redismock.NewClientMock()
	testNickname := "averageTeaEnjoyer"
	testEmail := "gigachad@example.com"
	cacheMock.ExpectGet(testNickname).SetVal(testEmail)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	d := &dataHandler{}
	d.cache = &RedisCache{cache}
	email, err := d.GetEmailByNickname(ctx, testNickname)
	if assert.Nil(t, err) {
		assert.Equal(t, testEmail, email)
	}
}

func TestGetEmailByNicknameCacheMiss(t *testing.T) {
	cache, cacheMock := redismock.NewClientMock()
	db, dbMock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error \"%v\" was not expected while opening a mock database connection", err)
	}
	d := &dataHandler{}
	d.cache = &RedisCache{cache}
	d.db = &PgsDB{db}
	testNickname := "PatrickBateman"
	testEmail := "americanpsycho@gmail.com"
	cacheMock.ExpectGet(testNickname).RedisNil()
	rows := sqlmock.NewRows([]string{"email"}).AddRow(testEmail)
	dbMock.ExpectQuery(regexp.QuoteMeta(`SELECT email FROM Users WHERE nickname = $1`)).WithArgs(testNickname).WillReturnRows(rows).RowsWillBeClosed()
	cacheMock.ExpectSet(testNickname, testEmail, cacheExpiration).SetVal("success")
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	email, err := d.GetEmailByNickname(ctx, testNickname)
	if assert.Nil(t, err) {
		assert.Equal(t, testEmail, email)
	}
}

func TestGetEmailByNicknameNotExists(t *testing.T) {
	cache, cacheMock := redismock.NewClientMock()
	db, dbMock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error \"%v\" was not expected while opening a mock database connection", err)
	}
	d := &dataHandler{}
	d.cache = &RedisCache{cache}
	d.db = &PgsDB{db}
	testNickname := "Moon"
	cacheMock.ExpectGet(testNickname).RedisNil()
	rows := sqlmock.NewRows([]string{"email"})
	dbMock.ExpectQuery(regexp.QuoteMeta(`SELECT email FROM Users WHERE nickname = $1`)).WithArgs(testNickname).WillReturnRows(rows).RowsWillBeClosed()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	email, err := d.GetEmailByNickname(ctx, testNickname)
	if assert.Nil(t, err) {
		assert.Equal(t, "", email)
	}
}

func TestCheckNicknameInDatabaseCacheHit(t *testing.T) {
	cache, cacheMock := redismock.NewClientMock()
	testNickname := "Aboba"
	cacheMock.ExpectGet(testNickname).SetVal("aboba@gmail.com")
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	d := &dataHandler{}
	d.cache = &RedisCache{cache}
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
	d := &dataHandler{}
	d.cache = &RedisCache{cache}
	d.db = &PgsDB{db}
	testNickname := "ThomasShelby"
	testEmail := "peakyblinders@example.com"
	cacheMock.ExpectGet(testNickname).RedisNil()
	rows := sqlmock.NewRows([]string{"email"}).AddRow(testEmail)
	dbMock.ExpectQuery(regexp.QuoteMeta(`SELECT email FROM Users WHERE nickname = $1`)).WithArgs(testNickname).WillReturnRows(rows).RowsWillBeClosed()
	cacheMock.ExpectSet(testNickname, testEmail, cacheExpiration).SetVal("success")
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
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
	d := &dataHandler{}
	d.cache = &RedisCache{cache}
	d.db = &PgsDB{db}
	testNickname := "WatermelonHater"
	cacheMock.ExpectGet(testNickname).RedisNil()
	rows := sqlmock.NewRows([]string{"email"})
	dbMock.ExpectQuery(regexp.QuoteMeta(`SELECT email FROM Users WHERE nickname = $1`)).WithArgs(testNickname).WillReturnRows(rows).RowsWillBeClosed()
	cacheMock.ExpectSet(testNickname, "", cacheExpiration).SetVal("success")
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
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
	d := &dataHandler{}
	d.cache = &RedisCache{cache}
	d.db = &PgsDB{db}
	testUser := User{"Newbie", "nwb@example.com"}
	dbMock.ExpectExec(regexp.QuoteMeta(`INSERT INTO Users VALUES ($1, $2)`)).WithArgs(testUser.Nickname, testUser.Email).WillReturnResult(sqlmock.NewResult(1, 1))
	cacheMock.ExpectDel(ListUsersKey).SetVal(1)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
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
	d := &dataHandler{}
	d.cache = &RedisCache{cache}
	d.db = &PgsDB{db}
	testUser := User{"Old", "old@example.com"}
	dbMock.ExpectExec(regexp.QuoteMeta(`DELETE FROM Users WHERE nickname=$1 AND email=$2`)).WithArgs(testUser.Nickname, testUser.Email).WillReturnResult(sqlmock.NewResult(1, 1))
	cacheMock.ExpectDel(ListUsersKey).SetVal(0)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	err = d.DeleteUserFromDatabase(ctx, testUser)
	assert.Nil(t, err)
}

func TestGetUsersFromDatabaseCacheHit(t *testing.T) {
	cache, cacheMock := redismock.NewClientMock()
	d := &dataHandler{}
	d.cache = &RedisCache{cache}
	cacheMock.ExpectGet(ListUsersKey).SetVal(`[{"nickname":"pupa","email":"buhga@gmail.com"},
                                               {"nickname":"lupa","email":"lteria@gmail.com"}]`)
	testUsers := []User{{"pupa", "buhga@gmail.com"}, {"lupa", "lteria@gmail.com"}}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
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
	d := &dataHandler{}
	d.cache = &RedisCache{cache}
	d.db = &PgsDB{db}
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
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
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
	d := &dataHandler{}
	d.cache = &RedisCache{cache}
	d.db = &PgsDB{db}
	cacheMock.ExpectGet(ListUsersKey).RedisNil()
	rows := sqlmock.NewRows([]string{"nickname", "email"})
	dbMock.ExpectQuery(regexp.QuoteMeta(`SELECT nickname, email FROM Users`)).WillReturnRows(rows).RowsWillBeClosed()
	cacheMock.ExpectSet(ListUsersKey, "null\n", cacheExpiration).SetVal("success")
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	result, err := d.GetUsersFromDatabase(ctx)
	if assert.Nil(t, err) {
		assert.Zero(t, len(result))
	}
}
