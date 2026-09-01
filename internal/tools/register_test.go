package tools

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"chatwoot-mcp/internal/guard"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestReadOnlyOmitsWriteTools(t *testing.T) {
	names := listToolNames(t, true)
	wantRead := map[string]bool{}
	for _, n := range ReadOnlyTools {
		wantRead[n] = true
	}
	for _, n := range names {
		if !wantRead[n] {
			t.Errorf("write tool registered in readonly: %s", n)
		}
	}
	for n := range wantRead {
		found := false
		for _, got := range names {
			if got == n {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing readonly tool %s", n)
		}
	}
}

func TestFullRegisterHas32Tools(t *testing.T) {
	names := listToolNames(t, false)
	if len(names) != 32 {
		t.Fatalf("got %d tools: %v", len(names), names)
	}
	forbidden := []string{"delete_contact", "merge_contacts", "create_webhook", "delete_message"}
	set := map[string]bool{}
	for _, n := range names {
		set[n] = true
	}
	for _, f := range forbidden {
		if set[f] {
			t.Errorf("destructive tool exposed: %s", f)
		}
	}
}

func listToolNames(t *testing.T, readOnly bool) []string {
	t.Helper()
	h := &Handler{
		Guard:  guard.New(nil),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "chatwoot", Version: "test"}, nil)
	Register(server, h, readOnly)

	ctx := context.Background()
	t1, t2 := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, t1, nil); err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "c", Version: "test"}, nil)
	session, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	var names []string
	for tinfo, err := range session.Tools(ctx, nil) {
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, tinfo.Name)
	}
	return names
}
