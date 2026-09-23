package config

import "time"

// raw is the YAML-decoded view using pointers so that absent fields can be
// distinguished from explicit zero values when applying defaults.
type raw struct {
	SSH struct {
		Host          *string   `yaml:"host"`
		Port          *int      `yaml:"port"`
		User          *string   `yaml:"user"`
		Password      *string   `yaml:"password"`
		Key           *string   `yaml:"key"`
		KeyPassphrase *string   `yaml:"key_passphrase"`
		Timeout       *Duration `yaml:"timeout"`
	} `yaml:"ssh"`
	Socks struct {
		Listen *string `yaml:"listen"`
	} `yaml:"socks"`
	HTTP struct {
		Listen *string `yaml:"listen"`
	} `yaml:"http"`
	Log struct {
		File  *string `yaml:"file"`
		Level *string `yaml:"level"`
	} `yaml:"log"`
	DrainTimeout *Duration `yaml:"drain_timeout"`
}

// Config is the top-level runtime configuration, with defaults applied.
type Config struct {
	SSH          SSHConfig
	Socks        SocksConfig
	HTTP         HTTPConfig
	Log          LogConfig
	DrainTimeout time.Duration
}

// SSHConfig describes the SSH tunnel target and authentication.
// Password takes priority over Key for authentication (see README).
type SSHConfig struct {
	Host          string
	Port          int
	User          string
	Password      string
	Key           string
	KeyPassphrase string
	Timeout       time.Duration
}

// SocksConfig describes the local SOCKS5 listener.
type SocksConfig struct {
	Listen string
}

// HTTPConfig describes the local HTTP CONNECT proxy listener.
type HTTPConfig struct {
	Listen string
}

// LogConfig describes logging behaviour.
type LogConfig struct {
	File  string
	Level string
}

// Defaults returns the default configuration.
func Defaults() Config {
	return Config{
		SSH: SSHConfig{
			Port:    22,
			Timeout: 30 * time.Second,
		},
		Socks:        SocksConfig{Listen: "127.0.0.1:1080"},
		HTTP:         HTTPConfig{Listen: ""},
		Log:          LogConfig{File: "./sshocks.log", Level: "info"},
		DrainTimeout: 5 * time.Second,
	}
}
