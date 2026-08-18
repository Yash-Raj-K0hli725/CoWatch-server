// Package rdb centralizes Redis client construction so the API server and
// the worker binary configure it identically from the same env vars.
package rdb

import (
	"os"
	"strconv"

	"github.com/redis/go-redis/v9"
)

// NewClient builds a Redis client from REDIS_ADDR / REDIS_PASSWORD /
// REDIS_DB, defaulting to a local single-node instance for dev.
func NewClient() *redis.Client {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	db := 0
	if v := os.Getenv("REDIS_DB"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			db = parsed
		}
	}
	return redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       db,
	})
}
