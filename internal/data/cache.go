package data

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
)

// CacheNil defines an error which is returned when no value
// responds to a key.
const CacheNil = CacheError("cache: nil")

type CacheError string

func (e CacheError) Error() string {
	return string(e)
}

// Cache interface represents a key-value cache.
type Cache interface {
	// Get returns a value responding to given key. If there is no value,
	// a CacheNil error should be returned.
	Get(ctx context.Context, key string) (string, error)

	// Set creates a key-value pair in the cache with given expiration time.
	Set(ctx context.Context, key, value string, expiration time.Duration) error

	// Del deletes a key-value pair from the cache by given key.
	Del(ctx context.Context, key string) error

	// Close closes connection with the cache, releasing resources.
	Close()
}

// RedisCache implements Cache interface with Redis client.
type RedisCache struct {
	cache *redis.Client
}

// NewRedisCache create a new RedisCache instance, using given string
// as the address of Redis cache. If connection to Redis fails, returns an error.
func NewRedisCache(redisAddress string) (*RedisCache, error) {
	rc := new(RedisCache)

	rc.cache = redis.NewClient(&redis.Options{
		Addr:     redisAddress,
		Password: "",
		DB:       0,
	})
	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()
	err := rc.cache.Ping(ctx).Err()
	if err != nil {
		return nil, err
	}
	return rc, nil
}

// Get searches for a value responding to given key. If there is no value,
// return CacheNil error.
func (rc *RedisCache) Get(ctx context.Context, key string) (string, error) {
	value, err := rc.cache.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", CacheNil
	} else if err != nil {
		return "", err
	}
	return value, nil
}

// Set creates a key-value pair in the cache, which will disappear when given time expires.
func (rc *RedisCache) Set(ctx context.Context, key, value string, expiration time.Duration) error {
	return rc.cache.Set(ctx, key, value, expiration).Err()
}

// Del deletes a key-value pair from the Redis cache.
func (rc *RedisCache) Del(ctx context.Context, key string) error {
	return rc.cache.Del(ctx, key).Err()
}

// Close closes connection with Redis cache.
func (rc *RedisCache) Close() {
	rc.cache.Close()
}
