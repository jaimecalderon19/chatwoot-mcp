package tools

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type setAttrsIn struct {
	ConversationID   int            `json:"conversation_id" jsonschema:"ID numérico de la conversación"`
	CustomAttributes map[string]any `json:"custom_attributes" jsonschema:"Pares clave-valor, ej etapa cotizacion, presupuesto 5000-10000"`
	Merge            *bool          `json:"merge,omitempty" jsonschema:"true conserva los atributos previos; false los reemplaza por completo. Default true"`
}

func (h *Handler) setConversationAttributes(ctx context.Context, _ *mcp.CallToolRequest, in setAttrsIn) (*mcp.CallToolResult, any, error) {
	start := time.Now()
	if in.ConversationID == 0 {
		return h.fail(errors.New("conversation_id es obligatorio"))
	}
	if in.CustomAttributes == nil {
		return h.fail(errors.New("custom_attributes es obligatorio"))
	}
	merge := true
	if in.Merge != nil {
		merge = *in.Merge
	}
	attrs := in.CustomAttributes
	if merge {
		conv, err := h.CW.GetConversation(ctx, in.ConversationID)
		if err != nil {
			h.logTool("set_conversation_attributes", in.ConversationID, 0, start, err)
			return h.fail(fmt.Errorf("no se pudieron leer atributos actuales: %w", err))
		}
		existing := conv.CustomAttributes
		if existing == nil {
			existing = map[string]any{}
		}
		merged := make(map[string]any, len(existing)+len(attrs))
		for k, v := range existing {
			merged[k] = v
		}
		for k, v := range attrs {
			merged[k] = v
		}
		attrs = merged
	}
	err := h.CW.SetCustomAttributes(ctx, in.ConversationID, attrs)
	h.logTool("set_conversation_attributes", in.ConversationID, 0, start, err)
	if err != nil {
		return h.fail(err)
	}
	return h.ok(map[string]any{"ok": true, "custom_attributes": attrs})
}

type removeAttrsIn struct {
	ConversationID int      `json:"conversation_id" jsonschema:"ID numérico de la conversación"`
	Keys           []string `json:"keys" jsonschema:"Lista de claves a eliminar, ej order_id"`
}

func (h *Handler) removeConversationAttributes(ctx context.Context, _ *mcp.CallToolRequest, in removeAttrsIn) (*mcp.CallToolResult, any, error) {
	start := time.Now()
	if in.ConversationID == 0 {
		return h.fail(errors.New("conversation_id es obligatorio"))
	}
	if len(in.Keys) == 0 {
		return h.fail(errors.New("keys es obligatorio"))
	}
	err := h.CW.DestroyCustomAttributes(ctx, in.ConversationID, in.Keys)
	h.logTool("remove_conversation_attributes", in.ConversationID, 0, start, err)
	if err != nil {
		return h.fail(err)
	}
	return h.ok(map[string]any{"ok": true, "removed": in.Keys})
}

type updateContactIn struct {
	ContactID        int            `json:"contact_id" jsonschema:"ID numérico del contacto"`
	Name             string         `json:"name,omitempty"`
	Email            string         `json:"email,omitempty"`
	PhoneNumber      string         `json:"phone_number,omitempty" jsonschema:"Formato E.164, ej +573001234567"`
	Identifier       string         `json:"identifier,omitempty" jsonschema:"ID del cliente en tu sistema externo (CRM, ERP)"`
	CustomAttributes map[string]any `json:"custom_attributes,omitempty"`
}

func (h *Handler) updateContact(ctx context.Context, _ *mcp.CallToolRequest, in updateContactIn) (*mcp.CallToolResult, any, error) {
	start := time.Now()
	if in.ContactID == 0 {
		return h.fail(errors.New("contact_id es obligatorio"))
	}
	if err := ValidateE164(in.PhoneNumber); err != nil {
		return h.fail(err)
	}
	body := map[string]any{}
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
	if len(body) == 0 {
		return h.fail(errors.New("indica al menos un campo a actualizar"))
	}
	c, err := h.CW.UpdateContact(ctx, in.ContactID, body)
	h.logTool("update_contact", 0, in.ContactID, start, err)
	if err != nil {
		return h.fail(err)
	}
	return h.ok(projectContact(*c))
}

type listAttrDefsIn struct {
	Model string `json:"model,omitempty" jsonschema:"Filtra por tipo de entidad. Valores: conversation_attribute, contact_attribute"`
}

func (h *Handler) listCustomAttributeDefinitions(ctx context.Context, _ *mcp.CallToolRequest, in listAttrDefsIn) (*mcp.CallToolResult, any, error) {
	start := time.Now()
	if in.Model != "" && !inList(in.Model, []string{"conversation_attribute", "contact_attribute"}) {
		return h.fail(fmt.Errorf("model inválido %q", in.Model))
	}
	defs, err := h.CW.ListCustomAttributeDefinitions(ctx)
	h.logTool("list_custom_attribute_definitions", 0, 0, start, err)
	if err != nil {
		return h.fail(err)
	}
	out := make([]map[string]any, 0, len(defs))
	for _, d := range defs {
		model := attrModelName(d.AttributeModel)
		if in.Model != "" && model != in.Model {
			continue
		}
		item := map[string]any{
			"attribute_key":          d.AttributeKey,
			"attribute_display_name": d.AttributeDisplayName,
			"attribute_display_type": d.AttributeDisplayType,
			"attribute_model":        model,
		}
		if len(d.AttributeValues) > 0 {
			item["attribute_values"] = d.AttributeValues
		}
		out = append(out, item)
	}
	return h.ok(map[string]any{"definitions": out})
}

func attrModelName(v any) string {
	switch t := v.(type) {
	case string:
		s := t
		if s == "0" || s == "conversation_attribute" {
			return "conversation_attribute"
		}
		if s == "1" || s == "contact_attribute" {
			return "contact_attribute"
		}
		return s
	case float64:
		if t == 0 {
			return "conversation_attribute"
		}
		if t == 1 {
			return "contact_attribute"
		}
	case int:
		if t == 0 {
			return "conversation_attribute"
		}
		if t == 1 {
			return "contact_attribute"
		}
	}
	return fmt.Sprint(v)
}

type emptyIn struct{}

func (h *Handler) listCannedResponses(ctx context.Context, _ *mcp.CallToolRequest, _ emptyIn) (*mcp.CallToolResult, any, error) {
	start := time.Now()
	list, err := h.CW.ListCannedResponses(ctx)
	h.logTool("list_canned_responses", 0, 0, start, err)
	if err != nil {
		return h.fail(err)
	}
	out := make([]map[string]any, 0, len(list))
	for _, r := range list {
		out = append(out, map[string]any{
			"id":         r.ID,
			"short_code": r.ShortCode,
			"content":    r.Content,
		})
	}
	return h.ok(map[string]any{"canned_responses": out})
}

type priorityIn struct {
	ConversationID int    `json:"conversation_id" jsonschema:"ID numérico de la conversación"`
	Priority       string `json:"priority" jsonschema:"Valores: urgent, high, medium, low, none"`
}

func (h *Handler) setConversationPriority(ctx context.Context, _ *mcp.CallToolRequest, in priorityIn) (*mcp.CallToolResult, any, error) {
	start := time.Now()
	if in.ConversationID == 0 {
		return h.fail(errors.New("conversation_id es obligatorio"))
	}
	if !inList(in.Priority, []string{"urgent", "high", "medium", "low", "none"}) {
		return h.fail(fmt.Errorf("priority inválida %q; usa urgent, high, medium, low o none", in.Priority))
	}
	err := h.CW.TogglePriority(ctx, in.ConversationID, in.Priority)
	h.logTool("set_conversation_priority", in.ConversationID, 0, start, err)
	if err != nil {
		return h.fail(err)
	}
	return h.ok(map[string]any{"ok": true, "priority": in.Priority})
}

type assignIn struct {
	ConversationID int `json:"conversation_id" jsonschema:"ID numérico de la conversación"`
	AssigneeID     int `json:"assignee_id,omitempty" jsonschema:"ID del agente. Si lo envías, team_id se ignora."`
	TeamID         int `json:"team_id,omitempty" jsonschema:"ID del equipo, para asignación por round-robin del equipo."`
}

func (h *Handler) assignConversation(ctx context.Context, _ *mcp.CallToolRequest, in assignIn) (*mcp.CallToolResult, any, error) {
	start := time.Now()
	if in.ConversationID == 0 {
		return h.fail(errors.New("conversation_id es obligatorio"))
	}
	if in.AssigneeID == 0 && in.TeamID == 0 {
		return h.fail(errors.New("indica assignee_id o team_id"))
	}
	err := h.CW.AssignConversation(ctx, in.ConversationID, in.AssigneeID, in.TeamID)
	h.logTool("assign_conversation", in.ConversationID, 0, start, err)
	if err != nil {
		return h.fail(err)
	}
	out := map[string]any{"ok": true}
	if in.AssigneeID != 0 {
		out["assignee_id"] = in.AssigneeID
	} else {
		out["team_id"] = in.TeamID
	}
	return h.ok(out)
}

func (h *Handler) listAgents(ctx context.Context, _ *mcp.CallToolRequest, _ emptyIn) (*mcp.CallToolResult, any, error) {
	start := time.Now()
	list, err := h.CW.ListAgents(ctx)
	h.logTool("list_agents", 0, 0, start, err)
	if err != nil {
		return h.fail(err)
	}
	out := make([]map[string]any, 0, len(list))
	for _, a := range list {
		out = append(out, map[string]any{
			"id":                  a.ID,
			"name":                a.Name,
			"email":               a.Email,
			"role":                a.Role,
			"availability_status": a.AvailabilityStatus,
			"confirmed":           a.Confirmed,
		})
	}
	return h.ok(map[string]any{"agents": out})
}

func (h *Handler) listTeams(ctx context.Context, _ *mcp.CallToolRequest, _ emptyIn) (*mcp.CallToolResult, any, error) {
	start := time.Now()
	list, err := h.CW.ListTeams(ctx)
	h.logTool("list_teams", 0, 0, start, err)
	if err != nil {
		return h.fail(err)
	}
	out := make([]map[string]any, 0, len(list))
	for _, t := range list {
		out = append(out, map[string]any{
			"id":                t.ID,
			"name":              t.Name,
			"description":       t.Description,
			"allow_auto_assign": t.AllowAutoAssign,
		})
	}
	return h.ok(map[string]any{"teams": out})
}
