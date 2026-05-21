# BANGLAB2BB2C Support

WhatsApp customer-support platform for BANGLAB2BB2C. Single binary, single tenant.

Forked from the upstream Whatomate WhatsApp Business Platform; trimmed to the support use case (live chat, agent assignment, AI auto-reply, SLA, analytics) with the multi-tenant scaffolding stripped out.

## What's here

- **WhatsApp Cloud API** integration (chat, templates, media).
- **Live chat** with WebSocket push to agents.
- **Agent assignment** — admins/leaders assign chats; agents claim from queue.
- **Chatbot / AI auto-reply** — keyword rules, flow builder, AI fallbacks (OpenAI / Anthropic / Google).
- **Canned responses** with `/shortcut` slash commands.
- **Conversation notes** (internal) and **tags**.
- **SLA tracking** and audit logs.
- **Voice calling + IVR** (optional, configurable).
- **Catalog** for support-context lookups.
- **Analytics** — agent performance, message volume, response times.

Phase 2 will remove campaigns, WhatsApp Flows, custom actions, widgets, import/export, outbound webhooks, and API keys — none of which the support use case needs.

## Quick start (Docker)

```bash
cd docker
cp .env.example .env   # optionally edit POSTGRES_* values
docker compose -f docker-compose.dev.yml up -d --build
```

The app boots on `http://localhost:8081`. The first run seeds one organization called **BANGLAB2BB2C** and a default admin defined in `config.toml`:

```
email:    admin@banglab2bb2c.local
password: admin
```

Change the password immediately after first login.

## Build from source

```bash
# Backend (port 8080)
make run-migrate

# Frontend (port 3000, proxies API to 8080)
cd frontend && npm install && npm run dev
```

Production single-binary build:

```bash
make build-prod
./banglab2bb2c server -migrate
```

## CLI

```bash
./banglab2bb2c server              # API + 1 embedded worker
./banglab2bb2c server -workers=0   # API only
./banglab2bb2c worker -workers=4   # Workers only (for scaling)
./banglab2bb2c version             # Show version
```

## Configuration

Copy `config.example.toml` to `config.toml` and edit. Every value can also be overridden by an environment variable with the `BANGLAB2BB2C_` prefix — for example, `BANGLAB2BB2C_DATABASE_HOST=db.internal` overrides `[database].host`.

## Tech stack

- Backend: Go (Fastglue) + Postgres + Redis.
- Frontend: Vue 3 + Vite + shadcn-vue.
- WhatsApp: Meta Cloud API.

## License

See [LICENSE](LICENSE).
