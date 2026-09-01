package chatwoot

import (
	"context"
	"encoding/json"
	"fmt"
)

func (c *Client) ListAgents(ctx context.Context) ([]Agent, error) {
	return cached(c, "agents", func() ([]Agent, error) {
		data, err := c.get(ctx, "/agents", nil)
		if err != nil {
			return nil, err
		}
		raw := extractPayload(data)
		var list []Agent
		if err := json.Unmarshal(raw, &list); err != nil {
			return nil, fmt.Errorf("decode agents: %w", err)
		}
		return list, nil
	})
}

func (c *Client) ListTeams(ctx context.Context) ([]Team, error) {
	return cached(c, "teams", func() ([]Team, error) {
		data, err := c.get(ctx, "/teams", nil)
		if err != nil {
			return nil, err
		}
		raw := extractPayload(data)
		var list []Team
		if err := json.Unmarshal(raw, &list); err != nil {
			return nil, fmt.Errorf("decode teams: %w", err)
		}
		return list, nil
	})
}

func (c *Client) ListCustomAttributeDefinitions(ctx context.Context) ([]CustomAttributeDef, error) {
	return cached(c, "custom_attribute_definitions", func() ([]CustomAttributeDef, error) {
		data, err := c.get(ctx, "/custom_attribute_definitions", nil)
		if err != nil {
			return nil, err
		}
		raw := extractPayload(data)
		var list []CustomAttributeDef
		if err := json.Unmarshal(raw, &list); err != nil {
			return nil, fmt.Errorf("decode custom_attribute_definitions: %w", err)
		}
		return list, nil
	})
}

func (c *Client) ListCannedResponses(ctx context.Context) ([]CannedResponse, error) {
	return cached(c, "canned_responses", func() ([]CannedResponse, error) {
		data, err := c.get(ctx, "/canned_responses", nil)
		if err != nil {
			return nil, err
		}
		raw := extractPayload(data)
		var list []CannedResponse
		if err := json.Unmarshal(raw, &list); err != nil {
			return nil, fmt.Errorf("decode canned_responses: %w", err)
		}
		return list, nil
	})
}
