package data

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "github.com/lib/pq"
)

// ConnectToPGS connects to PostgreSQL database, using given file.
// If connection or initialization of tables were failed, returns error.
func ConnectToPGS(pgsInfoFile string) (*sql.DB, error) {
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
	return db, nil
}

// createUsersTable executes a CREATE TABLE query to create necessary Users table.
func createUsersTable(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS Users (` +
		`nickname TEXT,` +
		`email TEXT,` +
		`UNIQUE (nickname));`)
	return err
}
