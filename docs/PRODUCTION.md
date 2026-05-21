# Production Deployment — `support.banglab2bb2c.com`

This guide walks through deploying BANGLAB2BB2C to a single Linux host using
Docker Compose, with Nginx + Let's Encrypt (Certbot) terminating TLS for the
domain `support.banglab2bb2c.com`.

The image is messaging-only — calling / IVR features (Piper TTS, ffmpeg, WebRTC
relay) are intentionally **not** bundled.

---

## 1. Architecture

```
                       Internet
                          │  443/tcp, 80/tcp
                          ▼
                    ┌───────────┐
                    │   nginx   │  TLS termination, HTTP→HTTPS,
                    │ (host net)│  static security headers
                    └─────┬─────┘
                          │ http://app:8080
                          ▼
                    ┌───────────┐
                    │    app    │  Go binary + embedded React frontend
                    └─────┬─────┘
                ┌─────────┴────────┐
                ▼                  ▼
          ┌─────────┐        ┌──────────┐
          │   db    │        │  redis   │
          │ Postgres│        │  cache   │
          └─────────┘        └──────────┘
```

All services live on the `banglab2bb2c` bridge network. Only `nginx` exposes
host ports (80/443); the database and Redis are reachable **only** from inside
the compose network.

A `certbot` sidecar renews certificates every 12h and shares the
`/etc/letsencrypt` volume with nginx. Nginx reloads itself every 6h to pick up
renewed certs.

---

## 2. Prerequisites

- A Linux host with a public IPv4 (or IPv6) address.
- DNS `A` record: `support.banglab2bb2c.com` → host IP. Verify with:
  ```bash
  dig +short A support.banglab2bb2c.com
  ```
- Docker Engine ≥ 24 and the Compose plugin (`docker compose version`).
- Firewall open on TCP 80 and 443. **Do not** open 5432 (Postgres) or 6379
  (Redis) — they stay private.
- A non-root user in the `docker` group (or sudo).

---

## 3. First-time setup

### 3.1 Clone and enter the docker directory

```bash
git clone <your-repo-url> /opt/banglab2bb2c
cd /opt/banglab2bb2c/docker
```

### 3.2 Create `.env` and `config.toml`

```bash
cp .env.prod.example .env
cp config.prod.example.toml config.toml
```

Generate the three secrets:

```bash
# Postgres password
openssl rand -base64 24

# AES encryption_key (used to encrypt WhatsApp tokens at rest)
openssl rand -hex 32

# JWT secret
openssl rand -hex 32
```

Edit `.env` and `config.toml` and replace every `CHANGE_ME` / `__CHANGE_ME...__`
placeholder. **The Postgres password must be identical in both files.**

Lock down the secrets:

```bash
chmod 600 .env config.toml
```

### 3.3 Bootstrap TLS

Nginx will refuse to start with the production vhost in place because the
referenced certificate files do not yet exist. The repo ships a bootstrap
vhost that serves only the ACME challenge.

```bash
cd /opt/banglab2bb2c/docker

# Swap real vhost out, bootstrap vhost in
mv nginx/conf.d/support.banglab2bb2c.com.conf      nginx/conf.d/support.banglab2bb2c.com.conf.disabled
mv nginx/conf.d/support.banglab2bb2c.com.init.conf nginx/conf.d/support.banglab2bb2c.com.conf

# Start nginx + the rest of the stack
docker compose -f docker-compose.prod.yml up -d --build

# Wait ~5 seconds for nginx to come up, then request the cert.
# Replace ACME_EMAIL if you didn't fill it in .env.
docker compose -f docker-compose.prod.yml run --rm \
  --entrypoint "" certbot \
  certbot certonly \
    --webroot -w /var/www/certbot \
    -d support.banglab2bb2c.com \
    --email "$(grep ^ACME_EMAIL .env | cut -d= -f2)" \
    --agree-tos --no-eff-email --non-interactive

# Swap the real vhost back in and reload nginx
mv nginx/conf.d/support.banglab2bb2c.com.conf          nginx/conf.d/support.banglab2bb2c.com.init.conf
mv nginx/conf.d/support.banglab2bb2c.com.conf.disabled nginx/conf.d/support.banglab2bb2c.com.conf
docker compose -f docker-compose.prod.yml exec nginx nginx -s reload
```

Verify:

```bash
curl -I https://support.banglab2bb2c.com/health
# Expect HTTP/2 200
```

### 3.4 First login

Open `https://support.banglab2bb2c.com` in a browser. Log in with the
`[default_admin]` email/password from `config.toml`, then immediately:

1. Create a real admin user from the UI.
2. Delete or change the default admin.
3. Comment out the `[default_admin]` block in `config.toml` and re-deploy
   (`docker compose ... up -d`).

---

## 4. Day-to-day operations

All commands run from `/opt/banglab2bb2c/docker`.

| Action | Command |
|---|---|
| Start / update stack | `docker compose -f docker-compose.prod.yml up -d --build` |
| Stop stack | `docker compose -f docker-compose.prod.yml down` |
| Tail app logs | `docker compose -f docker-compose.prod.yml logs -f app` |
| Tail nginx logs | `docker compose -f docker-compose.prod.yml logs -f nginx` |
| Restart app only | `docker compose -f docker-compose.prod.yml restart app` |
| Postgres shell | `docker compose -f docker-compose.prod.yml exec db psql -U banglab2bb2c -d banglab2bb2c` |
| Redis shell | `docker compose -f docker-compose.prod.yml exec redis redis-cli` |
| Force cert renewal | `docker compose -f docker-compose.prod.yml exec certbot certbot renew --force-renewal && docker compose -f docker-compose.prod.yml exec nginx nginx -s reload` |

### Healthchecks

The app container exposes `/health` (liveness) and `/ready` (db + redis ping).
Container-level healthchecks are configured against `/health`; verify with:

```bash
docker compose -f docker-compose.prod.yml ps
# Look for "healthy" in the STATUS column.
```

---

## 5. Upgrading the application

```bash
cd /opt/banglab2bb2c
git pull
cd docker
docker compose -f docker-compose.prod.yml build app
docker compose -f docker-compose.prod.yml up -d app
```

The `-migrate` flag in the app `CMD` runs DB migrations automatically on
start, so no separate migration step is needed.

---

## 6. Backups

### 6.1 Postgres

The data lives in the named volume `postgres-data`. A simple nightly dump
script:

```bash
#!/usr/bin/env bash
# /opt/banglab2bb2c/backup-db.sh
set -euo pipefail
cd /opt/banglab2bb2c/docker
STAMP=$(date -u +%Y%m%dT%H%M%SZ)
OUT=/var/backups/banglab2bb2c/pg-${STAMP}.sql.gz
mkdir -p "$(dirname "$OUT")"
docker compose -f docker-compose.prod.yml exec -T db \
  pg_dump -U banglab2bb2c banglab2bb2c | gzip > "$OUT"
# Keep last 14 days.
find /var/backups/banglab2bb2c -name 'pg-*.sql.gz' -mtime +14 -delete
```

Cron entry (`/etc/cron.d/banglab2bb2c-backup`):

```
30 2 * * * root /opt/banglab2bb2c/backup-db.sh >> /var/log/banglab2bb2c-backup.log 2>&1
```

### 6.2 Uploads

Media uploaded through the UI lands in the `app-uploads` Docker volume.
Mirror it to off-host storage:

```bash
docker run --rm -v app-uploads:/data -v /var/backups/banglab2bb2c:/out alpine \
  tar -czf /out/uploads-$(date -u +%Y%m%d).tar.gz -C /data .
```

For larger deployments, set `[storage] type = "s3"` in `config.toml` and let
S3 handle durability.

### 6.3 Certificates

`/etc/letsencrypt` lives in the `certbot-conf` volume. It can be re-issued
at any time, but keeping a copy avoids hitting Let's Encrypt rate limits during
a disaster restore.

---

## 7. Restoring from a backup

```bash
cd /opt/banglab2bb2c/docker
docker compose -f docker-compose.prod.yml up -d db
gunzip -c /var/backups/banglab2bb2c/pg-XXXXXXXX.sql.gz | \
  docker compose -f docker-compose.prod.yml exec -T db \
  psql -U banglab2bb2c -d banglab2bb2c
docker compose -f docker-compose.prod.yml up -d app nginx
```

---

## 8. Security checklist

- [ ] `config.toml` and `.env` are `chmod 600`, owned by root (or the deploy user).
- [ ] `[app] environment = "production"` and `debug = false`.
- [ ] `encryption_key` is a fresh, 64-hex-char value (never reused across envs).
- [ ] `jwt.secret` is a fresh, 64-hex-char value.
- [ ] `[default_admin]` is disabled after creating real admin users.
- [ ] Host firewall opens 80/443 only.
- [ ] Postgres / Redis ports are **not** in the `ports:` list of compose (they aren't, by default).
- [ ] HSTS verified: `curl -sI https://support.banglab2bb2c.com | grep -i strict-transport`.
- [ ] WhatsApp webhook URL set in Meta dashboard:
      `https://support.banglab2bb2c.com/api/webhook` with the
      `webhook_verify_token` from the account in the UI.
- [ ] Nightly DB backup cron is in place and the first run succeeded.

---

## 9. Troubleshooting

| Symptom | Likely cause / fix |
|---|---|
| `nginx: [emerg] cannot load certificate` on first boot | The real vhost was in place before certbot issued the cert. Follow the swap dance in §3.3. |
| 502 from nginx | App container is unhealthy or still booting. `docker compose ... logs app`. |
| `default_admin` cannot log in | The block was removed/edited *after* the users table was seeded — that's expected. Use the real admin you created. To recover access, use `psql` to clear the users table (last resort). |
| WhatsApp webhooks failing | Verify `webhook_verify_token` (set per-account in the UI) matches Meta, and `/api/webhook` is reachable: `curl -i 'https://support.banglab2bb2c.com/api/webhook?hub.mode=subscribe&hub.verify_token=YOUR&hub.challenge=ping'`. |
| TLS cert never renews | `docker compose ... logs certbot`. Common cause: port 80 isn't reachable from the internet, so the ACME challenge fails. |
| App can't reach DB | Confirm `[database] password` in `config.toml` matches `POSTGRES_PASSWORD` in `.env`. Both have to change together. |

---

## 10. File reference

| Path | Purpose |
|---|---|
| `docker/Dockerfile.prod` | Multi-stage build, alpine runtime, non-root, no calling deps. |
| `docker/docker-compose.prod.yml` | App + Postgres + Redis + Nginx + Certbot. |
| `docker/nginx/nginx.conf` | Base nginx config (gzip, buffers, log format). |
| `docker/nginx/conf.d/support.banglab2bb2c.com.conf` | HTTPS vhost with proxy + security headers. |
| `docker/nginx/conf.d/support.banglab2bb2c.com.init.conf` | Bootstrap HTTP-only vhost for the first cert issuance. |
| `docker/config.prod.example.toml` | Production config template. |
| `docker/.env.prod.example` | Production env template. |
