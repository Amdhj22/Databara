package telegram

import "encoding/json"

// apiResponse wraps every Bot API response. When OK is true, Result holds
// the typed payload (callers decode it). When OK is false, ErrorCode and
// Description carry the failure reason; Parameters may carry retry hints.
type apiResponse struct {
	OK          bool                `json:"ok"`
	Result      json.RawMessage     `json:"result,omitempty"`
	ErrorCode   int                 `json:"error_code,omitempty"`
	Description string              `json:"description,omitempty"`
	Parameters  *responseParameters `json:"parameters,omitempty"`
}

// responseParameters carries optional hints from Telegram. retry_after
// appears with 429 responses; we surface it via RateLimitError.
type responseParameters struct {
	RetryAfter int `json:"retry_after,omitempty"`
}
