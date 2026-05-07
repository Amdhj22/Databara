package claude

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

// maxCommentTokens caps the response length. The system prompt asks for one
// to two sentences (~200 chars); 300 tokens is a generous ceiling that still
// keeps the call latency low.
const maxCommentTokens = 300

// Comment generates a one-paragraph coaching note for the given activity.
//
// Each invocation is an independent /v1/messages request with no carry-over
// conversation state. Phase 3 will introduce a separate Chat method that
// reuses systemBlocks() so the cached prefix survives across follow-up
// questions about the same activity.
func (c *Client) Comment(ctx context.Context, req CommentRequest) (string, error) {
	if strings.TrimSpace(req.Summary) == "" {
		return "", errors.New("claude.Comment: Summary is empty")
	}

	resp, err := c.sdk.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     c.model,
		MaxTokens: maxCommentTokens,
		System:    systemBlocks(),
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(formatUserMessage(req))),
		},
	})
	if err != nil {
		return "", fmt.Errorf("claude.Comment: %w", err)
	}
	return extractText(resp)
}

// extractText concatenates every text block in the response. Anthropic's API
// can split a reply across multiple blocks (e.g. when extended thinking is
// on); for the comment path there is normally one block, but handling many
// is essentially free.
func extractText(msg *anthropic.Message) (string, error) {
	var parts []string
	for _, block := range msg.Content {
		if t, ok := block.AsAny().(anthropic.TextBlock); ok {
			parts = append(parts, t.Text)
		}
	}
	if len(parts) == 0 {
		return "", errors.New("claude: response had no text blocks")
	}
	return strings.TrimSpace(strings.Join(parts, "\n")), nil
}
