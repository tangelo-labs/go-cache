package bcache_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/allegro/bigcache"
	"github.com/stretchr/testify/require"
	"github.com/tangelo-labs/go-cache"
	"github.com/tangelo-labs/go-cache/provider/bcache"
)

func TestBigCacheCacher_Put(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	t.Run("GIVEN a bcache instance", func(t *testing.T) {
		bigcache.DefaultConfig(6 * time.Second)

		bigCache, err := bigcache.NewBigCache(bigcache.DefaultConfig(5 * time.Second))
		require.NoError(t, err)

		bigEncoder := cache.GobCodec[string]{}
		bCache := bcache.NewBigCache[string](bigCache, bigEncoder)

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
