package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"math"
	"os"
	"reflect"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"

	core "github.com/lin-string/go-playa/document"
	"github.com/lin-string/go-playa/internal/testcompat"
)

var requiredSections = []string{
	"document", "document.objects", "document.mapping", "document.tokens", "document.buffer", "document.xrefs", "pages", "content.text", "content.extract_text", "content.extract_text.tagged", "content.extract_text.untagged", "content.glyphs", "content.flatten", "content.interp", "content.streams", "content.tokens", "content.xobjects", "content.contents", "layout", "annotations", "outline", "destinations",
	"forms", "structure", "content.structure", "fonts", "content.paths", "content.images", "content.tags", "content.marked",
}

const (
	compatibilityLargePDFBytes                = 8 << 20
	compatibilityGroupHeapCollectionThreshold = 1 << 30
)

var compatibilityGroupMemoryCollectionMu sync.Mutex

func shouldCollectCompatGroupMemory(heapAlloc uint64) bool {
	return heapAlloc >= compatibilityGroupHeapCollectionThreshold
}

func collectCompatGroupMemory() {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	if !shouldCollectCompatGroupMemory(stats.HeapAlloc) {
		return
	}
	compatibilityGroupMemoryCollectionMu.Lock()
	defer compatibilityGroupMemoryCollectionMu.Unlock()
	runtime.ReadMemStats(&stats)
	if !shouldCollectCompatGroupMemory(stats.HeapAlloc) {
		return
	}
	runtime.GC()
	debug.FreeOSMemory()
}

func collectCompatPhaseMemory() {
	compatibilityGroupMemoryCollectionMu.Lock()
	defer compatibilityGroupMemoryCollectionMu.Unlock()
	runtime.GC()
	debug.FreeOSMemory()
}

type objectFieldMetadata struct {
	name      string
	index     int
	omitEmpty bool
}

type objectMetadata struct {
	fields    []objectFieldMetadata
	byName    map[string][]int
	omitEmpty map[int]bool
}

var objectMetadataCache sync.Map // map[reflect.Type]*objectMetadata

func reflectObjectMetadata(typ reflect.Type) *objectMetadata {
	if typ == nil {
		return nil
	}
	if cached, ok := objectMetadataCache.Load(typ); ok {
		return cached.(*objectMetadata)
	}
	metadata := &objectMetadata{
		byName:    make(map[string][]int),
		omitEmpty: make(map[int]bool),
	}
	for index := 0; index < typ.NumField(); index++ {
		field := typ.Field(index)
		if field.PkgPath != "" {
			continue
		}
		name, options, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name == "-" {
			continue
		}
		if name == "" {
			name = field.Name
		}
		metadata.fields = append(metadata.fields, objectFieldMetadata{
			name:      name,
			index:     index,
			omitEmpty: options == "omitempty",
		})
		metadata.byName[name] = append(metadata.byName[name], index)
		if options == "omitempty" {
			metadata.omitEmpty[index] = true
		}
	}
	actual, _ := objectMetadataCache.LoadOrStore(typ, metadata)
	return actual.(*objectMetadata)
}

// compatibilitySectionGroups keeps large-document comparisons bounded by
// giving expensive projection domains their own Go document and Playa
// snapshot process. Small documents stay single-pass to avoid repeating setup.
func compatibilitySectionGroups(pdf string, selected []string) [][]string {
	info, err := os.Stat(pdf)
	if err != nil || info.Size() < compatibilityLargePDFBytes {
		return [][]string{append([]string(nil), selected...)}
	}
	partitions := [][]string{
		{"document", "document.objects", "document.mapping", "document.tokens", "document.buffer", "document.xrefs", "destinations", "outline", "forms", "fonts"},
		{"structure"},
		{"pages", "content.text", "content.extract_text", "content.extract_text.tagged", "content.extract_text.untagged"},
		{"content.glyphs"},
		{"layout"},
		{"annotations", "content.structure", "content.tags", "content.marked"},
		{"content.flatten"},
		{"content.interp"},
		{"content.streams", "content.tokens", "content.contents"},
		{"content.xobjects"},
		{"content.paths"},
		{"content.images"},
	}
	seen := make(map[string]bool, len(selected))
	groups := make([][]string, 0, len(partitions)+1)
	for _, partition := range partitions {
		group := make([]string, 0, len(partition))
		for _, section := range selected {
			if containsString(partition, section) {
				group = append(group, section)
				seen[section] = true
			}
		}
		if len(group) > 0 {
			groups = append(groups, group)
		}
	}
	remaining := make([]string, 0)
	for _, section := range selected {
		if !seen[section] {
			remaining = append(remaining, section)
		}
	}
	if len(remaining) > 0 {
		groups = append(groups, remaining)
	}
	return groups
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

type sequentialPageLookup struct {
	document  *core.Document
	next      func() (core.Page, error, bool)
	stop      func()
	nextIndex int
	last      core.Page
	lastIndex int
	hasLast   bool
}

func newSequentialPageLookup(document *core.Document) *sequentialPageLookup {
	next, stop := iter.Pull2(document.Pages())
	return &sequentialPageLookup{document: document, next: next, stop: stop}
}

func (lookup *sequentialPageLookup) close() { lookup.stop() }

func (lookup *sequentialPageLookup) pageAt(index int) (core.Page, error) {
	if index < 0 || (lookup.hasLast && index < lookup.lastIndex) {
		return lookup.document.PageAt(index)
	}
	for lookup.nextIndex <= index {
		page, err, ok := lookup.next()
		if !ok {
			return core.Page{}, core.ErrPageNotFound
		}
		if err != nil {
			return core.Page{}, err
		}
		lookup.last = page
		lookup.lastIndex = lookup.nextIndex
		lookup.nextIndex++
		lookup.hasLast = true
	}
	return lookup.last, nil
}

func sequentialPageIndices(indices []int) bool {
	for index := 1; index < len(indices); index++ {
		if indices[index] < indices[index-1] {
			return false
		}
	}
	return true
}

func selectSections(selected []string) ([]string, error) {
	known := make(map[string]struct{}, len(requiredSections))
	for _, section := range requiredSections {
		known[section] = struct{}{}
	}
	if len(selected) == 0 {
		return append([]string(nil), requiredSections...), nil
	}
	for _, section := range selected {
		if _, ok := known[section]; !ok {
			return nil, fmt.Errorf("unknown compatibility section %q", section)
		}
	}
	return selected, nil
}

func projectSections(snapshot map[string]any, sections []string) map[string]any {
	result := map[string]any{"schema_version": snapshot["schema_version"]}
	has := make(map[string]bool, len(sections))
	for _, section := range sections {
		has[section] = true
	}
	if has["document"] {
		result["page_count"] = snapshot["page_count"]
		result["page_labels"] = snapshot["page_labels"]
		for _, field := range []string{"pdf_version", "is_tagged", "is_printable", "is_modifiable", "is_extractable"} {
			result[field] = snapshot[field]
		}
		for _, field := range []string{"info", "catalog", "names", "trailer", "open_action"} {
			result[field] = snapshot[field]
		}
	}
	if has["document.objects"] {
		result["objects"] = snapshot["objects"]
	}
	if has["document.mapping"] {
		result["mapping"] = snapshot["mapping"]
	}
	if has["document.tokens"] {
		result["tokens"] = snapshot["tokens"]
	}
	if has["document.buffer"] {
		result["buffer"] = snapshot["buffer"]
	}
	if has["document.xrefs"] {
		result["xrefs"] = snapshot["xrefs"]
	}
	for _, key := range []string{"destinations", "outline", "structure"} {
		if has[key] {
			result[key] = snapshot[key]
		}
	}
	if has["forms"] {
		result["need_appearances"] = snapshot["need_appearances"]
		result["forms"] = snapshot["forms"]
	}
	if has["fonts"] {
		result["document_fonts"] = snapshot["document_fonts"]
	}
	pageFields := has["pages"]
	pageSections := []string{"content.text", "content.extract_text", "content.extract_text.tagged", "content.extract_text.untagged", "content.glyphs", "content.flatten", "content.interp", "content.streams", "content.tokens", "content.xobjects", "content.contents", "layout", "content.paths", "content.images", "fonts", "content.tags", "content.marked", "annotations"}
	for _, section := range pageSections {
		pageFields = pageFields || has[section]
	}
	if !pageFields {
		return result
	}

	pages, _ := snapshot["pages"].([]any)
	projected := make([]any, 0, len(pages))
	for _, raw := range pages {
		page, _ := raw.(map[string]any)
		item := map[string]any{"index": page["index"]}
		if has["pages"] {
			for _, field := range []string{"label", "width", "height", "rotation", "parent_key"} {
				item[field] = page[field]
			}
		}
		for section, field := range map[string]string{
			"content.text": "text", "content.paths": "paths", "content.images": "images",
			"fonts": "fonts", "content.tags": "tags", "content.marked": "marked", "annotations": "annotations", "content.flatten": "flatten", "content.interp": "interp", "content.streams": "streams", "content.tokens": "tokens", "content.xobjects": "xobjects", "content.contents": "contents", "content.extract_text": "extract_text", "content.extract_text.tagged": "extract_text_tagged", "content.extract_text.untagged": "extract_text_untagged", "content.glyphs": "glyphs", "layout": "layout",
		} {
			if has[section] {
				item[field] = page[field]
			}
		}
		projected = append(projected, item)
	}
	result["pages"] = projected
	return result
}

func decodeJSON(data []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func compareJSON(expected, actual any, path string, tolerance float64, differences *[]string) {
	compareJSONValue(expected, reflect.ValueOf(actual), path, tolerance, differences)
}

func compareJSONValue(expected any, actual reflect.Value, path string, tolerance float64, differences *[]string) {
	actual = indirectValue(actual)
	if left, ok := expected.(json.Number); ok {
		right, ok := numericString(actual)
		if !ok {
			addDifference(differences, "%s: playa type=number, go type=%s", path, valueType(actual))
			return
		}
		lf, lerr := strconv.ParseFloat(left.String(), 64)
		rf, rerr := strconv.ParseFloat(right, 64)
		if lerr != nil || rerr != nil || !math.IsNaN(lf) && !math.IsNaN(rf) && math.Abs(lf-rf) > tolerance {
			addDifference(differences, "%s: playa=%s, go=%v", path, left, rf)
		}
		return
	}
	if _, ok := actualInterface(actual).(json.Number); ok {
		addDifference(differences, "%s: playa type=%T, go type=number", path, expected)
		return
	}
	switch left := expected.(type) {
	case map[string]any:
		if !isObjectValue(actual) {
			addDifference(differences, "%s: playa type=object, go type=%s", path, valueType(actual))
			return
		}
		keys := make([]string, 0, len(left))
		for key := range left {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			lv := left[key]
			child := key
			if path != "" {
				child = path + "." + key
			}
			rv, rok := objectField(actual, key)
			if !rok {
				addDifference(differences, "%s: missing Go value", child)
			} else {
				compareJSONValue(lv, rv, child, tolerance, differences)
			}
		}
		for _, key := range objectKeys(actual) {
			if _, exists := left[key]; exists {
				continue
			}
			child := key
			if path != "" {
				child = path + "." + key
			}
			addDifference(differences, "%s: unexpected Go value", child)
		}
	case []any:
		rightLen, ok := arrayLength(actual)
		if !ok {
			addDifference(differences, "%s: playa type=array, go type=%s", path, valueType(actual))
			return
		}
		if len(left) != rightLen {
			addDifference(differences, "%s: playa length=%d, go length=%d", path, len(left), rightLen)
		}
		for index := 0; index < len(left) && index < rightLen; index++ {
			compareJSONValue(left[index], actual.Index(index), fmt.Sprintf("%s[%d]", path, index), tolerance, differences)
		}
	default:
		if !scalarEqual(expected, actual) {
			addDifference(differences, "%s: playa=%v, go=%v", path, expected, actualInterface(actual))
		}
	}
}

func indirectValue(value reflect.Value) reflect.Value {
	for value.IsValid() && (value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer) {
		if value.IsNil() {
			return reflect.Value{}
		}
		value = value.Elem()
	}
	return value
}

func actualInterface(value reflect.Value) any {
	if !value.IsValid() || !value.CanInterface() {
		return nil
	}
	return value.Interface()
}

func valueType(value reflect.Value) string {
	if !value.IsValid() {
		return "null"
	}
	return value.Type().String()
}

func numericString(value reflect.Value) (string, bool) {
	value = indirectValue(value)
	if !value.IsValid() {
		return "", false
	}
	if number, ok := actualInterface(value).(json.Number); ok {
		return number.String(), true
	}
	switch value.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(value.Int(), 10), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(value.Uint(), 10), true
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(value.Float(), 'g', -1, value.Type().Bits()), true
	default:
		return "", false
	}
}

func isObjectValue(value reflect.Value) bool {
	value = indirectValue(value)
	if !value.IsValid() {
		return false
	}
	if value.Kind() == reflect.Map && value.Type().Key().Kind() == reflect.String {
		return true
	}
	return value.Kind() == reflect.Struct
}

func objectKeys(value reflect.Value) []string {
	value = indirectValue(value)
	if value.Kind() == reflect.Map {
		keys := value.MapKeys()
		out := make([]string, 0, len(keys))
		for _, key := range keys {
			out = append(out, key.String())
		}
		sort.Strings(out)
		return out
	}
	metadata := reflectObjectMetadata(value.Type())
	out := make([]string, 0, len(metadata.fields))
	for _, field := range metadata.fields {
		fieldValue := value.Field(field.index)
		if field.omitEmpty && emptyValue(fieldValue) {
			continue
		}
		out = append(out, field.name)
	}
	return out
}

func objectField(value reflect.Value, name string) (reflect.Value, bool) {
	value = indirectValue(value)
	if value.Kind() == reflect.Map {
		field := value.MapIndex(reflect.ValueOf(name).Convert(value.Type().Key()))
		return field, field.IsValid()
	}
	metadata := reflectObjectMetadata(value.Type())
	for _, index := range metadata.byName[name] {
		fieldValue := value.Field(index)
		if metadata.omitEmpty[index] && emptyValue(fieldValue) {
			continue
		}
		return fieldValue, true
	}
	if _, ok := metadata.byName[name]; !ok {
		return reflect.Value{}, false
	}
	return reflect.Value{}, false
}

func arrayLength(value reflect.Value) (int, bool) {
	value = indirectValue(value)
	if !value.IsValid() || value.Kind() != reflect.Array && value.Kind() != reflect.Slice {
		return 0, false
	}
	return value.Len(), true
}

func emptyValue(value reflect.Value) bool {
	if !value.IsValid() {
		return true
	}
	switch value.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return value.Len() == 0
	case reflect.Bool:
		return !value.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return value.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return value.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return value.Float() == 0
	case reflect.Interface, reflect.Pointer:
		return value.IsNil()
	}
	return false
}

func scalarEqual(expected any, actual reflect.Value) bool {
	actual = indirectValue(actual)
	if expected == nil {
		if !actual.IsValid() {
			return true
		}
		switch actual.Kind() {
		case reflect.Chan, reflect.Func, reflect.Map, reflect.Slice:
			return actual.IsNil()
		default:
			return false
		}
	}
	if !actual.IsValid() {
		return false
	}
	switch value := expected.(type) {
	case string:
		return actual.Kind() == reflect.String && actual.String() == value
	case bool:
		return actual.Kind() == reflect.Bool && actual.Bool() == value
	default:
		return reflect.DeepEqual(value, actualInterface(actual))
	}
}

func addDifference(differences *[]string, format string, args ...any) {
	*differences = append(*differences, fmt.Sprintf(format, args...))
}

func compareSnapshotStream(stream *snapshotStream, document *core.Document, pages []int, selected []string, tolerance float64) ([]string, error) {
	decoder := newCompareJSONDecoder(stream)
	decoder.UseNumber()
	metadataSections := comparisonMetadataSections(selected)
	structureSelected := false
	pageSelected := false
	objectsSelected := false
	mappingSelected := false
	tokensSelected := false
	for _, section := range selected {
		if section == "structure" {
			structureSelected = true
			continue
		}
		if isPageSection(section) {
			pageSelected = true
			continue
		}
		if section == "document.objects" {
			objectsSelected = true
			continue
		}
		if section == "document.mapping" {
			mappingSelected = true
			continue
		}
		if section == "document.tokens" {
			tokensSelected = true
			continue
		}
	}
	expectedHeaderMap, expectedStructure, err := readSnapshotHeader(decoder, structureSelected, headerFieldsForSections(selected))
	if err != nil {
		return nil, fmt.Errorf("decode Playa snapshot header: %w", err)
	}
	if expectedStructure != nil {
		defer func() {
			_ = expectedStructure.Close()
			_ = os.Remove(expectedStructure.Name())
		}()
	}
	actualIndices := pages
	actualMetadataMap, err := snapshotMetadataSections(document, metadataSections)
	if err != nil {
		return nil, fmt.Errorf("snapshot Go metadata: %w", err)
	}
	differences := make([]string, 0, 1)
	compareJSON(projectSections(expectedHeaderMap, metadataSections), projectSections(actualMetadataMap, metadataSections), "", tolerance, &differences)
	if structureSelected {
		if expectedStructure == nil {
			return nil, fmt.Errorf("playa structure projection is missing")
		}
		if _, err := expectedStructure.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}
		file, fileErr := os.CreateTemp("", "go-playa-structure-*.json")
		if fileErr != nil {
			return nil, fmt.Errorf("create structure projection: %w", fileErr)
		}
		path := file.Name()
		defer func() { _ = os.Remove(path) }()
		if err := testcompat.WriteStructureJSON(document, file); err != nil {
			_ = file.Close()
			return nil, err
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			_ = file.Close()
			return nil, err
		}
		if err := compareStructureStream(expectedStructure, file, tolerance, &differences); err != nil {
			_ = file.Close()
			return nil, err
		}
		if err := file.Close(); err != nil {
			return nil, err
		}
	}
	if mappingSelected {
		if err := compareMappingStream(decoder, document, tolerance, &differences); err != nil {
			return nil, err
		}
	}
	if objectsSelected {
		if err := compareObjectsStream(decoder, document, tolerance, &differences); err != nil {
			return nil, err
		}
	}
	if tokensSelected {
		if err := compareTokensStream(decoder, document, tolerance, &differences); err != nil {
			return nil, err
		}
	}
	// Metadata traversal can resolve a large structure tree and many object
	// streams before page comparison starts. It only returns JSON-compatible
	// values, so parsed document objects are no longer needed afterward.
	document.ReleaseTransientCaches()

	pageAt := document.PageAt
	pageProjector := testcompat.NewPageProjector(document)
	var sequentialPages *sequentialPageLookup
	if pageSelected && sequentialPageIndices(actualIndices) {
		sequentialPages = newSequentialPageLookup(document)
		defer sequentialPages.close()
		pageAt = sequentialPages.pageAt
	}
	pageCount := 0
	for pageNumber := 0; ; pageNumber++ {
		err := comparePageRecord(decoder, document, pageProjector, pageAt, actualIndices, pageNumber, pageSelected, selected, tolerance, &differences)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("decode Playa page %d: %w", pageNumber, err)
		}
		if !pageSelected {
			pageCount++
			continue
		}
		pageCount++
		// Document caches already have independent byte budgets. Retaining them
		// across page boundaries is essential for shared fonts, object streams,
		// and decoded resources; clearing every page turns a long document into
		// repeated parsing work. Group and phase boundaries still release all
		// transient state before the next compatibility domain starts.
	}
	if pageSelected && actualIndices != nil && len(actualIndices) != pageCount {
		addDifference(&differences, "pages: missing Playa pages")
	}
	return differences, nil
}

func comparisonMetadataSections(selected []string) []string {
	metadata := make([]string, 0, len(selected))
	for _, section := range selected {
		if section == "structure" || section == "document.objects" || section == "document.mapping" || section == "document.tokens" {
			continue
		}
		if isPageSection(section) && section != "fonts" {
			continue
		}
		metadata = append(metadata, section)
	}
	return metadata
}

func compareObjectsStream(expected jsonTokenDecoder, document *core.Document, tolerance float64, differences *[]string) error {
	temporary, err := os.CreateTemp("", "go-playa-objects-*.jsonl")
	if err != nil {
		return fmt.Errorf("create Go object projection: %w", err)
	}
	path := temporary.Name()
	defer func() { _ = os.Remove(path) }()
	count, writeErr := testcompat.WriteObjectsJSONL(document, temporary)
	if closeErr := temporary.Close(); writeErr == nil && closeErr != nil {
		writeErr = closeErr
	}
	if writeErr != nil {
		return fmt.Errorf("write Go object projection: %w", writeErr)
	}
	actualFile, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open Go object projection: %w", err)
	}
	defer func() { _ = actualFile.Close() }()
	actual := json.NewDecoder(actualFile)
	actual.UseNumber()
	buckets, err := newObjectRecordBuckets()
	if err != nil {
		return err
	}
	defer buckets.close()
	for index := 0; index < count; index++ {
		if err := buckets.copyRecord(expected, true); err != nil {
			return fmt.Errorf("decode Playa object record %d: %w", index, err)
		}
		if err := buckets.copyRecord(actual, false); err != nil {
			return fmt.Errorf("decode Go object record %d: %w", index, err)
		}
	}
	for index := range buckets.expected {
		expectedRecords, err := buckets.read(index, true)
		if err != nil {
			return err
		}
		actualRecords, err := buckets.read(index, false)
		if err != nil {
			return err
		}
		if err := compareObjectRecordSets(expectedRecords, actualRecords, tolerance, differences); err != nil {
			return err
		}
	}
	return nil
}

type expectedMappingRecord struct {
	Kind    string          `json:"kind"`
	Mapping json.RawMessage `json:"mapping"`
}

// compareMappingStream compares the Mapping-style XRef revision stream in
// order. Mapping.Len is the trailer size rather than the number of records,
// so the actual lazy Items sequence controls stream consumption.
func compareMappingStream(expected jsonTokenDecoder, document *core.Document, tolerance float64, differences *[]string) error {
	next, stop := iter.Pull2(document.Items())
	defer stop()
	for index := 0; ; index++ {
		actual, actualErr, ok := next()
		if actualErr != nil {
			return fmt.Errorf("resolve Go mapping record %d: %w", index, actualErr)
		}
		if !ok {
			return nil
		}
		var expectedRecord expectedMappingRecord
		if err := expected.Decode(&expectedRecord); err != nil {
			return fmt.Errorf("decode Playa mapping record %d: %w", index, err)
		}
		if expectedRecord.Kind != "mapping" {
			return fmt.Errorf("expected mapping record, got %q", expectedRecord.Kind)
		}
		actualRecord, err := testcompat.MappingSnapshot(document, actual)
		if err != nil {
			return fmt.Errorf("project Go mapping record %d: %w", index, err)
		}
		actualBytes, err := json.Marshal(actualRecord)
		if err != nil {
			return fmt.Errorf("encode Go mapping record %d: %w", index, err)
		}
		expectedDecoder := json.NewDecoder(bytes.NewReader(expectedRecord.Mapping))
		expectedDecoder.UseNumber()
		actualDecoder := json.NewDecoder(bytes.NewReader(actualBytes))
		actualDecoder.UseNumber()
		if err := compareJSONStreamValue(expectedDecoder, actualDecoder, fmt.Sprintf("mapping[%d]", index), tolerance, differences); err != nil {
			return err
		}
	}
}

const objectRecordBucketCount = 64

type objectRecordKey struct {
	Object     int
	Generation int
}

type objectRecordBuckets struct {
	expected []*os.File
	actual   []*os.File
}

func newObjectRecordBuckets() (*objectRecordBuckets, error) {
	buckets := &objectRecordBuckets{
		expected: make([]*os.File, objectRecordBucketCount),
		actual:   make([]*os.File, objectRecordBucketCount),
	}
	for index := 0; index < objectRecordBucketCount; index++ {
		file, err := os.CreateTemp("", "go-playa-expected-objects-*.jsonl")
		if err != nil {
			buckets.close()
			return nil, fmt.Errorf("create expected object bucket: %w", err)
		}
		buckets.expected[index] = file
		file, err = os.CreateTemp("", "go-playa-actual-objects-*.jsonl")
		if err != nil {
			buckets.close()
			return nil, fmt.Errorf("create Go object bucket: %w", err)
		}
		buckets.actual[index] = file
	}
	return buckets, nil
}

func (buckets *objectRecordBuckets) close() {
	for _, files := range [][]*os.File{buckets.expected, buckets.actual} {
		for _, file := range files {
			if file == nil {
				continue
			}
			name := file.Name()
			_ = file.Close()
			_ = os.Remove(name)
		}
	}
}

func (buckets *objectRecordBuckets) copyRecord(decoder jsonTokenDecoder, expected bool) error {
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return err
	}
	key, err := objectRecordKeyFromJSON(raw)
	if err != nil {
		return err
	}
	index := objectRecordBucket(key)
	files := buckets.actual
	if expected {
		files = buckets.expected
	}
	if _, err := files[index].Write(append(raw, '\n')); err != nil {
		return err
	}
	return nil
}

func (buckets *objectRecordBuckets) read(index int, expected bool) (map[objectRecordKey]json.RawMessage, error) {
	files := buckets.actual
	if expected {
		files = buckets.expected
	}
	file := files[index]
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(file)
	decoder.UseNumber()
	records := make(map[objectRecordKey]json.RawMessage)
	for {
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			if err == io.EOF {
				return records, nil
			}
			return nil, err
		}
		key, err := objectRecordKeyFromJSON(raw)
		if err != nil {
			return nil, err
		}
		records[key] = raw
	}
}

func objectRecordBucket(key objectRecordKey) int {
	value := uint64(uint32(key.Object))<<32 | uint64(uint32(key.Generation))
	return int((value ^ (value >> 33) ^ (value >> 17)) % objectRecordBucketCount)
}

func objectRecordKeyFromJSON(raw []byte) (objectRecordKey, error) {
	var envelope struct {
		Kind   string `json:"kind"`
		Object struct {
			Object     int `json:"object"`
			Generation int `json:"generation"`
		} `json:"object"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return objectRecordKey{}, err
	}
	if envelope.Kind != "object" {
		return objectRecordKey{}, fmt.Errorf("expected object record, got %q", envelope.Kind)
	}
	return objectRecordKey{Object: envelope.Object.Object, Generation: envelope.Object.Generation}, nil
}

func compareObjectRecordSets(expected, actual map[objectRecordKey]json.RawMessage, tolerance float64, differences *[]string) error {
	keys := make([]objectRecordKey, 0, len(expected))
	for key := range expected {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Object != keys[j].Object {
			return keys[i].Object < keys[j].Object
		}
		return keys[i].Generation < keys[j].Generation
	})
	for _, key := range keys {
		expectedRecord := expected[key]
		actualRecord, ok := actual[key]
		path := fmt.Sprintf("objects[%d %d R]", key.Object, key.Generation)
		if !ok {
			addDifference(differences, "%s: missing Go object", path)
			continue
		}
		expectedDecoder := json.NewDecoder(bytes.NewReader(expectedRecord))
		expectedDecoder.UseNumber()
		actualDecoder := json.NewDecoder(bytes.NewReader(actualRecord))
		actualDecoder.UseNumber()
		if err := compareJSONStreamValue(expectedDecoder, actualDecoder, path, tolerance, differences); err != nil {
			return err
		}
	}
	actualKeys := make([]objectRecordKey, 0, len(actual))
	for key := range actual {
		actualKeys = append(actualKeys, key)
	}
	sort.Slice(actualKeys, func(i, j int) bool {
		if actualKeys[i].Object != actualKeys[j].Object {
			return actualKeys[i].Object < actualKeys[j].Object
		}
		return actualKeys[i].Generation < actualKeys[j].Generation
	})
	for _, key := range actualKeys {
		if _, ok := expected[key]; !ok {
			addDifference(differences, "objects[%d %d R]: unexpected Go object", key.Object, key.Generation)
		}
	}
	return nil
}

type expectedTokenRecord struct {
	Kind  string `json:"kind"`
	Token struct {
		Kind  string          `json:"kind"`
		Value json.RawMessage `json:"value"`
	} `json:"token"`
}

func compareTokensStream(expected jsonTokenDecoder, document *core.Document, tolerance float64, differences *[]string) error {
	// Token comparison is already a pair of ordered streams. Do not serialize
	// the Go side to a temporary JSONL file and decode it again: on large PDFs
	// the lexer can produce millions of records, making that round trip the
	// dominant compatibility cost while retaining no useful data.
	next, stop := iter.Pull2(document.Tokens())
	defer stop()
	for index := 0; ; index++ {
		actual, err, ok := next()
		if !ok {
			if err != nil {
				return fmt.Errorf("resolve Go token %d: %w", index, err)
			}
			return nil
		}
		if err != nil {
			return fmt.Errorf("resolve Go token %d: %w", index, err)
		}
		var record expectedTokenRecord
		if err := expected.Decode(&record); err != nil {
			return fmt.Errorf("decode Playa token record %d: %w", index, err)
		}
		compareTokenRecord(record, actual, index, tolerance, differences)
	}
}

func compareTokenRecord(record expectedTokenRecord, token core.Token, index int, tolerance float64, differences *[]string) {
	path := fmt.Sprintf("tokens[%d]", index)
	if record.Kind != "token" {
		addDifference(differences, "%s.kind: playa=%q, want=\"token\"", path, record.Kind)
	}
	actual := testcompat.TokenSnapshot(token)
	if record.Token.Kind != actual.Kind {
		addDifference(differences, "%s.token.kind: playa=%q, go=%q", path, record.Token.Kind, actual.Kind)
	}
	valuePath := path + ".token.value"
	if len(record.Token.Value) == 0 {
		if actual.Value != nil {
			addDifference(differences, "%s: playa value is absent, go=%v", valuePath, actual.Value)
		}
		return
	}
	decoder := json.NewDecoder(bytes.NewReader(record.Token.Value))
	decoder.UseNumber()
	var expectedValue any
	if err := decoder.Decode(&expectedValue); err != nil {
		addDifference(differences, "%s: invalid Playa value: %v", valuePath, err)
		return
	}
	compareJSON(expectedValue, actual.Value, valuePath, tolerance, differences)
}

func compareStructureStream(expectedReader, actualReader io.Reader, tolerance float64, differences *[]string) error {
	expectedDecoder := json.NewDecoder(expectedReader)
	expectedDecoder.UseNumber()
	actualDecoder := json.NewDecoder(actualReader)
	actualDecoder.UseNumber()
	if err := expectJSONDelimiter(expectedDecoder, '[', "expected structure projection"); err != nil {
		return err
	}
	if err := expectJSONDelimiter(actualDecoder, '[', "go structure projection"); err != nil {
		return err
	}
	index := 0
	for {
		expectedMore := expectedDecoder.More()
		actualMore := actualDecoder.More()
		if !expectedMore || !actualMore {
			if expectedMore != actualMore {
				return fmt.Errorf("structure projection length differs at element %d", index)
			}
			break
		}
		if err := compareJSONStreamValue(expectedDecoder, actualDecoder, fmt.Sprintf("structure[%d]", index), tolerance, differences); err != nil {
			return err
		}
		index++
	}
	if err := expectJSONDelimiter(expectedDecoder, ']', "expected structure projection"); err != nil {
		return err
	}
	return expectJSONDelimiter(actualDecoder, ']', "go structure projection")
}

func expectJSONDelimiter(decoder jsonTokenDecoder, want json.Delim, label string) error {
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("decode %s: %w", label, err)
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != want {
		return fmt.Errorf("%s has unexpected token %v", label, token)
	}
	return nil
}

func compareJSONStreamValue(expected, actual jsonTokenDecoder, path string, tolerance float64, differences *[]string) error {
	expectedToken, err := expected.Token()
	if err != nil {
		return fmt.Errorf("decode expected %s: %w", path, err)
	}
	actualToken, err := actual.Token()
	if err != nil {
		return fmt.Errorf("decode go %s: %w", path, err)
	}
	expectedDelimiter, expectedIsDelimiter := expectedToken.(json.Delim)
	actualDelimiter, actualIsDelimiter := actualToken.(json.Delim)
	if expectedIsDelimiter || actualIsDelimiter {
		if !expectedIsDelimiter || !actualIsDelimiter || expectedDelimiter != actualDelimiter {
			addDifference(differences, "%s: structure token mismatch: playa=%v, go=%v", path, expectedToken, actualToken)
			return nil
		}
		if expectedDelimiter == '{' && unorderedPropertyObject(path) {
			return compareUnorderedPropertyObject(expected, actual, path, tolerance, differences)
		}
		switch expectedDelimiter {
		case '{':
			for {
				expectedMore := expected.More()
				actualMore := actual.More()
				if !expectedMore || !actualMore {
					if expectedMore != actualMore {
						addDifference(differences, "%s: object field count differs", path)
					}
					break
				}
				expectedKey, expectedErr := expected.Token()
				if expectedErr != nil {
					return expectedErr
				}
				actualKey, actualErr := actual.Token()
				if actualErr != nil {
					return actualErr
				}
				keyExpected, expectedOK := expectedKey.(string)
				keyActual, actualOK := actualKey.(string)
				if !expectedOK || !actualOK {
					return fmt.Errorf("%s has a non-string object key", path)
				}
				if keyExpected != keyActual {
					addDifference(differences, "%s: object key mismatch: playa=%q, go=%q", path, keyExpected, keyActual)
				}
				if err := compareJSONStreamValue(expected, actual, path+"."+keyExpected, tolerance, differences); err != nil {
					return err
				}
			}
			if err := expectJSONDelimiter(expected, '}', "expected "+path); err != nil {
				return err
			}
			return expectJSONDelimiter(actual, '}', "go "+path)
		case '[':
			index := 0
			for {
				expectedMore := expected.More()
				actualMore := actual.More()
				if !expectedMore || !actualMore {
					if expectedMore != actualMore {
						addDifference(differences, "%s: array length differs", path)
					}
					break
				}
				if err := compareJSONStreamValue(expected, actual, fmt.Sprintf("%s[%d]", path, index), tolerance, differences); err != nil {
					return err
				}
				index++
			}
			if err := expectJSONDelimiter(expected, ']', "expected "+path); err != nil {
				return err
			}
			return expectJSONDelimiter(actual, ']', "go "+path)
		default:
			return nil
		}
	}
	compareJSON(expectedToken, actualToken, path, tolerance, differences)
	return nil
}

func unorderedPropertyObject(path string) bool {
	return strings.HasSuffix(path, ".properties") || strings.HasSuffix(path, ".marked_properties") || strings.HasSuffix(path, ".attributes") || strings.Contains(path, ".attributes.") || strings.Contains(path, ".object.value") || strings.Contains(path, ".stream.dict") || strings.Contains(path, "mapping[")
}

func compareUnorderedPropertyObject(expected, actual jsonTokenDecoder, path string, tolerance float64, differences *[]string) error {
	readObject := func(decoder jsonTokenDecoder) (map[string]json.RawMessage, error) {
		values := map[string]json.RawMessage{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			key, ok := keyToken.(string)
			if !ok {
				return nil, fmt.Errorf("%s has a non-string object key", path)
			}
			var value json.RawMessage
			if err := decoder.Decode(&value); err != nil {
				return nil, err
			}
			values[key] = value
		}
		if _, err := decoder.Token(); err != nil {
			return nil, err
		}
		return values, nil
	}
	expectedValues, err := readObject(expected)
	if err != nil {
		return err
	}
	actualValues, err := readObject(actual)
	if err != nil {
		return err
	}
	expectedKeys := make([]string, 0, len(expectedValues))
	for key := range expectedValues {
		expectedKeys = append(expectedKeys, key)
	}
	sort.Strings(expectedKeys)
	for _, key := range expectedKeys {
		expectedValue := expectedValues[key]
		actualValue, ok := actualValues[key]
		child := path + "." + key
		if !ok {
			addDifference(differences, "%s: missing Go value", child)
			continue
		}
		expectedDecoder := json.NewDecoder(bytes.NewReader(expectedValue))
		expectedDecoder.UseNumber()
		actualDecoder := json.NewDecoder(bytes.NewReader(actualValue))
		actualDecoder.UseNumber()
		if err := compareJSONStreamValue(expectedDecoder, actualDecoder, child, tolerance, differences); err != nil {
			return err
		}
	}
	actualKeys := make([]string, 0, len(actualValues))
	for key := range actualValues {
		actualKeys = append(actualKeys, key)
	}
	sort.Strings(actualKeys)
	for _, key := range actualKeys {
		if _, ok := expectedValues[key]; !ok {
			addDifference(differences, "%s: unexpected Go value", path+"."+key)
		}
	}
	return nil
}

func headerFieldsForSections(sections []string) map[string]bool {
	fields := map[string]bool{"schema_version": true}
	for _, section := range sections {
		switch section {
		case "document":
			fields["page_count"] = true
			fields["page_labels"] = true
			fields["pdf_version"] = true
			fields["is_tagged"] = true
			fields["is_printable"] = true
			fields["is_modifiable"] = true
			fields["is_extractable"] = true
			fields["info"] = true
			fields["catalog"] = true
			fields["names"] = true
			fields["trailer"] = true
			fields["open_action"] = true
		case "document.buffer":
			fields["buffer"] = true
		case "document.xrefs":
			fields["xrefs"] = true
		case "destinations", "outline":
			fields[section] = true
		case "forms":
			fields["need_appearances"] = true
			fields["forms"] = true
		case "fonts":
			fields["document_fonts"] = true
		}
	}
	return fields
}

func readSnapshotHeader(decoder jsonTokenDecoder, wantStructure bool, fields map[string]bool) (map[string]any, *os.File, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, nil, err
	}
	start, ok := token.(json.Delim)
	if !ok || start != '{' {
		return nil, nil, fmt.Errorf("playa snapshot stream has an invalid header")
	}
	var kind string
	var snapshot map[string]any
	var structureFile *os.File
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return nil, nil, err
		}
		key, ok := keyToken.(string)
		if !ok {
			return nil, nil, fmt.Errorf("playa snapshot header has a non-string key")
		}
		switch key {
		case "kind":
			if err := decoder.Decode(&kind); err != nil {
				return nil, nil, err
			}
		case "snapshot":
			snapshot, structureFile, err = readSnapshotObject(decoder, wantStructure, fields)
			if err != nil {
				return nil, nil, err
			}
		default:
			var ignored any
			if err := decoder.Decode(&ignored); err != nil {
				return nil, nil, err
			}
		}
	}
	if _, err := decoder.Token(); err != nil {
		return nil, nil, err
	}
	if kind != "header" || snapshot == nil {
		return nil, nil, fmt.Errorf("playa snapshot stream has an invalid header")
	}
	return snapshot, structureFile, nil
}

func readSnapshotObject(decoder jsonTokenDecoder, wantStructure bool, fields map[string]bool) (map[string]any, *os.File, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, nil, err
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '{' {
		return nil, nil, fmt.Errorf("playa snapshot header must be a JSON object")
	}
	result := map[string]any{}
	var structureFile *os.File
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return nil, nil, err
		}
		key, ok := keyToken.(string)
		if !ok {
			return nil, nil, fmt.Errorf("playa snapshot object has a non-string key")
		}
		if key == "structure" {
			if wantStructure {
				structureFile, err = os.CreateTemp("", "go-playa-expected-structure-*.json")
				if err != nil {
					return nil, nil, err
				}
			}
			var writer io.Writer
			if structureFile != nil {
				writer = structureFile
			}
			if err := copyJSONValue(decoder, writer); err != nil {
				if structureFile != nil {
					_ = structureFile.Close()
				}
				return nil, nil, fmt.Errorf("read snapshot field %q: %w", key, err)
			}
			continue
		}
		if !fields[key] {
			if err := copyJSONValue(decoder, nil); err != nil {
				return nil, nil, fmt.Errorf("read snapshot field %q: %w", key, err)
			}
			continue
		}
		var value any
		if err := decoder.Decode(&value); err != nil {
			return nil, nil, fmt.Errorf("decode snapshot field %q: %w", key, err)
		}
		result[key] = value
	}
	if _, err := decoder.Token(); err != nil {
		return nil, nil, err
	}
	return result, structureFile, nil
}

func copyJSONValue(decoder jsonTokenDecoder, writer io.Writer) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	return copyJSONToken(decoder, token, writer)
}

func copyJSONToken(decoder jsonTokenDecoder, token any, writer io.Writer) error {
	if writer != nil {
		if err := writeJSONToken(writer, token); err != nil {
			return err
		}
	}
	switch delimiter := token.(type) {
	case json.Delim:
		switch delimiter {
		case '{':
			first := true
			for decoder.More() {
				key, err := decoder.Token()
				if err != nil {
					return err
				}
				if writer != nil {
					if !first {
						if _, err := io.WriteString(writer, ","); err != nil {
							return err
						}
					}
					first = false
					if err := writeJSONToken(writer, key); err != nil {
						return err
					}
					if _, err := io.WriteString(writer, ":"); err != nil {
						return err
					}
				}
				if err := copyJSONValue(decoder, writer); err != nil {
					return err
				}
			}
		case '[':
			first := true
			for decoder.More() {
				if writer != nil && !first {
					if _, err := io.WriteString(writer, ","); err != nil {
						return err
					}
				}
				first = false
				if err := copyJSONValue(decoder, writer); err != nil {
					return err
				}
			}
		}
		end, err := decoder.Token()
		if err != nil {
			return err
		}
		if writer != nil {
			return writeJSONToken(writer, end)
		}
	}
	return nil
}

func writeJSONToken(writer io.Writer, token any) error {
	if delimiter, ok := token.(json.Delim); ok {
		_, err := io.WriteString(writer, string(delimiter))
		return err
	}
	data, err := json.Marshal(token)
	if err != nil {
		return err
	}
	_, err = writer.Write(data)
	return err
}

func snapshotMetadataSections(document *core.Document, sections []string) (map[string]any, error) {
	report, err := testcompat.SnapshotMetadata(document, sections)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		return nil, fmt.Errorf("encode Go snapshot metadata: %w", err)
	}
	decoded, err := decodeJSON(encoded)
	if err != nil {
		return nil, err
	}
	metadata, ok := decoded.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("go snapshot metadata must be a JSON object")
	}
	return projectSections(metadata, sections), nil
}

func comparePageRecord(decoder jsonTokenDecoder, document *core.Document, projector *testcompat.PageProjector, pageAt func(int) (core.Page, error), actualIndices []int, pageNumber int, pageSelected bool, selected []string, tolerance float64, differences *[]string) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	start, ok := token.(json.Delim)
	if !ok || start != '{' {
		return fmt.Errorf("playa snapshot page record %d must be an object", pageNumber)
	}
	kind := ""
	pageSeen := false
	for decoder.More() {
		keyToken, keyErr := decoder.Token()
		if keyErr != nil {
			return keyErr
		}
		key, ok := keyToken.(string)
		if !ok {
			return fmt.Errorf("playa page record %d has a non-string field name", pageNumber)
		}
		switch key {
		case "kind":
			if err := decoder.Decode(&kind); err != nil {
				return err
			}
		case "page":
			pageSeen = true
			if actualIndices != nil && pageNumber >= len(actualIndices) {
				if err := copyJSONValue(decoder, nil); err != nil {
					return err
				}
				continue
			}
			if pageSelected {
				pageIndex := pageNumber
				if actualIndices != nil {
					pageIndex = actualIndices[pageNumber]
				}
				page, pageErr := pageAt(pageIndex)
				if pageErr != nil {
					return fmt.Errorf("resolve Go page %d: %w", pageIndex, pageErr)
				}
				if err := comparePageObject(decoder, document, projector, page, pageIndex, pageNumber, selected, tolerance, differences); err != nil {
					return err
				}
			} else if err := copyJSONValue(decoder, nil); err != nil {
				return err
			}
		default:
			if err := copyJSONValue(decoder, nil); err != nil {
				return err
			}
		}
	}
	if _, err := decoder.Token(); err != nil {
		return err
	}
	if kind != "page" || !pageSeen {
		return fmt.Errorf("playa snapshot stream has an invalid page record %d", pageNumber)
	}
	if actualIndices != nil && pageNumber >= len(actualIndices) {
		addDifference(differences, "pages: unexpected Playa page %d", pageNumber)
	}
	return nil
}

func comparePageObject(decoder jsonTokenDecoder, document *core.Document, projector *testcompat.PageProjector, page core.Page, pageIndex, pageNumber int, selected []string, tolerance float64, differences *[]string) error {
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("decode Playa page %d: %w", pageNumber, err)
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '{' {
		return fmt.Errorf("playa snapshot page %d must be an object", pageNumber)
	}
	has := make(map[string]bool, len(selected))
	for _, section := range selected {
		has[section] = true
	}
	actualBySection := make(map[string]testcompat.Page)
	path := fmt.Sprintf("pages[%d]", pageNumber)
	for decoder.More() {
		keyToken, keyErr := decoder.Token()
		if keyErr != nil {
			return fmt.Errorf("decode Playa page %d field: %w", pageNumber, keyErr)
		}
		key, ok := keyToken.(string)
		if !ok {
			return fmt.Errorf("playa page %d has a non-string field name", pageNumber)
		}
		if !pageFieldSelected(key, has) {
			if err := copyJSONValue(decoder, nil); err != nil {
				return fmt.Errorf("skip Playa page %d field %q: %w", pageNumber, key, err)
			}
			continue
		}
		if key == "index" {
			if err := compareJSONStreamReflect(decoder, reflect.ValueOf(pageIndex), path+"."+key, tolerance, differences); err != nil {
				return err
			}
			continue
		}
		if key == "paths" {
			if err := comparePathsStream(decoder, document, page, path+"."+key, tolerance, differences); err != nil {
				return err
			}
			continue
		}
		if key == "text" {
			if err := compareTextStream(decoder, document, page, path+"."+key, tolerance, differences); err != nil {
				return err
			}
			continue
		}
		if key == "images" {
			if err := compareImageStream(decoder, document, page, path+"."+key, tolerance, differences); err != nil {
				return err
			}
			continue
		}
		if key == "fonts" {
			if err := compareFontStream(decoder, document, page, path+"."+key, tolerance, differences); err != nil {
				return err
			}
			continue
		}
		if key == "tags" {
			if err := compareTagStream(decoder, document, page, path+"."+key, tolerance, differences); err != nil {
				return err
			}
			continue
		}
		if key == "annotations" {
			if err := compareAnnotationStream(decoder, document, page, path+"."+key, tolerance, differences); err != nil {
				return err
			}
			continue
		}
		section := pageSectionForField(key)
		actualPage, ok := actualBySection[section]
		if !ok {
			var pageErr error
			requestedSections := []string{section}
			// XObject records contain their own Contents projection. Keep that
			// nested field populated whenever the page comparison selected both
			// related domains, even though the top-level fields are compared in
			// separate passes.
			if section == "content.xobjects" && has["content.contents"] {
				requestedSections = append(requestedSections, "content.contents")
			}
			actualPage, pageErr = projector.PageSnapshotSections(page, pageIndex, requestedSections)
			if pageErr != nil {
				return pageErr
			}
			actualBySection[section] = actualPage
		}
		actualValue := reflect.ValueOf(pageFieldValue(actualPage, key))
		if typedPageField(key) {
			if err := compareTypedJSONStreamReflect(decoder, actualValue, path+"."+key, tolerance, differences); err != nil {
				return err
			}
		} else if err := compareJSONStreamReflect(decoder, actualValue, path+"."+key, tolerance, differences); err != nil {
			return err
		}
		if section != "pages" {
			delete(actualBySection, section)
		}
	}
	if _, err := decoder.Token(); err != nil {
		return fmt.Errorf("finish Playa page %d: %w", pageNumber, err)
	}
	return nil
}

func typedPageField(field string) bool {
	switch field {
	case "glyphs", "flatten", "interp", "streams", "tokens", "xobjects", "layout":
		return true
	default:
		return false
	}
}

func compareAnnotationStream(expected jsonTokenDecoder, document *core.Document, page core.Page, path string, tolerance float64, differences *[]string) error {
	token, err := expected.Token()
	if err != nil {
		return fmt.Errorf("decode expected %s: %w", path, err)
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '[' {
		addDifference(differences, "%s: playa type=%v, want array", path, token)
		return skipJSONValue(expected)
	}
	next, stop := iter.Pull2(document.Annotations(page))
	defer stop()
	index := 0
	for expected.More() {
		annotation, annotationErr, ok := next()
		if !ok {
			if annotationErr != nil {
				return fmt.Errorf("resolve Go annotation %d: %w", index, annotationErr)
			}
			addDifference(differences, "%s: playa has more values than Go", path)
			if err := skipJSONValue(expected); err != nil {
				return err
			}
			continue
		}
		if annotationErr != nil {
			return fmt.Errorf("resolve Go annotation %d: %w", index, annotationErr)
		}
		actual, snapshotErr := testcompat.AnnotationSnapshotWithDocument(document, annotation, page.Ref(), page.Index())
		if snapshotErr != nil {
			return fmt.Errorf("snapshot Go annotation %d: %w", index, snapshotErr)
		}
		if err := compareRawJSONValueStreamReflect(expected, reflect.ValueOf(actual), fmt.Sprintf("%s[%d]", path, index), tolerance, differences); err != nil {
			return err
		}
		index++
	}
	if err := expectJSONDelimiter(expected, ']', "expected "+path); err != nil {
		return err
	}
	if _, annotationErr, ok := next(); ok {
		_ = annotationErr
		addDifference(differences, "%s: Go has more values than Playa", path)
	} else if annotationErr != nil {
		return fmt.Errorf("resolve Go annotation %d: %w", index, annotationErr)
	}
	return nil
}

func compareTagStream(expected jsonTokenDecoder, document *core.Document, page core.Page, path string, tolerance float64, differences *[]string) error {
	token, err := expected.Token()
	if err != nil {
		return fmt.Errorf("decode expected %s: %w", path, err)
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '[' {
		addDifference(differences, "%s: playa type=%v, want array", path, token)
		return skipJSONValue(expected)
	}
	next, stop := iter.Pull2(document.PageTagsSeq(page))
	defer stop()
	nextUsable := func() (testcompat.Tag, bool, error) {
		for {
			tag, tagErr, ok := next()
			if !ok {
				return testcompat.Tag{}, false, tagErr
			}
			if tagErr != nil {
				return testcompat.Tag{}, false, tagErr
			}
			if !tag.HasMCID() && tag.MarkedTag() == "P" {
				continue
			}
			return testcompat.TagSnapshot(tag, page.Index()), true, nil
		}
	}
	index := 0
	for expected.More() {
		actual, ok, tagErr := nextUsable()
		if !ok {
			if tagErr != nil {
				return fmt.Errorf("resolve Go tag %d: %w", index, tagErr)
			}
			addDifference(differences, "%s: playa has more values than Go", path)
			if err := skipJSONValue(expected); err != nil {
				return err
			}
			continue
		}
		if err := compareRawJSONValueStreamReflect(expected, reflect.ValueOf(actual), fmt.Sprintf("%s[%d]", path, index), tolerance, differences); err != nil {
			return err
		}
		index++
	}
	if err := expectJSONDelimiter(expected, ']', "expected "+path); err != nil {
		return err
	}
	_, extra, tagErr := nextUsable()
	if tagErr != nil {
		return fmt.Errorf("resolve Go tag %d: %w", index, tagErr)
	}
	if extra {
		addDifference(differences, "%s: Go has more values than Playa", path)
	}
	return nil
}

func compareFontStream(expected jsonTokenDecoder, document *core.Document, page core.Page, path string, tolerance float64, differences *[]string) error {
	token, err := expected.Token()
	if err != nil {
		return fmt.Errorf("decode expected %s: %w", path, err)
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '[' {
		addDifference(differences, "%s: playa type=%v, want array", path, token)
		return skipJSONValue(expected)
	}
	next, stop := iter.Pull2(document.PageFontSeq(page))
	defer stop()
	nextUsable := func() (testcompat.Font, bool, error) {
		for {
			resource, fontErr, ok := next()
			if !ok {
				return testcompat.Font{}, false, fontErr
			}
			if fontErr != nil {
				return testcompat.Font{}, false, fontErr
			}
			font, usable := testcompat.FontSnapshot(resource)
			if usable {
				return font, true, nil
			}
		}
	}
	index := 0
	for expected.More() {
		actual, ok, fontErr := nextUsable()
		if !ok {
			if fontErr != nil {
				return fmt.Errorf("resolve Go font %d: %w", index, fontErr)
			}
			addDifference(differences, "%s: playa has more values than Go", path)
			if err := skipJSONValue(expected); err != nil {
				return err
			}
			continue
		}
		if err := compareRawJSONValueStreamReflect(expected, reflect.ValueOf(actual), fmt.Sprintf("%s[%d]", path, index), tolerance, differences); err != nil {
			return err
		}
		index++
	}
	if err := expectJSONDelimiter(expected, ']', "expected "+path); err != nil {
		return err
	}
	_, extra, fontErr := nextUsable()
	if fontErr != nil {
		return fmt.Errorf("resolve Go font %d: %w", index, fontErr)
	}
	if extra {
		addDifference(differences, "%s: Go has more values than Playa", path)
	}
	return nil
}

func compareImageStream(expected jsonTokenDecoder, document *core.Document, page core.Page, path string, tolerance float64, differences *[]string) error {
	token, err := expected.Token()
	if err != nil {
		return fmt.Errorf("decode expected %s: %w", path, err)
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '[' {
		addDifference(differences, "%s: playa type=%v, want array", path, token)
		return skipJSONValue(expected)
	}
	next, stop := iter.Pull2(document.PageImagesSeq(page))
	defer stop()
	index := 0
	for expected.More() {
		image, imageErr, ok := next()
		if !ok {
			if imageErr != nil {
				return fmt.Errorf("resolve Go image %d: %w", index, imageErr)
			}
			addDifference(differences, "%s: playa has more values than Go", path)
			if err := skipJSONValue(expected); err != nil {
				return err
			}
			continue
		}
		if imageErr != nil {
			return fmt.Errorf("resolve Go image %d: %w", index, imageErr)
		}
		actual := testcompat.ImageSnapshot(image, page.Index())
		if err := compareRawJSONValueStreamReflect(expected, reflect.ValueOf(actual), fmt.Sprintf("%s[%d]", path, index), tolerance, differences); err != nil {
			return err
		}
		index++
	}
	if err := expectJSONDelimiter(expected, ']', "expected "+path); err != nil {
		return err
	}
	if _, imageErr, ok := next(); ok {
		_ = imageErr
		addDifference(differences, "%s: Go has more values than Playa", path)
	} else if imageErr != nil {
		return fmt.Errorf("resolve Go image %d: %w", index, imageErr)
	}
	return nil
}

func compareTextStream(expected jsonTokenDecoder, document *core.Document, page core.Page, path string, tolerance float64, differences *[]string) error {
	token, err := expected.Token()
	if err != nil {
		return fmt.Errorf("decode expected %s: %w", path, err)
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '[' {
		addDifference(differences, "%s: playa type=%v, want array", path, token)
		return skipJSONValue(expected)
	}
	next, stop := iter.Pull2(page.Texts(document))
	defer stop()
	target := rawJSONReflectTarget{tolerance: tolerance}
	index := 0
	for expected.More() {
		textObject, textErr, ok := next()
		if !ok {
			if textErr != nil {
				return fmt.Errorf("resolve Go text %d: %w", index, textErr)
			}
			addDifference(differences, "%s: playa has more values than Go", path)
			if err := skipJSONValue(expected); err != nil {
				return err
			}
			continue
		}
		if textErr != nil {
			return fmt.Errorf("resolve Go text %d: %w", index, textErr)
		}
		actual := testcompat.TextSnapshot(textObject, page.Index())
		for glyph, glyphErr := range textObject.GlyphsSeq() {
			if glyphErr != nil {
				return fmt.Errorf("resolve Go text %d glyph: %w", index, glyphErr)
			}
			projected, glyphSnapshotErr := testcompat.GlyphSnapshot(document, glyph, page.Index())
			if glyphSnapshotErr != nil {
				return fmt.Errorf("resolve Go text %d glyph children: %w", index, glyphSnapshotErr)
			}
			actual.Glyphs = append(actual.Glyphs, projected)
		}
		target.actual = reflect.ValueOf(actual)
		target.mismatch = typedMismatch{}
		target.different = false
		if err := expected.Decode(&target); err != nil {
			return fmt.Errorf("decode typed expected %s[%d]: %w", path, index, err)
		}
		if target.different {
			addDifference(differences, "%s[%d]%s: playa=%v, go=%v", path, index, target.mismatch.path, target.mismatch.expected, target.mismatch.actual)
		}
		index++
	}
	if err := expectJSONDelimiter(expected, ']', "expected "+path); err != nil {
		return err
	}
	if _, textErr, ok := next(); ok {
		_ = textErr
		addDifference(differences, "%s: Go has more values than Playa", path)
	} else if textErr != nil {
		return fmt.Errorf("resolve Go text %d: %w", index, textErr)
	}
	return nil
}

func comparePathsStream(expected jsonTokenDecoder, document *core.Document, page core.Page, path string, tolerance float64, differences *[]string) error {
	token, err := expected.Token()
	if err != nil {
		return fmt.Errorf("decode expected %s: %w", path, err)
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '[' {
		addDifference(differences, "%s: playa type=%v, want array", path, token)
		return skipJSONValue(expected)
	}
	next, stop := iter.Pull2(document.PagePathsSeq(page))
	defer stop()
	index := 0
	for expected.More() {
		actual, actualErr, ok := next()
		if !ok {
			if actualErr != nil {
				return fmt.Errorf("resolve Go path %d: %w", index, actualErr)
			}
			addDifference(differences, "%s: playa has more values than Go", path)
			if err := skipJSONValue(expected); err != nil {
				return err
			}
			continue
		}
		if actualErr != nil {
			return fmt.Errorf("resolve Go path %d: %w", index, actualErr)
		}
		if err := compareRawJSONValueStreamReflect(expected, reflect.ValueOf(testcompat.PathSnapshot(actual, page.Index())), fmt.Sprintf("%s[%d]", path, index), tolerance, differences); err != nil {
			return err
		}
		index++
	}
	if err := expectJSONDelimiter(expected, ']', "expected "+path); err != nil {
		return err
	}
	if _, actualErr, ok := next(); ok {
		_ = actualErr
		addDifference(differences, "%s: Go has more values than Playa", path)
	} else if actualErr != nil {
		return fmt.Errorf("resolve Go path %d: %w", index, actualErr)
	}
	return nil
}

// compareJSONStreamReflect compares the expected JSON token stream directly
// with the typed Go projection. Marshaling the complete actual value first
// doubles the peak memory for large path/text arrays and defeats JSONL's
// bounded-memory contract.
func compareJSONStreamReflect(expected jsonTokenDecoder, actual reflect.Value, path string, tolerance float64, differences *[]string) error {
	expectedToken, err := expected.Token()
	if err != nil {
		return fmt.Errorf("decode expected %s: %w", path, err)
	}
	actual = indirectValue(actual)
	if delimiter, ok := expectedToken.(json.Delim); ok {
		switch delimiter {
		case '{':
			if !isObjectValue(actual) {
				addDifference(differences, "%s: playa type=object, go type=%s", path, valueType(actual))
				return skipJSONContainer(expected, '}')
			}
			if unorderedPropertyObject(path) {
				return compareUnorderedPropertyReflect(expected, actual, path, tolerance, differences)
			}
			seen := map[string]bool{}
			for expected.More() {
				keyToken, keyErr := expected.Token()
				if keyErr != nil {
					return keyErr
				}
				key, ok := keyToken.(string)
				if !ok {
					return fmt.Errorf("%s has a non-string object key", path)
				}
				seen[key] = true
				field, found := objectField(actual, key)
				if !found {
					addDifference(differences, "%s: missing Go value", joinJSONPath(path, key))
					if err := skipJSONValue(expected); err != nil {
						return err
					}
					continue
				}
				if err := compareJSONStreamReflect(expected, field, joinJSONPath(path, key), tolerance, differences); err != nil {
					return err
				}
			}
			if err := expectJSONDelimiter(expected, '}', "expected "+path); err != nil {
				return err
			}
			for _, key := range objectKeys(actual) {
				if !seen[key] {
					addDifference(differences, "%s: unexpected Go value", joinJSONPath(path, key))
				}
			}
			return nil
		case '[':
			length, ok := arrayLength(actual)
			if !ok {
				addDifference(differences, "%s: playa type=array, go type=%s", path, valueType(actual))
				return skipJSONContainer(expected, ']')
			}
			index := 0
			for expected.More() {
				if index >= length {
					addDifference(differences, "%s: playa array has more values than Go", path)
					if err := skipJSONValue(expected); err != nil {
						return err
					}
					continue
				}
				if err := compareJSONStreamReflect(expected, actual.Index(index), fmt.Sprintf("%s[%d]", path, index), tolerance, differences); err != nil {
					return err
				}
				index++
			}
			if err := expectJSONDelimiter(expected, ']', "expected "+path); err != nil {
				return err
			}
			if index != length {
				addDifference(differences, "%s: playa length=%d, go length=%d", path, index, length)
			}
			return nil
		}
	}
	compareJSON(expectedToken, actualInterface(actual), path, tolerance, differences)
	return nil
}

func compareTypedJSONStreamReflect(expected jsonTokenDecoder, actual reflect.Value, path string, tolerance float64, differences *[]string) error {
	actual = indirectValue(actual)
	if !actual.IsValid() {
		return compareJSONStreamReflect(expected, actual, path, tolerance, differences)
	}
	expected.DisallowUnknownFields()
	if actual.Kind() == reflect.Array || actual.Kind() == reflect.Slice {
		return compareTypedJSONArrayStreamReflect(expected, actual, path, tolerance, differences)
	}
	return compareRawJSONValueStreamReflect(expected, actual, path, tolerance, differences)
}

func compareRawJSONValueStreamReflect(expected jsonTokenDecoder, actual reflect.Value, path string, tolerance float64, differences *[]string) error {
	target := rawJSONReflectTarget{actual: actual, tolerance: tolerance}
	if err := expected.Decode(&target); err != nil {
		return fmt.Errorf("decode typed expected %s: %w", path, err)
	}
	if target.different {
		addDifference(differences, "%s%s: playa=%v, go=%v", path, target.mismatch.path, target.mismatch.expected, target.mismatch.actual)
	}
	return nil
}

func compareTypedJSONArrayStreamReflect(expected jsonTokenDecoder, actual reflect.Value, path string, tolerance float64, differences *[]string) error {
	token, err := expected.Token()
	if err != nil {
		return err
	}
	if token == nil {
		if actual.Kind() != reflect.Slice || !actual.IsNil() {
			addDifference(differences, "%s: playa=<nil>, go=%v", path, actualInterface(actual))
		}
		return nil
	}
	delimiter, ok := token.(json.Delim)
	if !ok || delimiter != '[' {
		addDifference(differences, "%s: expected array, playa=%v, go=%v", path, token, actualInterface(actual))
		return nil
	}
	if actual.Kind() == reflect.Slice && actual.IsNil() {
		addDifference(differences, "%s: playa=array, go=<nil>", path)
	}
	target := rawJSONReflectTarget{tolerance: tolerance}
	index := 0
	for expected.More() {
		if index >= actual.Len() {
			if err := skipJSONValue(expected); err != nil {
				return err
			}
			index++
			continue
		}
		target.actual = actual.Index(index)
		target.mismatch = typedMismatch{}
		target.different = false
		if err := expected.Decode(&target); err != nil {
			return fmt.Errorf("decode typed expected %s[%d]: %w", path, index, err)
		}
		if target.different {
			addDifference(differences, "%s[%d]%s: playa=%v, go=%v", path, index, target.mismatch.path, target.mismatch.expected, target.mismatch.actual)
		}
		index++
	}
	if err := expectJSONDelimiter(expected, ']', "expected "+path); err != nil {
		return err
	}
	if index != actual.Len() {
		addDifference(differences, "%s: playa length=%d, go length=%d", path, index, actual.Len())
	}
	return nil
}

type rawJSONReflectTarget struct {
	actual    reflect.Value
	tolerance float64
	mismatch  typedMismatch
	different bool
}

func (target *rawJSONReflectTarget) UnmarshalJSON(data []byte) error {
	mismatch, different, err := compareRawJSONReflect(data, target.actual, target.tolerance)
	if err != nil {
		return err
	}
	target.mismatch = mismatch
	target.different = different
	return nil
}

type rawJSONReflectComparator struct {
	data      []byte
	position  int
	tolerance float64
}

func compareRawJSONReflect(data []byte, actual reflect.Value, tolerance float64) (typedMismatch, bool, error) {
	comparator := rawJSONReflectComparator{data: data, tolerance: tolerance}
	mismatch, ok, err := comparator.compareValue(actual)
	if err != nil {
		return typedMismatch{}, false, err
	}
	if ok {
		return mismatch, true, nil
	}
	comparator.skipSpace()
	if comparator.position != len(comparator.data) {
		return typedMismatch{}, false, fmt.Errorf("unexpected JSON after byte %d", comparator.position)
	}
	return mismatch, ok, nil
}

func (comparator *rawJSONReflectComparator) compareValue(actual reflect.Value) (typedMismatch, bool, error) {
	comparator.skipSpace()
	if comparator.position >= len(comparator.data) {
		return typedMismatch{}, false, io.ErrUnexpectedEOF
	}
	switch comparator.data[comparator.position] {
	case 'n':
		if err := comparator.consumeLiteral("null"); err != nil {
			return typedMismatch{}, false, err
		}
		if isNilReflectValue(actual) {
			return typedMismatch{}, false, nil
		}
		concrete := indirectValue(actual)
		if concrete.IsValid() {
			zero := reflect.Zero(concrete.Type())
			if mismatch, ok := typedJSONMismatch(zero, concrete, comparator.tolerance); ok {
				return mismatch, true, nil
			}
			return typedMismatch{}, false, nil
		}
		return typedMismatch{expected: nil, actual: actualInterface(actual)}, true, nil
	case 't':
		return comparator.compareBool(actual, true, "true")
	case 'f':
		return comparator.compareBool(actual, false, "false")
	case '"':
		return comparator.compareString(actual)
	case '[':
		return comparator.compareArray(actual)
	case '{':
		return comparator.compareObject(actual)
	default:
		return comparator.compareNumber(actual)
	}
}

func isNilReflectValue(value reflect.Value) bool {
	if !value.IsValid() {
		return true
	}
	switch value.Kind() {
	case reflect.Interface, reflect.Pointer, reflect.Map, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func (comparator *rawJSONReflectComparator) compareBool(actual reflect.Value, expected bool, literal string) (typedMismatch, bool, error) {
	if err := comparator.consumeLiteral(literal); err != nil {
		return typedMismatch{}, false, err
	}
	actual = indirectValue(actual)
	if actual.IsValid() && actual.Kind() == reflect.Bool && actual.Bool() == expected {
		return typedMismatch{}, false, nil
	}
	return typedMismatch{expected: expected, actual: actualInterface(actual)}, true, nil
}

func (comparator *rawJSONReflectComparator) compareString(actual reflect.Value) (typedMismatch, bool, error) {
	raw, decoded, escaped, err := comparator.readString()
	if err != nil {
		return typedMismatch{}, false, err
	}
	actual = indirectValue(actual)
	if !actual.IsValid() || actual.Kind() != reflect.String {
		if !escaped {
			decoded = string(raw)
		}
		return typedMismatch{expected: decoded, actual: actualInterface(actual)}, true, nil
	}
	value := actual.String()
	if (!escaped && stringBytesEqual(value, raw)) || (escaped && value == decoded) {
		return typedMismatch{}, false, nil
	}
	if !escaped {
		decoded = string(raw)
	}
	return typedMismatch{expected: decoded, actual: value}, true, nil
}

func stringBytesEqual(value string, data []byte) bool {
	if len(value) != len(data) {
		return false
	}
	for index := range data {
		if value[index] != data[index] {
			return false
		}
	}
	return true
}

func (comparator *rawJSONReflectComparator) compareNumber(actual reflect.Value) (typedMismatch, bool, error) {
	start := comparator.position
	for comparator.position < len(comparator.data) {
		switch comparator.data[comparator.position] {
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '-', '+', '.', 'e', 'E':
			comparator.position++
		default:
			goto parsed
		}
	}
parsed:
	if start == comparator.position {
		return typedMismatch{}, false, fmt.Errorf("invalid JSON value at byte %d", start)
	}
	dynamic := actual.IsValid() && actual.Kind() == reflect.Interface
	actual = indirectValue(actual)
	raw := string(comparator.data[start:comparator.position])
	if !dynamic {
		switch actual.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			expected, err := strconv.ParseInt(raw, 10, actual.Type().Bits())
			if err != nil {
				return typedMismatch{}, false, err
			}
			if expected == actual.Int() {
				return typedMismatch{}, false, nil
			}
			return typedMismatch{expected: expected, actual: actual.Int()}, true, nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			expected, err := strconv.ParseUint(raw, 10, actual.Type().Bits())
			if err != nil {
				return typedMismatch{}, false, err
			}
			if expected == actual.Uint() {
				return typedMismatch{}, false, nil
			}
			return typedMismatch{expected: expected, actual: actual.Uint()}, true, nil
		}
	}
	expected, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return typedMismatch{}, false, err
	}
	value, ok := typedNumericValue(actual)
	if ok {
		if math.IsNaN(expected) || math.IsNaN(value) || math.Abs(expected-value) <= comparator.tolerance {
			return typedMismatch{}, false, nil
		}
	}
	return typedMismatch{expected: expected, actual: actualInterface(actual)}, true, nil
}

func (comparator *rawJSONReflectComparator) compareArray(actual reflect.Value) (typedMismatch, bool, error) {
	comparator.position++
	actual = indirectValue(actual)
	length, isArray := arrayLength(actual)
	if !isArray {
		return typedMismatch{expected: "array", actual: actualInterface(actual)}, true, nil
	}
	if actual.Kind() == reflect.Slice && actual.IsNil() {
		return typedMismatch{expected: []any{}, actual: nil}, true, nil
	}
	index := 0
	comparator.skipSpace()
	for comparator.position < len(comparator.data) && comparator.data[comparator.position] != ']' {
		if index >= length {
			return typedMismatch{path: ".length", expected: index + 1, actual: length}, true, nil
		}
		mismatch, ok, err := comparator.compareValue(actual.Index(index))
		if err != nil || ok {
			if ok {
				mismatch.path = fmt.Sprintf("[%d]", index) + mismatch.path
			}
			return mismatch, ok, err
		}
		index++
		comparator.skipSpace()
		if comparator.position < len(comparator.data) && comparator.data[comparator.position] == ',' {
			comparator.position++
			comparator.skipSpace()
			continue
		}
		break
	}
	if comparator.position >= len(comparator.data) || comparator.data[comparator.position] != ']' {
		return typedMismatch{}, false, fmt.Errorf("unterminated JSON array")
	}
	comparator.position++
	if index != length {
		return typedMismatch{path: ".length", expected: index, actual: length}, true, nil
	}
	return typedMismatch{}, false, nil
}

func (comparator *rawJSONReflectComparator) compareObject(actual reflect.Value) (typedMismatch, bool, error) {
	comparator.position++
	actual = indirectValue(actual)
	if !isObjectValue(actual) {
		return typedMismatch{expected: "object", actual: actualInterface(actual)}, true, nil
	}
	if actual.Kind() == reflect.Map && actual.IsNil() {
		return typedMismatch{expected: map[string]any{}, actual: nil}, true, nil
	}
	var metadata *objectMetadata
	var seenFields uint64
	var seenFieldsOverflow []bool
	seenKeys := map[string]bool(nil)
	if actual.Kind() == reflect.Struct {
		metadata = reflectObjectMetadata(actual.Type())
		if actual.NumField() > 64 {
			seenFieldsOverflow = make([]bool, actual.NumField())
		}
	} else {
		seenKeys = make(map[string]bool, actual.Len())
	}
	comparator.skipSpace()
	for comparator.position < len(comparator.data) && comparator.data[comparator.position] != '}' {
		rawKey, key, escapedKey, err := comparator.readString()
		if err != nil {
			return typedMismatch{}, false, err
		}
		if !escapedKey {
			key = string(rawKey)
		}
		comparator.skipSpace()
		if comparator.position >= len(comparator.data) || comparator.data[comparator.position] != ':' {
			return typedMismatch{}, false, fmt.Errorf("missing colon after JSON key %q", key)
		}
		comparator.position++
		var field reflect.Value
		if metadata != nil {
			indices := metadata.byName[key]
			if len(indices) == 0 {
				for _, candidate := range metadata.fields {
					if strings.EqualFold(candidate.name, key) {
						indices = []int{candidate.index}
						break
					}
				}
			}
			if len(indices) == 0 {
				return typedMismatch{}, false, fmt.Errorf("json: unknown field %q", key)
			}
			field = actual.Field(indices[0])
			if seenFieldsOverflow != nil {
				seenFieldsOverflow[indices[0]] = true
			} else {
				seenFields |= uint64(1) << indices[0]
			}
		} else {
			field = actual.MapIndex(reflect.ValueOf(key).Convert(actual.Type().Key()))
			if !field.IsValid() {
				return typedMismatch{path: "." + key, expected: "present", actual: nil}, true, nil
			}
			seenKeys[key] = true
		}
		mismatch, ok, err := comparator.compareValue(field)
		if err != nil || ok {
			if ok {
				mismatch.path = "." + key + mismatch.path
			}
			return mismatch, ok, err
		}
		comparator.skipSpace()
		if comparator.position < len(comparator.data) && comparator.data[comparator.position] == ',' {
			comparator.position++
			comparator.skipSpace()
			continue
		}
		break
	}
	if comparator.position >= len(comparator.data) || comparator.data[comparator.position] != '}' {
		return typedMismatch{}, false, fmt.Errorf("unterminated JSON object")
	}
	comparator.position++
	if metadata != nil {
		for _, field := range metadata.fields {
			seen := seenFieldsOverflow != nil && seenFieldsOverflow[field.index]
			if seenFieldsOverflow == nil {
				seen = seenFields&(uint64(1)<<field.index) != 0
			}
			if seen {
				continue
			}
			zero := reflect.Zero(actual.Field(field.index).Type())
			if mismatch, ok := typedJSONMismatch(zero, actual.Field(field.index), comparator.tolerance); ok {
				mismatch.path = "." + field.name + mismatch.path
				return mismatch, true, nil
			}
		}
	} else {
		keys := actual.MapKeys()
		sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })
		for _, key := range keys {
			if !seenKeys[key.String()] {
				return typedMismatch{path: "." + key.String(), expected: nil, actual: actualInterface(actual.MapIndex(key))}, true, nil
			}
		}
	}
	return typedMismatch{}, false, nil
}

func (comparator *rawJSONReflectComparator) readString() ([]byte, string, bool, error) {
	comparator.skipSpace()
	if comparator.position >= len(comparator.data) || comparator.data[comparator.position] != '"' {
		return nil, "", false, fmt.Errorf("expected JSON string at byte %d", comparator.position)
	}
	start := comparator.position
	comparator.position++
	contentStart := comparator.position
	escaped := false
	for comparator.position < len(comparator.data) {
		switch comparator.data[comparator.position] {
		case '\\':
			escaped = true
			comparator.position += 2
		case '"':
			raw := comparator.data[contentStart:comparator.position]
			comparator.position++
			if !escaped {
				return raw, "", false, nil
			}
			decoded, err := strconv.Unquote(string(comparator.data[start:comparator.position]))
			return raw, decoded, true, err
		default:
			comparator.position++
		}
	}
	return nil, "", false, io.ErrUnexpectedEOF
}

func (comparator *rawJSONReflectComparator) consumeLiteral(literal string) error {
	if len(comparator.data)-comparator.position < len(literal) || !bytes.Equal(comparator.data[comparator.position:comparator.position+len(literal)], []byte(literal)) {
		return fmt.Errorf("invalid JSON literal at byte %d", comparator.position)
	}
	comparator.position += len(literal)
	return nil
}

func (comparator *rawJSONReflectComparator) skipSpace() {
	for comparator.position < len(comparator.data) {
		switch comparator.data[comparator.position] {
		case ' ', '\t', '\r', '\n':
			comparator.position++
		default:
			return
		}
	}
}

type typedMismatch struct {
	path     string
	expected any
	actual   any
}

func typedJSONMismatch(expected, actual reflect.Value, tolerance float64) (typedMismatch, bool) {
	expected = indirectValue(expected)
	actual = indirectValue(actual)
	if !expected.IsValid() || !actual.IsValid() {
		if expected.IsValid() == actual.IsValid() {
			return typedMismatch{}, false
		}
		return typedMismatch{expected: actualInterface(expected), actual: actualInterface(actual)}, true
	}
	if left, leftOK := typedNumericValue(expected); leftOK {
		if right, rightOK := typedNumericValue(actual); rightOK {
			if !math.IsNaN(left) && !math.IsNaN(right) && math.Abs(left-right) > tolerance {
				return typedMismatch{expected: left, actual: right}, true
			}
			return typedMismatch{}, false
		}
	}
	if expected.Kind() == reflect.Map && expected.Type().Key().Kind() == reflect.String && isObjectValue(actual) && expected.Type() != actual.Type() {
		return typedJSONObjectMismatch(expected, actual, tolerance)
	}
	if expected.Type() != actual.Type() {
		return typedMismatch{expected: actualInterface(expected), actual: actualInterface(actual)}, true
	}
	switch expected.Kind() {
	case reflect.Struct:
		for _, field := range reflectObjectMetadata(expected.Type()).fields {
			if mismatch, ok := typedJSONMismatch(expected.Field(field.index), actual.Field(field.index), tolerance); ok {
				mismatch.path = "." + field.name + mismatch.path
				return mismatch, true
			}
		}
	case reflect.Array, reflect.Slice:
		if expected.Kind() == reflect.Slice && expected.IsNil() != actual.IsNil() {
			return typedMismatch{expected: actualInterface(expected), actual: actualInterface(actual)}, true
		}
		if expected.Len() != actual.Len() {
			return typedMismatch{path: ".length", expected: expected.Len(), actual: actual.Len()}, true
		}
		for index := 0; index < expected.Len(); index++ {
			if mismatch, ok := typedJSONMismatch(expected.Index(index), actual.Index(index), tolerance); ok {
				mismatch.path = fmt.Sprintf("[%d]", index) + mismatch.path
				return mismatch, true
			}
		}
	case reflect.Map:
		if expected.IsNil() != actual.IsNil() {
			return typedMismatch{expected: actualInterface(expected), actual: actualInterface(actual)}, true
		}
		if expected.Len() != actual.Len() {
			return typedMismatch{path: ".length", expected: expected.Len(), actual: actual.Len()}, true
		}
		keys := expected.MapKeys()
		sort.Slice(keys, func(i, j int) bool { return fmt.Sprint(keys[i].Interface()) < fmt.Sprint(keys[j].Interface()) })
		for _, key := range keys {
			actualValue := actual.MapIndex(key)
			if !actualValue.IsValid() {
				return typedMismatch{path: fmt.Sprintf("[%v]", key.Interface()), expected: actualInterface(expected.MapIndex(key)), actual: nil}, true
			}
			if mismatch, ok := typedJSONMismatch(expected.MapIndex(key), actualValue, tolerance); ok {
				mismatch.path = fmt.Sprintf("[%v]", key.Interface()) + mismatch.path
				return mismatch, true
			}
		}
	case reflect.Float32, reflect.Float64:
		left, right := expected.Float(), actual.Float()
		if !math.IsNaN(left) && !math.IsNaN(right) && math.Abs(left-right) > tolerance {
			return typedMismatch{expected: left, actual: right}, true
		}
	default:
		if !reflect.DeepEqual(actualInterface(expected), actualInterface(actual)) {
			return typedMismatch{expected: actualInterface(expected), actual: actualInterface(actual)}, true
		}
	}
	return typedMismatch{}, false
}

func typedJSONObjectMismatch(expected, actual reflect.Value, tolerance float64) (typedMismatch, bool) {
	if expected.IsNil() {
		return typedMismatch{expected: actualInterface(expected), actual: actualInterface(actual)}, true
	}
	seen := make(map[string]bool, expected.Len())
	keys := expected.MapKeys()
	sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })
	for _, key := range keys {
		name := key.String()
		seen[name] = true
		actualValue, ok := objectField(actual, name)
		if !ok {
			return typedMismatch{path: "." + name, expected: actualInterface(expected.MapIndex(key)), actual: nil}, true
		}
		if mismatch, ok := typedJSONMismatch(expected.MapIndex(key), actualValue, tolerance); ok {
			mismatch.path = "." + name + mismatch.path
			return mismatch, true
		}
	}
	for _, name := range objectKeys(actual) {
		if !seen[name] {
			actualValue, _ := objectField(actual, name)
			return typedMismatch{path: "." + name, expected: nil, actual: actualInterface(actualValue)}, true
		}
	}
	return typedMismatch{}, false
}

func typedNumericValue(value reflect.Value) (float64, bool) {
	switch value.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(value.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(value.Uint()), true
	case reflect.Float32, reflect.Float64:
		return value.Float(), true
	default:
		if value.IsValid() && value.CanInterface() {
			if number, ok := value.Interface().(json.Number); ok {
				parsed, err := strconv.ParseFloat(number.String(), 64)
				return parsed, err == nil
			}
		}
		return 0, false
	}
}

func compareUnorderedPropertyReflect(expected jsonTokenDecoder, actual reflect.Value, path string, tolerance float64, differences *[]string) error {
	seen := map[string]bool{}
	for expected.More() {
		keyToken, err := expected.Token()
		if err != nil {
			return err
		}
		key, ok := keyToken.(string)
		if !ok {
			return fmt.Errorf("%s has a non-string object key", path)
		}
		seen[key] = true
		field, found := objectField(actual, key)
		if !found {
			addDifference(differences, "%s: missing Go value", joinJSONPath(path, key))
			if err := skipJSONValue(expected); err != nil {
				return err
			}
			continue
		}
		if err := compareJSONStreamReflect(expected, field, joinJSONPath(path, key), tolerance, differences); err != nil {
			return err
		}
	}
	if err := expectJSONDelimiter(expected, '}', "expected "+path); err != nil {
		return err
	}
	for _, key := range objectKeys(actual) {
		if !seen[key] {
			addDifference(differences, "%s: unexpected Go value", joinJSONPath(path, key))
		}
	}
	return nil
}

func joinJSONPath(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

func skipJSONContainer(decoder jsonTokenDecoder, closing json.Delim) error {
	depth := 1
	for depth > 0 {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		if delimiter, ok := token.(json.Delim); ok {
			switch delimiter {
			case '{', '[':
				depth++
			case '}', ']':
				depth--
			}
		}
	}
	return nil
}

func skipJSONValue(decoder jsonTokenDecoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if delimiter, ok := token.(json.Delim); ok && (delimiter == '{' || delimiter == '[') {
		return skipJSONContainer(decoder, delimiter)
	}
	return nil
}

func pageSectionForField(field string) string {
	switch field {
	case "label", "width", "height", "rotation", "parent_key":
		return "pages"
	case "text":
		return "content.text"
	case "extract_text":
		return "content.extract_text"
	case "extract_text_tagged":
		return "content.extract_text.tagged"
	case "extract_text_untagged":
		return "content.extract_text.untagged"
	case "glyphs":
		return "content.glyphs"
	case "flatten":
		return "content.flatten"
	case "interp":
		return "content.interp"
	case "streams":
		return "content.streams"
	case "tokens":
		return "content.tokens"
	case "xobjects":
		return "content.xobjects"
	case "contents":
		return "content.contents"
	case "layout":
		return "layout"
	case "paths":
		return "content.paths"
	case "images":
		return "content.images"
	case "fonts":
		return "fonts"
	case "tags":
		return "content.tags"
	case "structure":
		return "content.structure"
	case "marked":
		return "content.marked"
	case "annotations":
		return "annotations"
	default:
		return "pages"
	}
}

func pageFieldSelected(field string, sections map[string]bool) bool {
	if field == "index" {
		return true
	}
	if field == "label" || field == "width" || field == "height" || field == "rotation" || field == "parent_key" {
		return sections["pages"]
	}
	for section, name := range map[string]string{
		"content.text": "text", "content.paths": "paths", "content.images": "images",
		"fonts": "fonts", "content.tags": "tags", "content.structure": "structure", "content.marked": "marked",
		"content.extract_text": "extract_text", "content.extract_text.tagged": "extract_text_tagged", "content.extract_text.untagged": "extract_text_untagged",
		"content.glyphs": "glyphs", "content.flatten": "flatten", "content.interp": "interp",
		"content.streams": "streams", "content.tokens": "tokens", "content.xobjects": "xobjects",
		"content.contents": "contents", "layout": "layout", "annotations": "annotations",
	} {
		if field == name {
			return sections[section]
		}
	}
	return false
}

func pageFieldValue(page testcompat.Page, field string) any {
	switch field {
	case "index":
		return page.Index
	case "label":
		return page.Label
	case "width":
		return page.Width
	case "height":
		return page.Height
	case "rotation":
		return page.Rotation
	case "parent_key":
		return page.ParentKey
	case "text":
		return page.Text
	case "extract_text":
		return page.ExtractText
	case "extract_text_tagged":
		return page.ExtractTextTagged
	case "extract_text_untagged":
		return page.ExtractTextUntagged
	case "glyphs":
		return page.Glyphs
	case "flatten":
		return page.Flatten
	case "interp":
		return page.Interp
	case "streams":
		return page.Streams
	case "tokens":
		return page.Tokens
	case "xobjects":
		return page.XObjects
	case "contents":
		return page.Contents
	case "layout":
		return page.Layout
	case "paths":
		return page.Paths
	case "images":
		return page.Images
	case "fonts":
		return page.Fonts
	case "tags":
		return page.Tags
	case "structure":
		return page.Structure
	case "marked":
		return page.Marked
	case "annotations":
		return page.Annotations
	default:
		return nil
	}
}

func isPageSection(section string) bool {
	switch section {
	case "pages", "content.text", "content.extract_text", "content.extract_text.tagged", "content.extract_text.untagged", "content.glyphs", "content.flatten", "content.interp", "content.streams", "content.tokens", "content.xobjects", "content.contents", "layout", "content.paths", "content.images", "fonts", "content.tags", "content.structure", "content.marked", "annotations":
		return true
	default:
		return false
	}
}
