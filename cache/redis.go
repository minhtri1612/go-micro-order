package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	redisClient *redis.Client
	ctx         = context.Background()
)

// InitRedis initializes Redis connection
func InitRedis() error {
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "redis" // Docker default
	}

	redisClient = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:6379", redisHost),
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	// Test connection
	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return nil
}

// Get retrieves a value from cache
func Get(key string, value interface{}) error {
	data, err := redisClient.Get(ctx, key).Result()
	if err == redis.Nil {
		return fmt.Errorf("key does not exist")
	} else if err != nil {
		return err
	}

	return json.Unmarshal([]byte(data), value)
}

// Set stores a value in cache with expiration
func Set(key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return redisClient.Set(ctx, key, data, expiration).Err()
}

// Delete removes a key from cache
func Delete(key string) error {
	return redisClient.Del(ctx, key).Err()
}

// GetOrSet retrieves value from cache or sets it if not exists
func GetOrSet(key string, value interface{}, expiration time.Duration, fn func() (interface{}, error)) error {
	// Try to get from cache first
	err := Get(key, value)
	if err == nil {
		return nil
	}

	// If not in cache, call the function
	result, err := fn()
	if err != nil {
		return err
	}

	// Store result in cache
	if err := Set(key, result, expiration); err != nil {
		return err
	}

	// Update the value reference
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, value)
}


func Close() error {
	if redisClient != nil {
		return redisClient.Close()
	}
	return nil
}

// Flush clears all keys in the current DB (useful for testing)
func Flush() error {
	if redisClient != nil {
		return redisClient.FlushDB(ctx).Err()
	}
	return nil
}