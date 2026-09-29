//go:build !darwin && !linux

package main

func physicalMemoryBytes() int64 { return 0 }

func availableMemoryBytes() int64 { return 0 }
