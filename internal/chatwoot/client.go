package chatwoot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"chatwoot-mcp/internal/config"
)

const (
	maxRetries = 2
	cacheTTL   = 5 * time.Minute
	userAgent  = "chatwoot-mcp/1.0"
)

type Client struct {
	baseURL    string
	token      string
	accountID  int
	timeout    time.Duration
	httpClient *http.Client
	logger     *slog.Logger
	cache      *ttlCache
}

func New(cfg *config.Config, logger *slog.Logger, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: cfg.Timeout}
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		token:      cfg.APIToken,
		accountID:  cfg.AccountID,
		timeout:    cfg.Timeout,
		httpClient: httpClient,
		logger:     logger,
		cache:      newTTLCache(cacheTTL),
	}
}

func (c *Client) accountURL(path string) string {
	return fmt.Sprintf("%s/api/v1/accounts/%d%s", c.baseURL, c.accountID, path)
}

func (c *Client) GetProfile(ctx context.Context) (*Profile, error) {
	data, err := c.do(ctx, http.MethodGet, c.baseURL+"/api/v1/profile", nil, nil)
	if err != nil {
		return nil, err
	}
	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse profile: %w", err)
	}
	return &p, nil
}

func (c *Client) do(ctx context.Context, method, rawURL string, query url.Values, body any) ([]byte, error) {
	retry := method == http.MethodGet
	var lastErr error
	attempts := 1
	if retry {
		attempts = 1 + maxRetries
	}

	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			wait := backoff(attempt)
			var ra time.Duration
			if apiErr, ok := lastErr.(*APIError); ok {
				ra = apiErr.RetryAfter
			}
			if ra > wait {
				wait = ra
			}
			if wait > 30*time.Second {
				wait = 30 * time.Second
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
			}
		}

		data, status, retryAfter, err := c.once(ctx, method, rawURL, query, body)
		if err != nil {
			if retry && isRetryableNet(err) && attempt < attempts-1 {
				lastErr = err
				c.logger.Debug("retry", "method", method, "url", sanitizeURL(rawURL), "attempt", attempt+1, "error", err.Error())
				continue
			}
			return nil, err
		}
		if status >= 200 && status < 300 {
			return data, nil
		}
		apiErr := mapError(status, data)
		apiErr.RetryAfter = retryAfter
		if retry && shouldRetryStatus(status) && attempt < attempts-1 {
			lastErr = apiErr
			c.logger.Debug("retry", "method", method, "url", sanitizeURL(rawURL), "attempt", attempt+1, "status", status)
			continue
		}
		return nil, apiErr
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("petición fallida")
}

func (c *Client) once(ctx context.Context, method, rawURL string, query url.Values, body any) ([]byte, int, time.Duration, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, 0, 0, err
	}
	if query != nil {
		q := u.Query()
		for k, vs := range query {
			for _, v := range vs {
				q.Add(k, v)
			}
		}
		u.RawQuery = q.Encode()
	}

	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, 0, err
		}
		rdr = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), rdr)
	if err != nil {
		return nil, 0, 0, err
	}
	req.Header.Set("api_access_token", c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	latency := time.Since(start)
	if err != nil {
		c.logger.Debug("http", "method", method, "path", u.Path, "latency_ms", latency.Milliseconds(), "error", err.Error())
		return nil, 0, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, 0, err
	}
	c.logger.Debug("http",
		"method", method,
		"path", u.Path,
		"status", resp.StatusCode,
		"latency_ms", latency.Milliseconds(),
	)
	return data, resp.StatusCode, parseRetryAfter(resp.Header.Get("Retry-After")), nil
}

func (c *Client) get(ctx context.Context, path string, query url.Values) ([]byte, error) {
	return c.do(ctx, http.MethodGet, c.accountURL(path), query, nil)
}

func (c *Client) post(ctx context.Context, path string, body any) ([]byte, error) {
	return c.do(ctx, http.MethodPost, c.accountURL(path), nil, body)
}

func (c *Client) put(ctx context.Context, path string, body any) ([]byte, error) {
	return c.do(ctx, http.MethodPut, c.accountURL(path), nil, body)
}

func (c *Client) postQuery(ctx context.Context, path string, query url.Values, body any) ([]byte, error) {
	return c.do(ctx, http.MethodPost, c.accountURL(path), query, body)
}

func backoff(attempt int) time.Duration {
	base := 200 * time.Millisecond
	d := base * time.Duration(1<<uint(attempt-1))
	j := time.Duration(rand.Int64N(int64(d/2) + 1))
	return d + j
}

func shouldRetryStatus(status int) bool {
	switch status {
	case 429, 502, 503, 504:
		return true
	default:
		return false
	}
}

func isRetryableNet(err error) bool {
	if err == nil {
		return false
	}
	if err == context.Canceled || err == context.DeadlineExceeded {
		return false
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "temporary") ||
		strings.Contains(msg, "eof") {
		return true
	}
	return true
}

func parseRetryAfter(h string) time.Duration {
	h = strings.TrimSpace(h)
	if h == "" {
		return 0
	}
	if n, err := strconv.Atoi(h); err == nil {
		if n < 0 {
			return 0
		}
		return time.Duration(n) * time.Second
	}
	if t, err := http.ParseTime(h); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d
		}
	}
	return 0
}

func sanitizeURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.User = nil
	u.RawQuery = ""
	return u.String()
}

func intQuery(q url.Values, key string, n int) {
	if n != 0 {
		q.Set(key, strconv.Itoa(n))
	}
}
