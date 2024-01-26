package cache

import "errors"

var (
	// ErrEncoding is returned by a Codec when it fails to encode a value.
	ErrEncoding = errors.New("failed to encode value")

	// ErrDecoding is returned by a Codec when it fails to decode a value.
	ErrDecoding = errors.New("failed to decode value")
)

// Codec represents a component that can encode and decode concrete message
// types.
type Codec[T any] interface {
	// Encode converts an arbitrary value type to bytes. It returns ErrEncoding
	// if the value cannot be encoded.
	Encode(data T) ([]byte, error)

	// Decode converts bytes to an arbitrary value type. It returns ErrDecoding
	// if the value cannot be decoded.
	Decode(data []byte) (T, error)
}
