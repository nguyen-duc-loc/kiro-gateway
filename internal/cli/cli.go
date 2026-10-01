// Package cli wires the foreground commands to the local gateway.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"kiro-gateway/internal/configstore"
	"kiro-gateway/internal/gateway"
)

const usage = `Usage: kiro-gateway <command>

Commands:
  serve     Start the local HTTP server
  version   Print the build version
  config init|check|upgrade   Create, validate, or explicitly upgrade settings
  account link|forget        Select or forget the saved IAM Identity Center session

Serve options:
  --listen  Numeric IPv4 loopback address and port (default 127.0.0.1:8787)

Environment:
  KIRO_GATEWAY_TOKEN  Required for serve, a random credential of at least 32 characters

Settings: ~/.config/kiro-gateway/config.json
Stop the gateway and finish manual JSON edits before running configuration commands.
Changes take effect after restart. An explicit --listen overrides saved settings.
Use owned directories (gateway mode 0700) and regular files (mode 0600), without symlinks.
Link saves a snapshot reference and clears mappings when the source changes.
Forget removes the reference and mappings, leaving Kiro CLI credentials intact.
Diagnostics go to stderr. No diagnostic files, conversations, or telemetry are retained.
`

// Run executes a command. Errors never include raw arguments or environment values.
func Run(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer, version string) error {
	return run(ctx, args, getenv, stdout, stderr, version, os.UserHomeDir)
}

func run(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer, version string, userHome func() (string, error)) error {
	if len(args) == 0 {
		return errors.New("choose serve or version; use --help for usage")
	}
	if version == "" {
		version = "dev"
	}
	switch args[0] {
	case "help", "--help", "-h":
		_, err := io.WriteString(stdout, usage)
		return err
	case "version":
		if len(args) != 1 {
			return errors.New("version does not accept arguments")
		}
		_, err := fmt.Fprintln(stdout, version)
		return err
	case "config", "account":
		return localCommand(ctx, args, stdout, stderr, userHome)
	case "serve":
		flags := flag.NewFlagSet("serve", flag.ContinueOnError)
		// The flag package otherwise echoes unknown arguments, which may contain secrets.
		flags.SetOutput(io.Discard)
		listen := flags.String("listen", "127.0.0.1:8787", "Numeric IPv4 loopback address and port")
		if err := flags.Parse(args[1:]); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				_, err := io.WriteString(stdout, usage)
				return err
			}
			return errors.New("invalid serve options; use serve --help for usage")
		}
		if flags.NArg() != 0 {
			return errors.New("serve does not accept positional arguments")
		}
		home, err := userHome()
		if err != nil {
			return errors.New("could not resolve the current user's home")
		}
		store, err := configstore.Open(home, true)
		if err != nil {
			return err
		}
		defer store.Close()
		document, _, err := store.Load()
		if err != nil {
			return err
		}
		explicit := false
		flags.Visit(func(f *flag.Flag) {
			if f.Name == "listen" {
				explicit = true
			}
		})
		if !explicit {
			*listen = document.Listen
		}
		logger := slog.New(slog.NewTextHandler(stderr, nil))
		return gateway.Run(ctx, *listen, getenv("KIRO_GATEWAY_TOKEN"), version, logger)
	default:
		return errors.New("unknown command; use --help for usage")
	}
}

func completion(logger *slog.Logger, command string, started time.Time, err error) {
	if err == nil {
		logger.Info("Local command completed", "event", command, "outcome", "success", "elapsed", time.Since(started))
		return
	}
	logger.Error("Local command failed", "event", command, "outcome", "failure", "elapsed", time.Since(started), "error_category", errorCategory(err))
}
