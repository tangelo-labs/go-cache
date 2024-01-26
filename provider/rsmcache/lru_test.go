package rsmcache_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/brianvoe/gofakeit/v6"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/tangelo-labs/go-cache"
	"github.com/tangelo-labs/go-cache/provider/rsmcache"
)

func TestLRU_Simple(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	t.Run("GIVEN two rsmcache instances", func(t *testing.T) {
		mini := miniredis.RunT(t)
		opts, err := redis.ParseURL(fmt.Sprintf("redis://%s", mini.Addr()))
		require.NoError(t, err)

		encoder := cache.GobCodec[string]{}
		cacheSize := 100
		channelName := time.Now().String()

		cacheOne, err := rsmcache.NewLRU[string](ctx, encoder, cacheSize, redis.NewClient(opts), channelName)
		require.NoError(t, err)

		cacheTwo, err := rsmcache.NewLRU[string](ctx, encoder, cacheSize, redis.NewClient(opts), channelName)
		require.NoError(t, err)

		t.Run("WHEN writing a value into first cache", func(t *testing.T) {
			key := gofakeit.UUID()
			value := gofakeit.UUID()

			require.NoError(t, cacheOne.Put(ctx, key, value))

			t.Run("THEN the value is eventually propagated to the second cache", func(t *testing.T) {
				require.Eventually(t, func() bool {
					v, gErr := cacheTwo.Get(ctx, key)
					if gErr != nil {
						return false
					}

					return v == value
				}, 5*time.Second, 100*time.Millisecond)
			})
		})
	})
}

func TestLRU_Concurrency(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	t.Run("GIVEN two rsmcache instances", func(t *testing.T) {
		mini := miniredis.RunT(t)
		opts, err := redis.ParseURL(fmt.Sprintf("redis://%s", mini.Addr()))
		require.NoError(t, err)

		encoder := cache.GobCodec[string]{}
		cacheSize := 100
		channelName := time.Now().String()

		cacheOne, err := rsmcache.NewLRU[string](ctx, encoder, cacheSize, redis.NewClient(opts), channelName)
		require.NoError(t, err)

		cacheTwo, err := rsmcache.NewLRU[string](ctx, encoder, cacheSize, redis.NewClient(opts), channelName)
		require.NoError(t, err)

		t.Run("WHEN sequentially writing the same key with different values on the first cache", func(t *testing.T) {
			key := gofakeit.UUID()
			value := gofakeit.UUID()
			n := 100

			for i := 0; i < n; i++ {
				value = gofakeit.UUID()

				require.NoError(t, cacheOne.Put(ctx, key, value))
			}

			t.Run("THEN the last written value is eventually propagated to the second cache", func(t *testing.T) {
				require.Eventually(t, func() bool {
					v, gErr := cacheTwo.Get(ctx, key)
					if gErr != nil {
						return false
					}

					return v == value
				}, 5*time.Second, 100*time.Millisecond)
			})
		})
	})
}

func TestLRU_Parallelism(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	t.Run("GIVEN two rsmcache instances", func(t *testing.T) {
		mini := miniredis.RunT(t)
		opts, err := redis.ParseURL(fmt.Sprintf("redis://%s", mini.Addr()))
		require.NoError(t, err)

		encoder := cache.GobCodec[string]{}
		cacheSize := 100
		channelName := time.Now().String()

		cacheOne, err := rsmcache.NewLRU[string](ctx, encoder, cacheSize, redis.NewClient(opts), channelName)
		require.NoError(t, err)

		cacheTwo, err := rsmcache.NewLRU[string](ctx, encoder, cacheSize, redis.NewClient(opts), channelName)
		require.NoError(t, err)

		t.Run("WHEN multiple goroutines writes the same key with different values", func(t *testing.T) {
			key := gofakeit.UUID()
			n := 100
			wg := sync.WaitGroup{}

			values := make([]string, 0)
			mu := sync.Mutex{}

			for i := 0; i < n; i++ {
				wg.Add(1)

				go func() {
					defer wg.Done()

					mu.Lock()
					defer mu.Unlock()

					v := gofakeit.UUID()
					values = append(values, v)

					if pErr := cacheOne.Put(ctx, key, v); pErr != nil {
						t.Errorf("error putting key `%s` with value `%s`", key, v)

						return
					}
				}()
			}

			wg.Wait()

			t.Run("THEN value written by the last goroutine is propagated to the second cache", func(t *testing.T) {
				// give time redis to propagate the last value
				time.Sleep(time.Second)

				lastValue := values[len(values)-1]
				v, gErr := cacheTwo.Get(ctx, key)

				require.NoError(t, gErr)
				require.EqualValues(t, lastValue, v)
			})
		})
	})
}
