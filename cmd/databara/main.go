package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/Amdhj22/databara/internal/analyzer"
	"github.com/Amdhj22/databara/internal/claude"
	"github.com/Amdhj22/databara/internal/config"
	"github.com/Amdhj22/databara/internal/server"
	"github.com/Amdhj22/databara/internal/storage"
	"github.com/Amdhj22/databara/internal/strava"
	"github.com/Amdhj22/databara/internal/telegram"
)

// queueBuffer is the depth of the worker channel between the HTTP handler
// and the activity-processing goroutine. Activities arrive one at a time
// per athlete, so 16 is plenty of headroom for bursts.
const queueBuffer = 16

// shutdownTimeout caps how long the HTTP server gets to drain in-flight
// requests after a SIGTERM. Anything longer just delays operator feedback.
const shutdownTimeout = 10 * time.Second

func main() {
	// .env is for local development only; ignore the error so production
	// deployments (which inject env vars directly) aren't bothered.
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		_, _ = os.Stderr.WriteString("config: " + err.Error() + "\n")
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.Server.SlogLevel(),
	}))
	slog.SetDefault(logger)

	pushChatID, err := pickPushChatID(cfg)
	if err != nil {
		slog.Error("startup", "err", err)
		os.Exit(1)
	}

	tokens := storage.NewFileTokenStore(filepath(cfg))
	stravaClient := strava.New(cfg.Strava.ClientID, cfg.Strava.ClientSecret, tokens)

	if err := seedRefreshTokenIfNeeded(stravaClient.Tokens, cfg); err != nil {
		slog.Error("seed token", "err", err)
		os.Exit(1)
	}

	claudeClient := claude.New(cfg.Claude.APIKey, cfg.Claude.Model)
	telegramClient := telegram.New(cfg.Telegram.BotToken)
	reviewer := analyzer.New(claudeClient)

	// Adapt Telegram's variadic SendMessage to the non-variadic Sender
	// contract. No options are passed today; future sport-specific routes
	// can add WithParseMode here without changing the server package.
	send := server.SenderFunc(func(ctx context.Context, chatID int64, text string) error {
		return telegramClient.SendMessage(ctx, chatID, text)
	})

	processor := &server.Processor{
		Fetcher:  stravaClient,
		Reviewer: reviewer,
		Sender:   send,
		ChatID:   pushChatID,
	}

	worker := server.NewWorker(queueBuffer, processor.Handle)
	app := server.New(cfg.Strava.VerifyToken, worker)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go worker.Run(ctx)

	mux := http.NewServeMux()
	app.Routes(mux)
	srv := &http.Server{
		Addr:              cfg.Server.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("server starting",
			"addr", cfg.Server.HTTPAddr,
			"chat_id", pushChatID,
			"model", cfg.Claude.Model)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("listen failed", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	slog.Info("shutdown initiated")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("http shutdown failed", "err", err)
	}
	worker.Wait()
	slog.Info("server stopped")
}

// pickPushChatID returns the single chat ID Databara pushes coaching notes
// to. Phase 1 is single-user, so the first allowed chat is chosen; an empty
// list is a configuration error.
func pickPushChatID(cfg *config.Config) (int64, error) {
	if len(cfg.Telegram.AllowedChatIDs) == 0 {
		return 0, errors.New("TELEGRAM_ALLOWED_CHAT_IDS must contain at least one chat ID")
	}
	return cfg.Telegram.AllowedChatIDs[0], nil
}

// filepath resolves the on-disk token file path. Currently a sibling of
// DBPath so both pieces of state live next to each other on disk; once
// SQLite lands in Phase 2 the token file moves into the same directory.
func filepath(cfg *config.Config) string {
	return cfg.Server.DBPath + ".tokens.json"
}

// seedRefreshTokenIfNeeded persists the bootstrap STRAVA_REFRESH_TOKEN from
// the environment into the on-disk store if no token has been saved yet.
// Without this step a brand-new install can't authenticate to Strava — the
// rotation-aware client expects a TokenSet on first call.
//
// On subsequent runs the on-disk token wins (rotation is authoritative);
// the env var is read only when the file is missing.
func seedRefreshTokenIfNeeded(src strava.TokenSource, cfg *config.Config) error {
	if cfg.Strava.RefreshToken == "" {
		return nil
	}
	if _, err := src.Get(context.Background()); err == nil {
		return nil // token already on disk; bootstrap not needed
	} else if !errors.Is(err, storage.ErrNoToken) {
		return err
	}

	// Seed an obviously-expired access token so the next API call triggers
	// a refresh against Strava using the bootstrap refresh token.
	seed := strava.TokenSet{
		AccessToken:  "bootstrap",
		RefreshToken: cfg.Strava.RefreshToken,
		ExpiresAt:    time.Unix(0, 0),
	}
	if err := src.Save(context.Background(), seed); err != nil {
		return err
	}
	slog.Info("seeded bootstrap refresh token from env")
	return nil
}
