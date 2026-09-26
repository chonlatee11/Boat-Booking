// Package httpx holds shared HTTP helpers: env config, error mapping, and the
// trust-boundary claim middleware (D-16, D-30, D-44).
package httpx

import (
	"fmt"
	"os"
)

// MustEnv reads an environment variable or panics naming the missing key.
func MustEnv(key string) string {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		panic(fmt.Sprintf("httpx: missing required env var %q", key))
	}
	return v
}

// EnvOr reads an environment variable or returns def if unset/empty.
func EnvOr(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}
