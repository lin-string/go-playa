package main

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	playa "github.com/lin-string/go-playa"
)

type benchmarkResult struct {
	Library          string `json:"library"`
	PDF              string `json:"pdf"`
	Task             string `json:"task"`
	RequestedWorkers int    `json:"requested_workers"`
	EffectiveWorkers int    `json:"effective_workers"`
	ObservedWorkers  int    `json:"observed_workers"`
	Pages            int    `json:"pages"`
	Texts            int    `json:"texts"`
	Glyphs           int    `json:"glyphs"`
	LayoutLines      int    `json:"layout_lines"`
	LayoutItems      int    `json:"layout_items"`
	Images           int    `json:"images"`
	Objects          int    `json:"objects"`
	ElapsedNS        int64  `json:"elapsed_ns"`
	AllocBytes       uint64 `json:"alloc_bytes"`
	PeakRSSBytes     uint64 `json:"peak_rss_bytes,omitempty"`
	SHA256           string `json:"sha256,omitempty"`
}

type pageRecord struct {
	index  int
	data   []byte
	texts  int
	glyphs int
}

type pageCountRecord struct {
	texts       int
	glyphs      int
	layoutLines int
	layoutItems int
	images      int
	imageFrames []byte
	ready       bool
}

type concurrencyTracker struct {
	active atomic.Int32
	max    atomic.Int32
}

func (t *concurrencyTracker) enter() func() {
	active := t.active.Add(1)
	for {
		current := t.max.Load()
		if active <= current || t.max.CompareAndSwap(current, active) {
			break
		}
	}
	return func() { t.active.Add(-1) }
}

func (t *concurrencyTracker) workers() int {
	if workers := int(t.max.Load()); workers > 0 {
		return workers
	}
	return 1
}

func runRecovered(name string, fn func() error) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("%s panic: %v", name, recovered)
		}
	}()
	return fn()
}

func runTextGlyphJSONL(path string, workers int) (benchmarkResult, error) {
	maxWorkers := normalizeWorkers(workers)
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	doc, err := openBenchmarkPDF(path)
	if err != nil {
		return benchmarkResult{}, err
	}
	defer func() { _ = doc.Close() }()
	records := make(chan pageRecord, maxInt(maxWorkers, 1)*2)
	workerDone := make(chan error, 1)
	var tracker concurrencyTracker
	go func() {
		var err error
		defer func() {
			if recovered := recover(); recovered != nil {
				err = fmt.Errorf("benchmark page worker goroutine panic: %v", recovered)
			}
			workerDone <- err
			close(records)
		}()
		err = runRecovered("benchmark page worker", func() error {
			return doc.ForEachPageConcurrent(context.Background(), func(page playa.Page) error {
				leave := tracker.enter()
				defer leave()
				record, err := projectPage(page, doc)
				if err != nil {
					return err
				}
				records <- record
				return nil
			}, playa.WithMaxPageWorkers(maxWorkers))
		})
	}()

	digest := sha256.New()
	pending := map[int]pageRecord{}
	next := 0
	result := benchmarkResult{Library: "go-playa", PDF: path, Task: "text-glyph-jsonl"}
	for record := range records {
		pending[record.index] = record
		for {
			ready, ok := pending[next]
			if !ok {
				break
			}
			writePageDigest(digest, ready.data)
			delete(pending, next)
			next++
			result.Pages++
			result.Texts += ready.texts
			result.Glyphs += ready.glyphs
		}
	}
	if err := <-workerDone; err != nil {
		return benchmarkResult{}, err
	}
	if len(pending) != 0 {
		return benchmarkResult{}, fmt.Errorf("page result gap before page %d", next)
	}
	result.RequestedWorkers = workers
	result.EffectiveWorkers = effectiveBenchmarkWorkers(result.Pages, workers)
	result.ObservedWorkers = tracker.workers()
	result.SHA256 = hex.EncodeToString(digest.Sum(nil))
	if err := finishBenchmarkSample(&result, before, start, doc.Close); err != nil {
		return benchmarkResult{}, err
	}
	return result, nil
}

func runCountTask(path, task string, workers int) (benchmarkResult, error) {
	if task != "open-pages" && task != "text-glyphs" && task != "layout-items" && task != "image-digests" && task != "objects" {
		return benchmarkResult{}, fmt.Errorf("unsupported count task %q", task)
	}
	maxWorkers := normalizeWorkers(workers)
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	doc, err := openBenchmarkPDF(path)
	if err != nil {
		return benchmarkResult{}, err
	}
	defer func() { _ = doc.Close() }()
	result := benchmarkResult{Library: "go-playa", PDF: path, Task: task, RequestedWorkers: workers}
	if task == "objects" {
		for _, err := range doc.Objects() {
			if err != nil {
				return benchmarkResult{}, err
			}
			result.Objects++
		}
		result.EffectiveWorkers = 1
		result.ObservedWorkers = 1
		result.Pages = doc.PageCount()
		if err := finishBenchmarkSample(&result, before, start, doc.Close); err != nil {
			return benchmarkResult{}, err
		}
		return result, nil
	}
	counts := map[int]pageCountRecord{}
	var countsMu sync.Mutex
	var tracker concurrencyTracker
	err = doc.ForEachPageConcurrent(context.Background(), func(page playa.Page) error {
		leave := tracker.enter()
		defer leave()
		texts, glyphs, layoutLines, layoutItems, images := 0, 0, 0, 0, 0
		index := page.Number() - 1
		if index < 0 {
			return fmt.Errorf("page index %d outside count range", index)
		}
		var imageFrames []byte
		switch task {
		case "text-glyphs":
			for text, err := range page.Texts(doc) {
				if err != nil {
					return err
				}
				texts++
				for _, err := range text.GlyphsSeq() {
					if err != nil {
						return err
					}
					glyphs++
				}
			}
		case "layout-items":
			layoutResult, err := page.Layout(doc, playa.DefaultLayoutOptions())
			if err != nil {
				return err
			}
			for range layoutResult.LinesSeq() {
				layoutLines++
			}
			for range layoutResult.ItemsSeq() {
				layoutItems++
			}
		case "image-digests":
			for image, err := range page.Images(doc) {
				if err != nil {
					return err
				}
				length, digest, err := image.DecodedStreamBufferDigestWithError()
				if err != nil {
					return err
				}
				rawDigest, err := hex.DecodeString(digest)
				if err != nil || len(rawDigest) != sha256.Size || length < 0 {
					return fmt.Errorf("invalid decoded image stream digest/length")
				}
				// Each fixed 56-byte record is page/index/decoded length (uint64
				// big endian), followed by the 32 raw SHA-256 bytes.
				var frame [56]byte
				binary.BigEndian.PutUint64(frame[:8], uint64(index))
				binary.BigEndian.PutUint64(frame[8:16], uint64(images))
				binary.BigEndian.PutUint64(frame[16:24], uint64(length))
				copy(frame[24:], rawDigest)
				imageFrames = append(imageFrames, frame[:]...)
				images++
			}
		}
		countsMu.Lock()
		counts[index] = pageCountRecord{texts: texts, glyphs: glyphs, layoutLines: layoutLines, layoutItems: layoutItems, images: images, imageFrames: imageFrames, ready: true}
		countsMu.Unlock()
		return nil
	}, playa.WithMaxPageWorkers(maxWorkers))
	if err != nil {
		return benchmarkResult{}, err
	}
	var imageDigest hash.Hash
	if task == "image-digests" {
		imageDigest = sha256.New()
		_, _ = imageDigest.Write([]byte("playa-image-digests-v1\x00"))
	}
	for index := 0; index < len(counts); index++ {
		count, ok := counts[index]
		if !ok || !count.ready {
			return benchmarkResult{}, fmt.Errorf("page count gap at page %d", index)
		}
		result.Pages++
		result.Texts += count.texts
		result.Glyphs += count.glyphs
		result.LayoutLines += count.layoutLines
		result.LayoutItems += count.layoutItems
		result.Images += count.images
		if imageDigest != nil {
			_, _ = imageDigest.Write(count.imageFrames)
		}
	}
	if imageDigest != nil {
		result.SHA256 = hex.EncodeToString(imageDigest.Sum(nil))
	}
	result.EffectiveWorkers = effectiveBenchmarkWorkers(result.Pages, workers)
	result.ObservedWorkers = tracker.workers()
	if err := finishBenchmarkSample(&result, before, start, doc.Close); err != nil {
		return benchmarkResult{}, err
	}
	return result, nil
}

// Completion includes document closure but excludes measurement bookkeeping.
func finishBenchmarkSample(result *benchmarkResult, before runtime.MemStats, start time.Time, closeDocument func() error) error {
	if err := closeDocument(); err != nil {
		return err
	}
	result.ElapsedNS = time.Since(start).Nanoseconds()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	result.AllocBytes = after.TotalAlloc - before.TotalAlloc
	result.PeakRSSBytes = peakRSSBytes()
	return nil
}

func normalizeWorkers(workers int) int {
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	if workers < 1 {
		return 1
	}
	return workers
}

func effectiveBenchmarkWorkers(pageCount, requested int) int {
	requested = normalizeWorkers(requested)
	if cpuLimit := runtime.GOMAXPROCS(0); requested > cpuLimit {
		requested = cpuLimit
	}
	pageLimit := (pageCount + 9) / 10
	if pageLimit < 1 {
		pageLimit = 1
	}
	if requested > pageLimit {
		requested = pageLimit
	}
	return requested
}

func projectPage(page playa.Page, doc *playa.Document) (pageRecord, error) {
	output := pageOutput{Page: page.Number() - 1, Texts: make([]textOutput, 0)}
	record := pageRecord{index: output.Page}
	for text, err := range page.Texts(doc) {
		if err != nil {
			return pageRecord{}, err
		}
		item := textOutput{Text: text.Text(), BBox: benchmarkBox(text.BBox()), Glyphs: make([]glyphOutput, 0)}
		for glyph, err := range text.GlyphsSeq() {
			if err != nil {
				return pageRecord{}, err
			}
			item.Glyphs = append(item.Glyphs, glyphOutput{
				Text: glyph.Text(), Origin: benchmarkPoint(glyph.Origin()), Displacement: benchmarkPoint(glyph.Displacement()), BBox: benchmarkBox(glyph.BBox()),
			})
		}
		record.texts++
		record.glyphs += len(item.Glyphs)
		output.Texts = append(output.Texts, item)
	}
	data, err := marshalBenchmarkJSON(output)
	if err != nil {
		return pageRecord{}, err
	}
	record.data = data
	return record, nil
}

func writePageDigest(dst hash.Hash, data []byte) {
	_, _ = dst.Write(data)
}

func writeBenchmarkJSONL(dst io.Writer, result benchmarkResult) error {
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = dst.Write(data)
	return err
}

func maxInt(value, fallback int) int {
	if value > fallback {
		return value
	}
	return fallback
}
