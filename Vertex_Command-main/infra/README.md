# `infra/` — Future Microservices Scaffolding (NOT production)

> **Read this before touching anything in `infra/`.**

This directory holds an **experimental, future microservices architecture**. It is **NOT wired into the application code** and **must NOT be used for production**.

## What production actually uses

The live production stack is the **ROOT monolith** at [`../docker-compose.yml`](../docker-compose.yml) — this is **Path A**, the architecture the application code is actually connected to:

```
postgres → vertex-app (Node) + analytics (Python/FastAPI) → prometheus + grafana → ingress (nginx) → certbot (ssl profile)
```

Deploy it from the project root:

```bash
cp .env.example .env      # fill in required keys
docker compose build
docker compose up -d
```

See the root [`README.md`](../README.md) (Deployment section) and [`MIGRATION_CHECKLIST.md`](./MIGRATION_CHECKLIST.md).

## What's in here (and why it's NOT live)

`docker-compose.microservices.yml` describes a different topology:

| Component | Status |
|---|---|
| `go-routing-engine` (Go, low-latency signal fan-out) | **Not connected** — the app does NOT publish to it |
| `redis` (Pub/Sub for master→follower signals) | **Not connected** — the app uses no Redis |
| `nodejs-api`, `frontend`, `nginx-proxy` | Separate split-service builds — superseded by the monolith |

The app's copy-trading / signal pipeline runs **in-process inside `vertex-app`** (see `server/copy-trading-engine.ts`, `server/signal-routes.ts`), not through Redis or the Go engine. This folder is kept as a documented roadmap for a future split into microservices.

## Why the compose file is renamed

It was renamed from `docker-compose.yml` → **`docker-compose.microservices.yml`** on purpose: a bare `docker compose up` inside `infra/` now finds **no default compose file**, so this experimental stack **cannot be launched by accident**. Both stacks publish host ports **80/443**, so they are mutually exclusive — only one can run at a time.

To run the microservices stack **intentionally** (experimentation only):

```bash
cd infra
docker compose -f docker-compose.microservices.yml config   # requires REDIS_PASSWORD, WEBHOOK_SECRET, etc.
docker compose -f docker-compose.microservices.yml up
```

## Still-relevant shared assets

These `infra/` subfolders ARE consumed by the **root** production stack (mounted as volumes in `../docker-compose.yml`):

- `infra/prometheus/` — Prometheus scrape config + alert rules
- `infra/grafana/` — Grafana datasource/dashboard provisioning + the 14-panel dashboard
- `infra/vps/` — VPS provisioning (`provision.sh`) and deploy (`deploy.sh`) scripts
