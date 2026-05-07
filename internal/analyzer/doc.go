// Package analyzer turns a Strava activity into a coaching note.
//
// Phase 1 keeps it sport-agnostic: format the headline metrics into a
// plain-text summary and hand that to the Claude wrapper. Phase 2 will add
// per-sport branches (cycling power analysis, running pace zones, swim
// stroke metrics).
package analyzer
