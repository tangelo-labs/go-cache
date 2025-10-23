package rsmcache

import (
	"context"
	"fmt"
	"strings"

	lru "github.com/hashicorp/golang-lru"
	"github.com/redis/go-redis/v9"
	"github.com/tangelo-labs/go-cache"
	"github.com/tangelo-labs/go-domain"
)

const (
	removeCmdPrefix = "::REMOVE::"
	flushCmd        = "::FLUSH::"
)

type lruWrapper[T any] struct {
	instanceID      domain.ID
	ctx             context.Context
	inner           *lru.Cache
	encoder         cache.Codec[T]
	envelopeEncoder cache.Codec[*envelope]

	client       *redis.Client
	channelName  string
	subscription *redis.PubSub
}

type envelope struct {
	InstanceID domain.ID
	Key        string
	Payload    []byte
}

// NewLRU creates an LRU cache of the given size.
//
// The provided encoder is used to serialize and deserialize the values when
// they are transferred to and from Redis.
//
// The provided client is used to publish and subscribe to the channel. And,
// the channelName is the name of the channel used to publish and subscribe, you
// must ensure that the same channel name is used for all the instances of the
// LRU cache in your application.
//
// When the same key is written by multiple instances at the same time (parallel
// writes), the last write wins.
//
// Except for Get & Has operations, all other ones are handled on reaction to
// the channel subscription.
func NewLRU[T any](
	ctx context.Context,
	encoder cache.Codec[T],
	size int,
	client *redis.Client,
	channelName string,
) (cache.SimpleCache[T], error) {
	if pErr := client.Ping(ctx).Err(); pErr != nil {
		return nil, pErr
	}

	inner, err := lru.New(size)
	if err != nil {
		return nil, err
	}

	c := &lruWrapper[T]{
		instanceID:      domain.NewID(),
		ctx:             ctx,
		inner:           inner,
		encoder:         encoder,
		envelopeEncoder: cache.GobCodec[*envelope]{},

		client:       client,
		channelName:  channelName,
		subscription: client.Subscribe(ctx, channelName),
	}

	go c.run()

	return c, nil
}

func (c *lruWrapper[T]) Get(_ context.Context, key string) (value T, err error) {
	var result T

	v, ok := c.inner.Get(key)
	if !ok {
		return result, fmt.Errorf("%w: key `%s` not found", cache.ErrItemNotFound, key)
	}

	return v.(T), nil
}

func (c *lruWrapper[T]) Put(ctx context.Context, key string, value T) error {
	rawPrf, err := c.encoder.Encode(value)
	if err != nil {
		return fmt.Errorf("%w: tyring to encode key `%s`", cache.ErrEncoding, key)
	}

	msgRaw, err := c.envelopeEncoder.Encode(&envelope{
		InstanceID: c.instanceID,
		Key:        key,
		Payload:    rawPrf,
	})
	if err != nil {
		return fmt.Errorf("%w: trying to encode envelope for key `%s`", cache.ErrEncoding, key)
	}

	pipe := c.client.TxPipeline()

	if pErr := pipe.Publish(ctx, c.channelName, msgRaw).Err(); pErr != nil {
		return fmt.Errorf("%w: trying to publish key `%s`, details = %w", cache.ErrInvalidValue, key, pErr)
	}

	if _, eErr := pipe.Exec(ctx); eErr != nil {
		return fmt.Errorf("%w: trying to execute transaction for key `%s`, details = %w", cache.ErrInvalidValue, key, eErr)
	}

	c.inner.Add(key, value)

	return nil
}

func (c *lruWrapper[T]) Has(_ context.Context, key string) (bool, error) {
	return c.inner.Contains(key), nil
}

func (c *lruWrapper[T]) Remove(ctx context.Context, key string) (bool, error) {
	pipe := c.client.TxPipeline()

	exists := c.inner.Remove(key)

	msgRaw, err := c.envelopeEncoder.Encode(&envelope{
		InstanceID: c.instanceID,
		Key:        fmt.Sprintf("%s%s", removeCmdPrefix, key),
	})
	if err != nil {
		return false, fmt.Errorf("%w: trying to encode envelope for key `%s`", cache.ErrEncoding, key)
	}

	if pErr := pipe.Publish(ctx, c.channelName, msgRaw).Err(); pErr != nil {
		return false, fmt.Errorf("%w: trying to publish remove key `%s`, details = %w", cache.ErrInvalidValue, key, pErr)
	}

	if _, eErr := pipe.Exec(ctx); eErr != nil {
		return false, fmt.Errorf("%w: trying to execute transaction for removing key `%s`, details = %w", cache.ErrInvalidValue, key, eErr)
	}

	return exists, nil
}

func (c *lruWrapper[T]) Flush(ctx context.Context) error {
	pipe := c.client.TxPipeline()

	key := flushCmd
	c.inner.Purge()

	msgRaw, err := c.envelopeEncoder.Encode(&envelope{
		InstanceID: c.instanceID,
		Key:        key,
	})
	if err != nil {
		return fmt.Errorf("%w: trying to encode envelope for key `%s`", cache.ErrEncoding, key)
	}

	if pErr := pipe.Publish(ctx, c.channelName, msgRaw).Err(); pErr != nil {
		return fmt.Errorf("%w: trying to publish flush, details = %w", cache.ErrInvalidValue, pErr)
	}

	if _, eErr := pipe.Exec(ctx); eErr != nil {
		return fmt.Errorf("%w: trying to execute transaction for key `%s`, details = %w", cache.ErrInvalidValue, key, eErr)
	}

	return nil
}

func (c *lruWrapper[T]) run() {
	done := c.ctx.Done()
	changes := c.subscription.Channel()

	for {
		select {
		case <-done:
			if cErr := c.subscription.Close(); cErr != nil {
				fmt.Printf("error closing redis pubsub subscription: %s\n", cErr)
			}

			if cErr := c.client.Close(); cErr != nil {
				fmt.Printf("error closing redis client: %s\n", cErr)
			}

			return
		case m := <-changes:
			env, err := c.resolveEnvelope(m)
			if err != nil {
				fmt.Printf("error decoding envelope: %s\n", err)

				continue
			}

			if env.InstanceID == c.instanceID {
				continue
			}

			switch {
			case strings.HasPrefix(env.Key, removeCmdPrefix):
				key := strings.TrimPrefix(env.Key, removeCmdPrefix)
				c.inner.Remove(key)

				continue
			case env.Key == flushCmd:
				c.inner.Purge()

				continue
			}

			value, err := c.resolveValue(env.Payload)
			if err != nil {
				fmt.Printf("error decoding payload: %s\n", err)

				continue
			}

			c.inner.Add(env.Key, value)
		}
	}
}

// resolveValue assumes that the given messages holds as payload the name of the
// key that was changed. Then it tries to get the value for that key and decode
// it using the encoder.
func (c *lruWrapper[T]) resolveValue(sValue []byte) (T, error) {
	var result T

	result, err := c.encoder.Decode(sValue)
	if err != nil {
		return result, fmt.Errorf("%w: error decoding value `%s`, details = %w", cache.ErrDecoding, sValue, err)
	}

	return result, nil
}

func (c *lruWrapper[T]) resolveEnvelope(m *redis.Message) (*envelope, error) {
	env, err := c.envelopeEncoder.Decode([]byte(m.Payload))
	if err != nil {
		return env, fmt.Errorf("%w: error decoding envelope `%s`, details = %w", cache.ErrDecoding, m.Payload, err)
	}

	return env, nil
}
