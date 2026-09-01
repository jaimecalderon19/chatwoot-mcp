# Chatwoot MCP Server (Go) — Especificación de implementación

> **Objetivo**: construir un servidor MCP en **Go** que exponga la Application API de Chatwoot como herramientas consumibles por un agente de ventas de IA.
> **Alcance de esta spec**: Tier 1 (núcleo conversacional) + Tier 2 (calificación, segmentación, seguimiento, outbound). El Tier 3 (Help Center, reportes, merge de contactos) queda **explícitamente fuera de alcance**.

---

## 1. Contexto: la API de Chatwoot

### 1.1 Las tres familias de API

Chatwoot expone tres categorías de API. Elegir mal es el error más común y cuesta días.

| Familia | Para qué sirve | Autenticación | Disponibilidad |
|---|---|---|---|
| **Application API** | Interactuar con una cuenta desde la perspectiva de **agente/admin**. Automatizar flujos, herramientas internas, operaciones en bulk. | `access_token` de usuario (Profile Settings) | Cloud + Self-hosted |
| Client API | Construir tu propia interfaz de chat para el usuario final (widget custom, app móvil). | `inbox_identifier` + `contact_identifier` | Cloud + Self-hosted |
| Platform API | Administrar la instalación: usuarios, roles, cuentas. | `access_token` de un Platform App (Super Admin Console) | **Solo** Self-hosted / Managed Hosting |

**Este MCP usa exclusivamente la Application API.** El agente de ventas actúa como un agente más dentro del inbox: lee la conversación, responde, etiqueta, califica y escala. Ese es precisamente el rol para el que la Application API está diseñada.

### 1.2 Base URL y headers

```
Base:   {CHATWOOT_BASE_URL}/api/v1/accounts/{CHATWOOT_ACCOUNT_ID}
Header: api_access_token: {CHATWOOT_API_TOKEN}
        Content-Type: application/json
```

Los endpoints de reportes viven bajo `/api/v2/...`, pero no se usan en esta spec.

### 1.3 User token vs AgentBot token

Existen dos esquemas de seguridad sobre el mismo header `api_access_token`:

- **`userApiKey`**: token de usuario. Acceso según los permisos del usuario. Se obtiene en la página de perfil o vía rails console.
- **`agentBotApiKey`**: token de bot, mucho más restringido. Solo **7** endpoints lo aceptan:
  - `POST /conversations`
  - `PATCH /conversations/{id}`
  - `POST /conversations/{id}/toggle_status`
  - `POST /conversations/{id}/toggle_priority`
  - `POST /conversations/{id}/toggle_typing_status`
  - `POST /conversations/{id}/assignments`
  - `POST /conversations/{id}/messages`

Como este MCP necesita leer contactos, buscar historial, listar etiquetas y crear contactos, **se usa token de usuario**. Recomendación operativa: crear un usuario dedicado en Chatwoot (`ia-ventas@…`) con rol de Agente, no reutilizar el token de un humano. Así los mensajes salen atribuidos al bot y la auditoría es limpia.

### 1.4 Paginación

- `GET /contacts`, `GET /contacts/search`, `POST /contacts/filter`: **page size = 15**, parámetro `page`.
- `GET /conversations`: parámetro `page`.
- `GET /conversations/{id}/messages`: **no usa `page`**, usa cursores `before` y `after` (IDs de mensaje).

El MCP debe exponer `page` donde aplique y documentar en la descripción del tool que las páginas son de 15 elementos, para que el modelo sepa cuándo pedir la siguiente.

### 1.5 Advertencia oficial sobre la documentación

La propia documentación de Chatwoot advierte que puede estar desactualizada respecto al comportamiento real, y recomienda inspeccionar las requests que hace la UI de Chatwoot en la pestaña Network del navegador para ver el payload exacto que funciona.

**Implicación para este proyecto**: la fuente de verdad para los schemas es el spec OpenAPI de la rama `develop`, no las páginas HTML:

```
https://raw.githubusercontent.com/chatwoot/chatwoot/develop/swagger/tag_groups/application_swagger.json
```

Los schemas de esta spec fueron extraídos de ese archivo. Ante duda o error 422, verificar contra el swagger y, si sigue fallando, contra la Network tab.

### 1.6 Trampas conocidas (leer antes de codificar)

1. **Las etiquetas se sobrescriben.** `POST /conversations/{id}/labels` y `POST /contacts/{id}/labels` **reemplazan** la lista completa de etiquetas, no agregan. Si el agente hace POST con `["lead-caliente"]`, borra todas las etiquetas que puso el equipo humano. → Ver §6.1 para el patrón obligatorio de read-then-write.
2. **Crear una conversación requiere `source_id`, no `contact_id` a secas.** El modelo de datos intermedio es `contact_inbox`. → Ver §6.2 para la secuencia de resolución.
3. **`GET /contacts` solo devuelve contactos "resolved"**, es decir, los que tienen valor en `identifier`, `email` o `phone_number`. Los contactos anónimos del widget no aparecen. Documentarlo en la descripción del tool.
4. **`GET /contacts/search` soporta principalmente búsqueda por email.** Para buscar por otros campos hay que usar `POST /contacts/filter`.
5. **`message_type` es string en el request pero entero en la respuesta.** Al enviar: `"outgoing"` / `"incoming"`. Al leer: `0` = incoming, `1` = outgoing, `2` = activity, `3` = template. El MCP debe **normalizar la respuesta a string** antes de devolverla al modelo; si no, el LLM confundirá quién dijo qué.
6. **`private: true` convierte el mensaje en nota interna.** El cliente no la ve. Es la herramienta más valiosa y más ignorada para un agente de ventas.
7. **`snoozed_until` es un timestamp Unix en segundos** (número, no ISO 8601) en `toggle_status`. Ojo: en el payload de `POST /conversations` el mismo campo sí acepta formato ISO. Inconsistencia real de la API.
8. **Los `content_type` de mensaje soportados varían por canal.** `input_select` funciona en el widget web pero no en todos los canales. Consultar la página *Supported Features on Channels* de los docs antes de prometer quick replies en WhatsApp.

---

## 2. Configuración: variables de entorno

**Requisito duro**: toda la configuración se toma de variables de entorno. Nada hardcodeado, ningún token en flags de CLI (quedan en el historial del shell y en `ps`).

| Variable | Obligatoria | Ejemplo | Notas |
|---|---|---|---|
| `CHATWOOT_BASE_URL` | ✅ | `https://app.chatwoot.com` | Sin slash final. Normalizar con `strings.TrimRight(v, "/")`. Validar que sea URL absoluta con esquema `https` (permitir `http` solo si `CHATWOOT_ALLOW_INSECURE=true`, para desarrollo local). |
| `CHATWOOT_API_TOKEN` | ✅ | `xxxxxxxxxxxx` | Token de usuario. **Nunca** loguearlo, ni completo ni truncado. |
| `CHATWOOT_ACCOUNT_ID` | ✅ | `1` | Entero. Va en todos los paths. Se fija por env para que el modelo no pueda saltar de cuenta. |
| `CHATWOOT_TIMEOUT_SECONDS` | ❌ | `30` | Default `30`. |
| `CHATWOOT_ALLOWED_LABELS` | ❌ | `lead-caliente,cotizacion-enviada` | CSV. Si está definido, los tools de etiquetas rechazan cualquier etiqueta fuera de la lista. Ver §7. |
| `CHATWOOT_READONLY` | ❌ | `false` | Si `true`, solo se registran los tools de lectura. Útil para evaluar el agente sin riesgo. |
| `LOG_LEVEL` | ❌ | `info` | `debug` \| `info` \| `warn` \| `error`. Logs siempre a **stderr** (stdout está ocupado por el transporte stdio de MCP). |

### 2.1 Validación al arranque

El servidor debe hacer **fail-fast**: si falta una variable obligatoria o `CHATWOOT_ACCOUNT_ID` no parsea como entero, escribir el error a stderr y `os.Exit(1)`. No arrancar en estado degradado.

Además, al arrancar debe hacer una llamada de verificación a `GET /api/v1/profile`. Si devuelve 401, el token es inválido y hay que abortar con un mensaje claro (`token inválido o revocado`). Si devuelve 200, loguear a nivel info el `id` y `email` del usuario autenticado — así el operador confirma que está usando el usuario bot y no el suyo.

```go
type Config struct {
	BaseURL       string
	APIToken      string
	AccountID     int
	Timeout       time.Duration
	AllowedLabels []string // nil = sin restricción
	ReadOnly      bool
}

func LoadConfig() (*Config, error) {
	// os.Getenv + validación. Devolver error agregado con TODAS las
	// variables faltantes, no solo la primera: ahorra iteraciones al operador.
}
```

### 2.2 `.env.example`

```dotenv
CHATWOOT_BASE_URL=https://app.chatwoot.com
CHATWOOT_API_TOKEN=
CHATWOOT_ACCOUNT_ID=1
CHATWOOT_TIMEOUT_SECONDS=30
CHATWOOT_ALLOWED_LABELS=
CHATWOOT_READONLY=false
LOG_LEVEL=info
```

El binario **no** debe leer `.env` por sí mismo en producción (que lo inyecte el orquestador). Se puede permitir carga opcional de `.env` solo si `APP_ENV=development`.

---

## 3. Stack y estructura del proyecto

### 3.1 Dependencias

- **Go 1.22+** (o la versión estable actual).
- **SDK MCP**: usar el SDK oficial de Go, `github.com/modelcontextprotocol/go-sdk`. Alternativa comunitaria madura: `github.com/mark3labs/mcp-go`. Verificar la versión vigente al iniciar el proyecto y fijarla en `go.mod` — el ecosistema MCP se mueve rápido.
- **HTTP**: `net/http` de la stdlib. No hace falta cliente externo.
- **Logging**: `log/slog` de la stdlib, handler JSON hacia stderr.
- Sin ORM, sin base de datos. El servidor es stateless salvo un caché en memoria opcional (§3.3).

### 3.2 Layout

```
chatwoot-mcp/
├── cmd/
│   └── chatwoot-mcp/
│       └── main.go              # carga config, construye server, arranca stdio
├── internal/
│   ├── config/
│   │   └── config.go            # §2
│   ├── chatwoot/
│   │   ├── client.go            # cliente HTTP: do(), retries, mapeo de errores
│   │   ├── conversations.go     # llamadas de conversaciones
│   │   ├── messages.go
│   │   ├── contacts.go
│   │   ├── labels.go
│   │   ├── inboxes.go
│   │   ├── meta.go              # agents, teams, custom_attribute_definitions, canned_responses
│   │   └── types.go             # structs de request/response
│   ├── tools/
│   │   ├── register.go          # registro de todos los tools, respeta READONLY
│   │   ├── tier1_*.go
│   │   ├── tier2_*.go
│   │   └── helpers.go           # normalización, formateo de salida para el LLM
│   └── guard/
│       └── labels.go            # validación contra CHATWOOT_ALLOWED_LABELS
├── .env.example
├── Dockerfile
├── Makefile
├── go.mod
└── README.md
```

### 3.3 Caché en memoria (opcional pero recomendado)

Tres endpoints devuelven datos casi estáticos que el agente consultará constantemente: `list_account_labels`, `list_agents`, `list_teams`, `list_custom_attribute_definitions`. Cachear en memoria con TTL de 5 minutos reduce latencia y consumo de rate limit. Invalidación: solo por TTL, no hace falta nada más sofisticado.

### 3.4 Cliente HTTP: requisitos

- Timeout desde config, aplicado con `context.WithTimeout` por request.
- **Reintentos**: máximo 2 reintentos con backoff exponencial + jitter, **solo** para `429`, `502`, `503`, `504` y errores de red. **Nunca** reintentar `POST /messages` sobre un error 5xx sin idempotencia — un mensaje duplicado al cliente es peor que un fallo visible. Regla: reintentar solo métodos `GET`; para escrituras, fallar y devolver el error al modelo.
- Respetar el header `Retry-After` en 429 si está presente. Chatwoot self-hosted puede tener rate limiting configurado.
- **Mapeo de errores a mensajes útiles para el LLM**. Un error crudo `422 Unprocessable Entity` no le dice nada al modelo. Traducir:

| Status | Mensaje devuelto al modelo |
|---|---|
| 401 | `Token de Chatwoot inválido o revocado. Es un problema de configuración del servidor, no reintentes.` |
| 403 | `El usuario del bot no tiene permiso para esta acción en Chatwoot.` |
| 404 | `No existe el recurso solicitado (verifica el ID de conversación o contacto).` |
| 422 | `Payload rechazado por Chatwoot: {body.errors}. Corrige los parámetros y reintenta.` |
| 429 | `Límite de peticiones alcanzado. Espera antes de reintentar.` |
| 5xx | `Error del servidor de Chatwoot. Puede ser transitorio.` |

Devolver siempre el body de error parseado (`description` y `errors[].{field,message,code}`) cuando exista.

### 3.5 Formato de salida hacia el modelo

Regla general: **devolver JSON compacto, no el payload crudo de Chatwoot**. Las respuestas de Chatwoot traen decenas de campos irrelevantes (`agent_last_seen_at`, `contact_last_seen_at`, `additional_attributes` con datos del navegador, etc.) que queman contexto sin aportar.

Cada tool debe proyectar solo los campos que un agente de ventas necesita. Ejemplo para un mensaje:

```json
{"id":123,"role":"agent","private":false,"content":"...","created_at":"2026-09-01T10:00:00Z","sender":"Ana (bot)","attachments":1}
```

En lugar de los ~25 campos que devuelve la API. Esto es la diferencia entre un agente que funciona con 20 mensajes de historial y uno que se queda sin contexto en 5.

---

## 4. Tier 1 — Tools del núcleo (7)

Sin estas siete el agente no puede sostener una conversación.

---

### 4.1 `send_message`

`POST /api/v1/accounts/{account_id}/conversations/{conversation_id}/messages`

La herramienta central. Envía una respuesta al cliente o deja una nota interna.

```json
{
  "name": "send_message",
  "description": "Envía un mensaje en una conversación de Chatwoot. Usa private=true para dejar una nota interna que el cliente NO ve (ideal para registrar la calificación del lead, objeciones detectadas o el motivo de un escalamiento). Usa private=false para responder al cliente.",
  "inputSchema": {
    "type": "object",
    "required": ["conversation_id", "content"],
    "properties": {
      "conversation_id": {"type": "integer", "description": "ID numérico de la conversación"},
      "content": {"type": "string", "description": "Texto del mensaje. Soporta markdown básico según el canal."},
      "private": {"type": "boolean", "default": false, "description": "true = nota interna invisible para el cliente"},
      "content_type": {
        "type": "string",
        "enum": ["text", "input_select", "cards", "form", "article", "input_email"],
        "default": "text",
        "description": "Deja 'text' salvo que necesites un formato interactivo. El soporte varía por canal."
      },
      "content_attributes": {
        "type": "object",
        "description": "Requerido cuando content_type no es 'text'. Para input_select: {\"items\":[{\"title\":\"Sí\",\"value\":\"si\"}]}"
      }
    }
  }
}
```

Implementación: `message_type` se fija **siempre** a `"outgoing"` desde el código; no se expone al modelo. Un agente no tiene razón para inyectar mensajes `incoming` (falsificar mensajes del cliente), y exponerlo es un riesgo.

`campoaign_id` y adjuntos multipart quedan fuera de alcance en v1.

**Nota sobre WhatsApp**: el envío de plantillas usa el campo `template_params` de este mismo endpoint, pero se expone como tool separado (§5.20) para no sobrecargar el schema de `send_message`.

---

### 4.2 `get_messages`

`GET /api/v1/accounts/{account_id}/conversations/{conversation_id}/messages`

```json
{
  "name": "get_messages",
  "description": "Obtiene el historial de mensajes de una conversación, del más antiguo al más reciente. Incluye notas internas. Usa 'before' con el ID del mensaje más antiguo que ya tienes para paginar hacia atrás.",
  "inputSchema": {
    "type": "object",
    "required": ["conversation_id"],
    "properties": {
      "conversation_id": {"type": "integer"},
      "before": {"type": "integer", "description": "Devuelve mensajes anteriores a este ID de mensaje"},
      "after": {"type": "integer", "description": "Devuelve mensajes posteriores a este ID de mensaje"}
    }
  }
}
```

**Transformación obligatoria en la respuesta**: mapear el `message_type` entero a un `role` legible.

| `message_type` | `role` a devolver |
|---|---|
| `0` | `customer` |
| `1` | `agent` |
| `2` | `system` (evento de actividad: asignación, cambio de estado) |
| `3` | `template` |

Filtrar los mensajes con `role: "system"` por defecto salvo que se pase `include_activity: true`, porque son ruido puro para el modelo (`"Conversation was marked resolved by Ana"`).

---

### 4.3 `get_conversation`

`GET /api/v1/accounts/{account_id}/conversations/{conversation_id}`

```json
{
  "name": "get_conversation",
  "description": "Obtiene el estado completo de una conversación: estado, prioridad, canal, agente asignado, etiquetas, atributos personalizados y datos del contacto. Llama esto al inicio de cada turno para saber en qué punto está el lead.",
  "inputSchema": {
    "type": "object",
    "required": ["conversation_id"],
    "properties": {"conversation_id": {"type": "integer"}}
  }
}
```

Proyección recomendada: `id`, `status`, `priority`, `inbox_id`, `channel`, `assignee` (`{id,name}` o `null`), `team` (o `null`), `labels[]`, `custom_attributes`, `contact` (`{id,name,email,phone_number}`), `last_activity_at`, `unread_count`. **Excluir** el array `messages` completo que viene embebido: para eso está `get_messages` con paginación.

---

### 4.4 `list_conversations`

`GET /api/v1/accounts/{account_id}/conversations`

```json
{
  "name": "list_conversations",
  "description": "Lista conversaciones con filtros. Página de resultados paginada; usa 'page' para avanzar.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "status": {"type": "string", "enum": ["all", "open", "resolved", "pending", "snoozed"], "default": "open"},
      "assignee_type": {"type": "string", "enum": ["me", "unassigned", "all", "assigned"], "description": "'me' = asignadas al usuario del bot; 'unassigned' = sin dueño, candidatas a atender"},
      "q": {"type": "string", "description": "Busca conversaciones cuyos mensajes contengan este término"},
      "inbox_id": {"type": "integer"},
      "team_id": {"type": "integer"},
      "labels": {"type": "array", "items": {"type": "string"}},
      "page": {"type": "integer", "default": 1}
    }
  }
}
```

`labels` va como array en query string. Serializar como `labels[]=a&labels[]=b`.

---

### 4.5 `get_contact`

`GET /api/v1/accounts/{account_id}/contacts/{id}`

```json
{
  "name": "get_contact",
  "description": "Obtiene la ficha de un contacto: nombre, email, teléfono, identificador externo y atributos personalizados (donde vive la información del lead que persiste entre conversaciones).",
  "inputSchema": {
    "type": "object",
    "required": ["contact_id"],
    "properties": {"contact_id": {"type": "integer"}}
  }
}
```

Proyección: `id`, `name`, `email`, `phone_number`, `identifier`, `custom_attributes`, `labels`, `created_at`, `last_activity_at`. Excluir `additional_attributes` completo (trae user agent, resolución de pantalla, etc.); si acaso, extraer solo `city`, `country` y `referer` cuando existan, que sí son útiles para vender.

---

### 4.6 `update_conversation_status`

`POST /api/v1/accounts/{account_id}/conversations/{conversation_id}/toggle_status`

```json
{
  "name": "update_conversation_status",
  "description": "Cambia el estado de una conversación. 'resolved' cierra el caso. 'pending' la deja esperando acción humana. 'snoozed' la posterga: si pasas snooze_until_iso reabre en esa fecha, y si lo omites reabre cuando el cliente responda.",
  "inputSchema": {
    "type": "object",
    "required": ["conversation_id", "status"],
    "properties": {
      "conversation_id": {"type": "integer"},
      "status": {"type": "string", "enum": ["open", "resolved", "pending", "snoozed"]},
      "snooze_until_iso": {"type": "string", "description": "Solo con status='snoozed'. Fecha y hora ISO 8601, ej: 2026-09-08T14:00:00Z"}
    }
  }
}
```

**Conversión obligatoria**: la API espera `snoozed_until` como **timestamp Unix en segundos** (número). El tool acepta ISO 8601 porque es lo que un LLM produce de forma fiable, y el servidor convierte. Validar que la fecha sea futura; si no, devolver error de validación sin llamar a la API.

Este es el mecanismo de follow-up nativo del agente de ventas: *"el cliente dijo que decide el martes"* → `snoozed` hasta el martes. No hace falta un scheduler externo.

---

### 4.7 `set_typing_indicator`

`POST /api/v1/accounts/{account_id}/conversations/{conversation_id}/toggle_typing_status`

```json
{
  "name": "set_typing_indicator",
  "description": "Muestra u oculta el indicador 'escribiendo...' al cliente. Actívalo antes de generar una respuesta larga para que la espera se sienta natural.",
  "inputSchema": {
    "type": "object",
    "required": ["conversation_id", "typing_status"],
    "properties": {
      "conversation_id": {"type": "integer"},
      "typing_status": {"type": "string", "enum": ["on", "off"]},
      "is_private": {"type": "boolean", "default": false, "description": "true si el indicador corresponde a la redacción de una nota interna"}
    }
  }
}
```

Devuelve 200 sin body. El tool debe responder algo como `{"ok":true}` — nunca una cadena vacía, que confunde a algunos clientes MCP.

---

## 5. Tier 2 — Tools de ventas (19)

### Calificación y CRM ligero

---

### 5.1 `set_conversation_attributes`

`POST /api/v1/accounts/{account_id}/conversations/{conversation_id}/custom_attributes`

El corazón de la calificación del lead.

```json
{
  "name": "set_conversation_attributes",
  "description": "Guarda o actualiza atributos personalizados en la conversación: presupuesto, producto de interés, etapa del pipeline, objeción detectada, fecha estimada de decisión. Por defecto hace merge con los atributos existentes. Consulta list_custom_attribute_definitions para conocer las claves válidas.",
  "inputSchema": {
    "type": "object",
    "required": ["conversation_id", "custom_attributes"],
    "properties": {
      "conversation_id": {"type": "integer"},
      "custom_attributes": {"type": "object", "description": "Pares clave-valor, ej: {\"etapa\":\"cotizacion\",\"presupuesto\":\"5000-10000\"}"},
      "merge": {"type": "boolean", "default": true, "description": "true conserva los atributos previos; false los reemplaza por completo"}
    }
  }
}
```

Forzar `merge: true` como default es importante: sin eso, el agente pisa la calificación acumulada en cada turno.

---

### 5.2 `remove_conversation_attributes`

`POST /api/v1/accounts/{account_id}/conversations/{conversation_id}/destroy_custom_attributes`

```json
{
  "name": "remove_conversation_attributes",
  "description": "Elimina claves específicas de los atributos personalizados de la conversación.",
  "inputSchema": {
    "type": "object",
    "required": ["conversation_id", "keys"],
    "properties": {
      "conversation_id": {"type": "integer"},
      "keys": {"type": "array", "items": {"type": "string"}, "description": "Lista de claves a eliminar, ej: [\"order_id\"]"}
    }
  }
}
```

El body de la API espera el campo `custom_attributes` como **array de strings** (no objeto). Detalle fácil de confundir con §5.1.

---

### 5.3 `update_contact`

`PUT /api/v1/accounts/{account_id}/contacts/{id}`

```json
{
  "name": "update_contact",
  "description": "Actualiza los datos del contacto. Úsalo cuando el cliente comparte su nombre, email o teléfono, o para guardar información que debe persistir entre conversaciones (a diferencia de los atributos de conversación, que son de este caso puntual).",
  "inputSchema": {
    "type": "object",
    "required": ["contact_id"],
    "properties": {
      "contact_id": {"type": "integer"},
      "name": {"type": "string"},
      "email": {"type": "string"},
      "phone_number": {"type": "string", "description": "Formato E.164, ej: +573001234567"},
      "identifier": {"type": "string", "description": "ID del cliente en tu sistema externo (CRM, ERP)"},
      "custom_attributes": {"type": "object"}
    }
  }
}
```

No exponer `blocked`, `avatar` ni `avatar_url`. Bloquear a un contacto no es decisión de un agente de IA.

Validar `phone_number` contra E.164 antes de enviar; Chatwoot lo rechaza con 422 si no cumple, y es un error frecuente cuando el modelo copia el número tal como lo escribió el cliente.

---

### 5.4 `list_custom_attribute_definitions`

`GET /api/v1/accounts/{account_id}/custom_attribute_definitions`

```json
{
  "name": "list_custom_attribute_definitions",
  "description": "Lista las definiciones de atributos personalizados configurados en la cuenta, con su clave, tipo de dato y si aplican a contacto o a conversación. Consúltalo antes de escribir atributos para usar las claves correctas y no inventar campos nuevos.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "model": {"type": "string", "enum": ["conversation_attribute", "contact_attribute"], "description": "Filtra por tipo de entidad"}
    }
  }
}
```

Cachear (§3.3). Proyección: `attribute_key`, `attribute_display_name`, `attribute_display_type`, `attribute_model`, `attribute_values` (para los de tipo lista).

---

### Segmentación y triage

---

### 5.5 `get_conversation_labels`

`GET /api/v1/accounts/{account_id}/conversations/{conversation_id}/labels`

```json
{
  "name": "get_conversation_labels",
  "description": "Lista las etiquetas actualmente aplicadas a una conversación.",
  "inputSchema": {
    "type": "object",
    "required": ["conversation_id"],
    "properties": {"conversation_id": {"type": "integer"}}
  }
}
```

---

### 5.6 `add_conversation_labels` ⚠️

`GET` + `POST /api/v1/accounts/{account_id}/conversations/{conversation_id}/labels`

**Este tool NO es un passthrough.** Implementa read-then-write. Ver §6.1.

```json
{
  "name": "add_conversation_labels",
  "description": "Agrega etiquetas a una conversación conservando las que ya tenía. Usa list_account_labels para ver las etiquetas disponibles; no inventes nuevas.",
  "inputSchema": {
    "type": "object",
    "required": ["conversation_id", "labels"],
    "properties": {
      "conversation_id": {"type": "integer"},
      "labels": {"type": "array", "items": {"type": "string"}, "minItems": 1}
    }
  }
}
```

---

### 5.7 `remove_conversation_labels`

Misma mecánica invertida: GET actuales → quitar las indicadas → POST el resto.

```json
{
  "name": "remove_conversation_labels",
  "description": "Quita etiquetas específicas de una conversación, conservando las demás.",
  "inputSchema": {
    "type": "object",
    "required": ["conversation_id", "labels"],
    "properties": {
      "conversation_id": {"type": "integer"},
      "labels": {"type": "array", "items": {"type": "string"}, "minItems": 1}
    }
  }
}
```

---

### 5.8 `list_account_labels`

`GET /api/v1/accounts/{account_id}/labels`

```json
{
  "name": "list_account_labels",
  "description": "Lista el catálogo de etiquetas configuradas en la cuenta. Consúltalo antes de etiquetar para usar solo etiquetas existentes.",
  "inputSchema": {"type": "object", "properties": {}}
}
```

Cachear. Proyección: `id`, `title`, `description`, `color`.

---

### 5.9 `get_contact_labels` / 5.10 `add_contact_labels`

`GET` y `POST /api/v1/accounts/{account_id}/contacts/{id}/labels`

Mismo patrón y **misma trampa de sobrescritura** que las de conversación. `add_contact_labels` también debe ser read-then-write.

Diferencia semántica que conviene documentar en la descripción del tool: las etiquetas de **contacto** describen al lead de forma permanente (`cliente-recurrente`, `sector-retail`); las de **conversación** describen este caso puntual (`cotizacion-enviada`).

---

### 5.11 `set_conversation_priority`

`POST /api/v1/accounts/{account_id}/conversations/{conversation_id}/toggle_priority`

```json
{
  "name": "set_conversation_priority",
  "description": "Ajusta la prioridad de la conversación. Súbela cuando detectes intención de compra clara, urgencia real o un cliente de alto valor.",
  "inputSchema": {
    "type": "object",
    "required": ["conversation_id", "priority"],
    "properties": {
      "conversation_id": {"type": "integer"},
      "priority": {"type": "string", "enum": ["urgent", "high", "medium", "low", "none"]}
    }
  }
}
```

Nota: `PATCH /conversations/{id}` también permite cambiar `priority` (y `sla_policy_id`, solo Enterprise). Usar el endpoint dedicado `toggle_priority`, que es más explícito, y no exponer el PATCH genérico.

---

### Seguimiento y handoff

---

### 5.12 `assign_conversation`

`POST /api/v1/accounts/{account_id}/conversations/{conversation_id}/assignments`

```json
{
  "name": "assign_conversation",
  "description": "Asigna la conversación a un agente humano o a un equipo. Úsalo para escalar cuando el lead está listo para cerrar, pide hablar con una persona, o la consulta excede tu alcance. Antes de escalar, deja una nota interna con send_message(private=true) resumiendo el contexto.",
  "inputSchema": {
    "type": "object",
    "required": ["conversation_id"],
    "properties": {
      "conversation_id": {"type": "integer"},
      "assignee_id": {"type": "integer", "description": "ID del agente. Si lo envías, team_id se ignora."},
      "team_id": {"type": "integer", "description": "ID del equipo, para asignación por round-robin del equipo."}
    }
  }
}
```

Validar en el servidor que venga al menos uno de los dos. Si `assignee_id` está presente, Chatwoot ignora `team_id` — documentarlo en la descripción para que el modelo no crea que puede hacer ambas.

Para desasignar, la API acepta `assignee_id: null`. Decidir si exponerlo; recomendación: **no**, un agente de IA no debería poder quitarle un caso a un humano.

---

### 5.13 `list_agents`

`GET /api/v1/accounts/{account_id}/agents`

```json
{
  "name": "list_agents",
  "description": "Lista los agentes humanos de la cuenta con su ID, nombre, rol y disponibilidad. Consúltalo antes de escalar para elegir a quién asignar.",
  "inputSchema": {"type": "object", "properties": {}}
}
```

Cachear. Proyección: `id`, `name`, `email`, `role`, `availability_status`, `confirmed`. El `availability_status` es clave: escalar a un agente offline es peor que dejar la conversación sin asignar.

---

### 5.14 `list_teams`

`GET /api/v1/accounts/{account_id}/teams`

```json
{
  "name": "list_teams",
  "description": "Lista los equipos de la cuenta. Escalar a un equipo suele ser mejor que a una persona concreta cuando no sabes quién está disponible.",
  "inputSchema": {"type": "object", "properties": {}}
}
```

Cachear. Proyección: `id`, `name`, `description`, `allow_auto_assign`.

---

### Contexto e histórico del lead

---

### 5.15 `list_contact_conversations`

`GET /api/v1/accounts/{account_id}/contacts/{id}/conversations`

```json
{
  "name": "list_contact_conversations",
  "description": "Lista todas las conversaciones anteriores de un contacto. Consúltalo SIEMPRE antes de hacer un pitch: te dice si ya es cliente, si ya recibió una cotización o si hubo una objeción previa.",
  "inputSchema": {
    "type": "object",
    "required": ["contact_id"],
    "properties": {"contact_id": {"type": "integer"}}
  }
}
```

Proyección mínima por conversación: `id`, `status`, `created_at`, `last_activity_at`, `labels`, `custom_attributes`. Sin mensajes.

---

### 5.16 `search_contacts`

`GET /api/v1/accounts/{account_id}/contacts/search`

```json
{
  "name": "search_contacts",
  "description": "Busca contactos por término, principalmente por email. Solo encuentra contactos 'resueltos' (con email, teléfono o identificador). Resultados paginados de 15 en 15. Para búsquedas por otros campos usa filter_contacts.",
  "inputSchema": {
    "type": "object",
    "required": ["q"],
    "properties": {
      "q": {"type": "string"},
      "sort": {"type": "string", "enum": ["name", "email", "phone_number", "last_activity_at", "-name", "-email", "-phone_number", "-last_activity_at"], "description": "El prefijo '-' invierte el orden"},
      "page": {"type": "integer", "default": 1}
    }
  }
}
```

---

### 5.17 `filter_contacts`

`POST /api/v1/accounts/{account_id}/contacts/filter`

El filtro potente. Sirve para prospección sobre la base existente.

```json
{
  "name": "filter_contacts",
  "description": "Filtra contactos por condiciones combinadas. Útil para prospección: 'leads en etapa cotización sin actividad en 7 días'. Resultados paginados de 15 en 15.",
  "inputSchema": {
    "type": "object",
    "required": ["payload"],
    "properties": {
      "payload": {
        "type": "array",
        "minItems": 1,
        "items": {
          "type": "object",
          "required": ["attribute_key", "filter_operator", "values"],
          "properties": {
            "attribute_key": {"type": "string", "description": "Nombre del atributo a filtrar, ej: 'email', 'country_code', o una clave de custom_attributes"},
            "filter_operator": {"type": "string", "enum": ["equal_to", "not_equal_to", "contains", "does_not_contain"]},
            "values": {"type": "array", "items": {"type": "string"}},
            "query_operator": {"type": "string", "enum": ["AND", "OR"], "description": "Cómo se combina con la condición siguiente. Debe ser null en la última condición."}
          }
        }
      },
      "page": {"type": "integer", "default": 1}
    }
  }
}
```

Validación en el servidor: el `query_operator` del **último** elemento debe ser `null` u omitirse. Si el modelo lo deja en `"AND"`, Chatwoot puede devolver 422 o resultados inesperados. Corregirlo automáticamente y loguear a debug.

---

### 5.18 `filter_conversations`

`POST /api/v1/accounts/{account_id}/conversations/filter`

Mismo schema de `payload` que §5.17. Atributos filtrables típicos: `status`, `assignee_id`, `inbox_id`, `labels`, `browser_language`, `country_code`, y cualquier clave de `custom_attributes`.

```json
{
  "name": "filter_conversations",
  "description": "Filtra conversaciones por condiciones combinadas, incluyendo atributos personalizados. Úsalo para encontrar tu propia cartera: 'conversaciones con etapa=cotizacion y prioridad alta'.",
  "inputSchema": { "…igual que filter_contacts…" }
}
```

---

### Outbound (iniciar conversación)

---

### 5.19 `create_contact`

`POST /api/v1/accounts/{account_id}/contacts`

```json
{
  "name": "create_contact",
  "description": "Crea un contacto nuevo. Requiere inbox_id: usa list_inboxes para elegir el canal. Antes de crear, busca con search_contacts para no duplicar leads.",
  "inputSchema": {
    "type": "object",
    "required": ["inbox_id"],
    "properties": {
      "inbox_id": {"type": "integer"},
      "name": {"type": "string"},
      "email": {"type": "string"},
      "phone_number": {"type": "string", "description": "Formato E.164"},
      "identifier": {"type": "string"},
      "custom_attributes": {"type": "object"}
    }
  }
}
```

La respuesta incluye el `contact_inbox` con su `source_id`. **Guardarlo y devolverlo en la salida del tool**, porque es exactamente lo que necesita `create_conversation` y evita una llamada extra.

---

### 5.20 `list_inboxes`

`GET /api/v1/accounts/{account_id}/inboxes`

```json
{
  "name": "list_inboxes",
  "description": "Lista los canales (inboxes) de la cuenta con su tipo: website, whatsapp, email, api, etc. Consúltalo para saber por dónde puedes iniciar una conversación saliente.",
  "inputSchema": {"type": "object", "properties": {}}
}
```

Cachear. Proyección: `id`, `name`, `channel_type`, `phone_number` (si aplica). El `channel_type` determina qué puede hacer el agente: solo los tipos **Website, Phone, Api y Email** admiten crear conversaciones vía API.

---

### 5.21 `resolve_contact_source_id`

`GET /contacts/{id}/contactable_inboxes` + `POST /contacts/{id}/contact_inboxes`

Tool compuesto. Resuelve la trampa del `source_id` (§6.2).

```json
{
  "name": "resolve_contact_source_id",
  "description": "Obtiene el source_id necesario para iniciar una conversación con un contacto en un canal específico. Si el contacto aún no está vinculado a ese canal, crea el vínculo. Llama esto antes de create_conversation.",
  "inputSchema": {
    "type": "object",
    "required": ["contact_id", "inbox_id"],
    "properties": {
      "contact_id": {"type": "integer"},
      "inbox_id": {"type": "integer"}
    }
  }
}
```

Devuelve `{"contact_id":1,"inbox_id":2,"source_id":"...","created":false}`.

---

### 5.22 `create_conversation`

`POST /api/v1/accounts/{account_id}/conversations`

```json
{
  "name": "create_conversation",
  "description": "Inicia una conversación saliente. Requiere source_id, que obtienes con resolve_contact_source_id. Puedes incluir el primer mensaje en la misma llamada.",
  "inputSchema": {
    "type": "object",
    "required": ["source_id", "inbox_id"],
    "properties": {
      "source_id": {"type": "string", "description": "Obtenido de resolve_contact_source_id"},
      "inbox_id": {"type": "integer", "description": "Solo tipos Website, Phone, Api o Email"},
      "contact_id": {"type": "integer"},
      "status": {"type": "string", "enum": ["open", "resolved", "pending"], "default": "open"},
      "assignee_id": {"type": "integer"},
      "team_id": {"type": "integer"},
      "custom_attributes": {"type": "object", "description": "Calificación inicial del lead"},
      "message": {
        "type": "object",
        "required": ["content"],
        "properties": {"content": {"type": "string"}},
        "description": "Primer mensaje de la conversación"
      }
    }
  }
}
```

Nota: el `snoozed_until` de este payload sí acepta ISO 8601 (`2030-07-21T17:32:28Z`), a diferencia de `toggle_status`. No se expone en v1.

---

### 5.23 `list_whatsapp_templates`

`GET /api/v1/accounts/{account_id}/inboxes/{id}/message_templates`

```json
{
  "name": "list_whatsapp_templates",
  "description": "Lista las plantillas de WhatsApp aprobadas y cacheadas para un inbox de WhatsApp. En WhatsApp, fuera de la ventana de 24h solo puedes iniciar contacto con una plantilla aprobada. Consulta esto antes de send_whatsapp_template.",
  "inputSchema": {
    "type": "object",
    "required": ["inbox_id"],
    "properties": {
      "inbox_id": {"type": "integer"},
      "name": {"type": "string", "description": "Filtra por nombre de plantilla"}
    }
  }
}
```

---

### 5.24 `send_whatsapp_template`

`POST /conversations/{id}/messages` con `template_params`

```json
{
  "name": "send_whatsapp_template",
  "description": "Envía una plantilla de WhatsApp pre-aprobada. Usa list_whatsapp_templates para ver las disponibles y sus variables.",
  "inputSchema": {
    "type": "object",
    "required": ["conversation_id", "template_name", "language", "category", "body_params"],
    "properties": {
      "conversation_id": {"type": "integer"},
      "template_name": {"type": "string", "description": "Nombre exacto de la plantilla aprobada en WhatsApp Business Manager"},
      "language": {"type": "string", "description": "Código BCP 47, ej: es, es_MX, en_US"},
      "category": {"type": "string", "enum": ["UTILITY", "MARKETING", "SHIPPING_UPDATE", "TICKET_UPDATE", "ISSUE_RESOLUTION"]},
      "body_params": {"type": "object", "description": "Variables del cuerpo indexadas por posición: {\"1\":\"Juan\",\"2\":\"12345\"}"},
      "header_media_url": {"type": "string", "description": "URL pública, solo para plantillas con header de media"},
      "header_media_type": {"type": "string", "enum": ["image", "video", "document"]},
      "rendered_content": {"type": "string", "description": "Texto ya renderizado de la plantilla, con las variables sustituidas. Es lo que queda en el transcript."}
    }
  }
}
```

Mapeo al payload de Chatwoot:

```json
{
  "content": "<rendered_content>",
  "message_type": "outgoing",
  "template_params": {
    "name": "<template_name>",
    "category": "<category>",
    "language": "<language>",
    "processed_params": {
      "body": { "1": "Juan" },
      "header": { "media_url": "...", "media_type": "image" }
    }
  }
}
```

Detalle importante: por defecto Chatwoot trata `content` como el texto **ya renderizado** (`content_mode: "rendered"`). Si prefieres enviar el cuerpo original con placeholders y dejar que Chatwoot sustituya, hay que pasar `content_mode: "raw_template"`. Recomendación: mantener el default `rendered` y exigir `rendered_content` al modelo — es más predecible y el transcript queda legible para el vendedor humano.

---

### 5.25 `list_canned_responses`

`GET /api/v1/accounts/{account_id}/canned_responses`

```json
{
  "name": "list_canned_responses",
  "description": "Lista las respuestas predefinidas de la cuenta. Úsalas para precios, condiciones comerciales y políticas: es texto aprobado por el equipo. Prefiere una canned response antes que improvisar cifras o compromisos.",
  "inputSchema": {"type": "object", "properties": {}}
}
```

Cachear. Proyección: `id`, `short_code`, `content`. Este tool es el guardrail más económico contra que el agente invente precios.

---

## 6. Lógica no trivial: implementar con cuidado

### 6.1 Etiquetas: read-then-write obligatorio

La API de etiquetas **sobrescribe**, no agrega. Implementación de `add_conversation_labels`:

```go
func (s *Server) AddConversationLabels(ctx context.Context, convID int, add []string) ([]string, error) {
	current, err := s.cw.GetConversationLabels(ctx, convID)
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer etiquetas actuales: %w", err)
	}

	set := make(map[string]struct{}, len(current)+len(add))
	ordered := make([]string, 0, len(current)+len(add))
	for _, l := range append(current, add...) {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		if _, dup := set[l]; dup {
			continue
		}
		set[l] = struct{}{}
		ordered = append(ordered, l)
	}

	if err := s.guard.ValidateLabels(add); err != nil { // §7
		return nil, err
	}
	return s.cw.SetConversationLabels(ctx, convID, ordered)
}
```

Puntos a cuidar:

- **Race condition real**: si un humano etiqueta al mismo tiempo, se pierde su cambio. No hay ETag ni If-Match en la API, así que no se puede resolver del todo. Mitigación práctica: mantener la ventana entre GET y POST lo más corta posible y no cachear nunca las etiquetas de una conversación concreta (sí las del catálogo de cuenta).
- Preservar el orden y deduplicar de forma case-sensitive: Chatwoot trata `Lead` y `lead` como etiquetas distintas.
- Validar solo las etiquetas **nuevas** contra la whitelist, no las preexistentes; si un humano puso una etiqueta fuera de la lista, el bot no debe fallar por eso.

### 6.2 Outbound: la cadena del `source_id`

`POST /conversations` requiere `source_id`. Un `contact_id` no basta. El modelo de datos es:

```
Contact ──< ContactInbox >── Inbox
              │
              └── source_id   ← esto es lo que pide la API
```

Secuencia que implementa `resolve_contact_source_id`:

1. `GET /contacts/{id}/contactable_inboxes` → devuelve `payload[]` con los inboxes en los que el contacto ya es contactable, cada uno con su `source_id`.
2. Si el `inbox_id` pedido está en la lista → devolver ese `source_id`, `created: false`.
3. Si no está → `POST /contacts/{id}/contact_inboxes` con `{"inbox_id": N}`. La respuesta trae el `source_id` nuevo. Devolver con `created: true`.
4. Si el paso 3 falla con 422, casi siempre es porque el `channel_type` del inbox no admite creación vía API. Devolver un error explícito: *"El inbox {N} es de tipo {tipo} y no admite conversaciones iniciadas por API. Tipos válidos: Website, Phone, Api, Email."*

Para inboxes de WhatsApp/SMS, el `source_id` suele ser el número de teléfono del contacto; se puede pasar explícitamente en el POST del paso 3. Contemplar un parámetro opcional `source_id` en el tool para ese caso.

Flujo completo de outbound que el agente debe seguir, y que conviene escribir en las descripciones de los tools:

```
search_contacts (¿ya existe?)
  └── no existe → create_contact (devuelve source_id)
  └── existe    → resolve_contact_source_id
                     └── create_conversation (con source_id + message)
```

### 6.3 Snooze como motor de follow-up

`toggle_status` con `status: "snoozed"`:

- Con `snoozed_until` (Unix segundos): reabre en esa fecha **y también** si el cliente responde antes.
- Sin `snoozed_until`: se postpone hasta la siguiente respuesta del cliente.

En ambos casos la conversación **siempre** reabre cuando el contacto responde. Esto significa que el agente no necesita polling ni cron: postergar es suficiente. Documentarlo en la descripción del tool porque es contraintuitivo y el modelo tenderá a inventar recordatorios propios.

Conversión requerida en el servidor:

```go
t, err := time.Parse(time.RFC3339, in.SnoozeUntilISO)
if err != nil {
	return nil, fmt.Errorf("snooze_until_iso debe ser ISO 8601 (ej: 2026-09-08T14:00:00Z): %w", err)
}
if !t.After(time.Now()) {
	return nil, errors.New("snooze_until_iso debe ser una fecha futura")
}
body.SnoozedUntil = t.Unix()
```

### 6.4 Normalización de `message_type`

Ver tabla en §4.2. Implementar como función única en `internal/tools/helpers.go` y usarla en `get_messages` y en cualquier respuesta que embeba mensajes. Es el bug más silencioso posible: si el agente confunde sus propios mensajes con los del cliente, la conversación se descarrila sin error visible en ningún log.

---

## 7. Guardrails

### 7.1 Superficie deliberadamente excluida

**No implementar como tools**, ni siquiera "por completitud":

- Cualquier `DELETE`: contactos, etiquetas, equipos, agentes, inboxes, mensajes, webhooks, automation rules.
- `POST /actions/contact_merge` — es destructivo, el contacto absorbido se elimina permanentemente.
- Gestión de agentes, equipos, inboxes y agent bots (crear/modificar/eliminar).
- `POST /webhooks` y `POST /automation_rules` — configuración de infraestructura.
- `PATCH /accounts/{id}` y todo lo de Platform API.
- Reportes (`/api/v2/...`) — Tier 3, fuera de alcance.
- Help Center / portales — Tier 3, fuera de alcance.

Razón: un LLM con acceso a `DELETE /contacts/{id}` es un incidente esperando a ocurrir. La regla es que el MCP no exponga nada cuyo peor caso sea pérdida irreversible de datos.

### 7.2 Whitelist de etiquetas

Si `CHATWOOT_ALLOWED_LABELS` está definido, `add_conversation_labels` y `add_contact_labels` deben rechazar etiquetas fuera de la lista con un error explicativo que incluya las permitidas, para que el modelo pueda corregirse en el siguiente intento:

```
Etiqueta 'super-mega-lead' no permitida. Etiquetas disponibles: lead-caliente, lead-tibio, cotizacion-enviada, no-interesado.
```

### 7.3 Modo solo lectura

Con `CHATWOOT_READONLY=true`, registrar únicamente: `get_messages`, `get_conversation`, `list_conversations`, `get_contact`, `get_conversation_labels`, `get_contact_labels`, `list_account_labels`, `list_agents`, `list_teams`, `list_inboxes`, `list_custom_attribute_definitions`, `list_canned_responses`, `list_contact_conversations`, `search_contacts`, `filter_contacts`, `filter_conversations`, `list_whatsapp_templates`.

Imprescindible para evaluar el agente contra datos de producción sin escribir nada.

### 7.4 Logging seguro

- Logs a **stderr**, siempre. `stdout` es el canal del transporte stdio de MCP; escribir ahí corrompe el protocolo.
- Nunca loguear `CHATWOOT_API_TOKEN`, ni parcialmente.
- Loguear cada llamada a tool con: nombre del tool, `conversation_id`/`contact_id`, status HTTP resultante y latencia. **No** loguear el `content` de los mensajes por defecto (son datos de clientes); solo bajo `LOG_LEVEL=debug`.
- Contemplar `redact()` para emails y teléfonos en logs de nivel info.

---

## 8. El MCP no es suficiente por sí solo: webhooks

El MCP es **pull**: el agente pregunta. Chatwoot es **push**: los mensajes llegan cuando llegan. El MCP le da manos al agente, pero no oídos.

Arquitectura completa:

```
Cliente escribe en WhatsApp
        ↓
Chatwoot recibe el mensaje
        ↓
Webhook  event=message_created  →  tu servicio orquestador
                                         ↓
                              invoca al agente LLM
                                         ↓
                              agente usa el MCP Server (Go)
                                         ↓
                              MCP llama a la Application API
                                         ↓
                              Chatwoot envía la respuesta al cliente
```

El webhook se configura **una vez y a mano** (o con `POST /webhooks`, que esta spec deja fuera del MCP a propósito). Eventos relevantes: `message_created`, `conversation_created`, `conversation_status_changed`.

Consideraciones para el orquestador, fuera del alcance de este repo pero que conviene tener presentes al diseñar:

- Filtrar en el webhook los mensajes cuyo `sender_type` sea `AgentBot` o el propio usuario del bot, o el agente se responderá a sí mismo en loop infinito.
- Ignorar `message_type: 2` (eventos de actividad).
- Debounce: si el cliente escribe tres mensajes seguidos en 5 segundos, esperar antes de invocar al agente en lugar de generar tres respuestas.

---

## 9. Verificación y pruebas

### 9.1 Smoke test manual (antes de escribir código Go)

Confirmar credenciales y `account_id` con curl:

```bash
export CHATWOOT_BASE_URL=https://app.chatwoot.com
export CHATWOOT_API_TOKEN=xxxx
export CHATWOOT_ACCOUNT_ID=1

# 1. ¿El token es válido y de qué usuario?
curl -s "$CHATWOOT_BASE_URL/api/v1/profile" \
  -H "api_access_token: $CHATWOOT_API_TOKEN" | jq '{id, name, email}'

# 2. ¿Hay conversaciones?
curl -s "$CHATWOOT_BASE_URL/api/v1/accounts/$CHATWOOT_ACCOUNT_ID/conversations?status=open" \
  -H "api_access_token: $CHATWOOT_API_TOKEN" | jq '.data.meta'

# 3. Catálogo de etiquetas (para poblar CHATWOOT_ALLOWED_LABELS)
curl -s "$CHATWOOT_BASE_URL/api/v1/accounts/$CHATWOOT_ACCOUNT_ID/labels" \
  -H "api_access_token: $CHATWOOT_API_TOKEN" | jq '.payload[].title'

# 4. Nota interna de prueba (no la ve el cliente)
curl -s -X POST "$CHATWOOT_BASE_URL/api/v1/accounts/$CHATWOOT_ACCOUNT_ID/conversations/1/messages" \
  -H "api_access_token: $CHATWOOT_API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"content":"prueba MCP","message_type":"outgoing","private":true}'
```

La forma exacta de las respuestas (`data.payload` vs `payload` vs array plano) **varía entre endpoints** en Chatwoot. Verificar con curl antes de escribir los structs de Go; no asumir consistencia.

### 9.2 Tests automatizados

- **Unit**: tests de tabla para la conversión ISO→Unix, la normalización de `message_type`, la deduplicación de etiquetas y la validación de `query_operator` en filtros.
- **Cliente HTTP**: `httptest.Server` con respuestas grabadas de Chatwoot (fixtures JSON en `testdata/`). Cubrir 200, 401, 404, 422 y 429.
- **Integración**: opcional, contra una instancia de Chatwoot en Docker, detrás de un build tag `//go:build integration`.
- **Test crítico**: verificar explícitamente que `add_conversation_labels` **no** borra las etiquetas preexistentes. Es el bug más probable de todo el proyecto.

### 9.3 Definition of done

- [ ] `LoadConfig` falla en arranque si falta cualquier variable obligatoria, y reporta todas las faltantes de una vez.
- [ ] Verificación de token contra `/api/v1/profile` al arrancar, con log del usuario autenticado.
- [ ] Los 7 tools de Tier 1 y los 19 de Tier 2 registrados y funcionales.
- [ ] `CHATWOOT_READONLY=true` deja fuera del registro todos los tools de escritura.
- [ ] Ningún tool destructivo expuesto (§7.1). Verificado por revisión de código.
- [ ] `add_*_labels` probado contra sobrescritura.
- [ ] Reintentos solo en GET; escrituras nunca se reintentan.
- [ ] Todos los logs a stderr; token nunca presente en ningún log.
- [ ] Respuestas proyectadas, no passthrough del payload crudo.
- [ ] `README.md` con las variables de entorno y el bloque de configuración para el cliente MCP.

---

## 10. Empaquetado

### 10.1 Dockerfile

Build multi-stage, imagen final `gcr.io/distroless/static` o `scratch`, binario estático (`CGO_ENABLED=0`), usuario no root. El binario habla por stdio, así que no expone puertos.

### 10.2 Makefile

Targets mínimos: `build`, `test`, `lint` (golangci-lint), `run` (carga `.env` para desarrollo), `docker`.

### 10.3 Configuración en el cliente MCP

```json
{
  "mcpServers": {
    "chatwoot": {
      "command": "/usr/local/bin/chatwoot-mcp",
      "env": {
        "CHATWOOT_BASE_URL": "https://app.chatwoot.com",
        "CHATWOOT_API_TOKEN": "...",
        "CHATWOOT_ACCOUNT_ID": "1",
        "CHATWOOT_ALLOWED_LABELS": "lead-caliente,lead-tibio,cotizacion-enviada,no-interesado"
      }
    }
  }
}
```

---

## 11. Referencia de endpoints (resumen)

Todos relativos a `{CHATWOOT_BASE_URL}/api/v1/accounts/{CHATWOOT_ACCOUNT_ID}`.

| Tool | Método | Path |
|---|---|---|
| `send_message` | POST | `/conversations/{id}/messages` |
| `get_messages` | GET | `/conversations/{id}/messages` |
| `get_conversation` | GET | `/conversations/{id}` |
| `list_conversations` | GET | `/conversations` |
| `get_contact` | GET | `/contacts/{id}` |
| `update_conversation_status` | POST | `/conversations/{id}/toggle_status` |
| `set_typing_indicator` | POST | `/conversations/{id}/toggle_typing_status` |
| `set_conversation_attributes` | POST | `/conversations/{id}/custom_attributes` |
| `remove_conversation_attributes` | POST | `/conversations/{id}/destroy_custom_attributes` |
| `update_contact` | PUT | `/contacts/{id}` |
| `list_custom_attribute_definitions` | GET | `/custom_attribute_definitions` |
| `get_conversation_labels` | GET | `/conversations/{id}/labels` |
| `add_conversation_labels` | GET+POST | `/conversations/{id}/labels` |
| `remove_conversation_labels` | GET+POST | `/conversations/{id}/labels` |
| `list_account_labels` | GET | `/labels` |
| `get_contact_labels` | GET | `/contacts/{id}/labels` |
| `add_contact_labels` | GET+POST | `/contacts/{id}/labels` |
| `set_conversation_priority` | POST | `/conversations/{id}/toggle_priority` |
| `assign_conversation` | POST | `/conversations/{id}/assignments` |
| `list_agents` | GET | `/agents` |
| `list_teams` | GET | `/teams` |
| `list_contact_conversations` | GET | `/contacts/{id}/conversations` |
| `search_contacts` | GET | `/contacts/search` |
| `filter_contacts` | POST | `/contacts/filter` |
| `filter_conversations` | POST | `/conversations/filter` |
| `create_contact` | POST | `/contacts` |
| `list_inboxes` | GET | `/inboxes` |
| `resolve_contact_source_id` | GET+POST | `/contacts/{id}/contactable_inboxes`, `/contacts/{id}/contact_inboxes` |
| `create_conversation` | POST | `/conversations` |
| `list_whatsapp_templates` | GET | `/inboxes/{id}/message_templates` |
| `send_whatsapp_template` | POST | `/conversations/{id}/messages` |
| `list_canned_responses` | GET | `/canned_responses` |

**Total: 32 tools** (7 Tier 1 + 25 Tier 2, contando los tools auxiliares de lectura que el Tier 2 requiere).

---

## 12. Fuentes

- Introducción a las APIs de Chatwoot: `https://developers.chatwoot.com/api-reference/introduction`
- Spec OpenAPI (fuente de verdad para schemas): `https://raw.githubusercontent.com/chatwoot/chatwoot/develop/swagger/tag_groups/application_swagger.json`
- Features soportadas por canal: `https://developers.chatwoot.com/self-hosted/supported-features`
- Estados de mensaje por canal: `https://developers.chatwoot.com/self-hosted/message-statuses`
- Rate limiting en self-hosted: `https://developers.chatwoot.com/self-hosted/monitoring/rate-limiting`
- Agent skill del CLI oficial (referencia de qué operaciones consideró esenciales el equipo de Chatwoot): `https://developers.chatwoot.com/cli/agent-skill`
