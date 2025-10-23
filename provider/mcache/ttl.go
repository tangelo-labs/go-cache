package mcache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jellydator/ttlcache/v2"
	"github.com/tangelo-labs/go-cache"
)

// UnlimitedItems is a special value that can be passed to NewTTL to indicate.
const UnlimitedItems = -1

type ttlWrapper[T any] struct {
	d     time.Duration
	inner *ttlcache.Cache
	mu    sync.RWMutex
}

// NewTTL builds a cache handler that will expire items after a given duration.
// If refreshTTLOnHit is true, the TTL will be reset on every hit.
// If itemsLimit is greater than 0, the cache will be limited to that number of items.
func NewTTL[T any](ttl time.Duration, itemsLimit int, refreshTTLOnHit bool) cache.SimpleCache[T] {
	c := ttlcache.NewCache()
	c.SkipTTLExtensionOnHit(!refreshTTLOnHit)

	if itemsLimit > 0 {
		c.SetCacheSizeLimit(itemsLimit)
	}

	return &ttlWrapper[T]{
		inner: c,
		d:     ttl,
	}
}

func (t *ttlWrapper[T]) Get(_ context.Context, key string) (value T, err error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.get(key)
}

func (t *ttlWrapper[T]) Put(_ context.Context, key string, value T) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.inner.SetWithTTL(key, value, t.d)
}

func (t *ttlWrapper[T]) Has(_ context.Context, key string) (bool, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	exists, err := t.has(key)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func (t *ttlWrapper[T]) Remove(_ context.Context, key string) (bool, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	exists, err := t.has(key)
	if err != nil {
		return false, err
	}

	if err := t.inner.Remove(key); err != nil {
		return false, err
	}

	return exists, nil
}

func (t *ttlWrapper[T]) Flush(_ context.Context) error {
	return t.inner.Purge()
}

func (t *ttlWrapper[T]) get(key string) (T, error) {
	var result T

	v, err := t.inner.Get(key)
	if err != nil {
		if errors.Is(err, ttlcache.ErrNotFound) {
			return result, fmt.Errorf("%w: key `%s` not found", cache.ErrItemNotFound, key)
		}

		return result, err
	}

	return v.(T), nil
}

func (t *ttlWrapper[T]) has(key string) (bool, error) {
	if _, err := t.get(key); err != nil {
		if errors.Is(err, cache.ErrItemNotFound) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}
