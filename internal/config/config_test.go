package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadConfigMissingAll(t *testing.T) {
	t.Setenv("CHATWOOT_BASE_URL", "")
	t.Setenv("CHATWOOT_API_TOKEN", "")
	t.Setenv("CHATWOOT_ACCOUNT_ID", "")
	_ = os.Unsetenv("CHATWOOT_BASE_URL")
	_ = os.Unsetenv("CHATWOOT_API_TOKEN")
	_ = os.Unsetenv("CHATWOOT_ACCOUNT_ID")
	_ = os.Unsetenv("CHATWOOT_TIMEOUT_SECONDS")
	_ = os.Unsetenv("CHATWOOT_ALLOWED_LABELS")
	_ = os.Unsetenv("CHATWOOT_READONLY")
	_ = os.Unsetenv("CHATWOOT_ALLOW_INSECURE")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, v := range []string{"CHATWOOT_BASE_URL", "CHATWOOT_API_TOKEN", "CHATWOOT_ACCOUNT_ID"} {
		if !strings.Contains(msg, v) {
			t.Errorf("missing %s in error: %s", v, msg)
		}
	}
}

func TestLoadConfigOK(t *testing.T) {
	t.Setenv("CHATWOOT_BASE_URL", "https://app.chatwoot.com/")
	t.Setenv("CHATWOOT_API_TOKEN", "tok")
	t.Setenv("CHATWOOT_ACCOUNT_ID", "7")
	t.Setenv("CHATWOOT_TIMEOUT_SECONDS", "15")
	t.Setenv("CHATWOOT_ALLOWED_LABELS", "lead-caliente, cotizacion-enviada, lead-caliente")
	t.Setenv("CHATWOOT_READONLY", "true")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BaseURL != "https://app.chatwoot.com" {
		t.Errorf("BaseURL=%q", cfg.BaseURL)
	}
	if cfg.AccountID != 7 {
		t.Errorf("AccountID=%d", cfg.AccountID)
	}
	if cfg.Timeout != 15*time.Second {
		t.Errorf("Timeout=%v", cfg.Timeout)
	}
	if !cfg.ReadOnly {
		t.Error("ReadOnly")
	}
	if len(cfg.AllowedLabels) != 2 {
		t.Errorf("AllowedLabels=%v", cfg.AllowedLabels)
	}
	if cfg.Transport != "http" {
		t.Errorf("Transport=%q", cfg.Transport)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr=%q", cfg.HTTPAddr)
	}
}

func TestLoadConfigRejectsHTTP(t *testing.T) {
	t.Setenv("CHATWOOT_BASE_URL", "http://localhost:3000")
	t.Setenv("CHATWOOT_API_TOKEN", "tok")
	t.Setenv("CHATWOOT_ACCOUNT_ID", "1")
	t.Setenv("CHATWOOT_ALLOW_INSECURE", "")
	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadConfigAllowsHTTPWhenInsecure(t *testing.T) {
	t.Setenv("CHATWOOT_BASE_URL", "http://localhost:3000")
	t.Setenv("CHATWOOT_API_TOKEN", "tok")
	t.Setenv("CHATWOOT_ACCOUNT_ID", "1")
	t.Setenv("CHATWOOT_ALLOW_INSECURE", "true")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BaseURL != "http://localhost:3000" {
		t.Errorf("BaseURL=%q", cfg.BaseURL)
	}
}

func TestLoadConfigBadAccountID(t *testing.T) {
	t.Setenv("CHATWOOT_BASE_URL", "https://app.chatwoot.com")
	t.Setenv("CHATWOOT_API_TOKEN", "tok")
	t.Setenv("CHATWOOT_ACCOUNT_ID", "abc")
	_, err := LoadConfig()
	if err == nil || !strings.Contains(err.Error(), "CHATWOOT_ACCOUNT_ID") {
		t.Fatalf("got %v", err)
	}
}

func TestLoadDotEnvDoesNotOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("FOO=fromfile\nBAR=filebar\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FOO", "fromenv")
	_ = os.Unsetenv("BAR")
	if err := LoadDotEnv(path); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("FOO") != "fromenv" {
		t.Errorf("FOO overridden")
	}
	if os.Getenv("BAR") != "filebar" {
		t.Errorf("BAR=%q", os.Getenv("BAR"))
	}
}

func TestHTTPPortFromEnv(t *testing.T) {
	t.Setenv("CHATWOOT_BASE_URL", "https://app.chatwoot.com")
	t.Setenv("CHATWOOT_API_TOKEN", "tok")
	t.Setenv("CHATWOOT_ACCOUNT_ID", "1")
	t.Setenv("PORT", "8080")
	t.Setenv("MCP_AUTH_TOKEN", "secret")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Transport != "http" || cfg.HTTPAddr != ":8080" {
		t.Fatalf("transport=%s addr=%s", cfg.Transport, cfg.HTTPAddr)
	}
	if cfg.AuthToken != "secret" {
		t.Errorf("AuthToken")
	}
}

func TestStdioTransport(t *testing.T) {
	t.Setenv("CHATWOOT_BASE_URL", "https://app.chatwoot.com")
	t.Setenv("CHATWOOT_API_TOKEN", "tok")
	t.Setenv("CHATWOOT_ACCOUNT_ID", "1")
	t.Setenv("MCP_TRANSPORT", "stdio")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Transport != "stdio" {
		t.Errorf("Transport=%q", cfg.Transport)
	}
}
