//go:build !darwin && !linux

package main

func peakRSSBytes() uint64 { return 0 }
