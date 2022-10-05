package data

import (
    "context"
    "crypto/rand"
    "database/sql"
    "encoding/base64"
    "encoding/json"
    "time"
    "strings"
    "os"
    "path/filepath"

    _ "github.com/lib/pq"

    "github.com/go-redis/redis/v8"
)

const (
    keySize = 128
    authExpiration time.Duration = 15 * time.Minute
    cacheExpiration time.Duration = time.Minute
)

type Data interface {
    Disconnect()
    GetOperation(ctx context.Context, key string) (Operation, error)
    SetOperation(ctx context.Context, user User, method string) (string, error)
    CheckNicknameInDatabase(ctx context.Context, nickname string) (bool, error)
    AddUserToDatabase(ctx context.Context, user User) error
    DeleteUserFromDatabase(ctx context.Context, user User) error
    GetUsersFromDatabase(ctx context.Context) ([]User, error)
}

type postgresRedisData struct {
    cache *redis.Client
    db *sql.DB
}

type User struct {
    Nickname string `json: "nickname"`
    Email string `json: "email"`
}

type Operation struct {
    User User `json: "user"`
    Method string `json: "method"`
}

func NewPGSRedisData(redisAddress, pgsInfoFile string) (*postgresRedisData, error) {
    d := new(postgresRedisData)
    d.cache = redis.NewClient(&redis.Options{
        Addr: redisAddress,
        Password: "",
        DB: 0,
    })
    _, err := d.cache.Ping(context.Background()).Result()
    if err != nil {
        return nil, err
    }
    if !filepath.IsAbs(pgsInfoFile) {
        pgsInfoFile, err = filepath.Abs(pgsInfoFile) 
        if err != nil {
            return nil, err
        }
    }
    pgsInfo, err := os.ReadFile(pgsInfoFile)
    d.db, err = sql.Open("postgres", string(pgsInfo))
    if err != nil {
        return nil, err
    }
    if err := d.db.Ping(); err != nil {
        return nil, err
    }
    if err := d.createUsersTable(); err != nil {
        return nil, err
    }
    return d, nil
}

func (d *postgresRedisData) createUsersTable() error {
    _, err := d.db.Exec(`CREATE TABLE IF NOT EXISTS Users (nickname TEXT, email TEXT);`)
    if err == nil {
        _, err = d.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS nickname_idx ON Users(nickname);`)
    }
    return err
}

func (d *postgresRedisData) Disconnect() {
    d.db.Close()
    d.cache.Close()
}

func (d *postgresRedisData) GetOperation(ctx context.Context, key string) (*Operation, error) {
    jsonData, err := d.cache.Get(ctx, key).Result()
    if err != nil {
        if err == redis.Nil {
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

func (d *postgresRedisData) SetOperation(ctx context.Context, user User, method string) (string, error) {
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
    err = d.cache.Set(ctx, key, buf.String(), authExpiration).Err()
    if err != nil {
        return "", err
    }
    return key, nil
}

func (d *postgresRedisData) CheckNicknameInDatabase(ctx context.Context, nickname string) (bool, error) {
    result, err := d.cache.Get(ctx, nickname).Bool()
    if err == redis.Nil {
        rows, err := d.db.QueryContext(ctx, "SELECT nickname FROM Users WHERE nickname = $1", nickname)        
        defer rows.Close()
        if err != nil {
            return false, err
        }
        result = rows.Next()
        _ = d.cache.Set(ctx, nickname, result, cacheExpiration)
    } else if err != nil {
        return false, err
    }
    return result, err
}

func (d *postgresRedisData) AddUserToDatabase(ctx context.Context, user User) error {
    _, err := d.db.ExecContext(ctx, "INSERT INTO Users VALUES ($1, $2)", user.Nickname, user.Email)
    return err
}

func (d *postgresRedisData) DeleteUserFromDatabase(ctx context.Context, user User) error {
    _, err := d.db.ExecContext(ctx, "DELETE FROM Users WHERE nickname=$1 AND email=$2", user.Nickname, user.Email)
    return err
}

func (d *postgresRedisData) GetUsersFromDatabase(ctx context.Context) ([]User, error) {
    var usersList []User
    jsonData, err := d.cache.Get(ctx, "UsersList").Result()
    if err == redis.Nil {
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

func (d *postgresRedisData) cacheMiss(ctx context.Context) ([]User, error) {
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
    if err := d.cache.Set(ctx, "UsersList", buf.String(), cacheExpiration).Err(); err != nil {
        return nil, err
    }
    return usersList, nil 
}
    

