package config

import (
	"errors"
	"fmt"
	"net"
)

// Sentinel errors returned by Validate, classifiable via errors.Is.
var (
	ErrEmptyHost     = errors.New("config: ssh.host is required")
	ErrEmptyUser     = errors.New("config: ssh.user is required")
	ErrNoCredentials = errors.New("config: ssh.password and ssh.key must not both be empty")
	ErrInvalidListen = errors.New("config: socks.listen must be host:port")
	ErrInvalidHTTP   = errors.New("config: http.listen must be host:port")
	ErrInvalidLevel  = errors.New("config: invalid log.level")
)

// Validate checks the config for correctness and returns the first error found.
// Errors are wrapped around the sentinels above so callers can use errors.Is.
func Validate(c Config) error {
	if c.SSH.Host == "" {
		return ErrEmptyHost
	}
	if c.SSH.User == "" {
		return ErrEmptyUser
	}
	if c.SSH.Password == "" && c.SSH.Key == "" {
		return ErrNoCredentials
	}

	if err := validateListen(c.Socks.Listen); err != nil {
		return err
	}
	if c.HTTP.Listen != "" {
		if err := validateHTTPListen(c.HTTP.Listen); err != nil {
			return err
		}
	}
	if err := validateLevel(c.Log.Level); err != nil {
		return err
	}
	return nil
}

func validateListen(listen string) error {
	if listen == "" {
		return fmt.Errorf("%w: empty value", ErrInvalidListen)
	}
	_, _, err := net.SplitHostPort(listen)
	if err != nil {
		return fmt.Errorf("%w: %q: %v", ErrInvalidListen, listen, err)
	}
	return nil
}

// validateHTTPListen is like validateListen but treats an empty value as
// "disabled", so the optional HTTP proxy listener is skipped when unset.
func validateHTTPListen(listen string) error {
	if listen == "" {
		return nil
	}
	_, _, err := net.SplitHostPort(listen)
	if err != nil {
		return fmt.Errorf("%w: %q: %v", ErrInvalidHTTP, listen, err)
	}
	return nil
}

func validateLevel(level string) error {
	switch level {
	case "", "debug", "info", "warn", "error":
		return nil
	default:
		return fmt.Errorf("%w: %q", ErrInvalidLevel, level)
	}
}
