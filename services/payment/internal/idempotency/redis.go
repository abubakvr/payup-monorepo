package idempotency

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

const keyPrefix = "idempotency:transfer:"
const defaultTTL = 24 * time.Hour

// Store provides idempotency for transfer requests. If Redis is nil, Get returns (nil, false) and Set is a no-op.
type Store struct {
	client *redis.Client
	ttl    time.Duration
}

// NewStore returns an idempotency store. If redisAddr is empty, returns a no-op store.
func NewStore(redisAddr, redisPassword string, ttl time.Duration) *Store {
	if redisAddr == "" {
		return &Store{}
	}
	if ttl <= 0 {
		ttl = defaultTTL
	}
	client := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       0,
	})
	return &Store{client: client, ttl: ttl}
}

// Get returns the cached response body for the idempotency key, or (nil, false) if not found or store disabled.
func (s *Store) Get(ctx context.Context, idempotencyKey string) (responseBody []byte, found bool) {
	if s.client == nil {
		return nil, false
	}
	key := keyPrefix + idempotencyKey
	data, err := s.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, false
	}
	if err != nil {
		return nil, false
	}
	return data, true
}

// Set stores the response body for the idempotency key with TTL. No-op if store disabled.
func (s *Store) Set(ctx context.Context, idempotencyKey string, responseBody []byte) {
	if s.client == nil {
		return
	}
	key := keyPrefix + idempotencyKey
	ttl := s.ttl
	if ttl <= 0 {
		ttl = defaultTTL
	}
	_ = s.client.Set(ctx, key, responseBody, ttl).Err()
}

// Ping returns true if Redis is configured and reachable.
func (s *Store) Ping(ctx context.Context) bool {
	if s.client == nil {
		return false
	}
	return s.client.Ping(ctx).Err() == nil
}
