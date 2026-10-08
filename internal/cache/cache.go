package cache

import (
	"context"
	"time"
)

// Loader is a function that fetches data when it is not found in the cache.
type Loader[T any] func(ctx context.Context) (T, error)

// Cache is the main interface for multi-layer caching (L0 singleflight → L1 in-process → L2 Redis → L3 DB).
type Cache interface {
	// GetOrLoad checks L1 → L2 → L3 and loads via loader using singleflight deduplication.
	GetOrLoad(ctx context.Context, key string, ttl time.Duration, load Loader[[]byte]) ([]byte, error)

	// Invalidate removes all keys matching the given prefix.
	Invalidate(ctx context.Context, keyPrefix string) error
}
