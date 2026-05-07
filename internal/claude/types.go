package claude

// CommentRequest is the input to Client.Comment.
//
// Sport is the Strava sport_type label (e.g. "Ride", "Run", "Swim"); the
// model uses it to color the tone of the response. Summary is a
// pre-formatted, plain-text digest of the activity's key metrics — one fact
// per line is enough. Building Summary from a strava.ActivityDetail lives in
// the caller (typically internal/analyzer) so this package stays free of
// Strava types and can be tested in isolation.
type CommentRequest struct {
	Sport   string
	Summary string
}
