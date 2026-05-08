package server

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/Amdhj22/databara/internal/strava"
)

// Server holds the HTTP-level dependencies: the verify token Strava echoes
// during subscription registration and the Worker that absorbs decoded
// events. Construction lives in cmd/databara/main.go; this package never
// touches config or environment variables directly.
type Server struct {
	VerifyToken string
	Worker      *Worker
}

// New returns a Server ready to mount onto an http.ServeMux. Callers wire
// it via Routes.
func New(verifyToken string, worker *Worker) *Server {
	return &Server{VerifyToken: verifyToken, Worker: worker}
}

// Routes registers every Databara endpoint on mux. Keeping it as a method
// on Server (instead of returning a fresh mux) lets main.go layer in its
// own middleware or extra routes without forking this code.
func (s *Server) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", healthz)
	mux.HandleFunc("GET /webhooks/strava", s.handleStravaChallenge)
	mux.HandleFunc("POST /webhooks/strava", s.handleStravaEvent)
}

// healthz answers the load-balancer / external monitor with a plain "ok".
// No auth — by design; nothing sensitive flows through this endpoint.
func healthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// handleStravaChallenge handles the GET half of Strava's webhook
// subscription handshake. Strava calls our callback with a verify_token
// and a challenge string; we echo the challenge back if the token matches.
//
// The actual comparison + JSON echo lives in strava.VerifyChallenge so the
// security-sensitive bit is colocated with the rest of the protocol code.
func (s *Server) handleStravaChallenge(w http.ResponseWriter, r *http.Request) {
	strava.VerifyChallenge(w, r, s.VerifyToken)
}

// handleStravaEvent handles the POST half: decode the event, push it onto
// the worker queue, return 200 immediately so Strava's 2-second deadline
// is never an issue.
//
// A full queue is logged and dropped — returning 5xx would just provoke
// Strava to retry into a deeper backlog. A malformed body returns 400 so
// the operator can spot a misconfigured subscription quickly.
func (s *Server) handleStravaEvent(w http.ResponseWriter, r *http.Request) {
	ev, err := strava.DecodeWebhookEvent(r)
	if err != nil {
		slog.Warn("invalid strava webhook payload", "err", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if err := s.Worker.Enqueue(ev); err != nil {
		if errors.Is(err, ErrQueueFull) {
			slog.Warn("worker queue full, dropping event",
				"object_id", ev.ObjectID, "object_type", ev.ObjectType)
		} else {
			slog.Error("worker enqueue failed", "err", err)
		}
	}
	w.WriteHeader(http.StatusOK)
}
