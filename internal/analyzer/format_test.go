package analyzer

import (
	"strings"
	"testing"
	"time"

	"github.com/Amdhj22/databara/internal/strava"
)

func ptr[T any](v T) *T { return &v }

func TestFormatActivity_FullCyclingRide(t *testing.T) {
	a := strava.ActivityDetail{
		Activity: strava.Activity{
			ID:            42,
			Name:          "Morning Ride",
			SportType:     strava.SportRide,
			StartDate:     time.Date(2026, 5, 7, 7, 0, 0, 0, time.UTC),
			ElapsedTime:   3700,
			MovingTime:    3600,
			Distance:      30000,
			TotalElevGain: 250,
		},
		AverageSpeed:     8.33,
		MaxSpeed:         13.0,
		AverageHeartrate: ptr(135.0),
		MaxHeartrate:     ptr(168.0),
		HasHeartrate:     true,
		AverageWatts:     ptr(180.0),
		WeightedAvgWatts: ptr(195),
		Kilojoules:       ptr(648.0),
		DeviceWatts:      true,
		Calories:         ptr(720.0),
		AvgCadence:       ptr(82.0),
	}

	got := formatActivity(a)

	must := []string{
		"Name: Morning Ride",
		"Started: 2026-05-07 07:00 UTC",
		"Distance: 30.00 km",
		"Moving time: 1:00:00",
		"Elapsed time: 1:01:40",
		"Avg speed: 29.99 km/h",
		"Max speed: 46.80 km/h",
		"Elevation gain: 250 m",
		"Avg HR: 135 bpm",
		"Max HR: 168 bpm",
		"Avg power: 180 W",
		"Normalized power: 195 W",
		"Work: 648 kJ",
		"Avg cadence: 82",
		"Calories: 720 kcal",
	}
	for _, want := range must {
		if !strings.Contains(got, want) {
			t.Errorf("missing line %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "estimated") {
		t.Errorf("did not expect 'estimated' label when DeviceWatts=true; got:\n%s", got)
	}
}

func TestFormatActivity_OmitsMissingFields(t *testing.T) {
	a := strava.ActivityDetail{
		Activity: strava.Activity{
			ID:         1,
			Name:       "Easy Run",
			SportType:  strava.SportRun,
			Distance:   5000,
			MovingTime: 1500,
		},
	}
	got := formatActivity(a)

	for _, banned := range []string{"HR", "power", "Calories", "Suffer", "Kilojoules", "cadence", "Elevation"} {
		if strings.Contains(got, banned) {
			t.Errorf("missing-metric line should be omitted, found %q in:\n%s", banned, got)
		}
	}
	if !strings.Contains(got, "Distance: 5.00 km") {
		t.Errorf("expected Distance line; got:\n%s", got)
	}
	if !strings.Contains(got, "Moving time: 25:00") {
		t.Errorf("expected Moving time 25:00; got:\n%s", got)
	}
}

func TestFormatActivity_EstimatedPowerLabel(t *testing.T) {
	a := strava.ActivityDetail{
		Activity:     strava.Activity{Name: "x"},
		AverageWatts: ptr(150.0),
		DeviceWatts:  false,
	}
	if got := formatActivity(a); !strings.Contains(got, "Avg power (estimated): 150 W") {
		t.Errorf("expected estimated label; got:\n%s", got)
	}
}

func TestFormatActivity_SkipsElapsedWhenEqualToMoving(t *testing.T) {
	a := strava.ActivityDetail{
		Activity: strava.Activity{
			Name:        "x",
			ElapsedTime: 1800,
			MovingTime:  1800,
		},
	}
	got := formatActivity(a)
	if strings.Contains(got, "Elapsed time") {
		t.Errorf("Elapsed time should be skipped when equal to MovingTime; got:\n%s", got)
	}
	if !strings.Contains(got, "Moving time: 30:00") {
		t.Errorf("expected Moving time 30:00; got:\n%s", got)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		in   time.Duration
		want string
	}{
		{30 * time.Second, "0:30"},
		{90 * time.Second, "1:30"},
		{1500 * time.Second, "25:00"},
		{3600 * time.Second, "1:00:00"},
		{3661 * time.Second, "1:01:01"},
		{7322 * time.Second, "2:02:02"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := formatDuration(tt.in); got != tt.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
