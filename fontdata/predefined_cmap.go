package fontdata

//go:generate python3 ../scripts/sync_cmap_resources.py --output-dir ../fontdata/cmapdata

import (
	"fmt"
	"io/fs"
	"strings"
	"sync"
)

var (
	predefinedCMapMu    sync.RWMutex
	predefinedCMapCache = map[string]*CMap{}
	predefinedCMapLoads = map[string]*predefinedCMapLoad{}
)

type predefinedCMapLoad struct {
	done chan struct{}
	cmap *CMap
	err  error
}

var predefinedCMapNames = embeddedPredefinedCMapNames()

func embeddedPredefinedCMapNames() map[string]string {
	entries, err := fs.ReadDir(CMapFiles, "cmapdata")
	if err != nil {
		panic(fmt.Sprintf("playa: enumerate embedded CMaps: %v", err))
	}
	names := make(map[string]string, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".cmap") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".cmap")
		names[name] = "cmapdata/" + entry.Name()
	}
	return names
}

func identityPredefinedCMap(name string) (*CMap, bool) {
	switch name {
	case "DLIdent-H":
		return NewIdentityCMap(false, 2), true
	case "DLIdent-V":
		return NewIdentityCMap(true, 2), true
	case "OneByteIdentityH":
		return NewIdentityCMap(false, 1), true
	case "OneByteIdentityV":
		return NewIdentityCMap(true, 1), true
	default:
		return nil, false
	}
}

// LoadPredefinedCMap returns the cached immutable Adobe public CMap.
func LoadPredefinedCMap(name string) (*CMap, error) {
	return loadPredefinedCMap(name)
}

func loadPredefinedCMap(name string) (*CMap, error) {
	predefinedCMapMu.RLock()
	if cached, ok := predefinedCMapCache[name]; ok {
		predefinedCMapMu.RUnlock()
		return cached, nil
	}
	predefinedCMapMu.RUnlock()

	predefinedCMapMu.Lock()
	if cached, ok := predefinedCMapCache[name]; ok {
		predefinedCMapMu.Unlock()
		return cached, nil
	}
	if loading, ok := predefinedCMapLoads[name]; ok {
		predefinedCMapMu.Unlock()
		<-loading.done
		return loading.cmap, loading.err
	}
	loading := &predefinedCMapLoad{done: make(chan struct{})}
	predefinedCMapLoads[name] = loading
	predefinedCMapMu.Unlock()

	// Parsing can recurse through a CMap's usecmap chain and is independent
	// for each name. Keep it outside the cache lock so concurrent page loads
	// do not serialize on the slow path.
	var cmap *CMap
	var err error
	if identity, ok := identityPredefinedCMap(name); ok {
		cmap = identity
		// identityPredefinedCMap is cheap, but use the same publish path as
		// parsed resources to preserve one canonical pointer per name.
	} else {
		cmap, err = parsePredefinedCMap(name, map[string]bool{})
	}

	predefinedCMapMu.Lock()
	if err == nil {
		if cached, ok := predefinedCMapCache[name]; ok {
			cmap = cached
		} else {
			predefinedCMapCache[name] = cmap
		}
	}
	loading.cmap = cmap
	loading.err = err
	delete(predefinedCMapLoads, name)
	close(loading.done)
	predefinedCMapMu.Unlock()
	return cmap, err
}

func parsePredefinedCMap(name string, stack map[string]bool) (*CMap, error) {
	if stack[name] {
		return nil, fmt.Errorf("playa: circular predefined CMap inheritance at %q", name)
	}
	path, ok := predefinedCMapNames[name]
	if !ok {
		// Keep the explicit registry for the historically curated set, while
		// allowing newly vendored Adobe resources to be discovered by name.
		// This prevents the source repository's resource set from drifting away
		// from a second hand-maintained list.
		path = "cmapdata/" + name + ".cmap"
		if _, err := CMapFiles.ReadFile(path); err != nil {
			return nil, fmt.Errorf("playa: unsupported predefined CMap %q", name)
		}
	}
	raw, err := CMapFiles.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("playa: read predefined CMap %q: %w", name, err)
	}
	cmap, err := ParseCMap(raw)
	if err != nil {
		return nil, fmt.Errorf("playa: parse predefined CMap %q: %w", name, err)
	}
	if cmap.UseCMap() != "" {
		stack[name] = true
		base, baseErr := parsePredefinedCMap(cmap.UseCMap(), stack)
		delete(stack, name)
		if baseErr != nil {
			return nil, fmt.Errorf("playa: resolve predefined CMap %q base %q: %w", name, cmap.UseCMap(), baseErr)
		}
		MergeUseCMap(cmap, base)
	}
	return cmap, nil
}
