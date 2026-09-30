// Package cli wires the foreground commands to the local gateway.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"

	"kiro-gateway/internal/gateway"
)

const usage = `Usage: kiro-gateway <command>

Commands:
  serve     Start the local HTTP server
  version   Print the build version

Serve options:
  --listen  Numeric IPv4 loopback address and port (default 127.0.0.1:8787)

Environment:
  KIRO_GATEWAY_TOKEN  Required for serve, a random credential of at least 32 characters
`

// Run executes a command. Errors never include raw arguments or environment values.
func Run(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer, version string) error {
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
		logger := slog.New(slog.NewTextHandler(stderr, nil))
		return gateway.Run(ctx, *listen, getenv("KIRO_GATEWAY_TOKEN"), version, logger)
	default:
		return errors.New("unknown command; use --help for usage")
	}
}
