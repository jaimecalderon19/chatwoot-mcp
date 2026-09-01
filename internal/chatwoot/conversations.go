package chatwoot

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

func (c *Client) GetConversation(ctx context.Context, id int) (*Conversation, error) {
	data, err := c.get(ctx, fmt.Sprintf("/conversations/%d", id), nil)
	if err != nil {
		return nil, err
	}
	return parseConversation(data)
}

func (c *Client) ListConversations(ctx context.Context, q ListConversationsQuery) (*ConversationList, error) {
	vals := url.Values{}
	status := q.Status
	if status == "" {
		status = "open"
	}
	vals.Set("status", status)
	if q.AssigneeType != "" {
		vals.Set("assignee_type", q.AssigneeType)
	}
	if q.Q != "" {
		vals.Set("q", q.Q)
	}
	intQuery(vals, "inbox_id", q.InboxID)
	intQuery(vals, "team_id", q.TeamID)
	page := q.Page
	if page <= 0 {
		page = 1
	}
	vals.Set("page", strconv.Itoa(page))
	for _, l := range q.Labels {
		vals.Add("labels[]", l)
	}
	data, err := c.get(ctx, "/conversations", vals)
	if err != nil {
		return nil, err
	}
	list, meta, err := parseConversations(data)
	if err != nil {
		return nil, err
	}
	return &ConversationList{Conversations: list, Meta: meta}, nil
}

type ListConversationsQuery struct {
	Status       string
	AssigneeType string
	Q            string
	InboxID      int
	TeamID       int
	Labels       []string
	Page         int
}

func (c *Client) ToggleStatus(ctx context.Context, id int, status string, snoozedUntil int64) (*Conversation, error) {
	body := map[string]any{"status": status}
	if snoozedUntil > 0 {
		body["snoozed_until"] = snoozedUntil
	}
	data, err := c.post(ctx, fmt.Sprintf("/conversations/%d/toggle_status", id), body)
	if err != nil {
		return nil, err
	}
	conv, err := parseConversation(data)
	if err != nil {
		return &Conversation{ID: id, Status: status}, nil
	}
	return conv, nil
}

func (c *Client) ToggleTyping(ctx context.Context, id int, typingStatus string, isPrivate bool) error {
	body := map[string]any{
		"typing_status": typingStatus,
		"is_private":    isPrivate,
	}
	_, err := c.post(ctx, fmt.Sprintf("/conversations/%d/toggle_typing_status", id), body)
	return err
}

func (c *Client) TogglePriority(ctx context.Context, id int, priority string) error {
	_, err := c.post(ctx, fmt.Sprintf("/conversations/%d/toggle_priority", id), map[string]any{
		"priority": priority,
	})
	return err
}

func (c *Client) AssignConversation(ctx context.Context, id int, assigneeID, teamID int) error {
	body := map[string]any{}
	if assigneeID != 0 {
		body["assignee_id"] = assigneeID
	} else {
		body["team_id"] = teamID
	}
	_, err := c.post(ctx, fmt.Sprintf("/conversations/%d/assignments", id), body)
	return err
}

func (c *Client) SetCustomAttributes(ctx context.Context, id int, attrs map[string]any) error {
	_, err := c.post(ctx, fmt.Sprintf("/conversations/%d/custom_attributes", id), map[string]any{
		"custom_attributes": attrs,
	})
	return err
}

func (c *Client) DestroyCustomAttributes(ctx context.Context, id int, keys []string) error {
	_, err := c.post(ctx, fmt.Sprintf("/conversations/%d/destroy_custom_attributes", id), map[string]any{
		"custom_attributes": keys,
	})
	return err
}

func (c *Client) CreateConversation(ctx context.Context, in CreateConversationInput) (*Conversation, error) {
	data, err := c.post(ctx, "/conversations", in)
	if err != nil {
		return nil, err
	}
	return parseConversation(data)
}

func (c *Client) FilterConversations(ctx context.Context, payload []FilterCondition, page int) (*ConversationList, error) {
	if page <= 0 {
		page = 1
	}
	q := url.Values{}
	q.Set("page", strconv.Itoa(page))
	body := map[string]any{"payload": payload}
	data, err := c.postQuery(ctx, "/conversations/filter", q, body)
	if err != nil {
		return nil, err
	}
	list, meta, err := parseConversations(data)
	if err != nil {
		return nil, err
	}
	return &ConversationList{Conversations: list, Meta: meta}, nil
}

func (c *Client) ListContactConversations(ctx context.Context, contactID int) ([]Conversation, error) {
	data, err := c.get(ctx, fmt.Sprintf("/contacts/%d/conversations", contactID), nil)
	if err != nil {
		return nil, err
	}
	list, _, err := parseConversations(data)
	return list, err
}

func NormalizeFilter(payload []FilterCondition) []FilterCondition {
	if len(payload) == 0 {
		return payload
	}
	out := make([]FilterCondition, len(payload))
	copy(out, payload)
	last := len(out) - 1
	if out[last].QueryOperator != "" {
		out[last].QueryOperator = ""
	}
	return out
}
