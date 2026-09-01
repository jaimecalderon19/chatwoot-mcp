package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"chatwoot-mcp/internal/chatwoot"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (h *Handler) listContactConversations(ctx context.Context, _ *mcp.CallToolRequest, in getContactIn) (*mcp.CallToolResult, any, error) {
	start := time.Now()
	if in.ContactID == 0 {
		return h.fail(errors.New("contact_id es obligatorio"))
	}
	list, err := h.CW.ListContactConversations(ctx, in.ContactID)
	h.logTool("list_contact_conversations", 0, in.ContactID, start, err)
	if err != nil {
		return h.fail(err)
	}
	out := make([]map[string]any, 0, len(list))
	for _, c := range list {
		out = append(out, projectConversationBrief(c))
	}
	return h.ok(map[string]any{"conversations": out})
}

type searchContactsIn struct {
	Q    string `json:"q" jsonschema:"Término de búsqueda, principalmente email"`
	Sort string `json:"sort,omitempty" jsonschema:"El prefijo - invierte el orden. Valores: name, email, phone_number, last_activity_at, -name, -email, -phone_number, -last_activity_at"`
	Page int    `json:"page,omitempty" jsonschema:"Páginas de 15 resultados. Default 1"`
}

func (h *Handler) searchContacts(ctx context.Context, _ *mcp.CallToolRequest, in searchContactsIn) (*mcp.CallToolResult, any, error) {
	start := time.Now()
	if in.Q == "" {
		return h.fail(errors.New("q es obligatorio"))
	}
	allowedSort := []string{
		"", "name", "email", "phone_number", "last_activity_at",
		"-name", "-email", "-phone_number", "-last_activity_at",
	}
	if !inList(in.Sort, allowedSort) {
		return h.fail(fmt.Errorf("sort inválido %q", in.Sort))
	}
	page := pageSizeNote(in.Page)
	res, err := h.CW.SearchContacts(ctx, in.Q, in.Sort, page)
	h.logTool("search_contacts", 0, 0, start, err)
	if err != nil {
		return h.fail(err)
	}
	items := make([]map[string]any, 0, len(res.Contacts))
	for _, c := range res.Contacts {
		items = append(items, projectContact(c))
	}
	return h.ok(map[string]any{
		"page":      page,
		"page_size": 15,
		"meta":      res.Meta,
		"contacts":  items,
	})
}

type filterConditionIn struct {
	AttributeKey   string   `json:"attribute_key" jsonschema:"Nombre del atributo a filtrar, ej email, country_code, o una clave de custom_attributes"`
	FilterOperator string   `json:"filter_operator" jsonschema:"Valores: equal_to, not_equal_to, contains, does_not_contain"`
	Values         []string `json:"values"`
	QueryOperator  string   `json:"query_operator,omitempty" jsonschema:"Cómo se combina con la condición siguiente. Valores: AND, OR. Debe omitirse en la última condición."`
}

type filterIn struct {
	Payload []filterConditionIn `json:"payload" jsonschema:"Lista de condiciones"`
	Page    int                 `json:"page,omitempty" jsonschema:"Páginas de 15 resultados. Default 1"`
}

func (h *Handler) filterContacts(ctx context.Context, _ *mcp.CallToolRequest, in filterIn) (*mcp.CallToolResult, any, error) {
	start := time.Now()
	payload, err := h.normalizeFilter(in.Payload)
	if err != nil {
		return h.fail(err)
	}
	page := pageSizeNote(in.Page)
	res, err := h.CW.FilterContacts(ctx, payload, page)
	h.logTool("filter_contacts", 0, 0, start, err)
	if err != nil {
		return h.fail(err)
	}
	items := make([]map[string]any, 0, len(res.Contacts))
	for _, c := range res.Contacts {
		items = append(items, projectContact(c))
	}
	return h.ok(map[string]any{
		"page":      page,
		"page_size": 15,
		"meta":      res.Meta,
		"contacts":  items,
	})
}

func (h *Handler) filterConversations(ctx context.Context, _ *mcp.CallToolRequest, in filterIn) (*mcp.CallToolResult, any, error) {
	start := time.Now()
	payload, err := h.normalizeFilter(in.Payload)
	if err != nil {
		return h.fail(err)
	}
	page := pageSizeNote(in.Page)
	res, err := h.CW.FilterConversations(ctx, payload, page)
	h.logTool("filter_conversations", 0, 0, start, err)
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

func (h *Handler) normalizeFilter(in []filterConditionIn) ([]chatwoot.FilterCondition, error) {
	if len(in) == 0 {
		return nil, errors.New("payload debe tener al menos una condición")
	}
	ops := []string{"equal_to", "not_equal_to", "contains", "does_not_contain"}
	out := make([]chatwoot.FilterCondition, 0, len(in))
	for i, c := range in {
		if c.AttributeKey == "" {
			return nil, fmt.Errorf("payload[%d].attribute_key es obligatorio", i)
		}
		if !inList(c.FilterOperator, ops) {
			return nil, fmt.Errorf("payload[%d].filter_operator inválido %q", i, c.FilterOperator)
		}
		if c.QueryOperator != "" && !inList(c.QueryOperator, []string{"AND", "OR"}) {
			return nil, fmt.Errorf("payload[%d].query_operator inválido %q", i, c.QueryOperator)
		}
		out = append(out, chatwoot.FilterCondition{
			AttributeKey:   c.AttributeKey,
			FilterOperator: c.FilterOperator,
			Values:         emptySlice(c.Values),
			QueryOperator:  c.QueryOperator,
		})
	}
	if out[len(out)-1].QueryOperator != "" {
		h.Logger.Debug("filter", slog.String("msg", "query_operator de la última condición omitido"))
	}
	return chatwoot.NormalizeFilter(out), nil
}
