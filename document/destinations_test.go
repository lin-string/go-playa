package document

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"

	"github.com/lin-string/go-playa/cacheconfig"
)

func TestDestinationsEmpty(t *testing.T) {
	d := &Document{trailer: Dict{}, objects: map[Ref]Object{}}
	if got, err := d.Destinations(); err != nil || len(got) != 0 {
		t.Fatalf("unexpected destinations = %#v, err = %v", got, err)
	}
}

func TestDestinationsPreferCatalogDictionaryOverNameTree(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{
				Name("Dests"): Dict{Name("legacy"): Array{Number(0), Name("Fit")}},
				Name("Names"): Dict{Name("Dests"): Dict{Name("Names"): Array{
					String("tree-only"), Array{Number(1), Name("Fit")},
				}}},
			},
		},
	}

	var entries []DestinationEntry
	for entry, err := range d.DestinationsSeq() {
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, entry)
	}
	if len(entries) != 1 || entries[0].Name() != "legacy" {
		t.Fatalf("destination entries = %#v, want only catalog Dests", entries)
	}
	values, err := d.Destinations()
	if err != nil || len(values) != 1 || values["legacy"] == nil || values["tree-only"] != nil {
		t.Fatalf("materialized destinations = %#v, err = %v", values, err)
	}
}

func TestCloneDestinationMapPreservesAbsentMap(t *testing.T) {
	if got := cloneDestinationMap(nil); got != nil {
		t.Fatalf("cloned absent destination map = %#v, want nil", got)
	}
}

func TestDestinationEntryWithErrorRejectsNilDocument(t *testing.T) {
	if destination, err := (DestinationEntry{}).DestinationWithError(nil); err != errNilDocument || destination != nil {
		t.Fatalf("destination = %#v, err = %v", destination, err)
	}
}

func TestNameTreeFollowMultiLevelIndirectNodeValues(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Ref{Object: 1}}, objects: map[Ref]Object{}}
	d.objects[Ref{Object: 1}] = Dict{Name("Names"): Ref{Object: 2}}
	d.objects[Ref{Object: 2}] = Dict{Name("Dests"): Ref{Object: 3}}
	d.objects[Ref{Object: 3}] = Dict{Name("Kids"): Ref{Object: 4}}
	d.objects[Ref{Object: 4}] = Array{Ref{Object: 5}}
	d.objects[Ref{Object: 5}] = Ref{Object: 6}
	d.objects[Ref{Object: 6}] = Dict{Name("Names"): Ref{Object: 7}}
	d.objects[Ref{Object: 7}] = Array{Ref{Object: 8}, Ref{Object: 9}}
	d.objects[Ref{Object: 8}] = String("chapter")
	d.objects[Ref{Object: 9}] = Ref{Object: 10}
	d.objects[Ref{Object: 10}] = Array{Number(0), Name("Fit")}

	var entries []NameTreeEntry
	for entry, err := range d.NameTreeSeq("Dests") {
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, entry)
	}
	if len(entries) != 1 || entries[0].Name() != "chapter" {
		t.Fatalf("name tree entries = %#v", entries)
	}
}

func TestNameTreeReportsUnresolvedExplicitReferences(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Ref{Object: 1}}, objects: map[Ref]Object{
		{Object: 1}: Dict{Name("Names"): Dict{Name("Dests"): Ref{Object: 2}}},
		{Object: 2}: Dict{Name("Names"): Ref{Object: 99}},
	}}
	for _, err := range d.NameTreeSeq("Dests") {
		if err == nil || err.Error() != "playa: name tree Names could not be resolved" {
			t.Fatalf("unresolved Names error = %v", err)
		}
		return
	}
	t.Fatal("unresolved Names produced no error")
}

func TestNameTreeRejectsMalformedLimits(t *testing.T) {
	tests := []struct {
		name   string
		limits Object
		want   string
	}{
		{name: "wrong arity", limits: Array{String("a")}, want: "Limits must contain two strings"},
		{name: "wrong type", limits: Array{String("a"), Number(1)}, want: "Limits must contain two strings"},
		{name: "reversed", limits: Array{String("z"), String("a")}, want: "Limits are out of order"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d := &Document{trailer: Dict{Name("Root"): Ref{Object: 1}}, objects: map[Ref]Object{
				{Object: 1}: Dict{Name("Names"): Dict{Name("Dests"): Dict{Name("Limits"): test.limits, Name("Names"): Array{String("a"), Array{Number(0), Name("Fit")}}}}},
			}}
			for _, err := range d.NameTreeSeq("Dests") {
				if err == nil || !strings.Contains(err.Error(), test.want) {
					t.Fatalf("Limits error = %v, want %q", err, test.want)
				}
				return
			}
			t.Fatal("malformed Limits produced no error")
		})
	}
}

func TestNameTreeRejectsUnsortedOrOutOfRangeNames(t *testing.T) {
	tests := []struct {
		name string
		tree Dict
		want string
	}{
		{
			name: "unsorted",
			tree: Dict{Name("Names"): Array{String("z"), Array{}, String("a"), Array{}}},
			want: "Names are not sorted",
		},
		{
			name: "outside limits",
			tree: Dict{Name("Limits"): Array{String("b"), String("d")}, Name("Names"): Array{String("a"), Array{}}},
			want: "name is outside Limits",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d := &Document{trailer: Dict{Name("Root"): Ref{Object: 1}}, objects: map[Ref]Object{
				{Object: 1}: Dict{Name("Names"): Dict{Name("Dests"): test.tree}},
			}}
			for _, err := range d.NameTreeSeq("Dests") {
				if err == nil {
					continue
				}
				if !strings.Contains(err.Error(), test.want) {
					t.Fatalf("NameTree error = %v, want %q", err, test.want)
				}
				return
			}
			t.Fatal("invalid Names produced no error")
		})
	}
}

func TestNameTreeRejectsChildLimitsOutsideParent(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Ref{Object: 1}}, objects: map[Ref]Object{
		{Object: 1}: Dict{Name("Names"): Dict{Name("Dests"): Dict{
			Name("Limits"): Array{String("b"), String("d")},
			Name("Kids"):   Array{Ref{Object: 2}},
		}}},
		{Object: 2}: Dict{
			Name("Limits"): Array{String("a"), String("c")},
			Name("Names"):  Array{String("b"), Array{Number(0), Name("Fit")}},
		},
	}}
	for _, err := range d.NameTreeSeq("Dests") {
		if err == nil || !strings.Contains(err.Error(), "child Limits are outside parent") {
			if err == nil {
				continue
			}
			t.Fatalf("child Limits error = %v", err)
		}
		return
	}
	t.Fatal("child Limits outside parent produced no error")
}

func TestNameTreeRejectsMixedNamesAndKidsNode(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Ref{Object: 1}}, objects: map[Ref]Object{
		{Object: 1}: Dict{Name("Names"): Dict{Name("Dests"): Dict{
			Name("Names"): Array{String("a"), Array{}},
			Name("Kids"):  Array{},
		}}},
	}}
	for _, err := range d.NameTreeSeq("Dests") {
		if err == nil || !strings.Contains(err.Error(), "cannot contain both Names and Kids") {
			t.Fatalf("mixed node error = %v", err)
		}
		return
	}
	t.Fatal("mixed Names/Kids node produced no error")
}

func TestNameTreeRejectsOverlappingSiblingLimits(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Ref{Object: 1}}, objects: map[Ref]Object{
		{Object: 1}: Dict{Name("Names"): Dict{Name("Dests"): Dict{Name("Kids"): Array{Ref{Object: 2}, Ref{Object: 3}}}}},
		{Object: 2}: Dict{Name("Limits"): Array{String("a"), String("d")}, Name("Names"): Array{String("b"), Array{}}},
		{Object: 3}: Dict{Name("Limits"): Array{String("c"), String("e")}, Name("Names"): Array{String("c"), Array{}}},
	}}
	for _, err := range d.NameTreeSeq("Dests") {
		if err == nil {
			continue
		}
		if !strings.Contains(err.Error(), "sibling Limits overlap") {
			t.Fatalf("sibling Limits error = %v", err)
		}
		return
	}
	t.Fatal("overlapping sibling Limits produced no error")
}

func TestDestinationsSequenceReportsUnresolvedDirectRoot(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Ref{Object: 1}}, objects: map[Ref]Object{
		{Object: 1}: Dict{Name("Dests"): Ref{Object: 99}},
	}}
	for _, err := range d.DestinationsSeq() {
		if err == nil || err.Error() != "playa: catalog Dests could not be resolved" {
			t.Fatalf("unresolved direct Dests error = %v", err)
		}
		break
	}
	firstErr := d.destinationsRootErr
	if firstErr == nil || !d.destinationsRootErrReady {
		t.Fatalf("direct Dests error was not cached: ready=%v err=%v", d.destinationsRootErrReady, firstErr)
	}
	for _, err := range d.DestinationsSeq() {
		if err == nil || err != firstErr {
			t.Fatalf("cached direct Dests error = %v, want %v", err, firstErr)
		}
		return
	}
	t.Fatal("cached unresolved direct Dests produced no error")
}

func TestDestinationRootErrorCacheHonorsBudget(t *testing.T) {
	d := &Document{
		cacheOptionsConfigured: true,
		cacheOptions:           cacheconfig.Options{DestinationsRootErrorBytes: 0},
		trailer:                Dict{Name("Root"): Ref{Object: 1}},
		objects:                map[Ref]Object{{Object: 1}: Dict{Name("Dests"): Ref{Object: 99}}},
	}
	var firstErr, secondErr error
	for _, err := range d.DestinationsSeq() {
		firstErr = err
		break
	}
	for _, err := range d.DestinationsSeq() {
		secondErr = err
		break
	}
	if firstErr == nil || secondErr == nil || firstErr == secondErr {
		t.Fatalf("destination root errors = %v/%v, want uncached independent errors", firstErr, secondErr)
	}
	if d.destinationsRootErrReady {
		t.Fatal("destination root error was retained with zero budget")
	}
}

func TestNameTreeDoesNotRevisitSharedIndirectNodes(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Ref{Object: 1}}, objects: map[Ref]Object{
		{Object: 1}: Dict{Name("Names"): Dict{Name("Dests"): Dict{Name("Kids"): Array{Ref{Object: 2}, Ref{Object: 4}}}}},
		{Object: 2}: Ref{Object: 3},
		{Object: 3}: Dict{Name("Names"): Array{String("shared"), Number(1)}},
		{Object: 4}: Ref{Object: 3},
	}}
	var entries []NameTreeEntry
	for entry, err := range d.NameTreeSeq("Dests") {
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, entry)
	}
	if len(entries) != 1 || entries[0].Name() != "shared" {
		t.Fatalf("shared name-tree entries = %#v", entries)
	}
}

func TestDestinationsSequencePrefersDirectDictionaryAndStopsEarly(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("Dests"): Dict{Name("direct"): Number(1)}},
			{Object: 2, Generation: 0}: Dict{Name("Names"): Array{String("first"), Number(2), String("second"), Number(3)}},
		},
	}
	d.objects[Ref{Object: 1, Generation: 0}] = Dict{Name("Dests"): Dict{Name("direct"): Number(1)}, Name("Names"): Dict{Name("Dests"): Ref{Object: 2, Generation: 0}}}

	var got []string
	for entry, err := range d.DestinationsSeq() {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, entry.Name())
		if len(got) == 2 {
			break
		}
	}
	if want := []string{"direct"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("destination sequence = %#v, want %#v", got, want)
	}
}

func TestDestinationsSequenceIsRepeatable(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("Dests"): Dict{Name("b"): Number(2), Name("a"): Number(1)}},
		},
	}
	collect := func() []string {
		var names []string
		for entry, err := range d.DestinationsSeq() {
			if err != nil {
				t.Fatal(err)
			}
			names = append(names, entry.Name())
		}
		return names
	}
	first, second := collect(), collect()
	if len(first) != 2 || first[0] != "a" || first[1] != "b" || len(second) != len(first) || second[0] != first[0] || second[1] != first[1] {
		t.Fatalf("destination traversals = %#v and %#v", first, second)
	}
}

func TestDestinationsSequenceDoesNotDoubleCountCachedEntries(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("Dests"): Dict{Name("chapter"): String("target")}},
		},
	}
	for range d.DestinationsSeq() {
	}
	firstBytes := d.destinationCacheBytes
	for range d.DestinationsSeq() {
	}
	if d.destinationCacheBytes != firstBytes {
		t.Fatalf("repeated destination traversal changed cache bytes: first=%d second=%d", firstBytes, d.destinationCacheBytes)
	}
}

func TestConcurrentDestinationIteration(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("Dests"): Dict{Name("a"): Number(1), Name("b"): Number(2)}},
		},
	}
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
			for entry, err := range d.DestinationsSeq() {
				if err != nil {
					errs <- err
					return
				}
				if entry.Name() == "" {
					errs <- fmt.Errorf("empty destination name")
					return
				}
				count++
			}
			if count != 2 {
				errs <- fmt.Errorf("destination entry count = %d", count)
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

func TestDestinationsSequencePopulatesIncrementalCache(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Names"): Dict{Name("Dests"): Ref{Object: 2}}},
			{Object: 2}: Dict{Name("Names"): Array{String("chapter"), Array{Number(1), Name("Fit")}}},
		},
	}
	for entry, err := range d.DestinationsSeq() {
		if err != nil || entry.Name() != "chapter" {
			t.Fatalf("destination = %#v, err = %v", entry, err)
		}
		break
	}
	if _, ok := d.destinationCache["chapter"]; !ok {
		t.Fatal("destination sequence did not populate incremental cache")
	}
	delete(d.objects, Ref{Object: 2})
	got := d.ResolveDestination(String("chapter"))
	if got == nil || got.View() != "Fit" {
		t.Fatalf("cached destination = %#v", got)
	}
}

func TestDestinationsReturnsIndependentCachedMaps(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("Dests"): Dict{Name("chapter"): Array{Number(1), Name("Fit")}}},
		},
	}
	first, err := d.Destinations()
	if err != nil {
		t.Fatal(err)
	}
	delete(first, "chapter")
	second, err := d.Destinations()
	if err != nil {
		t.Fatal(err)
	}
	if second["chapter"] == nil {
		t.Fatalf("destination cache was exposed: %#v", second)
	}
	if !d.destinationsReady {
		t.Fatal("destination cache was not marked ready")
	}
}

func TestDestinationsIgnoresMalformedNameTreeWhenDirectDictionaryExists(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{
				Name("Dests"): Dict{Name("direct"): Array{Number(1), Name("Fit")}},
				Name("Names"): Dict{Name("Dests"): Dict{Name("Names"): Array{String("only-key")}}},
			},
		},
	}
	if got, err := d.Destinations(); err != nil || len(got) != 1 || got["direct"] == nil {
		t.Fatalf("destinations with malformed unused name tree = %#v, err = %v", got, err)
	}
}

func TestMalformedNameTreeDoesNotAffectDirectDestinationCache(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{
				Name("Dests"): Dict{Name("direct"): Array{Number(1), Name("Fit")}},
				Name("Names"): Dict{Name("Dests"): Dict{Name("Names"): Array{String("only-key")}}},
			},
		},
	}
	if got, err := d.Destinations(); err != nil || len(got) != 1 || got["direct"] == nil {
		t.Fatalf("destinations with malformed unused name tree = %#v, err = %v", got, err)
	}
	destination := d.ResolveDestination(String("direct"))
	if destination == nil || destination.View() != "Fit" {
		t.Fatalf("valid direct destination was suppressed after malformed tree: %#v", destination)
	}
	if !d.destinationsReady || d.destinationsErr != nil {
		t.Fatalf("direct destination cache state = ready=%v err=%v", d.destinationsReady, d.destinationsErr)
	}
}

func TestMalformedNameTreeDoesNotHideCachedNamedDestinationError(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Names"): Dict{Name("Dests"): Dict{Name("Names"): Array{
				String("valid"), Array{Number(0), Name("Fit")}, String("broken"),
			}}}},
		},
	}
	for _, err := range d.DestinationsSeq() {
		if err != nil {
			break
		}
	}
	if _, err := d.ResolveDestinationWithError(String("valid")); err == nil {
		t.Fatal("cached named destination hid malformed name-tree error")
	}
}

func TestResolveDestinationWithErrorReportsMalformedNameTree(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Names"): Dict{Name("Dests"): Dict{Name("Names"): Array{String("only-key")}}}},
		},
	}
	if got, err := d.ResolveDestinationWithError(String("missing")); err == nil || got != nil {
		t.Fatalf("malformed named destination = %#v, err = %v", got, err)
	}
	if got := d.ResolveDestination(String("missing")); got != nil {
		t.Fatalf("compatibility destination = %#v, want nil", got)
	}
}

func TestDestinationsDoNotShareCachedObjects(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("Dests"): Dict{Name("chapter"): Dict{Name("Target"): String("original")}}},
		},
	}
	first, err := d.Destinations()
	if err != nil {
		t.Fatal(err)
	}
	first["chapter"].(Dict)[Name("Target")] = String("changed")
	second, err := d.Destinations()
	if err != nil {
		t.Fatal(err)
	}
	value, _ := second["chapter"].(Dict)[Name("Target")].(String)
	if string(value) != "original" {
		t.Fatalf("destination object cache was exposed: %#v", second["chapter"])
	}
}

func TestDestinationCacheDoesNotRetainOversizedValues(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("Dests"): Dict{Name("large"): String(make([]byte, destinationCacheLimit+1))}},
		},
	}
	value := d.destinationByName("large")
	if value == nil {
		t.Fatal("oversized destination was not resolved")
	}
	if len(d.destinationCache) != 0 || d.destinationCacheBytes != 0 {
		t.Fatalf("oversized destination entered cache: entries=%d bytes=%d", len(d.destinationCache), d.destinationCacheBytes)
	}
}

func TestDestinationCacheRejectsAccountingOverflow(t *testing.T) {
	d := &Document{
		destinationCache:      map[string]Object{},
		destinationCacheBytes: math.MaxInt - 1,
	}
	d.cacheDestination("new", Number(1))
	if _, ok := d.destinationCache["new"]; ok {
		t.Fatal("destination cache accepted an overflowing byte counter")
	}
	if d.destinationCacheBytes != math.MaxInt-1 {
		t.Fatalf("destination cache byte counter changed: got=%d", d.destinationCacheBytes)
	}
}

func TestDestinationsDoesNotRetainOversizedMaterializedResults(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("Dests"): Dict{Name("large"): String(make([]byte, destinationCacheLimit+1))}},
		},
	}
	got, err := d.Destinations()
	if err != nil || len(got) != 1 || got["large"] == nil {
		t.Fatalf("materialized destination = %#v, err = %v", got, err)
	}
	if d.destinationsCacheable || d.destinationsCache != nil || d.destinationsCacheBytes != 0 {
		t.Fatalf("oversized materialized destination entered cache: cacheable=%v cache=%v bytes=%d", d.destinationsCacheable, d.destinationsCache, d.destinationsCacheBytes)
	}
}

func TestNameTreeSequenceWalksNamedTreeLazily(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("Names"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("JavaScript"): Ref{Object: 3, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("Names"): Array{String("first"), Number(1), String("second"), Number(2)}},
		},
	}
	var names []string
	for entry, err := range d.NameTreeSeq("JavaScript") {
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, entry.Name())
		break
	}
	if len(names) != 1 || names[0] != "first" {
		t.Fatalf("name tree entries = %#v", names)
	}
	if d.nameTreeReady["JavaScript"] {
		t.Fatal("partial name-tree traversal was cached")
	}
	var all []string
	for entry, err := range d.NameTreeSeq("JavaScript") {
		if err != nil {
			t.Fatal(err)
		}
		all = append(all, entry.Name())
	}
	if len(all) != 2 || all[0] != "first" || all[1] != "second" {
		t.Fatalf("complete name tree = %#v", all)
	}
	if !d.nameTreeReady["JavaScript"] || len(d.nameTreeCache["JavaScript"]) != 2 {
		t.Fatalf("name tree cache = %#v", d.nameTreeCache)
	}
	delete(d.objects, Ref{Object: 3, Generation: 0})
	var cached []string
	for entry, err := range d.NameTreeSeq("JavaScript") {
		if err != nil {
			t.Fatal(err)
		}
		cached = append(cached, entry.Name())
	}
	if len(cached) != 2 || cached[0] != "first" || cached[1] != "second" {
		t.Fatalf("cached name tree = %#v", cached)
	}
}

func TestNameTreeSequenceDoesNotCacheOversizedValues(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("Names"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("JavaScript"): Ref{Object: 3, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("Names"): Array{String("script"), String(make([]byte, nameTreeCacheLimit+1))}},
		},
	}
	for entry, err := range d.NameTreeSeq("JavaScript") {
		if err != nil {
			t.Fatal(err)
		}
		if entry.Name() != "script" {
			t.Fatalf("name-tree entry = %#v", entry)
		}
	}
	if d.nameTreeReady["JavaScript"] {
		t.Fatal("oversized name-tree value entered cache")
	}
}

func TestNameTreeSequenceCachesTerminalErrors(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("Names"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("JavaScript"): Ref{Object: 3, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("Names"): Array{String("broken")}},
		},
	}
	var firstErr error
	for _, err := range d.NameTreeSeq("JavaScript") {
		if err != nil {
			firstErr = err
			break
		}
	}
	if firstErr == nil {
		t.Fatal("malformed name tree produced no error")
	}
	d.objects[Ref{Object: 3, Generation: 0}] = Dict{Name("Names"): Array{String("fixed"), Number(1)}}
	var secondErr error
	for _, err := range d.NameTreeSeq("JavaScript") {
		if err != nil {
			secondErr = err
			break
		}
	}
	if secondErr == nil || secondErr.Error() != firstErr.Error() {
		t.Fatalf("cached name-tree error = %v, want %v", secondErr, firstErr)
	}
}

func TestNameTreeErrorCacheHasBoundedBytes(t *testing.T) {
	const attempts = 4096
	root := Dict{}
	for i := 0; i < attempts; i++ {
		root[Name(fmt.Sprintf("Tree%d", i))] = Ref{Object: 3}
	}
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("Names"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: root,
			{Object: 3, Generation: 0}: Dict{Name("Names"): Array{String("broken")}},
		},
	}
	for i := 0; i < attempts; i++ {
		for _, err := range d.NameTreeSeq(fmt.Sprintf("Tree%d", i)) {
			if err == nil {
				t.Fatalf("name tree %d unexpectedly succeeded", i)
			}
			break
		}
	}
	if d.nameTreeErrorBytes == 0 || d.nameTreeErrorBytes > nameTreeErrorCacheLimit {
		t.Fatalf("name-tree error cache bytes = %d, want a positive bounded value", d.nameTreeErrorBytes)
	}
}

func TestNameTreeErrorCacheHonorsBudget(t *testing.T) {
	d := &Document{
		cacheOptionsConfigured: true,
		cacheOptions:           cacheconfig.Options{NameTreeErrorBytes: 0},
		trailer:                Dict{Name("Root"): Dict{Name("Names"): Dict{Name("Broken"): Dict{Name("Names"): Array{String("broken")}}}}},
	}
	var firstErr, secondErr error
	for _, err := range d.NameTreeSeq("Broken") {
		firstErr = err
		break
	}
	for _, err := range d.NameTreeSeq("Broken") {
		secondErr = err
		break
	}
	if firstErr == nil || secondErr == nil || firstErr == secondErr {
		t.Fatalf("name-tree errors = %v/%v, want uncached independent errors", firstErr, secondErr)
	}
	if len(d.nameTreeErrors) != 0 {
		t.Fatalf("name-tree error cache retained entry with zero budget: %#v", d.nameTreeErrors)
	}
}

func TestNameTreeCacheUsesConfiguredBudgetAboveDefault(t *testing.T) {
	d := &Document{
		cacheOptions:           cacheconfig.Options{NameTreeBytes: nameTreeCacheLimit + 4096},
		cacheOptionsConfigured: true,
		trailer:                Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("Names"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("JavaScript"): Ref{Object: 3, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("Names"): Array{String("script"), String(make([]byte, nameTreeCacheLimit+1))}},
		},
	}
	for _, err := range d.NameTreeSeq("JavaScript") {
		if err != nil {
			t.Fatal(err)
		}
	}
	if !d.nameTreeReady["JavaScript"] {
		t.Fatal("configured name-tree budget was not applied")
	}
}

func TestConcurrentNameTreeIteration(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("Names"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("JavaScript"): Ref{Object: 3, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("Names"): Array{String("first"), Number(1), String("second"), Number(2)}},
		},
	}
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
			for entry, err := range d.NameTreeSeq("JavaScript") {
				if err != nil {
					errs <- err
					return
				}
				if entry.Name() == "" {
					errs <- fmt.Errorf("empty name-tree entry")
					return
				}
				count++
			}
			if count != 2 {
				errs <- fmt.Errorf("name-tree entry count = %d", count)
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

func TestNameTreeFinalizeDoesNotShareCachedObjects(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("Names"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("JavaScript"): Ref{Object: 3, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("Names"): Array{String("script"), Dict{Name("Code"): String("original")}}},
		},
	}
	for _, err := range d.NameTreeSeq("JavaScript") {
		if err != nil {
			t.Fatal(err)
		}
	}
	for entry, err := range d.NameTreeSeq("JavaScript") {
		if err != nil {
			t.Fatal(err)
		}
		snapshot := entry.Finalize()
		copyValue := snapshot.ValueCopy().(Dict)
		copyValue[Name("Code")] = String("changed")
	}
	for entry, err := range d.NameTreeSeq("JavaScript") {
		if err != nil {
			t.Fatal(err)
		}
		value, _ := entry.ValueCopy().(Dict)[Name("Code")].(String)
		if string(value) != "original" {
			t.Fatalf("name-tree object cache was exposed: %#v", entry.ValueCopy())
		}
	}
}

func TestDestinationsRejectsNameTreeCycle(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("Names"): Ref{Object: 2, Generation: 0}},
			{Object: 2, Generation: 0}: Dict{Name("Dests"): Ref{Object: 3, Generation: 0}},
			{Object: 3, Generation: 0}: Dict{Name("Kids"): Array{Ref{Object: 3, Generation: 0}}},
		},
	}
	if got, err := d.Destinations(); err != nil || len(got) != 0 {
		t.Fatalf("destinations from cyclic tree = %#v, err = %v", got, err)
	}
}

func TestNameTreeSequenceReportsMalformedNodes(t *testing.T) {
	tests := []struct {
		name  string
		tree  Dict
		child map[Ref]Object
		want  string
	}{
		{name: "odd Names", tree: Dict{Name("Names"): Array{String("only-key")}}, want: "unmatched key"},
		{name: "non-string key", tree: Dict{Name("Names"): Array{Number(1), Number(2)}}, want: "key 0 is not a string"},
		{name: "non-array Kids", tree: Dict{Name("Kids"): Number(1)}, want: "Kids is not an array"},
		{name: "non-dictionary child", tree: Dict{Name("Kids"): Array{Number(1)}}, want: "child is not a dictionary"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			objects := map[Ref]Object{{Object: 1, Generation: 0}: Dict{
				Name("Names"): Dict{Name("Dests"): test.tree},
			}}
			for ref, object := range test.child {
				objects[ref] = object
			}
			d := &Document{
				trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
				objects: objects,
			}
			for _, err := range d.NameTreeSeq("Dests") {
				if err == nil {
					t.Fatal("malformed name tree produced a value")
				}
				if !strings.Contains(err.Error(), test.want) {
					t.Fatalf("error = %v, want substring %q", err, test.want)
				}
				return
			}
			t.Fatal("malformed name tree produced no error")
		})
	}
}

func TestResolveDestinationKeepsExistingPageReference(t *testing.T) {
	d := &Document{objects: map[Ref]Object{{Object: 9, Generation: 0}: Dict{Name("Type"): Name("Page")}}}
	got := d.ResolveDestination(Array{Ref{Object: 9, Generation: 0}, Name("Fit")})
	if got == nil {
		t.Fatalf("destination = %#v", got)
	}
	page, hasPage := got.PageRef()
	if !hasPage || page != (Ref{Object: 9, Generation: 0}) || got.View() != "Fit" {
		t.Fatalf("destination = %#v", got)
	}
}

func TestDestinationEntryResolvesValue(t *testing.T) {
	d := &Document{}
	entry := newDestinationEntry("chapter", Array{Number(1), Name("Fit")})
	got := entry.Destination(d)
	if got == nil {
		t.Fatalf("destination = %#v", got)
	}
	index, hasIndex := got.PageIndex()
	if !hasIndex || index != 0 || got.View() != "Fit" {
		t.Fatalf("destination = %#v", got)
	}
	if (DestinationEntry{}).Destination(nil) != nil {
		t.Fatal("nil document should not resolve destination")
	}
}

func TestDestinationEntryWithErrorReportsMalformedValue(t *testing.T) {
	d := &Document{}
	entry := newDestinationEntry("broken", Array{Number(1), Name("FitH")})
	if destination, err := entry.DestinationWithError(d); err == nil || destination != nil {
		t.Fatalf("destination = %#v, err = %v", destination, err)
	}
	if destination := entry.Destination(d); destination != nil {
		t.Fatalf("compatibility destination = %#v", destination)
	}
}

func TestResolveDestinationStopsAfterNamedDestinationMatch(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1, Generation: 0}},
		objects: map[Ref]Object{
			{Object: 1, Generation: 0}: Dict{Name("Names"): Dict{Name("Dests"): Ref{Object: 2, Generation: 0}}},
			{Object: 2, Generation: 0}: Dict{Name("Names"): Array{
				String("target"), Array{Number(1), Name("Fit")},
				String("later"), Array{Number(2), Name("Fit")},
			}},
		},
	}
	got := d.ResolveDestination(String("target"))
	if got == nil {
		t.Fatalf("destination = %#v", got)
	}
	index, hasIndex := got.PageIndex()
	if !hasIndex || index != 0 {
		t.Fatalf("destination = %#v", got)
	}
}

func TestResolveDestinationAcceptsOneBasedPageNumber(t *testing.T) {
	d := &Document{}
	got := d.ResolveDestination(Array{Number(1), Name("FitH"), Number(846)})
	if got == nil {
		t.Fatalf("destination = %#v", got)
	}
	index, hasIndex := got.PageIndex()
	if !hasIndex || index != 0 || got.View() != "FitH" || len(got.ParamsCopy()) != 1 {
		t.Fatalf("destination = %#v", got)
	}
}

func TestResolveDestinationRejectsMalformedKnownViewParameters(t *testing.T) {
	d := &Document{}
	for _, destination := range []Object{
		Array{Number(1), Name("FitR"), Number(0), Number(0), Number(600)},
		Array{Number(1), Name("FitH")},
		Array{Number(1), Name("XYZ"), Number(0), Number(800), Number(1), Number(99)},
	} {
		if got := d.ResolveDestination(destination); got != nil {
			t.Fatalf("malformed destination %v resolved to %#v", destination, got)
		}
	}
}

func TestResolveDestinationDefaultsInvalidPageObjectToFirstPage(t *testing.T) {
	d := &Document{}
	got := d.ResolveDestination(Array{Null{}, Number(0), Number(0), Number(1)})
	if got == nil {
		t.Fatalf("destination = %#v", got)
	}
	index, hasIndex := got.PageIndex()
	if !hasIndex || index != 0 || got.View() != "" || len(got.ParamsCopy()) != 2 {
		t.Fatalf("destination = %#v", got)
	}
}
