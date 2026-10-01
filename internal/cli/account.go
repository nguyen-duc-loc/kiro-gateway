package cli

import (
	"context"
	"errors"

	"kiro-gateway/internal/config"
	"kiro-gateway/internal/credentials"
)

type sessionCapture interface {
	Capture(context.Context) (config.Session, error)
}

func linkSession(ctx context.Context, store settingsStore, d config.Document, home string) (string, error) {
	return captureAndLink(ctx, store, d, credentials.Reader{Home: home})
}

func captureAndLink(ctx context.Context, store settingsStore, d config.Document, source sessionCapture) (string, error) {
	s, err := source.Capture(ctx)
	if err != nil {
		return "", err
	}
	if !d.Link(s) {
		return "Session is already linked; no changes made.", store.Cleanup()
	}
	if err := store.Save(d, false); err != nil {
		return "", err
	}
	return "Saved IAM Identity Center session linked; model mappings are empty.", nil
}

func captureCategory(err error) string {
	switch {
	case errors.Is(err, credentials.ErrBusy):
		return "source_busy"
	case errors.Is(err, credentials.ErrTimeout):
		return "source_timeout"
	case errors.Is(err, credentials.ErrCanceled):
		return "source_canceled"
	case errors.Is(err, credentials.ErrExpired):
		return "credential_expired"
	case errors.Is(err, credentials.ErrRecord):
		return "record_invalid"
	case errors.Is(err, credentials.ErrSource):
		return "source_unavailable"
	default:
		return "local_command"
	}
}
