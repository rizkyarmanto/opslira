// Package redis builds the Redis clients shared by the process.
//
// Two logical databases are used: DB0 for data (sessions, probe state,
// beszel tokens) and DB1 for short-lived dashboard caches, matching the
// Redis 7 layout in docker-compose.yml.
package redis

import (
	goredis "github.com/redis/go-redis/v9"

	"opslira/backend/internal/config"
)

// NewData returns the DB0 client for sessions and probe state.
func NewData(cfg config.Redis) *goredis.Client {
	return goredis.NewClient(&goredis.Options{
		Addr:     addr(cfg),
		Password: cfg.Password,
		DB:       cfg.DB,
	})
}

// NewCache returns the DB1 client for short-lived dashboard caches.
func NewCache(cfg config.Redis) *goredis.Client {
	return goredis.NewClient(&goredis.Options{
		Addr:     addr(cfg),
		Password: cfg.Password,
		DB:       cfg.CacheDB,
	})
}

func addr(cfg config.Redis) string {
	return cfg.Host + ":" + cfg.Port
}
