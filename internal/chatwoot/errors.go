package chatwoot

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type APIError struct {
	Status      int
	Message     string
	Description string
	RetryAfter  time.Duration
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	if e.Description != "" && !strings.Contains(e.Message, e.Description) {
		return e.Message + " " + e.Description
	}
	return e.Message
}

func mapError(status int, body []byte) *APIError {
	desc, extra := parseErrorBody(body)
	switch status {
	case 401:
		return &APIError{
			Status:      401,
			Message:     "Token de Chatwoot inválido o revocado. Es un problema de configuración del servidor, no reintentes.",
			Description: desc,
		}
	case 403:
		return &APIError{
			Status:      403,
			Message:     "El usuario del bot no tiene permiso para esta acción en Chatwoot.",
			Description: desc,
		}
	case 404:
		return &APIError{
			Status:      404,
			Message:     "No existe el recurso solicitado (verifica el ID de conversación o contacto).",
			Description: desc,
		}
	case 422:
		detail := extra
		if detail == "" {
			detail = desc
		}
		msg := "Payload rechazado por Chatwoot"
		if detail != "" {
			msg += ": " + detail
		}
		msg += ". Corrige los parámetros y reintenta."
		return &APIError{Status: 422, Message: msg, Description: desc}
	case 429:
		return &APIError{
			Status:      429,
			Message:     "Límite de peticiones alcanzado. Espera antes de reintentar.",
			Description: desc,
		}
	default:
		if status >= 500 {
			return &APIError{
				Status:      status,
				Message:     "Error del servidor de Chatwoot. Puede ser transitorio.",
				Description: desc,
			}
		}
		msg := fmt.Sprintf("Error HTTP %d de Chatwoot.", status)
		if desc != "" {
			msg += " " + desc
		}
		return &APIError{Status: status, Message: msg, Description: desc}
	}
}

func parseErrorBody(body []byte) (desc string, extra string) {
	body = bytesTrim(body)
	if len(body) == 0 {
		return "", ""
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		s := strings.TrimSpace(string(body))
		if len(s) > 300 {
			s = s[:300]
		}
		return s, ""
	}
	desc = firstString(m, "description", "error", "message")
	extra = formatErrors(m["errors"])
	return desc, extra
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func formatErrors(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case []any:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			switch e := item.(type) {
			case string:
				parts = append(parts, e)
			case map[string]any:
				field, _ := e["field"].(string)
				msg, _ := e["message"].(string)
				code, _ := e["code"].(string)
				var b strings.Builder
				if field != "" {
					b.WriteString(field)
					b.WriteString(": ")
				}
				b.WriteString(msg)
				if code != "" {
					b.WriteString(" (")
					b.WriteString(code)
					b.WriteString(")")
				}
				if s := b.String(); strings.TrimSpace(s) != "" && s != ": " {
					parts = append(parts, strings.TrimSpace(strings.TrimPrefix(s, ": ")))
				}
			}
		}
		return strings.Join(parts, "; ")
	case map[string]any:
		parts := make([]string, 0, len(t))
		for k, val := range t {
			parts = append(parts, fmt.Sprintf("%s: %v", k, val))
		}
		return strings.Join(parts, "; ")
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprint(t)
		}
		return string(b)
	}
}

func bytesTrim(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}
