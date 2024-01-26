package rscache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/tangelo-labs/go-cache"
)

type redisCache[T any] struct {
	client    *redis.Client
	encoder   cache.Codec[T]
	ttl       time.Duration
	keyPrefix string
}

// NewRedisCache builds a new cache interface that uses Redis as a backend.
func NewRedisCache[T any](client *redis.Client, encoder cache.Codec[T], ttl time.Duration, keyPrefix string) cache.SimpleCache[T] {
	return &redisCache[T]{
		client:    client,
		encoder:   encoder,
		ttl:       ttl,
		keyPrefix: keyPrefix,
	}
}

func (r redisCache[T]) Get(ctx context.Context, key string) (T, error) {
	var result T

	rawPrf, err := r.client.Get(ctx, r.arrangeKey(key)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return result, fmt.Errorf("%w: trying to get key %s", cache.ErrItemNotFound, key)
		}

		return result, err
	}

	prf, err := r.encoder.Decode([]byte(rawPrf))
	if err != nil {
		return result, fmt.Errorf("%w: trying to decode key %s", cache.ErrDecoding, key)
	}

	return prf, nil
}

func (r redisCache[T]) Put(ctx context.Context, key string, value T) error {
	rawPrf, err := r.encoder.Encode(value)
	if err != nil {
		return fmt.Errorf("%w: tyring to encode key %s", cache.ErrEncoding, key)
	}

	if sErr := r.client.Set(ctx, r.arrangeKey(key), rawPrf, r.ttl).Err(); sErr != nil {
		return sErr
	}

	return nil
}

func (r redisCache[T]) Has(ctx context.Context, key string) (bool, error) {
	if _, err := r.Get(ctx, key); err != nil {
		if errors.Is(err, cache.ErrItemNotFound) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func (r redisCache[T]) Remove(ctx context.Context, key string) (bool, error) {
	cmd, err := r.client.Del(ctx, r.arrangeKey(key)).Result()
	if err != nil {
		return false, err
	}

	if cmd == 0 {
		return false, nil
	}

	return true, nil
}

func (r redisCache[T]) Flush(ctx context.Context) error {
	if err := r.client.FlushDB(ctx).Err(); err != nil {
		return err
	}

	return nil
}

func (r redisCache[T]) arrangeKey(key string) string {
	return fmt.Sprintf("%s:%s", r.keyPrefix, key)
}
