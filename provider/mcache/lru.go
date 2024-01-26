package mcache

import (
	"context"
	"fmt"

	lru "github.com/hashicorp/golang-lru"
	"github.com/tangelo-labs/go-cache"
)

type lruWrapper[T any] struct {
	inner *lru.Cache
}

// NewLRU creates an LRU of the given size.
func NewLRU[T any](size int) (cache.SimpleCache[T], error) {
	inner, err := lru.New(size)
	if err != nil {
		return nil, err
	}

	return &lruWrapper[T]{inner: inner}, nil
}

func (l *lruWrapper[T]) Get(_ context.Context, key string) (value T, err error) {
	var result T

	v, ok := l.inner.Get(key)
	if !ok {
		return result, fmt.Errorf("%w: key `%s` not found", cache.ErrItemNotFound, key)
	}

	return v.(T), nil
}

func (l *lruWrapper[T]) Put(_ context.Context, key string, value T) error {
	l.inner.Add(key, value)

	return nil
}

func (l *lruWrapper[T]) Has(_ context.Context, key string) (bool, error) {
	return l.inner.Contains(key), nil
}

func (l *lruWrapper[T]) Remove(_ context.Context, key string) (bool, error) {
	return l.inner.Remove(key), nil
}

func (l *lruWrapper[T]) Flush(_ context.Context) error {
	l.inner.Purge()

	return nil
}
