// Command playa-engineering-check enforces engineering contracts in Go source.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/lin-string/go-playa/internal/engineeringcheck"
)

func main() {
	root := flag.String("root", ".", "source root to check")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "playa-engineering-check: unexpected arguments")
		os.Exit(2)
	}
	diagnostics, err := engineeringcheck.Scan(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "playa-engineering-check:", err)
		os.Exit(2)
	}
	for _, diagnostic := range diagnostics {
		fmt.Fprintln(os.Stderr, diagnostic)
	}
	if len(diagnostics) > 0 {
		os.Exit(1)
	}
}
