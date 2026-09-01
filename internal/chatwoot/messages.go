package chatwoot

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

func (c *Client) SendMessage(ctx context.Context, conversationID int, in SendMessageInput) (*Message, error) {
	if in.MessageType == "" {
		in.MessageType = "outgoing"
	}
	data, err := c.post(ctx, fmt.Sprintf("/conversations/%d/messages", conversationID), in)
	if err != nil {
		return nil, err
	}
	msgs, err := parseMessages(data)
	if err == nil && len(msgs) == 1 {
		return &msgs[0], nil
	}
	var msg Message
	if err := unmarshalPayload(data, &msg); err != nil {
		raw := extractPayload(data)
		if err2 := unmarshalPayload(raw, &msg); err2 != nil && msg.ID == 0 {
			return &Message{Content: in.Content, Private: in.Private, MessageType: 1}, nil
		}
	}
	return &msg, nil
}

func (c *Client) GetMessages(ctx context.Context, conversationID, before, after int) ([]Message, error) {
	q := url.Values{}
	if before != 0 {
		q.Set("before", strconv.Itoa(before))
	}
	if after != 0 {
		q.Set("after", strconv.Itoa(after))
	}
	data, err := c.get(ctx, fmt.Sprintf("/conversations/%d/messages", conversationID), q)
	if err != nil {
		return nil, err
	}
	return parseMessages(data)
}
