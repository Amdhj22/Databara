// Package server is Databara's HTTP front door and activity-processing
// pipeline.
//
// The package owns three responsibilities, one file each:
//   - server.go   — HTTP routing (Strava webhook GET/POST, healthz)
//   - worker.go   — buffered queue + single-consumer goroutine
//   - process.go  — the fetch → review → send pipeline driven by the worker
//
// All cross-package collaborators are accessed through small local
// interfaces (Fetcher, Reviewer, Sender) so this package depends only on
// the strava package for wire-format types. main.go wires the concrete
// strava.Client / analyzer.Analyzer / telegram.Client into those slots.
package server
