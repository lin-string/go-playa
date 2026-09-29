package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lin-string/go-playa/internal/testcompat"
)

const (
	// Bump when the Playa snapshot/projection representation changes. The
	// upstream commit alone does not invalidate snapshots produced by an older
	// compatibility driver.
	cacheVersion = 41
	// Keep cache temporary files long enough that an unusually slow active
	// compatibility run cannot be mistaken for an abandoned conversion.
	compatTempRetention  = 24 * time.Hour
	compatGZIPReadBuffer = 1 << 20
)

func newBufferedGZIPReader(source io.Reader) (*gzip.Reader, error) {
	return gzip.NewReader(bufio.NewReaderSize(source, compatGZIPReadBuffer))
}

type cachePruneReport struct {
	Files int
	Bytes int64
}

type upstreamConfig struct {
	Package string
	Version string
	Tag     string
	Commit  string
}

// pruneCompatCache removes only cache entries that the current cache key can
// no longer address. An unreadable entry is deliberately left in place so a
// cleanup command cannot turn an uncertain format into data loss.
func pruneCompatCache(cacheDir string) (cachePruneReport, error) {
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return cachePruneReport{}, nil
		}
		return cachePruneReport{}, err
	}
	var report cachePruneReport
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(cacheDir, entry.Name())
		if isCompatTemporary(entry.Name()) {
			info, err := entry.Info()
			if err != nil || time.Since(info.ModTime()) <= compatTempRetention {
				continue
			}
			if err := os.Remove(path); err != nil {
				return report, err
			}
			report.Files++
			report.Bytes += info.Size()
			continue
		}
		if filepath.Ext(entry.Name()) != ".jsonl" && filepath.Ext(entry.Name()) != ".gz" {
			continue
		}
		compressed := strings.HasSuffix(entry.Name(), ".jsonl.gz")
		if !compressed && !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		version, ok, err := cacheFileVersion(path, compressed)
		if err != nil || !ok || version >= cacheVersion {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if err := os.Remove(path); err != nil {
			return report, err
		}
		report.Files++
		report.Bytes += info.Size()
	}
	return report, nil
}

func isCompatTemporary(name string) bool {
	return (strings.HasPrefix(name, ".compat-") && strings.HasSuffix(name, ".tmp.gz")) ||
		(strings.HasPrefix(name, ".compat-convert-") && strings.HasSuffix(name, ".jsonl.gz"))
}

func cacheFileVersion(path string, compressed bool) (int, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, false, err
	}
	defer func() { _ = file.Close() }()
	var reader io.Reader = file
	var gzipReader *gzip.Reader
	if compressed {
		gzipReader, err = newBufferedGZIPReader(file)
		if err != nil {
			return 0, false, err
		}
		defer func() { _ = gzipReader.Close() }()
		reader = gzipReader
	}
	decoder := json.NewDecoder(reader)
	var header struct {
		Kind     string                     `json:"kind"`
		Metadata map[string]json.RawMessage `json:"metadata"`
	}
	if err := decoder.Decode(&header); err != nil || header.Kind != "header" {
		if err == nil {
			err = fmt.Errorf("invalid compatibility cache header")
		}
		return 0, false, err
	}
	value, ok := header.Metadata["cache_version"]
	if !ok {
		return 0, false, nil
	}
	var version int
	if err := json.Unmarshal(value, &version); err != nil {
		return 0, false, err
	}
	return version, true, nil
}

type cacheMetadata map[string]any

type snapshotStream struct {
	io.Reader
	io.Closer
}

type playaSnapshotError struct {
	message string
}

func (err *playaSnapshotError) Error() string {
	return "playa snapshot failed: " + err.message
}

// reopenableGZIP is a seekable view over a compressed snapshot. Generated
// Playa output can be many gigabytes of JSONL; keeping it compressed on disk
// avoids a second uncompressed copy while still allowing the cache writer and
// comparator to rewind the stream without retaining it in memory.
type reopenableGZIP struct {
	path   string
	file   *os.File
	reader *gzip.Reader
	closed bool
}

func openReopenableGZIP(path string) (*reopenableGZIP, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	reader, err := newBufferedGZIPReader(file)
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	return &reopenableGZIP{path: path, file: file, reader: reader}, nil
}

func (stream *reopenableGZIP) Read(data []byte) (int, error) {
	if stream.closed {
		return 0, os.ErrClosed
	}
	return stream.reader.Read(data)
}

func (stream *reopenableGZIP) Seek(offset int64, whence int) (int64, error) {
	if stream.closed {
		return 0, os.ErrClosed
	}
	if offset != 0 || whence != io.SeekStart {
		return 0, fmt.Errorf("compressed snapshot only supports rewind to the beginning")
	}
	if err := stream.reader.Close(); err != nil {
		return 0, err
	}
	if err := stream.file.Close(); err != nil {
		return 0, err
	}
	reopened, err := openReopenableGZIP(stream.path)
	if err != nil {
		return 0, err
	}
	stream.file = reopened.file
	stream.reader = reopened.reader
	return 0, nil
}

func (stream *reopenableGZIP) Close() error {
	if stream.closed {
		return nil
	}
	stream.closed = true
	var first error
	if err := stream.reader.Close(); err != nil {
		first = err
	}
	if err := stream.file.Close(); err != nil && first == nil {
		first = err
	}
	return first
}

// removeOnClose owns a generated compatibility snapshot. The file must stay
// seekable while it is compared, but it is not a cache entry and must not
// survive the stream lifetime.
type removeOnClose struct {
	closer io.Closer
	path   string
	once   sync.Once
	err    error
}

func (closer *removeOnClose) Close() error {
	closer.once.Do(func() {
		if err := closer.closer.Close(); err != nil {
			closer.err = err
		}
		if err := os.Remove(closer.path); err != nil && !os.IsNotExist(err) && closer.err == nil {
			closer.err = err
		}
	})
	return closer.err
}

func (stream *snapshotStream) Seek(offset int64, whence int) (int64, error) {
	seekable, ok := stream.Reader.(io.Seeker)
	if !ok {
		return 0, fmt.Errorf("snapshot stream is not seekable")
	}
	return seekable.Seek(offset, whence)
}

func rewindSnapshot(snapshot io.Reader) error {
	seekable, ok := snapshot.(io.Seeker)
	if !ok {
		return fmt.Errorf("snapshot stream is not seekable")
	}
	_, err := seekable.Seek(0, io.SeekStart)
	return err
}

func loadCachedSnapshot(cacheDir, pdf string, pages []int, space string, sections ...string) (*snapshotStream, bool, error) {
	return loadCachedSnapshotWithPassword(cacheDir, pdf, pages, space, "", false, sections...)
}

func loadCachedSnapshotWithPassword(cacheDir, pdf string, pages []int, space, password string, passwordSet bool, sections ...string) (*snapshotStream, bool, error) {
	metadata, err := makeCacheMetadataWithPassword(pdf, pages, space, password, passwordSet, sections...)
	if err != nil {
		return nil, false, err
	}
	digest := cacheDigest(metadata)
	compressedPath := filepath.Join(cacheDir, digest+".jsonl.gz")
	if stream, ok, openErr := openGZIPCache(compressedPath); openErr != nil || ok {
		return stream, ok, openErr
	}
	path := filepath.Join(cacheDir, digest+".jsonl")
	if stream, ok, openErr := openJSONLCache(path); openErr != nil || ok {
		return stream, ok, openErr
	}
	legacyMetadata := cacheMetadata{}
	for key, value := range metadata {
		legacyMetadata[key] = value
	}
	legacyMetadata["cache_version"] = 1
	legacyPath := filepath.Join(cacheDir, cacheDigest(legacyMetadata)+".json")
	legacy, err := os.Open(legacyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	// Version 1 caches contain one complete JSON document. The converter keeps
	// only the small header fields in memory and streams every page directly
	// into the compressed v2 representation, including for multi-gigabyte files.
	defer func() { _ = legacy.Close() }()
	temporary, err := os.CreateTemp(cacheDir, ".compat-convert-*.jsonl.gz")
	if err != nil {
		return nil, false, nil
	}
	compressed := gzip.NewWriter(temporary)
	if err := convertLegacyCache(legacy, legacyMetadata, compressed); err != nil {
		_ = compressed.Close()
		_ = temporary.Close()
		_ = os.Remove(temporary.Name())
		return nil, false, nil
	}
	if err := compressed.Close(); err != nil {
		_ = temporary.Close()
		_ = os.Remove(temporary.Name())
		return nil, false, nil
	}
	closeErr := temporary.Close()
	renameErr := error(nil)
	if closeErr == nil {
		renameErr = os.Rename(temporary.Name(), compressedPath)
	}
	if closeErr != nil || renameErr != nil {
		_ = os.Remove(temporary.Name())
		if closeErr != nil {
			return nil, false, closeErr
		}
		return nil, false, renameErr
	}
	stream, ok, err := openGZIPCache(compressedPath)
	return stream, ok, err
}

func openJSONLCache(path string) (*snapshotStream, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	decoder := json.NewDecoder(file)
	var header struct {
		Kind     string                     `json:"kind"`
		Metadata map[string]json.RawMessage `json:"metadata"`
	}
	if err := decoder.Decode(&header); err != nil || header.Kind != "header" || !currentCacheMetadata(header.Metadata) {
		_ = file.Close()
		return nil, false, nil
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		_ = file.Close()
		return nil, false, err
	}
	return &snapshotStream{Reader: file, Closer: file}, true, nil
}

func openGZIPCache(path string) (*snapshotStream, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	reader, err := newBufferedGZIPReader(file)
	if err != nil {
		_ = file.Close()
		return nil, false, nil
	}
	decoder := json.NewDecoder(reader)
	var header struct {
		Kind     string                     `json:"kind"`
		Metadata map[string]json.RawMessage `json:"metadata"`
	}
	if err := decoder.Decode(&header); err != nil || header.Kind != "header" || !currentCacheMetadata(header.Metadata) {
		_ = reader.Close()
		_ = file.Close()
		return nil, false, nil
	}
	if err := reader.Close(); err != nil {
		_ = file.Close()
		return nil, false, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		_ = file.Close()
		return nil, false, err
	}
	reader, err = newBufferedGZIPReader(file)
	if err != nil {
		_ = file.Close()
		return nil, false, err
	}
	return &snapshotStream{Reader: reader, Closer: multiCloser{reader, file}}, true, nil
}

func currentCacheMetadata(metadata map[string]json.RawMessage) bool {
	value, ok := metadata["cache_version"]
	if !ok {
		return false
	}
	var version int
	return json.Unmarshal(value, &version) == nil && version == cacheVersion
}

type multiCloser []io.Closer

func (closers multiCloser) Close() error {
	var first error
	for _, closer := range closers {
		if err := closer.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func makeCacheMetadata(pdf string, pages []int, space string, sections ...string) (cacheMetadata, error) {
	return makeCacheMetadataWithPassword(pdf, pages, space, "", false, sections...)
}

func makeCacheMetadataWithPassword(pdf string, pages []int, space, password string, passwordSet bool, sections ...string) (cacheMetadata, error) {
	pdfDigest, err := fileDigest(pdf)
	if err != nil {
		return nil, fmt.Errorf("compatibility cache PDF digest: %w", err)
	}
	upstreamPath, err := repositoryFile("compat", "upstream.toml")
	if err != nil {
		return nil, err
	}
	upstream, err := loadUpstreamConfig(upstreamPath)
	if err != nil {
		return nil, err
	}
	if pages == nil {
		pages = []int{}
	}
	metadata := cacheMetadata{
		"cache_version":  cacheVersion,
		"schema_version": testcompat.SchemaVersion,
		"package":        upstream.Package,
		"version":        upstream.Version,
		"tag":            upstream.Tag,
		"commit":         upstream.Commit,
		"pdf_sha256":     pdfDigest,
		"pages":          pages,
		"space":          space,
		"sections":       append([]string(nil), sections...),
	}
	if passwordSet {
		hash := sha256.Sum256([]byte(password))
		metadata["password_set"] = true
		metadata["password_sha256"] = hex.EncodeToString(hash[:])
	} else {
		metadata["password_set"] = false
	}
	return metadata, nil
}

func repositoryFile(parts ...string) (string, error) {
	directory, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("compatibility repository directory: %w", err)
	}
	for {
		path := filepath.Join(append([]string{directory}, parts...)...)
		if _, err := os.Stat(filepath.Join(directory, "go.mod")); err == nil {
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			break
		}
		directory = parent
	}
	return "", fmt.Errorf("compatibility repository file %s not found", filepath.Join(parts...))
}

func loadUpstreamConfig(path string) (upstreamConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return upstreamConfig{}, fmt.Errorf("read compatibility upstream metadata: %w", err)
	}
	values := map[string]string{}
	inPlaya := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "[playa]" {
			inPlaya = true
			continue
		}
		if strings.HasPrefix(line, "[") {
			inPlaya = false
			continue
		}
		if !inPlaya || line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, raw, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		value, err := strconv.Unquote(strings.TrimSpace(raw))
		if err != nil {
			return upstreamConfig{}, fmt.Errorf("parse compatibility upstream %s: %w", strings.TrimSpace(key), err)
		}
		values[strings.TrimSpace(key)] = value
	}
	upstream := upstreamConfig{
		Package: values["package"],
		Version: values["version"],
		Tag:     values["tag"],
		Commit:  values["commit"],
	}
	for key, value := range map[string]string{
		"package": upstream.Package,
		"version": upstream.Version,
		"tag":     upstream.Tag,
		"commit":  upstream.Commit,
	} {
		if value == "" {
			return upstreamConfig{}, fmt.Errorf("compatibility upstream metadata is missing playa.%s", key)
		}
	}
	return upstream, nil
}

func fileDigest(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	hash := sha256.New()
	if _, err := io.CopyBuffer(hash, file, make([]byte, 1024*1024)); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func cacheDigest(metadata cacheMetadata) string {
	encoded, _ := json.Marshal(metadata)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func generatePlayaSnapshotWithPassword(pdf string, pages []int, space, password string, passwordSet bool, sections ...string) (*snapshotStream, error) {
	return generatePlayaSnapshotWithPasswordContext(context.Background(), pdf, pages, space, password, passwordSet, sections...)
}

func generatePlayaSnapshotWithPasswordContext(ctx context.Context, pdf string, pages []int, space, password string, passwordSet bool, sections ...string) (*snapshotStream, error) {
	temporary, err := os.CreateTemp("", "go-playa-snapshot-*.jsonl.gz")
	if err != nil {
		return nil, fmt.Errorf("create Playa snapshot stream: %w", err)
	}
	path := temporary.Name()
	cleanup := func() {
		_ = temporary.Close()
		_ = os.Remove(path)
	}
	if err := temporary.Close(); err != nil {
		_ = os.Remove(path)
		return nil, fmt.Errorf("close Playa snapshot stream: %w", err)
	}
	// The JSONL writer releases Playa's page-local caches after each page, so
	// the whole snapshot can be produced by one document lifetime. Keeping the
	// process alive avoids reopening large PDFs and restarting Python for every
	// page batch while retaining bounded page-local memory.
	if err := runPlayaSnapshotProcessContext(ctx, pdf, pages, space, password, passwordSet, false, sections, path); err != nil {
		cleanup()
		return nil, err
	}
	reader, err := openReopenableGZIP(path)
	if err != nil {
		cleanup()
		return nil, err
	}
	return &snapshotStream{
		Reader: reader,
		Closer: &removeOnClose{closer: reader, path: path},
	}, nil
}

func runPlayaSnapshotProcessContext(ctx context.Context, pdf string, pages []int, space, password string, passwordSet, headerOnly bool, sections []string, outputPath string) error {
	args := []string{"run", "--project", "compat", "python", "scripts/compare_playa.py", "--snapshot-only", "--snapshot-jsonl", "--no-cache", "--space", space}
	if headerOnly {
		args = append(args, "--header-only")
	}
	for _, page := range pages {
		args = append(args, "--page", fmt.Sprint(page))
	}
	for _, section := range sections {
		args = append(args, "--section", section)
	}
	if passwordSet {
		args = append(args, "--password", password)
	}
	args = append(args, pdf)
	output, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open Playa snapshot output: %w", err)
	}
	compressed := gzip.NewWriter(output)
	command := exec.CommandContext(ctx, "uv", args...)
	stdout, err := command.StdoutPipe()
	if err != nil {
		_ = compressed.Close()
		_ = output.Close()
		return fmt.Errorf("open Playa snapshot pipe: %w", err)
	}
	var stderr strings.Builder
	command.Stderr = &stderr
	compatibilityProcessPhaseGate.acquire(comparePhaseOracle)
	defer compatibilityProcessPhaseGate.release(comparePhaseOracle)
	if err := command.Start(); err != nil {
		_ = compressed.Close()
		_ = output.Close()
		return fmt.Errorf("start Playa snapshot: %w", err)
	}
	_, copyErr := io.Copy(compressed, stdout)
	waitErr := command.Wait()
	compressErr := compressed.Close()
	closeErr := output.Close()
	if copyErr != nil {
		return fmt.Errorf("compress Playa snapshot: %w", copyErr)
	}
	if waitErr != nil {
		if stderr.Len() > 0 {
			return &playaSnapshotError{message: strings.TrimSpace(stderr.String())}
		}
		return fmt.Errorf("start Playa snapshot: %w", waitErr)
	}
	if compressErr != nil {
		return fmt.Errorf("close Playa snapshot compression: %w", compressErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close Playa snapshot output: %w", closeErr)
	}
	return nil
}

func cacheSnapshotWithPasswordContext(ctx context.Context, cacheDir, pdf string, pages []int, space, password string, passwordSet bool, sections ...string) (*snapshotStream, bool, error) {
	snapshot, hit, err := loadCachedSnapshotWithPassword(cacheDir, pdf, pages, space, password, passwordSet, sections...)
	if err != nil || hit {
		return snapshot, hit, err
	}
	snapshot, err = generatePlayaSnapshotWithPasswordContext(ctx, pdf, pages, space, password, passwordSet, sections...)
	if err != nil {
		return nil, false, err
	}
	metadata, err := makeCacheMetadataWithPassword(pdf, pages, space, password, passwordSet, sections...)
	if err != nil {
		_ = snapshot.Close()
		return nil, false, err
	}
	if err := os.MkdirAll(cacheDir, 0o755); err == nil {
		path := filepath.Join(cacheDir, cacheDigest(metadata)+".jsonl.gz")
		temporary, createErr := os.CreateTemp(cacheDir, ".compat-*.tmp.gz")
		if createErr == nil {
			if seekErr := rewindSnapshot(snapshot); seekErr != nil {
				_ = temporary.Close()
				_ = os.Remove(temporary.Name())
				_ = snapshot.Close()
				return nil, false, seekErr
			}
			compressed := gzip.NewWriter(temporary)
			writeErr := copyCacheStream(compressed, metadata, snapshot)
			if writeErr == nil {
				writeErr = compressed.Close()
			} else {
				_ = compressed.Close()
			}
			closeErr := temporary.Close()
			if writeErr == nil && closeErr == nil {
				if renameErr := os.Rename(temporary.Name(), path); renameErr == nil {
					if cached, openErr := os.Open(path); openErr == nil {
						_ = cached.Close()
						if cachedStream, cachedHit, cachedErr := openGZIPCache(path); cachedErr == nil && cachedHit {
							_ = snapshot.Close()
							return cachedStream, false, nil
						}
					}
				}
			}
			_ = os.Remove(temporary.Name())
		}
	}
	if snapshot != nil {
		if err := rewindSnapshot(snapshot); err != nil {
			_ = snapshot.Close()
			return nil, false, err
		}
	}
	return snapshot, false, nil
}

func copyCacheStream(writer io.Writer, metadata cacheMetadata, snapshot io.Reader) error {
	encodedMetadata, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(writer, `{"kind":"header","metadata":`); err != nil {
		return err
	}
	if _, err := writer.Write(encodedMetadata); err != nil {
		return err
	}
	if _, err := io.WriteString(writer, ","); err != nil {
		return err
	}
	decoder := json.NewDecoder(snapshot)
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	start, ok := token.(json.Delim)
	if !ok || start != '{' {
		return fmt.Errorf("compatibility snapshot header is not an object")
	}
	foundSnapshot := false
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return err
		}
		key, ok := keyToken.(string)
		if !ok {
			return fmt.Errorf("compatibility snapshot header has a non-string key")
		}
		if key == "snapshot" {
			if foundSnapshot {
				return fmt.Errorf("compatibility snapshot header has duplicate snapshot")
			}
			if _, err := io.WriteString(writer, "\"snapshot\":"); err != nil {
				return err
			}
			snapshotToken, err := decoder.Token()
			if err != nil {
				return err
			}
			if snapshotToken == nil {
				return fmt.Errorf("compatibility snapshot header is missing snapshot")
			}
			if err := copyJSONToken(decoder, snapshotToken, writer); err != nil {
				return err
			}
			foundSnapshot = true
			continue
		}
		if err := copyJSONValue(decoder, nil); err != nil {
			return err
		}
	}
	if _, err := decoder.Token(); err != nil {
		return err
	}
	if !foundSnapshot {
		return fmt.Errorf("compatibility snapshot header is missing snapshot")
	}
	if _, err := io.WriteString(writer, "}\n"); err != nil {
		return err
	}
	for {
		if err := copyJSONValue(decoder, writer); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if _, err := io.WriteString(writer, "\n"); err != nil {
			return err
		}
	}
	return nil
}

func convertLegacyCache(reader io.ReadSeeker, metadata cacheMetadata, writer io.Writer) error {
	pageFile, err := os.CreateTemp("", "go-playa-legacy-pages-*.jsonl")
	if err != nil {
		return err
	}
	pageName := pageFile.Name()
	defer func() {
		_ = pageFile.Close()
		_ = os.Remove(pageName)
	}()
	decoder := json.NewDecoder(reader)
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return fmt.Errorf("legacy cache is not a JSON object")
	}
	var actualMetadata map[string]any
	snapshotFields := map[string]json.RawMessage{}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return err
		}
		switch key {
		case "metadata":
			if err := decoder.Decode(&actualMetadata); err != nil {
				return err
			}
		case "snapshot":
			if err := decodeLegacySnapshot(decoder, snapshotFields, pageFile); err != nil {
				return err
			}
		default:
			var ignored json.RawMessage
			if err := decoder.Decode(&ignored); err != nil {
				return err
			}
		}
	}
	if _, err := decoder.Token(); err != nil {
		return err
	}
	expectedMetadata, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	gotMetadata, err := json.Marshal(actualMetadata)
	if err != nil || !bytes.Equal(expectedMetadata, gotMetadata) {
		return fmt.Errorf("legacy cache metadata mismatch")
	}
	currentMetadata := cacheMetadata{}
	for key, value := range metadata {
		currentMetadata[key] = value
	}
	currentMetadata["cache_version"] = cacheVersion
	currentMetadataJSON, err := json.Marshal(currentMetadata)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(writer, `{"kind":"header","metadata":`); err != nil {
		return err
	}
	if _, err := writer.Write(currentMetadataJSON); err != nil {
		return err
	}
	if _, err := io.WriteString(writer, `,"snapshot":`); err != nil {
		return err
	}
	encodedFields, err := json.Marshal(snapshotFields)
	if err != nil {
		return err
	}
	if _, err := writer.Write(encodedFields); err != nil {
		return err
	}
	if _, err := io.WriteString(writer, "}\n"); err != nil {
		return err
	}
	if _, err := pageFile.Seek(0, io.SeekStart); err != nil {
		return err
	}
	_, err = io.Copy(writer, pageFile)
	return err
}

func decodeLegacySnapshot(decoder *json.Decoder, fields map[string]json.RawMessage, pages io.Writer) error {
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return fmt.Errorf("legacy snapshot is not a JSON object")
	}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return err
		}
		name, ok := key.(string)
		if !ok {
			return fmt.Errorf("legacy snapshot key is not a string")
		}
		if name != "pages" {
			var value json.RawMessage
			if err := decoder.Decode(&value); err != nil {
				return err
			}
			fields[name] = value
			continue
		}
		token, err := decoder.Token()
		if err != nil || token != json.Delim('[') {
			return fmt.Errorf("legacy snapshot pages is not an array")
		}
		for decoder.More() {
			var page json.RawMessage
			if err := decoder.Decode(&page); err != nil {
				return err
			}
			if _, err := io.WriteString(pages, `{"kind":"page","page":`); err != nil {
				return err
			}
			if _, err := pages.Write(page); err != nil {
				return err
			}
			if _, err := io.WriteString(pages, "}\n"); err != nil {
				return err
			}
		}
		if _, err := decoder.Token(); err != nil {
			return err
		}
	}
	_, err = decoder.Token()
	return err
}
