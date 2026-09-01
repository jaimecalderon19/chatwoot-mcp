# chatwoot-mcp

Servidor MCP en Go que expone la Application API de Chatwoot como herramientas para un agente de ventas.

Habla por **stdio**. Toda la configuración sale de variables de entorno; el token nunca se acepta por flags.

## Variables de entorno

| Variable | Obligatoria | Default | Notas |
|---|---|---|---|
| `CHATWOOT_BASE_URL` | sí | | Sin slash final. Solo `https`, salvo `CHATWOOT_ALLOW_INSECURE=true`. |
| `CHATWOOT_API_TOKEN` | sí | | Token de **usuario** (no AgentBot). Nunca se loguea. |
| `CHATWOOT_ACCOUNT_ID` | sí | | Entero. Queda fijo para que el modelo no salte de cuenta. |
| `CHATWOOT_TIMEOUT_SECONDS` | no | `30` | Timeout por request. |
| `CHATWOOT_ALLOWED_LABELS` | no | vacío = sin restricción | CSV. Si está definido, `add_*_labels` rechaza etiquetas fuera de la lista. |
| `CHATWOOT_READONLY` | no | `false` | Si `true`, solo tools de lectura. |
| `CHATWOOT_ALLOW_INSECURE` | no | `false` | Permite `http` (desarrollo local). |
| `LOG_LEVEL` | no | `info` | `debug` \| `info` \| `warn` \| `error`. Logs a **stderr**. |
| `APP_ENV` | no | | Si es `development`, carga `.env` opcional. |

Copia `.env.example` a `.env` para desarrollo local.

## Compilar y probar

```bash
make build
make test
```

El binario queda en `bin/chatwoot-mcp`.

Al arrancar, el servidor llama a `GET /api/v1/profile`. Si el token es inválido, aborta. Si es válido, loguea el `id` y `email` del usuario autenticado.

## Cliente MCP

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

## Docker

```bash
make docker
```

La imagen es distroless, usuario no root, sin puertos: el proceso habla por stdio.

## Tools

32 tools (Tier 1 conversacional + Tier 2 ventas). Con `CHATWOOT_READONLY=true` solo se registran las de lectura.

No se exponen `DELETE`, merge de contactos, gestión de agentes/inboxes, webhooks, automation rules, reportes ni Help Center.

## Outbound

```
search_contacts
  └── no existe → create_contact (devuelve source_id)
  └── existe    → resolve_contact_source_id
                     └── create_conversation (source_id + message)
```
