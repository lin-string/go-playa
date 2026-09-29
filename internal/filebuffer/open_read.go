//go:build !darwin && !linux

// Package filebuffer opens immutable files with a lifecycle-bound byte view.
package filebuffer

import "os"

// Open returns an owned file buffer on platforms without the mmap backend.
func Open(path string) ([]byte, func() error, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	return data, func() error { return nil }, nil
}
