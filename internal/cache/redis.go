package cache

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

type Redis struct {
	client redis.UniversalClient
}

func NewRedis(client redis.UniversalClient) *Redis {
	return &Redis{client: client}
}

func (r *Redis) Get(ctx context.Context, key string) ([]byte, error) {
	val, err := r.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrMiss
	}
	return val, err
}

func (r *Redis) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl <= 0 {
		return r.client.Del(ctx, key).Err()
	}
	return r.client.Set(ctx, key, value, ttl).Err()
}

func (r *Redis) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

func (r *Redis) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

func (r *Redis) Close() error { return r.client.Close() }
