package cache

import "context"

// SimpleCache is a generic simple cache definition.
type SimpleCache[T any] interface {
	// Get returns key's value from the cache. It returns ErrItemNotFound when
	// the requested key does not exist.
	Get(ctx context.Context, key string) (value T, err error)

	// Put adds a value to the cache.
	Put(ctx context.Context, key string, value T) error

	// Has checks if a key exists in cache.
	Has(ctx context.Context, key string) (bool, error)

	// Remove deletes the given key. It returns true if the key was found and
	// deleted, otherwise it returns false.
	Remove(ctx context.Context, key string) (bool, error)

	// Flush deletes all keys in the cache.
	Flush(ctx context.Context) error
}
