package redis

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

const userExistsKeyPrefix = "user:exists:"

var rdb *redis.Client

func getRedisAddr() string {
	if a := os.Getenv("REDIS_ADDR"); a != "" {
		return a
	}
	return "redis:6379"
}

func InitRedis() {
	rdb = redis.NewClient(&redis.Options{
		Addr:     getRedisAddr(),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})

	ctx := context.Background()
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		log.Fatalf("failed to connect to redis: %v", err)
	}
	log.Println("connected to redis")
}

// GetUserExists returns (exists, foundInCache). Only "exists" is cached; if foundInCache is false, caller should check DB.
func GetUserExists(ctx context.Context, userID string) (exists bool, foundInCache bool) {
	if rdb == nil {
		return false, false
	}
	key := userExistsKeyPrefix + userID
	val, err := rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, false
	}
	if err != nil {
		return false, false
	}
	return val == "1", true
}

// SetUserExists caches that the user exists. TTL recommended: 15–30 min (e.g. 900s). Call after DB confirms user exists.
func SetUserExists(ctx context.Context, userID string, ttl time.Duration) {
	if rdb == nil || ttl <= 0 {
		return
	}
	key := userExistsKeyPrefix + userID
	_ = rdb.Set(ctx, key, "1", ttl).Err()
}

func ProcessTransaction(ctx context.Context, transactionID string, fn func() error) error {
	ok, err := rdb.SetNX(ctx, transactionID, "processing", 60*time.Minute).Result()
	if err != nil {
		return err
	}

	if !ok {
		return nil
	}

	err = fn()
	if err != nil {
		rdb.Del(ctx, transactionID)
		return err
	}

	rdb.Set(ctx, transactionID, "Done", 24*time.Hour)
	return nil
}
