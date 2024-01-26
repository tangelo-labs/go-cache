package mcache_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tangelo-labs/go-cache/provider/mcache"
)

func TestLRU(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cache, err := mcache.NewLRU[int](256)
	require.NoError(t, err)

	require.NoError(t, cache.Put(ctx, "key", 1))

	has, err := cache.Has(ctx, "key")
	require.NoError(t, err)
	require.True(t, has)

	found, err := cache.Get(ctx, "key")
	require.NoError(t, err)
	require.Equal(t, 1, found)

	require.NoError(t, cache.Flush(ctx))
}
