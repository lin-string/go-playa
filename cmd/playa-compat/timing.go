package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

type timingMode string

const (
	timingModeAuto   timingMode = "auto"
	timingModeOff    timingMode = "off"
	timingModeReport timingMode = "report"
	timingModeTrack  timingMode = "track"

	timingHistoryVersion  = 1
	timingMinimumSamples  = 3
	timingMaximumSamples  = 5
	timingRegressionRatio = 1.35
	timingRegressionDelta = 500 * time.Millisecond
	timingLockPoll        = 10 * time.Millisecond
	timingLockWait        = 10 * time.Second
)

func parseTimingMode(value string) (timingMode, error) {
	mode := timingMode(value)
	switch mode {
	case timingModeAuto, timingModeOff, timingModeReport, timingModeTrack:
		return mode, nil
	default:
		return "", fmt.Errorf("invalid timing mode %q; want auto, off, report, or track", value)
	}
}

func resolveTimingMode(mode timingMode, getenv func(string) string) timingMode {
	if mode != timingModeAuto {
		return mode
	}
	if getenv("CI") != "" || getenv("GITHUB_ACTIONS") != "" {
		return timingModeReport
	}
	return timingModeTrack
}

type timingSample struct {
	SourceCommit string `json:"source_commit"`
	ElapsedNS    int64  `json:"elapsed_ns"`
	Regressed    bool   `json:"regressed"`
}

type timingMachine struct {
	GOOS         string `json:"goos"`
	GOARCH       string `json:"goarch"`
	CPU          string `json:"cpu"`
	LogicalCPUs  int    `json:"logical_cpus"`
	MaxProcs     int    `json:"gomaxprocs"`
	GoVersion    string `json:"go_version"`
	OracleCommit string `json:"oracle_commit"`
	CacheVersion int    `json:"cache_version"`
	Workers      int    `json:"workers"`
	CachePolicy  string `json:"cache_policy"`
}

type timingHistory struct {
	Version   int                       `json:"version"`
	Machine   timingMachine             `json:"machine"`
	Workloads map[string][]timingSample `json:"workloads"`
}

type timingWorkload struct {
	PDFSHA256      string   `json:"pdf_sha256"`
	Pages          []int    `json:"pages"`
	Space          string   `json:"space"`
	Sections       []string `json:"sections"`
	PasswordSet    bool     `json:"password_set"`
	PasswordSHA256 string   `json:"password_sha256,omitempty"`
}

type timingResult struct {
	PDF              string   `json:"pdf"`
	PDFSHA256        string   `json:"pdf_sha256,omitempty"`
	Pages            []int    `json:"pages"`
	Space            string   `json:"space"`
	Sections         []string `json:"sections"`
	ElapsedNS        int64    `json:"elapsed_ns"`
	CacheHit         bool     `json:"cache_hit"`
	Mode             string   `json:"mode"`
	Eligible         bool     `json:"eligible"`
	BaselineMedianNS int64    `json:"baseline_median_ns,omitempty"`
	Ratio            float64  `json:"ratio,omitempty"`
	DeltaNS          int64    `json:"delta_ns,omitempty"`
	Regressed        bool     `json:"regressed"`
}

type timingRecorder struct {
	mu           sync.Mutex
	mode         timingMode
	cacheDir     string
	workers      int
	noCache      bool
	profiled     bool
	machine      timingMachine
	sourceCommit string
}

func newTimingRecorder(mode timingMode, cacheDir string, workers int, noCache, profiled bool) *timingRecorder {
	return &timingRecorder{
		mode: mode, cacheDir: cacheDir, workers: workers, noCache: noCache, profiled: profiled,
	}
}

func timingMedian(samples []time.Duration) time.Duration {
	if len(samples) == 0 {
		return 0
	}
	ordered := append([]time.Duration(nil), samples...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	middle := len(ordered) / 2
	if len(ordered)%2 != 0 {
		return ordered[middle]
	}
	return ordered[middle-1]/2 + ordered[middle]/2 + (ordered[middle-1]%2+ordered[middle]%2)/2
}

func timingRegressed(history []time.Duration, current time.Duration) bool {
	if len(history) < timingMinimumSamples {
		return false
	}
	median := timingMedian(history)
	return current-median > timingRegressionDelta && float64(current) > float64(median)*timingRegressionRatio
}

func retainTimingSamples(samples []timingSample) []timingSample {
	baselines := make([]timingSample, 0, timingMaximumSamples)
	for _, sample := range samples {
		if !sample.Regressed {
			baselines = append(baselines, sample)
		}
	}
	if len(baselines) > timingMaximumSamples {
		baselines = baselines[len(baselines)-timingMaximumSamples:]
	}
	retained := append([]timingSample(nil), baselines...)
	if len(samples) > 0 && samples[len(samples)-1].Regressed {
		retained = append(retained, samples[len(samples)-1])
	}
	return retained
}

func (r *timingRecorder) record(task compareTask, cacheHit bool, elapsed time.Duration) (timingResult, []string) {
	result := timingResult{
		PDF: task.pdf, Pages: append([]int(nil), task.pages...), Space: task.space,
		Sections: append([]string(nil), task.sections...), ElapsedNS: int64(elapsed),
		CacheHit: cacheHit, Mode: string(r.mode),
		Eligible: r.workers == 1 && cacheHit && !r.noCache && !r.profiled,
	}
	if result.Pages == nil {
		result.Pages = []int{}
	}
	digest := task.fixtureSHA256
	if digest == "" {
		var err error
		digest, err = fileDigest(task.pdf)
		if err != nil {
			result.Eligible = false
			return result, []string{fmt.Sprintf("digest %s: %v", task.pdf, err)}
		}
	}
	result.PDFSHA256 = digest
	if r.mode != timingModeTrack || !result.Eligible {
		return result, nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.machine.GOOS == "" {
		r.machine = detectTimingMachine(r.workers, r.noCache)
	}
	if r.sourceCommit == "" {
		r.sourceCommit = detectSourceCommit()
	}
	workload := timingWorkload{
		PDFSHA256: digest, Pages: result.Pages, Space: task.space,
		Sections: append([]string(nil), task.sections...), PasswordSet: task.passwordSet,
	}
	sort.Strings(workload.Sections)
	if task.passwordSet {
		hash := sha256.Sum256([]byte(task.password))
		workload.PasswordSHA256 = hex.EncodeToString(hash[:])
	}
	workloadKey := timingDigest(workload)
	historyPath := filepath.Join(r.cacheDir, "timings", timingDigest(r.machine)+".json")
	var warnings []string
	if err := withTimingHistoryLock(historyPath, func() error {
		history, usable, loadWarnings := loadTimingHistory(historyPath, r.machine)
		warnings = append(warnings, loadWarnings...)
		if !usable {
			return nil
		}
		prior := history.Workloads[workloadKey]
		baselineDurations := make([]time.Duration, 0, len(prior))
		for _, sample := range prior {
			if !sample.Regressed && sample.ElapsedNS >= 0 {
				baselineDurations = append(baselineDurations, time.Duration(sample.ElapsedNS))
			}
		}
		if len(baselineDurations) >= timingMinimumSamples {
			median := timingMedian(baselineDurations)
			result.BaselineMedianNS = int64(median)
			result.DeltaNS = int64(elapsed - median)
			if median > 0 {
				result.Ratio = float64(elapsed) / float64(median)
			}
			result.Regressed = timingRegressed(baselineDurations, elapsed)
		}
		history.Workloads[workloadKey] = retainTimingSamples(append(prior, timingSample{
			SourceCommit: r.sourceCommit, ElapsedNS: int64(elapsed), Regressed: result.Regressed,
		}))
		return writeTimingHistory(historyPath, history)
	}); err != nil {
		warnings = append(warnings, fmt.Sprintf("update %s: %v", historyPath, err))
	}
	return result, warnings
}

// withTimingHistoryLock serializes a history read-modify-write transaction
// across independent compatibility processes. An atomic directory lock keeps
// this portable. Locks are never stolen based on age: if a process crashes,
// later runs warn after a bounded wait and leave the history untouched until
// the abandoned lock is removed with the local timing cache.
func withTimingHistoryLock(historyPath string, update func() error) error {
	directory := filepath.Dir(historyPath)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	lockPath := historyPath + ".lock"
	deadline := time.Now().Add(timingLockWait)
	for {
		err := os.Mkdir(lockPath, 0o700)
		if err == nil {
			updateErr := update()
			removeErr := os.Remove(lockPath)
			if updateErr != nil {
				return updateErr
			}
			if removeErr != nil && !os.IsNotExist(removeErr) {
				return fmt.Errorf("release timing history lock %s: %w", lockPath, removeErr)
			}
			return nil
		}
		if !os.IsExist(err) {
			return fmt.Errorf("acquire timing history lock %s: %w", lockPath, err)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for timing history lock %s", lockPath)
		}
		time.Sleep(timingLockPoll)
	}
}

func timingDigest(value any) string {
	encoded, _ := json.Marshal(value)
	hash := sha256.Sum256(encoded)
	return hex.EncodeToString(hash[:])
}

func loadTimingHistory(path string, machine timingMachine) (timingHistory, bool, []string) {
	history := timingHistory{Version: timingHistoryVersion, Machine: machine, Workloads: map[string][]timingSample{}}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return history, true, nil
	}
	if err != nil {
		return history, false, []string{fmt.Sprintf("read %s: %v", path, err)}
	}
	var stored timingHistory
	if err := json.Unmarshal(data, &stored); err != nil {
		return history, false, []string{fmt.Sprintf("ignore corrupt timing history %s: %v", path, err)}
	}
	if stored.Version != timingHistoryVersion {
		return history, false, []string{fmt.Sprintf("ignore timing history %s with version %d", path, stored.Version)}
	}
	if timingDigest(stored.Machine) != timingDigest(machine) {
		return history, false, []string{fmt.Sprintf("ignore timing history %s for a different machine", path)}
	}
	if stored.Workloads == nil {
		stored.Workloads = map[string][]timingSample{}
	}
	return stored, true, nil
}

func writeTimingHistory(path string, history timingHistory) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	temporary, err := os.CreateTemp(filepath.Dir(path), ".timing-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	removeTemporary := true
	defer func() {
		_ = temporary.Close()
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()
	if _, err := temporary.Write(data); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	removeTemporary = false
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer func() { _ = directory.Close() }()
	return directory.Sync()
}

func detectTimingMachine(workers int, noCache bool) timingMachine {
	oracleCommit := "unknown"
	if path, err := repositoryFile("compat", "upstream.toml"); err == nil {
		if upstream, err := loadUpstreamConfig(path); err == nil {
			oracleCommit = upstream.Commit
		}
	}
	policy := "cached"
	if noCache {
		policy = "no-cache"
	}
	return timingMachine{
		GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, CPU: currentCPUIdentity(),
		LogicalCPUs: runtime.NumCPU(), MaxProcs: runtime.GOMAXPROCS(0), GoVersion: runtime.Version(), OracleCommit: oracleCommit,
		CacheVersion: cacheVersion, Workers: workers, CachePolicy: policy,
	}
}

func detectSourceCommit() string {
	if commit := strings.TrimSpace(os.Getenv("GITHUB_SHA")); commit != "" {
		return commit
	}
	rootFile, err := repositoryFile("go.mod")
	if err != nil {
		return "unknown"
	}
	command := exec.Command("git", "rev-parse", "HEAD")
	command.Dir = filepath.Dir(rootFile)
	output, err := command.Output()
	if err != nil {
		return "unknown"
	}
	commit := strings.TrimSpace(string(output))
	if commit == "" {
		return "unknown"
	}
	return commit
}
