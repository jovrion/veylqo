// Command veylqo is the process entrypoint. It loads configuration,
// blank-imports the database drivers, applies migrations, and owns
// server shutdown. It is not a business layer (ADR-0002, ADR-0003).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

func main() {
	cfg, err := parseConfig(os.Args[1:], os.LookupEnv)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, "veylqo:", redact(err.Error(), envSecrets(os.LookupEnv)))
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := serve(ctx, cfg, logger); err != nil {
		fmt.Fprintln(os.Stderr, "veylqo:", redact(err.Error(), cfg.secrets()))
		os.Exit(1)
	}
}
