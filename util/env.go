package util

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// EnvOr returns the env var's value, or def if it's unset/empty. Shared by
// cmd/server and cmd/worker so both binaries parse the same env vars the
// same way.
func EnvOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func EnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func EnvDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

// HostnamePID builds a reasonably-unique identity for this process (used
// as a Redis Streams consumer name), so multiple replicas sharing one
// consumer group don't collide.
func HostnamePID(fallbackName string) string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = fallbackName
	}
	return fmt.Sprintf("%s-%d", host, os.Getpid())
}
