package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"runtime"
	"runtime/pprof"
	"strconv"

	playa "github.com/lin-string/go-playa"
)

// Two decimal places place the digest grid above cross-runtime OCR geometry
// noise while retaining 0.01 PDF user-unit precision.
const benchmarkFloatDigits = 2

// Break decimal half-way ties consistently when independent runtimes land on
// opposite sides of the same mathematical value by a few floating-point ULPs.
const benchmarkFloatTieEpsilon = 1e-9

type benchmarkFloat float64

func (value benchmarkFloat) MarshalJSON() ([]byte, error) {
	number := float64(value)
	if !math.IsNaN(number) && !math.IsInf(number, 0) && math.Abs(number) < 0.5*math.Pow10(-benchmarkFloatDigits) {
		number = 0
	}
	if math.IsNaN(number) || math.IsInf(number, 0) {
		return nil, fmt.Errorf("non-finite benchmark float %v", number)
	}
	scale := float64(math.Pow10(benchmarkFloatDigits))
	magnitude := math.Floor(math.Abs(number)*scale + 0.5 + benchmarkFloatTieEpsilon)
	number = math.Copysign(magnitude/scale, number)
	return []byte(strconv.FormatFloat(number, 'f', benchmarkFloatDigits, 64)), nil
}

func benchmarkPoint(value [2]float64) [2]benchmarkFloat {
	return [2]benchmarkFloat{benchmarkFloat(value[0]), benchmarkFloat(value[1])}
}

func benchmarkBox(value [4]float64) [4]benchmarkFloat {
	return [4]benchmarkFloat{benchmarkFloat(value[0]), benchmarkFloat(value[1]), benchmarkFloat(value[2]), benchmarkFloat(value[3])}
}

type glyphOutput struct {
	Text         string            `json:"text"`
	Origin       [2]benchmarkFloat `json:"origin"`
	Displacement [2]benchmarkFloat `json:"displacement"`
	BBox         [4]benchmarkFloat `json:"bbox"`
}

type textOutput struct {
	Text   string            `json:"text"`
	BBox   [4]benchmarkFloat `json:"bbox"`
	Glyphs []glyphOutput     `json:"glyphs"`
}

type pageOutput struct {
	Page  int          `json:"page"`
	Texts []textOutput `json:"texts"`
}

func openBenchmarkPDF(path string) (*playa.Document, error) {
	return playa.Open(path, playa.WithCoordinateSpace(playa.CoordinateSpacePage))
}

func marshalBenchmarkJSON(value any) ([]byte, error) {
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(output.Bytes(), []byte{'\n'}), nil
}

func main() {
	cpuprofile := flag.String("cpuprofile", "", "write a CPU profile")
	memprofile := flag.String("memprofile", "", "write a heap profile after the run")
	mutexprofile := flag.String("mutexprofile", "", "write a mutex contention profile after the run")
	blockprofile := flag.String("blockprofile", "", "write a goroutine blocking profile after the run")
	task := flag.String("task", "text-glyph-jsonl", "benchmark task (open-pages, text-glyphs, text-glyph-jsonl, layout-items, image-digests, objects)")
	workers := flag.Int("workers", 1, "page workers for a benchmark task")
	jsonl := flag.Bool("jsonl", false, "write benchmark result records as JSONL")
	flag.Parse()
	if *task == "" {
		*task = "text-glyph-jsonl"
	}
	_ = *jsonl
	if len(flag.Args()) < 1 {
		fmt.Fprintln(os.Stderr, "usage: go run ./bench/go <pdf> [<pdf> ...]")
		os.Exit(2)
	}
	if *cpuprofile != "" {
		file, err := os.Create(*cpuprofile)
		if err != nil {
			fatal("cpu profile", err)
		}
		if err := pprof.StartCPUProfile(file); err != nil {
			fatal("cpu profile", err)
		}
		defer func() {
			pprof.StopCPUProfile()
			_ = file.Close()
		}()
	}
	if *mutexprofile != "" {
		runtime.SetMutexProfileFraction(1)
	}
	if *blockprofile != "" {
		runtime.SetBlockProfileRate(1)
	}
	if *task != "" {
		if *task != "text-glyph-jsonl" && *task != "open-pages" && *task != "text-glyphs" && *task != "layout-items" && *task != "image-digests" && *task != "objects" {
			fatal("benchmark", fmt.Errorf("unsupported task %q", *task))
		}
		if *workers > 0 {
			runtime.GOMAXPROCS(*workers)
		}
		for _, path := range flag.Args() {
			var result benchmarkResult
			var err error
			if *task == "text-glyph-jsonl" {
				result, err = runTextGlyphJSONL(path, *workers)
			} else {
				result, err = runCountTask(path, *task, *workers)
			}
			if err != nil {
				fatal(path, err)
			}
			if err := writeBenchmarkJSONL(os.Stdout, result); err != nil {
				fatal("benchmark output", err)
			}
		}
		if *memprofile != "" {
			file, err := os.Create(*memprofile)
			if err != nil {
				fatal("memory profile", err)
			}
			if err := pprof.WriteHeapProfile(file); err != nil {
				_ = file.Close()
				fatal("memory profile", err)
			}
			if err := file.Close(); err != nil {
				fatal("memory profile", err)
			}
		}
		if err := writeRuntimeProfile("mutex", *mutexprofile); err != nil {
			fatal("mutex profile", err)
		}
		if err := writeRuntimeProfile("block", *blockprofile); err != nil {
			fatal("block profile", err)
		}
		return
	}

}

func writeRuntimeProfile(name, path string) error {
	if path == "" {
		return nil
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	profile := pprof.Lookup(name)
	if profile == nil {
		_ = file.Close()
		return fmt.Errorf("runtime profile %q is unavailable", name)
	}
	if err := profile.WriteTo(file, 0); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func fatal(path string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
	os.Exit(1)
}
