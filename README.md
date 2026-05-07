# Databara

Personal AI training assistant. When a new activity lands on Strava, Databara fetches it, runs an analysis, and pushes a coaching note to Telegram. You can reply on Telegram to discuss the activity in natural language.

Supports Cycling, Running, and Swimming.

## Status

Pre-PoC. Scaffold only.

## Architecture

```
[Strava upload] -- webhook -->  Databara  -- Telegram Bot API --> [you]
                                   |   ^
                                   |   |
                              Strava REST API
                              Anthropic Claude API
```

- HTTP server receives Strava and Telegram webhooks.
- Activities are fetched, analyzed per sport, summarized via Claude, and sent to Telegram.
- User replies are routed back through Claude with conversation context.
- All state lives in a local SQLite file.

## Requirements

- Go 1.25+
- A Strava API application (`client_id`, `client_secret`)
- A Telegram bot token (from `@BotFather`)
- An Anthropic API key
- A public HTTPS endpoint (e.g. Cloudflare Tunnel) for webhooks

## Setup

```bash
git clone git@github.com:Amdhj22/databara.git
cd databara
cp .env.example .env
# Fill in .env

make run
```

## Development

```bash
make build   # produce bin/databara
make test    # go test -race
make lint    # golangci-lint
make tidy    # go mod tidy
make fmt     # gofmt + goimports
```

## Layout

```
cmd/databara/        Entry point
internal/
  config/            Environment config loader
  strava/            Strava OAuth, API client, webhook handler
  telegram/          Telegram bot client and webhook handler
  claude/            Anthropic Claude client and prompt builders
  analyzer/          Per-sport activity analysis
  load/              Training Load metrics (ATL, CTL, TSB)
  storage/           SQLite repositories
  chat/              Conversation context management
migrations/          SQL migration files
deploy/              launchd plist, cloudflared config
```

## Roadmap

See the project notebook for phased goals (PoC → MVP → Two-way chat → Training Load → Reports).

## License

Private.
