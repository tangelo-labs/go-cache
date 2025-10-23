package rsmcache

import (
	"context"
	"fmt"
	"strings"

	lru "github.com/hashicorp/golang-lru"
	"github.com/redis/go-redis/v9"
	"github.com/tangelo-labs/go-cache"
)

const (
	removeCmdPrefix = "::REMOVE::"
	flushCmd        = "::FLUSH::"
)

type lruWrapper[T any] struct {
	ctx     context.Context
	inner   *lru.Cache
	encoder cache.Codec[T]

	client       *redis.Client
	channelName  string
	subscription *redis.PubSub
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
		ctx:     ctx,
		inner:   inner,
		encoder: encoder,

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

	pipe := c.client.TxPipeline()

	if sErr := pipe.Set(ctx, key, rawPrf, 0).Err(); sErr != nil {
		return fmt.Errorf("%w: trying to set key `%s`, details = %w", cache.ErrInvalidValue, key, sErr)
	}

	if pErr := pipe.Publish(ctx, c.channelName, key).Err(); pErr != nil {
		return fmt.Errorf("%w: trying to publish key `%s`, details = %w", cache.ErrInvalidValue, key, pErr)
	}

	if _, eErr := pipe.Exec(ctx); eErr != nil {
		return fmt.Errorf("%w: trying to execute transaction for key `%s`, details = %w", cache.ErrInvalidValue, key, eErr)
	}

	return nil
}

func (c *lruWrapper[T]) Has(_ context.Context, key string) (bool, error) {
	return c.inner.Contains(key), nil
}

func (c *lruWrapper[T]) Remove(ctx context.Context, key string) (bool, error) {
	pipe := c.client.TxPipeline()

	removeKey := fmt.Sprintf("%s%s", removeCmdPrefix, key)

	var errPipe error
	defer func() {
		if errPipe != nil {
			c.inner.Remove(key)
		}
	}()

	exists := c.inner.Contains(key)

	if sErr := pipe.Set(ctx, removeKey, "", 0).Err(); sErr != nil {
		errPipe = sErr

		return false, fmt.Errorf("%w: trying to set remove key `%s`, details = %w", cache.ErrInvalidValue, key, sErr)
	}

	if pErr := pipe.Publish(ctx, c.channelName, removeKey).Err(); pErr != nil {
		errPipe = pErr

		return false, fmt.Errorf("%w: trying to publish remove key `%s`, details = %w", cache.ErrInvalidValue, key, pErr)
	}

	if _, eErr := pipe.Exec(ctx); eErr != nil {
		errPipe = eErr

		return false, fmt.Errorf("%w: trying to execute transaction for removing key `%s`, details = %w", cache.ErrInvalidValue, key, eErr)
	}

	return exists, nil
}

func (c *lruWrapper[T]) Flush(ctx context.Context) error {
	pipe := c.client.TxPipeline()

	key := flushCmd

	var errPipe error
	defer func() {
		if errPipe != nil {
			c.inner.Purge()
		}
	}()

	if sErr := pipe.Set(ctx, key, "", 0).Err(); sErr != nil {
		errPipe = sErr

		return fmt.Errorf("%w: trying to set key `%s`, details = %w", cache.ErrInvalidValue, key, sErr)
	}

	if pErr := pipe.Publish(ctx, c.channelName, key).Err(); pErr != nil {
		errPipe = pErr

		return fmt.Errorf("%w: trying to publish key `%s`, details = %w", cache.ErrInvalidValue, key, pErr)
	}

	if _, eErr := pipe.Exec(ctx); eErr != nil {
		errPipe = eErr

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
			switch {
			case strings.HasPrefix(m.Payload, removeCmdPrefix):
				key := strings.TrimPrefix(m.Payload, removeCmdPrefix)
				c.inner.Remove(key)

				continue
			case m.Payload == flushCmd:
				c.inner.Purge()

				continue
			}

			value, err := c.resolveValue(m)
			if err != nil {
				fmt.Printf("error decoding payload: %s\n", err)

				continue
			}

			c.inner.Add(m.Payload, value)
		}
	}
}

// resolveValue assumes that the given messages holds as payload the name of the
// key that was changed. Then it tries to get the value for that key and decode
// it using the encoder.
func (c *lruWrapper[T]) resolveValue(m *redis.Message) (T, error) {
	var result T

	changedKey := m.Payload
	sValue, err := c.client.Get(c.ctx, changedKey).Result()

	if err != nil {
		return result, fmt.Errorf("%w: error getting value for key `%s`, details = %w", cache.ErrDecoding, changedKey, err)
	}

	result, err = c.encoder.Decode([]byte(sValue))
	if err != nil {
		return result, fmt.Errorf("%w: error decoding value `%s`, details = %w", cache.ErrDecoding, sValue, err)
	}

	return result, nil
}
