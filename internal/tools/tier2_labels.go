package tools

import (
	"context"
	"errors"
	"fmt"
	"time"

	"chatwoot-mcp/internal/chatwoot"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (h *Handler) getConversationLabels(ctx context.Context, _ *mcp.CallToolRequest, in idConversationIn) (*mcp.CallToolResult, any, error) {
	start := time.Now()
	if in.ConversationID == 0 {
		return h.fail(errors.New("conversation_id es obligatorio"))
	}
	labels, err := h.CW.GetConversationLabels(ctx, in.ConversationID)
	h.logTool("get_conversation_labels", in.ConversationID, 0, start, err)
	if err != nil {
		return h.fail(err)
	}
	return h.ok(map[string]any{"labels": emptySlice(labels)})
}

type labelsIn struct {
	ConversationID int      `json:"conversation_id" jsonschema:"ID numérico de la conversación"`
	Labels         []string `json:"labels" jsonschema:"Lista de etiquetas"`
}

func (h *Handler) addConversationLabels(ctx context.Context, _ *mcp.CallToolRequest, in labelsIn) (*mcp.CallToolResult, any, error) {
	start := time.Now()
	if in.ConversationID == 0 {
		return h.fail(errors.New("conversation_id es obligatorio"))
	}
	if len(in.Labels) == 0 {
		return h.fail(errors.New("labels debe tener al menos un elemento"))
	}
	if err := h.Guard.Validate(in.Labels); err != nil {
		return h.fail(err)
	}
	current, err := h.CW.GetConversationLabels(ctx, in.ConversationID)
	if err != nil {
		h.logTool("add_conversation_labels", in.ConversationID, 0, start, err)
		return h.fail(fmt.Errorf("no se pudo leer etiquetas actuales: %w", err))
	}
	ordered := chatwoot.MergeLabels(current, in.Labels)
	out, err := h.CW.SetConversationLabels(ctx, in.ConversationID, ordered)
	h.logTool("add_conversation_labels", in.ConversationID, 0, start, err)
	if err != nil {
		return h.fail(err)
	}
	return h.ok(map[string]any{"labels": emptySlice(out)})
}

func (h *Handler) removeConversationLabels(ctx context.Context, _ *mcp.CallToolRequest, in labelsIn) (*mcp.CallToolResult, any, error) {
	start := time.Now()
	if in.ConversationID == 0 {
		return h.fail(errors.New("conversation_id es obligatorio"))
	}
	if len(in.Labels) == 0 {
		return h.fail(errors.New("labels debe tener al menos un elemento"))
	}
	current, err := h.CW.GetConversationLabels(ctx, in.ConversationID)
	if err != nil {
		h.logTool("remove_conversation_labels", in.ConversationID, 0, start, err)
		return h.fail(fmt.Errorf("no se pudo leer etiquetas actuales: %w", err))
	}
	rest := chatwoot.SubtractLabels(current, in.Labels)
	out, err := h.CW.SetConversationLabels(ctx, in.ConversationID, rest)
	h.logTool("remove_conversation_labels", in.ConversationID, 0, start, err)
	if err != nil {
		return h.fail(err)
	}
	return h.ok(map[string]any{"labels": emptySlice(out)})
}

func (h *Handler) listAccountLabels(ctx context.Context, _ *mcp.CallToolRequest, _ emptyIn) (*mcp.CallToolResult, any, error) {
	start := time.Now()
	list, err := h.CW.ListAccountLabels(ctx)
	h.logTool("list_account_labels", 0, 0, start, err)
	if err != nil {
		return h.fail(err)
	}
	out := make([]map[string]any, 0, len(list))
	for _, l := range list {
		out = append(out, map[string]any{
			"id":          l.ID,
			"title":       l.Title,
			"description": l.Description,
			"color":       l.Color,
		})
	}
	return h.ok(map[string]any{"labels": out})
}

func (h *Handler) getContactLabels(ctx context.Context, _ *mcp.CallToolRequest, in getContactIn) (*mcp.CallToolResult, any, error) {
	start := time.Now()
	if in.ContactID == 0 {
		return h.fail(errors.New("contact_id es obligatorio"))
	}
	labels, err := h.CW.GetContactLabels(ctx, in.ContactID)
	h.logTool("get_contact_labels", 0, in.ContactID, start, err)
	if err != nil {
		return h.fail(err)
	}
	return h.ok(map[string]any{"labels": emptySlice(labels)})
}

type contactLabelsIn struct {
	ContactID int      `json:"contact_id" jsonschema:"ID numérico del contacto"`
	Labels    []string `json:"labels" jsonschema:"Lista de etiquetas"`
}

func (h *Handler) addContactLabels(ctx context.Context, _ *mcp.CallToolRequest, in contactLabelsIn) (*mcp.CallToolResult, any, error) {
	start := time.Now()
	if in.ContactID == 0 {
		return h.fail(errors.New("contact_id es obligatorio"))
	}
	if len(in.Labels) == 0 {
		return h.fail(errors.New("labels debe tener al menos un elemento"))
	}
	if err := h.Guard.Validate(in.Labels); err != nil {
		return h.fail(err)
	}
	current, err := h.CW.GetContactLabels(ctx, in.ContactID)
	if err != nil {
		h.logTool("add_contact_labels", 0, in.ContactID, start, err)
		return h.fail(fmt.Errorf("no se pudo leer etiquetas actuales: %w", err))
	}
	ordered := chatwoot.MergeLabels(current, in.Labels)
	out, err := h.CW.SetContactLabels(ctx, in.ContactID, ordered)
	h.logTool("add_contact_labels", 0, in.ContactID, start, err)
	if err != nil {
		return h.fail(err)
	}
	return h.ok(map[string]any{"labels": emptySlice(out)})
}
