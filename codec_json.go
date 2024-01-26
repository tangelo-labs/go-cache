package cache

import (
	"encoding/json"
)

var _ Codec[any] = JSONCodec[any]{}

// JSONCodec is a codec that uses json package for encoding and decoding
// objects.
type JSONCodec[T any] struct{}

// Encode converts an arbitrary value type to bytes.
func (b JSONCodec[T]) Encode(data T) ([]byte, error) {
	return json.Marshal(data)
}

// Decode converts bytes to an arbitrary value type.
func (b JSONCodec[T]) Decode(data []byte) (T, error) {
	var result T

	err := json.Unmarshal(data, &result)

	return result, err
}
