//go:build linux

package main

import (
	"os"
	"strconv"
	"strings"
)

func physicalMemoryBytes() int64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	return linuxMemoryBytes(data, "MemTotal:")
}

func availableMemoryBytes() int64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	return linuxMemoryBytes(data, "MemAvailable:")
}

func linuxMemoryBytes(data []byte, name string) int64 {
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != name {
			continue
		}
		kilobytes, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil || kilobytes <= 0 || kilobytes > int64(^uint64(0)>>1)/1024 {
			return 0
		}
		return kilobytes * 1024
	}
	return 0
}
