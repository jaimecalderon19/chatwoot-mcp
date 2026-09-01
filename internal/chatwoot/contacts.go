package chatwoot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

func (c *Client) GetContact(ctx context.Context, id int) (*Contact, error) {
	data, err := c.get(ctx, fmt.Sprintf("/contacts/%d", id), nil)
	if err != nil {
		return nil, err
	}
	return parseContact(data)
}

func (c *Client) UpdateContact(ctx context.Context, id int, body map[string]any) (*Contact, error) {
	data, err := c.put(ctx, fmt.Sprintf("/contacts/%d", id), body)
	if err != nil {
		return nil, err
	}
	return parseContact(data)
}

func (c *Client) CreateContact(ctx context.Context, body map[string]any) (*CreateContactResult, error) {
	data, err := c.post(ctx, "/contacts", body)
	if err != nil {
		return nil, err
	}
	return parseCreateContact(data)
}

func (c *Client) SearchContacts(ctx context.Context, q, sort string, page int) (*ContactList, error) {
	vals := url.Values{}
	vals.Set("q", q)
	if sort != "" {
		vals.Set("sort", sort)
	}
	if page <= 0 {
		page = 1
	}
	vals.Set("page", strconv.Itoa(page))
	data, err := c.get(ctx, "/contacts/search", vals)
	if err != nil {
		return nil, err
	}
	list, meta, err := parseContacts(data)
	if err != nil {
		return nil, err
	}
	return &ContactList{Contacts: list, Meta: meta}, nil
}

func (c *Client) FilterContacts(ctx context.Context, payload []FilterCondition, page int) (*ContactList, error) {
	if page <= 0 {
		page = 1
	}
	q := url.Values{}
	q.Set("page", strconv.Itoa(page))
	data, err := c.postQuery(ctx, "/contacts/filter", q, map[string]any{"payload": payload})
	if err != nil {
		return nil, err
	}
	list, meta, err := parseContacts(data)
	if err != nil {
		return nil, err
	}
	return &ContactList{Contacts: list, Meta: meta}, nil
}

func (c *Client) ContactableInboxes(ctx context.Context, contactID int) ([]ContactableInbox, error) {
	data, err := c.get(ctx, fmt.Sprintf("/contacts/%d/contactable_inboxes", contactID), nil)
	if err != nil {
		return nil, err
	}
	raw := extractPayload(data)
	var list []ContactableInbox
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("decode contactable_inboxes: %w", err)
	}
	return list, nil
}

func (c *Client) CreateContactInbox(ctx context.Context, contactID, inboxID int, sourceID string) (*ContactInbox, error) {
	body := map[string]any{"inbox_id": inboxID}
	if sourceID != "" {
		body["source_id"] = sourceID
	}
	data, err := c.post(ctx, fmt.Sprintf("/contacts/%d/contact_inboxes", contactID), body)
	if err != nil {
		return nil, err
	}
	var ci ContactInbox
	if err := unmarshalPayload(data, &ci); err != nil {
		return nil, err
	}
	return &ci, nil
}

func parseContact(data []byte) (*Contact, error) {
	raw := extractPayload(data)
	var wrap struct {
		Contact *Contact `json:"contact"`
	}
	if err := json.Unmarshal(raw, &wrap); err == nil && wrap.Contact != nil && wrap.Contact.ID != 0 {
		normalizeContact(wrap.Contact)
		return wrap.Contact, nil
	}
	var c Contact
	if err := json.Unmarshal(raw, &c); err != nil {
		if err2 := json.Unmarshal(data, &c); err2 != nil {
			return nil, fmt.Errorf("decode contact: %w", err)
		}
	}
	normalizeContact(&c)
	return &c, nil
}

func parseCreateContact(data []byte) (*CreateContactResult, error) {
	raw := extractPayload(data)
	var wrap struct {
		Contact      *Contact      `json:"contact"`
		ContactInbox *ContactInbox `json:"contact_inbox"`
	}
	_ = json.Unmarshal(raw, &wrap)
	if wrap.Contact != nil {
		normalizeContact(wrap.Contact)
		return &CreateContactResult{Contact: *wrap.Contact, ContactInbox: wrap.ContactInbox}, nil
	}
	c, err := parseContact(data)
	if err != nil {
		return nil, err
	}
	res := &CreateContactResult{Contact: *c}
	if len(c.ContactInboxes) > 0 {
		ci := c.ContactInboxes[0]
		res.ContactInbox = &ci
	}
	return res, nil
}

func normalizeContact(c *Contact) {
	if c.CustomAttributes == nil {
		c.CustomAttributes = map[string]any{}
	}
	if c.AdditionalAttributes == nil {
		c.AdditionalAttributes = map[string]any{}
	}
	if c.Labels == nil {
		c.Labels = []string{}
	}
}
