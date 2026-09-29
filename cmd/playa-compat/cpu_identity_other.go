//go:build !darwin && !linux

package main

import "runtime"

func currentCPUIdentity() string { return runtime.GOARCH }
