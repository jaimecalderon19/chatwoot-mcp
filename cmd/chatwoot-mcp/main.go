package main

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"chatwoot-mcp/internal/chatwoot"
	"chatwoot-mcp/internal/config"
	"chatwoot-mcp/internal/guard"
	"chatwoot-mcp/internal/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const version = "1.0.0"

func main() {
	logger := setupLogger(os.Getenv("LOG_LEVEL"))
	slog.SetDefault(logger)

	if strings.EqualFold(os.Getenv("APP_ENV"), "development") {
		if err := config.LoadDotEnv(".env"); err != nil {
			logger.Error("no se pudo leer .env", "error", err)
			os.Exit(1)
		}
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	cw := chatwoot.New(cfg, logger, nil)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	profile, err := cw.GetProfile(ctx)
	cancel()
	if err != nil {
		if apiErr, ok := err.(*chatwoot.APIError); ok && apiErr.Status == 401 {
			logger.Error("token inválido o revocado")
			os.Exit(1)
		}
		logger.Error("no se pudo verificar el token", "error", err)
		os.Exit(1)
	}
	logger.Info("autenticado", "id", profile.ID, "email", profile.Email, "name", profile.Name)

	h := &tools.Handler{
		CW:     cw,
		Guard:  guard.New(cfg.AllowedLabels),
		Logger: logger,
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "chatwoot", Version: version}, nil)
	tools.Register(server, h, cfg.ReadOnly)
	if cfg.ReadOnly {
		logger.Info("modo solo lectura")
	}

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		logger.Error("servidor MCP", "error", err)
		os.Exit(1)
	}
}

func setupLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: lvl}))
}
