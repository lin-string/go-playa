# Core 14 AFM Files

This directory contains a mirror of the Adobe PostScript(R) Core 14 AFM
files, originally published at:
https://www.adobe.com/devnet/font/pdfs/Core14_AFMs.zip

The files are used as input to the generated standard font metrics in
`fontdata/standard_metrics_generated.go`. The synchronization script
can refresh them from the public mirror when the original Adobe endpoint is
unavailable.

The synchronizer normalizes line endings and trailing whitespace. Each AFM
records this modification in a `Comment Modified by go-playa` line; the
metrics and original notices are preserved. The accompanying `LICENSE`
must remain with the AFM files.
