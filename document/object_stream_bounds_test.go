package document

import (
	"bytes"
	"errors"
	"sync"
	"testing"

	"github.com/lin-string/go-playa/cacheconfig"
)

func TestLoadObjectStreamRejectsInvalidFirstWithoutPanic(t *testing.T) {
	d := &Document{data: []byte("\n10 0 obj\n<< /Type /ObjStm /N 1 /First 99 /Length 3 >>\nstream\n1 0\nendstream\nendobj"), objects: map[Ref]Object{}, expandedObjStms: map[int]bool{}}
	err := d.loadObjectStream(map[Ref]xrefEntry{{Object: 10}: {offset: 1}}, 10)
	if err == nil {
		t.Fatal("invalid object stream was accepted")
	}
	var parseErr *ParseError
	if !errors.As(err, &parseErr) || parseErr.Operation() != "object stream" {
		t.Fatalf("object stream error = %v", err)
	}
}

func TestLoadObjectStreamRecoversOversizedFirstForIndexedPayload(t *testing.T) {
	data := []byte("\n10 0 obj\n<< /Type /ObjStm /N 1 /First 99 /Length 23 >>\nstream\n3 0 << /Type /Page >>\nendstream\nendobj")
	entries := map[Ref]xrefEntry{
		{Object: 10}: {offset: 1},
		{Object: 3}:  newCompressedXRefEntry(10, 0, false, true),
	}
	d := &Document{data: data, objects: map[Ref]Object{}, expandedObjStms: map[int]bool{}}
	if err := d.loadObjectStream(entries, 10); err != nil {
		t.Fatalf("recoverable oversized First rejected: %v", err)
	}
	if _, ok := d.objects[Ref{Object: 3}].(Dict); !ok {
		t.Fatalf("recovered object = %#v, want page dictionary", d.objects[Ref{Object: 3}])
	}
}

func TestLoadObjectStreamRejectsHugeCountBeforeRecoveryWalk(t *testing.T) {
	data := []byte("\n10 0 obj\n<< /Type /ObjStm /N 999999999 /First 99 /Length 23 >>\nstream\n3 0 << /Type /Page >>\nendstream\nendobj")
	entries := map[Ref]xrefEntry{{Object: 10}: {offset: 1}}
	d := &Document{data: data, objects: map[Ref]Object{}, expandedObjStms: map[int]bool{}}
	if err := d.loadObjectStream(entries, 10); err == nil {
		t.Fatal("huge object-stream count was accepted")
	}
}

func TestLoadObjectStreamRepeatsDeferredParseError(t *testing.T) {
	d := &Document{data: []byte("\n10 0 obj\n<< /Type /ObjStm /N 1 /First 99 /Length 3 >>\nstream\n1 0\nendstream\nendobj"), objects: map[Ref]Object{}, expandedObjStms: map[int]bool{}}
	entries := map[Ref]xrefEntry{{Object: 10}: {offset: 1}}
	first := d.loadObjectStream(entries, 10)
	second := d.loadObjectStream(entries, 10)
	if first == nil || second == nil {
		t.Fatalf("repeated object stream errors = %v, %v", first, second)
	}
	if first != second {
		t.Fatalf("repeated object stream error was not cached: first=%v second=%v", first, second)
	}
	if d.objectStreamErrors[10] != first {
		t.Fatal("object stream error cache does not retain the returned error")
	}
	var firstParse, secondParse *ParseError
	if !errors.As(first, &firstParse) || !errors.As(second, &secondParse) || firstParse.Operation() != secondParse.Operation() {
		t.Fatalf("object stream errors = %v, %v", first, second)
	}
}

func TestLoadObjectStreamHonorsErrorCacheBudget(t *testing.T) {
	d := &Document{
		cacheOptionsConfigured: true,
		cacheOptions:           cacheconfig.Options{ObjectStreamErrorBytes: 0},
		data:                   []byte("\n10 0 obj\n<< /Type /ObjStm /N 1 /First 99 /Length 3 >>\nstream\n1 0\nendstream\nendobj"),
		objects:                map[Ref]Object{},
		expandedObjStms:        map[int]bool{},
	}
	entries := map[Ref]xrefEntry{{Object: 10}: {offset: 1}}
	firstErr := d.loadObjectStream(entries, 10)
	secondErr := d.loadObjectStream(entries, 10)
	if firstErr == nil || secondErr == nil || firstErr == secondErr {
		t.Fatalf("object stream errors = %v/%v, want uncached independent errors", firstErr, secondErr)
	}
	if len(d.objectStreamErrors) != 0 {
		t.Fatalf("object stream error cache retained entry with zero budget: %#v", d.objectStreamErrors)
	}
}

func TestLoadObjectStreamErrorsHaveBoundedCache(t *testing.T) {
	d := &Document{
		data:            []byte("\n10 0 obj\n42\nendobj"),
		objects:         map[Ref]Object{},
		expandedObjStms: map[int]bool{},
	}
	entries := map[Ref]xrefEntry{}
	const attempts = 4096
	for i := 0; i < attempts; i++ {
		entries[Ref{Object: 10 + i}] = xrefEntry{offset: 1}
		if err := d.loadObjectStream(entries, 10+i); err == nil {
			t.Fatalf("object stream %d unexpectedly loaded", 10+i)
		}
	}
	if len(d.objectStreamErrors) >= attempts {
		t.Fatalf("object stream error cache retained every failure: entries=%d", len(d.objectStreamErrors))
	}
}

func TestLoadObjectStreamDoesNotCacheMalformedContainer(t *testing.T) {
	d := &Document{
		data:            []byte("\n10 0 obj\n42\nendobj"),
		objects:         map[Ref]Object{},
		expandedObjStms: map[int]bool{},
	}
	entries := map[Ref]xrefEntry{{Object: 10}: {offset: 1}}
	err := d.loadObjectStream(entries, 10)
	if err == nil {
		t.Fatal("non-stream object stream was accepted")
	}
	if d.expandedObjStms[10] {
		t.Fatal("malformed object stream was marked expanded")
	}
	if err := d.loadObjectStream(entries, 10); err == nil {
		t.Fatal("malformed object stream was silently cached")
	}
}

func TestLoadObjectStreamRejectsNonObjStmType(t *testing.T) {
	d := &Document{data: []byte("\n10 0 obj\n<< /Type /NotObjStm /N 0 /First 0 /Length 0 >>\nstream\n\nendstream\nendobj"), objects: map[Ref]Object{}, expandedObjStms: map[int]bool{}}
	if err := d.loadObjectStream(map[Ref]xrefEntry{{Object: 10}: {offset: 1}}, 10); err == nil {
		t.Fatal("object stream with non-ObjStm Type was accepted")
	}
	if d.expandedObjStms[10] {
		t.Fatal("malformed object stream was marked expanded")
	}
}

func TestLoadObjectStreamResolvesNAndFirst(t *testing.T) {
	d := &Document{data: []byte("\n10 0 obj\n<< /Type /ObjStm /N 11 0 R /First 12 0 R /Length 3 >>\nstream\n1 0\nendstream\nendobj"), objects: map[Ref]Object{}, expandedObjStms: map[int]bool{}}
	d.objects[Ref{Object: 11}] = Number(1)
	d.objects[Ref{Object: 12}] = Number(99)
	// The indirect dictionary values are resolved before the bounds check;
	// this fixture verifies the path without requiring a full xref table.
	if err := d.loadObjectStream(map[Ref]xrefEntry{{Object: 10}: {offset: 1}}, 10); err == nil {
		t.Fatal("expected invalid First after resolving indirect values")
	}
}

func TestLoadObjectStreamFollowsMultiLevelIndirectHeaderFields(t *testing.T) {
	d := &Document{data: []byte("\n10 0 obj\n<< /Type 11 0 R /N 12 0 R /First 13 0 R /Length 8 >>\nstream\n1 0\n<<>>\nendstream\nendobj"), objects: map[Ref]Object{}, expandedObjStms: map[int]bool{}}
	d.objects[Ref{Object: 11}] = Ref{Object: 14}
	d.objects[Ref{Object: 12}] = Ref{Object: 15}
	d.objects[Ref{Object: 13}] = Ref{Object: 16}
	d.objects[Ref{Object: 14}] = Name("ObjStm")
	d.objects[Ref{Object: 15}] = Number(1)
	d.objects[Ref{Object: 16}] = Number(4)
	entries := map[Ref]xrefEntry{{Object: 10}: {offset: 1}, {Object: 1}: newCompressedXRefEntry(10, 0, false, true)}
	if err := d.loadObjectStream(entries, 10); err != nil {
		t.Fatal(err)
	}
	if _, ok := d.objects[Ref{Object: 1}]; !ok {
		t.Fatal("multi-level indirect object stream did not expand object 1")
	}
}

func TestLoadObjectStreamRejectsImpossibleObjectCountBeforeAllocating(t *testing.T) {
	d := &Document{data: []byte("\n10 0 obj\n<< /Type /ObjStm /N 1000000 /First 4 /Length 4 >>\nstream\n1 0\nendstream\nendobj"), objects: map[Ref]Object{}, expandedObjStms: map[int]bool{}}
	err := d.loadObjectStream(map[Ref]xrefEntry{{Object: 10}: {offset: 1}}, 10)
	if err == nil {
		t.Fatal("impossible object stream count was accepted")
	}
}

func TestLoadObjectStreamRejectsNonIntegerHeaderFields(t *testing.T) {
	d := &Document{data: []byte("\n10 0 obj\n<< /Type /ObjStm /N 1 /First 4 /Length 8 >>\nstream\nfoo 0\n<<>>\nendstream\nendobj"), objects: map[Ref]Object{}, expandedObjStms: map[int]bool{}}
	if err := d.loadObjectStream(map[Ref]xrefEntry{{Object: 10}: {offset: 1}}, 10); err == nil {
		t.Fatal("non-integer object stream header was accepted")
	}
}

func TestLoadObjectStreamRejectsTrailingHeaderData(t *testing.T) {
	d := &Document{data: []byte("\n10 0 obj\n<< /Type /ObjStm /N 1 /First 9 /Length 13 >>\nstream\n1 0 junk\n<<>>\nendstream\nendobj"), objects: map[Ref]Object{}, expandedObjStms: map[int]bool{}}
	if err := d.loadObjectStream(map[Ref]xrefEntry{{Object: 10}: {offset: 1}}, 10); err == nil {
		t.Fatal("trailing object stream header data was accepted")
	}
}

func TestLoadObjectStreamRejectsNegativeObjectNumbers(t *testing.T) {
	d := &Document{data: []byte("\n10 0 obj\n<< /Type /ObjStm /N 1 /First 5 /Length 9 >>\nstream\n-1 0\n<<>>\nendstream\nendobj"), objects: map[Ref]Object{}, expandedObjStms: map[int]bool{}}
	if err := d.loadObjectStream(map[Ref]xrefEntry{{Object: 10}: {offset: 1}}, 10); err == nil {
		t.Fatal("negative object stream object number was accepted")
	}
}

func TestLoadObjectStreamPreservesStableLookupIndexAndInvalidatesDependentCaches(t *testing.T) {
	entries := map[Ref]xrefEntry{
		{Object: 10}: {offset: 1},
		{Object: 1}:  newCompressedXRefEntry(10, 0, false, true),
	}
	d := &Document{
		data:             []byte("\n10 0 obj\n<< /Type /ObjStm /N 1 /First 4 /Length 8 >>\nstream\n1 0\n<<>>\nendstream\nendobj"),
		xrefs:            entries,
		objects:          map[Ref]Object{},
		expandedObjStms:  map[int]bool{},
		objectRefs:       []Ref{{Object: 99}},
		objectRefsReady:  true,
		lookupCache:      map[int]Ref{1: {Object: 1}, 10: {Object: 10}},
		lookupFound:      map[int]bool{1: true, 10: true},
		lookupIndexReady: true,
		pageMissing:      map[Ref]bool{{Object: 99}: true},
		pageCountCache:   1,
		pageCountReady:   true,
	}
	if err := d.loadObjectStream(entries, 10); err != nil {
		t.Fatal(err)
	}
	if d.objectRefsReady || d.objectRefs != nil || d.pageMissing != nil || d.pageCountReady {
		t.Fatalf("stale object-dependent caches remain: refs=%v pages=%v count=%v", d.objectRefsReady, d.pageMissing, d.pageCountReady)
	}
	if !d.lookupIndexReady || d.lookupCache[1] != (Ref{Object: 1}) || !d.lookupFound[1] {
		t.Fatalf("stable object-number index was discarded: ready=%v lookup=%v found=%v", d.lookupIndexReady, d.lookupCache, d.lookupFound)
	}
}

func TestLoadObjectStreamConcurrentBuildPublishesOnce(t *testing.T) {
	d := &Document{
		data:            []byte("\n10 0 obj\n<< /Type /ObjStm /N 1 /First 4 /Length 8 >>\nstream\n1 0\n<<>>\nendstream\nendobj"),
		objects:         map[Ref]Object{},
		expandedObjStms: map[int]bool{},
	}
	entries := map[Ref]xrefEntry{{Object: 10}: {offset: 1}, {Object: 1}: newCompressedXRefEntry(10, 0, false, true)}
	const readers = 32
	var wg sync.WaitGroup
	errs := make(chan error, readers)
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- d.loadObjectStream(entries, 10)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if !d.expandedObjStms[10] || len(d.objects) != 1 {
		t.Fatalf("concurrent object stream state = expanded=%v objects=%d", d.expandedObjStms[10], len(d.objects))
	}
}

func TestReleaseTransientCachesSynchronizesObjectStreamExpansion(t *testing.T) {
	d := &Document{
		data:    []byte("\n10 0 obj\n<< /Type /ObjStm /N 1 /First 4 /Length 8 >>\nstream\n1 0\n<<>>\nendstream\nendobj"),
		objects: map[Ref]Object{}, expandedObjStms: map[int]bool{},
	}
	entries := map[Ref]xrefEntry{{Object: 10}: {offset: 1}, {Object: 1}: newCompressedXRefEntry(10, 0, false, true)}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 16; j++ {
				_ = d.loadObjectStream(entries, 10)
			}
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 16; j++ {
				d.ReleaseTransientCaches()
			}
		}()
	}
	wg.Wait()
}

func TestIndirectBodyResolvesIndirectLength(t *testing.T) {
	data := []byte("\n1 0 obj\n<< /Length 2 0 R >>\nstream\nabc endobj xyz\nendstream\nendobj\n2 0 obj\n13\nendobj")
	body, ok := indirectBody(data, 1, func(object Object) Object {
		if ref, ok := object.(Ref); ok && ref.Object == 2 {
			return Number(14)
		}
		return object
	})
	if !ok || !bytes.Contains(body, []byte("abc endobj xyz")) {
		t.Fatalf("indirect-length body = %q, ok=%v", body, ok)
	}
}

func TestIndirectBodyFollowsMultiLevelIndirectLength(t *testing.T) {
	data := []byte("\n1 0 obj\n<< /Length 2 0 R >>\nstream\nabc endobj xyzendstreamBendstream\nendobj\n2 0 obj\n3 0 R\nendobj\n3 0 obj\n21\nendobj")
	body, ok := indirectBody(data, 1, func(object Object) Object {
		switch ref := object.(type) {
		case Ref:
			switch ref.Object {
			case 2:
				return Ref{Object: 3}
			case 3:
				return Number(21)
			}
		}
		return object
	})
	if !ok || !bytes.Contains(body, []byte("abc endobj xyzendstreamB")) {
		t.Fatalf("multi-level indirect-length body = %q, ok=%v", body, ok)
	}
}
