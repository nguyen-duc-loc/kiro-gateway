package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"kiro-gateway/internal/config"
	"kiro-gateway/internal/configstore"
)

// The consumer owns the minimal persistence contract; adapters own all I/O.
type settingsStore interface {
	Load() (config.Document, bool, error)
	Save(config.Document, bool) error
	Cleanup() error
}

func localCommand(ctx context.Context, args []string, stdout, stderr io.Writer, userHome func() (string, error)) (result error) {
	if len(args) == 2 && args[1] == "--help" || len(args) == 3 && args[2] == "--help" && validLocal(args[:2]) {
		_, err := io.WriteString(stdout, usage)
		return err
	}
	if !validLocal(args) {
		return errors.New("invalid local command; use --help for usage")
	}
	command := args[0] + "_" + args[1]
	mutating := command != "config_check"
	if mutating {
		started := time.Now()
		logger := slog.New(slog.NewTextHandler(stderr, nil))
		defer func() { completion(logger, command, started, result) }()
	}
	if ctx.Err() != nil {
		return errors.New("local command canceled; retry when ready")
	}
	home, err := userHome()
	if err != nil {
		return errors.New("could not resolve the current user's home")
	}
	store, err := configstore.Open(home, mutating)
	if err != nil {
		return err
	}
	defer store.Close()
	message, err := settingsCommand(ctx, command, store, home)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, message)
	return err
}

func validLocal(args []string) bool {
	if len(args) != 2 {
		return false
	}
	return args[0] == "config" && (args[1] == "init" || args[1] == "check" || args[1] == "upgrade") || args[0] == "account" && (args[1] == "link" || args[1] == "forget")
}

func settingsCommand(ctx context.Context, command string, store settingsStore, home string) (string, error) {
	if command == "config_init" {
		// Save uses a nonreplacing link, including when the existing file is invalid.
		if err := store.Save(config.Default(), true); err != nil {
			return "", err
		}
		return "Configuration created.", nil
	}
	d, exists, err := store.Load()
	if err != nil {
		return "", err
	}
	switch command {
	case "config_check":
		if !exists {
			return "No saved configuration; defaults are valid.", nil
		}
		return "Configuration valid.", nil
	case "config_upgrade":
		if !exists {
			return "", configstore.ErrMissing
		}
		return "Configuration is already current.", store.Cleanup()
	case "account_forget":
		if !d.Forget() {
			return "No session is linked; no changes made.", store.Cleanup()
		}
		if err := store.Save(d, false); err != nil {
			return "", err
		}
		return "Session reference and model mappings removed.", nil
	case "account_link":
		if !exists {
			return "", configstore.ErrMissing
		}
		return linkSession(ctx, store, d, home)
	}
	return "", errors.New("invalid local command")
}

func errorCategory(err error) string {
	switch {
	case errors.Is(err, config.ErrInvalid):
		return "configuration_invalid"
	case errors.Is(err, config.ErrVersion):
		return "configuration_version"
	case errors.Is(err, configstore.ErrLocked):
		return "configuration_locked"
	case errors.Is(err, configstore.ErrUncertain):
		return "save_uncertain"
	case errors.Is(err, configstore.ErrUnsafe):
		return "unsafe_path"
	default:
		return captureCategory(err)
	}
}
