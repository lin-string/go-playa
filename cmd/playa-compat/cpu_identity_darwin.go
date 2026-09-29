//go:build darwin

package main

import (
	"os/exec"
	"runtime"
	"strings"
)

func currentCPUIdentity() string {
	output, err := exec.Command("/usr/sbin/sysctl", "-n", "machdep.cpu.brand_string").Output()
	if err != nil || strings.TrimSpace(string(output)) == "" {
		return runtime.GOARCH
	}
	return strings.TrimSpace(string(output))
}
