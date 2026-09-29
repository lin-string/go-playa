// playa is the command-line inspector for go-playa documents.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"iter"
	"os"
	"strconv"
	"strings"

	"github.com/lin-string/go-playa"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		if errors.Is(err, errUsage) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

var errUsage = errors.New("playa: usage error")

func run(arguments []string, output io.Writer) (err error) {
	var pages string
	var modes modeFlags
	flags := flag.NewFlagSet("playa", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.BoolVar(&modes.content, "content-streams", false, "dump page content operations")
	flags.BoolVar(&modes.outline, "outline", false, "dump the document outline")
	flags.BoolVar(&modes.structure, "structure", false, "dump the structure tree")
	flags.BoolVar(&modes.text, "text", false, "extract page text")
	flags.BoolVar(&modes.textObjects, "text-objects", false, "dump text objects")
	flags.BoolVar(&modes.images, "images", false, "dump page images")
	flags.BoolVar(&modes.fonts, "fonts", false, "dump document fonts")
	flags.StringVar(&pages, "pages", "", "comma-separated zero-based page indices")
	if err := flags.Parse(arguments); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			flags.SetOutput(output)
			flags.Usage()
			return nil
		}
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if flags.NArg() != 1 {
		return fmt.Errorf("%w: one PDF path is required", errUsage)
	}
	indices, err := parsePages(pages)
	if err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if modes.count() != 1 {
		return fmt.Errorf("%w: select exactly one mode", errUsage)
	}
	doc, err := playa.Open(flags.Arg(0))
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := doc.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()
	return modes.run(doc, indices, output)
}

type modeFlags struct {
	content, outline, structure, text, textObjects, images, fonts bool
}

func (m modeFlags) count() int {
	return boolInt(m.content) + boolInt(m.outline) + boolInt(m.structure) + boolInt(m.text) + boolInt(m.textObjects) + boolInt(m.images) + boolInt(m.fonts)
}

func (m modeFlags) run(doc *playa.Document, indices []int, output io.Writer) error {
	if m.outline {
		outline, err := doc.CollectOutline()
		if err != nil {
			return err
		}
		return encodeJSON(output, outline)
	}
	if m.structure {
		structure, err := doc.StructureTree()
		if err != nil {
			return err
		}
		return encodeJSON(output, structure)
	}
	if m.fonts {
		fonts, err := doc.Fonts()
		if err != nil {
			return err
		}
		return encodeJSON(output, fonts)
	}
	for page, err := range doc.Pages() {
		if err != nil {
			return err
		}
		if !selectedPage(page.Number()-1, indices) {
			continue
		}
		switch {
		case m.text:
			for text, err := range page.Texts(doc) {
				if err != nil {
					return err
				}
				if _, err := fmt.Fprintln(output, text.Text()); err != nil {
					return err
				}
			}
		case m.content:
			if err := encodePageSequence(output, page.Contents(doc)); err != nil {
				return err
			}
		case m.textObjects:
			if err := encodePageSequence(output, page.Texts(doc)); err != nil {
				return err
			}
		case m.images:
			if err := encodePageSequence(output, page.Images(doc)); err != nil {
				return err
			}
		}
	}
	return nil
}

func encodePageSequence[T any](output io.Writer, sequence iter.Seq2[T, error]) error {
	for value, err := range sequence {
		if err != nil {
			return err
		}
		if err := encodeJSON(output, value); err != nil {
			return err
		}
	}
	return nil
}

func encodeJSON(output io.Writer, value any) error {
	return json.NewEncoder(output).Encode(value)
}

func selectedPage(index int, indices []int) bool {
	if len(indices) == 0 {
		return true
	}
	for _, selected := range indices {
		if selected == index {
			return true
		}
	}
	return false
}

func parsePages(value string) ([]int, error) {
	if value == "" {
		return nil, nil
	}
	parts := strings.Split(value, ",")
	pages := make([]int, 0, len(parts))
	for _, part := range parts {
		index, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || index < 0 {
			return nil, fmt.Errorf("invalid zero-based page index %q", part)
		}
		pages = append(pages, index)
	}
	return pages, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
