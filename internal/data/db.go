package data

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "github.com/lib/pq"
)

// DB interface represents a database with User scheme.
type DB interface {
    // GetEmailByNickname returns email responding to given nickname. If there is
    // no such nickname, returns empty string.
    GetEmailByNickname(ctx context.Context, nickname string) (string, error)

    // InsertUser inserts a new record with given User data to database.
    // Returns a boolean value if the insertion affected any rows in the DB.
    InsertUser(ctx context.Context, user User) (bool, error)

    // DeleteUser removes records for given User data from database.
    // Return a boolean value if the insertion affected any rows in the DB.
    DeleteUser(ctx context.Context, user User) (bool, error)

    // SelectAllUsers returns a slice of User according to rows' data in the DB.
    SelectAllUsers(ctx context.Context) ([]User, error)
}

// PgsDB implements DB interface with PostgreSQL database.
type PgsDB struct {
    db *sql.DB
}

// ConnectToPGS connects to PostgreSQL database, using given file.
// If connection or initialization of tables were failed, returns error.
func NewPgsDB(pgsInfoFile string) (*PgsDB, error) {
	var err error
	if !filepath.IsAbs(pgsInfoFile) {
		pgsInfoFile, err = filepath.Abs(pgsInfoFile)
		if err != nil {
			return nil, err
		}
	}
	pgsInfo, err := os.ReadFile(pgsInfoFile)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("postgres", string(pgsInfo))
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	if err := createUsersTable(db); err != nil {
		db.Close()
		return nil, err
	}
    return &PgsDB{db: db}, nil
}

// createUsersTable executes a CREATE TABLE query to create necessary Users table.
func createUsersTable(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS Users (` +
		`nickname TEXT,` +
		`email TEXT,` +
		`UNIQUE (nickname));`)
	return err
}

// GetEmailByNickname returns email address responding to given nickname.
// If there is no user with such nickname, returns empty string.
func (pdb *PgsDB) GetEmailByNickname(ctx context.Context, nickname string) (string, error) {
    var email string
    row := pdb.db.QueryRowContext(ctx, "SELECT email FROM Users WHERE nickname = $1", nickname)
    err := row.Scan(&email)
    if err == sql.ErrNoRows {
        email = ""
    } else if err != nil {
        return "", err
    }
    return email
}

// InsertUser inserts a new record for given user to database and returns
// true if the query affected any rows.
func (pdb *PgsDB) InsertUser(ctx context.Context, user User) (bool, error) {
    result, err := pdb.db.ExecContext(ctx, "INSERT INTO Users VALUES ($1, $2)", user.Nickname, user.Email)
    if err != nil {
        return false, err
    }
    rows, err := result.RowsAffected()
    if err != nil {
        return false, err
    }
    return rows > 0, nil
}

// DeleteUser deletes record for user from database and returns true
// if the query affected any rows.
func (pdb *PgsDB) DeleteUser(ctx context.Context, user User) (bool, error) {
    result, err := pdb.db.ExecContext(ctx, "DELETE FROM Users WHERE nickname=$1 AND email=$2", user.Nickname, user.Email)
    if err != nil {
        return false, err
    }
    rows, err := result.RowsAffected()
    if err != nil {
        return false, err
    }
    return rows > 0, nil
}

// SelectAllUsers returns a slice of User according to all records from database.
func (pdb *PgsDB) SelectAllUsers(ctx context.Context) ([]User, error) {
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
    return usersList, nil
}
