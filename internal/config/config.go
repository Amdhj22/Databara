package config

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/caarlos0/env/v11"
)

// Config bundles every runtime setting Databara needs.
//
// Values are loaded from process environment variables. Local development can
// optionally place a .env file next to the binary; main is expected to load it
// before calling Load (see github.com/joho/godotenv).
type Config struct {
	Strava   Strava
	Telegram Telegram
	Claude   Claude
	Server   Server
}

// Strava holds OAuth credentials and webhook subscription secrets.
type Strava struct {
	ClientID     string `env:"STRAVA_CLIENT_ID,required"`
	ClientSecret string `env:"STRAVA_CLIENT_SECRET,required"`
	// VerifyToken is echoed back when Strava registers our webhook URL.
	VerifyToken string `env:"STRAVA_VERIFY_TOKEN,required"`
	// RefreshToken is the initial athlete refresh token. It is rotated on
	// every token refresh and the latest value is persisted in storage; this
	// field exists only to bootstrap a fresh install.
	RefreshToken string `env:"STRAVA_REFRESH_TOKEN"`
}

// Telegram holds bot credentials and access control settings.
type Telegram struct {
	BotToken string `env:"TELEGRAM_BOT_TOKEN,required"`
	// WebhookSecret is sent back by Telegram in the
	// X-Telegram-Bot-Api-Secret-Token header so we can reject forged updates.
	WebhookSecret  string  `env:"TELEGRAM_WEBHOOK_SECRET,required"`
	AllowedChatIDs []int64 `env:"TELEGRAM_ALLOWED_CHAT_IDS" envSeparator:","`
}

// Claude holds Anthropic API credentials and model selection.
type Claude struct {
	APIKey string `env:"ANTHROPIC_API_KEY,required"`
	Model  string `env:"ANTHROPIC_MODEL" envDefault:"claude-sonnet-4-6"`
}

// Server holds process-level settings: bind address, log level, on-disk
// database path, and the public base URL that Strava/Telegram webhooks point
// at.
type Server struct {
	HTTPAddr      string `env:"HTTP_ADDR" envDefault:":8080"`
	LogLevel      string `env:"LOG_LEVEL" envDefault:"info"`
	DBPath        string `env:"DB_PATH" envDefault:"./databara.db"`
	PublicBaseURL string `env:"PUBLIC_BASE_URL,required"`
}

// Load reads configuration from the process environment.
//
// It does not touch .env files; callers (typically main) should load those
// first if desired. Returning the parsed config is intentionally separate from
// dotenv loading so tests can drive configuration purely through t.Setenv.
func Load() (*Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}

// SlogLevel maps the LOG_LEVEL string onto slog.Level. Unknown values fall
// back to Info so a typo never silences the logger entirely.
func (s Server) SlogLevel() slog.Level {
	switch strings.ToLower(s.LogLevel) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
