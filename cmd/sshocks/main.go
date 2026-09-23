package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"sshocks/internal/app"
	"sshocks/internal/config"
	"sshocks/internal/log"
)

func main() {
	cfgPath := flag.String("config", "config.yaml", "path to YAML config file")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		// Unreadable/invalid config is fatal; exit non-zero.
		os.Stderr.WriteString("sshocks: config error: " + err.Error() + "\n")
		os.Exit(1)
	}

	if err := config.Validate(cfg); err != nil {
		// Validation failure is fatal; exit non-zero per requirements.
		os.Stderr.WriteString("sshocks: config validation: " + err.Error() + "\n")
		os.Exit(1)
	}

	logger, err := log.NewFile(cfg.Log.File, cfg.Log.Level)
	if err != nil {
		os.Stderr.WriteString("sshocks: log init error: " + err.Error() + "\n")
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())

	a := app.New(cfg, logger)

	// On SIGINT/SIGTERM cancel the context. The connector observes the
	// cancellation; the listener is closed inside app.Run so server.Run returns.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		logger.Infof("signal received, shutting down...")
		cancel()
	}()

	// Block until the signal cancels ctx (clean shutdown) or a fatal server
	// error occurs.
	runErr := a.Run(ctx)

	// Graceful drain of active sessions, tunnel close, then logger flush+close.
	// Done on the main goroutine so the drain completes deterministically before
	// process exit.
	a.Shutdown()

	if runErr != nil {
		// Logged by Shutdown's drain as well; surface the fatal code path.
		os.Exit(1)
	}
}
