package tools

import (
	"context"
	"errors"
	"fmt"
	"time"

	"chatwoot-mcp/internal/chatwoot"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type sendMessageIn struct {
	ConversationID    int            `json:"conversation_id" jsonschema:"ID numérico de la conversación"`
	Content           string         `json:"content" jsonschema:"Texto del mensaje. Soporta markdown básico según el canal."`
	Private           bool           `json:"private,omitempty" jsonschema:"true = nota interna invisible para el cliente"`
	ContentType       string         `json:"content_type,omitempty" jsonschema:"Deja text salvo que necesites un formato interactivo. El soporte varía por canal. Valores: text, input_select, cards, form, article, input_email"`
	ContentAttributes map[string]any `json:"content_attributes,omitempty" jsonschema:"Requerido cuando content_type no es text. Para input_select: items con title y value"`
}

func (h *Handler) sendMessage(ctx context.Context, _ *mcp.CallToolRequest, in sendMessageIn) (*mcp.CallToolResult, any, error) {
	start := time.Now()
	if in.ConversationID == 0 {
		return h.fail(errors.New("conversation_id es obligatorio"))
	}
	if in.Content == "" {
		return h.fail(errors.New("content es obligatorio"))
	}
	ct := in.ContentType
	if ct == "" {
		ct = "text"
	}
	allowed := []string{"text", "input_select", "cards", "form", "article", "input_email"}
	if !inList(ct, allowed) {
		return h.fail(fmt.Errorf("content_type inválido %q; usa text, input_select, cards, form, article o input_email", ct))
	}
	if ct != "text" && len(in.ContentAttributes) == 0 {
		return h.fail(errors.New("content_attributes es obligatorio cuando content_type no es text"))
	}
	req := chatwoot.SendMessageInput{
		Content:     in.Content,
		MessageType: "outgoing",
		Private:     in.Private,
		ContentType: ct,
	}
	if ct != "text" {
		req.ContentAttributes = in.ContentAttributes
	}
	msg, err := h.CW.SendMessage(ctx, in.ConversationID, req)
	h.logTool("send_message", in.ConversationID, 0, start, err)
	if err != nil {
		return h.fail(err)
	}
	return h.ok(projectMessage(*msg))
}

type getMessagesIn struct {
	ConversationID  int  `json:"conversation_id" jsonschema:"ID numérico de la conversación"`
	Before          int  `json:"before,omitempty" jsonschema:"Devuelve mensajes anteriores a este ID de mensaje"`
	After           int  `json:"after,omitempty" jsonschema:"Devuelve mensajes posteriores a este ID de mensaje"`
	IncludeActivity bool `json:"include_activity,omitempty" jsonschema:"Si true, incluye eventos de actividad del sistema"`
}

func (h *Handler) getMessages(ctx context.Context, _ *mcp.CallToolRequest, in getMessagesIn) (*mcp.CallToolResult, any, error) {
	start := time.Now()
	if in.ConversationID == 0 {
		return h.fail(errors.New("conversation_id es obligatorio"))
	}
	msgs, err := h.CW.GetMessages(ctx, in.ConversationID, in.Before, in.After)
	h.logTool("get_messages", in.ConversationID, 0, start, err)
	if err != nil {
		return h.fail(err)
	}
	sortMessages(msgs)
	out := make([]map[string]any, 0, len(msgs))
	for _, m := range msgs {
		role := MessageRole(int(m.MessageType))
		if role == "system" && !in.IncludeActivity {
			continue
		}
		out = append(out, projectMessage(m))
	}
	return h.ok(map[string]any{"messages": out})
}

type idConversationIn struct {
	ConversationID int `json:"conversation_id" jsonschema:"ID numérico de la conversación"`
}

func (h *Handler) getConversation(ctx context.Context, _ *mcp.CallToolRequest, in idConversationIn) (*mcp.CallToolResult, any, error) {
	start := time.Now()
	if in.ConversationID == 0 {
		return h.fail(errors.New("conversation_id es obligatorio"))
	}
	conv, err := h.CW.GetConversation(ctx, in.ConversationID)
	h.logTool("get_conversation", in.ConversationID, 0, start, err)
	if err != nil {
		return h.fail(err)
	}
	return h.ok(projectConversation(*conv))
}

type listConversationsIn struct {
	Status       string   `json:"status,omitempty" jsonschema:"Filtro de estado. Valores: all, open, resolved, pending, snoozed. Default open"`
	AssigneeType string   `json:"assignee_type,omitempty" jsonschema:"me = asignadas al usuario del bot; unassigned = sin dueño, candidatas a atender. Valores: me, unassigned, all, assigned"`
	Q            string   `json:"q,omitempty" jsonschema:"Busca conversaciones cuyos mensajes contengan este término"`
	InboxID      int      `json:"inbox_id,omitempty"`
	TeamID       int      `json:"team_id,omitempty"`
	Labels       []string `json:"labels,omitempty"`
	Page         int      `json:"page,omitempty" jsonschema:"Número de página. Páginas de 15 resultados. Default 1"`
}

func (h *Handler) listConversations(ctx context.Context, _ *mcp.CallToolRequest, in listConversationsIn) (*mcp.CallToolResult, any, error) {
	start := time.Now()
	status := in.Status
	if status == "" {
		status = "open"
	}
	if !inList(status, []string{"all", "open", "resolved", "pending", "snoozed"}) {
		return h.fail(fmt.Errorf("status inválido %q", status))
	}
	if in.AssigneeType != "" && !inList(in.AssigneeType, []string{"me", "unassigned", "all", "assigned"}) {
		return h.fail(fmt.Errorf("assignee_type inválido %q", in.AssigneeType))
	}
	page := pageSizeNote(in.Page)
	res, err := h.CW.ListConversations(ctx, chatwoot.ListConversationsQuery{
		Status:       status,
		AssigneeType: in.AssigneeType,
		Q:            in.Q,
		InboxID:      in.InboxID,
		TeamID:       in.TeamID,
		Labels:       in.Labels,
		Page:         page,
	})
	h.logTool("list_conversations", 0, 0, start, err)
	if err != nil {
		return h.fail(err)
	}
	items := make([]map[string]any, 0, len(res.Conversations))
	for _, c := range res.Conversations {
		items = append(items, projectConversation(c))
	}
	return h.ok(map[string]any{
		"page":          page,
		"page_size":     15,
		"meta":          res.Meta,
		"conversations": items,
	})
}

type getContactIn struct {
	ContactID int `json:"contact_id" jsonschema:"ID numérico del contacto"`
}

func (h *Handler) getContact(ctx context.Context, _ *mcp.CallToolRequest, in getContactIn) (*mcp.CallToolResult, any, error) {
	start := time.Now()
	if in.ContactID == 0 {
		return h.fail(errors.New("contact_id es obligatorio"))
	}
	c, err := h.CW.GetContact(ctx, in.ContactID)
	h.logTool("get_contact", 0, in.ContactID, start, err)
	if err != nil {
		return h.fail(err)
	}
	return h.ok(projectContact(*c))
}

type updateStatusIn struct {
	ConversationID int    `json:"conversation_id" jsonschema:"ID numérico de la conversación"`
	Status         string `json:"status" jsonschema:"Valores: open, resolved, pending, snoozed"`
	SnoozeUntilISO string `json:"snooze_until_iso,omitempty" jsonschema:"Solo con status snoozed. Fecha y hora ISO 8601, ej: 2026-09-08T14:00:00Z"`
}

func (h *Handler) updateConversationStatus(ctx context.Context, _ *mcp.CallToolRequest, in updateStatusIn) (*mcp.CallToolResult, any, error) {
	start := time.Now()
	if in.ConversationID == 0 {
		return h.fail(errors.New("conversation_id es obligatorio"))
	}
	if !inList(in.Status, []string{"open", "resolved", "pending", "snoozed"}) {
		return h.fail(fmt.Errorf("status inválido %q; usa open, resolved, pending o snoozed", in.Status))
	}
	var until int64
	if in.SnoozeUntilISO != "" {
		if in.Status != "snoozed" {
			return h.fail(errors.New("snooze_until_iso solo aplica con status snoozed"))
		}
		n, err := ParseFutureUnix(in.SnoozeUntilISO, h.now())
		if err != nil {
			return h.fail(err)
		}
		until = n
	}
	conv, err := h.CW.ToggleStatus(ctx, in.ConversationID, in.Status, until)
	h.logTool("update_conversation_status", in.ConversationID, 0, start, err)
	if err != nil {
		return h.fail(err)
	}
	return h.ok(projectConversation(*conv))
}

type typingIn struct {
	ConversationID int    `json:"conversation_id" jsonschema:"ID numérico de la conversación"`
	TypingStatus   string `json:"typing_status" jsonschema:"Valores: on, off"`
	IsPrivate      bool   `json:"is_private,omitempty" jsonschema:"true si el indicador corresponde a la redacción de una nota interna"`
}

func (h *Handler) setTypingIndicator(ctx context.Context, _ *mcp.CallToolRequest, in typingIn) (*mcp.CallToolResult, any, error) {
	start := time.Now()
	if in.ConversationID == 0 {
		return h.fail(errors.New("conversation_id es obligatorio"))
	}
	if !inList(in.TypingStatus, []string{"on", "off"}) {
		return h.fail(fmt.Errorf("typing_status inválido %q; usa on u off", in.TypingStatus))
	}
	err := h.CW.ToggleTyping(ctx, in.ConversationID, in.TypingStatus, in.IsPrivate)
	h.logTool("set_typing_indicator", in.ConversationID, 0, start, err)
	if err != nil {
		return h.fail(err)
	}
	return h.ok(map[string]any{"ok": true})
}
