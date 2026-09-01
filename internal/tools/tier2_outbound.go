package tools

import (
	"context"
	"errors"
	"fmt"
	"time"

	"chatwoot-mcp/internal/chatwoot"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type createContactIn struct {
	InboxID          int            `json:"inbox_id" jsonschema:"ID del inbox/canal. Usa list_inboxes."`
	Name             string         `json:"name,omitempty"`
	Email            string         `json:"email,omitempty"`
	PhoneNumber      string         `json:"phone_number,omitempty" jsonschema:"Formato E.164"`
	Identifier       string         `json:"identifier,omitempty"`
	CustomAttributes map[string]any `json:"custom_attributes,omitempty"`
}

func (h *Handler) createContact(ctx context.Context, _ *mcp.CallToolRequest, in createContactIn) (*mcp.CallToolResult, any, error) {
	start := time.Now()
	if in.InboxID == 0 {
		return h.fail(errors.New("inbox_id es obligatorio"))
	}
	if err := ValidateE164(in.PhoneNumber); err != nil {
		return h.fail(err)
	}
	body := map[string]any{"inbox_id": in.InboxID}
	if in.Name != "" {
		body["name"] = in.Name
	}
	if in.Email != "" {
		body["email"] = in.Email
	}
	if in.PhoneNumber != "" {
		body["phone_number"] = in.PhoneNumber
	}
	if in.Identifier != "" {
		body["identifier"] = in.Identifier
	}
	if in.CustomAttributes != nil {
		body["custom_attributes"] = in.CustomAttributes
	}
	res, err := h.CW.CreateContact(ctx, body)
	contactID := 0
	if res != nil {
		contactID = res.Contact.ID
	}
	h.logTool("create_contact", 0, contactID, start, err)
	if err != nil {
		return h.fail(err)
	}
	out := projectContact(res.Contact)
	if res.ContactInbox != nil {
		out["source_id"] = res.ContactInbox.SourceID
		inboxID := res.ContactInbox.InboxID
		if inboxID == 0 && res.ContactInbox.Inbox != nil {
			inboxID = res.ContactInbox.Inbox.ID
		}
		if inboxID != 0 {
			out["inbox_id"] = inboxID
		}
	}
	return h.ok(out)
}

func (h *Handler) listInboxes(ctx context.Context, _ *mcp.CallToolRequest, _ emptyIn) (*mcp.CallToolResult, any, error) {
	start := time.Now()
	list, err := h.CW.ListInboxes(ctx)
	h.logTool("list_inboxes", 0, 0, start, err)
	if err != nil {
		return h.fail(err)
	}
	out := make([]map[string]any, 0, len(list))
	for _, in := range list {
		item := map[string]any{
			"id":           in.ID,
			"name":         in.Name,
			"channel_type": in.ChannelType,
			"channel":      chatwoot.ChannelKind(in.ChannelType),
			"api_create":   chatwoot.AllowsAPICreate(in.ChannelType),
		}
		if in.PhoneNumber != "" {
			item["phone_number"] = in.PhoneNumber
		}
		out = append(out, item)
	}
	return h.ok(map[string]any{"inboxes": out})
}

type resolveSourceIn struct {
	ContactID int    `json:"contact_id" jsonschema:"ID numérico del contacto"`
	InboxID   int    `json:"inbox_id" jsonschema:"ID del inbox/canal"`
	SourceID  string `json:"source_id,omitempty" jsonschema:"Opcional. Para WhatsApp/SMS suele ser el teléfono del contacto"`
}

func (h *Handler) resolveContactSourceID(ctx context.Context, _ *mcp.CallToolRequest, in resolveSourceIn) (*mcp.CallToolResult, any, error) {
	start := time.Now()
	if in.ContactID == 0 || in.InboxID == 0 {
		return h.fail(errors.New("contact_id e inbox_id son obligatorios"))
	}
	list, err := h.CW.ContactableInboxes(ctx, in.ContactID)
	if err != nil {
		h.logTool("resolve_contact_source_id", 0, in.ContactID, start, err)
		return h.fail(err)
	}
	for _, item := range list {
		if item.Inbox.ID == in.InboxID && item.SourceID != "" {
			h.logTool("resolve_contact_source_id", 0, in.ContactID, start, nil)
			return h.ok(map[string]any{
				"contact_id": in.ContactID,
				"inbox_id":   in.InboxID,
				"source_id":  item.SourceID,
				"created":    false,
			})
		}
	}
	ci, err := h.CW.CreateContactInbox(ctx, in.ContactID, in.InboxID, in.SourceID)
	if err != nil {
		kind := h.inboxKind(ctx, in.InboxID)
		if apiErr, ok := err.(*chatwoot.APIError); ok && apiErr.Status == 422 {
			h.logTool("resolve_contact_source_id", 0, in.ContactID, start, err)
			return h.fail(fmt.Errorf(
				"El inbox %d es de tipo %s y no admite conversaciones iniciadas por API. Tipos válidos: Website, Phone, Api, Email.",
				in.InboxID, kind,
			))
		}
		h.logTool("resolve_contact_source_id", 0, in.ContactID, start, err)
		return h.fail(err)
	}
	h.logTool("resolve_contact_source_id", 0, in.ContactID, start, nil)
	return h.ok(map[string]any{
		"contact_id": in.ContactID,
		"inbox_id":   in.InboxID,
		"source_id":  ci.SourceID,
		"created":    true,
	})
}

func (h *Handler) inboxKind(ctx context.Context, inboxID int) string {
	in, err := h.CW.GetInbox(ctx, inboxID)
	if err != nil || in == nil || in.ChannelType == "" {
		return "desconocido"
	}
	k := chatwoot.ChannelKind(in.ChannelType)
	if k == "" {
		return in.ChannelType
	}
	return k
}

type createConversationIn struct {
	SourceID         string         `json:"source_id" jsonschema:"Obtenido de resolve_contact_source_id"`
	InboxID          int            `json:"inbox_id" jsonschema:"Solo tipos Website, Phone, Api o Email"`
	ContactID        int            `json:"contact_id,omitempty"`
	Status           string         `json:"status,omitempty" jsonschema:"Valores: open, resolved, pending. Default open"`
	AssigneeID       int            `json:"assignee_id,omitempty"`
	TeamID           int            `json:"team_id,omitempty"`
	CustomAttributes map[string]any `json:"custom_attributes,omitempty" jsonschema:"Calificación inicial del lead"`
	Message          *struct {
		Content string `json:"content"`
	} `json:"message,omitempty" jsonschema:"Primer mensaje de la conversación"`
}

func (h *Handler) createConversation(ctx context.Context, _ *mcp.CallToolRequest, in createConversationIn) (*mcp.CallToolResult, any, error) {
	start := time.Now()
	if in.SourceID == "" || in.InboxID == 0 {
		return h.fail(errors.New("source_id e inbox_id son obligatorios"))
	}
	status := in.Status
	if status == "" {
		status = "open"
	}
	if !inList(status, []string{"open", "resolved", "pending"}) {
		return h.fail(fmt.Errorf("status inválido %q", status))
	}
	req := chatwoot.CreateConversationInput{
		SourceID:         in.SourceID,
		InboxID:          in.InboxID,
		ContactID:        in.ContactID,
		Status:           status,
		AssigneeID:       in.AssigneeID,
		TeamID:           in.TeamID,
		CustomAttributes: in.CustomAttributes,
	}
	if in.Message != nil && in.Message.Content != "" {
		req.Message = &struct {
			Content string `json:"content"`
		}{Content: in.Message.Content}
	}
	conv, err := h.CW.CreateConversation(ctx, req)
	cid := 0
	if conv != nil {
		cid = conv.ID
	}
	h.logTool("create_conversation", cid, in.ContactID, start, err)
	if err != nil {
		return h.fail(err)
	}
	return h.ok(projectConversation(*conv))
}

type listTemplatesIn struct {
	InboxID int    `json:"inbox_id" jsonschema:"ID del inbox de WhatsApp"`
	Name    string `json:"name,omitempty" jsonschema:"Filtra por nombre de plantilla"`
}

func (h *Handler) listWhatsAppTemplates(ctx context.Context, _ *mcp.CallToolRequest, in listTemplatesIn) (*mcp.CallToolResult, any, error) {
	start := time.Now()
	if in.InboxID == 0 {
		return h.fail(errors.New("inbox_id es obligatorio"))
	}
	list, err := h.CW.ListWhatsAppTemplates(ctx, in.InboxID, in.Name)
	h.logTool("list_whatsapp_templates", 0, 0, start, err)
	if err != nil {
		return h.fail(err)
	}
	out := make([]map[string]any, 0, len(list))
	for _, t := range list {
		item := map[string]any{"name": t.Name}
		if t.Language != "" {
			item["language"] = t.Language
		}
		if t.Status != "" {
			item["status"] = t.Status
		}
		if t.Category != "" {
			item["category"] = t.Category
		}
		if t.ID != nil {
			item["id"] = t.ID
		}
		if t.Components != nil {
			item["components"] = t.Components
		}
		out = append(out, item)
	}
	return h.ok(map[string]any{"templates": out})
}

type sendTemplateIn struct {
	ConversationID  int               `json:"conversation_id" jsonschema:"ID numérico de la conversación"`
	TemplateName    string            `json:"template_name" jsonschema:"Nombre exacto de la plantilla aprobada en WhatsApp Business Manager"`
	Language        string            `json:"language" jsonschema:"Código BCP 47, ej: es, es_MX, en_US"`
	Category        string            `json:"category" jsonschema:"Valores: UTILITY, MARKETING, SHIPPING_UPDATE, TICKET_UPDATE, ISSUE_RESOLUTION"`
	BodyParams      map[string]string `json:"body_params" jsonschema:"Variables del cuerpo indexadas por posición: 1 Juan, 2 12345"`
	HeaderMediaURL  string            `json:"header_media_url,omitempty" jsonschema:"URL pública, solo para plantillas con header de media"`
	HeaderMediaType string            `json:"header_media_type,omitempty" jsonschema:"Valores: image, video, document"`
	RenderedContent string            `json:"rendered_content,omitempty" jsonschema:"Texto ya renderizado de la plantilla, con las variables sustituidas. Es lo que queda en el transcript."`
}

func (h *Handler) sendWhatsAppTemplate(ctx context.Context, _ *mcp.CallToolRequest, in sendTemplateIn) (*mcp.CallToolResult, any, error) {
	start := time.Now()
	if in.ConversationID == 0 || in.TemplateName == "" || in.Language == "" || in.Category == "" {
		return h.fail(errors.New("conversation_id, template_name, language y category son obligatorios"))
	}
	if in.BodyParams == nil {
		return h.fail(errors.New("body_params es obligatorio"))
	}
	cats := []string{"UTILITY", "MARKETING", "SHIPPING_UPDATE", "TICKET_UPDATE", "ISSUE_RESOLUTION"}
	if !inList(in.Category, cats) {
		return h.fail(fmt.Errorf("category inválida %q", in.Category))
	}
	if in.HeaderMediaType != "" && !inList(in.HeaderMediaType, []string{"image", "video", "document"}) {
		return h.fail(fmt.Errorf("header_media_type inválido %q", in.HeaderMediaType))
	}
	processed := map[string]any{"body": in.BodyParams}
	if in.HeaderMediaURL != "" {
		header := map[string]any{"media_url": in.HeaderMediaURL}
		if in.HeaderMediaType != "" {
			header["media_type"] = in.HeaderMediaType
		}
		processed["header"] = header
	}
	content := in.RenderedContent
	if content == "" {
		content = in.TemplateName
	}
	msg, err := h.CW.SendMessage(ctx, in.ConversationID, chatwoot.SendMessageInput{
		Content:     content,
		MessageType: "outgoing",
		TemplateParams: map[string]any{
			"name":             in.TemplateName,
			"category":         in.Category,
			"language":         in.Language,
			"processed_params": processed,
		},
	})
	h.logTool("send_whatsapp_template", in.ConversationID, 0, start, err)
	if err != nil {
		return h.fail(err)
	}
	return h.ok(projectMessage(*msg))
}
