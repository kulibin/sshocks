package socks

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"sshocks/internal/log"
)

// Server is a minimal RFC 1928 SOCKS5 TCP server.
type Server struct {
	listener net.Listener
	dialer   Dialer
	log      log.Logger

	sessions  sync.WaitGroup
	sessID    int64 // atomic
	sessCtx   context.Context
	cancel    context.CancelFunc
	listenErr error
}

// NewServer creates a SOCKS5 server bound to listen, dialing targets through the
// provided Dialer. Sessions are tracked so Drain can wait / force-close them.
func NewServer(listen string, dialer Dialer, l log.Logger) *Server {
	ln, err := net.Listen("tcp", listen)
	if err != nil {
		if l != nil {
			l.Errorf("socks: listen %s failed: %v", listen, err)
		}
		return &Server{dialer: dialer, log: l, listenErr: err}
	}
	sessCtx, cancel := context.WithCancel(context.Background())
	return &Server{
		listener: ln,
		dialer:   dialer,
		log:      l,
		sessCtx:  sessCtx,
		cancel:   cancel,
	}
}

// Listen returns the underlying net.Listener.
func (s *Server) Listen() net.Listener { return s.listener }

// ListenErr returns the listener creation error, if any.
func (s *Server) ListenErr() error { return s.listenErr }

// Run accepts connections until ctx is cancelled, then returns. Active sessions
// are drained separately via Drain.
func (s *Server) Run(ctx context.Context) error {
	if s.listener == nil {
		return s.listenErr
	}
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				if s.log != nil {
					s.log.Warnf("socks: accept error: %v", err)
				}
				return err
			}
		}
		id := atomic.AddInt64(&s.sessID, 1)
		s.sessions.Add(1)
		go func() {
			defer s.sessions.Done()
			handleSession(s.sessCtx, conn, s.dialer, s.log, id)
		}()
	}
}

// Shutdown closes the listener so no new sessions are accepted.
func (s *Server) Shutdown() error {
	if s.listener == nil {
		return nil
	}
	return s.listener.Close()
}

// Wait blocks until all accepted sessions finish.
func (s *Server) Wait() {
	s.sessions.Wait()
}

// Drain closes the listener and waits for active sessions up to timeout, then
// force-closes the remaining ones by cancelling their context.
func (s *Server) Drain(timeout time.Duration) {
	if s.log != nil {
		s.log.Infof("socks: draining sessions (timeout %s)...", timeout)
	}
	_ = s.Shutdown()

	done := make(chan struct{})
	go func() {
		s.Wait()
		close(done)
	}()
	select {
	case <-done:
		if s.log != nil {
			s.log.Infof("socks: drain complete, all sessions closed")
		}
	case <-time.After(timeout):
		if s.log != nil {
			s.log.Warnf("socks: drain timeout reached, forcing close of remaining sessions")
		}
		s.cancel()
	}
}
