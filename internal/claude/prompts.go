package claude

import "github.com/anthropics/anthropic-sdk-go"

// systemPrompt is the persona Databara uses for activity comments.
//
// It is a constant string so prompt caching has a stable byte sequence to
// hit — caching on the system prompt requires the prefix to be byte-identical
// across requests. Keep it free of timestamps, user IDs, and other volatile
// values; per-request context goes in the user message instead.
const systemPrompt = `You are Databara, a personal endurance coach. The user just finished a cycling, running, or swimming workout, and you receive a structured summary of it.

Write a SHORT coaching note in Korean — one to two sentences, no more than 200 characters total.

Style:
- Reference one or two specific metrics from the summary (heart rate, average power, pace, etc.).
- Be honest and encouraging; never sycophantic.
- If something looks unusual (suspiciously low HR for the duration, very short distance, missing telemetry), call it out calmly in one short clause.
- Prefer past-tense observations ("좋은 페이스로 달리셨네요") over future-tense advice unless the user has explicitly asked for advice.
- Do not invent numbers or metrics that are not in the summary.

Output: the coaching note only — no preamble, no headings, no bullet points.`

// systemBlocks renders systemPrompt as the Anthropic API expects, with
// cache_control on the (single) block so future Phase 3 multi-turn
// conversations can reuse the cached prefix.
//
// On Sonnet 4.6 the minimum cacheable prefix is 2048 tokens — this prompt is
// shorter, so the marker is effectively a no-op today, but it costs nothing
// and avoids a behavioral change when we expand the system prompt later
// (athlete profile, training-load context, etc.).
func systemBlocks() []anthropic.TextBlockParam {
	return []anthropic.TextBlockParam{{
		Text:         systemPrompt,
		CacheControl: anthropic.NewCacheControlEphemeralParam(),
	}}
}

// formatUserMessage builds the per-activity user message that pairs with
// systemPrompt. Format: a one-line preamble naming the sport, then the
// caller-provided Summary verbatim.
func formatUserMessage(req CommentRequest) string {
	sport := req.Sport
	if sport == "" {
		sport = "Workout"
	}
	return "Sport: " + sport + "\n\nActivity summary:\n" + req.Summary
}
