// playa-compat emits the compatibility JSON projection for one PDF.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/lin-string/go-playa"
	"github.com/lin-string/go-playa/internal/testcompat"
	"github.com/lin-string/go-playa/internal/testfixture"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := runContext(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		if isUsageError(err) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

type usageError struct {
	err error
}

func (e usageError) Error() string { return e.err.Error() }
func (e usageError) Unwrap() error { return e.err }

func isUsageError(err error) bool {
	var target usageError
	return errors.As(err, &target)
}

func run(arguments []string, output io.Writer) (err error) {
	return runContext(context.Background(), arguments, output)
}

func runContext(ctx context.Context, arguments []string, output io.Writer) (err error) {
	var pdf string
	var pdfs stringList
	var pages string
	var memprofile string
	var cpuprofile string
	var spaces stringList
	var compare bool
	var jsonl bool
	var noCache bool
	var cacheDir string
	var tolerance float64
	var sections []string
	var release bool
	var pruneCache bool
	var workers int
	var memoryLimitMiB int64
	var password string
	var passwordSet bool
	var pdfAssociation bool
	var issueDraft string
	var issueStatus bool
	var timingModeValue string
	flags := flag.NewFlagSet("playa-compat", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.Var(&pdfs, "pdf", "path to a PDF file; repeat for multiple compatibility jobs")
	flags.StringVar(&pages, "pages", "", "comma-separated zero-based page indices; empty selects every page")
	flags.StringVar(&memprofile, "memprofile", "", "write a heap profile after snapshotting")
	flags.StringVar(&cpuprofile, "cpuprofile", "", "write a CPU profile for the full command")
	flags.Var(&spaces, "space", "coordinate space: page, screen, default, or user; repeat with --compare")
	flags.BoolVar(&compare, "compare", false, "compare the Go snapshot with the pinned Playa snapshot")
	flags.BoolVar(&jsonl, "jsonl", false, "write a streaming JSONL snapshot instead of one JSON document")
	flags.BoolVar(&noCache, "no-cache", false, "disable Playa snapshot caching")
	flags.StringVar(&cacheDir, "cache-dir", ".compat-cache", "Playa snapshot cache directory")
	flags.Float64Var(&tolerance, "tolerance", 1e-6, "absolute float tolerance")
	flags.BoolVar(&release, "release", false, "fail when the compatibility manifest has pending sections")
	flags.BoolVar(&pruneCache, "prune-cache", false, "remove confirmed older JSONL compatibility cache entries")
	flags.IntVar(&workers, "workers", 0, "maximum concurrent compatibility jobs; 0 uses GOMAXPROCS")
	flags.Int64Var(&memoryLimitMiB, "memory-limit-mib", 0, "estimated aggregate memory budget in MiB; 0 selects a system-aware default")
	flags.BoolVar(&pdfAssociation, "pdf-association", false, "compare the pinned PDF Association compatibility corpus")
	flags.StringVar(&issueDraft, "issue-draft", "", "render a neutral upstream issue draft for a recorded PDF Association difference")
	flags.BoolVar(&issueStatus, "issue-status", false, "list recorded upstream issue state for PDF Association differences")
	flags.StringVar(&timingModeValue, "timing-mode", "auto", "compatibility timing policy: auto, off, report, or track")
	flags.Func("password", "PDF password; the value is never stored in compatibility cache metadata", func(value string) error {
		password, passwordSet = value, true
		return nil
	})
	flags.Func("section", "compatibility section to compare; repeat to select several", func(value string) error {
		sections = append(sections, value)
		return nil
	})
	if err := flags.Parse(arguments); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			flags.SetOutput(output)
			flags.Usage()
			return nil
		}
		return usageError{err: err}
	}
	timingPolicy, err := parseTimingMode(timingModeValue)
	if err != nil {
		return usageError{err: err}
	}
	timingPolicy = resolveTimingMode(timingPolicy, os.Getenv)
	if issueDraft != "" {
		if len(pdfs) != 0 || pdfAssociation || compare || jsonl || release || pruneCache || issueStatus {
			return usageError{err: fmt.Errorf("--issue-draft cannot be combined with PDF or compatibility execution flags")}
		}
		draft, err := renderPDFAIssueDraft(issueDraft)
		if err != nil {
			return err
		}
		_, err = io.WriteString(output, draft)
		return err
	}
	if issueStatus {
		if len(pdfs) != 0 || pdfAssociation || compare || jsonl || release || pruneCache {
			return usageError{err: fmt.Errorf("--issue-status cannot be combined with PDF or compatibility execution flags")}
		}
		manifest, err := testfixture.PDFAManifestValue()
		if err != nil {
			return err
		}
		_, err = io.WriteString(output, renderPDFAUpstreamIssueStatus(manifest))
		return err
	}
	if pruneCache {
		if len(pdfs) != 0 || pdfAssociation || compare || jsonl || release {
			return usageError{err: fmt.Errorf("--prune-cache cannot be combined with PDF or compatibility execution flags")}
		}
		report, err := pruneCompatCache(cacheDir)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(output, "compat cache: removed %d file(s), %d byte(s)\n", report.Files, report.Bytes)
		return err
	}
	if pdfAssociation {
		if !compare {
			return usageError{err: fmt.Errorf("--pdf-association requires --compare")}
		}
		if len(pdfs) != 0 || pages != "" || len(sections) != 0 || passwordSet {
			return usageError{err: fmt.Errorf("--pdf-association selects its own PDFs, pages, sections, and password")}
		}
	} else if len(pdfs) == 0 {
		return usageError{err: fmt.Errorf("--pdf is required")}
	}
	if memoryLimitMiB < 0 || memoryLimitMiB > int64(^uint64(0)>>1)>>20 {
		return usageError{err: fmt.Errorf("--memory-limit-mib must be zero or a positive MiB value")}
	}
	indices, err := parsePages(pages)
	if err != nil {
		return usageError{err: err}
	}
	if len(spaces) == 0 {
		spaces = append(spaces, "page")
	}
	coordinateSpaces := make([]playa.CoordinateSpace, len(spaces))
	for index, space := range spaces {
		coordinateSpace, parseErr := parseCoordinateSpace(space)
		if parseErr != nil {
			return usageError{err: parseErr}
		}
		coordinateSpaces[index] = coordinateSpace
	}
	if compare {
		for _, coordinateSpace := range coordinateSpaces {
			if coordinateSpace == playa.CoordinateSpaceUser {
				return usageError{err: fmt.Errorf("playa compatibility oracle does not support coordinate space %q; use page, screen, or default", coordinateSpace)}
			}
		}
		stopCPU, profileErr := startCPUProfile(cpuprofile)
		if profileErr != nil {
			return profileErr
		}
		defer func() {
			if stopErr := stopCPU(); stopErr != nil && err == nil {
				err = stopErr
			}
		}()
		if release {
			if err := rejectPendingSections(); err != nil {
				return err
			}
		}
		memoryAutomatic := memoryLimitMiB == 0
		memoryLimit := memoryLimitMiB << 20
		physicalMemory := physicalMemoryBytes()
		availableAtStart := availableMemoryBytes()
		if memoryAutomatic {
			memoryLimit = automaticCompareMemoryLimit(physicalMemory)
		}
		workers = resolveCompareWorkers(workers)
		residentAtStart := currentRuntimeCommittedBytes()
		effectiveAtStart := dynamicCompareMemoryCapacity(memoryLimit, availableAtStart, residentAtStart, compareMemoryReserve)
		if _, err := fmt.Fprintf(output, "compat scheduler: workers=%d memory_limit=%d MiB effective_capacity=%d MiB auto_memory=%t available=%d MiB runtime_committed=%d MiB live_reserve=%d MiB\n", workers, memoryLimit>>20, effectiveAtStart>>20, memoryAutomatic, availableAtStart>>20, residentAtStart>>20, compareMemoryReserve>>20); err != nil {
			return err
		}
		var selected []string
		var knownDifferences []testfixture.PDFADifference
		var compatKnownDifferences []compatDifferenceRecord
		var tasks []compareTask
		if pdfAssociation {
			cases, err := testfixture.PDFACasesFrom(ctx, ".", true)
			if err != nil {
				return err
			}
			manifest, err := testfixture.PDFAManifestValue()
			if err != nil {
				return err
			}
			knownDifferences = manifest.Differences
			tasks = buildPDFACompareTasks(cases, spaces)
		} else {
			selected, err = selectSections(sections)
			if err != nil {
				return usageError{err: err}
			}
			manifest, err := loadCompatDifferenceManifest()
			if err != nil {
				return err
			}
			compatKnownDifferences = manifest.Records
			tasks = make([]compareTask, 0, len(pdfs)*len(spaces))
			for _, path := range pdfs {
				digest, err := fileDigest(path)
				if err != nil {
					return fmt.Errorf("digest compatibility fixture %q: %w", path, err)
				}
				for _, space := range spaces {
					tasks = append(tasks, compareTask{pdf: path, space: space, pages: indices, password: password, passwordSet: passwordSet, fixtureSHA256: digest})
				}
			}
		}
		groupTasks := buildCompatCompareTasks(tasks, selected)
		timingRecorder := newTimingRecorder(timingPolicy, cacheDir, workers, noCache, cpuprofile != "" || memprofile != "")
		if !noCache {
			if _, err := fmt.Fprintf(output, "compat cache preflight: jobs=%d\n", len(groupTasks)); err != nil {
				return err
			}
			if err := runCompareTasksWithResourceProbe(groupTasks, workers, memoryLimit, compareMemoryReserve, availableMemoryBytes, currentRuntimeCommittedBytes, func(task compareTask, _ io.Writer) error {
				expected, _, cacheErr := cacheSnapshotWithPasswordContext(ctx, cacheDir, task.pdf, task.pages, task.space, task.password, task.passwordSet, task.sections...)
				if cacheErr != nil {
					if pdfAssociation {
						_, cacheErr = reconcilePDFAKnownDifferences(task, nil, cacheErr, knownDifferences)
					}
					return cacheErr
				}
				return expected.Close()
			}, io.Discard); err != nil {
				return fmt.Errorf("prepare compatibility cache: %w", err)
			}
			// Cache preflight may leave gigabytes of dead gzip/digest heap pages.
			// Return them before RSS becomes input to the Go-phase capacity.
			collectCompatPhaseMemory()
			if memoryAutomatic {
				availableForCompare := availableMemoryBytes()
				residentForCompare := currentRuntimeCommittedBytes()
				effectiveForCompare := dynamicCompareMemoryCapacity(memoryLimit, availableForCompare, residentForCompare, compareMemoryReserve)
				if _, err := fmt.Fprintf(output, "compat scheduler: phase=compare memory_limit=%d MiB effective_capacity=%d MiB available=%d MiB runtime_committed=%d MiB live_reserve=%d MiB\n", memoryLimit>>20, effectiveForCompare>>20, availableForCompare>>20, residentForCompare>>20, compareMemoryReserve>>20); err != nil {
					return err
				}
			}
		}
		compareErr := runCompareTasksWithResourceProbe(groupTasks, workers, memoryLimit, compareMemoryReserve, availableMemoryBytes, currentRuntimeCommittedBytes, func(task compareTask, taskOutput io.Writer) error {
			actualDifferences, cacheHit, goElapsed, compareErr := compareSnapshotOnceContext(ctx, task.pdf, task.pages, task.space, task.password, task.passwordSet, cacheDir, noCache, tolerance, task.sections)
			var matched []string
			if pdfAssociation {
				var knownErr error
				matched, knownErr = reconcilePDFAKnownDifferences(task, actualDifferences, compareErr, knownDifferences)
				if knownErr != nil {
					return knownErr
				}
				compareErr = nil
				actualDifferences = nil
			} else if compareErr == nil {
				var knownErr error
				matched, knownErr = reconcileCompatKnownDifferences(task, actualDifferences, compatKnownDifferences)
				if knownErr != nil {
					return knownErr
				}
				actualDifferences = nil
			}
			if compareErr != nil {
				return compareErr
			}
			if len(actualDifferences) > 0 {
				return fmt.Errorf("FAIL %s [%s] sections=%s\n%s", task.pdf, task.space, strings.Join(task.sections, ","), strings.Join(actualDifferences, "\n"))
			}
			if timingPolicy != timingModeOff && goElapsed > 0 {
				result, warnings := timingRecorder.record(task, cacheHit, goElapsed)
				encoded, err := json.Marshal(result)
				if err != nil {
					return fmt.Errorf("encode compatibility timing: %w", err)
				}
				if _, err := fmt.Fprintf(taskOutput, "TIMING %s\n", encoded); err != nil {
					return err
				}
				for _, warning := range warnings {
					if _, err := fmt.Fprintf(taskOutput, "TIMING-WARNING %s\n", warning); err != nil {
						return err
					}
				}
				if result.Regressed {
					if _, err := fmt.Fprintf(taskOutput, "TIMING-WARNING regression pdf=%s space=%s sections=%s elapsed=%s baseline=%s ratio=%.2f\n", task.pdf, task.space, strings.Join(task.sections, ","), goElapsed, time.Duration(result.BaselineMedianNS), result.Ratio); err != nil {
						return err
					}
				}
			}
			if len(matched) > 0 {
				_, err := fmt.Fprintf(taskOutput, "KNOWN %s [%s] sections=%s records=%s\n", task.pdf, task.space, strings.Join(task.sections, ","), strings.Join(matched, ","))
				return err
			}
			return nil
		}, output)
		if compareErr == nil {
			suffix := ""
			if !noCache {
				suffix = " (cache hit)"
			}
			for _, task := range tasks {
				if _, err := fmt.Fprintf(output, "PASS %s [%s]%s\n", task.pdf, task.space, suffix); err != nil {
					compareErr = err
					break
				}
			}
		}
		if compareErr == nil {
			compareErr = writeHeapProfile(memprofile)
		}
		return compareErr
	}
	if len(pdfs) != 1 {
		return usageError{err: fmt.Errorf("--pdf may be repeated only with --compare")}
	}
	if len(spaces) != 1 {
		return usageError{err: fmt.Errorf("--space may be repeated only with --compare")}
	}
	pdf = pdfs[0]
	stopCPU, profileErr := startCPUProfile(cpuprofile)
	if profileErr != nil {
		return profileErr
	}
	defer func() {
		if stopErr := stopCPU(); stopErr != nil && err == nil {
			err = stopErr
		}
	}()
	options := []playa.OpenOption{playa.WithCoordinateSpace(coordinateSpaces[0])}
	if passwordSet {
		options = append(options, playa.WithPassword(password))
	}
	doc, err := playa.Open(pdf, options...)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := doc.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("playa: close document: %w", closeErr)
		}
	}()
	if jsonl {
		if err := testcompat.SnapshotJSONL(doc, testcompat.Options{Pages: indices, Sections: sections}, output); err != nil {
			return err
		}
		if profileErr := writeHeapProfile(memprofile); profileErr != nil {
			return profileErr
		}
		return nil
	}
	snapshot, err := testcompat.Snapshot(doc, testcompat.Options{Pages: indices, Sections: sections})
	if err != nil {
		return err
	}
	if profileErr := writeHeapProfile(memprofile); profileErr != nil {
		return profileErr
	}
	if err := json.NewEncoder(output).Encode(snapshot); err != nil {
		return fmt.Errorf("playa: encode compatibility projection: %w", err)
	}
	return nil
}

func buildPDFACompareTasks(cases []testfixture.PDFACase, spaces []string) []compareTask {
	tasks := make([]compareTask, 0, len(cases)*len(spaces))
	for _, testCase := range cases {
		for _, space := range spaces {
			tasks = append(tasks, compareTask{
				pdf:           testCase.Path,
				space:         space,
				password:      testCase.Fixture.Password,
				passwordSet:   testCase.Fixture.Password != "",
				sections:      append([]string(nil), testCase.Fixture.Sections...),
				fixtureID:     testCase.Fixture.ID,
				fixtureSHA256: testCase.Fixture.SHA256,
			})
		}
	}
	return tasks
}

func startCPUProfile(path string) (func() error, error) {
	if path == "" {
		return func() error { return nil }, nil
	}
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("playa: CPU profile: %w", err)
	}
	if err := pprof.StartCPUProfile(file); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("playa: CPU profile: %w", err)
	}
	return func() error {
		pprof.StopCPUProfile()
		if err := file.Close(); err != nil {
			return fmt.Errorf("playa: CPU profile: %w", err)
		}
		return nil
	}, nil
}

func writeHeapProfile(path string) error {
	if path == "" {
		return nil
	}
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("playa: memory profile: %w", err)
	}
	runtime.GC()
	if err := pprof.WriteHeapProfile(file); err != nil {
		_ = file.Close()
		return fmt.Errorf("playa: memory profile: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("playa: memory profile: %w", err)
	}
	return nil
}

func rejectPendingSections() error {
	data, err := os.ReadFile("compat/manifest.toml")
	if err != nil {
		return fmt.Errorf("read compatibility manifest: %w", err)
	}
	if strings.Contains(string(data), `status = "pending"`) {
		return fmt.Errorf("release compatibility check requires no pending sections")
	}
	return nil
}

func compareSnapshotOnceContext(ctx context.Context, pdf string, pages []int, space, password string, passwordSet bool, cacheDir string, noCache bool, tolerance float64, selected []string) ([]string, bool, time.Duration, error) {
	var err error
	var expected *snapshotStream
	cacheHit := false
	if noCache {
		expected, err = generatePlayaSnapshotWithPasswordContext(ctx, pdf, pages, space, password, passwordSet, selected...)
	} else {
		expected, cacheHit, err = cacheSnapshotWithPasswordContext(ctx, cacheDir, pdf, pages, space, password, passwordSet, selected...)
	}
	if err != nil {
		return nil, false, 0, err
	}
	defer func() { _ = expected.Close() }()
	coordinateSpace, err := parseCoordinateSpace(space)
	if err != nil {
		return nil, cacheHit, 0, usageError{err: err}
	}
	compatibilityProcessPhaseGate.acquire(comparePhaseGo)
	started := time.Now()
	defer func() {
		// A large metadata or structure group can leave gigabytes of dead heap
		// after its document closes. Reclaim it before releasing the Go phase;
		// no later oracle subprocess can overlap the retained heap.
		collectCompatGroupMemory()
		compatibilityProcessPhaseGate.release(comparePhaseGo)
	}()
	options := []playa.OpenOption{playa.WithCoordinateSpace(coordinateSpace)}
	if passwordSet {
		options = append(options, playa.WithPassword(password))
	}
	doc, err := playa.Open(pdf, options...)
	if err != nil {
		return nil, cacheHit, time.Since(started), err
	}
	differences, compareErr := compareSnapshotStream(expected, doc, pages, selected, tolerance)
	closeErr := doc.Close()
	goElapsed := time.Since(started)
	if compareErr != nil {
		return nil, cacheHit, goElapsed, fmt.Errorf("compare snapshot: %w", compareErr)
	}
	if closeErr != nil {
		return nil, cacheHit, goElapsed, fmt.Errorf("playa: close document: %w", closeErr)
	}
	return differences, cacheHit, goElapsed, nil
}

func parseCoordinateSpace(value string) (playa.CoordinateSpace, error) {
	switch playa.CoordinateSpace(value) {
	case playa.CoordinateSpacePage, playa.CoordinateSpaceScreen, playa.CoordinateSpaceDefault, playa.CoordinateSpaceUser:
		return playa.CoordinateSpace(value), nil
	default:
		return "", fmt.Errorf("invalid coordinate space %q", value)
	}
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
