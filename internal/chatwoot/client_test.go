package chatwoot

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"chatwoot-mcp/internal/config"
)

func testClient(t *testing.T, h http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	cfg := &config.Config{
		BaseURL:   srv.URL,
		APIToken:  "secret-token-never-log",
		AccountID: 1,
		Timeout:   2 * time.Second,
	}
	c := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), srv.Client())
	return c, srv
}

func TestGetProfile200(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/profile" {
			t.Errorf("path=%s", r.URL.Path)
		}
		if r.Header.Get("api_access_token") != "secret-token-never-log" {
			t.Error("missing token header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":42,"name":"IA Ventas","email":"ia-ventas@example.com"}`))
	})
	p, err := c.GetProfile(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != 42 || p.Email != "ia-ventas@example.com" {
		t.Fatalf("%+v", p)
	}
}

func TestMap401(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"description":"Invalid token"}`))
	})
	_, err := c.GetProfile(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "inválido o revocado") {
		t.Errorf("err=%v", err)
	}
}

func TestMap404(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{}`))
	})
	_, err := c.GetConversation(context.Background(), 99)
	if err == nil || !strings.Contains(err.Error(), "No existe el recurso") {
		t.Fatalf("err=%v", err)
	}
}

func TestMap422IncludesErrors(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(422)
		_, _ = w.Write([]byte(`{"description":"Validation failed","errors":[{"field":"phone_number","message":"invalid","code":"bad"}]}`))
	})
	_, err := c.UpdateContact(context.Background(), 1, map[string]any{"phone_number": "x"})
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "Payload rechazado") || !strings.Contains(msg, "phone_number") {
		t.Errorf("err=%s", msg)
	}
}

func TestGETRetriesOn429(t *testing.T) {
	var n atomic.Int32
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if n.Add(1) < 3 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(429)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"payload":[]}`))
	})
	_, err := c.ListAccountLabels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n.Load() != 3 {
		t.Errorf("attempts=%d", n.Load())
	}
}

func TestPOSTDoesNotRetry5xx(t *testing.T) {
	var n atomic.Int32
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		w.WriteHeader(503)
		_, _ = w.Write([]byte(`{"error":"down"}`))
	})
	_, err := c.SendMessage(context.Background(), 1, SendMessageInput{Content: "hola", MessageType: "outgoing"})
	if err == nil {
		t.Fatal("expected error")
	}
	if n.Load() != 1 {
		t.Errorf("POST retried: attempts=%d", n.Load())
	}
	if !strings.Contains(err.Error(), "transitorio") {
		t.Errorf("err=%v", err)
	}
}

func TestGetConversationProjectsWithoutMessages(t *testing.T) {
	body, err := os.ReadFile("testdata/conversation.json")
	if err != nil {
		t.Fatal(err)
	}
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	})
	conv, err := c.GetConversation(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if conv.ID != 10 || conv.Contact == nil || conv.Contact.Name != "Juan" {
		t.Fatalf("%+v", conv)
	}
	if conv.Assignee == nil || conv.Assignee.Name != "Ana" {
		t.Fatalf("assignee=%+v", conv.Assignee)
	}
}

func TestAddConversationLabelsDoesNotOverwrite(t *testing.T) {
	var posted []string
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/labels"):
			_, _ = w.Write([]byte(`{"payload":["existente","humano"]}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/labels"):
			var body struct {
				Labels []string `json:"labels"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			posted = body.Labels
			b, _ := json.Marshal(map[string]any{"payload": body.Labels})
			_, _ = w.Write(b)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	})
	current, err := c.GetConversationLabels(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	merged := MergeLabels(current, []string{"lead-caliente"})
	got, err := c.SetConversationLabels(context.Background(), 1, merged)
	if err != nil {
		t.Fatal(err)
	}
	if len(posted) != 3 {
		t.Fatalf("posted=%v", posted)
	}
	if posted[0] != "existente" || posted[1] != "humano" || posted[2] != "lead-caliente" {
		t.Fatalf("posted=%v", posted)
	}
	if len(got) != 3 {
		t.Fatalf("got=%v", got)
	}
}

func TestLogsNeverContainToken(t *testing.T) {
	var buf strings.Builder
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"id":1}`))
	}))
	t.Cleanup(srv.Close)
	cfg := &config.Config{BaseURL: srv.URL, APIToken: "SUPERSECRETTOKEN", AccountID: 1, Timeout: time.Second}
	c := New(cfg, logger, srv.Client())
	_, _ = c.GetProfile(context.Background())
	if strings.Contains(buf.String(), "SUPERSECRETTOKEN") {
		t.Fatalf("token leaked in logs: %s", buf.String())
	}
}

func TestListConversationsLabelsQuery(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()["labels[]"]
		if len(q) != 2 || q[0] != "a" || q[1] != "b" {
			t.Errorf("labels query=%v raw=%s", q, r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"meta":{"all_count":0},"payload":[]}}`))
	})
	_, err := c.ListConversations(context.Background(), ListConversationsQuery{
		Labels: []string{"a", "b"},
		Page:   1,
	})
	if err != nil {
		t.Fatal(err)
	}
}
