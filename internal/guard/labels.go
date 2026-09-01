package guard

import (
	"fmt"
	"strings"
)

type Labels struct {
	allowed map[string]struct{}
	list    []string
}

func New(allowed []string) *Labels {
	if len(allowed) == 0 {
		return &Labels{}
	}
	m := make(map[string]struct{}, len(allowed))
	list := make([]string, 0, len(allowed))
	for _, l := range allowed {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		if _, ok := m[l]; ok {
			continue
		}
		m[l] = struct{}{}
		list = append(list, l)
	}
	if len(list) == 0 {
		return &Labels{}
	}
	return &Labels{allowed: m, list: list}
}

func (g *Labels) Validate(add []string) error {
	if g == nil || g.allowed == nil {
		return nil
	}
	var rejected []string
	for _, l := range add {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		if _, ok := g.allowed[l]; !ok {
			rejected = append(rejected, l)
		}
	}
	if len(rejected) == 0 {
		return nil
	}
	return fmt.Errorf(
		"Etiqueta %s no permitida. Etiquetas disponibles: %s.",
		quoteList(rejected),
		strings.Join(g.list, ", "),
	)
}

func quoteList(items []string) string {
	if len(items) == 1 {
		return "'" + items[0] + "'"
	}
	quoted := make([]string, 0, len(items))
	for _, i := range items {
		quoted = append(quoted, "'"+i+"'")
	}
	return strings.Join(quoted, ", ")
}
