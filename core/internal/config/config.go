// Package config holds rewynd's zero-config defaults and resolved paths.
package config

import (
	"os"
	"path/filepath"
	"strconv"
)

const (
	DefaultOTLPAddr     = "127.0.0.1:4318" // OTLP/HTTP
	DefaultOTLPGRPCAddr = "127.0.0.1:4317" // OTLP/gRPC (the SDK default)
	DefaultMaxRequests  = 1000
)

// MaxRequests is the ring-buffer ceiling: how many recent requests to retain before pruning.
// Defaults to 1000 (plenty for local dev); raise it via REWYND_MAX_REQUESTS for large projects
// or long sessions that want to leave nothing behind.
func MaxRequests() int {
	if v := os.Getenv("REWYND_MAX_REQUESTS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return DefaultMaxRequests
}

func DataDir() string {
	if d := os.Getenv("REWYND_HOME"); d != "" {
		return d
	}
	if d, err := os.UserConfigDir(); err == nil {
		return filepath.Join(d, "rewynd")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".rewynd")
}

func DBPath() string {
	if p := os.Getenv("REWYND_DB"); p != "" {
		return p
	}
	return filepath.Join(DataDir(), "rewynd.sqlite")
}
