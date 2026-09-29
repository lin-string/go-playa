//go:build darwin || linux

// Package filebuffer opens immutable files with a lifecycle-bound byte view.
package filebuffer

import (
	"fmt"
	"os"
	"sync"
	"syscall"
)

// Open returns a read-only file view and an idempotent release function.
func Open(path string) ([]byte, func() error, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, nil, err
	}
	size := info.Size()
	if size < 0 || int64(int(size)) != size {
		_ = file.Close()
		return nil, nil, fmt.Errorf("playa: PDF file size %d is not addressable", size)
	}
	if size == 0 {
		if err := file.Close(); err != nil {
			return nil, nil, err
		}
		return []byte{}, func() error { return nil }, nil
	}
	data, err := syscall.Mmap(int(file.Fd()), 0, int(size), syscall.PROT_READ, syscall.MAP_PRIVATE)
	closeErr := file.Close()
	if err != nil {
		return nil, nil, err
	}
	if closeErr != nil {
		_ = syscall.Munmap(data)
		return nil, nil, closeErr
	}
	var once sync.Once
	var releaseErr error
	release := func() error {
		once.Do(func() { releaseErr = syscall.Munmap(data) })
		return releaseErr
	}
	return data, release, nil
}
