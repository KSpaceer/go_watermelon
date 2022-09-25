package data

import (
    "context"
    "crypto/rand"
    "database/sql"
    "encoding/base64"
    "encoding/json"
    "time"
    "strings"

    "github.com/go-redis/redis/v9"
)

const (
    keySize = 128
    expiration time.Duration = 15 * time.Minute
)

type Data struct {
    cache *redis.Client
    db *sql.DB
}

type User struct {
    nickname, email string
}

type Operation struct {
    User *User
    Method string
}

func (d *Data) GetOperation(key string) (Operation, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 1 * time.Second) 
    defer cancel()
    jsonData, err := d.cache.Get(ctx, key)
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

func (d *Data) SetOperation(user User, method string) error {
    opn := Operation{&user, method}
    buf := new(strings.Builder) 
    err := json.NewEncoder(buf).Encode(&opn)
    if err != nil {
        return err
    }
    ctx, cancel := context.WithTimeout(context.Background(), 1 * time.Second)
    defer cancel()
    key := make([]byte, keySize)
    _, err = rand.Read(key)
    if err != nil {
        return err
    }
    err = d.cache.Set(ctx, base64.URLEncoding.EncodeToString(key), buf.String(), expiration).Err()
    return err
}
