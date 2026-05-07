package analyzer

import (
	"fmt"
	"strings"
	"time"

	"github.com/Amdhj22/databara/internal/strava"
)

// formatActivity turns a Strava ActivityDetail into the multi-line Summary
// string the Claude prompt expects. Missing metrics are skipped entirely —
// the system prompt is told not to invent numbers, so emitting nothing is
// safer than an "N/A" line that the model might quote back.
//
// All values are emitted in metric units (km, km/h, bpm, W, kJ, kcal). The
// timestamp is formatted in UTC so log lines and tests stay deterministic
// across hosts; downstream consumers can localize for display.
//
// Phase 2 will branch on activity.SportType for sport-specific metrics
// (running pace, swim 100m pace + SWOLF, cycling NP/IF/TSS).
func formatActivity(a strava.ActivityDetail) string {
	var lines []string

	if a.Name != "" {
		lines = append(lines, "Name: "+a.Name)
	}
	if !a.StartDate.IsZero() {
		lines = append(lines, "Started: "+a.StartDate.UTC().Format("2006-01-02 15:04 UTC"))
	}

	if a.Distance > 0 {
		lines = append(lines, fmt.Sprintf("Distance: %.2f km", a.Distance/1000))
	}
	if a.MovingTime > 0 {
		lines = append(lines, "Moving time: "+formatDuration(time.Duration(a.MovingTime)*time.Second))
	}
	if a.ElapsedTime > 0 && a.ElapsedTime != a.MovingTime {
		lines = append(lines, "Elapsed time: "+formatDuration(time.Duration(a.ElapsedTime)*time.Second))
	}
	if a.AverageSpeed > 0 {
		lines = append(lines, fmt.Sprintf("Avg speed: %.2f km/h", a.AverageSpeed*3.6))
	}
	if a.MaxSpeed > 0 {
		lines = append(lines, fmt.Sprintf("Max speed: %.2f km/h", a.MaxSpeed*3.6))
	}
	if a.TotalElevGain > 0 {
		lines = append(lines, fmt.Sprintf("Elevation gain: %.0f m", a.TotalElevGain))
	}

	if a.HasHeartrate && a.AverageHeartrate != nil {
		lines = append(lines, fmt.Sprintf("Avg HR: %.0f bpm", *a.AverageHeartrate))
	}
	if a.MaxHeartrate != nil {
		lines = append(lines, fmt.Sprintf("Max HR: %.0f bpm", *a.MaxHeartrate))
	}

	if a.AverageWatts != nil {
		label := "Avg power"
		if !a.DeviceWatts {
			label += " (estimated)"
		}
		lines = append(lines, fmt.Sprintf("%s: %.0f W", label, *a.AverageWatts))
	}
	if a.WeightedAvgWatts != nil {
		lines = append(lines, fmt.Sprintf("Normalized power: %d W", *a.WeightedAvgWatts))
	}
	if a.Kilojoules != nil {
		lines = append(lines, fmt.Sprintf("Work: %.0f kJ", *a.Kilojoules))
	}

	if a.AvgCadence != nil {
		lines = append(lines, fmt.Sprintf("Avg cadence: %.0f", *a.AvgCadence))
	}
	if a.Calories != nil {
		lines = append(lines, fmt.Sprintf("Calories: %.0f kcal", *a.Calories))
	}
	if a.SufferScore != nil {
		lines = append(lines, fmt.Sprintf("Suffer score: %.0f", *a.SufferScore))
	}

	return strings.Join(lines, "\n")
}

// formatDuration prints d as h:mm:ss for durations of an hour or more, and
// m:ss otherwise. Strava's elapsed/moving times come in whole seconds, so
// sub-second precision is intentionally dropped.
func formatDuration(d time.Duration) string {
	total := int(d / time.Second)
	h, rem := total/3600, total%3600
	m, s := rem/60, rem%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}
