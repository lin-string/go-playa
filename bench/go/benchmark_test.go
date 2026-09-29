package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/lin-string/go-playa/internal/testfixture"
)

func TestBenchmarkCLIDefaultJSONL(t *testing.T) {
	path := testfixture.Path(t, "form_simple.pdf")
	command := exec.Command(os.Args[0], "-test.run=^TestBenchmarkCLIHelper$")
	command.Env = append(os.Environ(), "GO_PLAYA_SAMPLE_HELPER="+path, "GOMAXPROCS=8")
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(output), &result); err != nil {
		t.Fatalf("sample stdout must be JSON: %q: %v", output, err)
	}
	if result["requested_workers"] != float64(1) || result["task"] != "text-glyph-jsonl" {
		t.Fatalf("default sample = %v", result)
	}
}

func TestBenchmarkCLIHelper(t *testing.T) {
	path := os.Getenv("GO_PLAYA_SAMPLE_HELPER")
	if path == "" {
		return
	}
	flag.CommandLine = flag.NewFlagSet("benchmark", flag.ExitOnError)
	os.Args = []string{"benchmark", path}
	main()
	os.Exit(0)
}

func TestFinishBenchmarkSampleIncludesCloseAndPropagatesErrors(t *testing.T) {
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	const delay = 10 * time.Millisecond
	result := benchmarkResult{}
	closed := false
	if err := finishBenchmarkSample(&result, before, start, func() error {
		time.Sleep(delay)
		closed = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !closed || result.ElapsedNS < delay.Nanoseconds() {
		t.Fatalf("close excluded from elapsed: closed=%v elapsed=%d", closed, result.ElapsedNS)
	}
	wantErr := fmt.Errorf("close failed")
	if err := finishBenchmarkSample(&result, before, start, func() error { return wantErr }); err != wantErr {
		t.Fatalf("close error = %v, want %v", err, wantErr)
	}
}

func TestBenchmarkResultsDistinguishWorkerTopology(t *testing.T) {
	path := testfixture.Path(t, "form_simple.pdf")
	for _, task := range []string{"open-pages", "objects", "text-glyph-jsonl"} {
		t.Run(task, func(t *testing.T) {
			var result benchmarkResult
			var err error
			if task == "text-glyph-jsonl" {
				result, err = runTextGlyphJSONL(path, 4)
			} else {
				result, err = runCountTask(path, task, 4)
			}
			if err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			if err := writeBenchmarkJSONL(&output, result); err != nil {
				t.Fatal(err)
			}
			var record map[string]any
			if err := json.Unmarshal(output.Bytes(), &record); err != nil {
				t.Fatal(err)
			}
			for key, want := range map[string]float64{"requested_workers": 4, "effective_workers": 1, "observed_workers": 1} {
				if got := record[key]; got != want {
					t.Errorf("%s = %v, want %v", key, got, want)
				}
			}
			if result.Pages != 1 {
				t.Errorf("pages = %d, want 1, including for objects", result.Pages)
			}
		})
	}
}

func TestCountTaskJSONOmitsUnavailableDigest(t *testing.T) {
	result, err := runCountTask(testfixture.Path(t, "form_simple.pdf"), "open-pages", 1)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := writeBenchmarkJSONL(&output, result); err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if digest, present := record["sha256"]; present {
		t.Fatalf("count-only JSON must omit unavailable digest, got %v", digest)
	}
}

func TestImageDigestCanonicalFrameAndWorkerStability(t *testing.T) {
	path := testfixture.Path(t, "acceptance_rgb_image.pdf")
	doc, err := openBenchmarkPDF(path)
	if err != nil {
		t.Fatal(err)
	}
	want := sha256.New()
	_, _ = want.Write([]byte("playa-image-digests-v1\x00"))
	for page, err := range doc.Pages() {
		if err != nil {
			t.Fatal(err)
		}
		imageIndex := 0
		for image, err := range page.Images(doc) {
			if err != nil {
				t.Fatal(err)
			}
			length, digest, err := image.DecodedStreamBufferDigestWithError()
			if err != nil {
				t.Fatal(err)
			}
			raw, err := hex.DecodeString(digest)
			if err != nil {
				t.Fatal(err)
			}
			var frame [56]byte
			binary.BigEndian.PutUint64(frame[:8], uint64(page.Number()-1))
			binary.BigEndian.PutUint64(frame[8:16], uint64(imageIndex))
			binary.BigEndian.PutUint64(frame[16:24], uint64(length))
			copy(frame[24:], raw)
			_, _ = want.Write(frame[:])
			imageIndex++
		}
	}
	if err := doc.Close(); err != nil {
		t.Fatal(err)
	}
	for _, workers := range []int{1, 2, 4} {
		result, err := runCountTask(path, "image-digests", workers)
		if err != nil {
			t.Fatal(err)
		}
		if result.SHA256 != hex.EncodeToString(want.Sum(nil)) {
			t.Fatalf("workers=%d digest=%s want=%x", workers, result.SHA256, want.Sum(nil))
		}
	}
}

func TestCountTaskJSONIncludesExplicitZeroCounts(t *testing.T) {
	var output bytes.Buffer
	if err := writeBenchmarkJSONL(&output, benchmarkResult{}); err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"pages", "texts", "glyphs", "objects", "layout_lines", "layout_items", "images"} {
		if value, ok := record[field]; !ok || value != float64(0) {
			t.Errorf("%s zero count missing: %v", field, record)
		}
	}
}

func benchmarkPDF(b testing.TB) string {
	b.Helper()
	path := os.Getenv("GO_PLAYA_BENCH_PDF")
	if path == "" {
		path = testfixture.Path(b, "riscv-unprivileged.pdf")
	}
	if _, err := os.Stat(path); err != nil {
		b.Fatalf("benchmark PDF %q: %v", path, err)
	}
	return path
}

func TestStreamingPageDigest(t *testing.T) {
	path := testfixture.Path(t, "form_simple.pdf")
	sequential, err := runTextGlyphJSONL(path, 1)
	if err != nil {
		t.Fatal(err)
	}
	concurrent, err := runTextGlyphJSONL(path, 4)
	if err != nil {
		t.Fatal(err)
	}
	if sequential.Pages == 0 || sequential.Pages != concurrent.Pages {
		t.Fatalf("page counts = %d and %d", sequential.Pages, concurrent.Pages)
	}
	if sequential.Texts != concurrent.Texts || sequential.Glyphs != concurrent.Glyphs {
		t.Fatalf("counts differ: sequential=%+v concurrent=%+v", sequential, concurrent)
	}
	if sequential.SHA256 != concurrent.SHA256 {
		t.Fatalf("digests differ: %s and %s", sequential.SHA256, concurrent.SHA256)
	}
	var output bytes.Buffer
	if err := writeBenchmarkJSONL(&output, concurrent); err != nil {
		t.Fatal(err)
	}
	if output.Len() == 0 || output.Bytes()[output.Len()-1] != '\n' {
		t.Fatalf("JSONL output is not newline terminated: %q", output.String())
	}
}

func TestPageProjectionDigestUsesDelimiterFreeCanonicalObjects(t *testing.T) {
	digest := sha256.New()
	writePageDigest(digest, []byte(`{"page":0}`))
	writePageDigest(digest, []byte(`{"page":1}`))
	want := sha256.Sum256([]byte(`{"page":0}{"page":1}`))
	if got := digest.Sum(nil); !bytes.Equal(got, want[:]) {
		t.Fatalf("page projection digest = %x, want %x", got, want)
	}
	withNewlines := sha256.Sum256([]byte("{\"page\":0}\n{\"page\":1}\n"))
	if bytes.Equal(digest.Sum(nil), withNewlines[:]) {
		t.Fatal("page projection digest unexpectedly uses JSON Lines framing")
	}
}

func TestBenchmarkJSONUsesCanonicalFloats(t *testing.T) {
	encoded, err := marshalBenchmarkJSON(pageOutput{
		Page: 0,
		Texts: []textOutput{{
			Text: "<&",
			BBox: benchmarkBox([4]float64{1.2, -71.52499999999999, 71.52499999999999, 394.1236574}),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"page":0,"texts":[{"text":"<&","bbox":[1.20,-71.53,71.53,394.12],"glyphs":null}]}`
	if string(encoded) != want {
		t.Fatalf("canonical JSON = %s, want %s", encoded, want)
	}
}

func TestEffectiveBenchmarkWorkersUsesPageLimit(t *testing.T) {
	if got := effectiveBenchmarkWorkers(1, 4); got != 1 {
		t.Fatalf("workers for one page = %d, want 1", got)
	}
	if got := effectiveBenchmarkWorkers(10, 4); got != 1 {
		t.Fatalf("workers for ten pages = %d, want 1", got)
	}
	if got := effectiveBenchmarkWorkers(11, 4); got != 2 {
		t.Fatalf("workers for eleven pages = %d, want 2", got)
	}
	want := 4
	if cpuLimit := runtime.GOMAXPROCS(0); want > cpuLimit {
		want = cpuLimit
	}
	if got := effectiveBenchmarkWorkers(1000, 4); got != want {
		t.Fatalf("workers for large document = %d, want %d", got, want)
	}
	requested := runtime.GOMAXPROCS(0) + 1
	if got := effectiveBenchmarkWorkers(1000, requested); got > runtime.GOMAXPROCS(0) {
		t.Fatalf("workers exceed GOMAXPROCS: got %d, max %d", got, runtime.GOMAXPROCS(0))
	}
}

func TestRunRecoveredConvertsPanicToError(t *testing.T) {
	err := runRecovered("test worker", func() error {
		panic("boom")
	})
	if err == nil || !strings.Contains(err.Error(), "test worker panic: boom") {
		t.Fatalf("recovered panic = %v", err)
	}
}

func TestCountTasksAgreeOnPageCounts(t *testing.T) {
	path := testfixture.Path(t, "form_simple.pdf")
	pages, err := runCountTask(path, "open-pages", 1)
	if err != nil {
		t.Fatal(err)
	}
	texts, err := runCountTask(path, "text-glyphs", 4)
	if err != nil {
		t.Fatal(err)
	}
	if pages.Pages == 0 || pages.Pages != texts.Pages {
		t.Fatalf("page counts = %d and %d", pages.Pages, texts.Pages)
	}
}

func TestProfileTasksCoverLayoutImagesAndObjects(t *testing.T) {
	layoutResult, err := runCountTask(testfixture.Path(t, "acceptance_tagged_text.pdf"), "layout-items", 1)
	if err != nil {
		t.Fatal(err)
	}
	if layoutResult.Pages == 0 || layoutResult.LayoutLines == 0 || layoutResult.LayoutItems == 0 {
		t.Fatalf("layout counts = %+v", layoutResult)
	}
	imageResult, err := runCountTask(testfixture.Path(t, "acceptance_rgb_image.pdf"), "image-digests", 1)
	if err != nil {
		t.Fatal(err)
	}
	if imageResult.Pages == 0 || imageResult.Images == 0 {
		t.Fatalf("image counts = %+v", imageResult)
	}
	objectResult, err := runCountTask(testfixture.Path(t, "form_simple.pdf"), "objects", 1)
	if err != nil {
		t.Fatal(err)
	}
	if objectResult.Objects == 0 {
		t.Fatalf("object counts = %+v", objectResult)
	}
}

func benchmarkTextProjection(b testing.TB, path string) []pageOutput {
	b.Helper()
	doc, err := openBenchmarkPDF(path)
	if err != nil {
		b.Fatal(err)
	}
	defer func() {
		if err := doc.Close(); err != nil {
			b.Fatal(err)
		}
	}()

	outputs := make([]pageOutput, 0)
	pageIndex := 0
	for page, err := range doc.Pages() {
		if err != nil {
			b.Fatal(err)
		}
		output := pageOutput{Page: pageIndex}
		for text, err := range page.Texts(doc) {
			if err != nil {
				b.Fatal(err)
			}
			item := textOutput{Text: text.Text(), BBox: benchmarkBox(text.BBox()), Glyphs: make([]glyphOutput, 0)}
			for glyph, err := range text.GlyphsSeq() {
				if err != nil {
					b.Fatal(err)
				}
				item.Glyphs = append(item.Glyphs, glyphOutput{
					Text: glyph.Text(), Origin: benchmarkPoint(glyph.Origin()), Displacement: benchmarkPoint(glyph.Displacement()), BBox: benchmarkBox(glyph.BBox()),
				})
			}
			output.Texts = append(output.Texts, item)
		}
		outputs = append(outputs, output)
		pageIndex++
	}
	return outputs
}

func BenchmarkOpenAndPages(b *testing.B) {
	path := benchmarkPDF(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		doc, err := openBenchmarkPDF(path)
		if err != nil {
			b.Fatal(err)
		}
		pages := 0
		for _, err := range doc.Pages() {
			if err != nil {
				b.Fatal(err)
			}
			pages++
		}
		if err := doc.Close(); err != nil {
			b.Fatal(err)
		}
		b.SetBytes(int64(pages))
	}
}

func BenchmarkTextsAndGlyphs(b *testing.B) {
	path := benchmarkPDF(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		doc, err := openBenchmarkPDF(path)
		if err != nil {
			b.Fatal(err)
		}
		texts, glyphs := 0, 0
		for page, err := range doc.Pages() {
			if err != nil {
				b.Fatal(err)
			}
			for text, err := range page.Texts(doc) {
				if err != nil {
					b.Fatal(err)
				}
				texts++
				for _, err := range text.GlyphsSeq() {
					if err != nil {
						b.Fatal(err)
					}
					glyphs++
				}
			}
		}
		if err := doc.Close(); err != nil {
			b.Fatal(err)
		}
		b.ReportMetric(float64(texts), "texts/op")
		b.ReportMetric(float64(glyphs), "glyphs/op")
	}
}

// BenchmarkSequentialTextGlyphJSON measures the complete single-thread
// workload used by the migration baseline, including output serialization.
func BenchmarkSequentialTextGlyphJSON(b *testing.B) {
	path := benchmarkPDF(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := runTextGlyphJSONL(path, 1)
		if err != nil {
			b.Fatal(err)
		}
		b.ReportMetric(float64(result.Texts), "texts/op")
		b.ReportMetric(float64(result.Glyphs), "glyphs/op")
	}
}

func BenchmarkConcurrentTextGlyphJSON(b *testing.B) {
	path := benchmarkPDF(b)
	for _, workers := range []int{1, 2, 4, 8} {
		b.Run(fmt.Sprintf("workers=%d", workers), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				result, err := runTextGlyphJSONL(path, workers)
				if err != nil {
					b.Fatal(err)
				}
				b.ReportMetric(float64(result.Texts), "texts/op")
				b.ReportMetric(float64(result.Glyphs), "glyphs/op")
			}
		})
	}
}

func BenchmarkJSONProjection(b *testing.B) {
	outputs := benchmarkTextProjection(b, benchmarkPDF(b))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, output := range outputs {
			if _, err := marshalBenchmarkJSON(output); err != nil {
				b.Fatal(err)
			}
		}
	}
}
