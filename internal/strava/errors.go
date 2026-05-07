package strava

import "errors"

// Sentinel errors returned by the client. Use errors.Is to test, never compare
// directly — wrapped errors are routine (we attach request URL/status).
var (
	// ErrUnauthorized is returned for HTTP 401 responses. Usually means the
	// access token expired and a refresh did not complete in time.
	ErrUnauthorized = errors.New("strava: unauthorized")

	// ErrForbidden is returned for HTTP 403. The token's OAuth scope is
	// insufficient for this endpoint (most often missing `activity:read_all`).
	ErrForbidden = errors.New("strava: forbidden")

	// ErrNotFound is returned for HTTP 404. The activity may have been deleted
	// or the athlete revoked access.
	ErrNotFound = errors.New("strava: not found")

	// ErrRateLimited is returned for HTTP 429. Strava enforces 200/15min and
	// 2000/day caps; back off and retry later.
	ErrRateLimited = errors.New("strava: rate limited")

	// ErrUnexpectedStatus wraps any other non-2xx response so callers can
	// surface the raw status without losing the sentinel pivot above.
	ErrUnexpectedStatus = errors.New("strava: unexpected status")
)
