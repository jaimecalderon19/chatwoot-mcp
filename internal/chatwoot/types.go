package chatwoot

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

type FlexTime time.Time

func (t FlexTime) Time() time.Time { return time.Time(t) }

func (t FlexTime) IsZero() bool { return time.Time(t).IsZero() }

func (t FlexTime) RFC3339() string {
	tt := time.Time(t)
	if tt.IsZero() {
		return ""
	}
	return tt.UTC().Format(time.RFC3339)
}

func (t *FlexTime) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" || string(b) == `""` || string(b) == "0" {
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err == nil {
		i, err := n.Int64()
		if err == nil && i > 0 {
			if i > 1e12 {
				*t = FlexTime(time.UnixMilli(i).UTC())
			} else {
				*t = FlexTime(time.Unix(i, 0).UTC())
			}
			return nil
		}
		f, err := n.Float64()
		if err == nil && f > 0 {
			*t = FlexTime(time.Unix(int64(f), 0).UTC())
			return nil
		}
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return nil
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	layouts := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05.000Z07:00",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, s)
		if err == nil {
			*t = FlexTime(parsed.UTC())
			return nil
		}
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil && i > 0 {
		*t = FlexTime(time.Unix(i, 0).UTC())
	}
	return nil
}

type MessageType int

func (m *MessageType) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	var n int
	if err := json.Unmarshal(b, &n); err == nil {
		*m = MessageType(n)
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return nil
	}
	switch strings.ToLower(s) {
	case "incoming":
		*m = 0
	case "outgoing":
		*m = 1
	case "activity":
		*m = 2
	case "template":
		*m = 3
	}
	return nil
}

type NamedEntity struct {
	ID   int    `json:"id"`
	Name string `json:"name,omitempty"`
}

type Profile struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Contact struct {
	ID                   int            `json:"id"`
	Name                 string         `json:"name"`
	Email                string         `json:"email"`
	PhoneNumber          string         `json:"phone_number"`
	Identifier           string         `json:"identifier"`
	CustomAttributes     map[string]any `json:"custom_attributes"`
	AdditionalAttributes map[string]any `json:"additional_attributes"`
	Labels               []string       `json:"labels"`
	CreatedAt            FlexTime       `json:"created_at"`
	LastActivityAt       FlexTime       `json:"last_activity_at"`
	ContactInboxes       []ContactInbox `json:"contact_inboxes"`
	AvailabilityStatus   string         `json:"availability_status,omitempty"`
}

type ContactInbox struct {
	SourceID string `json:"source_id"`
	Inbox    *Inbox `json:"inbox,omitempty"`
	InboxID  int    `json:"inbox_id,omitempty"`
}

type Inbox struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	ChannelType string `json:"channel_type"`
	PhoneNumber string `json:"phone_number,omitempty"`
}

type Conversation struct {
	ID                   int            `json:"id"`
	InboxID              int            `json:"inbox_id"`
	Status               string         `json:"status"`
	Priority             string         `json:"priority"`
	Channel              string         `json:"channel"`
	Labels               []string       `json:"labels"`
	CustomAttributes     map[string]any `json:"custom_attributes"`
	AdditionalAttributes map[string]any `json:"additional_attributes"`
	UnreadCount          int            `json:"unread_count"`
	LastActivityAt       FlexTime       `json:"last_activity_at"`
	CreatedAt            FlexTime       `json:"created_at"`
	Assignee             *NamedEntity   `json:"assignee"`
	Team                 *NamedEntity   `json:"team"`
	Contact              *Contact       `json:"contact"`
}

type Message struct {
	ID          int            `json:"id"`
	Content     string         `json:"content"`
	MessageType MessageType    `json:"message_type"`
	Private     bool           `json:"private"`
	ContentType string         `json:"content_type"`
	CreatedAt   FlexTime       `json:"created_at"`
	Sender      *MessageSender `json:"sender"`
	Attachments []any          `json:"attachments"`
}

type MessageSender struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	AvailableName string `json:"available_name"`
	Type          string `json:"type"`
}

type Agent struct {
	ID                 int    `json:"id"`
	Name               string `json:"name"`
	Email              string `json:"email"`
	Role               string `json:"role"`
	AvailabilityStatus string `json:"availability_status"`
	Confirmed          bool   `json:"confirmed"`
}

type Team struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	AllowAutoAssign bool   `json:"allow_auto_assign"`
}

type Label struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Color       string `json:"color"`
}

type CustomAttributeDef struct {
	AttributeKey         string   `json:"attribute_key"`
	AttributeDisplayName string   `json:"attribute_display_name"`
	AttributeDisplayType any      `json:"attribute_display_type"`
	AttributeModel       any      `json:"attribute_model"`
	AttributeValues      []string `json:"attribute_values"`
}

type CannedResponse struct {
	ID        int    `json:"id"`
	ShortCode string `json:"short_code"`
	Content   string `json:"content"`
}

type FilterCondition struct {
	AttributeKey   string   `json:"attribute_key"`
	FilterOperator string   `json:"filter_operator"`
	Values         []string `json:"values"`
	QueryOperator  string   `json:"query_operator,omitempty"`
}

type ContactableInbox struct {
	SourceID string `json:"source_id"`
	Inbox    Inbox  `json:"inbox"`
}

type CreateContactResult struct {
	Contact      Contact
	ContactInbox *ContactInbox
}

type ConversationList struct {
	Conversations []Conversation
	Meta          map[string]any
}

type ContactList struct {
	Contacts []Contact
	Meta     map[string]any
}

type WhatsAppTemplate struct {
	Name       string `json:"name"`
	Language   string `json:"language,omitempty"`
	Status     string `json:"status,omitempty"`
	Category   string `json:"category,omitempty"`
	Components any    `json:"components,omitempty"`
	ID         any    `json:"id,omitempty"`
}

type CreateConversationInput struct {
	SourceID         string         `json:"source_id"`
	InboxID          int            `json:"inbox_id"`
	ContactID        int            `json:"contact_id,omitempty"`
	Status           string         `json:"status,omitempty"`
	AssigneeID       int            `json:"assignee_id,omitempty"`
	TeamID           int            `json:"team_id,omitempty"`
	CustomAttributes map[string]any `json:"custom_attributes,omitempty"`
	Message          *struct {
		Content string `json:"content"`
	} `json:"message,omitempty"`
}

type SendMessageInput struct {
	Content           string         `json:"content"`
	MessageType       string         `json:"message_type"`
	Private           bool           `json:"private,omitempty"`
	ContentType       string         `json:"content_type,omitempty"`
	ContentAttributes map[string]any `json:"content_attributes,omitempty"`
	TemplateParams    map[string]any `json:"template_params,omitempty"`
}
