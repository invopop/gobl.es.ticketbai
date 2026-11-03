package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/invopop/gobl.es.ticketbai/internal/gateways"
)

// build data provided by goreleaser and mage setup
var (
	version = "dev"
	date    = ""
)

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	return root().cmd().ExecuteContext(ctx)
}

func inputFilename(args []string) string {
	if len(args) > 0 && args[0] != "-" {
		return args[0]
	}
	return ""
}

type errorBody struct {
	Key   string `json:"key,omitempty"`
	Code  string `json:"code,omitempty"`
	Error string `json:"error"`
}

func handleError(err error) {
	if err == nil {
		return
	}

	eb := new(errorBody)
	if e, ok := err.(*gateways.Error); ok {
		eb.Key = e.Key()
		eb.Code = e.Code()
		eb.Error = e.Message()
	} else {
		eb.Error = err.Error()
	}

	data, err := json.Marshal(eb)
	if err != nil {
		panic(err)
	}
	fmt.Fprintf(os.Stderr, "%s\n", string(data))
}
