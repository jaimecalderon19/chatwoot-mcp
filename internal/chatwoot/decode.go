package chatwoot

import (
	"encoding/json"
	"fmt"
	"strings"
)

func extractPayload(data []byte) json.RawMessage {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return data
	}
	if p, ok := top["payload"]; ok && len(p) > 0 && string(p) != "null" {
		return p
	}
	if d, ok := top["data"]; ok && len(d) > 0 && string(d) != "null" {
		var inner map[string]json.RawMessage
		if err := json.Unmarshal(d, &inner); err == nil {
			if p, ok := inner["payload"]; ok && len(p) > 0 && string(p) != "null" {
				return p
			}
		}
		return d
	}
	return data
}

func extractMeta(data []byte) map[string]any {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return map[string]any{}
	}
	if d, ok := top["data"]; ok {
		var inner map[string]json.RawMessage
		if err := json.Unmarshal(d, &inner); err == nil {
			if m, ok := inner["meta"]; ok {
				return unmarshalMap(m)
			}
		}
	}
	if m, ok := top["meta"]; ok {
		return unmarshalMap(m)
	}
	return map[string]any{}
}

func unmarshalMap(raw json.RawMessage) map[string]any {
	out := map[string]any{}
	_ = json.Unmarshal(raw, &out)
	if out == nil {
		out = map[string]any{}
	}
	return out
}

func unmarshalPayload[T any](data []byte, dest *T) error {
	raw := extractPayload(data)
	if err := json.Unmarshal(raw, dest); err == nil {
		return nil
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}
	return nil
}

func parseStringLabels(raw json.RawMessage) []string {
	out := []string{}
	if len(raw) == 0 || string(raw) == "null" {
		return out
	}
	var ss []string
	if err := json.Unmarshal(raw, &ss); err == nil {
		for _, s := range ss {
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	var objs []struct {
		Title string `json:"title"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal(raw, &objs); err == nil {
		for _, o := range objs {
			s := strings.TrimSpace(o.Title)
			if s == "" {
				s = strings.TrimSpace(o.Name)
			}
			if s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

func parseConversation(data []byte) (*Conversation, error) {
	raw := extractPayload(data)
	conv, err := decodeConversation(raw)
	if err != nil {
		conv, err = decodeConversation(data)
	}
	if err != nil {
		return nil, err
	}
	return conv, nil
}

func decodeConversation(raw json.RawMessage) (*Conversation, error) {
	var wrap struct {
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(raw, &wrap); err == nil && len(wrap.Payload) > 0 {
		var inner Conversation
		if err := json.Unmarshal(wrap.Payload, &inner); err == nil && inner.ID != 0 {
			fillConversationMeta(wrap.Payload, &inner)
			return &inner, nil
		}
	}

	var c Conversation
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, err
	}
	fillConversationMeta(raw, &c)
	return &c, nil
}

func fillConversationMeta(raw json.RawMessage, c *Conversation) {
	var aux struct {
		Labels json.RawMessage `json:"labels"`
		Meta   struct {
			Sender   json.RawMessage `json:"sender"`
			Assignee json.RawMessage `json:"assignee"`
			Team     json.RawMessage `json:"team"`
			Channel  string          `json:"channel"`
		} `json:"meta"`
		Contact json.RawMessage `json:"contact"`
	}
	_ = json.Unmarshal(raw, &aux)
	if labels := parseStringLabels(aux.Labels); len(labels) > 0 {
		c.Labels = labels
	}
	if c.Labels == nil {
		c.Labels = []string{}
	}
	if c.CustomAttributes == nil {
		c.CustomAttributes = map[string]any{}
	}
	if c.Assignee == nil {
		c.Assignee = namedFromRaw(aux.Meta.Assignee)
	}
	if c.Team == nil {
		c.Team = namedFromRaw(aux.Meta.Team)
	}
	if c.Contact == nil {
		if ct := contactFromRaw(aux.Meta.Sender); ct != nil {
			c.Contact = ct
		} else {
			c.Contact = contactFromRaw(aux.Contact)
		}
	}
	if c.Channel == "" && aux.Meta.Channel != "" {
		c.Channel = aux.Meta.Channel
	}
}

func namedFromRaw(raw json.RawMessage) *NamedEntity {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var n NamedEntity
	if err := json.Unmarshal(raw, &n); err != nil || n.ID == 0 && n.Name == "" {
		return nil
	}
	return &n
}

func contactFromRaw(raw json.RawMessage) *Contact {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var c Contact
	if err := json.Unmarshal(raw, &c); err != nil || c.ID == 0 && c.Name == "" && c.Email == "" {
		return nil
	}
	if c.CustomAttributes == nil {
		c.CustomAttributes = map[string]any{}
	}
	if c.Labels == nil {
		c.Labels = []string{}
	}
	return &c
}

func parseConversations(data []byte) ([]Conversation, map[string]any, error) {
	raw := extractPayload(data)
	meta := extractMeta(data)
	var list []json.RawMessage
	if err := json.Unmarshal(raw, &list); err != nil {
		var wrapped struct {
			Payload []json.RawMessage `json:"payload"`
		}
		if err2 := json.Unmarshal(raw, &wrapped); err2 != nil {
			return nil, meta, fmt.Errorf("decode conversations: %w", err)
		}
		list = wrapped.Payload
	}
	out := make([]Conversation, 0, len(list))
	for _, item := range list {
		c, err := decodeConversation(item)
		if err != nil || c == nil {
			continue
		}
		out = append(out, *c)
	}
	return out, meta, nil
}

func parseContacts(data []byte) ([]Contact, map[string]any, error) {
	raw := extractPayload(data)
	meta := extractMeta(data)
	var list []Contact
	if err := json.Unmarshal(raw, &list); err != nil {
		var wrapped struct {
			Payload []Contact `json:"payload"`
		}
		if err2 := json.Unmarshal(raw, &wrapped); err2 != nil {
			return nil, meta, fmt.Errorf("decode contacts: %w", err)
		}
		list = wrapped.Payload
	}
	for i := range list {
		if list[i].CustomAttributes == nil {
			list[i].CustomAttributes = map[string]any{}
		}
		if list[i].Labels == nil {
			list[i].Labels = []string{}
		}
	}
	return list, meta, nil
}

func parseMessages(data []byte) ([]Message, error) {
	raw := extractPayload(data)
	var msgs []Message
	if err := json.Unmarshal(raw, &msgs); err == nil {
		return msgs, nil
	}
	var obj struct {
		Payload  []Message `json:"payload"`
		Messages []Message `json:"messages"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil {
		if len(obj.Payload) > 0 {
			return obj.Payload, nil
		}
		if len(obj.Messages) > 0 {
			return obj.Messages, nil
		}
	}
	if err := json.Unmarshal(data, &obj); err == nil {
		if len(obj.Payload) > 0 {
			return obj.Payload, nil
		}
		return obj.Messages, nil
	}
	return []Message{}, nil
}

func ChannelKind(channelType string) string {
	s := strings.ToLower(strings.TrimSpace(channelType))
	s = strings.TrimPrefix(s, "channel::")
	switch {
	case strings.Contains(s, "web"):
		return "website"
	case s == "api":
		return "api"
	case strings.Contains(s, "email"):
		return "email"
	case strings.Contains(s, "sms"), strings.Contains(s, "twilio"), strings.Contains(s, "phone"):
		return "phone"
	case strings.Contains(s, "whatsapp"):
		return "whatsapp"
	default:
		if s == "" {
			return ""
		}
		return s
	}
}

func AllowsAPICreate(channelType string) bool {
	switch ChannelKind(channelType) {
	case "website", "phone", "api", "email":
		return true
	default:
		return false
	}
}
