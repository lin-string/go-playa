package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOpenGZIPCacheStreamsSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshot.jsonl.gz")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := gzip.NewWriter(file)
	input := fmt.Sprintf(`{"kind":"header","metadata":{"cache_version":%d},"snapshot":{"schema_version":1}}
{"kind":"page","page":{"index":0}}
`, cacheVersion)
	if _, err := writer.Write([]byte(input)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	stream, hit, err := openGZIPCache(path)
	if err != nil || !hit {
		t.Fatalf("stream = %v, hit = %v, err = %v", stream, hit, err)
	}
	defer func() { _ = stream.Close() }()
	var decoded bytes.Buffer
	if _, err := io.Copy(&decoded, stream); err != nil {
		t.Fatal(err)
	}
	if decoded.String() != input {
		t.Fatalf("decoded = %q, want %q", decoded.String(), input)
	}
}

func TestOpenGZIPCacheRejectsStaleVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stale.jsonl.gz")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := gzip.NewWriter(file)
	if _, err := fmt.Fprintf(writer,
		`{"kind":"header","metadata":{"cache_version":%d}}`+"\n",
		cacheVersion-1,
	); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	stream, hit, err := openGZIPCache(path)
	if err != nil {
		t.Fatal(err)
	}
	if hit || stream != nil {
		t.Fatalf("stale cache accepted: stream=%v hit=%v", stream, hit)
	}
}

func TestOpenJSONLCacheRejectsStaleVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stale.jsonl")
	if err := os.WriteFile(path, []byte(fmt.Sprintf(
		`{"kind":"header","metadata":{"cache_version":%d}}`+"\n",
		cacheVersion-1,
	)), 0o600); err != nil {
		t.Fatal(err)
	}

	stream, hit, err := openJSONLCache(path)
	if err != nil {
		t.Fatal(err)
	}
	if hit || stream != nil {
		t.Fatalf("stale cache accepted: stream=%v hit=%v", stream, hit)
	}
}

func TestPruneCompatCacheRemovesOnlyConfirmedOlderJSONLSnapshots(t *testing.T) {
	dir := t.TempDir()
	writeCache := func(name, metadata string, compressed bool) {
		t.Helper()
		path := filepath.Join(dir, name)
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		var writer io.Writer = file
		var gzipWriter *gzip.Writer
		if compressed {
			gzipWriter = gzip.NewWriter(file)
			writer = gzipWriter
		}
		if _, err := fmt.Fprintf(writer, `{"kind":"header","metadata":%s}\n`, metadata); err != nil {
			_ = file.Close()
			t.Fatal(err)
		}
		if gzipWriter != nil {
			if err := gzipWriter.Close(); err != nil {
				_ = file.Close()
				t.Fatal(err)
			}
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}
	writeCache("old.jsonl.gz", `{"cache_version":18}`, true)
	writeCache("old.jsonl", `{"cache_version":18}`, false)
	writeCache("current.jsonl.gz", fmt.Sprintf(`{"cache_version":%d}`, cacheVersion), true)
	writeCache("future.jsonl", fmt.Sprintf(`{"cache_version":%d}`, cacheVersion+1), false)
	if err := os.WriteFile(filepath.Join(dir, "unreadable.jsonl.gz"), []byte("not gzip"), 0o600); err != nil {
		t.Fatal(err)
	}

	report, err := pruneCompatCache(dir)
	if err != nil {
		t.Fatal(err)
	}
	if report.Files != 2 || report.Bytes <= 0 {
		t.Fatalf("prune report = %#v, want two removed files with positive bytes", report)
	}
	for _, name := range []string{"old.jsonl.gz", "old.jsonl"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("old cache %s still exists: %v", name, err)
		}
	}
	for _, name := range []string{"current.jsonl.gz", "future.jsonl", "unreadable.jsonl.gz"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("cache %s was unexpectedly removed: %v", name, err)
		}
	}
}

func TestPruneCompatCacheRemovesOnlyStaleTemporaryFiles(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, ".compat-old.tmp.gz")
	oldConvertPath := filepath.Join(dir, ".compat-convert-old.jsonl.gz")
	newPath := filepath.Join(dir, ".compat-new.tmp.gz")
	if err := os.WriteFile(oldPath, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldConvertPath, []byte("converted"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte("active"), 0o600); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-24*time.Hour - time.Hour)
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(oldConvertPath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	report, err := pruneCompatCache(dir)
	if err != nil {
		t.Fatal(err)
	}
	if report.Files != 2 || report.Bytes != int64(len("stale")+len("converted")) {
		t.Fatalf("prune report = %#v, want two stale temporary files", report)
	}
	for _, path := range []string{oldPath, oldConvertPath} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("stale temporary file %s still exists: %v", path, err)
		}
	}
	if _, err := os.Stat(newPath); err != nil {
		t.Fatalf("new temporary file was unexpectedly removed: %v", err)
	}
}

func TestCopyCacheStreamWritesMetadataAndJSONLPages(t *testing.T) {
	metadata := cacheMetadata{"cache_version": 2, "space": "page"}
	snapshot := []byte("{\"kind\":\"header\",\"snapshot\":{\"z\":1,\"a\":2}}\n{\"kind\":\"page\",\"page\":{\"index\":0}}\n")
	var encoded bytes.Buffer
	if err := copyCacheStream(&encoded, metadata, strings.NewReader(string(snapshot))); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(encoded.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %d, want 2", len(lines))
	}
	var header struct {
		Metadata map[string]any  `json:"metadata"`
		Snapshot json.RawMessage `json:"snapshot"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &header); err != nil {
		t.Fatal(err)
	}
	if header.Metadata["space"] != "page" || !bytes.Equal(header.Snapshot, []byte(`{"z":1,"a":2}`)) {
		t.Fatalf("header = %#v, snapshot = %s", header.Metadata, header.Snapshot)
	}
	if !bytes.Equal([]byte(lines[1]), []byte(`{"kind":"page","page":{"index":0}}`)) {
		t.Fatalf("page = %s", lines[1])
	}
}

func TestCopyCacheStreamDoesNotWriteOneMaterializedSnapshot(t *testing.T) {
	var snapshot strings.Builder
	snapshot.WriteString("{\"kind\":\"header\",\"snapshot\":{\"items\":[")
	for index := 0; index < 128; index++ {
		if index > 0 {
			snapshot.WriteByte(',')
		}
		fmt.Fprintf(&snapshot, "%d", index)
	}
	snapshot.WriteString("]}}\n")

	var encoded maxWriteBuffer
	encoded.limit = 64
	if err := copyCacheStream(&encoded, cacheMetadata{"space": "page"}, strings.NewReader(snapshot.String())); err != nil {
		t.Fatalf("copyCacheStream() = %v", err)
	}
	if !strings.Contains(encoded.String(), "\"kind\":\"header\"") {
		t.Fatalf("encoded cache header = %q", encoded.String())
	}
}

type maxWriteBuffer struct {
	bytes.Buffer
	limit int
}

type countingReader struct {
	reader io.Reader
	reads  int
}

func (reader *countingReader) Read(data []byte) (int, error) {
	reader.reads++
	return reader.reader.Read(data)
}

func TestNewBufferedGZIPReaderAvoidsSmallUnderlyingReads(t *testing.T) {
	input := make([]byte, 2<<20)
	for index := range input {
		input[index] = byte(index*31 + index>>8)
	}
	var compressed bytes.Buffer
	writer, err := gzip.NewWriterLevel(&compressed, gzip.NoCompression)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(input); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	source := &countingReader{reader: bytes.NewReader(compressed.Bytes())}
	reader, err := newBufferedGZIPReader(source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(io.Discard, reader); err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if source.reads > 4 {
		t.Fatalf("compressed source reads = %d, want at most 4", source.reads)
	}
}

func (w *maxWriteBuffer) Write(value []byte) (int, error) {
	if len(value) > w.limit {
		return 0, fmt.Errorf("write of %d bytes exceeds limit %d", len(value), w.limit)
	}
	return w.Buffer.Write(value)
}

func TestCacheMetadataSeparatesPasswordVariants(t *testing.T) {
	pdf := filepath.Join(t.TempDir(), "input.pdf")
	if err := os.WriteFile(pdf, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	without, err := makeCacheMetadataWithPassword(pdf, nil, "page", "", false)
	if err != nil {
		t.Fatal(err)
	}
	with, err := makeCacheMetadataWithPassword(pdf, nil, "page", "secret", true)
	if err != nil {
		t.Fatal(err)
	}
	if without["password_set"] == with["password_set"] || without["password_sha256"] == with["password_sha256"] {
		t.Fatalf("password cache metadata is not isolated: without=%v with=%v", without, with)
	}
	if _, present := with["password"]; present {
		t.Fatal("cache metadata retained the raw password")
	}
}

func TestLoadUpstreamConfigRequiresAndReturnsCanonicalMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "upstream.toml")
	input := `[playa]
package = "example-playa"
version = "9.8.7"
tag = "v9.8.7"
commit = "0123456789abcdef"
coordinate_space = "page"
`
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}

	upstream, err := loadUpstreamConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if upstream.Package != "example-playa" || upstream.Version != "9.8.7" || upstream.Tag != "v9.8.7" || upstream.Commit != "0123456789abcdef" {
		t.Fatalf("upstream metadata = %#v", upstream)
	}

	if err := os.WriteFile(path, []byte("[playa]\npackage = \"example-playa\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadUpstreamConfig(path); err == nil {
		t.Fatal("incomplete upstream metadata was accepted")
	}
}

func TestCacheMetadataUsesCanonicalUpstreamConfig(t *testing.T) {
	pdf := filepath.Join(t.TempDir(), "input.pdf")
	if err := os.WriteFile(pdf, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	upstreamPath, err := repositoryFile("compat", "upstream.toml")
	if err != nil {
		t.Fatal(err)
	}
	upstream, err := loadUpstreamConfig(upstreamPath)
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := makeCacheMetadata(pdf, nil, "page")
	if err != nil {
		t.Fatal(err)
	}
	if metadata["package"] != upstream.Package || metadata["version"] != upstream.Version || metadata["tag"] != upstream.Tag || metadata["commit"] != upstream.Commit {
		t.Fatalf("cache metadata = %#v, upstream = %#v", metadata, upstream)
	}
}

func TestConvertLegacyCacheStreamsPages(t *testing.T) {
	metadata := cacheMetadata{"cache_version": 1, "schema_version": "go-playa.compat/v1", "pages": []any{}, "space": "page"}
	legacy := `{"metadata":{"cache_version":1,"pages":[],"schema_version":"go-playa.compat/v1","space":"page"},"snapshot":{"schema_version":"go-playa.compat/v1","page_count":2,"pages":[{"index":0,"text":[1]},{"index":1,"text":[2]}]}}`
	var output bytes.Buffer
	if err := convertLegacyCache(strings.NewReader(legacy), metadata, &output); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("lines = %d, want 3", len(lines))
	}
	var header map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &header); err != nil {
		t.Fatal(err)
	}
	if header["kind"] != "header" {
		t.Fatalf("header kind = %#v", header["kind"])
	}
	if !strings.Contains(lines[1], `"index":0`) || !strings.Contains(lines[2], `"index":1`) {
		t.Fatalf("page lines = %q", lines[1:])
	}
}

func TestLoadCachedSnapshotSkipsMalformedLegacyCache(t *testing.T) {
	dir := t.TempDir()
	pdf := filepath.Join(dir, "input.pdf")
	if err := os.WriteFile(pdf, []byte("pdf"), 0o644); err != nil {
		t.Fatal(err)
	}
	metadata, err := makeCacheMetadata(pdf, nil, "page")
	if err != nil {
		t.Fatal(err)
	}
	legacyMetadata := cacheMetadata{}
	for key, value := range metadata {
		legacyMetadata[key] = value
	}
	legacyMetadata["cache_version"] = 1
	legacyPath := filepath.Join(dir, cacheDigest(legacyMetadata)+".json")
	legacy, err := os.Create(legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.WriteString(`{"metadata":`); err != nil {
		_ = legacy.Close()
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}

	stream, hit, err := loadCachedSnapshot(dir, pdf, nil, "page")
	if err != nil {
		t.Fatal(err)
	}
	if stream != nil || hit {
		t.Fatalf("stream = %v, hit = %v, want no cache result", stream, hit)
	}
}

func TestLoadCachedSnapshotConvertsLegacyCacheToGZIP(t *testing.T) {
	dir := t.TempDir()
	pdf := filepath.Join(dir, "input.pdf")
	if err := os.WriteFile(pdf, []byte("pdf"), 0o644); err != nil {
		t.Fatal(err)
	}
	metadata, err := makeCacheMetadata(pdf, nil, "page")
	if err != nil {
		t.Fatal(err)
	}
	legacyMetadata := cacheMetadata{}
	for key, value := range metadata {
		legacyMetadata[key] = value
	}
	legacyMetadata["cache_version"] = 1
	legacyPath := filepath.Join(dir, cacheDigest(legacyMetadata)+".json")
	legacy := `{"metadata":` + mustJSON(t, legacyMetadata) + `,"snapshot":{"schema_version":"go-playa.compat/v1","pages":[{"index":0}]}}`
	if err := os.WriteFile(legacyPath, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	stream, hit, err := loadCachedSnapshot(dir, pdf, nil, "page")
	if err != nil || !hit {
		t.Fatalf("stream = %v, hit = %v, err = %v", stream, hit, err)
	}
	defer func() { _ = stream.Close() }()
	var output bytes.Buffer
	if _, err := io.Copy(&output, stream); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"kind":"page"`) {
		t.Fatalf("converted snapshot = %s", output.String())
	}
	if _, err := os.Stat(filepath.Join(dir, cacheDigest(metadata)+".jsonl.gz")); err != nil {
		t.Fatalf("compressed cache missing: %v", err)
	}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestGeneratePlayaSnapshotUsesOneProcessPerSectionGroup(t *testing.T) {
	toolDir := t.TempDir()
	callLog := filepath.Join(toolDir, "calls")
	uvPath := filepath.Join(toolDir, "uv")
	uvScript := `#!/bin/sh
echo call >> "$PLAYA_COMPAT_TEST_CALLS"
printf '%s\n' '{"kind":"header","snapshot":{"page_count":100}}' '{"kind":"page","page":{"index":0}}'
`
	if err := os.WriteFile(uvPath, []byte(uvScript), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", toolDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("PLAYA_COMPAT_TEST_CALLS", callLog)

	snapshot, err := generatePlayaSnapshotWithPassword("fixture.pdf", nil, "page", "", false, "content.text")
	if err != nil {
		t.Fatal(err)
	}
	temporary, ok := snapshot.Reader.(*reopenableGZIP)
	if !ok {
		t.Fatalf("snapshot reader = %T, want *reopenableGZIP", snapshot.Reader)
	}
	path := temporary.path
	data, err := io.ReadAll(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := snapshot.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("rewind compressed snapshot: %v", err)
	}
	rewound, err := io.ReadAll(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, rewound) {
		t.Fatal("compressed snapshot could not be reopened for rewind")
	}

	calls, err := os.ReadFile(callLog)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(calls), "call\n"); got != 1 {
		t.Fatalf("Playa snapshot process count = %d, want 1", got)
	}
	if err := snapshot.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("temporary snapshot still exists after Close: %v", err)
	}
}

func TestGeneratePlayaSnapshotRemovesTempFileWhenCanceled(t *testing.T) {
	toolDir := t.TempDir()
	callLog := filepath.Join(toolDir, "started")
	uvPath := filepath.Join(toolDir, "uv")
	uvScript := `#!/bin/sh
printf started > "$PLAYA_COMPAT_TEST_STARTED"
exec sleep 60
`
	if err := os.WriteFile(uvPath, []byte(uvScript), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", toolDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("PLAYA_COMPAT_TEST_STARTED", callLog)

	before := make(map[string]struct{})
	paths, err := filepath.Glob(filepath.Join(os.TempDir(), "go-playa-snapshot-*.jsonl.gz"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		before[path] = struct{}{}
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := generatePlayaSnapshotWithPasswordContext(ctx, "fixture.pdf", nil, "page", "", false, "content.text")
		done <- err
	}()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		if _, err := os.Stat(callLog); err == nil {
			break
		}
		select {
		case <-deadline.C:
			t.Fatal("timed out waiting for Playa snapshot process")
		case <-time.After(10 * time.Millisecond):
		}
	}
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("generatePlayaSnapshotWithPasswordContext() error = nil, want cancellation error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("generatePlayaSnapshotWithPasswordContext() did not return after cancellation")
	}

	paths, err = filepath.Glob(filepath.Join(os.TempDir(), "go-playa-snapshot-*.jsonl.gz"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		if _, exists := before[path]; !exists {
			t.Fatalf("canceled snapshot left temporary file %s", path)
		}
	}
}
