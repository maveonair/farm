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

	"github.com/maveonair/farm/internal/app"
)

const usage = `usage:
  farm run -config PATH
  farm validate -config PATH
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("farm stopped", "event", "service_failed", "error", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New(usage)
	}

	switch args[0] {
	case "run":
		return runDaemon(args[1:])
	case "validate":
		return validate(args[1:])
	default:
		return fmt.Errorf("unknown command %q\n%s", args[0], usage)
	}
}

func validate(args []string) error {
	flags := flag.NewFlagSet("validate", flag.ContinueOnError)
	path := flags.String("config", "", "configuration path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *path == "" {
		return errors.New("config path is required")
	}

	if err := app.Validate(*path); err != nil {
		return err
	}
	fmt.Println("configuration is valid")
	return nil
}

func runDaemon(args []string) error {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	path := flags.String("config", "", "configuration path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *path == "" {
		return errors.New("config path is required")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return app.Run(ctx, *path)
}
