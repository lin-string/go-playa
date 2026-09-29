package document

import (
	"context"
	"fmt"
	"runtime"
	"sync"

	"github.com/lin-string/go-playa/layout"
)

func recoveredPanicError(operation string, recovered any) error {
	if err, ok := recovered.(error); ok {
		return fmt.Errorf("playa: %s panic: %w", operation, err)
	}
	return fmt.Errorf("playa: %s panic: %v", operation, recovered)
}

type pageConcurrencyConfig struct {
	maxWorkers     int
	pagesPerWorker int
}

// PageConcurrencyOption configures adaptive page-worker scheduling. Use
// WithMaxPageWorkers and WithPagesPerWorker to construct options.
type PageConcurrencyOption interface {
	applyPageConcurrency(*pageConcurrencyConfig)
}

type pageConcurrencyOptionFunc func(*pageConcurrencyConfig)

func (option pageConcurrencyOptionFunc) applyPageConcurrency(config *pageConcurrencyConfig) {
	option(config)
}

// WithMaxPageWorkers sets the maximum number of page workers. A non-positive
// value selects GOMAXPROCS; all values are bounded by GOMAXPROCS and page count.
func WithMaxPageWorkers(workers int) PageConcurrencyOption {
	return pageConcurrencyOptionFunc(func(config *pageConcurrencyConfig) {
		config.maxWorkers = workers
	})
}

// WithPagesPerWorker sets the page budget for each potential worker. A
// non-positive value uses the default of ten pages per worker.
func WithPagesPerWorker(pages int) PageConcurrencyOption {
	return pageConcurrencyOptionFunc(func(config *pageConcurrencyConfig) {
		config.pagesPerWorker = pages
	})
}

func resolvePageConcurrencyOptions(options []PageConcurrencyOption) pageConcurrencyConfig {
	config := pageConcurrencyConfig{pagesPerWorker: 10}
	for _, option := range options {
		if option != nil {
			option.applyPageConcurrency(&config)
		}
	}
	if config.pagesPerWorker <= 0 {
		config.pagesPerWorker = 10
	}
	return config
}

// ForEachPageConcurrent runs fn on each page with adaptive parallelism bounded
// by GOMAXPROCS and page count. Defaults allow one worker per ten pages, with
// an automatic maximum. Nil options are ignored and later options take precedence.
// Page values are borrowed document views; fn must finish with a page before
// returning and must not call Close or ReleaseTransientCaches. The document
// remains usable for other read-only operations while callbacks are running.
// The receiver, context, and callback are required and must be non-nil.
func (d *Document) ForEachPageConcurrent(ctx context.Context, fn func(Page) error, options ...PageConcurrencyOption) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	config := resolvePageConcurrencyOptions(options)
	return d.forEachAdaptivePageConcurrent(ctx, config.maxWorkers, config.pagesPerWorker, fn)
}

// ForEachPageLayoutConcurrent runs Playa-compatible text layout for each page
// with adaptive page-level parallelism. Layout results are callback-owned and
// must not be used after the callback returns unless finalized or copied.
// The receiver, context, and callback are required and must be non-nil.
func (d *Document) ForEachPageLayoutConcurrent(ctx context.Context, layoutOptions layout.Options, fn func(Page, LayoutResult) error, options ...PageConcurrencyOption) error {
	return d.ForEachPageConcurrent(ctx, func(page Page) error {
		layout, err := page.Layout(d, layoutOptions)
		if err != nil {
			return err
		}
		return fn(page, layout)
	}, options...)
}

// forEachAdaptivePageConcurrent grows the worker pool as pages arrive. This
// preserves the small-document worker cap without calling CollectPages, which
// would materialize every remaining page before the first callback runs.
func (d *Document) forEachAdaptivePageConcurrent(ctx context.Context, requested, pagesPerWorker int, fn func(Page) error) error {
	if requested <= 0 {
		requested = runtime.GOMAXPROCS(0)
	}
	if requested < 1 {
		requested = 1
	}
	if cpuLimit := runtime.GOMAXPROCS(0); requested > cpuLimit {
		requested = cpuLimit
	}
	if pagesPerWorker <= 0 {
		pagesPerWorker = 10
	}
	if requested == 1 {
		return d.forEachPageSerial(ctx, fn)
	}

	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	jobs := make(chan Page, requested*2)
	var wg sync.WaitGroup
	var errMu sync.Mutex
	var firstErr error
	recordError := func(err error) {
		if err == nil {
			return
		}
		errMu.Lock()
		if firstErr == nil {
			firstErr = err
			cancel()
		}
		errMu.Unlock()
	}
	worker := func() {
		defer wg.Done()
		defer func() {
			if recovered := recover(); recovered != nil {
				recordError(recoveredPanicError("page worker", recovered))
			}
		}()
		for {
			select {
			case <-workCtx.Done():
				return
			case page, ok := <-jobs:
				if !ok {
					return
				}
				if workCtx.Err() != nil {
					return
				}
				if err := fn(page); err != nil {
					recordError(err)
					return
				}
			}
		}
	}
	started := 0
	startWorkers := func(want int) {
		if want > requested {
			want = requested
		}
		for started < want {
			started++
			wg.Add(1)
			go worker()
		}
	}
	seen := 0
sendPages:
	for page, pageErr := range d.pagesNoCache() {
		if pageErr != nil {
			recordError(pageErr)
			break
		}
		seen++
		startWorkers((seen + pagesPerWorker - 1) / pagesPerWorker)
		select {
		case <-workCtx.Done():
			break sendPages
		case jobs <- page:
		}
	}
	close(jobs)
	wg.Wait()

	errMu.Lock()
	callbackErr := firstErr
	errMu.Unlock()
	if callbackErr != nil {
		return callbackErr
	}
	return ctx.Err()
}

func (d *Document) forEachPageSerial(ctx context.Context, fn func(Page) error) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = recoveredPanicError("page worker", recovered)
		}
	}()
	for page, pageErr := range d.pagesNoCache() {
		if pageErr != nil {
			return pageErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := fn(page); err != nil {
			return err
		}
	}
	return ctx.Err()
}
