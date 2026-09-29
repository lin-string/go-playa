package document

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/lin-string/go-playa/internal/testfixture"
	"github.com/lin-string/go-playa/layout"
)

func TestPageConcurrencyConfiguration(t *testing.T) {
	for _, test := range []struct {
		name    string
		options []PageConcurrencyOption
		want    pageConcurrencyConfig
	}{
		{name: "defaults", want: pageConcurrencyConfig{pagesPerWorker: 10}},
		{name: "nil option", options: []PageConcurrencyOption{nil}, want: pageConcurrencyConfig{pagesPerWorker: 10}},
		{name: "explicit", options: []PageConcurrencyOption{WithMaxPageWorkers(4), WithPagesPerWorker(2)}, want: pageConcurrencyConfig{maxWorkers: 4, pagesPerWorker: 2}},
		{name: "zero pages", options: []PageConcurrencyOption{WithPagesPerWorker(0)}, want: pageConcurrencyConfig{pagesPerWorker: 10}},
		{name: "negative pages", options: []PageConcurrencyOption{WithPagesPerWorker(-1)}, want: pageConcurrencyConfig{pagesPerWorker: 10}},
		{name: "last option wins", options: []PageConcurrencyOption{WithMaxPageWorkers(4), WithMaxPageWorkers(2), WithPagesPerWorker(1), WithPagesPerWorker(3)}, want: pageConcurrencyConfig{maxWorkers: 2, pagesPerWorker: 3}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := resolvePageConcurrencyOptions(test.options); got != test.want {
				t.Fatalf("configuration = %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestPageConcurrencySchedulerWorkerBounds(t *testing.T) {
	cpuLimit := runtime.GOMAXPROCS(0)
	if cpuLimit > 2 {
		runtime.GOMAXPROCS(2)
		t.Cleanup(func() { runtime.GOMAXPROCS(cpuLimit) })
	}
	workerLimit := runtime.GOMAXPROCS(0)
	for _, test := range []struct {
		name      string
		pages     int
		options   []PageConcurrencyOption
		wantPeak  int32
		needsGrow bool
	}{
		{name: "default ten pages", pages: 10, wantPeak: 1},
		{name: "default eleven pages", pages: 11, wantPeak: 2, needsGrow: true},
		{name: "zero maximum is automatic", pages: 11, options: []PageConcurrencyOption{WithMaxPageWorkers(0)}, wantPeak: 2, needsGrow: true},
		{name: "negative maximum is automatic", pages: 11, options: []PageConcurrencyOption{WithMaxPageWorkers(-1)}, wantPeak: 2, needsGrow: true},
		{name: "explicit maximum respects CPU limit", pages: 21, options: []PageConcurrencyOption{WithMaxPageWorkers(100), WithPagesPerWorker(1)}, wantPeak: int32(workerLimit)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.needsGrow && workerLimit < 2 {
				t.Skip("worker growth requires GOMAXPROCS >= 2")
			}
			synctest.Test(t, func(t *testing.T) {
				kids := make(Array, test.pages)
				for i := range kids {
					kids[i] = Dict{Name("Type"): Name("Page")}
				}
				doc := &Document{trailer: Dict{Name("Root"): Dict{
					Name("Type"): Name("Catalog"),
					Name("Pages"): Dict{
						Name("Type"):  Name("Pages"),
						Name("Count"): Number(test.pages),
						Name("Kids"):  kids,
					},
				}}}
				release := make(chan struct{})
				defer close(release)
				done := make(chan error, 1)
				var active, completed atomic.Int32
				go func() {
					done <- doc.ForEachPageConcurrent(context.Background(), func(Page) error {
						active.Add(1)
						<-release
						active.Add(-1)
						completed.Add(1)
						return nil
					}, test.options...)
				}()
				var peak int32
				for {
					// Every callback blocks at release. Waiting for all goroutines
					// to block lets the producer start every worker it currently can,
					// so measuring overlap never races worker startup or uses sleeps.
					synctest.Wait()
					overlap := active.Load()
					if overlap > peak {
						peak = overlap
					}
					if overlap > test.wantPeak {
						t.Fatalf("active workers = %d, want at most %d", overlap, test.wantPeak)
					}
					select {
					case err := <-done:
						if err != nil {
							t.Fatal(err)
						}
						if got := completed.Load(); got != int32(test.pages) {
							t.Fatalf("completed pages = %d, want %d", got, test.pages)
						}
						if peak != test.wantPeak {
							t.Fatalf("peak worker overlap = %d, want %d", peak, test.wantPeak)
						}
						return
					default:
					}
					if overlap == 0 {
						t.Fatal("scheduler stalled without an active callback")
					}
					release <- struct{}{}
				}
			})
		})
	}
}

func TestForEachPageConcurrentVisitsEachPageOnce(t *testing.T) {
	doc := openFixture(t, testfixture.Path(t, "acceptance_page_labels.pdf"))
	pages, err := doc.CollectPages()
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	seen := map[int]int{}
	err = doc.ForEachPageConcurrent(context.Background(), func(page Page) error {
		if page.number > len(pages) {
			return errors.New("unexpected page number")
		}
		mu.Lock()
		seen[page.number]++
		mu.Unlock()
		return nil
	}, WithMaxPageWorkers(4), WithPagesPerWorker(1))
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != len(pages) {
		t.Fatalf("visited %d pages, want %d", len(seen), len(pages))
	}
	for number, count := range seen {
		if count != 1 {
			t.Fatalf("page %d visited %d times", number, count)
		}
	}
}

func TestForEachPageConcurrentReturnsCallbackError(t *testing.T) {
	doc := openFixture(t, testfixture.Path(t, "form_simple.pdf"))
	want := errors.New("stop")
	if err := doc.ForEachPageConcurrent(context.Background(), func(Page) error { return want }, WithMaxPageWorkers(2)); !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestForEachPageConcurrentHonorsCancellation(t *testing.T) {
	doc := openFixture(t, testfixture.Path(t, "acceptance_page_labels.pdf"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := doc.ForEachPageConcurrent(ctx, func(Page) error { t.Fatal("callback ran after cancellation"); return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestForEachPageConcurrentSkipsQueuedCallbacksAfterStop(t *testing.T) {
	previous := runtime.GOMAXPROCS(2)
	t.Cleanup(func() { runtime.GOMAXPROCS(previous) })
	callbackErr := errors.New("stop queued callbacks")
	for _, test := range []struct {
		name         string
		firstError   error
		wantError    error
		cancelParent bool
	}{
		{name: "cancellation", wantError: context.Canceled, cancelParent: true},
		{name: "first callback error", firstError: callbackErr, wantError: callbackErr},
	} {
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				// The queue and cancellation channel can both be ready. Exercise
				// repeated staged trials so either select choice is covered.
				for trial := range 64 {
					kids := make(Array, 8)
					for i := range kids {
						kids[i] = Dict{Name("Type"): Name("Page")}
					}
					doc := &Document{trailer: Dict{Name("Root"): Dict{
						Name("Type"): Name("Catalog"),
						Name("Pages"): Dict{
							Name("Type"):  Name("Pages"),
							Name("Count"): Number(len(kids)),
							Name("Kids"):  kids,
						},
					}}}
					ctx, cancel := context.WithCancel(context.Background())
					firstRelease := make(chan struct{})
					secondRelease := make(chan struct{})
					done := make(chan error, 1)
					var calls atomic.Int32
					go func() {
						done <- doc.ForEachPageConcurrent(ctx, func(Page) error {
							switch calls.Add(1) {
							case 1:
								<-firstRelease
								return test.firstError
							case 2:
								<-secondRelease
							}
							return nil
						}, WithMaxPageWorkers(2), WithPagesPerWorker(1))
					}()
					// Two callbacks hold both workers. The remaining pages fill the
					// four-slot job queue and block the producer before stopping.
					synctest.Wait()
					if got := calls.Load(); got != 2 {
						t.Fatalf("trial %d: active callbacks = %d, want 2", trial, got)
					}
					if test.cancelParent {
						cancel()
					}
					close(firstRelease)
					// The first callback has returned and its error, if any, has
					// canceled the worker context before the second is released.
					synctest.Wait()
					select {
					case err := <-done:
						t.Fatalf("trial %d: returned before running callback finished: %v", trial, err)
					default:
					}
					close(secondRelease)
					err := <-done
					cancel()
					if !errors.Is(err, test.wantError) {
						t.Fatalf("trial %d: error = %v, want %v", trial, err, test.wantError)
					}
					if got := calls.Load(); got != 2 {
						t.Fatalf("trial %d: callbacks started = %d, want only the 2 already running before stop", trial, got)
					}
				}
			})
		})
	}
}

func TestForEachPageConcurrentDoesNotMaterializeRemainingPages(t *testing.T) {
	doc := openFixture(t, testfixture.Path(t, "acceptance_page_labels.pdf"))
	want := errors.New("stop after first page")
	if err := doc.ForEachPageConcurrent(context.Background(), func(Page) error { return want }, WithMaxPageWorkers(1)); !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}

	doc.cacheMu.RLock()
	ready := doc.pagesReady
	cachedPages := len(doc.pagesCache)
	pageCache := len(doc.pageCache)
	doc.cacheMu.RUnlock()
	if ready || cachedPages != 0 {
		t.Fatalf("early callback error materialized page list: ready=%v pages=%d", ready, cachedPages)
	}
	if pageCache != 0 {
		t.Fatalf("one-pass callback retained page cache: %d", pageCache)
	}
}

func TestAdaptivePageWorkersDoesNotMaterializeRemainingPages(t *testing.T) {
	doc := openFixture(t, testfixture.Path(t, "acceptance_page_labels.pdf"))
	want := errors.New("stop after first adaptive page")
	err := doc.ForEachPageConcurrent(context.Background(), func(Page) error {
		return want
	}, WithMaxPageWorkers(4))
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
	doc.cacheMu.RLock()
	ready := doc.pagesReady
	cachedPages := len(doc.pagesCache)
	pageCache := len(doc.pageCache)
	doc.cacheMu.RUnlock()
	if ready || cachedPages != 0 {
		t.Fatalf("adaptive early callback error materialized page list: ready=%v pages=%d", ready, cachedPages)
	}
	if pageCache != 0 {
		t.Fatalf("adaptive one-pass callback retained page cache: %d", pageCache)
	}
}

func TestConcurrentPageLayoutVisitsEveryPage(t *testing.T) {
	doc := openFixture(t, testfixture.Path(t, "acceptance_page_labels.pdf"))
	var mu sync.Mutex
	seen := map[int]bool{}
	err := doc.ForEachPageLayoutConcurrent(context.Background(), layout.Options{}, func(page Page, layout LayoutResult) error {
		_ = layout.LinesCopy()
		mu.Lock()
		seen[page.number] = true
		mu.Unlock()
		return nil
	}, WithMaxPageWorkers(3), WithPagesPerWorker(2))
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != 4 {
		t.Fatalf("layout pages = %d, want 4", len(seen))
	}
}

func TestConcurrentPageLayoutPropagatesLayoutErrors(t *testing.T) {
	doc := openFixture(t, testfixture.Path(t, "acceptance_page_labels.pdf"))
	want := errors.New("layout callback failure")
	err := doc.ForEachPageLayoutConcurrent(context.Background(), layout.Options{}, func(Page, LayoutResult) error {
		return want
	}, WithMaxPageWorkers(2))
	if !errors.Is(err, want) {
		t.Fatalf("layout error = %v, want %v", err, want)
	}
}

func TestConcurrentPageLayoutRecoversCallbackPanic(t *testing.T) {
	doc := openFixture(t, testfixture.Path(t, "form_simple.pdf"))
	err := doc.ForEachPageLayoutConcurrent(context.Background(), layout.Options{}, func(Page, LayoutResult) error {
		panic("layout callback panic")
	}, WithMaxPageWorkers(2))
	if err == nil || err.Error() != "playa: page worker panic: layout callback panic" {
		t.Fatalf("layout callback panic error = %v", err)
	}
}
