//go:build darwin || linux

package main

import (
	"runtime"
	"syscall"
)

func peakRSSBytes() uint64 {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil || usage.Maxrss <= 0 {
		return 0
	}
	value := uint64(usage.Maxrss)
	if runtime.GOOS == "linux" {
		return value * 1024
	}
	return value
}
