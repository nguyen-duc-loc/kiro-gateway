// Package gateway owns local authentication, HTTP transport, and server lifecycle.
package gateway

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"sync/atomic"
	"syscall"
	"time"
)

const shutdownTimeout = 5 * time.Second

// Run binds a validated loopback address and serves until cancellation or failure.
// It never obtains Kiro credentials or starts an upstream request.
func Run(ctx context.Context, listen, token, version string, logger *slog.Logger) error {
	address, err := netip.ParseAddrPort(listen)
	if err != nil || !address.Addr().Is4() || !address.Addr().IsLoopback() {
		return errors.New("--listen must be a numeric IPv4 loopback address and port, such as 127.0.0.1:8787")
	}
	if len(token) < 32 {
		return errors.New("KIRO_GATEWAY_TOKEN is required and must contain at least 32 characters")
	}
	for _, c := range token {
		if c < '!' || c > '~' {
			return errors.New("KIRO_GATEWAY_TOKEN must contain visible ASCII characters without spaces")
		}
	}

	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp4", address.String())
	if err != nil {
		if errors.Is(err, syscall.EADDRINUSE) {
			return errors.New("cannot start gateway: listen port is already in use")
		}
		return errors.New("cannot start gateway: could not bind the loopback listener")
	}
	defer listener.Close()

	server := &http.Server{
		Handler:           newHealthHandler(token, version, logger),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
		BaseContext:       func(net.Listener) context.Context { return ctx },
		ErrorLog:          log.New(safeServerLog{logger}, "", 0),
	}
	result := make(chan error, 1)
	go func() { result <- server.Serve(listener) }()
	logger.Info(fmt.Sprintf("Gateway listening on %s", listener.Addr()), "event", "listening")

	select {
	case err := <-result:
		_ = server.Close()
		if !errors.Is(err, http.ErrServerClosed) {
			return errors.New("gateway HTTP server stopped unexpectedly")
		}
		return nil
	case <-ctx.Done():
		logger.Info("Stopping gateway", "event", "stopping")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			logger.Warn("Shutdown deadline reached; remaining connections closed", "event", "shutdown", "error_category", "shutdown_timeout")
		}
		if err := <-result; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return errors.New("gateway HTTP server stopped unexpectedly")
		}
		logger.Info("Gateway stopped", "event", "stopped")
		return nil
	}
}

func newHealthHandler(token, version string, logger *slog.Logger) http.Handler {
	// Hash first so constant time comparison always receives equal length values.
	expected := sha256.Sum256([]byte("Bearer " + token))
	health, _ := json.Marshal(struct {
		Status  string `json:"status"`
		Version string `json:"version"`
	}{Status: "running", Version: version})
	health = append(health, '\n')
	var sequence atomic.Uint64

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		requestID := sequence.Add(1)
		status := http.StatusOK
		category := ""
		defer func() {
			attrs := []any{"event", "request", "request_id", requestID, "outcome", status, "elapsed", time.Since(started)}
			if category != "" {
				attrs = append(attrs, "error_category", category)
			}
			logger.Info("Request completed", attrs...)
		}()

		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		provided := sha256.Sum256([]byte(r.Header.Get("Authorization")))
		if subtle.ConstantTimeCompare(expected[:], provided[:]) != 1 || len(r.Header.Values("Authorization")) != 1 {
			status = http.StatusUnauthorized
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "Unauthorized", status)
			return
		}
		if r.URL.Path != "/healthz" {
			status = http.StatusNotFound
			http.Error(w, "Not found", status)
			return
		}
		if r.Method != http.MethodGet {
			status = http.StatusMethodNotAllowed
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "Method not allowed", status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(health); err != nil {
			category = "response_write"
		}
	})
}

// net/http can log raw panic values. Keep those out of routine diagnostics.
type safeServerLog struct{ logger *slog.Logger }

func (s safeServerLog) Write(p []byte) (int, error) {
	s.logger.Error("HTTP transport error", "event", "transport_error", "error_category", "http_server")
	return len(p), nil
}
