package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestTimingModeParsingAndAutomaticCISelection(t *testing.T) {
	for _, value := range []string{"auto", "off", "report", "track"} {
		if mode, err := parseTimingMode(value); err != nil || string(mode) != value {
			t.Fatalf("parseTimingMode(%q) = %q, %v", value, mode, err)
		}
	}
	if _, err := parseTimingMode("invalid"); err == nil {
		t.Fatal("parseTimingMode accepted an invalid mode")
	}
	lookup := func(name string) string {
		if name == "CI" {
			return "true"
		}
		return ""
	}
	if got := resolveTimingMode(timingModeAuto, lookup); got != timingModeReport {
		t.Fatalf("automatic CI mode = %q, want report", got)
	}
	if got := resolveTimingMode(timingModeAuto, func(string) string { return "" }); got != timingModeTrack {
		t.Fatalf("automatic local mode = %q, want track", got)
	}
	if got := resolveTimingMode(timingModeOff, lookup); got != timingModeOff {
		t.Fatalf("explicit off mode = %q, want off", got)
	}
}

func TestRunRejectsInvalidTimingMode(t *testing.T) {
	if err := run([]string{"--timing-mode", "invalid"}, os.Stdout); err == nil || !isUsageError(err) {
		t.Fatalf("invalid timing mode error = %v, want usage error", err)
	}
}

func TestTimingRegressionRequiresRatioAndAbsoluteThreshold(t *testing.T) {
	history := []time.Duration{2 * time.Second, 2100 * time.Millisecond, 1900 * time.Millisecond}
	if timingRegressed(history, 2500*time.Millisecond) {
		t.Fatal("absolute-only increase was reported")
	}
	if !timingRegressed(history, 2800*time.Millisecond) {
		t.Fatal("combined ratio and absolute increase was missed")
	}
	if timingRegressed(history[:2], 4*time.Second) {
		t.Fatal("regression was reported before three baseline samples")
	}
}

func TestRetainTimingSamplesExcludesWarningsFromBaseline(t *testing.T) {
	var samples []timingSample
	for index := range 7 {
		samples = append(samples, timingSample{ElapsedNS: int64(index + 1)})
	}
	samples = append(samples, timingSample{ElapsedNS: 99, Regressed: true})
	retained := retainTimingSamples(samples)
	if len(retained) != 6 || retained[0].ElapsedNS != 3 || retained[4].ElapsedNS != 7 || retained[5].ElapsedNS != 99 || !retained[5].Regressed {
		t.Fatalf("retained samples = %#v, want five recent baselines plus current warning", retained)
	}
	next := retainTimingSamples(append(retained, timingSample{ElapsedNS: 8}))
	if len(next) != 5 || next[0].ElapsedNS != 4 || next[4].ElapsedNS != 8 {
		t.Fatalf("samples after recovery = %#v, want five non-regressed samples", next)
	}
}

func testTimingMachine() timingMachine {
	return timingMachine{
		GOOS: "test", GOARCH: "test", CPU: "test cpu", LogicalCPUs: 1,
		MaxProcs: 1, GoVersion: "go-test", OracleCommit: "oracle", CacheVersion: cacheVersion,
		Workers: 1, CachePolicy: "cached",
	}
}

func TestTimingMachineIdentityIncludesGOMAXPROCS(t *testing.T) {
	machine := detectTimingMachine(1, false)
	if machine.MaxProcs != runtime.GOMAXPROCS(0) {
		t.Fatalf("timing machine GOMAXPROCS = %d, want %d", machine.MaxProcs, runtime.GOMAXPROCS(0))
	}
}

func TestTimingHistoryLockSerializesIndependentWriters(t *testing.T) {
	historyPath := filepath.Join(t.TempDir(), "machine.json")
	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- withTimingHistoryLock(historyPath, func() error {
			close(firstEntered)
			<-releaseFirst
			return nil
		})
	}()
	<-firstEntered
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(historyPath+".lock", old, old); err != nil {
		t.Fatal(err)
	}

	secondEntered := make(chan struct{})
	secondDone := make(chan error, 1)
	go func() {
		secondDone <- withTimingHistoryLock(historyPath, func() error {
			close(secondEntered)
			return nil
		})
	}()
	select {
	case <-secondEntered:
		t.Fatal("second history writer entered while the first writer held the lock")
	case <-time.After(100 * time.Millisecond):
	}
	close(releaseFirst)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-secondDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("second history writer did not acquire the released lock")
	}
	if _, err := os.Stat(historyPath + ".lock"); !os.IsNotExist(err) {
		t.Fatalf("history lock remains after writers completed: %v", err)
	}
}

func testTimingTask() compareTask {
	return compareTask{
		pdf: "fixture.pdf", fixtureSHA256: strings.Repeat("a", 64), space: "page",
		pages: []int{0, 1}, sections: []string{"pages", "document"},
	}
}

func TestTimingRecorderTracksLocalBaselineAndExcludesRegression(t *testing.T) {
	cacheDir := t.TempDir()
	recorder := newTimingRecorder(timingModeTrack, cacheDir, 1, false, false)
	recorder.machine = testTimingMachine()
	recorder.sourceCommit = "source"
	task := testTimingTask()

	for _, elapsed := range []time.Duration{time.Second, 1100 * time.Millisecond, 900 * time.Millisecond} {
		result, warnings := recorder.record(task, true, elapsed)
		if result.Regressed || len(warnings) != 0 {
			t.Fatalf("baseline result = %#v, warnings=%v", result, warnings)
		}
	}
	regressed, warnings := recorder.record(task, true, 2*time.Second)
	if len(warnings) != 0 || !regressed.Regressed || regressed.BaselineMedianNS != int64(time.Second) {
		t.Fatalf("regressed result = %#v, warnings=%v", regressed, warnings)
	}
	if regressed.Ratio != 2 || regressed.DeltaNS != int64(time.Second) {
		t.Fatalf("regression comparison = ratio %.2f delta %d", regressed.Ratio, regressed.DeltaNS)
	}
	if result, warnings := recorder.record(task, true, 1050*time.Millisecond); result.Regressed || len(warnings) != 0 {
		t.Fatalf("post-warning result = %#v, warnings=%v", result, warnings)
	}

	historyPath := filepath.Join(cacheDir, "timings", timingDigest(recorder.machine)+".json")
	data, err := os.ReadFile(historyPath)
	if err != nil {
		t.Fatal(err)
	}
	var history timingHistory
	if err := json.Unmarshal(data, &history); err != nil {
		t.Fatal(err)
	}
	if history.Version != timingHistoryVersion || len(history.Workloads) != 1 {
		t.Fatalf("timing history = %#v", history)
	}
	for _, samples := range history.Workloads {
		if len(samples) != 4 {
			t.Fatalf("retained samples = %#v, want four non-regressed samples", samples)
		}
		for _, sample := range samples {
			if sample.Regressed {
				t.Fatalf("warned sample entered baseline: %#v", samples)
			}
		}
	}
	entries, err := os.ReadDir(filepath.Dir(historyPath))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".timing-") {
			t.Fatalf("atomic timing write left temporary file %q", entry.Name())
		}
	}
}

func TestTimingRecorderReportAndIneligibleModesDoNotTouchHistory(t *testing.T) {
	for _, test := range []struct {
		name     string
		mode     timingMode
		workers  int
		cacheHit bool
		noCache  bool
		profiled bool
		eligible bool
	}{
		{name: "report only", mode: timingModeReport, workers: 1, cacheHit: true, eligible: true},
		{name: "multiple workers", mode: timingModeTrack, workers: 2, cacheHit: true},
		{name: "cache miss", mode: timingModeTrack, workers: 1},
		{name: "cache disabled", mode: timingModeTrack, workers: 1, cacheHit: true, noCache: true},
		{name: "profiled", mode: timingModeTrack, workers: 1, cacheHit: true, profiled: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			cacheDir := filepath.Join(root, "absent")
			recorder := newTimingRecorder(test.mode, cacheDir, test.workers, test.noCache, test.profiled)
			result, warnings := recorder.record(testTimingTask(), test.cacheHit, time.Second)
			if len(warnings) != 0 || result.Eligible != test.eligible {
				t.Fatalf("result = %#v, warnings=%v", result, warnings)
			}
			if _, err := os.Stat(cacheDir); !os.IsNotExist(err) {
				t.Fatalf("history directory exists in non-tracking run: %v", err)
			}
		})
	}
}

func TestTimingRecorderIgnoresCorruptAndFutureHistory(t *testing.T) {
	for _, test := range []struct {
		name string
		data []byte
	}{
		{name: "corrupt", data: []byte("not json")},
		{name: "future", data: []byte(`{"version":999,"machine":{},"workloads":{}}`)},
	} {
		t.Run(test.name, func(t *testing.T) {
			cacheDir := t.TempDir()
			recorder := newTimingRecorder(timingModeTrack, cacheDir, 1, false, false)
			recorder.machine = testTimingMachine()
			recorder.sourceCommit = "source"
			path := filepath.Join(cacheDir, "timings", timingDigest(recorder.machine)+".json")
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, test.data, 0o600); err != nil {
				t.Fatal(err)
			}
			result, warnings := recorder.record(testTimingTask(), true, time.Second)
			if !result.Eligible || len(warnings) != 1 || !strings.Contains(warnings[0], "ignore") {
				t.Fatalf("result = %#v, warnings=%v", result, warnings)
			}
			stored, err := os.ReadFile(path)
			if err != nil || string(stored) != string(test.data) {
				t.Fatalf("ignored history was overwritten: %q, %v", stored, err)
			}
		})
	}
}
