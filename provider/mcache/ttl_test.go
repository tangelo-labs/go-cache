package mcache_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tangelo-labs/go-cache"
	"github.com/tangelo-labs/go-cache/provider/mcache"
)

func TestTTL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cache := mcache.NewTTL[int](time.Hour, mcache.UnlimitedItems, false)

	require.NoError(t, cache.Put(ctx, "key", 1))

	has, err := cache.Has(ctx, "key")
	require.NoError(t, err)
	require.True(t, has)

	found, err := cache.Get(ctx, "key")
	require.NoError(t, err)
	require.Equal(t, 1, found)

	removed, err := cache.Remove(ctx, "key")
	require.NoError(t, err)
	require.True(t, removed)

	has, err = cache.Has(ctx, "key")
	require.NoError(t, err)
	require.False(t, has)

	require.NoError(t, cache.Flush(ctx))
}

func TestTTLItemLimit(t *testing.T) {
	t.Run("GIVEN a ttl cache with a limit of 1 item", func(t *testing.T) {
		ctx := context.Background()
		c := mcache.NewTTL[int](time.Hour, 1, false)

		t.Run("WHEN adding 2 items", func(t *testing.T) {
			require.NoError(t, c.Put(ctx, "key1", 1))
			require.NoError(t, c.Put(ctx, "key2", 1))

			t.Run("THEN the first item is removed", func(t *testing.T) {
				_, err := c.Get(ctx, "key1")
				require.ErrorIs(t, err, cache.ErrItemNotFound)
			})
		})
	})
}
