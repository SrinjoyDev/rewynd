// Package config holds rewynd's zero-config defaults and resolved paths.
package config

import (
	"os"
	"path/filepath"
)

const (
	DefaultOTLPAddr    = "127.0.0.1:4318"
	DefaultMaxRequests = 1000
)

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
