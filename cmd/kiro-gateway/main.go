package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"kiro-gateway/internal/cli"
)

// version can be set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := cli.Run(ctx, os.Args[1:], os.Getenv, os.Stdout, os.Stderr, version)
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
