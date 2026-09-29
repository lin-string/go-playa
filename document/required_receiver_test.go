package document

import "testing"

func requireProgrammerPanic(t *testing.T, run func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("nil required receiver did not trigger a programmer error")
		}
	}()
	run()
}

func TestRequiredDocumentReceivers(t *testing.T) {
	for name, run := range map[string]func(){
		"close":       func() { _ = (*Document)(nil).Close() },
		"permissions": func() { _ = (*Document)(nil).IsPrintable() },
		"info":        func() { _, _ = (*Document)(nil).InfoWithError() },
		"metadata":    func() { _, _ = (*Document)(nil).MetadataWithError() },
		"mapping":     func() { _ = (*Document)(nil).Len() },
		"name tree iteration": func() {
			for range (*Document)(nil).NameTreeSeq("Dests") {
			}
		},
	} {
		t.Run(name, func(t *testing.T) { requireProgrammerPanic(t, run) })
	}
}

func TestRequiredFontReceiver(t *testing.T) {
	requireProgrammerPanic(t, func() { _ = (*Font)(nil).Name() })
}

func TestFontMetricResolverIsRequired(t *testing.T) {
	requireProgrammerPanic(t, func() { setFontMetricsWithResolver(&Font{}, Dict{}, nil) })
}
