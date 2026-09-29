package document_test

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/lin-string/go-playa/document"
	"github.com/lin-string/go-playa/internal/testfixture"
)

var (
	_ func(*document.Document, context.Context, func(document.Page) error, ...document.PageConcurrencyOption) error                                                = (*document.Document).ForEachPageConcurrent
	_ func(*document.Document, context.Context, document.LayoutOptions, func(document.Page, document.LayoutResult) error, ...document.PageConcurrencyOption) error = (*document.Document).ForEachPageLayoutConcurrent
)

func TestConcurrentPageAPIRequiresContext(t *testing.T) {
	doc, err := document.Open(testfixture.Path(t, "acceptance_page_labels.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	defer func() {
		if recover() == nil {
			t.Fatal("nil required context did not trigger a programmer error")
		}
	}()
	var nilContext context.Context
	_ = doc.ForEachPageConcurrent(nilContext, func(document.Page) error { return nil })
}

func TestConcurrentPageAPIRequiredCallbacksUsePanicRecovery(t *testing.T) {
	doc, err := document.Open(testfixture.Path(t, "acceptance_page_labels.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	for _, workers := range []int{1, 2} {
		for _, layout := range []bool{false, true} {
			var err error
			if layout {
				err = doc.ForEachPageLayoutConcurrent(context.Background(), document.LayoutOptions{}, nil, document.WithMaxPageWorkers(workers))
			} else {
				err = doc.ForEachPageConcurrent(context.Background(), nil, document.WithMaxPageWorkers(workers))
			}
			if err == nil || !strings.HasPrefix(err.Error(), "playa: page worker panic:") {
				t.Fatalf("workers=%d layout=%v: nil required callback error = %v, want recovered invocation panic", workers, layout, err)
			}
		}
	}
}

func TestConcurrentPageAPIVisitsFixture(t *testing.T) {
	doc, err := document.Open(testfixture.Path(t, "acceptance_page_labels.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	var visits atomic.Int32
	err = doc.ForEachPageConcurrent(context.Background(), func(page document.Page) error {
		if page.Width(doc) <= 0 || page.Height(doc) <= 0 {
			t.Errorf("invalid page size: %v x %v", page.Width(doc), page.Height(doc))
		}
		visits.Add(1)
		return nil
	}, document.WithMaxPageWorkers(3))
	if err != nil {
		t.Fatal(err)
	}
	if got := visits.Load(); got != 4 {
		t.Fatalf("concurrent page visits = %d, want 4", got)
	}
}

func TestAdaptiveConcurrentPageAPIVisitsFixture(t *testing.T) {
	doc, err := document.Open(testfixture.Path(t, "acceptance_page_labels.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	var visits atomic.Int32
	err = doc.ForEachPageConcurrent(context.Background(), func(document.Page) error {
		visits.Add(1)
		return nil
	}, document.WithMaxPageWorkers(4), document.WithPagesPerWorker(10))
	if err != nil {
		t.Fatal(err)
	}
	if got := visits.Load(); got != 4 {
		t.Fatalf("adaptive concurrent page visits = %d, want 4", got)
	}
}

func TestConcurrentPageAPIRecoversCallbackPanic(t *testing.T) {
	previous := runtime.GOMAXPROCS(2)
	t.Cleanup(func() { runtime.GOMAXPROCS(previous) })
	doc, err := document.Open(testfixture.Path(t, "acceptance_page_labels.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()
	for _, test := range []struct {
		name    string
		options []document.PageConcurrencyOption
	}{
		{name: "default"},
		{name: "serial", options: []document.PageConcurrencyOption{document.WithMaxPageWorkers(1)}},
		{name: "adaptive", options: []document.PageConcurrencyOption{document.WithMaxPageWorkers(2), document.WithPagesPerWorker(1)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			want := errors.New("callback failure")
			for _, value := range []any{"callback failure", want} {
				err := doc.ForEachPageConcurrent(context.Background(), func(document.Page) error {
					panic(value)
				}, test.options...)
				if err == nil || err.Error() != "playa: page worker panic: callback failure" {
					t.Fatalf("callback panic error = %v, want recovered page worker panic", err)
				}
				if value == want && !errors.Is(err, want) {
					t.Fatalf("callback panic error = %v does not wrap %v", err, want)
				}
			}
		})
	}
}
