package rscache_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/tangelo-labs/go-cache"
	"github.com/tangelo-labs/go-cache/provider/rscache"
)

func TestRedisCache(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	t.Run("GIVEN a rscache instance", func(t *testing.T) {
		mini := miniredis.RunT(t)
		opts, err := redis.ParseURL(fmt.Sprintf("redis://%s", mini.Addr()))
		require.NoError(t, err)

		encoder := cache.GobCodec[string]{}
		bCache := rscache.NewRedisCache[string](redis.NewClient(opts), encoder, 5*time.Second, "profiletest")

		t.Run("WHEN a new key is set in cache", func(t *testing.T) {
			err = bCache.Put(ctx, "key", "value")

			t.Run("THEN no error is raised AND the key is present in the cache", func(t *testing.T) {
				require.NoError(t, err)

				got, gErr := bCache.Get(ctx, "key")
				require.NoError(t, gErr)
				require.Equal(t, "value", got)
			})
		})

		t.Run("WHEN a existing key is set in the cache", func(t *testing.T) {
			err = bCache.Put(ctx, "key", "valueTwo")

			t.Run("THEN no error is raised AND the value is updated", func(t *testing.T) {
				require.NoError(t, err)

				got, gErr := bCache.Get(ctx, "key")
				require.NoError(t, gErr)
				require.Equal(t, "valueTwo", got)
			})
		})

		t.Run("WHEN a existing key is deleted THEN no error is raised and true is returned AND the key is not present in the cache", func(t *testing.T) {
			b, eErr := bCache.Remove(ctx, "key")
			require.NoError(t, eErr)
			require.True(t, b)

			_, gErr := bCache.Get(ctx, "key")
			require.ErrorIs(t, gErr, cache.ErrItemNotFound)
		})

		t.Run("WHEN asking if cache has a non existing key THEN it no error AND false", func(t *testing.T) {
			has, hErr := bCache.Has(ctx, "non-existing-key")
			require.NoError(t, hErr)
			require.False(t, has)
		})

		t.Run("WHEN a non existing key is deleted THEN false is returned", func(t *testing.T) {
			b, rErr := bCache.Remove(ctx, "key")

			require.NoError(t, rErr)
			require.False(t, b)
		})

		t.Run("WHEN flushing a cache with multiple keys saved ", func(t *testing.T) {
			for i := 0; i < 10; i++ {
				err = bCache.Put(ctx, fmt.Sprintf("key%d", i), fmt.Sprintf("valueTwo%d", i))
				require.NoError(t, err)
			}

			err = bCache.Flush(ctx)

			t.Run("THEN no error is raised AND the cacher is empty", func(t *testing.T) {
				require.NoError(t, err)

				for i := 0; i < 10; i++ {
					_, gErr := bCache.Get(ctx, "key"+strconv.Itoa(i))
					require.ErrorIs(t, gErr, cache.ErrItemNotFound)
				}
			})
		})
	})
}
