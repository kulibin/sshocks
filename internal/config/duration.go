package config

import (
	"time"

	"gopkg.in/yaml.v3"
)

// Duration is a time.Duration that unmarshals from a YAML string like "30s".
// A zero value means "unset" and is replaced by the default in Load.
type Duration struct {
	time.Duration
}

// NewDuration wraps d.
func NewDuration(d time.Duration) Duration {
	return Duration{Duration: d}
}

// UnmarshalYAML parses the scalar value using time.ParseDuration.
func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = parsed
	return nil
}
