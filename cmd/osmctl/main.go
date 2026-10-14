package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/happyhackingspace/osmctl/internal/cli"
)

var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := cli.Execute(ctx, version)
	stop()
	os.Exit(code)
}
