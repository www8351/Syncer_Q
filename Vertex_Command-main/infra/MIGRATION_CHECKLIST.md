# Vertex Command — Production Deployment Checklist (Path A)

Step-by-step guide for deploying the **ROOT monolith stack** to a dedicated server / VPS.

> **Path A is the production target.** This checklist deploys the root `docker-compose.yml`
> (`postgres → vertex-app + analytics → prometheus + grafana → ingress → certbot`), the stack
> the application code is actually wired to.
>
> The microservices stack in `infra/docker-compose.microservices.yml` (Go engine + Redis) is
> **future scaffolding — not wired to the app and not deployed here.** See `infra/README.md`.
>
> **Repo layout:** the git root is a thin wrapper; the application and `docker-compose.yml` live in
> the `Vertex_Command-main/` subdirectory. All `docker compose` commands below run from there
> (referred to as the *project root*).

---

## Phase 1: Server Provisioning

### 1.1 Server Requirements
- [ ] Ubuntu 22.04 LTS / Debian 12 (or newer) dedicated server / VPS
- [ ] Minimum specs: 4 vCPU, 8 GB RAM, 80 GB SSD
- [ ] Static public IPv4 address
- [ ] SSH access with key-based authentication
- [ ] Firewall will allow only SSH + 80 + 443

### 1.2 Harden the box
- [ ] Run `infra/vps/provision.sh` as root on the fresh server. It creates the `vertex` deploy
      user, hardens SSH (Ed25519 only, root login off, non-standard port), configures UFW
      (deny-all + SSH/80/443), fail2ban, kernel sysctl hardening, and installs Docker Engine +
      the Compose plugin.

---

## Phase 2: Docker Engine

- [ ] `provision.sh` installs Docker + Compose. Verify:
```bash
docker --version
docker compose version
```
- [ ] Confirm the `vertex` user can run Docker (`docker ps` without sudo).

---

## Phase 3: DNS

- [ ] Point an A record for your apex domain (and `www`) at the server's public IP.
```bash
dig +short yourdomain.com
dig +short www.yourdomain.com   # both should return the server IP
```
> DNS propagation can take up to 48 h. Wait for resolution before requesting TLS certs (Phase 6).

---

## Phase 4: Configure the Application

### 4.1 Clone
```bash
# As the deploy user
cd /home/vertex
git clone <repository-url> vertex-command
cd vertex-command/Vertex_Command-main      # <-- project root (where docker-compose.yml lives)
```

### 4.2 Environment (`.env`)
```bash
cp .env.example .env
nano .env
```
**Required** (stack/app will not start without these):
- [ ] `POSTGRES_PASSWORD` — strong password for the containerized Postgres
- [ ] `SESSION_SECRET` — `openssl rand -hex 32`
- [ ] `CREDENTIALS_ENCRYPTION_KEY` — `openssl rand -hex 32` (must be exactly 64 hex / 32 bytes; the app aborts on boot otherwise)

**Required for production HTTPS:**
- [ ] `VERTEX_DOMAIN` — your domain (used by Grafana + certbot)
- [ ] `CERTBOT_EMAIL` — Let's Encrypt registration email

**Optional** (feature-gated; safe to leave blank): `SIGNAL_WEBHOOK_SECRET`, broker keys
(`TOPSTEPX_*`, `TRADOVATE_*`), `STRIPE_*`, `OPENAI_API_KEY`, `GRAFANA_ADMIN_PASSWORD`.
See `.env.example` for the full annotated list.

> **Database:** Postgres is **containerized** by the root compose (the `postgres` service, `pgdata`
> volume). You do **not** provision an external DB and you do **not** run `npm run db:push` by hand —
> `entrypoint.sh` runs `drizzle-kit push` automatically on every `vertex-app` boot.

---

## Phase 5: Build & Start

### 5.1 Build images
```bash
docker compose build          # Node (vertex-app), Python (analytics), Nginx (ingress)
```

### 5.2 Start the stack
```bash
docker compose up -d
```
This brings up `postgres → vertex-app + analytics → prometheus + grafana → ingress`. The ingress
self-signs a 30-day bootstrap TLS cert on first start, so HTTPS is available immediately on 443.

> On boot, `entrypoint.sh` waits for Postgres, runs `drizzle-kit push` (creates/syncs the schema),
> then starts the Node server. First boot takes ~40 s (migrations + seeding) before the app listens —
> this is covered by the healthcheck `start_period`.

### 5.3 Confirm health
```bash
docker compose ps     # postgres, vertex-app, analytics, prometheus, grafana → "healthy"; ingress "running"
```

---

## Phase 6: Real TLS (Let's Encrypt)

Once DNS resolves to the server:
```bash
docker compose --profile ssl up -d certbot
```
The certbot service obtains/renews a real certificate into the shared `ssl_certs` volume (cert name
`vertex`) via the webroot challenge and auto-renews every 12 h. The ingress picks it up on reload,
replacing the self-signed bootstrap cert.

Verify:
```bash
curl -I https://yourdomain.com/api/health      # 200, real cert
```

---

## Phase 7: Smoke Tests

```bash
# App health (DB-backed) — direct on the container
docker compose exec vertex-app wget -qO- http://127.0.0.1:5000/api/health
#   → {"status":"healthy"|"degraded","services":{"database":true, ...}}

# Prometheus metrics (served on the container; the ingress denies /metrics externally by design)
docker compose exec vertex-app wget -qO- http://127.0.0.1:5000/metrics | head

# Analytics health
docker compose exec analytics python -c "import urllib.request as u; print(u.urlopen('http://127.0.0.1:8100/health').read())"

# Through the Nginx ingress (TLS)
curl -k https://localhost/api/health          # 200, routed to vertex-app
curl -k https://localhost/                     # 200, React SPA

# Observability
#   Grafana:    https://yourdomain.com/grafana/   (admin / GRAFANA_ADMIN_PASSWORD)
#   Prometheus: internal only (scrapes vertex-app + analytics)
```

> **Local / non-80-443 hosts:** publish on alternate ports with
> `HTTP_PORT=8080 HTTPS_PORT=8443 docker compose up -d` and test against `https://localhost:8443`.

---

## Phase 8: Post-Deploy

- [ ] Log in, confirm the dashboard loads and live data streams (SSE).
- [ ] If using Stripe, update the webhook endpoint to `https://yourdomain.com/api/v1/webhook` (or the billing webhook path) and set `STRIPE_WEBHOOK_SECRET`.
- [ ] Confirm daily `pg_dump` backups are scheduled (logged by `vertex-app` on boot).
- [ ] Check Grafana dashboards are populating from Prometheus.

> **Known issue — restarts over an existing DB:** `vertex-app` currently crash-loops on a *restart/redeploy*
> against an already-initialized database, because the Postgres rate-limit store
> (`@acpr/rate-limit-postgresql`, `server/index.ts`) re-runs its `init` migration and hits
> `relation "unique_session_key" already exists`. **Fresh first deploys are unaffected.** Until the
> app-side fix lands, a redeploy needs either a one-off cleanup of the rate-limit tables or the store's
> init made idempotent. Track before relying on rolling restarts in production.

---

## Phase 9: Rollback

```bash
# Stop the stack (keep data)
docker compose down

# Stop and wipe ALL data (Postgres, Prometheus, Grafana volumes)
docker compose down -v

# Roll back to a previous image/commit
git checkout <previous-tag> && docker compose up -d --build
```

---

## Maintenance

```bash
docker compose logs -f vertex-app          # tail app logs
docker compose ps                          # service health
docker compose pull && docker compose up -d   # update base images (postgres/prometheus/grafana)
docker compose --profile ssl up -d certbot     # force a cert renewal check
```
