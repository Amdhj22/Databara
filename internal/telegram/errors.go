package telegram

import (
	"errors"
	"fmt"
	"time"
)

// Sentinel errors returned by the client. Use errors.Is to branch — calls
// wrap the sentinel with the API description so the message is informative.
//
// Telegram returns HTTP 200 even for application failures and signals the
// real status via the envelope's `error_code`, so these correspond to
// Bot API codes, not transport-level codes.
var (
	ErrBadRequest   = errors.New("telegram: bad request")    // 400
	ErrUnauthorized = errors.New("telegram: unauthorized")   // 401
	ErrForbidden    = errors.New("telegram: forbidden")      // 403
	ErrNotFound     = errors.New("telegram: not found")      // 404
	ErrConflict     = errors.New("telegram: conflict")       // 409
	ErrRateLimited  = errors.New("telegram: rate limited")   // 429
	ErrServerError  = errors.New("telegram: server error")   // 5xx
	ErrAPIError     = errors.New("telegram: api error")      // unknown code
)

// RateLimitError wraps ErrRateLimited and exposes Telegram's retry_after
// hint as a parsed time.Duration. Use errors.As to read RetryAfter and
// errors.Is(err, ErrRateLimited) to branch on the sentinel.
type RateLimitError struct {
	Description string
	RetryAfter  time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("%s: %s (retry after %s)", ErrRateLimited, e.Description, e.RetryAfter)
}

// Is makes errors.Is(err, ErrRateLimited) match a *RateLimitError.
func (e *RateLimitError) Is(target error) bool {
	return target == ErrRateLimited
}

// mapError translates an OK==false Bot API envelope into the sentinel-
// wrapped error families above. The HTTP layer never sees these — Telegram
// returns 200 even on application errors.
func mapError(resp apiResponse) error {
	desc := resp.Description
	switch {
	case resp.ErrorCode == 400:
		return fmt.Errorf("%w: %s", ErrBadRequest, desc)
	case resp.ErrorCode == 401:
		return fmt.Errorf("%w: %s", ErrUnauthorized, desc)
	case resp.ErrorCode == 403:
		return fmt.Errorf("%w: %s", ErrForbidden, desc)
	case resp.ErrorCode == 404:
		return fmt.Errorf("%w: %s", ErrNotFound, desc)
	case resp.ErrorCode == 409:
		return fmt.Errorf("%w: %s", ErrConflict, desc)
	case resp.ErrorCode == 429:
		retry := time.Duration(0)
		if resp.Parameters != nil {
			retry = time.Duration(resp.Parameters.RetryAfter) * time.Second
		}
		return &RateLimitError{Description: desc, RetryAfter: retry}
	case resp.ErrorCode >= 500:
		return fmt.Errorf("%w: %s", ErrServerError, desc)
	default:
		return fmt.Errorf("%w: code=%d %s", ErrAPIError, resp.ErrorCode, desc)
	}
}
