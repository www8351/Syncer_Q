<div align="center">

# ⚡ VERTEX COMMAND

### Professional Prop-Trading Command Center

*Manage dozens of funded trading accounts across multiple prop firms — with live broker streaming, automated risk enforcement, copy trading, and a full trading journal.*

[![Node](https://img.shields.io/badge/Node-24.x-339933?logo=node.js&logoColor=white)](https://nodejs.org)
[![React](https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=black)](https://react.dev)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.6-3178C6?logo=typescript&logoColor=white)](https://www.typescriptlang.org)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15-4169E1?logo=postgresql&logoColor=white)](https://www.postgresql.org)
[![Drizzle](https://img.shields.io/badge/Drizzle-ORM-C5F74F)](https://orm.drizzle.team)
[![Stripe](https://img.shields.io/badge/Stripe-Billing-635BFF?logo=stripe&logoColor=white)](https://stripe.com)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](#-license)

</div>

---

## 📖 Overview

**Vertex Command** is a full-stack SaaS platform built for serious prop-firm traders who juggle many funded accounts at once. It connects directly to **Tradovate**, **TopstepX (ProjectX)**, and **Rithmic**, streams live positions and equity, and automatically enforces drawdown rules — flattening accounts at market the moment a breach is detected.

On top of execution it ships a complete **trading journal**, a **Pandas-powered analytics engine**, **copy trading** with risk-managed fan-out, **TradingView signal ingestion**, **Stripe billing**, and a hardened production stack (Nginx WAF, Prometheus + Grafana, CI/CD to a zero-trust VPS).

The UI is **multi-language** (Hebrew default with full RTL, plus English, Arabic, Spanish).

> **Tagline:** *Your accounts, one cockpit. Risk enforced before you can blink.*

---

## ✨ Features

| Domain | Highlights |
|---|---|
| 🔌 **Broker Integrations** | Real API clients for Tradovate, TopstepX, Rithmic — auth, account discovery, Market/Limit/Stop orders, live WebSocket + fast-poll streaming |
| 🛡️ **Automated Risk Enforcement** | Real-time equity ticks, HWM + trailing/static drawdown, breach detection at 80/90/100%, **emergency auto-flatten** with idempotency lock + audit trail |
| 👥 **Copy Trading** | Master→follower replication, proportional/fixed sizing, symbol filters, unified risk pipeline, orphan detection, reconciliation daemon, panic flatten |
| 📓 **Trading Journal** | CRUD entries, psychology tracking, calendar, trade replay, CSV import/export, tax reports, shareable public reports, playbooks/strategies |
| 📊 **Analytics Engine** | FastAPI + Pandas microservice: Profit Factor, Expectancy, R-multiples, MAE/MFE, equity curve, streaks, latency/slippage analysis |
| 📡 **Signal Ingestion** | HMAC-SHA256 authenticated TradingView webhooks → risk-checked order fan-out, atomic dedup, dynamic symbol mapping |
| 🏛️ **Prop-Firm Rule Engine** | Tier-aware rules per firm (TopstepX/Tradovate/Rithmic), 40% consistency rule, drawdown math, "what to trade today" priority scoring |
| 💳 **Billing & Plans** | Stripe subscriptions, 7-day trial, 4 tiers, per-plan feature gating + account limits |
| 🔐 **Security** | AES-256-GCM credential encryption, TOTP 2FA, Helmet CSP, CSRF double-submit, rate limiting, account lockout, CSV-injection guard |
| 📈 **Observability** | Prometheus metrics, 14-panel Grafana dashboard, SSE system-health stream, dependency scanning, daily DB backups |
| 🌍 **i18n + Mobile** | 4 languages with RTL/LTR, theme engine (light/dark + 10 accents), responsive slide-out nav |

---

## 🧱 Tech Stack

**Frontend** — React 19 · Vite 7 · Tailwind CSS v4 · shadcn/ui (Radix) · Framer Motion · Recharts · lightweight-charts · React Three Fiber · Zustand · TanStack Query · wouter · react-i18next

**Backend** — Node.js + Express 5 · TypeScript · Drizzle ORM · PostgreSQL 15 · WebSocket (`ws`) · Passport (local) · express-session + connect-pg-simple · Zod

**Services** — Python FastAPI + Pandas (analytics) · Go routing engine (low-latency signal fan-out) · Stripe · Gmail API · OpenAI

**Infra** — Docker (multi-stage) · Nginx (WAF + TLS) · Prometheus · Grafana · GitHub Actions CI/CD · Redis Pub/Sub

---

## 🏗️ Architecture

```mermaid
flowchart TD
    U[Browser · React 19 SPA] -->|HTTPS| NG[Nginx Ingress / WAF + TLS]
    TV[TradingView Webhook] -->|HMAC SHA-256| NG
    NG --> API[Node.js + Express API]
    API --> PG[(PostgreSQL 15 · Drizzle)]
    API -->|proxy /analytics| PY[FastAPI + Pandas]
    PY --> PG
    API <-->|WS / fast-poll| BRK[Brokers: Tradovate · TopstepX · Rithmic]
    API --> RISK[Risk Enforcer → auto-flatten]
    API --> COPY[Copy Engine + Reconciliation Daemon]
    API -->|/metrics| PROM[Prometheus] --> GRAF[Grafana]
    API --> STR[Stripe Billing]
```

**Data flow in one line:** broker streams → equity tick processor → rule engine → on breach the risk-enforcer cancels orders + closes positions at market, writes an audit trail, and pushes a Discord/Telegram webhook.

---

## 🚀 Quick Start (Local Dev)

### Prerequisites
- **Node.js 20+** (24.x recommended)
- **Docker** (for the dev PostgreSQL container)

### 1. Install dependencies
```bash
npm install
```

### 2. Configure environment
```bash
cp .env.example .env
```
Minimum required keys in `.env`:
```env
DATABASE_URL=postgres://vertex:vertex@localhost:5433/vertex_command
CREDENTIALS_ENCRYPTION_KEY=<64 hex chars / 32 bytes>   # openssl rand -hex 32
SESSION_SECRET=<random string>
SIGNAL_WEBHOOK_SECRET=<random string>
```

### 3. Start the database
```bash
docker compose -f docker-compose.dev.yml up -d
```
> Spins up `postgres:15-alpine` on **port 5433** (mapped from container 5432) with a healthcheck.

### 4. Push the schema *(first run only)*
```bash
npm run db:push
```

### 5. Run the dev server
```bash
npm run dev
```
App serves on **http://localhost:5000** (Express + Vite middleware — API and SPA on one port).

| Endpoint | Purpose |
|---|---|
| `GET /api/health` | DB + Stripe health (503 if unhealthy) |
| `GET /metrics` | Prometheus metrics |

---

## 📜 NPM Scripts

| Script | Action |
|---|---|
| `npm run dev` | Dev server (`tsx server/index.ts`, NODE_ENV=development) |
| `npm run dev:client` | Vite client only on port 3003 |
| `npm run build` | Production build (`script/build.ts`) |
| `npm start` | Run production bundle (`node dist/index.cjs`) |
| `npm run check` | TypeScript type check (`tsc`) |
| `npm run db:push` | Push Drizzle schema to the database |

---

## 📁 Project Structure

```
.
├── client/              # React 19 SPA (pages, components, hooks, stores, i18n)
├── server/              # Express API, broker clients, engines, routes
│   ├── index.ts             # Bootstrap (CORS, sessions, routes, startup tasks)
│   ├── routes.ts            # Core API + auth
│   ├── encryption.ts        # AES-256-GCM at rest
│   ├── rule-engine.ts       # Account status / drawdown / priority
│   ├── risk-enforcer.ts     # Emergency auto-flatten on breach
│   ├── copy-trading-engine.ts
│   ├── tradovate-* / topstepx-* / rithmic-*   # Broker clients + streaming
│   └── ...
├── shared/              # Drizzle schema (core, integrations, billing, copy, journal)
├── analytics/           # Python FastAPI + Pandas microservice
├── infra/               # Production compose, Go engine, Nginx, Prometheus, Grafana, VPS scripts
├── nginx/               # Ingress / WAF config + TLS bootstrap
├── migrations/          # SQL migrations
├── Dockerfile           # Multi-stage app image
├── docker-compose.yml       # Full production stack (7 services)
└── docker-compose.dev.yml   # Dev PostgreSQL only
```

---

## 🗄️ Data Model

**26 tables** across 5 Drizzle schema files:

- **Core** — users, firms, firm_tiers, accounts, withdrawals, balance_history, alerts, monthly_reports, audit_log, settings, risk_interventions, equity_ticks
- **Integrations** — integration_providers, integration_connections, integration_accounts, imported_trades, sync_jobs, sync_logs
- **Billing** — plans, subscriptions, invoices, payment_methods, billing_events
- **Copy Trading** — copy_trading_groups, copy_trading_followers, copy_trading_orders, signal_mappings, processed_signals
- **Journal** — journal_entries, journal_psychology, journal_daily_summary, playbooks, trade_tags, trade_screenshots, …

---

## 💳 Plans

| Plan | Price | Accounts | Connections | Copy / Journal | Notable |
|---|---|---|---|---|---|
| **Free** | $0 | 2 | 1 | 0 / 0 | Basics |
| **Basic** | $29/mo | 5 | 3 | 3 / 1 | Integrations, exports, priority engine |
| **Pro** | $79/mo | 15 | 10 | 10 / 5 | AI chatbot, auto-sync |
| **Unlimited** | $199/mo | ∞ | 100 | ∞ / ∞ | Team support |

Plan features are enforced server-side via `requirePlanFeature()` (fail-closed); admin role bypasses all limits.

---

## 🔐 Security

- **AES-256-GCM** encryption for broker credentials + TOTP secrets at rest
- **2FA/TOTP** with authenticator apps + backup codes
- **Helmet + strict CSP**, **CSRF** double-submit cookie, **XSS** + **CSV-injection** sanitization
- **Rate limiting** (login 5/min, register 3/min, API 100/min) + progressive slow-down, backed by Postgres store
- **Account lockout** after 5 failed attempts (15-min), admin unlock
- **HMAC-SHA256** signed signal webhooks (constant-time compare)
- Daily automated **pg_dump backups**, dependency scanning on startup, security-event monitoring

---

## 🚢 Deployment

> **Production target: the ROOT monolith stack (Path A)** — the `docker-compose.yml` at the project root. The `infra/` directory is **future microservices scaffolding — not wired to the app, not for production** (see [`infra/README.md`](infra/README.md)).

Production ships as a **7-service Docker Compose** stack: `postgres → vertex-app + analytics → prometheus + grafana → ingress (Nginx) → certbot (ssl profile)`, fronted by an Nginx WAF with TLS 1.2/1.3, HSTS, rate-limit zones, and SSE passthrough. Only the ingress publishes host ports 80/443; everything else stays on the internal network.

### 1 · Configure `.env`
```bash
cp .env.example .env
# Fill the REQUIRED keys. Generate the secrets with:
openssl rand -hex 32     # SESSION_SECRET, and CREDENTIALS_ENCRYPTION_KEY (must be 64 hex)
```
Mandatory: `POSTGRES_PASSWORD`, `SESSION_SECRET`, `CREDENTIALS_ENCRYPTION_KEY`. `.env` is required in practice — `CREDENTIALS_ENCRYPTION_KEY` and `SIGNAL_WEBHOOK_SECRET` reach the app **only** through it. See `.env.example` for the full REQUIRED/OPTIONAL list.

### 2 · Build
```bash
docker compose build      # builds Node (app), Python (analytics), Nginx (ingress) images
```

### 3 · Start
```bash
docker compose up -d                          # full stack (publishes host 80/443)
docker compose --profile ssl up -d certbot    # real Let's Encrypt TLS (after DNS points at the host)
```
The ingress self-signs a 30-day bootstrap cert on first start, so HTTPS works immediately; the `certbot` profile replaces it with a real certificate. Schema migrations run automatically on boot via `entrypoint.sh` (`drizzle-kit push`).

> **Dev box where 80/443 are taken?** Override the published ports — `HTTP_PORT=8080 HTTPS_PORT=8443 docker compose up -d`.

### 4 · Verify
```bash
docker compose ps                                                            # all → healthy
docker compose exec vertex-app wget -qO- http://127.0.0.1:5000/api/health    # {"services":{"database":true}}
curl -k https://localhost/api/health                                         # 200 through the Nginx ingress
```

CI/CD via **GitHub Actions**: type-check + dependency/secret audit → Docker buildx validation → SSH deploy to a hardened VPS with rolling restart, health-gated **automatic rollback**, and image pruning. Full server walkthrough: [`infra/MIGRATION_CHECKLIST.md`](infra/MIGRATION_CHECKLIST.md).

---

## 📄 License

[MIT](LICENSE) © Vertex Command

<div align="center">

**Built for traders who refuse to babysit a dozen dashboards.**

</div>
