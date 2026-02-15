package redis

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

var rdb *redis.Client

func InitRedis() {
	rdb = redis.NewClient(&redis.Options{
		Addr:     "payup-redis:6379",
		Password: "",
		DB:       0,
	})

	ctx := context.Background()
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		log.Fatalf("failed to connect to redis: %v", err)
	}
	log.Println("connected to redis")
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
