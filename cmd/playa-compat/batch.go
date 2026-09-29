package main

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
)

type compareTask struct {
	pdf           string
	space         string
	pages         []int
	password      string
	passwordSet   bool
	memoryBytes   int64
	sections      []string
	fixtureID     string
	fixtureSHA256 string
}

type compareTaskResult struct {
	index int
	path  string
	err   error
}

const (
	defaultCompareMemoryLimit = int64(3 << 30)
	compareTaskBaseMemory     = int64(600 << 20)
	compareTaskFileFactor     = int64(7)
	compareLightBaseMemory    = int64(300 << 20)
	compareLayoutBaseMemory   = int64(500 << 20)
	compareXObjectBaseMemory  = int64(400 << 20)
	compareHeavyBaseMemory    = int64(700 << 20)
	compareLightFileFactor    = int64(3)
	compareMediumFileFactor   = int64(4)
	compareMemoryReserve      = int64(2 << 30)
	compareMemoryQuantum      = int64(64 << 20)
)

type comparePhase uint8

const (
	comparePhaseIdle comparePhase = iota
	comparePhaseGo
	comparePhaseOracle
)

type comparePhaseGate struct {
	cond      *sync.Cond
	mode      comparePhase
	preferred comparePhase
	active    int
	waiting   [3]int
}

func newComparePhaseGate() *comparePhaseGate {
	return &comparePhaseGate{cond: sync.NewCond(&sync.Mutex{})}
}

func otherComparePhase(phase comparePhase) comparePhase {
	if phase == comparePhaseGo {
		return comparePhaseOracle
	}
	return comparePhaseGo
}

func (gate *comparePhaseGate) acquire(phase comparePhase) {
	gate.cond.L.Lock()
	defer gate.cond.L.Unlock()
	gate.waiting[phase]++
	for {
		if gate.active == 0 {
			if gate.preferred == comparePhaseIdle || gate.preferred == phase || gate.waiting[gate.preferred] == 0 {
				break
			}
		} else if gate.mode == phase && gate.waiting[otherComparePhase(phase)] == 0 {
			break
		}
		gate.cond.Wait()
	}
	gate.waiting[phase]--
	if gate.active == 0 {
		gate.mode = phase
		gate.preferred = comparePhaseIdle
	}
	gate.active++
}

func (gate *comparePhaseGate) release(phase comparePhase) {
	gate.cond.L.Lock()
	defer gate.cond.L.Unlock()
	if gate.active == 0 || gate.mode != phase {
		panic("playa-compat: release inactive comparison phase")
	}
	gate.active--
	if gate.active != 0 {
		return
	}
	other := otherComparePhase(phase)
	if gate.waiting[other] > 0 {
		gate.preferred = other
	} else if gate.waiting[phase] > 0 {
		gate.preferred = phase
	} else {
		gate.preferred = comparePhaseIdle
	}
	gate.mode = comparePhaseIdle
	gate.cond.Broadcast()
}

var compatibilityProcessPhaseGate = newComparePhaseGate()

func compareTaskMemory(pdf string) int64 {
	return compareTaskMemoryEstimate(pdf, compareTaskBaseMemory, compareTaskFileFactor)
}

func compareTaskGroupMemory(pdf string, sections []string) int64 {
	if len(sections) == 1 {
		switch sections[0] {
		case "content.glyphs", "content.paths", "content.images":
			return compareTaskMemoryEstimate(pdf, compareLightBaseMemory, compareMediumFileFactor)
		case "layout":
			return compareTaskMemoryEstimate(pdf, compareLayoutBaseMemory, compareLightFileFactor)
		case "content.xobjects":
			return compareTaskMemoryEstimate(pdf, compareXObjectBaseMemory, compareMediumFileFactor)
		case "content.flatten", "content.interp":
			return compareTaskMemory(pdf)
		}
	}
	if containsString(sections, "content.streams") || containsString(sections, "document.objects") {
		return compareTaskMemoryEstimate(pdf, compareHeavyBaseMemory, compareTaskFileFactor)
	}
	if containsString(sections, "pages") || containsString(sections, "content.text") || containsString(sections, "annotations") {
		return compareTaskMemoryEstimate(pdf, compareLightBaseMemory, compareLightFileFactor)
	}
	return compareTaskMemory(pdf)
}

func compareTaskMemoryEstimate(pdf string, base, factor int64) int64 {
	info, err := os.Stat(pdf)
	if err != nil || info.Size() <= 0 {
		return base
	}
	// A comparison holds the Go document and one section-group projection while
	// it streams the oracle snapshot. Full-corpus measurements show about
	// 600 MiB of fixed allowance is conservative for text groups. Image-heavy
	// files scale much faster than their raw byte length, so charge seven times
	// the input size for decoded streams and temporary projections. Live system
	// headroom remains the final dispatch guard; this is a scheduling estimate,
	// not an allocation limit.
	if factor <= 0 || info.Size() > (int64(^uint64(0)>>1)-base)/factor {
		return int64(^uint64(0) >> 1)
	}
	return base + factor*info.Size()
}

func buildCompatCompareTasks(tasks []compareTask, selected []string) []compareTask {
	groupTasks := make([]compareTask, 0, len(tasks))
	for _, task := range tasks {
		taskSections := selected
		if len(task.sections) > 0 {
			taskSections = task.sections
		}
		for _, group := range compatibilitySectionGroups(task.pdf, taskSections) {
			groupTask := task
			groupTask.sections = append([]string(nil), group...)
			if groupTask.memoryBytes <= 0 {
				groupTask.memoryBytes = compareTaskGroupMemory(groupTask.pdf, group)
			}
			groupTasks = append(groupTasks, groupTask)
		}
	}
	return groupTasks
}

func compareTaskBudgetBytes(task compareTask) int64 {
	bytes := task.memoryBytes
	if bytes < 1 {
		bytes = compareTaskBaseMemory
	}
	return bytes
}

func automaticCompareMemoryLimit(total int64) int64 {
	limit := defaultCompareMemoryLimit
	if total > 0 {
		limit = (total/5)*3 + (total%5)*3/5
		if reserved := total - compareMemoryReserve; reserved < limit {
			limit = reserved
		}
	}
	if limit < compareMemoryQuantum {
		limit = compareMemoryQuantum
	}
	return limit / compareMemoryQuantum * compareMemoryQuantum
}

func dynamicCompareMemoryCapacity(limit, available, resident, reserve int64) int64 {
	if available <= 0 {
		return limit
	}
	capacity := available - reserve
	if capacity < 0 {
		capacity = 0
	}
	if resident > 0 {
		if resident > int64(^uint64(0)>>1)-capacity {
			capacity = int64(^uint64(0) >> 1)
		} else {
			capacity += resident
		}
	}
	if capacity > limit {
		return limit
	}
	return capacity
}

func currentRuntimeCommittedBytes() int64 {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	committed := stats.Sys
	if stats.HeapReleased < committed {
		committed -= stats.HeapReleased
	} else {
		committed = 0
	}
	if committed > uint64(^uint64(0)>>1) {
		return int64(^uint64(0) >> 1)
	}
	return int64(committed)
}

func resolveCompareWorkers(workers int) int {
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	if workers < 1 {
		return 1
	}
	return workers
}

// runCompareTasks runs independent compatibility jobs with bounded goroutine
// parallelism. Completed tasks send only temporary-file metadata through the
// result channel; one output worker copies each file as soon as it arrives and
// removes it, so task output is contiguous without output locks or in-memory
// result buffers.
func runCompareTasks(tasks []compareTask, workers int, run func(compareTask, io.Writer) error, output io.Writer) error {
	return runCompareTasksWithMemoryLimit(tasks, workers, defaultCompareMemoryLimit, run, output)
}

func runCompareTasksWithMemoryLimit(tasks []compareTask, workers int, memoryLimit int64, run func(compareTask, io.Writer) error, output io.Writer) error {
	return runCompareTasksWithResourceProbe(tasks, workers, memoryLimit, 0, nil, nil, run, output)
}

// runCompareTasksWithResourceProbe applies the configured aggregate task
// budget and, when available is non-nil, checks current system headroom before
// every dispatch. A disappearing probe is treated as unknown and leaves the
// configured budget in force; a positive probe never permits crossing reserve.
func runCompareTasksWithResourceProbe(tasks []compareTask, workers int, memoryLimit, reserve int64, available, resident func() int64, run func(compareTask, io.Writer) error, output io.Writer) error {
	if len(tasks) == 0 {
		return nil
	}
	workers = resolveCompareWorkers(workers)
	if workers > len(tasks) {
		workers = len(tasks)
	}
	if memoryLimit <= 0 {
		memoryLimit = defaultCompareMemoryLimit
	}
	for _, task := range tasks {
		memoryBytes := compareTaskBudgetBytes(task)
		if memoryBytes > memoryLimit {
			return fmt.Errorf("%s: estimated memory %d MiB exceeds compatibility memory limit %d MiB", task.pdf, (memoryBytes+(1<<20)-1)>>20, memoryLimit>>20)
		}
	}
	errors := make([]error, len(tasks))
	jobs := make(chan int)
	completed := make(chan int, workers)
	// One result is published per task, so this batch-sized metadata channel
	// never backpressures a task worker even when stdout is temporarily slow.
	results := make(chan compareTaskResult, len(tasks))
	var wg sync.WaitGroup
	var outputWG sync.WaitGroup
	var outputWorkerErr error
	outputWG.Add(1)
	go func() {
		defer outputWG.Done()
		defer func() {
			if recovered := recover(); recovered != nil {
				outputWorkerErr = fmt.Errorf("compatibility output worker panic: %v", recovered)
			}
		}()
		for result := range results {
			func() {
				var outputErr error
				defer func() {
					if recovered := recover(); recovered != nil {
						outputErr = fmt.Errorf("compatibility output worker panic: %v", recovered)
					}
					if outputErr != nil {
						errors[result.index] = outputErr
					}
					if result.path != "" {
						_ = os.Remove(result.path)
					}
				}()
				if result.err != nil {
					outputErr = result.err
					return
				}
				file, openErr := os.Open(result.path)
				if openErr != nil {
					outputErr = fmt.Errorf("open compatibility task output: %w", openErr)
					return
				}
				_, copyErr := io.Copy(output, file)
				closeErr := file.Close()
				if copyErr != nil {
					outputErr = fmt.Errorf("write compatibility results: %w", copyErr)
				} else if closeErr != nil {
					outputErr = fmt.Errorf("close compatibility task output: %w", closeErr)
				}
			}()
		}
	}()
	worker := func() {
		defer wg.Done()
		for index := range jobs {
			func() {
				defer func() { completed <- index }()
				file, err := os.CreateTemp("", "go-playa-compat-task-*.out")
				if err != nil {
					results <- compareTaskResult{index: index, err: fmt.Errorf("create compatibility task output: %w", err)}
					return
				}
				path := file.Name()
				var taskErr error
				defer func() {
					if recovered := recover(); recovered != nil {
						taskErr = fmt.Errorf("compatibility worker panic: %v", recovered)
					}
					if closeErr := file.Close(); closeErr != nil && taskErr == nil {
						taskErr = fmt.Errorf("close compatibility task output: %w", closeErr)
					}
					results <- compareTaskResult{index: index, path: path, err: taskErr}
				}()
				taskErr = run(tasks[index], file)
			}()
		}
	}
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go worker()
	}
	pending := make([]int, len(tasks))
	for index := range tasks {
		pending[index] = index
	}
	running := 0
	usedMemory := int64(0)
	var scheduleErr error
	for len(pending) > 0 || running > 0 {
		candidatePosition := -1
		candidateIndex := -1
		candidateMemory := int64(0)
		dynamicCapacity := memoryLimit
		observedAvailable := int64(0)
		if running < workers && len(pending) > 0 {
			if available != nil {
				observedAvailable = available()
				observedResident := int64(0)
				if resident != nil {
					observedResident = resident()
				}
				dynamicCapacity = dynamicCompareMemoryCapacity(memoryLimit, observedAvailable, observedResident, reserve)
			}
			for position, index := range pending {
				memoryBytes := compareTaskBudgetBytes(tasks[index])
				if memoryBytes <= dynamicCapacity-usedMemory {
					candidatePosition = position
					candidateIndex = index
					candidateMemory = memoryBytes
					break
				}
			}
		}
		if candidatePosition >= 0 {
			select {
			case jobs <- candidateIndex:
				pending = append(pending[:candidatePosition], pending[candidatePosition+1:]...)
				usedMemory += candidateMemory
				running++
			case index := <-completed:
				usedMemory -= compareTaskBudgetBytes(tasks[index])
				running--
			}
			continue
		}
		if running == 0 {
			required := int64(^uint64(0) >> 1)
			for _, index := range pending {
				if memoryBytes := compareTaskBudgetBytes(tasks[index]); memoryBytes < required {
					required = memoryBytes
				}
			}
			scheduleErr = fmt.Errorf("available system memory %d MiB cannot safely start the smallest pending compatibility task (%d MiB required with %d MiB reserved)", observedAvailable>>20, (required+(1<<20)-1)>>20, reserve>>20)
			break
		}
		index := <-completed
		usedMemory -= compareTaskBudgetBytes(tasks[index])
		running--
	}
	close(jobs)
	wg.Wait()
	close(results)
	outputWG.Wait()
	if outputWorkerErr != nil {
		return outputWorkerErr
	}
	if scheduleErr != nil {
		return scheduleErr
	}

	failed := make([]string, 0)
	for index, err := range errors {
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", tasks[index].pdf, err))
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("%d compatibility jobs failed: %s", len(failed), strings.Join(failed, "; "))
	}
	return nil
}
