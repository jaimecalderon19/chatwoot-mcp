package config

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	BaseURL       string
	APIToken      string
	AccountID     int
	Timeout       time.Duration
	AllowedLabels []string
	ReadOnly      bool
}

func LoadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}
	return sc.Err()
}

func LoadConfig() (*Config, error) {
	var missing []string
	var errs []string

	baseURL := strings.TrimSpace(os.Getenv("CHATWOOT_BASE_URL"))
	if baseURL == "" {
		missing = append(missing, "CHATWOOT_BASE_URL")
	}

	token := strings.TrimSpace(os.Getenv("CHATWOOT_API_TOKEN"))
	if token == "" {
		missing = append(missing, "CHATWOOT_API_TOKEN")
	}

	accountRaw := strings.TrimSpace(os.Getenv("CHATWOOT_ACCOUNT_ID"))
	if accountRaw == "" {
		missing = append(missing, "CHATWOOT_ACCOUNT_ID")
	}

	if len(missing) > 0 {
		errs = append(errs, "faltan variables de entorno: "+strings.Join(missing, ", "))
	}

	cfg := &Config{
		APIToken: token,
		Timeout:  30 * time.Second,
	}

	if baseURL != "" {
		normalized, err := normalizeBaseURL(baseURL)
		if err != nil {
			errs = append(errs, err.Error())
		} else {
			cfg.BaseURL = normalized
		}
	}

	if accountRaw != "" {
		id, err := strconv.Atoi(accountRaw)
		if err != nil || id <= 0 {
			errs = append(errs, "CHATWOOT_ACCOUNT_ID debe ser un entero positivo")
		} else {
			cfg.AccountID = id
		}
	}

	if v := strings.TrimSpace(os.Getenv("CHATWOOT_TIMEOUT_SECONDS")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			errs = append(errs, "CHATWOOT_TIMEOUT_SECONDS debe ser un entero positivo")
		} else {
			cfg.Timeout = time.Duration(n) * time.Second
		}
	}

	cfg.AllowedLabels = parseCSV(os.Getenv("CHATWOOT_ALLOWED_LABELS"))
	cfg.ReadOnly = parseBool(os.Getenv("CHATWOOT_READONLY"))

	if len(errs) > 0 {
		return nil, fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return cfg, nil
}

func normalizeBaseURL(raw string) (string, error) {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("CHATWOOT_BASE_URL debe ser una URL absoluta (ej: https://app.chatwoot.com)")
	}
	allowInsecure := parseBool(os.Getenv("CHATWOOT_ALLOW_INSECURE"))
	switch u.Scheme {
	case "https":
	case "http":
		if !allowInsecure {
			return "", fmt.Errorf("CHATWOOT_BASE_URL debe usar https (o define CHATWOOT_ALLOW_INSECURE=true para desarrollo local)")
		}
	default:
		return "", fmt.Errorf("CHATWOOT_BASE_URL debe usar esquema https")
	}
	return raw, nil
}

func parseCSV(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseBool(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}
