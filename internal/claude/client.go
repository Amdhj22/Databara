package claude

import (
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// Client wraps the Anthropic SDK with Databara's defaults: the model picked
// at construction time, plus the system prompt configured in prompts.go.
//
// It is safe for concurrent use; the underlying SDK client owns its HTTP
// connection pool and built-in retry logic.
type Client struct {
	sdk   anthropic.Client
	model anthropic.Model
}

// New returns a Client wired to the given API key and model.
//
// The variadic opts parameter forwards directly to anthropic.NewClient, so
// tests can inject option.WithBaseURL(httptestServer.URL) and production
// callers can layer in a custom http.Client, retry policy, request timeout,
// and so on.
func New(apiKey, model string, opts ...option.RequestOption) *Client {
	base := []option.RequestOption{option.WithAPIKey(apiKey)}
	return &Client{
		sdk:   anthropic.NewClient(append(base, opts...)...),
		model: anthropic.Model(model),
	}
}
