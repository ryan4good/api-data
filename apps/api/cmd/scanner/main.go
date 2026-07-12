package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"bizdevops/apps/api/internal/modules/scanner"
)

type output struct {
	Status     scanner.Status      `json:"status"`
	Operations []scanner.Operation `json:"operations"`
}

func run(args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: scanner <repository-root>")
	}
	operations, err := scanner.Analyze(args[0])
	if err != nil {
		return err
	}
	return json.NewEncoder(stdout).Encode(output{
		Status:     scanner.StatusSucceeded,
		Operations: operations,
	})
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
