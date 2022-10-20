package data

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

const (
	keySize                       = 128              // authentication key size
	authExpiration  time.Duration = 15 * time.Minute // auth method info expiration time in cache
	cacheExpiration time.Duration = time.Minute      // other info expiration time
	connectTimeout  time.Duration = time.Second      // contextual timeout for connections to DB and cache
	connectAttempts               = 4                // amount of attempts for connections
	ListUsersKey                  = "UsersList"      // key for cache to get the list of users
)

// Data manipulates data in both database in cache, allowing to add,
// delete and get users from data resources. Also it gets and sets authentication
// operation from cache. Disconnect() must be called to close all connection and
// avoid leaks.
type Data interface {
	// Disconnect closes existing connection to database and cache.
	Disconnect()

	// GetOperation tries to get an authentication operation info by given key from cache.
	// If there is no such key, GetOperation returns an empty Operation.
	// In case GetOperation succeeds, it will return Operation with user info and method to be
	// executed.
	GetOperation(ctx context.Context, key string) (*Operation, error)

	// SetOperation creates an Operation instance using passed user and method values,
	// then generates a key which is used to write the operation into cache.
	// If SetOperation succeeds, it will return generated key.
	SetOperation(ctx context.Context, user User, method string) (string, error)

	// CheckNicknameInDatabase selects all rows from database with given nickname and
	// returns true if there are any records.
	CheckNicknameInDatabase(ctx context.Context, nickname string) (bool, error)

	// GetEmailByNickname returns email responding to given nickname. If there is
	// no nickname in database, returns empty string.
	GetEmailByNickname(ctx context.Context, nickname string) (string, error)

	// AddUserToDatabase adds new record to database using given user.
	AddUserToDatabase(ctx context.Context, user User) error

	// DeleteUserFromDatabase deletes all records which have user's email and nickname.
	DeleteUserFromDatabase(ctx context.Context, user User) error

	// GetUsersFromDatabase transforms all records from database to slice of User structs
	// and returns it.
	GetUsersFromDatabase(ctx context.Context) ([]User, error)
}

// dataHandler implements Data interface and used as its basic implementation.
type dataHandler struct {
	cache Cache
	db    *sql.DB
}

// User represents a user with certain nickname and email.
type User struct {
	Nickname string `json:"nickname"`
	Email    string `json:"email"`
}

// Operation represents a method which will be executed
// upon user
type Operation struct {
	User   User   `json:"user"`
	Method string `json:"method"`
}

// NewData creates a new Data instance using given Cache
// and DB.
func NewData(cache Cache, db *sql.DB) Data {
	return &dataHandler{cache, db}
}

// Disconnect closes connections to database and cache.
func (d *dataHandler) Disconnect() {
	d.db.Close()
	d.cache.Close()
}

// GetOperation gets JSON formatted value from cache by given key and returns decoded
// data as Operation struct. If there is no such key in cache, return empty Operation struct.
func (d *dataHandler) GetOperation(ctx context.Context, key string) (*Operation, error) {
	jsonData, err := d.cache.Get(ctx, key)
	if err != nil {
		if err == CacheNil {
			return &Operation{}, nil
		} else {
			return nil, err
		}
	}
	var opn Operation
	err = json.NewDecoder(strings.NewReader(jsonData)).Decode(&opn)
	if err != nil {
		return nil, err
	}
	return &opn, nil
}

// SetOperation composes given User and method into Operation, then encodes it into JSON formatted
// string. After this a base64-encoded key is generated randomly. Then JSON string is inserted
// into cache by the key.
func (d *dataHandler) SetOperation(ctx context.Context, user User, method string) (string, error) {
	opn := Operation{user, method}
	buf := new(strings.Builder)
	err := json.NewEncoder(buf).Encode(&opn)
	if err != nil {
		return "", err
	}
	keyBuf := make([]byte, keySize)
	_, err = rand.Read(keyBuf)
	if err != nil {
		return "", err
	}
	key := base64.URLEncoding.EncodeToString(keyBuf)
	err = d.cache.Set(ctx, key, buf.String(), authExpiration)
	if err != nil {
		return "", err
	}
	return key, nil
}

// CheckNicknameInDatabase checks whether the nickname in database or no. In first place, it
// checks cache. If there is no nickname in cache, the search continues in database. The database
// result is cached and returned.
func (d *dataHandler) CheckNicknameInDatabase(ctx context.Context, nickname string) (bool, error) {
	email, err := d.GetEmailByNickname(ctx, nickname)
	if err != nil {
		return false, nil
	}
	return email != "", nil
}

// GetEmailByNickname gets email of a user by given nickname. In first place, it checks cache.
// If there is no nickname in cache, the search continues in database. The database is cached.
// If no such nickname found in database or cache, returns empty string.
func (d *dataHandler) GetEmailByNickname(ctx context.Context, nickname string) (string, error) {
	email, err := d.cache.Get(ctx, nickname)
	if err == CacheNil {
		row := d.db.QueryRowContext(ctx, "SELECT email FROM Users WHERE nickname = $1", nickname)
		err = row.Scan(&email)
		if err == sql.ErrNoRows {
			email = ""
		} else if err != nil {
			return "", err
		}
		d.cache.Set(ctx, nickname, email, cacheExpiration)
		return email, nil
	} else if err != nil {
		return "", err
	}
	return email, nil
}

// AddUserToDatabase adds new user record into database. It also deletes record with ListUsersKey
// from cache because its' value is outdated (if the insertion succeeds).
func (d *dataHandler) AddUserToDatabase(ctx context.Context, user User) error {
	result, err := d.db.ExecContext(ctx, "INSERT INTO Users VALUES ($1, $2)", user.Nickname, user.Email)
	if rows, err := result.RowsAffected(); err == nil && rows > 0 {
		d.cache.Del(ctx, ListUsersKey)
	}
	return err
}

// DeleteUserFromDatabase deletes records of user from database. In case of success, it also
// deletes record with ListUsersKey from cache because its' value is outdated.
func (d *dataHandler) DeleteUserFromDatabase(ctx context.Context, user User) error {
	result, err := d.db.ExecContext(ctx, "DELETE FROM Users WHERE nickname=$1 AND email=$2", user.Nickname, user.Email)
	if rows, err := result.RowsAffected(); err == nil && rows > 0 {
		d.cache.Del(ctx, ListUsersKey)
	}
	return err
}

// GetUsersFromDatabase gets all users records from database or cache and returns them as
// User slice
func (d *dataHandler) GetUsersFromDatabase(ctx context.Context) ([]User, error) {
	var usersList []User
	jsonData, err := d.cache.Get(ctx, ListUsersKey)
	if err == CacheNil {
		if usersList, err = d.cacheMiss(ctx); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else if err := json.NewDecoder(strings.NewReader(jsonData)).Decode(&usersList); err != nil {
		return nil, err
	}
	return usersList, nil
}

// cacheMiss is called when GetUsersFromDatabase didn't found record with ListUsersKey in cache.
// It selects all rows from database and inserts them into User slice, then encodes the slice
// into JSON string and adds it into cache. After this cacheMiss returns created User slice.
func (d *dataHandler) cacheMiss(ctx context.Context) ([]User, error) {
	var usersList []User
	rows, err := d.db.QueryContext(ctx, "SELECT nickname, email FROM Users")
	defer rows.Close()
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.Nickname, &user.Email); err != nil {
			return nil, err
		}
		usersList = append(usersList, user)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	buf := new(strings.Builder)
	if err := json.NewEncoder(buf).Encode(&usersList); err != nil {
		return nil, err
	}
	if err := d.cache.Set(ctx, ListUsersKey, buf.String(), cacheExpiration); err != nil {
		return nil, err
	}
	return usersList, nil
}
