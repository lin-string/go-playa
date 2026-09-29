package document

import (
	"sync"
	"testing"

	"github.com/lin-string/go-playa/internal/testfixture"
)

func openFixture(t *testing.T, path string) *Document {
	t.Helper()
	doc, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = doc.Close() })
	return doc
}

func TestConcurrentPageReadsShareDocument(t *testing.T) {
	doc := openFixture(t, testfixture.Path(t, "riscv-unprivileged.pdf"))
	pages, err := doc.CollectPages()
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) == 0 {
		t.Fatal("fixture has no pages")
	}
	if len(pages) > 8 {
		pages = pages[:8]
	}

	type result struct {
		texts  int
		glyphs int
		err    error
	}
	results := make(chan result, len(pages))
	var wg sync.WaitGroup
	for _, page := range pages {
		page := page
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = page.BBox(doc)
			_ = page.Resources(doc)
			texts, glyphs := 0, 0
			for text, err := range page.Texts(doc) {
				if err != nil {
					results <- result{err: err}
					return
				}
				texts++
				for _, err := range text.GlyphsSeq() {
					if err != nil {
						results <- result{err: err}
						return
					}
					glyphs++
				}
			}
			results <- result{texts: texts, glyphs: glyphs}
		}()
	}
	wg.Wait()
	close(results)

	var got result
	for item := range results {
		if item.err != nil {
			t.Fatal(item.err)
		}
		got.texts += item.texts
		got.glyphs += item.glyphs
	}
	if got.texts == 0 {
		t.Fatal("concurrent extraction returned no text objects")
	}
	if got.glyphs == 0 {
		t.Fatal("concurrent extraction returned no glyphs")
	}

	sequential := result{}
	for _, page := range pages {
		_ = page.BBox(doc)
		_ = page.Resources(doc)
		for text, err := range page.Texts(doc) {
			if err != nil {
				t.Fatal(err)
			}
			sequential.texts++
			for _, err := range text.GlyphsSeq() {
				if err != nil {
					t.Fatal(err)
				}
				sequential.glyphs++
			}
		}
	}
	if got != sequential {
		t.Fatalf("concurrent result %+v differs from sequential result %+v", got, sequential)
	}
}

func TestConcurrentObjectResolutionPublishesCompleteValues(t *testing.T) {
	doc := openFixture(t, testfixture.Path(t, "riscv-unprivileged.pdf"))
	var ref Ref
	for candidate, entry := range doc.xrefs {
		if !entry.isFree() {
			ref = candidate
			break
		}
	}
	if ref == (Ref{}) {
		t.Fatal("fixture has no indirect objects")
	}

	const readers = 32
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, readers)
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for attempt := 0; attempt < 8; attempt++ {
				if _, ok := doc.Object(ref); !ok {
					errs <- errNilDocument
					return
				}
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}

func TestConcurrentObjectIterationAndPageCount(t *testing.T) {
	doc := openFixture(t, testfixture.Path(t, "form_simple.pdf"))
	const readers = 16
	var wg sync.WaitGroup
	errs := make(chan error, readers)
	start := make(chan struct{})
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for object, err := range doc.Objects() {
				if err != nil {
					errs <- err
					return
				}
				if object.Ref().Object == 0 {
					errs <- errNilDocument
					return
				}
			}
			if doc.PageCount() == 0 {
				errs <- errNilDocument
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}

func TestConcurrentPageLabelIteration(t *testing.T) {
	doc := openFixture(t, testfixture.Path(t, "form_simple.pdf"))
	const readers = 16
	var wg sync.WaitGroup
	errs := make(chan error, readers)
	start := make(chan struct{})
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			count := 0
			for label, err := range doc.PageLabelsSeq() {
				if err != nil {
					errs <- err
					return
				}
				if label == "" {
					errs <- errNilDocument
					return
				}
				count++
			}
			if count == 0 {
				errs <- errNilDocument
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}
