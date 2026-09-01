package tools

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"time"

	"chatwoot-mcp/internal/chatwoot"
	"chatwoot-mcp/internal/guard"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	e164Re  = regexp.MustCompile(`^\+[1-9]\d{1,14}$`)
	emailRe = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	phoneRe = regexp.MustCompile(`\+[1-9]\d{7,14}`)
)

type Handler struct {
	CW     *chatwoot.Client
	Guard  *guard.Labels
	Logger *slog.Logger
	Now    func() time.Time
}

func (h *Handler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

func (h *Handler) ok(v any) (*mcp.CallToolResult, any, error) {
	text, err := compactJSON(v)
	if err != nil {
		return nil, nil, err
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, nil, nil
}

func (h *Handler) fail(err error) (*mcp.CallToolResult, any, error) {
	if err == nil {
		return h.ok(map[string]any{"ok": true})
	}
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
	}, nil, nil
}

func (h *Handler) logTool(name string, conversationID, contactID int, start time.Time, err error) {
	attrs := []any{
		"tool", name,
		"latency_ms", time.Since(start).Milliseconds(),
	}
	if conversationID != 0 {
		attrs = append(attrs, "conversation_id", conversationID)
	}
	if contactID != 0 {
		attrs = append(attrs, "contact_id", contactID)
	}
	if err != nil {
		attrs = append(attrs, "error", err.Error())
		h.Logger.Error("tool", attrs...)
		return
	}
	h.Logger.Info("tool", attrs...)
}

func compactJSON(v any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

func MessageRole(messageType int) string {
	switch messageType {
	case 0:
		return "customer"
	case 1:
		return "agent"
	case 2:
		return "system"
	case 3:
		return "template"
	default:
		return "unknown"
	}
}

func ParseFutureUnix(iso string, now time.Time) (int64, error) {
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		t, err = time.Parse(time.RFC3339Nano, iso)
	}
	if err != nil {
		return 0, fmt.Errorf("snooze_until_iso debe ser ISO 8601 (ej: 2026-09-08T14:00:00Z): %w", err)
	}
	if !t.After(now) {
		return 0, errors.New("snooze_until_iso debe ser una fecha futura")
	}
	return t.Unix(), nil
}

func ValidateE164(phone string) error {
	if phone == "" {
		return nil
	}
	if !e164Re.MatchString(phone) {
		return fmt.Errorf("phone_number debe estar en formato E.164 (ej: +573001234567), recibido %q", phone)
	}
	return nil
}

func Redact(s string) string {
	s = emailRe.ReplaceAllString(s, "***@***")
	s = phoneRe.ReplaceAllString(s, "+***")
	return s
}

func projectMessage(m chatwoot.Message) map[string]any {
	out := map[string]any{
		"id":         m.ID,
		"role":       MessageRole(int(m.MessageType)),
		"private":    m.Private,
		"content":    m.Content,
		"created_at": m.CreatedAt.RFC3339(),
	}
	if m.Sender != nil {
		out["sender"] = formatSender(m.Sender)
	}
	n := len(m.Attachments)
	if n > 0 {
		out["attachments"] = n
	} else {
		out["attachments"] = 0
	}
	return out
}

func formatSender(s *chatwoot.MessageSender) string {
	name := s.Name
	if name == "" {
		name = s.AvailableName
	}
	t := strings.ToLower(s.Type)
	if t == "agent_bot" || t == "agentbot" {
		if name == "" {
			return "bot"
		}
		return name + " (bot)"
	}
	return name
}

func projectConversation(c chatwoot.Conversation) map[string]any {
	labels := c.Labels
	if labels == nil {
		labels = []string{}
	}
	attrs := c.CustomAttributes
	if attrs == nil {
		attrs = map[string]any{}
	}
	out := map[string]any{
		"id":                c.ID,
		"status":            c.Status,
		"priority":          c.Priority,
		"inbox_id":          c.InboxID,
		"channel":           c.Channel,
		"assignee":          projectNamed(c.Assignee),
		"team":              projectNamed(c.Team),
		"labels":            labels,
		"custom_attributes": attrs,
		"unread_count":      c.UnreadCount,
	}
	if ts := c.LastActivityAt.RFC3339(); ts != "" {
		out["last_activity_at"] = ts
	}
	if ts := c.CreatedAt.RFC3339(); ts != "" {
		out["created_at"] = ts
	}
	if c.Contact != nil {
		out["contact"] = projectContactBrief(*c.Contact)
	} else {
		out["contact"] = nil
	}
	return out
}

func projectConversationBrief(c chatwoot.Conversation) map[string]any {
	labels := c.Labels
	if labels == nil {
		labels = []string{}
	}
	attrs := c.CustomAttributes
	if attrs == nil {
		attrs = map[string]any{}
	}
	out := map[string]any{
		"id":                c.ID,
		"status":            c.Status,
		"labels":            labels,
		"custom_attributes": attrs,
	}
	if ts := c.CreatedAt.RFC3339(); ts != "" {
		out["created_at"] = ts
	}
	if ts := c.LastActivityAt.RFC3339(); ts != "" {
		out["last_activity_at"] = ts
	}
	return out
}

func projectNamed(n *chatwoot.NamedEntity) any {
	if n == nil || (n.ID == 0 && n.Name == "") {
		return nil
	}
	return map[string]any{"id": n.ID, "name": n.Name}
}

func projectContact(c chatwoot.Contact) map[string]any {
	labels := c.Labels
	if labels == nil {
		labels = []string{}
	}
	attrs := c.CustomAttributes
	if attrs == nil {
		attrs = map[string]any{}
	}
	out := map[string]any{
		"id":                c.ID,
		"name":              c.Name,
		"email":             c.Email,
		"phone_number":      c.PhoneNumber,
		"identifier":        c.Identifier,
		"custom_attributes": attrs,
		"labels":            labels,
	}
	if ts := c.CreatedAt.RFC3339(); ts != "" {
		out["created_at"] = ts
	}
	if ts := c.LastActivityAt.RFC3339(); ts != "" {
		out["last_activity_at"] = ts
	}
	aa := c.AdditionalAttributes
	if city, ok := stringVal(aa, "city"); ok {
		out["city"] = city
	}
	if country, ok := stringVal(aa, "country"); ok {
		out["country"] = country
	} else if country, ok := stringVal(aa, "country_code"); ok {
		out["country"] = country
	}
	if referer, ok := stringVal(aa, "referer"); ok {
		out["referer"] = referer
	}
	return out
}

func projectContactBrief(c chatwoot.Contact) map[string]any {
	return map[string]any{
		"id":           c.ID,
		"name":         c.Name,
		"email":        c.Email,
		"phone_number": c.PhoneNumber,
	}
}

func stringVal(m map[string]any, key string) (string, bool) {
	if m == nil {
		return "", false
	}
	v, ok := m[key]
	if !ok || v == nil {
		return "", false
	}
	s := strings.TrimSpace(fmt.Sprint(v))
	if s == "" || s == "<nil>" {
		return "", false
	}
	return s, true
}

func sortMessages(msgs []chatwoot.Message) {
	sort.SliceStable(msgs, func(i, j int) bool {
		ti := msgs[i].CreatedAt.Time()
		tj := msgs[j].CreatedAt.Time()
		if !ti.Equal(tj) && !ti.IsZero() && !tj.IsZero() {
			return ti.Before(tj)
		}
		return msgs[i].ID < msgs[j].ID
	})
}

func inList(v string, allowed []string) bool {
	for _, a := range allowed {
		if v == a {
			return true
		}
	}
	return false
}

func emptySlice[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func pageSizeNote(page int) int {
	if page <= 0 {
		return 1
	}
	return page
}
