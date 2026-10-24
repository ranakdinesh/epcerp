package infrastructure

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/ranakdinesh/epcERP/internal/logging"
)

// HTTPServer owns API transport settings and graceful shutdown.
type HTTPServer struct {
	server *http.Server
	logger logging.Logger
}

func NewHTTPServer(address string, handler http.Handler, logger logging.Logger) *HTTPServer {
	return &HTTPServer{
		server: &http.Server{
			Addr: address, Handler: handler,
			ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second,
			WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second,
			MaxHeaderBytes: 16 << 10,
			ErrorLog:       ServerErrorLogger(logger),
		},
		logger: logger,
	}
}

// Run serves requests until the listener fails or the runtime context is canceled.
func (s *HTTPServer) Run(ctx context.Context) error {
	s.logger.Info().Str("event", "api.starting").Msg("API starting")
	finished := make(chan error, 1)
	go func() { finished <- s.server.ListenAndServe() }()
	select {
	case err := <-finished:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		s.logger.Info().Str("event", "api.shutdown.started").Msg("API shutting down")
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.server.Shutdown(shutdown); err != nil {
			_ = s.server.Close()
			return err
		}
		return nil
	}
}

// ServerErrorLogger suppresses raw net/http diagnostics (which can contain peer
// addresses, panic values or TLS details) and emits a bounded operational event.
func ServerErrorLogger(logger logging.Logger) *log.Logger {
	return log.New(serverErrorWriter{logger}, "", 0)
}

type serverErrorWriter struct{ logger logging.Logger }

func (w serverErrorWriter) Write(message []byte) (int, error) {
	w.logger.Error().Str("event", "http.server.error").Msg("HTTP server reported an error")
	return len(message), nil
}
