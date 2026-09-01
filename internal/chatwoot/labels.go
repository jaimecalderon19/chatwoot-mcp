package chatwoot

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func (c *Client) GetConversationLabels(ctx context.Context, conversationID int) ([]string, error) {
	data, err := c.get(ctx, fmt.Sprintf("/conversations/%d/labels", conversationID), nil)
	if err != nil {
		return nil, err
	}
	return decodeLabelList(data)
}

func (c *Client) SetConversationLabels(ctx context.Context, conversationID int, labels []string) ([]string, error) {
	data, err := c.post(ctx, fmt.Sprintf("/conversations/%d/labels", conversationID), map[string]any{
		"labels": labels,
	})
	if err != nil {
		return nil, err
	}
	out, err := decodeLabelList(data)
	if err != nil {
		return labels, nil
	}
	return out, nil
}

func (c *Client) GetContactLabels(ctx context.Context, contactID int) ([]string, error) {
	data, err := c.get(ctx, fmt.Sprintf("/contacts/%d/labels", contactID), nil)
	if err != nil {
		return nil, err
	}
	return decodeLabelList(data)
}

func (c *Client) SetContactLabels(ctx context.Context, contactID int, labels []string) ([]string, error) {
	data, err := c.post(ctx, fmt.Sprintf("/contacts/%d/labels", contactID), map[string]any{
		"labels": labels,
	})
	if err != nil {
		return nil, err
	}
	out, err := decodeLabelList(data)
	if err != nil {
		return labels, nil
	}
	return out, nil
}

func (c *Client) ListAccountLabels(ctx context.Context) ([]Label, error) {
	return cached(c, "account_labels", func() ([]Label, error) {
		data, err := c.get(ctx, "/labels", nil)
		if err != nil {
			return nil, err
		}
		raw := extractPayload(data)
		var list []Label
		if err := json.Unmarshal(raw, &list); err != nil {
			return nil, fmt.Errorf("decode labels: %w", err)
		}
		return list, nil
	})
}

func decodeLabelList(data []byte) ([]string, error) {
	raw := extractPayload(data)
	out := parseStringLabels(raw)
	if len(out) > 0 {
		return out, nil
	}
	var wrap struct {
		Payload json.RawMessage `json:"payload"`
		Labels  json.RawMessage `json:"labels"`
	}
	if err := json.Unmarshal(data, &wrap); err == nil {
		if ls := parseStringLabels(wrap.Payload); len(ls) > 0 {
			return ls, nil
		}
		if ls := parseStringLabels(wrap.Labels); len(ls) > 0 {
			return ls, nil
		}
	}
	if string(raw) == "[]" || len(raw) == 0 || string(raw) == "null" {
		return []string{}, nil
	}
	return out, nil
}

func MergeLabels(current, add []string) []string {
	set := make(map[string]struct{}, len(current)+len(add))
	ordered := make([]string, 0, len(current)+len(add))
	for _, l := range append(append([]string{}, current...), add...) {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		if _, dup := set[l]; dup {
			continue
		}
		set[l] = struct{}{}
		ordered = append(ordered, l)
	}
	return ordered
}

func SubtractLabels(current, remove []string) []string {
	drop := make(map[string]struct{}, len(remove))
	for _, l := range remove {
		l = strings.TrimSpace(l)
		if l != "" {
			drop[l] = struct{}{}
		}
	}
	out := make([]string, 0, len(current))
	seen := make(map[string]struct{}, len(current))
	for _, l := range current {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		if _, ok := drop[l]; ok {
			continue
		}
		if _, dup := seen[l]; dup {
			continue
		}
		seen[l] = struct{}{}
		out = append(out, l)
	}
	return out
}
