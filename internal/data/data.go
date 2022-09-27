package data

import (
    "context"
    "crypto/rand"
    "database/sql"
    "encoding/base64"
    "encoding/json"
    "time"
    "strings"

    _ "github.com/lib/pq"

    "github.com/go-redis/redis/v9"
)

const (
    keySize = 128
    authExpiration time.Duration = 15 * time.Minute
    cacheExpiration time.Duration = time.Minute
)

type Data struct {
    cache *redis.Client
    db *sql.DB
}

type User struct {
    Nickname string `json: "nickname"`
    Email string `json: "email"`
}

type Operation struct {
    User *User `json: "user"`
    Method string `json: "method"`
}

func (d *Data) Connect(redisPort, pgsPort int) {
    d.cache = redis.NewClient(&redisOptions{
        Addr: fmt.Sprintf("localhost: %d", redisPort),
        Password: "",
        DB: 0,
    }

    pgsFile, err := os.Open(pgsParamsFile)
}

func (d *Data) GetOperation(ctx context.Context, key string) (Operation, error) {
    jsonData, err := d.cache.Get(ctx, key).Result()
    if err != nil {
        if err == redis.Nil {
            return &Operation{}, nil
        } else {
            return nil, err
        }
    }
    var opn Operation
    err = json.NewDecoder(strings.NewReader(jsonData.String())).Decode(&opn)
    if err != nil {
        return nil, err
    }
    return &opn, nil
}

func (d *Data) SetOperation(ctx context.Context, user User, method string) (string, error) {
    opn := Operation{&user, method}
    buf := new(strings.Builder) 
    err := json.NewEncoder(buf).Encode(&opn)
    if err != nil {
        return "", err
    }
    keyBuf := make([]byte, keySize)
    _, err = rand.Read(key)
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

func (d *Data) CheckNicknameInDatabase(ctx context.Context, nickname string) (bool, error) {
    result, err := d.cache.Get(ctx, nickname).Bool()
    if err == redis.Nil {
        rows, err := d.db.QueryContext(ctx, "SELECT nickname FROM Users WHERE nickname = ?", nickname)        
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

func (d *Data) AddUserToDatabase(ctx context.Context, user User) error {
    _, err := d.db.ExecContext(ctx, "INSERT INTO Users VALUES ($1, $2)", user.Nickname, user.Email)
    return err
}

func (d *Data) DeleteUserFromDatabase(ctx context.Context, user User) error {
    defer cancel()
    _, err := d.db.ExecContext(ctx, "DELETE FROM Users WHERE nickname=$1 AND email=$2", user.Nickname, user.Email)
    return err
}

func (d *Data) GetUsersFromDatabase() ([]User, error) {
    var usersList []User
    jsonData, err := d.cache.Get(ctx, "UsersList") 
    if err == redis.Nil {
        if usersList, err = d.cacheMiss(); err != nil {
            return nil, err
        }
    } else if err != nil {
        return nil, err
    } else if err := json.NewDecoder(strings.NewReader(jsonData)).Decode(&usersList); err != nil {
        return nil, err
    }
    return usersList
}

func (d *Data) cacheMiss() ([]User, error) {
    var usersList []User
    var rows *sql.Rows
    rows, err = d.db.QueryContext(ctx, "SELECT nickname, email FROM Users")
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
    if err := d.cache.Set(ctx, "UsersList", buf.String(), cacheExpiration); err != nil {
        return nil, err
    }
    return usersList, nil 
}
    

