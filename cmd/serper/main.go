// Package main provides a CLI tool for Serper.dev searches.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ai8future/chassis-go/call"
	chassisconfig "github.com/ai8future/chassis-go/config"
	"github.com/ai8future/chassis-go/logz"

	"serper_mod/serper"
)

// Config holds CLI configuration loaded from environment.
type Config struct {
	APIKey  string        `env:"SERPER_API_KEY" required:"true"`
	BaseURL string        `env:"SERPER_BASE_URL" default:"https://google.serper.dev"`
	Num     int           `env:"SERPER_NUM" default:"10"`
	GL      string        `env:"SERPER_GL" default:"us"`
	HL      string        `env:"SERPER_HL" default:"en"`
	Timeout time.Duration `env:"SERPER_TIMEOUT" default:"30s"`
	LogLevel string       `env:"LOG_LEVEL" default:"error"`
}

func main() {
	cfg := chassisconfig.MustLoad[Config]()
	logger := logz.New(cfg.LogLevel)

	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: serper <query>")
		os.Exit(1)
	}
	query := strings.Join(os.Args[1:], " ")

	caller := call.New(
		call.WithTimeout(cfg.Timeout),
		call.WithRetry(3, 500*time.Millisecond),
	)

	client, err := serper.New(cfg.APIKey,
		serper.WithBaseURL(cfg.BaseURL),
		serper.WithDoer(caller),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	logger.Debug("searching", "query", query, "num", cfg.Num, "gl", cfg.GL)

	// call.Client already enforces per-attempt timeouts and handles retries,
	// so no additional context timeout is needed here.
	resp, err := client.Search(context.Background(), &serper.SearchRequest{
		Q:   query,
		Num: cfg.Num,
		GL:  cfg.GL,
		HL:  cfg.HL,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	out, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error formatting response: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(out))
}
