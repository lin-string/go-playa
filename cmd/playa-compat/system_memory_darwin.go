//go:build darwin

package main

import (
	"os/exec"
	"strconv"
	"strings"
)

func physicalMemoryBytes() int64 {
	output, err := exec.Command("/usr/sbin/sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return 0
	}
	bytes, err := strconv.ParseInt(strings.TrimSpace(string(output)), 10, 64)
	if err != nil || bytes <= 0 {
		return 0
	}
	return bytes
}

func availableMemoryBytes() int64 {
	total := physicalMemoryBytes()
	if total <= 0 {
		return 0
	}
	output, err := exec.Command("/usr/bin/memory_pressure", "-Q").Output()
	if err != nil {
		return 0
	}
	for _, field := range strings.Fields(string(output)) {
		if !strings.HasSuffix(field, "%") {
			continue
		}
		percent, err := strconv.ParseInt(strings.TrimSuffix(field, "%"), 10, 64)
		if err != nil || percent < 0 || percent > 100 {
			continue
		}
		return (total/100)*percent + (total%100)*percent/100
	}
	return 0
}
