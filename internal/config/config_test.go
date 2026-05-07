package config

import (
	"log/slog"
	"os"
	"testing"
)

// envKeys lists every variable Config touches. Tests clear these before
// running so a stray value in the developer's shell can't make a "missing
// required" assertion silently pass.
var envKeys = []string{
	"STRAVA_CLIENT_ID",
	"STRAVA_CLIENT_SECRET",
	"STRAVA_VERIFY_TOKEN",
	"STRAVA_REFRESH_TOKEN",
	"TELEGRAM_BOT_TOKEN",
	"TELEGRAM_WEBHOOK_SECRET",
	"TELEGRAM_ALLOWED_CHAT_IDS",
	"ANTHROPIC_API_KEY",
	"ANTHROPIC_MODEL",
	"HTTP_ADDR",
	"LOG_LEVEL",
	"DB_PATH",
	"PUBLIC_BASE_URL",
}

func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range envKeys {
		saved, ok := os.LookupEnv(k)
		_ = os.Unsetenv(k)
		if ok {
			t.Cleanup(func() { _ = os.Setenv(k, saved) })
		}
	}
}

func setRequired(t *testing.T) {
	t.Helper()
	t.Setenv("STRAVA_CLIENT_ID", "cid")
	t.Setenv("STRAVA_CLIENT_SECRET", "csec")
	t.Setenv("STRAVA_VERIFY_TOKEN", "vtok")
	t.Setenv("TELEGRAM_BOT_TOKEN", "btok")
	t.Setenv("TELEGRAM_WEBHOOK_SECRET", "wsec")
	t.Setenv("ANTHROPIC_API_KEY", "ak")
	t.Setenv("PUBLIC_BASE_URL", "https://databara.example.com")
}

func TestLoad_Defaults(t *testing.T) {
	clearEnv(t)
	setRequired(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Server.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr default = %q, want :8080", cfg.Server.HTTPAddr)
	}
	if cfg.Server.LogLevel != "info" {
		t.Errorf("LogLevel default = %q, want info", cfg.Server.LogLevel)
	}
	if cfg.Server.DBPath != "./databara.db" {
		t.Errorf("DBPath default = %q, want ./databara.db", cfg.Server.DBPath)
	}
	if cfg.Claude.Model != "claude-sonnet-4-6" {
		t.Errorf("Claude.Model default = %q, want claude-sonnet-4-6", cfg.Claude.Model)
	}
	if len(cfg.Telegram.AllowedChatIDs) != 0 {
		t.Errorf("AllowedChatIDs = %v, want empty", cfg.Telegram.AllowedChatIDs)
	}
}

func TestLoad_AllowedChatIDsParsing(t *testing.T) {
	clearEnv(t)
	setRequired(t)
	t.Setenv("TELEGRAM_ALLOWED_CHAT_IDS", "111,2222,-3333")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	want := []int64{111, 2222, -3333}
	if len(cfg.Telegram.AllowedChatIDs) != len(want) {
		t.Fatalf("AllowedChatIDs = %v, want %v", cfg.Telegram.AllowedChatIDs, want)
	}
	for i, v := range want {
		if cfg.Telegram.AllowedChatIDs[i] != v {
			t.Errorf("AllowedChatIDs[%d] = %d, want %d", i, cfg.Telegram.AllowedChatIDs[i], v)
		}
	}
}

func TestLoad_MissingRequiredFails(t *testing.T) {
	tests := []string{
		"STRAVA_CLIENT_ID",
		"STRAVA_CLIENT_SECRET",
		"STRAVA_VERIFY_TOKEN",
		"TELEGRAM_BOT_TOKEN",
		"TELEGRAM_WEBHOOK_SECRET",
		"ANTHROPIC_API_KEY",
		"PUBLIC_BASE_URL",
	}
	for _, missing := range tests {
		t.Run(missing, func(t *testing.T) {
			clearEnv(t)
			setRequired(t)
			_ = os.Unsetenv(missing)

			if _, err := Load(); err == nil {
				t.Fatalf("Load() with %s unset returned nil error, want error", missing)
			}
		})
	}
}

func TestSlogLevel(t *testing.T) {
	tests := []struct {
		in   string
		want slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"", slog.LevelInfo},
		{"bogus", slog.LevelInfo},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got := Server{LogLevel: tt.in}.SlogLevel()
			if got != tt.want {
				t.Errorf("SlogLevel(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
