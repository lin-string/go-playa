package main

import "testing"

func TestShouldCollectCompatGroupMemoryUsesProcessThreshold(t *testing.T) {
	if shouldCollectCompatGroupMemory(compatibilityGroupHeapCollectionThreshold - 1) {
		t.Fatal("group-boundary collection ran below the process threshold")
	}
	if !shouldCollectCompatGroupMemory(compatibilityGroupHeapCollectionThreshold) {
		t.Fatal("group-boundary collection skipped the process threshold")
	}
}
