# chatwoot-mcp

Servidor MCP en Go que expone la Application API de Chatwoot como herramientas para un agente de ventas.

Por defecto habla por **HTTP** (Streamable MCP) en `PORT` (default `8080`). Toda la configuración sale de variables de entorno; el token nunca se acepta por flags.

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
| `MCP_TRANSPORT` | no | `http` | `http` o `stdio`. |
| `PORT` | no | `8080` | Puerto HTTP. Railway lo inyecta; en Networking usa el mismo número. |
| `MCP_AUTH_TOKEN` | no | vacío | Bearer token para el endpoint MCP. Recomendado en público. |
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

Healthcheck: `GET /healthz`. MCP: `POST /mcp`.

## Cliente MCP remoto

En Railway, genera el domain con el mismo puerto que `PORT` (normalmente `8080`).

```json
{
  "mcpServers": {
    "chatwoot": {
      "type": "http",
      "url": "https://TU-SERVICIO.up.railway.app/mcp",
      "headers": {
        "Authorization": "Bearer TU_MCP_AUTH_TOKEN"
      }
    }
  }
}
```

Sin auth (`MCP_AUTH_TOKEN` vacío):

```json
{
  "mcpServers": {
    "chatwoot": {
      "type": "http",
      "url": "https://TU-SERVICIO.up.railway.app/mcp"
    }
  }
}
```

## Cliente MCP local (stdio)

```json
{
  "mcpServers": {
    "chatwoot": {
      "command": "/usr/local/bin/chatwoot-mcp",
      "env": {
        "MCP_TRANSPORT": "stdio",
        "CHATWOOT_BASE_URL": "https://app.chatwoot.com",
        "CHATWOOT_API_TOKEN": "...",
        "CHATWOOT_ACCOUNT_ID": "1"
      }
    }
  }
}
```

## Docker

```bash
make docker
```

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
