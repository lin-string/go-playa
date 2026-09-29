//go:build linux

package main

import (
	"os"
	"runtime"
	"strings"
)

func currentCPUIdentity() string {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return runtime.GOARCH
	}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(line, ":")
		if ok && (strings.TrimSpace(key) == "model name" || strings.TrimSpace(key) == "Hardware") {
			if identity := strings.TrimSpace(value); identity != "" {
				return identity
			}
		}
	}
	return runtime.GOARCH
}
