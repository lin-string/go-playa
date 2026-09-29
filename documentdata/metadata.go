// Package documentdata owns dependency-free document-level value models.
package documentdata

import "strconv"

// Metadata contains normalized PDF Info dictionary values.
type Metadata map[string]string

// CloneMetadata returns an independent metadata snapshot.
func CloneMetadata(source Metadata) Metadata {
	if source == nil {
		return nil
	}
	out := make(Metadata, len(source))
	for key, value := range source {
		out[key] = value
	}
	return out
}

// FormatNumber formats a finite PDF number using Playa's compact projection.
func FormatNumber(value float64) string {
	if value == float64(int(value)) {
		return strconv.Itoa(int(value))
	}
	return strconv.FormatFloat(value, 'g', -1, 64)
}
