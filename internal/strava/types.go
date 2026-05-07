package strava

import "time"

// SportType mirrors Strava's `sport_type` taxonomy. Only the subset Databara
// understands today is enumerated; unknown values pass through as-is so a new
// sport from Strava doesn't break ingestion.
type SportType string

const (
	SportRide         SportType = "Ride"
	SportVirtualRide  SportType = "VirtualRide"
	SportRun          SportType = "Run"
	SportTrailRun     SportType = "TrailRun"
	SportSwim         SportType = "Swim"
	SportWorkout      SportType = "Workout"
)

// TokenSet is the OAuth2 credential bundle Strava returns. It is rotated on
// every refresh; persist the latest value before issuing the next API call.
type TokenSet struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// Expired reports whether the access token is past its expiry, with a small
// safety margin to avoid racing the clock against a request in flight.
func (t TokenSet) Expired() bool {
	return time.Now().Add(30 * time.Second).After(t.ExpiresAt)
}

// Activity is the trimmed-down summary returned by the activity list and
// embedded inside webhook follow-up fetches.
type Activity struct {
	ID            int64     `json:"id"`
	AthleteID     int64     `json:"-"`
	Name          string    `json:"name"`
	SportType     SportType `json:"sport_type"`
	StartDate     time.Time `json:"start_date"`
	ElapsedTime   int       `json:"elapsed_time"`
	MovingTime    int       `json:"moving_time"`
	Distance      float64   `json:"distance"`
	TotalElevGain float64   `json:"total_elevation_gain"`
}

// ActivityDetail extends Activity with metrics that are only populated on the
// dedicated detail endpoint (HR, power, calories, etc.). Fields stay pointers
// so missing telemetry survives a JSON round-trip without being confused with
// a legitimate zero value.
type ActivityDetail struct {
	Activity

	AverageSpeed      float64  `json:"average_speed"`
	MaxSpeed          float64  `json:"max_speed"`
	AverageHeartrate  *float64 `json:"average_heartrate,omitempty"`
	MaxHeartrate      *float64 `json:"max_heartrate,omitempty"`
	AverageWatts      *float64 `json:"average_watts,omitempty"`
	WeightedAvgWatts  *int     `json:"weighted_average_watts,omitempty"`
	Kilojoules        *float64 `json:"kilojoules,omitempty"`
	HasHeartrate      bool     `json:"has_heartrate"`
	DeviceWatts       bool     `json:"device_watts"`
	Calories          *float64 `json:"calories,omitempty"`
	AvgCadence        *float64 `json:"average_cadence,omitempty"`
	SufferScore       *float64 `json:"suffer_score,omitempty"`
}

// Stream is a single time-aligned channel from the activity streams endpoint
// (e.g. heartrate, watts, distance). Series can be float64 or int but Strava
// always sends them as numbers, so float64 is the safe choice.
type Stream struct {
	Type         string    `json:"type"`
	Data         []float64 `json:"data"`
	SeriesType   string    `json:"series_type"`
	OriginalSize int       `json:"original_size"`
	Resolution   string    `json:"resolution"`
}
