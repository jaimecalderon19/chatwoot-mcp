package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
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

	if cfg.Transport == "stdio" {
		if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			logger.Error("servidor MCP", "error", err)
			os.Exit(1)
		}
		return
	}

	if cfg.AuthToken == "" {
		logger.Warn("MCP_AUTH_TOKEN vacío: el endpoint MCP queda público")
	}

	mcpHandler := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{Logger: logger})

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "mcp": "/mcp"})
			return
		}
		http.NotFound(w, r)
	})
	mux.Handle("/mcp", mcpHandler)
	mux.Handle("/mcp/", mcpHandler)

	handler := withAuth(cfg.AuthToken, mux)
	logger.Info("mcp http", "addr", cfg.HTTPAddr, "path", "/mcp")
	if err := http.ListenAndServe(cfg.HTTPAddr, handler); err != nil {
		logger.Error("servidor HTTP", "error", err)
		os.Exit(1)
	}
}

func withAuth(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" || r.URL.Path == "/" {
			next.ServeHTTP(w, r)
			return
		}
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}
		got := bearerToken(r)
		if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func bearerToken(r *http.Request) string {
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(h) >= 7 && strings.EqualFold(h[:7], "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	if v := strings.TrimSpace(r.Header.Get("X-Api-Key")); v != "" {
		return v
	}
	return h
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
