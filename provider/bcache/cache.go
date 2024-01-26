package bcache

import (
	"context"
	"errors"
	"fmt"

	"github.com/allegro/bigcache"
	"github.com/tangelo-labs/go-cache"
)

type bigCache[T any] struct {
	db      *bigcache.BigCache
	encoder cache.Codec[T]
}

// NewBigCache adapts a BigCache instance to an implementation of
// cache.SimpleCache interface.
func NewBigCache[T any](db *bigcache.BigCache, encoder cache.Codec[T]) cache.SimpleCache[T] {
	return &bigCache[T]{
		db:      db,
		encoder: encoder,
	}
}

func (b bigCache[T]) Get(_ context.Context, key string) (T, error) {
	var result T

	value, err := b.db.Get(key)
	if err != nil {
		if errors.Is(err, bigcache.ErrEntryNotFound) {
			return result, fmt.Errorf("%w: trying to get key %s", cache.ErrItemNotFound, key)
		}

		return result, err
	}

	decoded, err := b.encoder.Decode(value)
	if err != nil {
		return result, fmt.Errorf("%w: trying to decode key %s, %w", cache.ErrInvalidValue, key, err)
	}

	return decoded, nil
}

func (b bigCache[T]) Put(_ context.Context, key string, val T) error {
	rawVal, err := b.encoder.Encode(val)
	if err != nil {
		return fmt.Errorf("%w: tyring to encode key %s with value %v", cache.ErrInvalidValue, key, val)
	}

	if sErr := b.db.Set(key, rawVal); sErr != nil {
		return sErr
	}

	return nil
}

func (b bigCache[T]) Has(ctx context.Context, key string) (bool, error) {
	if _, err := b.Get(ctx, key); err != nil {
		if errors.Is(err, cache.ErrItemNotFound) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func (b bigCache[T]) Remove(_ context.Context, key string) (bool, error) {
	if err := b.db.Delete(key); err != nil {
		if errors.Is(err, bigcache.ErrEntryNotFound) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func (b bigCache[T]) Flush(_ context.Context) error {
	return b.db.Reset()
}
