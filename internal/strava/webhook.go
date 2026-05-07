package strava

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Webhook event semantics from Strava's Push Subscriptions guide.
const (
	ObjectTypeActivity = "activity"
	ObjectTypeAthlete  = "athlete"

	AspectCreate = "create"
	AspectUpdate = "update"
	AspectDelete = "delete"
)

// WebhookEvent is the payload Strava POSTs to our callback URL whenever a
// subscribed athlete uploads, edits, or deletes an activity.
//
// Updates is intentionally a free-form map: Strava only sends the changed
// fields and the schema differs by ObjectType.
type WebhookEvent struct {
	ObjectType     string         `json:"object_type"`
	ObjectID       int64          `json:"object_id"`
	AspectType     string         `json:"aspect_type"`
	OwnerID        int64          `json:"owner_id"`
	SubscriptionID int64          `json:"subscription_id"`
	EventTime      int64          `json:"event_time"`
	Updates        map[string]any `json:"updates,omitempty"`
}

// IsActivityCreate reports whether this event represents a new upload — the
// only case Phase 1 of Databara reacts to.
func (e WebhookEvent) IsActivityCreate() bool {
	return e.ObjectType == ObjectTypeActivity && e.AspectType == AspectCreate
}

// Subscription is the response Strava returns when listing or registering
// push subscriptions.
type Subscription struct {
	ID            int64  `json:"id"`
	ApplicationID int64  `json:"application_id"`
	CallbackURL   string `json:"callback_url"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// Webhook subscription endpoints live outside /api/v3, so they bypass
// Client.do() and the bearer-token auth path.
const subscriptionsURL = "https://www.strava.com/api/v3/push_subscriptions"

// RegisterSubscription tells Strava to start sending webhook events to
// callbackURL. verifyToken is the random secret Strava echoes back during
// the GET handshake (see VerifyChallenge).
func (c *Client) RegisterSubscription(ctx context.Context, callbackURL, verifyToken string) (Subscription, error) {
	form := url.Values{
		"client_id":     {c.ClientID},
		"client_secret": {c.ClientSecret},
		"callback_url":  {callbackURL},
		"verify_token":  {verifyToken},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, subscriptionsURL, strings.NewReader(form.Encode()))
	if err != nil {
		return Subscription{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var sub Subscription
	if err := c.doSubscriptionRequest(req, &sub); err != nil {
		return Subscription{}, err
	}
	return sub, nil
}

// ListSubscriptions returns every active subscription registered for the
// current Strava application.
func (c *Client) ListSubscriptions(ctx context.Context) ([]Subscription, error) {
	q := url.Values{
		"client_id":     {c.ClientID},
		"client_secret": {c.ClientSecret},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, subscriptionsURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	var subs []Subscription
	if err := c.doSubscriptionRequest(req, &subs); err != nil {
		return nil, err
	}
	return subs, nil
}

// DeleteSubscription cancels a subscription by ID. Strava returns 204 on
// success.
func (c *Client) DeleteSubscription(ctx context.Context, id int64) error {
	q := url.Values{
		"client_id":     {c.ClientID},
		"client_secret": {c.ClientSecret},
	}
	endpoint := fmt.Sprintf("%s/%d?%s", subscriptionsURL, id, q.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	return c.doSubscriptionRequest(req, nil)
}

// doSubscriptionRequest is the subscription-endpoint twin of Client.do(). It
// reuses HTTPClient and mapStatus but skips bearer-token injection since
// these endpoints authenticate via client_id/client_secret in the form.
func (c *Client) doSubscriptionRequest(req *http.Request, out any) error {
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("subscription request: %w", err)
	}
	defer resp.Body.Close()
	if err := mapStatus(resp); err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// VerifyChallenge implements the GET handshake half of Strava's webhook
// subscription flow. Strava calls our callback URL with `hub.mode=subscribe`,
// `hub.verify_token=<our secret>`, and `hub.challenge=<random>`. We compare
// the verify token (constant-time-ish; values are short and known) and echo
// the challenge back as `{"hub.challenge": "..."}`.
//
// expected is the verify_token we registered with the subscription.
func VerifyChallenge(w http.ResponseWriter, r *http.Request, expected string) {
	q := r.URL.Query()
	if q.Get("hub.mode") != "subscribe" {
		http.Error(w, "invalid hub.mode", http.StatusBadRequest)
		return
	}
	if q.Get("hub.verify_token") != expected {
		http.Error(w, "verify_token mismatch", http.StatusForbidden)
		return
	}
	challenge := q.Get("hub.challenge")
	if challenge == "" {
		http.Error(w, "missing hub.challenge", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"hub.challenge": challenge})
}

// DecodeWebhookEvent reads a WebhookEvent off an HTTP request body. It
// enforces a small max payload size to keep an attacker from streaming gigs
// at us; Strava events are well under 1KiB in practice.
func DecodeWebhookEvent(r *http.Request) (WebhookEvent, error) {
	const maxBody = 16 << 10
	body := http.MaxBytesReader(nil, r.Body, maxBody)
	defer body.Close()
	buf, err := io.ReadAll(body)
	if err != nil {
		return WebhookEvent{}, fmt.Errorf("read webhook body: %w", err)
	}
	var ev WebhookEvent
	if err := json.NewDecoder(bytes.NewReader(buf)).Decode(&ev); err != nil {
		return WebhookEvent{}, fmt.Errorf("decode webhook event: %w", err)
	}
	if ev.ObjectType == "" || ev.AspectType == "" {
		return WebhookEvent{}, errors.New("webhook event missing object/aspect type")
	}
	return ev, nil
}
