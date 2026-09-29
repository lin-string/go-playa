package main

import (
	"errors"
	"io"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRunCompareTasksBoundsGoroutinesAndKeepsTaskOutputContiguous(t *testing.T) {
	tasks := []compareTask{{pdf: "a"}, {pdf: "b"}, {pdf: "c"}, {pdf: "d"}}
	var mu sync.Mutex
	active, maximum := 0, 0
	var output strings.Builder
	err := runCompareTasks(tasks, 2, func(task compareTask, result io.Writer) error {
		mu.Lock()
		active++
		if active > maximum {
			maximum = active
		}
		mu.Unlock()
		time.Sleep(time.Millisecond)
		_, writeErr := io.WriteString(result, task.pdf+"\n")
		mu.Lock()
		active--
		mu.Unlock()
		return writeErr
	}, &output)
	if err != nil {
		t.Fatal(err)
	}
	if maximum > 2 {
		t.Fatalf("maximum concurrent jobs = %d, want <= 2", maximum)
	}
	lines := strings.Split(strings.TrimSuffix(output.String(), "\n"), "\n")
	if len(lines) != len(tasks) {
		t.Fatalf("output lines = %#v, want one line per task", lines)
	}
	seen := make(map[string]bool, len(lines))
	for _, line := range lines {
		seen[line] = true
	}
	for _, task := range tasks {
		if !seen[task.pdf] {
			t.Fatalf("output missing task %q: %#v", task.pdf, lines)
		}
	}
}

func TestCompareTaskMemoryChargesDecodedFileExpansion(t *testing.T) {
	path := t.TempDir() + "/large.pdf"
	const size = int64(20 << 20)
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(path, size); err != nil {
		t.Fatal(err)
	}
	want := compareTaskBaseMemory + compareTaskFileFactor*size
	if got := compareTaskMemory(path); got != want {
		t.Fatalf("compareTaskMemory() = %d, want %d", got, want)
	}
}

func TestCompareTaskGroupMemoryUsesMeasuredSectionClass(t *testing.T) {
	path := t.TempDir() + "/text.pdf"
	const size = int64(40 << 20)
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(path, size); err != nil {
		t.Fatal(err)
	}
	full := compareTaskMemory(path)
	glyphs := compareTaskGroupMemory(path, []string{"content.glyphs"})
	if glyphs >= full {
		t.Fatalf("glyph group memory = %d MiB, want below full-task estimate %d MiB", glyphs>>20, full>>20)
	}
	if heavy := compareTaskGroupMemory(path, []string{"content.streams", "content.tokens", "content.contents"}); heavy < glyphs {
		t.Fatalf("stream/token group memory = %d MiB, want >= glyph group %d MiB", heavy>>20, glyphs>>20)
	}
}

func TestBuildCompatCompareTasksExpandsLargePDFSectionGroups(t *testing.T) {
	path := t.TempDir() + "/large.pdf"
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(path, compatibilityLargePDFBytes); err != nil {
		t.Fatal(err)
	}
	base := []compareTask{
		{pdf: path, space: "page", memoryBytes: 1234},
		{pdf: path, space: "screen", memoryBytes: 1234},
	}
	tasks := buildCompatCompareTasks(base, requiredSections)
	want := len(base) * len(compatibilitySectionGroups(path, requiredSections))
	if len(tasks) != want {
		t.Fatalf("cache tasks = %d, want %d", len(tasks), want)
	}
	for _, task := range tasks {
		if len(task.sections) == 0 || task.memoryBytes != 1234 {
			t.Fatalf("cache task = %#v", task)
		}
	}
}

func TestBuildCompatCompareTasksUsesTheSameLargePDFGroupsAsCachePreflight(t *testing.T) {
	path := t.TempDir() + "/large.pdf"
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(path, compatibilityLargePDFBytes); err != nil {
		t.Fatal(err)
	}
	base := []compareTask{{pdf: path, space: "page", memoryBytes: 1234}}
	tasks := buildCompatCompareTasks(base, requiredSections)
	wantGroups := compatibilitySectionGroups(path, requiredSections)
	if len(tasks) != len(wantGroups) {
		t.Fatalf("compare tasks = %d, want %d section groups", len(tasks), len(wantGroups))
	}
	for index, task := range tasks {
		if !reflect.DeepEqual(task.sections, wantGroups[index]) {
			t.Fatalf("compare task %d sections = %#v, want %#v", index, task.sections, wantGroups[index])
		}
	}
}

func TestRunCompareTasksSerializesWeightHeavyTasks(t *testing.T) {
	tasks := []compareTask{{pdf: "heavy-a", memoryBytes: 2 << 30}, {pdf: "heavy-b", memoryBytes: 2 << 30}}
	var mu sync.Mutex
	active := 0
	var overlapped bool
	var output strings.Builder
	err := runCompareTasks(tasks, 2, func(task compareTask, result io.Writer) error {
		mu.Lock()
		active++
		if active > 1 {
			overlapped = true
		}
		mu.Unlock()
		time.Sleep(time.Millisecond)
		_, writeErr := io.WriteString(result, task.pdf+"\n")
		mu.Lock()
		active--
		mu.Unlock()
		return writeErr
	}, &output)
	if err != nil {
		t.Fatal(err)
	}
	if overlapped {
		t.Fatal("weight-heavy compatibility tasks overlapped")
	}
}

func TestRunCompareTasksAllowsHeavyTasksWithinIndependentMemoryBudget(t *testing.T) {
	tasks := []compareTask{{pdf: "heavy-a", memoryBytes: 1 << 30}, {pdf: "heavy-b", memoryBytes: 1 << 30}}
	var mu sync.Mutex
	active := 0
	maximum := 0
	var output strings.Builder
	err := runCompareTasks(tasks, 4, func(task compareTask, result io.Writer) error {
		mu.Lock()
		active++
		if active > maximum {
			maximum = active
		}
		mu.Unlock()
		time.Sleep(10 * time.Millisecond)
		_, writeErr := io.WriteString(result, task.pdf+"\n")
		mu.Lock()
		active--
		mu.Unlock()
		return writeErr
	}, &output)
	if err != nil {
		t.Fatal(err)
	}
	if maximum < 2 {
		t.Fatalf("maximum concurrent jobs = %d, want 2 when memory budget is independent of worker count", maximum)
	}
}

func TestRunCompareTasksRespectsExplicitMemoryLimit(t *testing.T) {
	tasks := []compareTask{{pdf: "a", memoryBytes: 700 << 20}, {pdf: "b", memoryBytes: 700 << 20}}
	var mu sync.Mutex
	active := 0
	maximum := 0
	var output strings.Builder
	err := runCompareTasksWithMemoryLimit(tasks, 4, 1<<30, func(task compareTask, result io.Writer) error {
		mu.Lock()
		active++
		if active > maximum {
			maximum = active
		}
		mu.Unlock()
		time.Sleep(time.Millisecond)
		_, writeErr := io.WriteString(result, task.pdf+"\n")
		mu.Lock()
		active--
		mu.Unlock()
		return writeErr
	}, &output)
	if err != nil {
		t.Fatal(err)
	}
	if maximum != 1 {
		t.Fatalf("maximum concurrent jobs = %d, want 1 under explicit memory limit", maximum)
	}
}

func TestRunCompareTasksRejectsTaskOverMemoryLimit(t *testing.T) {
	var output strings.Builder
	err := runCompareTasksWithMemoryLimit([]compareTask{{pdf: "oversized", memoryBytes: 2 << 30}}, 2, 1<<30, func(compareTask, io.Writer) error {
		t.Fatal("oversized task ran")
		return nil
	}, &output)
	if err == nil || !strings.Contains(err.Error(), "exceeds compatibility memory limit") {
		t.Fatalf("oversized task error = %v", err)
	}
}

func TestAutomaticCompareMemoryLimitPreservesSystemHeadroom(t *testing.T) {
	const total = int64(16 << 30)
	want := int64(9792 << 20)
	if got := automaticCompareMemoryLimit(total); got != want {
		t.Fatalf("automaticCompareMemoryLimit(16 GiB) = %d MiB, want %d MiB", got>>20, want>>20)
	}
}

func TestAutomaticBudgetCanFillTenWorkersForMeasuredTextGroups(t *testing.T) {
	const textFixtureBytes = int64(40 << 20)
	taskMemory := compareTaskBaseMemory + compareTaskFileFactor*textFixtureBytes
	tasks := make([]compareTask, 10)
	for index := range tasks {
		tasks[index] = compareTask{pdf: string(rune('a' + index)), memoryBytes: taskMemory}
	}
	var mu sync.Mutex
	active, maximum := 0, 0
	var output strings.Builder
	err := runCompareTasksWithMemoryLimit(tasks, 10, automaticCompareMemoryLimit(16<<30), func(task compareTask, result io.Writer) error {
		mu.Lock()
		active++
		if active > maximum {
			maximum = active
		}
		mu.Unlock()
		time.Sleep(20 * time.Millisecond)
		_, writeErr := io.WriteString(result, task.pdf+"\n")
		mu.Lock()
		active--
		mu.Unlock()
		return writeErr
	}, &output)
	if err != nil {
		t.Fatal(err)
	}
	if maximum != 10 {
		t.Fatalf("maximum concurrent measured text groups = %d, want 10", maximum)
	}
}

func TestResolveCompareWorkersUsesAllAvailableGoCPUsByDefault(t *testing.T) {
	if got := resolveCompareWorkers(0); got != runtime.GOMAXPROCS(0) {
		t.Fatalf("resolveCompareWorkers(0) = %d, want GOMAXPROCS %d", got, runtime.GOMAXPROCS(0))
	}
	if got := resolveCompareWorkers(3); got != 3 {
		t.Fatalf("resolveCompareWorkers(3) = %d, want explicit value", got)
	}
}

func TestDynamicCompareMemoryCapacityCountsResidentWorkWithoutCrossingReserve(t *testing.T) {
	const limit = int64(10 << 30)
	const available = int64(6 << 30)
	const resident = int64(1 << 30)
	want := int64(5 << 30)
	if got := dynamicCompareMemoryCapacity(limit, available, resident, compareMemoryReserve); got != want {
		t.Fatalf("dynamicCompareMemoryCapacity() = %d MiB, want %d MiB", got>>20, want>>20)
	}
}

func TestPhysicalMemoryBytesOnSupportedSystems(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("physical memory detection uses the fallback on this platform")
	}
	if got := physicalMemoryBytes(); got <= 0 {
		t.Fatalf("physicalMemoryBytes() = %d on %s", got, runtime.GOOS)
	}
}

func TestAvailableMemoryBytesOnSupportedSystems(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("available memory detection uses the fallback on this platform")
	}
	available := availableMemoryBytes()
	if available <= 0 {
		t.Fatalf("availableMemoryBytes() = %d on %s", available, runtime.GOOS)
	}
	if total := physicalMemoryBytes(); total > 0 && available > total {
		t.Fatalf("availableMemoryBytes() = %d exceeds physicalMemoryBytes() = %d", available, total)
	}
}

func TestCurrentRuntimeCommittedBytesIsPositive(t *testing.T) {
	if committed := currentRuntimeCommittedBytes(); committed <= 0 {
		t.Fatalf("currentRuntimeCommittedBytes() = %d", committed)
	}
}

func TestRunCompareTasksPausesDispatchWhenSystemHeadroomShrinks(t *testing.T) {
	tasks := []compareTask{{pdf: "a", memoryBytes: 400 << 20}, {pdf: "b", memoryBytes: 400 << 20}}
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	var probeMu sync.Mutex
	probes := 0
	available := func() int64 {
		probeMu.Lock()
		defer probeMu.Unlock()
		probes++
		if probes == 2 {
			return 500 << 20
		}
		return 2 << 30
	}
	var runMu sync.Mutex
	active, maximum := 0, 0
	done := make(chan error, 1)
	go func() {
		done <- runCompareTasksWithResourceProbe(tasks, 2, 1<<30, 256<<20, available, nil, func(task compareTask, result io.Writer) error {
			runMu.Lock()
			active++
			if active > maximum {
				maximum = active
			}
			runMu.Unlock()
			if task.pdf == "a" {
				close(firstStarted)
				<-releaseFirst
			}
			_, err := io.WriteString(result, task.pdf+"\n")
			runMu.Lock()
			active--
			runMu.Unlock()
			return err
		}, io.Discard)
	}()
	select {
	case <-firstStarted:
		close(releaseFirst)
	case <-time.After(time.Second):
		t.Fatal("first task did not start")
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if maximum != 1 {
		t.Fatalf("maximum concurrent jobs = %d, want 1 after available memory shrank", maximum)
	}
}

func TestRunCompareTasksSchedulesFittingTaskPastBlockedHeavyTask(t *testing.T) {
	releaseFirst := make(chan struct{})
	lightStarted := make(chan struct{})
	done := make(chan error, 1)
	var output strings.Builder
	go func() {
		done <- runCompareTasksWithMemoryLimit([]compareTask{
			{pdf: "first", memoryBytes: 700 << 20},
			{pdf: "blocked", memoryBytes: 700 << 20},
			{pdf: "light", memoryBytes: 300 << 20},
		}, 2, 1<<30, func(task compareTask, result io.Writer) error {
			if task.pdf == "first" {
				<-releaseFirst
			}
			if task.pdf == "light" {
				close(lightStarted)
			}
			_, err := io.WriteString(result, task.pdf+"\n")
			return err
		}, &output)
	}()
	select {
	case <-lightStarted:
		close(releaseFirst)
	case <-time.After(time.Second):
		close(releaseFirst)
		t.Fatal("fitting task was blocked behind a task that exceeded the remaining memory budget")
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestComparePhaseGateAllowsSamePhaseAndExcludesOppositePhase(t *testing.T) {
	gate := newComparePhaseGate()
	gate.acquire(comparePhaseGo)

	sameEntered := make(chan struct{})
	go func() {
		gate.acquire(comparePhaseGo)
		close(sameEntered)
		gate.release(comparePhaseGo)
	}()
	select {
	case <-sameEntered:
	case <-time.After(time.Second):
		t.Fatal("same-phase work did not overlap")
	}

	oppositeEntered := make(chan struct{})
	go func() {
		gate.acquire(comparePhaseOracle)
		close(oppositeEntered)
		gate.release(comparePhaseOracle)
	}()
	select {
	case <-oppositeEntered:
		t.Fatal("opposite phases overlapped")
	case <-time.After(10 * time.Millisecond):
	}
	gate.release(comparePhaseGo)
	select {
	case <-oppositeEntered:
	case <-time.After(time.Second):
		t.Fatal("opposite phase did not start after active phase drained")
	}
}

func TestRunCompareTasksUsesFileBackedTaskOutput(t *testing.T) {
	var output strings.Builder
	err := runCompareTasks([]compareTask{{pdf: "file-backed"}}, 1, func(_ compareTask, result io.Writer) error {
		if _, ok := result.(*os.File); !ok {
			t.Fatalf("task output type = %T, want *os.File", result)
		}
		return nil
	}, &output)
	if err != nil {
		t.Fatal(err)
	}
}

type signalWriter struct {
	mu       sync.Mutex
	data     strings.Builder
	observed chan struct{}
	once     sync.Once
}

func (w *signalWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	_, _ = w.data.Write(p)
	if strings.Contains(w.data.String(), "short\n") {
		w.once.Do(func() { close(w.observed) })
	}
	return len(p), nil
}

func TestRunCompareTasksEmitsCompletedTasksImmediately(t *testing.T) {
	allowLong := make(chan struct{})
	writer := &signalWriter{observed: make(chan struct{})}
	done := make(chan error, 1)
	go func() {
		done <- runCompareTasks([]compareTask{{pdf: "long"}, {pdf: "short"}}, 2, func(task compareTask, result io.Writer) error {
			if task.pdf == "long" {
				<-allowLong
			}
			_, err := io.WriteString(result, task.pdf+"\n")
			return err
		}, writer)
	}()
	select {
	case <-writer.observed:
		close(allowLong)
	case <-time.After(time.Second):
		close(allowLong)
		t.Fatal("short task output was delayed until long task completed")
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

type panicOutputWriter struct{}

func (panicOutputWriter) Write([]byte) (int, error) { panic("output failure") }

func TestRunCompareTasksRecoversOutputWorkerPanic(t *testing.T) {
	err := runCompareTasks([]compareTask{{pdf: "output-panic.pdf"}}, 1, func(_ compareTask, result io.Writer) error {
		_, err := io.WriteString(result, "result\n")
		return err
	}, panicOutputWriter{})
	if err == nil || !strings.Contains(err.Error(), "compatibility output worker panic") {
		t.Fatalf("output worker panic = %v", err)
	}
}

func TestRunCompareTasksPropagatesWorkerPanic(t *testing.T) {
	var output strings.Builder
	err := runCompareTasks([]compareTask{{pdf: "panic.pdf"}}, 1, func(compareTask, io.Writer) error {
		panic("test panic")
	}, &output)
	if err == nil || !strings.Contains(err.Error(), "compatibility worker panic") {
		t.Fatalf("panic error = %v", err)
	}
	if errors.Is(err, io.EOF) {
		t.Fatal("panic was replaced by unrelated EOF")
	}
}
