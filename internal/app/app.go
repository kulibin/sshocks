// Package app wires the config, logger, SSH tunnel, SOCKS5 server and the
// optional HTTP CONNECT proxy together and orchestrates their lifecycle.
package app

import (
	"context"
	"fmt"

	"sshocks/internal/config"
	"sshocks/internal/httpproxy"
	"sshocks/internal/log"
	"sshocks/internal/socks"
	"sshocks/internal/tunnel"
)

// Version is the build version reported at startup.
const Version = "v1.0.0"

// Application is the top-level wiring and runtime.
type Application struct {
	cfg     config.Config
	log     log.Logger
	conn    *tunnel.Connector
	server  *socks.Server
	httpSrv *httpproxy.Server
}

// New builds an Application from a validated config and logger. It wires the
// components but starts nothing; call Run, then Shutdown on exit.
func New(cfg config.Config, logger log.Logger) *Application {
	tc := tunnel.NewConfig(
		cfg.SSH.Host,
		cfg.SSH.Port,
		cfg.SSH.User,
		cfg.SSH.Password,
		cfg.SSH.Key,
		cfg.SSH.KeyPassphrase,
		cfg.SSH.Timeout,
	)

	conn := tunnel.NewConnector(tc, logger)
	srv := socks.NewServer(cfg.Socks.Listen, conn, logger)

	// Optional local DNS resolver. When servers are configured, target hostnames
	// are resolved locally against those servers (bypassing a broken local
	// resolver) and the resulting IP is forwarded through the tunnel. Empty means
	// hostnames are resolved by the remote side, as before.
	dnsResolver, err := tunnel.NewDNSResolver(cfg.DNS.Servers, 0)
	if err != nil {
		logger.Errorf("dns: resolver init: %v", err)
	} else if dnsResolver != nil {
		conn.SetResolver(dnsResolver)
		logger.Infof("dns: local resolution via servers %v", cfg.DNS.Servers)
	}

	a := &Application{cfg: cfg, log: logger, conn: conn, server: srv}

	// The HTTP CONNECT proxy is optional; only created when listen is set.
	if cfg.HTTP.Listen != "" {
		a.httpSrv = httpproxy.NewServer(cfg.HTTP.Listen, conn, logger)
	}

	return a
}

// Run starts the SSH connector and the SOCKS5 listener, then blocks. It returns
// when ctx is cancelled (listener is closed so Accept unblocks) or on a fatal
// server/transport error. Active sessions are handled by Shutdown.
func (a *Application) Run(ctx context.Context) error {
	a.log.Infof("sshocks %s starting", Version)
	a.log.Infof("config: ssh=%s/%d user=%s auth=%s",
		a.cfg.SSH.Host, a.cfg.SSH.Port, a.cfg.SSH.User, a.authSummary())

	// SSH connector (autodial + reconnect) in the background.
	go a.conn.Run(ctx)

	if a.httpSrv != nil {
		// Bind happened at construction; surface a bind failure as fatal.
		if err := a.httpSrv.ListenErr(); err != nil {
			return fmt.Errorf("httpproxy: listen %s: %w", a.cfg.HTTP.Listen, err)
		}
		a.log.Infof("httpproxy: listening on %s", a.cfg.HTTP.Listen)
		// Run in the background until ctx cancels; Shutdown drains its sessions.
		// A listener Accept error is logged but not fatal (the SOCKS server keeps
		// serving), so we ignore the return value.
		go func() {
			if err := a.httpSrv.Run(ctx); err != nil {
				a.log.Warnf("httpproxy: server stopped: %v", err)
			}
		}()
	}

	// When ctx is cancelled, close the listeners so their Accept() unblocks and
	// the servers can exit cleanly.
	go func() {
		<-ctx.Done()
		_ = a.server.Shutdown()
		if a.httpSrv != nil {
			_ = a.httpSrv.Shutdown()
		}
	}()

	return a.server.Run(ctx)
}

// Shutdown performs a graceful drain: stop accepting, drain active sessions up to
// the configured drain timeout, then close the tunnel and the logger.
func (a *Application) Shutdown() {
	a.log.Infof("shutdown: draining active socks sessions...")
	a.server.Drain(a.cfg.DrainTimeout)

	if a.httpSrv != nil {
		a.log.Infof("shutdown: draining active http proxy sessions...")
		a.httpSrv.Drain(a.cfg.DrainTimeout)
	}

	if err := a.conn.Close(); err != nil {
		a.log.Warnf("tunnel: close error: %v", err)
	}
	a.log.Flush()
	if err := a.log.Close(); err != nil {
		a.log.Errorf("logger close error: %v", err)
	}
}

// authSummary reports which auth method takes priority for startup logging.
func (a *Application) authSummary() string {
	if a.cfg.SSH.Password != "" {
		return "password"
	}
	if a.cfg.SSH.Key != "" {
		return "key"
	}
	return "none"
}
