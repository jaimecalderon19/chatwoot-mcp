package chatwoot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

func (c *Client) ListInboxes(ctx context.Context) ([]Inbox, error) {
	return cached(c, "inboxes", func() ([]Inbox, error) {
		data, err := c.get(ctx, "/inboxes", nil)
		if err != nil {
			return nil, err
		}
		raw := extractPayload(data)
		var list []Inbox
		if err := json.Unmarshal(raw, &list); err != nil {
			return nil, fmt.Errorf("decode inboxes: %w", err)
		}
		return list, nil
	})
}

func (c *Client) GetInbox(ctx context.Context, id int) (*Inbox, error) {
	data, err := c.get(ctx, fmt.Sprintf("/inboxes/%d", id), nil)
	if err != nil {
		return nil, err
	}
	var in Inbox
	if err := unmarshalPayload(data, &in); err != nil {
		return nil, err
	}
	return &in, nil
}

func (c *Client) ListWhatsAppTemplates(ctx context.Context, inboxID int, name string) ([]WhatsAppTemplate, error) {
	q := url.Values{}
	if name != "" {
		q.Set("name", name)
	}
	data, err := c.get(ctx, fmt.Sprintf("/inboxes/%d/message_templates", inboxID), q)
	if err != nil {
		return nil, err
	}
	raw := extractPayload(data)
	var list []WhatsAppTemplate
	if err := json.Unmarshal(raw, &list); err != nil {
		var wrap struct {
			Payload []WhatsAppTemplate `json:"payload"`
			Data    []WhatsAppTemplate `json:"data"`
		}
		if err2 := json.Unmarshal(data, &wrap); err2 != nil {
			return nil, fmt.Errorf("decode message templates: %w", err)
		}
		if len(wrap.Payload) > 0 {
			return wrap.Payload, nil
		}
		return wrap.Data, nil
	}
	if name != "" {
		filtered := make([]WhatsAppTemplate, 0, len(list))
		for _, t := range list {
			if t.Name == name {
				filtered = append(filtered, t)
			}
		}
		return filtered, nil
	}
	return list, nil
}
