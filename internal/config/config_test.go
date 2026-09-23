package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaults(t *testing.T) {
	d := Defaults()
	if d.SSH.Port != 22 {
		t.Errorf("default SSH.Port = %d, want 22", d.SSH.Port)
	}
	if d.SSH.Timeout != 30*time.Second {
		t.Errorf("default SSH.Timeout = %v, want 30s", d.SSH.Timeout)
	}
	if d.Socks.Listen != "127.0.0.1:1080" {
		t.Errorf("default socks.Listen = %q, want 127.0.0.1:1080", d.Socks.Listen)
	}
	if d.Log.File != "./sshocks.log" {
		t.Errorf("default log.file = %q", d.Log.File)
	}
	if d.Log.Level != "info" {
		t.Errorf("default log.level = %q", d.Log.Level)
	}
	if d.DrainTimeout != 5*time.Second {
		t.Errorf("default drain_timeout = %v, want 5s", d.DrainTimeout)
	}
}

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	return path
}

func TestLoadAppliesDefaults(t *testing.T) {
	path := writeTemp(t, `
ssh:
  host: example.com
  user: alice
  password: hunter2
`)
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.SSH.Host != "example.com" || c.SSH.User != "alice" || c.SSH.Password != "hunter2" {
		t.Errorf("loaded ssh = %+v", c.SSH)
	}
	if c.SSH.Port != 22 || c.SSH.Timeout != 30*time.Second {
		t.Errorf("defaults not applied: port=%d timeout=%v", c.SSH.Port, c.SSH.Timeout)
	}
	if c.Socks.Listen != "127.0.0.1:1080" {
		t.Errorf("default socks.Listen not applied: %q", c.Socks.Listen)
	}
	if err := Validate(c); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestLoadOverrides(t *testing.T) {
	path := writeTemp(t, `
ssh:
  host: 1.2.3.4
  port: 2222
  user: bob
  key: ~/.ssh/id_ed25519
  key_passphrase: pass
  timeout: 10s
socks:
  listen: 127.0.0.1:1081
log:
  file: /var/log/sshocks.log
  level: debug
drain_timeout: 2s
`)
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.SSH.Port != 2222 {
		t.Errorf("Port = %d, want 2222", c.SSH.Port)
	}
	if c.SSH.Timeout != 10*time.Second {
		t.Errorf("Timeout = %v, want 10s", c.SSH.Timeout)
	}
	if c.Socks.Listen != "127.0.0.1:1081" {
		t.Errorf("Listen = %q", c.Socks.Listen)
	}
	if c.Log.Level != "debug" || c.Log.File != "/var/log/sshocks.log" {
		t.Errorf("Log = %+v", c.Log)
	}
	if c.DrainTimeout != 2*time.Second {
		t.Errorf("DrainTimeout = %v, want 2s", c.DrainTimeout)
	}
	if err := Validate(c); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	path := writeTemp(t, `ssh: [ this is not valid`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadInvalidDuration(t *testing.T) {
	path := writeTemp(t, `
ssh:
  host: h
  user: u
  password: p
  timeout: notaduration
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid duration")
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name string
		c    Config
		want error
	}{
		{"empty host", emptyHost(), ErrEmptyHost},
		{"empty user", emptyUser(), ErrEmptyUser},
		{"no credentials", noCreds(), ErrNoCredentials},
		{"bad listen", badListen(), ErrInvalidListen},
		{"bad http listen", badHTTPListen(), ErrInvalidHTTP},
		{"bad level", badLevel(), ErrInvalidLevel},
		{"bad dns server", badDNSServer(), ErrInvalidDNSServer},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.c)
			if !errors.Is(err, tc.want) {
				t.Errorf("Validate = %v, want errors.Is %v", err, tc.want)
			}
		})
	}
}

func TestValidateOK(t *testing.T) {
	c := Defaults()
	c.SSH.Host = "1.2.3.4"
	c.SSH.User = "alice"
	c.SSH.Password = "secret"
	if err := Validate(c); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

// helpers build specific Configs for the table above.
func emptyHost() Config {
	c := Defaults()
	c.SSH.Host = ""
	c.SSH.User = "u"
	c.SSH.Password = "p"
	return c
}

func emptyUser() Config {
	c := Defaults()
	c.SSH.Host = "h"
	c.SSH.User = ""
	c.SSH.Password = "p"
	return c
}

func noCreds() Config {
	c := Defaults()
	c.SSH.Host = "h"
	c.SSH.User = "u"
	c.SSH.Password = ""
	c.SSH.Key = ""
	return c
}

func badListen() Config {
	c := Defaults()
	c.SSH.Host = "h"
	c.SSH.User = "u"
	c.SSH.Password = "p"
	c.Socks.Listen = "no-colon"
	return c
}

func badHTTPListen() Config {
	c := Defaults()
	c.SSH.Host = "h"
	c.SSH.User = "u"
	c.SSH.Password = "p"
	c.HTTP.Listen = "no-colon"
	return c
}

func badLevel() Config {
	c := Defaults()
	c.SSH.Host = "h"
	c.SSH.User = "u"
	c.SSH.Password = "p"
	c.Log.Level = "verbose"
	return c
}

func badDNSServer() Config {
	c := Defaults()
	c.SSH.Host = "h"
	c.SSH.User = "u"
	c.SSH.Password = "p"
	c.DNS.Servers = []string{"8.8.8.8", "bogus"}
	return c
}

func TestLoadDNSServers(t *testing.T) {
	path := writeTemp(t, `
ssh:
  host: 1.2.3.4
  user: u
  password: p
dns:
  servers:
    - 8.8.8.8
    - 8.8.4.4
    - 1.1.1.1
`)
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(c.DNS.Servers) != 3 {
		t.Fatalf("DNS.Servers = %v, want 3 entries", c.DNS.Servers)
	}
	if err := Validate(c); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestValidateInvalidDNSServer(t *testing.T) {
	c := Defaults()
	c.SSH.Host = "h"
	c.SSH.User = "u"
	c.SSH.Password = "p"
	c.DNS.Servers = []string{"not-an-ip"}
	if err := Validate(c); !errors.Is(err, ErrInvalidDNSServer) {
		t.Fatalf("Validate = %v, want ErrInvalidDNSServer", err)
	}
}
