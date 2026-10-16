// Package config loads x360-xexds configuration from the environment.
package config

import (
	"fmt"
	"strings"
)

// MinKeyLen is the shortest accepted X360_XEXDS_KEY, in bytes.
const MinKeyLen = 16

type Config struct {
	Addr  string // X360_XEXDS_ADDR, default :3074 (UDP; the dashboard dials 3074)
	Key   string // X360_XEXDS_KEY, optional: signs the NAT-probe cookie; random per process when empty
	Debug bool   // X360_XEXDS_DEBUG: log every datagram
}

// Load reads the environment through getenv (os.Getenv in production).
func Load(getenv func(string) string) (*Config, error) {
	c := &Config{
		Addr:  getenv("X360_XEXDS_ADDR"),
		Key:   getenv("X360_XEXDS_KEY"),
		Debug: isTrue(getenv("X360_XEXDS_DEBUG")),
	}
	if c.Addr == "" {
		c.Addr = ":3074"
	}
	if c.Key != "" && len(c.Key) < MinKeyLen {
		return nil, fmt.Errorf("X360_XEXDS_KEY must be at least %d bytes", MinKeyLen)
	}
	return c, nil
}

func isTrue(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
