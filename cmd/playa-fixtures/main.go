// Command playa-fixtures synchronizes public PDF Association test fixtures.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/lin-string/go-playa/internal/testfixture"
)

var (
	syncPDFAFixtures  = testfixture.SyncPDFAFrom
	checkPDFAFixtures = testfixture.CheckPDFAFrom
	pdfaPaths         = testfixture.PDFAPathsFrom
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "playa-fixtures: determine working directory: %v\n", err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	if err := run(ctx, cwd, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "playa-fixtures: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, start string, args []string, output io.Writer) error {
	flags := flag.NewFlagSet("playa-fixtures", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	pdfa := flags.Bool("pdf-association", false, "operate on the public PDF Association corpus")
	check := flags.Bool("check", false, "verify local fixtures without changing them")
	listCompat := flags.Bool("list-compat", false, "print verified compatibility fixture paths")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			flags.SetOutput(output)
			flags.Usage()
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", flags.Args())
	}
	if !*pdfa {
		return fmt.Errorf("select --pdf-association for this corpus; use make public-fixtures-pull for the public large-PDF corpus")
	}
	if *check && *listCompat {
		return fmt.Errorf("--check and --list-compat are mutually exclusive")
	}
	if *listCompat {
		paths, err := pdfaPaths(ctx, start, true)
		if err != nil {
			return err
		}
		for _, path := range paths {
			if _, err := fmt.Fprintln(output, path); err != nil {
				return fmt.Errorf("write fixture path: %w", err)
			}
		}
		return nil
	}
	if *check {
		if err := checkPDFAFixtures(ctx, start); err != nil {
			return err
		}
		_, err := fmt.Fprintln(output, "PDF Association test fixtures are verified")
		return err
	}
	if err := syncPDFAFixtures(ctx, start); err != nil {
		return err
	}
	_, err := fmt.Fprintln(output, "PDF Association test fixtures are ready")
	return err
}
