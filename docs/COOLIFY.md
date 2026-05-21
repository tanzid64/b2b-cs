# Deploying to Coolify — `support.banglab2bb2c.com`

Coolify handles reverse proxy, TLS, builds, and resource lifecycle for you.
This guide assumes Coolify v4+ is already installed on a server with a public IP.

You have two reasonable shapes for this app on Coolify. Pick one:

| Option | What it is | When to pick |
|---|---|---|
| **A. Split (recommended)** | App as one Application; Postgres and Redis as separate Coolify-managed resources. | You want Coolify-handled DB backups, easy version bumps, and resource isolation. |
| **B. Bundled** | A single Coolify Application that includes app + Postgres + Redis via `docker-compose.coolify.yml`. | You want one thing to start/stop and don't care about Coolify-managed DB tooling. |

---

## 0. Prerequisites

- Coolify already installed and the server accessible at `coolify.your-host.com` (or your private URL).
- DNS A record: **`support.banglab2bb2c.com` → your Coolify server's public IP**. Coolify cannot issue a Let's Encrypt cert without this.
- Repo accessible to Coolify (a public GitHub repo, or a private one with a deploy key — Coolify walks you through this when you add the source).
- Two secrets ready (generate locally, then paste into Coolify):
  ```bash
  openssl rand -hex 32   # BANGLAB2BB2C_APP_ENCRYPTION_KEY
  openssl rand -hex 32   # BANGLAB2BB2C_JWT_SECRET
  openssl rand -base64 24  # POSTGRES_PASSWORD
  ```

---

## Option A — Split deployment (recommended)

### A.1 Provision Postgres

1. Coolify dashboard → **Project** → your project → **+ New Resource** → **PostgreSQL**.
2. Version: `17` (Alpine). Name: `banglab2bb2c-db`.
3. Set:
   - **Username**: `banglab2bb2c`
   - **Password**: paste the `openssl rand -base64 24` value
   - **Database**: `banglab2bb2c`
4. Click **Deploy**. Wait for the green dot.
5. Open the resource → **Connect** tab → copy the **internal** connection string. It'll look like:
   ```
   postgres://banglab2bb2c:****@<coolify-internal-name>:5432/banglab2bb2c
   ```
   You need the **hostname** part (`<coolify-internal-name>`) for the app's env vars.

### A.2 Provision Redis

1. **+ New Resource** → **Redis**.
2. Version: `7`. Name: `banglab2bb2c-redis`.
3. No auth needed (it lives on the private Coolify network). Click **Deploy**.
4. Note the internal hostname under **Connect**.

### A.3 Create the Application

1. **+ New Resource** → **Application** → **Public Repository** (or **Private**, depending on your repo).
2. Repository: paste the repo URL. Branch: `main`.
3. **Build Pack**: **Dockerfile**.
4. **Dockerfile location**: `docker/Dockerfile.prod`.
5. **Base directory**: `.` (repo root — the Dockerfile copies from there).
6. **Ports Exposes**: `8080`.

Click **Continue** but **don't deploy yet** — set env vars first.

### A.4 Domain + TLS

1. In the app's **Configuration** tab → **Domains** → add `https://support.banglab2bb2c.com`.
   Including the `https://` tells Coolify's Traefik to obtain a Let's Encrypt cert.
2. Save. Coolify will request the cert after the first deploy succeeds.

### A.5 Environment variables

Inside the app → **Environment Variables** → add:

| Key | Value | Notes |
|---|---|---|
| `BANGLAB2BB2C_APP_ENCRYPTION_KEY` | `<openssl rand -hex 32>` | **mark as secret** |
| `BANGLAB2BB2C_JWT_SECRET` | `<openssl rand -hex 32>` | **mark as secret** |
| `BANGLAB2BB2C_SERVER_ALLOWED_ORIGINS` | `https://support.banglab2bb2c.com` | |
| `BANGLAB2BB2C_DATABASE_HOST` | `<postgres internal hostname>` | from A.1 step 5 |
| `BANGLAB2BB2C_DATABASE_PORT` | `5432` | |
| `BANGLAB2BB2C_DATABASE_USER` | `banglab2bb2c` | |
| `BANGLAB2BB2C_DATABASE_PASSWORD` | `<the postgres password>` | **mark as secret** |
| `BANGLAB2BB2C_DATABASE_NAME` | `banglab2bb2c` | |
| `BANGLAB2BB2C_REDIS_HOST` | `<redis internal hostname>` | from A.2 |
| `BANGLAB2BB2C_REDIS_PORT` | `6379` | |
| `BANGLAB2BB2C_COOKIE_SECURE` | `true` | |
| `BANGLAB2BB2C_RATE_LIMIT_ENABLED` | `true` | |
| `BANGLAB2BB2C_RATE_LIMIT_TRUST_PROXY` | `true` | Coolify's Traefik sets `X-Forwarded-For`. |
| `BANGLAB2BB2C_DEFAULT_ADMIN_EMAIL` | `you@example.com` | only used on first boot |
| `BANGLAB2BB2C_DEFAULT_ADMIN_PASSWORD` | `<initial password>` | **mark as secret**; change after first login |

> **Why `BANGLAB2BB2C_*` prefix?** The app uses koanf's env provider — env vars
> prefixed `BANGLAB2BB2C_` and separated by `_` map onto TOML keys with `.` as
> the separator (e.g. `BANGLAB2BB2C_DATABASE_HOST` → `database.host`).

### A.6 Persistent storage for uploads

Inside the app → **Storages** → **+ Add**:
- **Type**: Volume
- **Name**: `app-uploads`
- **Mount path**: `/app/uploads`

This survives redeploys. Skip it only if you set `BANGLAB2BB2C_STORAGE_TYPE=s3` and the S3 keys.

### A.7 Healthcheck

Inside **Configuration** → **Healthcheck** (if Coolify offers it for your build pack):
- **Path**: `/health`
- **Port**: `8080`

The Dockerfile already declares a `HEALTHCHECK`, so this is belt-and-braces.

### A.8 Deploy

Click **Deploy**. First build takes ~3–6 minutes (frontend + Go compile). Watch the build logs in real-time from the Coolify UI. On success:

- App status: green.
- TLS cert visible under **Domains**.
- `https://support.banglab2bb2c.com/health` returns `{"status":"ok"}`.

### A.9 First login

Open the site, log in with the `DEFAULT_ADMIN_EMAIL` / `_PASSWORD` you set. Immediately:

1. Create a real admin user.
2. Delete the default admin (or change its password).
3. **Remove** `BANGLAB2BB2C_DEFAULT_ADMIN_PASSWORD` from Coolify env vars and redeploy — it won't be used again, but no point in leaving the credential around.

---

## Option B — Bundled deployment

If you'd rather ship the whole stack as one Coolify Application:

1. **+ New Resource** → **Application** → **Docker Compose**.
2. Repo + branch as in A.3.
3. **Docker Compose location**: `docker/docker-compose.coolify.yml`.
4. **Service to expose**: `app`. **Port**: `8080`.
5. Domain: same step as A.4.
6. Env vars — paste only the *secret* ones (Coolify reads the rest from the compose file). Required:

| Key | Value |
|---|---|
| `BANGLAB2BB2C_APP_ENCRYPTION_KEY` | secret |
| `BANGLAB2BB2C_JWT_SECRET` | secret |
| `POSTGRES_PASSWORD` | secret |
| `ADMIN_PASSWORD` | initial admin password |
| `ADMIN_EMAIL` | initial admin email |

7. Deploy.

The compose file uses Coolify's magic `${SERVICE_FQDN_APP}` placeholder for the
allowed-origins default, so you don't have to hardcode the domain.

---

## Updates

Push to `main` (or whichever branch you connected) and Coolify auto-rebuilds —
the **Auto Deploy on Push** toggle has to be on. Migrations run automatically
because the image's `CMD` includes `-migrate`.

To roll back: Coolify → app → **Deployments** tab → pick a previous successful
build → **Redeploy this deployment**.

---

## WhatsApp webhook

Once the domain is live, the webhook URL for Meta is:

```
https://support.banglab2bb2c.com/api/webhook
```

The `webhook_verify_token` is set per-account in the app's UI (Settings → Accounts → your account).

---

## Troubleshooting

| Symptom | Likely cause / fix |
|---|---|
| Deploy succeeds but domain returns "no available server" | Coolify's Traefik hasn't issued the cert yet. Check **Domains** in the app — there's usually a status. DNS not pointing to the Coolify host is the most common cause. |
| App crashes on boot with `failed to load config` | A required env var is missing. The minimum set is the table in §A.5. |
| `panic: dial tcp: lookup db ...` | `BANGLAB2BB2C_DATABASE_HOST` doesn't match the Postgres resource's internal hostname. Open the Postgres resource → **Connect** tab → grab the host. |
| `JWT secret must be at least 32 characters in production` | `BANGLAB2BB2C_JWT_SECRET` is too short. Use `openssl rand -hex 32` (64 chars). |
| Uploads vanish on redeploy | You forgot the volume mount (§A.6). |
| Login session immediately drops | Cookie `Secure` flag set but the domain isn't HTTPS (or you're testing without TLS). Make sure Coolify's Traefik is terminating HTTPS — it does by default once a domain is added with `https://`. |
| Rate-limit blocks legit users from one IP | Behind Coolify's Traefik all requests appear to come from the proxy unless `BANGLAB2BB2C_RATE_LIMIT_TRUST_PROXY=true` (set in §A.5). Verify it's there. |

---

## Optional — using a `config.toml` instead of env vars

If you prefer the TOML-file flow from `docs/PRODUCTION.md`:

1. Inside the app → **Storages** → **+ Add** → **File mount**.
2. **Mount path**: `/app/config.toml`. Paste your production TOML as the content.
3. Env vars still override anything in the file, so you can mix both.

Most teams find the env-var path simpler on Coolify — one screen, secrets are
masked, no file editing — but the choice is yours.
