package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Load reads and parses the YAML config file at path, applies defaults for any
// absent fields, and returns the resulting Config. Use Validate to check the
// result before use.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("config: read %q: %w", path, err)
	}

	var r raw
	if err := yaml.Unmarshal(data, &r); err != nil {
		return Config{}, fmt.Errorf("config: parse %q: %w", path, err)
	}

	return apply(r), nil
}

// apply merges a decoded raw config over the defaults, filling only the fields
// that were present in the file.
func apply(r raw) Config {
	c := Defaults()

	if r.SSH.Host != nil {
		c.SSH.Host = *r.SSH.Host
	}
	if r.SSH.Port != nil {
		c.SSH.Port = *r.SSH.Port
	}
	if r.SSH.User != nil {
		c.SSH.User = *r.SSH.User
	}
	if r.SSH.Password != nil {
		c.SSH.Password = *r.SSH.Password
	}
	if r.SSH.Key != nil {
		c.SSH.Key = *r.SSH.Key
	}
	if r.SSH.KeyPassphrase != nil {
		c.SSH.KeyPassphrase = *r.SSH.KeyPassphrase
	}
	if r.SSH.Timeout != nil {
		c.SSH.Timeout = r.SSH.Timeout.Duration
	}

	if r.Socks.Listen != nil {
		c.Socks.Listen = *r.Socks.Listen
	}

	if r.HTTP.Listen != nil {
		c.HTTP.Listen = *r.HTTP.Listen
	}

	if r.DNS.Servers != nil {
		c.DNS.Servers = r.DNS.Servers
	}

	if r.Log.File != nil {
		c.Log.File = *r.Log.File
	}
	if r.Log.Level != nil {
		c.Log.Level = *r.Log.Level
	}

	if r.DrainTimeout != nil {
		c.DrainTimeout = r.DrainTimeout.Duration
	}

	return c
}
