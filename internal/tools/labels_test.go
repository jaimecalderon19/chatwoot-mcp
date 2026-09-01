package tools

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"chatwoot-mcp/internal/chatwoot"
	"chatwoot-mcp/internal/config"
	"chatwoot-mcp/internal/guard"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestAddConversationLabelsKeepsExisting(t *testing.T) {
	var posted []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"payload":["existente","humano"]}`))
			return
		}
		var body struct {
			Labels []string `json:"labels"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		posted = append([]string{}, body.Labels...)
		b, _ := json.Marshal(map[string]any{"payload": body.Labels})
		_, _ = w.Write(b)
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{BaseURL: srv.URL, APIToken: "t", AccountID: 1, Timeout: time.Second}
	h := &Handler{
		CW:     chatwoot.New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), srv.Client()),
		Guard:  guard.New(nil),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	res, _, err := h.addConversationLabels(context.Background(), &mcp.CallToolRequest{}, labelsIn{
		ConversationID: 1,
		Labels:         []string{"lead-caliente"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res != nil && res.IsError {
		t.Fatalf("tool error: %+v", res.Content)
	}
	if len(posted) != 3 || posted[0] != "existente" || posted[1] != "humano" || posted[2] != "lead-caliente" {
		t.Fatalf("posted=%v (existing labels were overwritten)", posted)
	}
}

func TestUpdateStatusSendsUnixSnooze(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"payload":{"id":1,"status":"snoozed"}}`))
	}))
	t.Cleanup(srv.Close)
	cfg := &config.Config{BaseURL: srv.URL, APIToken: "t", AccountID: 1, Timeout: time.Second}
	now := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	h := &Handler{
		CW:     chatwoot.New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), srv.Client()),
		Guard:  guard.New(nil),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:    func() time.Time { return now },
	}
	res, _, err := h.updateConversationStatus(context.Background(), &mcp.CallToolRequest{}, updateStatusIn{
		ConversationID: 1,
		Status:         "snoozed",
		SnoozeUntilISO: "2026-09-08T14:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res != nil && res.IsError {
		t.Fatalf("tool error: %+v", res.Content)
	}
	want := float64(time.Date(2026, 9, 8, 14, 0, 0, 0, time.UTC).Unix())
	if got["status"] != "snoozed" {
		t.Errorf("status=%v", got["status"])
	}
	if got["snoozed_until"] != want {
		t.Errorf("snoozed_until=%v want %v", got["snoozed_until"], want)
	}
}

func TestAddConversationLabelsWhitelist(t *testing.T) {
	h := &Handler{
		Guard:  guard.New([]string{"lead-caliente"}),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	res, _, err := h.addConversationLabels(context.Background(), &mcp.CallToolRequest{}, labelsIn{
		ConversationID: 1,
		Labels:         []string{"super-mega-lead"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected tool error")
	}
}
