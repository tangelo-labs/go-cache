package cache

import (
	"bytes"
	"encoding/gob"
)

var _ Codec[any] = GobCodec[any]{}

// GobCodec is a codec that uses gob package for encoding and decoding objects.
type GobCodec[T any] struct{}

// Encode converts an arbitrary value type to bytes.
func (b GobCodec[T]) Encode(data T) ([]byte, error) {
	var buff bytes.Buffer

	if err := gob.NewEncoder(&buff).Encode(data); err != nil {
		return nil, err
	}

	return buff.Bytes(), nil
}

// Decode converts bytes to an arbitrary value type.
func (b GobCodec[T]) Decode(data []byte) (T, error) {
	var result T

	if err := gob.NewDecoder(bytes.NewBuffer(data)).Decode(&result); err != nil {
		return result, err
	}

	return result, nil
}
