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
					v1, gErr := cacheOne.Get(ctx, key)
					if gErr != nil {
						return false
					}

					v2, gErr := cacheTwo.Get(ctx, key)
					if gErr != nil {
						return false
					}

					return v1 == value && v2 == value
				}, 5*time.Second, 100*time.Millisecond)
			})
		})

		t.Run("WHEN removing a key from one cache", func(t *testing.T) {
			key := gofakeit.UUID()
			value := gofakeit.UUID()

			require.NoError(t, cacheOne.Put(ctx, key, value))

			require.Eventually(t, func() bool {
				exists, errH := cacheOne.Has(ctx, key)
				if errH != nil {
					return false
				}

				return exists
			}, 5*time.Second, 100*time.Millisecond)

			deleted, err := cacheOne.Remove(ctx, key)
			require.NoError(t, err)
			require.True(t, deleted)

			t.Run("THEN it is deleted at the other caches", func(t *testing.T) {
				require.Eventually(t, func() bool {
					exists1, gErr := cacheOne.Has(ctx, key)
					if gErr != nil {
						return false
					}

					exists2, gErr := cacheTwo.Has(ctx, key)
					if gErr != nil {
						return false
					}

					return !exists1 && !exists2
				}, 5*time.Second, 100*time.Millisecond)
			})
		})

		t.Run("WHEN flushing one cache", func(t *testing.T) {
			key := gofakeit.UUID()
			value := gofakeit.UUID()

			require.NoError(t, cacheOne.Put(ctx, key, value))

			require.Eventually(t, func() bool {
				exists, errH := cacheOne.Has(ctx, key)
				if errH != nil {
					return false
				}

				return exists
			}, 5*time.Second, 100*time.Millisecond)

			require.NoError(t, cacheOne.Flush(ctx))

			t.Run("THEN then all keys are deleted from all caches", func(t *testing.T) {
				require.Eventually(t, func() bool {
					exists1, gErr := cacheOne.Has(ctx, key)
					if gErr != nil {
						return false
					}

					exists2, gErr := cacheTwo.Has(ctx, key)
					if gErr != nil {
						return false
					}

					return !exists1 && !exists2
				}, 5*time.Second, 100*time.Millisecond)
			})
		})
	})
}

// TestLRU_Parallelism this test simulates a real world scenario where multiple
// goroutines are writing the same key with different values through different instances
// of an application. Expected result is that the last written value is propagated to all
// instances, and that the propagation is eventually consistent.
func TestLRU_Parallelism(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	t.Run("GIVEN 10 rsmcache instances", func(t *testing.T) {
		mini := miniredis.RunT(t)
		opts, err := redis.ParseURL(fmt.Sprintf("redis://%s", mini.Addr()))
		require.NoError(t, err)

		encoder := cache.GobCodec[string]{}
		cacheSize := 100
		channelName := time.Now().String()
		caches := make([]cache.SimpleCache[string], 10)

		for i := 0; i < 10; i++ {
			c, err := rsmcache.NewLRU[string](ctx, encoder, cacheSize, redis.NewClient(opts), channelName)
			require.NoError(t, err)

			caches[i] = c
		}

		t.Run("WHEN multiple goroutines writes the same key with different values through different caches", func(t *testing.T) {
			key := gofakeit.UUID()
			n := 100
			wg := sync.WaitGroup{}

			values := make([]string, 0)
			mu := sync.Mutex{}

			for ci := range caches {
				for i := 0; i < n; i++ {
					wg.Add(1)

					go func(lru cache.SimpleCache[string]) {
						defer wg.Done()

						mu.Lock()
						defer mu.Unlock()

						v := gofakeit.UUID()
						values = append(values, v)

						if pErr := lru.Put(ctx, key, v); pErr != nil {
							t.Errorf("error putting key `%s` with value `%s`", key, v)

							return
						}
					}(caches[ci])
				}
			}

			wg.Wait()

			t.Run("THEN the last written value is propagated to all caches", func(t *testing.T) {
				// give time redis to propagate the last value
				time.Sleep(2 * time.Second)

				lastWrittenValue := values[len(values)-1]

				for _, lru := range caches {
					v, gErr := lru.Get(ctx, key)

					require.NoError(t, gErr)
					require.EqualValues(t, lastWrittenValue, v)
				}
			})
		})
	})
}
