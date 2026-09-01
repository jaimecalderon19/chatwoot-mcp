package tools

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var ReadOnlyTools = []string{
	"get_messages",
	"get_conversation",
	"list_conversations",
	"get_contact",
	"get_conversation_labels",
	"get_contact_labels",
	"list_account_labels",
	"list_agents",
	"list_teams",
	"list_inboxes",
	"list_custom_attribute_definitions",
	"list_canned_responses",
	"list_contact_conversations",
	"search_contacts",
	"filter_contacts",
	"filter_conversations",
	"list_whatsapp_templates",
}

func Register(server *mcp.Server, h *Handler, readOnly bool) {
	registerRead(server, h)
	if !readOnly {
		registerWrite(server, h)
	}
}

func registerRead(server *mcp.Server, h *Handler) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_messages",
		Description: "Obtiene el historial de mensajes de una conversación, del más antiguo al más reciente. Incluye notas internas. Usa 'before' con el ID del mensaje más antiguo que ya tienes para paginar hacia atrás. Por defecto omite eventos de actividad del sistema.",
	}, h.getMessages)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_conversation",
		Description: "Obtiene el estado completo de una conversación: estado, prioridad, canal, agente asignado, etiquetas, atributos personalizados y datos del contacto. Llama esto al inicio de cada turno para saber en qué punto está el lead.",
	}, h.getConversation)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_conversations",
		Description: "Lista conversaciones con filtros. Páginas de 15 resultados; usa 'page' para avanzar.",
	}, h.listConversations)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_contact",
		Description: "Obtiene la ficha de un contacto: nombre, email, teléfono, identificador externo y atributos personalizados (donde vive la información del lead que persiste entre conversaciones). GET /contacts solo encuentra contactos resueltos (con identifier, email o teléfono).",
	}, h.getContact)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_conversation_labels",
		Description: "Lista las etiquetas actualmente aplicadas a una conversación.",
	}, h.getConversationLabels)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_contact_labels",
		Description: "Lista las etiquetas permanentes del contacto (describen al lead: cliente-recurrente, sector-retail). Distintas de las etiquetas de conversación, que describen este caso puntual.",
	}, h.getContactLabels)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_account_labels",
		Description: "Lista el catálogo de etiquetas configuradas en la cuenta. Consúltalo antes de etiquetar para usar solo etiquetas existentes.",
	}, h.listAccountLabels)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_agents",
		Description: "Lista los agentes humanos de la cuenta con su ID, nombre, rol y disponibilidad. Consúltalo antes de escalar para elegir a quién asignar. No asignes a un agente offline.",
	}, h.listAgents)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_teams",
		Description: "Lista los equipos de la cuenta. Escalar a un equipo suele ser mejor que a una persona concreta cuando no sabes quién está disponible.",
	}, h.listTeams)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_inboxes",
		Description: "Lista los canales (inboxes) de la cuenta con su tipo: website, whatsapp, email, api, etc. Consúltalo para saber por dónde puedes iniciar una conversación saliente. Solo Website, Phone, Api y Email admiten crear conversaciones vía API.",
	}, h.listInboxes)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_custom_attribute_definitions",
		Description: "Lista las definiciones de atributos personalizados configurados en la cuenta, con su clave, tipo de dato y si aplican a contacto o a conversación. Consúltalo antes de escribir atributos para usar las claves correctas y no inventar campos nuevos.",
	}, h.listCustomAttributeDefinitions)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_canned_responses",
		Description: "Lista las respuestas predefinidas de la cuenta. Úsalas para precios, condiciones comerciales y políticas: es texto aprobado por el equipo. Prefiere una canned response antes que improvisar cifras o compromisos.",
	}, h.listCannedResponses)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_contact_conversations",
		Description: "Lista todas las conversaciones anteriores de un contacto. Consúltalo SIEMPRE antes de hacer un pitch: te dice si ya es cliente, si ya recibió una cotización o si hubo una objeción previa.",
	}, h.listContactConversations)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_contacts",
		Description: "Busca contactos por término, principalmente por email. Solo encuentra contactos resueltos (con email, teléfono o identificador). Resultados paginados de 15 en 15. Para búsquedas por otros campos usa filter_contacts.",
	}, h.searchContacts)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "filter_contacts",
		Description: "Filtra contactos por condiciones combinadas. Útil para prospección: leads en etapa cotización sin actividad en 7 días. Resultados paginados de 15 en 15. El query_operator de la última condición se omite automáticamente.",
	}, h.filterContacts)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "filter_conversations",
		Description: "Filtra conversaciones por condiciones combinadas, incluyendo atributos personalizados. Úsalo para encontrar tu propia cartera: conversaciones con etapa=cotizacion y prioridad alta. Páginas de 15. El query_operator de la última condición se omite automáticamente.",
	}, h.filterConversations)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_whatsapp_templates",
		Description: "Lista las plantillas de WhatsApp aprobadas y cacheadas para un inbox de WhatsApp. En WhatsApp, fuera de la ventana de 24h solo puedes iniciar contacto con una plantilla aprobada. Consulta esto antes de send_whatsapp_template.",
	}, h.listWhatsAppTemplates)
}

func registerWrite(server *mcp.Server, h *Handler) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "send_message",
		Description: "Envía un mensaje en una conversación de Chatwoot. Usa private=true para dejar una nota interna que el cliente NO ve (ideal para registrar la calificación del lead, objeciones detectadas o el motivo de un escalamiento). Usa private=false para responder al cliente.",
	}, h.sendMessage)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_conversation_status",
		Description: "Cambia el estado de una conversación. resolved cierra el caso. pending la deja esperando acción humana. snoozed la posterga: si pasas snooze_until_iso reabre en esa fecha (y también si el cliente responde antes); si lo omites, reabre cuando el cliente responda. No hace falta un scheduler externo.",
	}, h.updateConversationStatus)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "set_typing_indicator",
		Description: "Muestra u oculta el indicador 'escribiendo...' al cliente. Actívalo antes de generar una respuesta larga para que la espera se sienta natural.",
	}, h.setTypingIndicator)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "set_conversation_attributes",
		Description: "Guarda o actualiza atributos personalizados en la conversación: presupuesto, producto de interés, etapa del pipeline, objeción detectada, fecha estimada de decisión. Por defecto hace merge con los atributos existentes. Consulta list_custom_attribute_definitions para conocer las claves válidas.",
	}, h.setConversationAttributes)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "remove_conversation_attributes",
		Description: "Elimina claves específicas de los atributos personalizados de la conversación.",
	}, h.removeConversationAttributes)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_contact",
		Description: "Actualiza los datos del contacto. Úsalo cuando el cliente comparte su nombre, email o teléfono, o para guardar información que debe persistir entre conversaciones (a diferencia de los atributos de conversación, que son de este caso puntual).",
	}, h.updateContact)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "add_conversation_labels",
		Description: "Agrega etiquetas a una conversación conservando las que ya tenía. Usa list_account_labels para ver las etiquetas disponibles; no inventes nuevas. Las etiquetas de conversación describen este caso puntual (cotizacion-enviada).",
	}, h.addConversationLabels)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "remove_conversation_labels",
		Description: "Quita etiquetas específicas de una conversación, conservando las demás.",
	}, h.removeConversationLabels)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "add_contact_labels",
		Description: "Agrega etiquetas permanentes al contacto conservando las que ya tenía. Las etiquetas de contacto describen al lead (cliente-recurrente, sector-retail), no este caso puntual. Usa list_account_labels; no inventes nuevas.",
	}, h.addContactLabels)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "set_conversation_priority",
		Description: "Ajusta la prioridad de la conversación. Súbela cuando detectes intención de compra clara, urgencia real o un cliente de alto valor. Valores: urgent, high, medium, low, none.",
	}, h.setConversationPriority)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "assign_conversation",
		Description: "Asigna la conversación a un agente humano o a un equipo. Úsalo para escalar cuando el lead está listo para cerrar, pide hablar con una persona, o la consulta excede tu alcance. Antes de escalar, deja una nota interna con send_message(private=true) resumiendo el contexto. Si envías assignee_id, team_id se ignora.",
	}, h.assignConversation)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_contact",
		Description: "Crea un contacto nuevo. Requiere inbox_id: usa list_inboxes para elegir el canal. Antes de crear, busca con search_contacts para no duplicar leads. La respuesta incluye source_id para create_conversation.",
	}, h.createContact)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "resolve_contact_source_id",
		Description: "Obtiene el source_id necesario para iniciar una conversación con un contacto en un canal específico. Si el contacto aún no está vinculado a ese canal, crea el vínculo. Llama esto antes de create_conversation. Flujo outbound: search_contacts → create_contact o resolve_contact_source_id → create_conversation.",
	}, h.resolveContactSourceID)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_conversation",
		Description: "Inicia una conversación saliente. Requiere source_id, que obtienes con resolve_contact_source_id (o create_contact). inbox_id solo tipos Website, Phone, Api o Email. Puedes incluir el primer mensaje en la misma llamada.",
	}, h.createConversation)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "send_whatsapp_template",
		Description: "Envía una plantilla de WhatsApp pre-aprobada. Usa list_whatsapp_templates para ver las disponibles y sus variables. Fuera de la ventana de 24h es la única forma de iniciar contacto por WhatsApp.",
	}, h.sendWhatsAppTemplate)
}
