package strava

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// GetActivity fetches the full ActivityDetail for the given activity ID.
// Strava only populates power/HR/calories on this endpoint, not on the list
// or webhook payload.
func (c *Client) GetActivity(ctx context.Context, id int64) (ActivityDetail, error) {
	var detail ActivityDetail
	path := fmt.Sprintf("/activities/%d", id)
	if err := c.do(ctx, http.MethodGet, path, nil, &detail); err != nil {
		return ActivityDetail{}, err
	}
	return detail, nil
}

// StreamKey is a typed enum of the channels Strava can return on
// /activities/{id}/streams. Not exhaustive — add as Databara needs them.
type StreamKey string

const (
	StreamHeartrate StreamKey = "heartrate"
	StreamWatts     StreamKey = "watts"
	StreamCadence   StreamKey = "cadence"
	StreamVelocity  StreamKey = "velocity_smooth"
	StreamAltitude  StreamKey = "altitude"
	StreamDistance  StreamKey = "distance"
	StreamTime      StreamKey = "time"
)

// GetStreams pulls the requested channels for an activity. It returns a map
// keyed by Stream.Type so callers can pick channels without scanning a slice.
//
// Strava returns a JSON array; we decode into the map by walking entries.
// Resolution is left at default ("high") which yields one sample per second
// for cycling/running and a coarser cadence for swimming.
func (c *Client) GetStreams(ctx context.Context, id int64, keys []StreamKey) (map[string]Stream, error) {
	if len(keys) == 0 {
		return nil, errors.New("strava.GetStreams: keys must be non-empty")
	}
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = string(k)
	}
	q := url.Values{
		"keys":        {strings.Join(parts, ",")},
		"key_by_type": {"true"},
	}
	path := fmt.Sprintf("/activities/%d/streams?%s", id, q.Encode())

	out := make(map[string]Stream)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}
