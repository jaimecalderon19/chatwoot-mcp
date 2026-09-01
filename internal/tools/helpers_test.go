package tools

import (
	"strings"
	"testing"
	"time"

	"chatwoot-mcp/internal/chatwoot"
)

func TestMessageRole(t *testing.T) {
	cases := map[int]string{
		0: "customer",
		1: "agent",
		2: "system",
		3: "template",
		9: "unknown",
	}
	for in, want := range cases {
		if got := MessageRole(in); got != want {
			t.Errorf("MessageRole(%d)=%q want %q", in, got, want)
		}
	}
}

func TestParseFutureUnix(t *testing.T) {
	now := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	got, err := ParseFutureUnix("2026-09-08T14:00:00Z", now)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 9, 8, 14, 0, 0, 0, time.UTC).Unix()
	if got != want {
		t.Errorf("got %d want %d", got, want)
	}
}

func TestParseFutureUnixPast(t *testing.T) {
	now := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	_, err := ParseFutureUnix("2026-08-01T14:00:00Z", now)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseFutureUnixBadFormat(t *testing.T) {
	_, err := ParseFutureUnix("martes", time.Now())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateE164(t *testing.T) {
	if err := ValidateE164("+573001234567"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateE164(""); err != nil {
		t.Fatal(err)
	}
	if err := ValidateE164("3001234567"); err == nil {
		t.Fatal("expected error")
	}
	if err := ValidateE164("+57 300"); err == nil {
		t.Fatal("expected error")
	}
}

func TestRedact(t *testing.T) {
	s := Redact("escribe a ana@empresa.com o al +573001234567")
	if strings.Contains(s, "ana@") || strings.Contains(s, "3001234567") {
		t.Errorf("not redacted: %s", s)
	}
}

func TestNormalizeFilterClearsLastOperator(t *testing.T) {
	in := []chatwoot.FilterCondition{
		{AttributeKey: "email", FilterOperator: "equal_to", Values: []string{"a@b.c"}, QueryOperator: "AND"},
		{AttributeKey: "country_code", FilterOperator: "equal_to", Values: []string{"CO"}, QueryOperator: "AND"},
	}
	out := chatwoot.NormalizeFilter(in)
	if out[0].QueryOperator != "AND" {
		t.Errorf("first operator=%q", out[0].QueryOperator)
	}
	if out[1].QueryOperator != "" {
		t.Errorf("last operator=%q", out[1].QueryOperator)
	}
}
