package telegram

import (
	"context"
	"errors"
	"strings"
)

// sendMessageParams is the JSON body for the sendMessage Bot API method.
// Field tags must match Telegram's wire format exactly; omitempty on every
// optional field keeps the request body minimal so request signatures stay
// stable across calls.
type sendMessageParams struct {
	ChatID                int64  `json:"chat_id"`
	Text                  string `json:"text"`
	ParseMode             string `json:"parse_mode,omitempty"`
	DisableNotification   bool   `json:"disable_notification,omitempty"`
	DisableWebPagePreview bool   `json:"disable_web_page_preview,omitempty"`
}

// SendOption configures a single sendMessage call. Compose with
// WithParseMode, WithSilent, WithoutPreview.
type SendOption func(*sendMessageParams)

// WithParseMode formats the text. Use "MarkdownV2" or "HTML"; MarkdownV2
// requires escaping `_*[]()~` (and several other characters) in the message
// body — see https://core.telegram.org/bots/api#markdownv2-style.
func WithParseMode(mode string) SendOption {
	return func(p *sendMessageParams) { p.ParseMode = mode }
}

// WithSilent disables the notification sound for the message.
func WithSilent() SendOption {
	return func(p *sendMessageParams) { p.DisableNotification = true }
}

// WithoutPreview suppresses link previews for URLs in the text.
func WithoutPreview() SendOption {
	return func(p *sendMessageParams) { p.DisableWebPagePreview = true }
}

// SendMessage pushes text to the given chat. It returns nil on success and
// a sentinel-wrapped error otherwise (see errors.go).
//
// Telegram caps message text at 4096 characters; longer bodies come back as
// ErrBadRequest. Chunking is the caller's responsibility — Phase 1 coaching
// notes are well under the limit.
func (c *Client) SendMessage(ctx context.Context, chatID int64, text string, opts ...SendOption) error {
	if strings.TrimSpace(text) == "" {
		return errors.New("telegram.SendMessage: text is empty")
	}
	params := sendMessageParams{ChatID: chatID, Text: text}
	for _, opt := range opts {
		opt(&params)
	}
	return c.do(ctx, "sendMessage", params, nil)
}
