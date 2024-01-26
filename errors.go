package cache

import "errors"

var (
	// ErrItemNotFound whenever an entry is not found in the cache system.
	ErrItemNotFound = errors.New("item not found")

	// ErrInvalidValue usually returned when trying to encode/decode a value.
	ErrInvalidValue = errors.New("invalid value")
)
