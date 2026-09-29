package document

import (
	"bytes"
	"fmt"
	"math"
	"sync"
	"testing"

	"github.com/lin-string/go-playa/cacheconfig"
)

func TestActionRawCopyDoesNotExposeSource(t *testing.T) {
	action := newActionWithRaw(Dict{Name("Meta"): Dict{Name("Value"): String("original")}})
	copy := action.RawCopy()
	copy[Name("Meta")].(Dict)[Name("Value")] = String("changed")
	if got := action.RawCopy()[Name("Meta")].(Dict)[Name("Value")].(String); string(got) != "original" {
		t.Fatalf("action raw copy aliases source: %q", got)
	}
}

func TestResolveActionNormalizesCommonPDFActions(t *testing.T) {
	d := &Document{}
	action := d.ResolveAction(Dict{
		Name("S"): Name("GoToR"),
		Name("D"): Array{Number(2), Name("Fit")},
		Name("F"): String("other.pdf"),
	})
	index, hasIndex := 0, false
	if action != nil && action.destination != nil {
		index, hasIndex = action.destination.PageIndex()
	}
	if action == nil || action.Kind() != "GoToR" || action.destination == nil || !hasIndex || index != 1 || action.File() != "other.pdf" {
		t.Fatalf("action = %#v", action)
	}
}

func TestResolveActionRejectsMalformedSubtype(t *testing.T) {
	d := &Document{}
	for _, value := range []Object{
		Dict{Name("D"): Array{Number(1), Name("Fit")}},
		Dict{Name("S"): String("GoTo")},
	} {
		if action := d.ResolveAction(value); action != nil {
			t.Fatalf("malformed action %v resolved to %#v", value, action)
		}
	}
}

func TestResolveActionWithErrorReportsMalformedRoot(t *testing.T) {
	d := &Document{}
	for _, value := range []Object{
		String("invalid"),
		Dict{Name("D"): Array{Number(1), Name("Fit")}},
		Dict{Name("S"): Name("GoTo"), Name("D"): Array{Number(1), Name("FitH")}},
	} {
		if action, err := d.ResolveActionWithError(value); err == nil || action != nil {
			t.Fatalf("malformed action = %#v, err = %v", action, err)
		}
	}
}

func TestResolveActionWithErrorKeepsNextLazy(t *testing.T) {
	d := &Document{}
	action, err := d.ResolveActionWithError(Dict{
		Name("S"):    Name("Named"),
		Name("N"):    Name("NextPage"),
		Name("Next"): String("invalid"),
	})
	if err != nil || action == nil {
		t.Fatalf("action = %#v, err = %v", action, err)
	}
	if action.nextReady {
		t.Fatal("ResolveActionWithError eagerly materialized /Next")
	}
}

func TestResolveActionWithErrorReportsUnresolvedURI(t *testing.T) {
	d := &Document{}
	if action, err := d.ResolveActionWithError(Dict{Name("S"): Name("URI"), Name("URI"): Ref{Object: 99}}); err == nil || action != nil {
		t.Fatalf("unresolved URI action = %#v, err=%v", action, err)
	}
}

func TestResolveActionWithErrorReportsMalformedJavaScriptStream(t *testing.T) {
	d := &Document{}
	value := Dict{
		Name("S"):  Name("JavaScript"),
		Name("JS"): newStream(Dict{Name("Filter"): Name("FlateDecode")}, []byte("not a flate stream")),
	}
	if action, err := d.ResolveActionWithError(value); err == nil || action != nil {
		t.Fatalf("malformed JavaScript stream action = %#v, err=%v", action, err)
	}
	action := d.ResolveAction(value)
	if action == nil {
		t.Fatal("lenient action resolution dropped malformed JavaScript stream")
	}
	if script, err := action.ScriptWithError(); err == nil || script != "" {
		t.Fatalf("ScriptWithError = %q, err=%v", script, err)
	}
	if _, err := action.FinalizeWithError(); err == nil {
		t.Fatal("FinalizeWithError hid malformed JavaScript stream")
	}
}

func TestDestinationsReportsMalformedDirectRoot(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Dict{Name("Dests"): Number(1)}}}
	for _, err := range d.DestinationsSeq() {
		if err == nil {
			t.Fatal("malformed direct destinations root produced an entry")
		}
		return
	}
	t.Fatal("malformed direct destinations root produced no error")
}

func TestDestinationsReportsMalformedCatalogRoot(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Number(1)}}
	for _, err := range d.DestinationsSeq() {
		if err == nil {
			t.Fatal("malformed catalog root produced a destination")
		}
		return
	}
	t.Fatal("malformed catalog root produced no error")
}

func TestNameTreeReportsMalformedNamesContainer(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Dict{Name("Names"): Number(1)}}}
	for _, err := range d.NameTreeSeq("Dests") {
		if err == nil {
			t.Fatal("malformed Names container produced an entry")
		}
		return
	}
	t.Fatal("malformed Names container produced no error")
}

func TestResolveActionResolvesIndirectFileSpecificationFields(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{Name("UF"): Ref{Object: 2}},
		{Object: 2}: String("remote.pdf"),
	}}
	action := d.ResolveAction(Dict{
		Name("S"): Name("GoToR"),
		Name("F"): Ref{Object: 1},
		Name("D"): Array{Number(1), Name("Fit")},
	})
	if action == nil || action.File() != "remote.pdf" {
		t.Fatalf("indirect file specification = %#v", action)
	}
}

func TestResolveActionFollowsMultiLevelParametersAndNext(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Ref{Object: 2},
		{Object: 2}: Array{Ref{Object: 3}, Ref{Object: 4}, Ref{Object: 6}},
		{Object: 3}: Number(2), {Object: 4}: Name("FitH"),
		{Object: 6}: Ref{Object: 7}, {Object: 7}: Number(720),
		{Object: 8}: Dict{Name("S"): Ref{Object: 9}, Name("N"): Ref{Object: 10}},
		{Object: 9}: Name("Named"), {Object: 10}: Name("NextPage"),
	}}
	a := d.ResolveAction(Dict{Name("S"): Name("GoTo"), Name("D"): Ref{Object: 1}, Name("Next"): Ref{Object: 8}})
	if a == nil || a.destination == nil || a.destination.View() != "FitH" || len(a.destination.ParamsCopy()) != 1 {
		t.Fatalf("indirect action destination = %#v", a)
	}
	if _, err := a.FinalizeWithError(); err != nil {
		t.Fatalf("indirect action next chain = %v", err)
	}
	next, err := a.NextCopy()
	if err != nil || len(next) != 1 || next[0].Kind() != "Named" {
		t.Fatalf("indirect action next = %#v, err=%v", next, err)
	}
}

func TestResolveActionFollowsMultiLevelIndirectNextReference(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Ref{Object: 2},
		{Object: 2}: Dict{Name("S"): Name("Named"), Name("N"): Name("NextPage")},
	}}
	a := d.ResolveAction(Dict{Name("S"): Name("Named"), Name("N"): Name("FirstPage"), Name("Next"): Ref{Object: 1}})
	if a == nil {
		t.Fatal("multi-level Next action was not resolved")
	}
	next, err := a.NextCopy()
	if err != nil || len(next) != 1 || next[0].Kind() != "Named" || next[0].Name() != "NextPage" {
		t.Fatalf("multi-level Next action = %#v, err=%v", next, err)
	}
}

func TestResolveActionReportsUnresolvedIndirectNextEntry(t *testing.T) {
	a := (&Document{}).ResolveAction(Dict{
		Name("S"):    Name("Named"),
		Name("N"):    Name("FirstPage"),
		Name("Next"): Ref{Object: 99},
	})
	if a == nil {
		t.Fatal("missing root action")
	}
	for _, err := range a.NextSeq() {
		if err == nil || err.Error() != "playa: /Next entry could not be resolved" {
			t.Fatalf("unresolved /Next error = %v", err)
		}
		return
	}
	t.Fatal("unresolved /Next entry produced no error")
}

func TestResolveActionReportsUnresolvedIndirectNextArray(t *testing.T) {
	a := (&Document{}).ResolveAction(Dict{
		Name("S"):    Name("Named"),
		Name("N"):    Name("FirstPage"),
		Name("Next"): Ref{Object: 99},
	})
	if a == nil {
		t.Fatal("missing root action")
	}
	if _, err := a.FinalizeWithError(); err == nil || err.Error() != "playa: /Next entry could not be resolved" {
		t.Fatalf("unresolved /Next finalize error = %v", err)
	}
}

func TestResolveActionHandlesNamedAndJavaScriptActions(t *testing.T) {
	d := &Document{}
	named := d.ResolveAction(Dict{Name("S"): Name("Named"), Name("N"): Name("NextPage")})
	if named == nil || named.Kind() != "Named" || named.Name() != "NextPage" {
		t.Fatalf("named action = %#v", named)
	}
	javascript := d.ResolveAction(Dict{Name("S"): Name("JavaScript"), Name("JS"): String("app.alert('x')")})
	if javascript == nil || javascript.Kind() != "JavaScript" || javascript.Script() != "app.alert('x')" {
		t.Fatalf("javascript action = %#v", javascript)
	}
	streamScript := d.ResolveAction(Dict{Name("S"): Name("JavaScript"), Name("JS"): newStream(nil, []byte("app.alert('stream')"))})
	if streamScript == nil || streamScript.Script() != "app.alert('stream')" {
		t.Fatalf("stream javascript action = %#v", streamScript)
	}
}

func TestResolveActionPreservesNextActionChain(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{Name("S"): Name("Named"), Name("N"): Name("NextPage")},
	}}
	action := d.ResolveAction(Dict{
		Name("S"):   Name("URI"),
		Name("URI"): String("https://example.com"),
		Name("Next"): Array{
			Ref{Object: 1},
			Dict{Name("S"): Name("JavaScript"), Name("JS"): String("next()")},
		},
	})
	if action == nil || action.nextReady || action.nextRaw == nil {
		t.Fatalf("action next chain was not lazy = %#v", action)
	}
	count := 0
	for next, err := range action.NextSeq() {
		if err != nil {
			t.Fatal(err)
		}
		count++
		if (count == 1 && next.Kind() != "Named") || (count == 2 && next.Script() != "next()") {
			t.Fatalf("NextSeq item %d = %#v", count, next)
		}
	}
	if count != 2 {
		t.Fatalf("NextSeq count = %d, want 2", count)
	}
}

func TestResolveActionStopsCyclicNextChain(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{Name("S"): Name("Named"), Name("N"): Name("NextPage"), Name("Next"): Ref{Object: 1}},
	}}
	action := d.ResolveAction(Ref{Object: 1})
	if action == nil || action.nextReady {
		t.Fatalf("cyclic action chain = %#v", action)
	}
	copy, err := action.NextCopy()
	if err != nil {
		t.Fatal(err)
	}
	if count := len(copy); count != 0 {
		t.Fatalf("cyclic action next count = %d", count)
	}
}

func TestResolveActionStopsMultiLevelCyclicNextChain(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{Name("S"): Name("Named"), Name("N"): Name("NextPage"), Name("Next"): Ref{Object: 2}},
		{Object: 2}: Ref{Object: 1},
	}}
	a := d.ResolveAction(Ref{Object: 1})
	if a == nil {
		t.Fatal("root action was not resolved")
	}
	next, err := a.NextCopy()
	if err != nil {
		t.Fatalf("multi-level cyclic Next chain returned an error: %v", err)
	}
	if len(next) != 0 {
		t.Fatalf("multi-level cyclic Next chain yielded %d actions", len(next))
	}
}

func TestResolveActionReportsMalformedNextChain(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{Name("S"): Name("Named"), Name("N"): Name("NextPage"), Name("Next"): Ref{Object: 2}},
		{Object: 2}: String("invalid action"),
	}}
	action := d.ResolveAction(Ref{Object: 1})
	if action == nil {
		t.Fatal("missing root action")
	}
	if _, err := action.MarshalJSON(); err == nil {
		t.Fatal("malformed action JSON produced no error")
	}
	if snapshot, err := action.FinalizeWithError(); err == nil || snapshot.Kind() != "" {
		t.Fatalf("malformed action finalize = %#v, err=%v", snapshot, err)
	}
	if next, err := action.NextCopy(); err == nil || next != nil {
		t.Fatalf("malformed action next copy = %#v, err=%v", next, err)
	}
	count := 0
	for next, err := range action.NextSeq() {
		if err != nil {
			if count != 0 {
				t.Fatalf("malformed action reported after %d items", count)
			}
			return
		}
		count++
		if next == nil {
			t.Fatal("nil action without error")
		}
	}
	t.Fatal("malformed action chain produced no error")
}

func TestResolveActionReportsMalformedNextAfterValidItems(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{Name("S"): Name("Named"), Name("N"): Name("NextPage"), Name("Next"): Array{
			Dict{Name("S"): Name("Named"), Name("N"): Name("PrevPage")},
			String("invalid action"),
		}},
	}}
	action := d.ResolveAction(Ref{Object: 1})
	if action == nil {
		t.Fatal("missing root action")
	}
	var names []string
	var gotErr error
	for next, err := range action.NextSeq() {
		if err != nil {
			gotErr = err
			break
		}
		names = append(names, next.Name())
	}
	if len(names) != 1 || names[0] != "PrevPage" || gotErr == nil {
		t.Fatalf("next results = %v, err=%v", names, gotErr)
	}
}

func TestResolveActionNextSequenceDefersLaterArrayEntries(t *testing.T) {
	d := &Document{}
	action := d.ResolveAction(Dict{
		Name("S"):   Name("URI"),
		Name("URI"): String("https://example.com"),
		Name("Next"): Array{
			Dict{Name("S"): Name("Named"), Name("N"): Name("PrevPage")},
			String("invalid action"),
		},
	})
	if action == nil {
		t.Fatal("missing root action")
	}
	for next, err := range action.NextSeq() {
		if err != nil || next == nil || next.Name() != "PrevPage" {
			t.Fatalf("first lazy Next item = %#v, err=%v", next, err)
		}
		break
	}
	if len(action.next) != 1 || action.nextErr != nil || action.nextReady {
		t.Fatalf("later Next entry was resolved before demand: next=%d err=%v ready=%v", len(action.next), action.nextErr, action.nextReady)
	}
	var gotErr error
	for _, err := range action.NextSeq() {
		if err != nil {
			gotErr = err
			break
		}
	}
	if gotErr == nil {
		t.Fatal("deferred malformed Next entry produced no error")
	}
}

func TestResolveActionNextCopyReportsNestedMalformedChain(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{Name("S"): Name("Named"), Name("N"): Name("Start"), Name("Next"): Ref{Object: 2}},
		{Object: 2}: Dict{Name("S"): Name("Named"), Name("N"): Name("Child"), Name("Next"): Ref{Object: 3}},
		{Object: 3}: String("invalid action"),
	}}
	action := d.ResolveAction(Ref{Object: 1})
	if action == nil {
		t.Fatal("missing root action")
	}
	if next, err := action.NextCopy(); err == nil || next != nil {
		t.Fatalf("nested malformed action next copy = %#v, err=%v", next, err)
	}
}

func TestActionFinalizeDropsLazyDocumentState(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{Name("S"): Name("Named"), Name("N"): Name("NextPage"), Name("Next"): Ref{Object: 2}},
		{Object: 2}: Dict{Name("S"): Name("Named"), Name("N"): Name("PrevPage")},
	}}
	action := d.ResolveAction(Ref{Object: 1})
	if action == nil || action.nextReady {
		t.Fatalf("action was not lazy: %#v", action)
	}
	snapshot := action.Finalize()
	if snapshot.document != nil || snapshot.nextRaw != nil || !snapshot.nextReady || len(snapshot.next) != 1 {
		t.Fatalf("finalized action retained lazy state: %#v", snapshot)
	}
	if snapshot.next[0].document != nil || snapshot.next[0].nextRaw != nil {
		t.Fatalf("finalized child retained lazy state: %#v", snapshot.next[0])
	}
}

func TestActionValueCopiesDropLazyDocumentState(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{Name("S"): Name("Named"), Name("N"): Name("NextPage")},
	}}
	action := d.ResolveAction(Ref{Object: 1})
	annotation := Annotation{actionValue: action}
	annotationCopy := annotation.ActionValueCopy()
	if annotationCopy == nil || annotationCopy.document != nil || annotationCopy.nextRaw != nil {
		t.Fatalf("annotation action copy retained lazy state: %#v", annotationCopy)
	}
	outline := OutlineNode{actionValue: action}
	outlineCopy := outline.ActionValueCopy()
	if outlineCopy == nil || outlineCopy.document != nil || outlineCopy.nextRaw != nil {
		t.Fatalf("outline action copy retained lazy state: %#v", outlineCopy)
	}
}

func TestResolveActionCachesIndirectJavaScriptStreams(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 7}: newStream(nil, []byte("app.alert('cached')")),
	}}
	action := Dict{Name("S"): Name("JavaScript"), Name("JS"): Ref{Object: 7}}
	first := d.ResolveAction(action)
	second := d.ResolveAction(action)
	if first == nil || second == nil || first.Script() != "app.alert('cached')" || second.Script() != first.Script() {
		t.Fatalf("cached actions = %#v, %#v", first, second)
	}
	if len(d.actionScriptCache) != 1 {
		t.Fatalf("action script cache size = %d", len(d.actionScriptCache))
	}
}

func TestResolveActionDoesNotCacheMalformedIndirectJavaScript(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 7}: newStream(Dict{Name("Filter"): Name("FlateDecode")}, []byte("not a flate stream")),
	}}
	action := Dict{Name("S"): Name("JavaScript"), Name("JS"): Ref{Object: 7}}
	first := d.ResolveAction(action)
	if first == nil {
		t.Fatal("lenient action resolution dropped malformed JavaScript")
	}
	if _, err := first.ScriptWithError(); err == nil {
		t.Fatal("first malformed JavaScript access lost its error")
	}
	if len(d.actionScriptCache) != 0 {
		t.Fatalf("malformed JavaScript entered cache: %#v", d.actionScriptCache)
	}
	second := d.ResolveAction(action)
	if second == nil {
		t.Fatal("second lenient action resolution dropped malformed JavaScript")
	}
	if _, err := second.ScriptWithError(); err == nil {
		t.Fatal("malformed JavaScript error was hidden by an empty cache entry")
	}
}

func TestResolveActionDoesNotCacheOversizedJavaScript(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 7}: newStream(nil, bytes.Repeat([]byte{'x'}, actionScriptCacheLimit+1)),
	}}
	action := Dict{Name("S"): Name("JavaScript"), Name("JS"): Ref{Object: 7}}
	resolved := d.ResolveAction(action)
	if resolved == nil || len(resolved.Script()) != actionScriptCacheLimit+1 {
		t.Fatalf("oversized script was not returned intact: action=%#v", resolved)
	}
	if len(d.actionScriptCache) != 0 {
		t.Fatalf("oversized script entered cache: size=%d", len(d.actionScriptCache))
	}
}

func TestResolveActionDoesNotCacheOversizedAction(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 8}: Dict{Name("S"): Name("URI"), Name("URI"): String(bytes.Repeat([]byte{'x'}, actionCacheLimit+1))},
	}}
	if action := d.ResolveAction(Ref{Object: 8}); action == nil {
		t.Fatal("oversized action was not resolved")
	}
	if len(d.actionCache) != 0 || d.actionCacheBytes != 0 {
		t.Fatalf("oversized action entered cache: entries=%d bytes=%d", len(d.actionCache), d.actionCacheBytes)
	}
}

func TestResolveActionCachesIndirectActionGraphs(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{Name("S"): Name("Named"), Name("N"): Name("NextPage")},
	}}
	first := d.ResolveAction(Ref{Object: 1})
	if first == nil || first.Kind() != "Named" {
		t.Fatalf("first action = %#v", first)
	}
	d.objects[Ref{Object: 1}] = Dict{Name("S"): Name("URI"), Name("URI"): String("changed")}
	snapshot := first.Finalize()
	if snapshot.Kind() != "Named" {
		t.Fatalf("finalized action kind = %q", snapshot.Kind())
	}
	second := d.ResolveAction(Ref{Object: 1})
	if second != first || second.Kind() != "Named" {
		t.Fatalf("cached action = %#v, first = %#v", second, first)
	}
	if len(d.actionCache) != 1 {
		t.Fatalf("action cache size = %d", len(d.actionCache))
	}
}

func TestResolveActionReusesCacheAcrossIndirectReferenceChains(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Ref{Object: 2},
		{Object: 2}: Dict{Name("S"): Name("Named"), Name("N"): Name("NextPage")},
	}}
	first := d.ResolveAction(Ref{Object: 1})
	second := d.ResolveAction(Ref{Object: 2})
	if first == nil || second == nil || first != second {
		t.Fatalf("indirect action cache entries differ: first=%p second=%p", first, second)
	}
	if len(d.actionCache) != 1 {
		t.Fatalf("action cache size = %d, want 1", len(d.actionCache))
	}
}

func TestActionNextSequenceReusesCanonicalCachedGraph(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{Name("S"): Name("Named"), Name("N"): Name("NextPage"), Name("Next"): Ref{Object: 2}},
		{Object: 2}: Dict{Name("S"): Name("Named"), Name("N"): Name("End"), Name("Next"): Ref{Object: 3}},
		{Object: 3}: Dict{Name("S"): Name("Named"), Name("N"): Name("Last")},
	}}
	action := d.ResolveAction(Ref{Object: 1})
	if action == nil {
		t.Fatal("missing root action")
	}
	var names []string
	for next, err := range action.NextSeq() {
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, next.Name())
	}
	if got := fmt.Sprint(names); got != "[End]" {
		t.Fatalf("first next sequence = %s", got)
	}
	root := d.actionCache[Ref{Object: 1}]
	if root == nil || !root.nextReady || len(root.next) != 1 {
		t.Fatalf("canonical action cache = %#v", root)
	}
	copy, err := action.NextCopy()
	if err != nil {
		t.Fatal(err)
	}
	if got := len(copy); got != 1 {
		t.Fatalf("cached next copy length = %d", got)
	}
}

func TestActionAliasCopiesCanonicalNextState(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{Name("S"): Name("Named"), Name("N"): Name("Start"), Name("Next"): Ref{Object: 2}},
		{Object: 2}: Dict{Name("S"): Name("Named"), Name("N"): Name("End")},
	}}
	canonical := d.ResolveAction(Ref{Object: 1})
	if canonical == nil {
		t.Fatal("missing canonical action")
	}
	if _, err := canonical.NextCopy(); err != nil {
		t.Fatal(err)
	}
	alias := &Action{document: d, actionRef: Ref{Object: 1}, hasActionRef: true}
	alias.materializeNext()
	if !alias.nextReady || len(alias.next) != 1 || alias.next[0].Name() != "End" {
		t.Fatalf("canonical next state was not copied: ready=%v next=%#v err=%v", alias.nextReady, alias.next, alias.nextErr)
	}
}

func TestConcurrentActionResolutionAndNextMaterialization(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{Name("S"): Name("Named"), Name("N"): Name("Start"), Name("Next"): Ref{Object: 2}},
		{Object: 2}: Dict{Name("S"): Name("Named"), Name("N"): Name("End")},
	}}
	const readers = 16
	var wg sync.WaitGroup
	errs := make(chan error, readers)
	start := make(chan struct{})
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			action := d.ResolveAction(Ref{Object: 1})
			if action == nil {
				errs <- fmt.Errorf("missing action")
				return
			}
			snapshot := action.Finalize()
			count := 0
			for next, err := range snapshot.NextSeq() {
				if err != nil {
					errs <- err
					return
				}
				if next.Name() != "End" {
					errs <- fmt.Errorf("unexpected next action %q", next.Name())
					return
				}
				count++
			}
			if count != 1 {
				errs <- fmt.Errorf("next action count = %d", count)
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

func TestResolveActionCacheDoesNotShareDestinationParams(t *testing.T) {
	d := &Document{
		objects: map[Ref]Object{
			{Object: 1}: Dict{
				Name("S"): Name("GoTo"),
				Name("D"): Array{Ref{Object: 2}, Name("XYZ"), Number(10), Number(20), Number(1)},
			},
		},
	}

	first := d.ResolveAction(Ref{Object: 1})
	second := d.ResolveAction(Ref{Object: 1})
	if first == nil || second == nil || first.destination == nil || second.destination == nil {
		t.Fatal("expected cached GoTo destinations")
	}
	if len(second.destination.ParamsCopy()) != 3 {
		t.Fatalf("destination params length = %d, want 3", len(second.destination.ParamsCopy()))
	}
	snapshot := first.Finalize()
	params := snapshot.destination.ParamsCopy()
	params[0] = Number(99)
	cachedParams := second.destination.ParamsCopy()
	if value, ok := cachedParams[0].(Number); !ok || value != 10 {
		t.Fatalf("cached destination params were aliased: %v", cachedParams[0])
	}
}

func TestResolveActionCacheDoesNotShareNestedRawFields(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{
			Name("S"):    Name("Named"),
			Name("N"):    Name("NextPage"),
			Name("Meta"): Dict{Name("Value"): String("original")},
		},
	}}
	first := d.ResolveAction(Ref{Object: 1})
	if first == nil {
		t.Fatal("expected action")
	}
	snapshot := first.Finalize()
	snapshotRaw := snapshot.RawCopy()
	snapshotRaw[Name("Meta")].(Dict)[Name("Value")] = String("changed")
	second := d.ResolveAction(Ref{Object: 1})
	if second == nil {
		t.Fatal("expected cached action")
	}
	value, _ := second.RawCopy()[Name("Meta")].(Dict)[Name("Value")].(String)
	if string(value) != "original" {
		t.Fatalf("action raw dictionary was exposed: %#v", second.RawCopy())
	}
}

func TestResolveActionCacheDoesNotShareNestedDestinationParams(t *testing.T) {
	d := &Document{objects: map[Ref]Object{
		{Object: 1}: Dict{
			Name("S"): Name("GoTo"),
			Name("D"): Array{Ref{Object: 2}, Name("XYZ"), Dict{Name("Value"): String("original")}, Number(20), Number(1)},
		},
	}}
	first := d.ResolveAction(Ref{Object: 1})
	if first == nil || first.destination == nil {
		t.Fatal("expected destination action")
	}
	snapshot := first.Finalize()
	params := snapshot.destination.ParamsCopy()
	params[0].(Dict)[Name("Value")] = String("changed")
	second := d.ResolveAction(Ref{Object: 1})
	if second == nil || second.destination == nil {
		t.Fatal("expected cached destination action")
	}
	value, _ := second.destination.ParamsCopy()[0].(Dict)[Name("Value")].(String)
	if string(value) != "original" {
		t.Fatalf("destination parameter was exposed: %#v", second.destination.ParamsCopy())
	}
}

func TestResolveDestinationCachesNamedDestination(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Dict{Name("Names"): Dict{
		Name("Dests"): Dict{Name("Names"): Array{String("chapter"), Array{Number(1), Name("Fit")}}},
	}}}}
	first := d.ResolveDestination(String("chapter"))
	second := d.ResolveDestination(String("chapter"))
	if first == nil || second == nil || first.View() != "Fit" || second.View() != first.View() {
		t.Fatalf("cached destinations = %#v, %#v", first, second)
	}
	if len(d.destinationCache) != 1 {
		t.Fatalf("destination cache size = %d", len(d.destinationCache))
	}
}

func TestResolveDestinationWithErrorCachesMalformedReference(t *testing.T) {
	ref := Ref{Object: 20}
	d := &Document{objects: map[Ref]Object{ref: Dict{Name("D"): Array{Number(1), Name("FitH")}}}}
	_, firstErr := d.ResolveDestinationWithError(ref)
	_, secondErr := d.ResolveDestinationWithError(ref)
	if firstErr == nil || secondErr == nil || firstErr != secondErr {
		t.Fatalf("destination errors = %v/%v, want one cached error", firstErr, secondErr)
	}
	if d.destinationErrors[ref] != firstErr {
		t.Fatalf("cached destination error = %v, want %v", d.destinationErrors[ref], firstErr)
	}
}

func TestResolveDestinationWithErrorHonorsErrorCacheBudget(t *testing.T) {
	ref := Ref{Object: 20}
	d := &Document{
		cacheOptionsConfigured: true,
		cacheOptions:           cacheconfig.Options{DestinationErrorBytes: 0},
		objects:                map[Ref]Object{ref: Dict{Name("D"): Array{Number(1), Name("FitH")}}},
	}
	_, firstErr := d.ResolveDestinationWithError(ref)
	_, secondErr := d.ResolveDestinationWithError(ref)
	if firstErr == nil || secondErr == nil || firstErr == secondErr {
		t.Fatalf("destination errors = %v/%v, want uncached independent errors", firstErr, secondErr)
	}
	if len(d.destinationErrors) != 0 {
		t.Fatalf("destination error cache retained entry with zero budget: %#v", d.destinationErrors)
	}
}

func TestDestinationResolvesPageReferenceAndIndex(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 3}, Ref{Object: 4}}, Name("Count"): Number(2)},
			{Object: 3}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2}},
			{Object: 4}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2}},
		},
	}
	byRef, err := newDestination(Ref{Object: 4}, true, 0, false, "", nil).PageObject(d)
	if err != nil || byRef.ref != (Ref{Object: 4}) {
		t.Fatalf("reference page = %#v, err = %v", byRef, err)
	}
	byIndex, err := newDestination(Ref{}, false, 1, true, "", nil).PageObject(d)
	if err != nil || byIndex.ref != (Ref{Object: 4}) {
		t.Fatalf("index page = %#v, err = %v", byIndex, err)
	}
}

func TestDestinationExposesViewGeometry(t *testing.T) {
	d := &Document{
		space:   CoordinateSpacePage,
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 3}}, Name("Count"): Number(1)},
			{Object: 3}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2}, Name("MediaBox"): Array{Number(10), Number(20), Number(110), Number(220)}},
		},
	}
	destination := newDestination(Ref{Object: 3}, true, 0, false, "XYZ", []Object{Number(30), Number(100), Number(2)})
	if got, ok := destination.Left(d); !ok || got != 20 {
		t.Fatalf("left = %v, present=%v", got, ok)
	}
	if got, ok := destination.Top(d); !ok || got != 80 {
		t.Fatalf("top = %v, present=%v", got, ok)
	}
	if got, ok := destination.Pos(d); !ok || got != [2]float64{20, 80} {
		t.Fatalf("pos = %v, present=%v", got, ok)
	}
	if got, ok := destination.BBox(d); !ok || got != [4]float64{20, -20, 90, 80} {
		t.Fatalf("bbox = %v, present=%v", got, ok)
	}
	if got, ok := destination.Zoom(d); !ok || got != 2 {
		t.Fatalf("zoom = %v, present=%v", got, ok)
	}
}

func TestDestinationRejectsOverflowingViewGeometry(t *testing.T) {
	d := &Document{space: CoordinateSpacePage}
	page := Page{ref: Ref{Object: 3}, dict: Dict{Name("MediaBox"): Array{Number(0), Number(0), Number(math.MaxFloat64), Number(math.MaxFloat64)}, Name("UserUnit"): Number(2)}}
	d.pageCache = map[Ref]Page{page.ref: page}
	destination := newDestination(page.ref, true, 0, false, "XYZ", []Object{Number(math.MaxFloat64), Number(math.MaxFloat64)})
	if _, ok := destination.Pos(d); ok {
		t.Fatal("overflowing destination position was accepted")
	}
	if _, ok := destination.BBox(d); ok {
		t.Fatal("overflowing destination bbox was accepted")
	}
}

func TestDestinationDefaultsUsePageDimensionsNotMediaBoxOrigin(t *testing.T) {
	d := &Document{
		space:   CoordinateSpacePage,
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 3}}, Name("Count"): Number(1)},
			{Object: 3}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2}, Name("MediaBox"): Array{Number(10), Number(20), Number(110), Number(220)}},
		},
	}
	destination := newDestination(Ref{Object: 3}, true, 0, false, "XYZ", []Object{Null{}, Null{}, Null{}})
	if got, ok := destination.Pos(d); !ok || got != [2]float64{-10, 180} {
		t.Fatalf("default destination position = %v, present=%v", got, ok)
	}
}

func TestDestinationRotatedFitCoordinatesUseAnchorTransform(t *testing.T) {
	d := &Document{
		space:   CoordinateSpacePage,
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 3}}, Name("Count"): Number(1)},
			{Object: 3}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2}, Name("Rotate"): Number(90), Name("MediaBox"): Array{Number(10), Number(20), Number(110), Number(220)}},
		},
	}

	fitV := newDestination(Ref{Object: 3}, true, 0, false, "FitV", []Object{Number(30)})
	if got, ok := fitV.Left(d); !ok || got != 180 {
		t.Fatalf("rotated FitV left = %v, present=%v", got, ok)
	}

	fitR := newDestination(Ref{Object: 3}, true, 0, false, "FitR", []Object{Number(30), Number(40), Number(80), Number(140)})
	if got, ok := fitR.Left(d); !ok || got != 120 {
		t.Fatalf("rotated FitR left = %v, present=%v", got, ok)
	}
	if got, ok := fitR.Top(d); !ok || got != 80 {
		t.Fatalf("rotated FitR top = %v, present=%v", got, ok)
	}
}

func TestActionResolvesOnlyLocalGoToPages(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 3}}, Name("Count"): Number(1)},
			{Object: 3}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2}},
		},
	}
	action := newAction("GoTo")
	action.destination = newDestination(Ref{}, false, 0, true, "", nil)
	page, err := action.PageObject(d)
	if err != nil || page.ref != (Ref{Object: 3}) {
		t.Fatalf("page = %#v, err = %v", page, err)
	}
	remote := newAction("GoToR")
	remote.destination = action.destination
	if _, err := remote.PageObject(d); err != ErrPageNotFound {
		t.Fatalf("remote action error = %v", err)
	}
}

func TestOpenActionResolvesCatalogAction(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Dict{Name("OpenAction"): Dict{Name("S"): Name("GoTo"), Name("D"): Array{Number(1), Name("Fit")}}}}}
	action := d.OpenAction()
	hasIndex := false
	if action != nil && action.destination != nil {
		_, hasIndex = action.destination.PageIndex()
	}
	if action == nil || action.Kind() != "GoTo" || action.destination == nil || !hasIndex {
		t.Fatalf("open action = %#v", action)
	}
	if !d.openActionReady || d.openActionCache == nil {
		t.Fatalf("open action was not cached: ready=%v cache=%v", d.openActionReady, d.openActionCache)
	}
}

func TestOpenActionAcceptsDirectDestinationArray(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Dict{
		Name("OpenAction"): Array{Number(2), Name("Fit")},
	}}}
	action := d.OpenAction()
	index, hasIndex := 0, false
	if action != nil && action.destination != nil {
		index, hasIndex = action.destination.PageIndex()
	}
	if action == nil || action.Kind() != "GoTo" || action.destination == nil || !hasIndex || index != 1 {
		t.Fatalf("direct open destination = %#v", action)
	}
}

func TestOpenActionWithErrorReportsMalformedAction(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Dict{
		Name("OpenAction"): Dict{Name("S"): Name("GoTo"), Name("D"): Array{Number(1), Name("FitH")}},
	}}}
	_, firstErr := d.OpenActionWithError()
	_, secondErr := d.OpenActionWithError()
	if firstErr == nil || secondErr == nil || firstErr != secondErr {
		t.Fatalf("open action errors = %v/%v, want one cached error", firstErr, secondErr)
	}
	if d.openActionErr != firstErr || !d.openActionErrReady {
		t.Fatalf("cached open action error = %v/%v", d.openActionErrReady, d.openActionErr)
	}
}

func TestOpenActionWithErrorAllowsMissingAction(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Dict{}}}
	if action, err := d.OpenActionWithError(); err != nil || action != nil {
		t.Fatalf("missing open action = %#v, err = %v", action, err)
	}
	if !d.openActionReady || d.openActionCache != nil {
		t.Fatalf("missing open action was not cached: ready=%v cache=%v", d.openActionReady, d.openActionCache)
	}
}

func TestOpenActionWithErrorReportsMalformedCatalogRoot(t *testing.T) {
	d := &Document{trailer: Dict{Name("Root"): Number(1)}}
	if action, err := d.OpenActionWithError(); err == nil || action != nil {
		t.Fatalf("malformed catalog open action = %#v, err=%v", action, err)
	}
}

func TestResolveActionWithErrorCachesMalformedReference(t *testing.T) {
	ref := Ref{Object: 10}
	d := &Document{objects: map[Ref]Object{
		ref: Dict{Name("S"): Name("GoTo"), Name("D"): Array{Number(1), Name("FitH")}},
	}}
	_, firstErr := d.ResolveActionWithError(ref)
	_, secondErr := d.ResolveActionWithError(ref)
	if firstErr == nil || secondErr == nil || firstErr != secondErr {
		t.Fatalf("action errors = %v/%v, want one cached error", firstErr, secondErr)
	}
	if d.actionErrors[ref] != firstErr {
		t.Fatalf("cached action error = %v, want %v", d.actionErrors[ref], firstErr)
	}
}

func TestResolveActionWithErrorHonorsErrorCacheBudget(t *testing.T) {
	ref := Ref{Object: 10}
	d := &Document{
		cacheOptionsConfigured: true,
		cacheOptions:           cacheconfig.Options{ActionErrorBytes: 0},
		objects:                map[Ref]Object{ref: Dict{Name("S"): Name("GoTo"), Name("D"): Array{Number(1), Name("FitH")}}},
	}
	_, firstErr := d.ResolveActionWithError(ref)
	_, secondErr := d.ResolveActionWithError(ref)
	if firstErr == nil || secondErr == nil || firstErr == secondErr {
		t.Fatalf("action errors = %v/%v, want uncached independent errors", firstErr, secondErr)
	}
	if len(d.actionErrors) != 0 {
		t.Fatalf("action error cache retained entry with zero budget: %#v", d.actionErrors)
	}
}

func TestDestinationIgnoresMalformedCoordinatesInsteadOfUsingZero(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 3}}, Name("Count"): Number(1)},
			{Object: 3}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2}, Name("MediaBox"): Array{Number(0), Number(0), Number(600), Number(800)}},
		},
	}
	destination := newDestination(Ref{Object: 3}, true, 0, false, "XYZ", []Object{Number(100), String("invalid")})
	position, ok := destination.Pos(d)
	if !ok || position[1] != 800 {
		t.Fatalf("malformed destination position = %#v, present=%v", position, ok)
	}
}

func TestDestinationIgnoresNonFiniteCoordinates(t *testing.T) {
	d := &Document{
		trailer: Dict{Name("Root"): Ref{Object: 1}},
		objects: map[Ref]Object{
			{Object: 1}: Dict{Name("Pages"): Ref{Object: 2}},
			{Object: 2}: Dict{Name("Type"): Name("Pages"), Name("Kids"): Array{Ref{Object: 3}}, Name("Count"): Number(1)},
			{Object: 3}: Dict{Name("Type"): Name("Page"), Name("Parent"): Ref{Object: 2}, Name("MediaBox"): Array{Number(0), Number(0), Number(600), Number(800)}},
		},
	}
	destination := newDestination(Ref{Object: 3}, true, 0, false, "XYZ", []Object{Number(math.Inf(1)), Number(math.NaN()), Number(math.Inf(1))})
	position, ok := destination.Pos(d)
	if !ok || position != [2]float64{0, 800} {
		t.Fatalf("non-finite destination position = %#v, present=%v", position, ok)
	}
	if _, ok := destination.Top(d); ok {
		t.Fatal("non-finite destination top unexpectedly present")
	}
	if _, ok := destination.Zoom(d); ok {
		t.Fatal("non-finite destination zoom unexpectedly present")
	}
}
