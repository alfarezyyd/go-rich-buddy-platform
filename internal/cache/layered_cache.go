package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

const (
	// l1MaxEntries is the maximum number of items held in the L1 in-process LRU cache.
	l1MaxEntries = 512

	// jitterPercent adds ±jitterPercent/2 randomness to TTLs to prevent cache stampedes.
	jitterPercent = 10
)

// l1Entry holds a cached value with its expiry deadline.
type l1Entry struct {
	value     []byte
	expiresAt time.Time
}

// LayeredCache is a multi-layer cache: L0 singleflight → L1 in-process → L2 Redis.
// L3 (DB snapshot) is handled by the caller-supplied Loader function.
type LayeredCache struct {
	redisClient *redis.Client
	l1          map[string]*l1Entry
	flightGroup singleflight.Group
}

// NewLayeredCache creates a new LayeredCache backed by Redis for L2.
func NewLayeredCache(redisClient *redis.Client) Cache {
	return &LayeredCache{
		redisClient: redisClient,
		l1:          make(map[string]*l1Entry, l1MaxEntries),
	}
}

// GetOrLoad retrieves a value from the cache hierarchy, falling back to the Loader.
// Concurrent requests for the same key are deduplicated via singleflight (L0).
func (layeredCache *LayeredCache) GetOrLoad(ctx context.Context, key string, ttl time.Duration, load Loader[[]byte]) ([]byte, error) {
	// L1: check in-process cache first (fast path, no network).
	if entry, found := layeredCache.getL1(key); found {
		return entry, nil
	}

	// L0 + L2 + L3: deduplicate concurrent requests for the same key.
	result, err, _ := layeredCache.flightGroup.Do(key, func() (any, error) {
		// L2: try Redis.
		if layeredCache.redisClient != nil {
			redisVal, redisErr := layeredCache.redisClient.Get(ctx, key).Bytes()
			if redisErr == nil {
				layeredCache.setL1(key, redisVal, ttl)
				return redisVal, nil
			}
		}

		// L3 / DB: call the provided loader function.
		loaded, loadErr := load(ctx)
		if loadErr != nil {
			return nil, fmt.Errorf("cache loader error for key %q: %w", key, loadErr)
		}

		jitteredTTL := applyJitter(ttl)

		// Populate L2.
		if layeredCache.redisClient != nil {
			_ = layeredCache.redisClient.Set(ctx, key, loaded, jitteredTTL).Err()
		}

		// Populate L1.
		layeredCache.setL1(key, loaded, jitteredTTL)

		return loaded, nil
	})

	if err != nil {
		return nil, err
	}

	return result.([]byte), nil
}

// Invalidate removes all keys matching the given prefix from L1 and L2.
func (layeredCache *LayeredCache) Invalidate(ctx context.Context, keyPrefix string) error {
	// Evict matching L1 entries.
	for k := range layeredCache.l1 {
		if len(k) >= len(keyPrefix) && k[:len(keyPrefix)] == keyPrefix {
			delete(layeredCache.l1, k)
		}
	}

	// Evict from Redis using SCAN to handle large key spaces without blocking.
	if layeredCache.redisClient == nil {
		return nil
	}

	pattern := keyPrefix + "*"
	var cursor uint64
	for {
		var keys []string
		var scanErr error
		keys, cursor, scanErr = layeredCache.redisClient.Scan(ctx, cursor, pattern, 100).Result()
		if scanErr != nil {
			return fmt.Errorf("cache invalidate scan error: %w", scanErr)
		}

		if len(keys) > 0 {
			if delErr := layeredCache.redisClient.Del(ctx, keys...).Err(); delErr != nil {
				return fmt.Errorf("cache invalidate delete error: %w", delErr)
			}
		}

		if cursor == 0 {
			break
		}
	}

	return nil
}

// getL1 retrieves a value from the in-process LRU cache, respecting TTL.
func (layeredCache *LayeredCache) getL1(key string) ([]byte, bool) {
	entry, ok := layeredCache.l1[key]
	if !ok || time.Now().After(entry.expiresAt) {
		if ok {
			delete(layeredCache.l1, key)
		}
		return nil, false
	}
	return entry.value, true
}

// setL1 stores a value in the in-process LRU cache with the given TTL.
// When at capacity, a random entry is evicted to bound memory usage.
func (layeredCache *LayeredCache) setL1(key string, value []byte, ttl time.Duration) {
	if len(layeredCache.l1) >= l1MaxEntries {
		// Simple random eviction to keep the map bounded without a full LRU structure.
		for k := range layeredCache.l1 {
			delete(layeredCache.l1, k)
			break
		}
	}
	layeredCache.l1[key] = &l1Entry{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
}

// applyJitter adds random ±jitterPercent/2 jitter to a TTL to prevent stampedes.
func applyJitter(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return ttl
	}
	delta := int64(ttl) * int64(jitterPercent) / 100
	jitter := rand.Int63n(delta*2) - delta //nolint:gosec // non-cryptographic use
	return ttl + time.Duration(jitter)
}

// GetOrLoadJSON is a convenience wrapper that marshals/unmarshals a typed value via the cache.
func GetOrLoadJSON[T any](ctx context.Context, c Cache, key string, ttl time.Duration, load func(ctx context.Context) (T, error)) (T, error) {
	var zero T
	raw, err := c.GetOrLoad(ctx, key, ttl, func(ctx context.Context) ([]byte, error) {
		val, loadErr := load(ctx)
		if loadErr != nil {
			return nil, loadErr
		}
		return json.Marshal(val)
	})
	if err != nil {
		return zero, err
	}

	var result T
	if unmarshalErr := json.Unmarshal(raw, &result); unmarshalErr != nil {
		return zero, fmt.Errorf("cache unmarshal error: %w", unmarshalErr)
	}
	return result, nil
}
