package document

import (
	"sync"
	"testing"

	"github.com/lin-string/go-playa/internal/testfixture"
)

func TestConcurrentFontLazyInitialization(t *testing.T) {
	doc := openFixture(t, testfixture.Path(t, "acceptance_cjk_cid.pdf"))
	pages, err := doc.CollectPages()
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) == 0 {
		t.Fatal("fixture has no pages")
	}
	fonts, err := doc.pageFontsDirect(pages[0])
	if err != nil {
		t.Fatal(err)
	}
	var font *Font
	for _, candidate := range fonts {
		font = candidate
		break
	}
	if font == nil {
		t.Fatal("fixture has no page fonts")
	}

	const readers = 16
	start := make(chan struct{})
	errs := make(chan error, readers)
	var wg sync.WaitGroup
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if font.Finalize() == nil {
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

func TestConcurrentZeroValueFontLazyMutexInitialization(t *testing.T) {
	var font Font
	const readers = 16
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, _ = font.EncodingValue(65)
		}()
	}
	close(start)
	wg.Wait()
}

func TestConcurrentFontDecodeAndEncodingUpdates(t *testing.T) {
	font := NewSimpleFont("concurrent-encoding")
	font.widths[65] = 600

	const readers = 16
	const rounds = 32
	start := make(chan struct{})
	var wg sync.WaitGroup
	var decoded int
	var decodedMu sync.Mutex
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for round := 0; round < rounds; round++ {
				for glyph := range font.DecodeGlyphsSeq([]byte{'A'}) {
					if glyph.Text() == "" {
						t.Errorf("decoded empty glyph at round %d", round)
					}
					decodedMu.Lock()
					decoded++
					decodedMu.Unlock()
				}
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for round := 0; round < rounds; round++ {
			font.applyEncoding("WinAnsiEncoding")
			font.applyDifferences(Array{Number(65), Name("A")})
		}
	}()
	close(start)
	wg.Wait()
	if decoded != readers*rounds {
		t.Fatalf("decoded %d glyphs, want %d", decoded, readers*rounds)
	}
}
