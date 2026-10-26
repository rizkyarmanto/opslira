package probes

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisCache struct{ rdb redis.Cmdable }

// NewRedisCache adapts a go-redis client (use the DB1 cache client, the Redis
// equivalent of Laravel's Cache facade) to the Cache interface.
func NewRedisCache(rdb redis.Cmdable) Cache { return redisCache{rdb: rdb} }

func (c redisCache) Get(ctx context.Context, key string) (string, bool) {
	value, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		return "", false
	}
	return value, true
}

func (c redisCache) Set(ctx context.Context, key, value string, ttl time.Duration) {
	// Best-effort like the dashboard caches: a cache write failure must not
	// fail the probe.
	_ = c.rdb.Set(ctx, key, value, ttl).Err()
}

func (c redisCache) Del(ctx context.Context, keys ...string) {
	_ = c.rdb.Del(ctx, keys...).Err()
}

// CacheGetJSON reads and decodes a cached JSON object; misses and decode
// failures both report a miss, so callers recompute. A nil cache always
// misses.
func CacheGetJSON(ctx context.Context, cache Cache, key string, out any) bool {
	if cache == nil {
		return false
	}
	raw, ok := cache.Get(ctx, key)
	if !ok {
		return false
	}
	return json.Unmarshal([]byte(raw), out) == nil
}

// CacheSetJSON stores a JSON value with a TTL; nil cache is a no-op.
func CacheSetJSON(ctx context.Context, cache Cache, key string, value any, ttl time.Duration) {
	if cache == nil {
		return
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return
	}
	cache.Set(ctx, key, string(encoded), ttl)
}
