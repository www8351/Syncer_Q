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

On top of execution it ships a complete **trading journal**, a **Pandas-powered analytics engine**, **copy trading** with risk-managed fan-out, **TradingView signal ingestion**, **Stripe billing**, and a hybrid deploy: **SPA on Vercel + hardened backend on a single VPS** (Nginx WAF + TLS, GitHub Actions CI/CD).

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
| 📈 **Observability** | `/metrics` endpoint (Prometheus-format), SSE system-health stream, dependency scanning, daily DB backups |
| 🌍 **i18n + Mobile** | 4 languages with RTL/LTR, theme engine (light/dark + 10 accents), responsive slide-out nav |

---

## 🧱 Tech Stack

**Frontend** — React 19 · Vite 7 · Tailwind CSS v4 · shadcn/ui (Radix) · Framer Motion · Recharts · lightweight-charts · React Three Fiber · Zustand · TanStack Query · wouter · react-i18next

**Backend** — Node.js + Express 5 · TypeScript · Drizzle ORM · PostgreSQL 15 · WebSocket (`ws`) · Passport (local) · express-session + connect-pg-simple · Zod

**Services** — Python FastAPI + Pandas (analytics) · Stripe · Gmail API · OpenAI · Google Identity Services (Sign-In)

**Infra** — Vercel (frontend SPA) · single VPS via Docker Compose (multi-stage) · Nginx (WAF + TLS, Let's Encrypt) · GitHub Actions CI/CD (SSH deploy)

---

## 🏗️ Architecture

```mermaid
flowchart TD
    U[Browser] -->|static SPA| VER[Vercel · React 19 SPA]
    VER -->|cross-origin HTTPS · VITE_API_URL| NG[VPS · Nginx Ingress / WAF + TLS]
    TV[TradingView Webhook] -->|HMAC SHA-256| NG
    NG --> API[Node.js + Express API]
    API --> PG[(PostgreSQL 15 · Drizzle)]
    API -->|proxy /analytics| PY[FastAPI + Pandas]
    PY --> PG
    API <-->|WS / fast-poll| BRK[Brokers: Tradovate · TopstepX · Rithmic]
    API --> RISK[Risk Enforcer → auto-flatten]
    API --> COPY[Copy Engine + Reconciliation Daemon]
    API --> STR[Stripe Billing]
```

> **Deploy topology:** the SPA is served by **Vercel** (`syncer-q.vercel.app`); it calls the **VPS backend** cross-origin via `VITE_API_URL`. CORS allows the Vercel origin (`FRONTEND_ORIGINS`) and the session cookie is `SameSite=None; Secure` (`CROSS_SITE_COOKIES=true`).

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
├── infra/vps/           # Single-VPS scripts: provision.sh (hardening), deploy.sh
├── nginx/               # Ingress / WAF config + TLS bootstrap
├── migrations/          # SQL migrations
├── vercel.json          # Vercel SPA config (framework vite, dist/public, SPA rewrite)
├── Dockerfile           # Multi-stage app image
├── docker-compose.yml       # Backend stack (5 services: postgres + app + analytics + nginx + certbot)
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

**Hybrid split:** the React SPA is hosted on **Vercel** (`https://syncer-q.vercel.app`); the Express + Postgres backend runs on a **single VPS** as a 5-service Docker Compose stack (`postgres → vertex-app + analytics → ingress (Nginx) → certbot`). The SPA calls the backend cross-origin via `VITE_API_URL`; everything but the Nginx ingress (host 80/443) stays on the internal network.

### A · Frontend → Vercel

In the Vercel project (Settings):
- **Root Directory** = `Vertex_Command-main` (the app lives in the nested subdir; Vercel handles monorepos natively).
- **Environment Variable** (Production + Preview): `VITE_API_URL=https://<vps-backend-host>` (e.g. `https://api.syncer-q.com`).
- Framework auto-detected as **vite**; build command + output dir come from `vercel.json` (`npx vite build` → `dist/public`, with an SPA rewrite of `/(.*)` → `/index.html`).

### B · Backend → single VPS

**1 · Provision** (fresh Debian/Ubuntu, as root): `infra/vps/provision.sh` (UFW, fail2ban, SSH hardening, Docker + Compose).

**2 · Configure `.env`**
```bash
cp .env.example .env
openssl rand -hex 32     # SESSION_SECRET, and CREDENTIALS_ENCRYPTION_KEY (must be 64 hex)
```
Mandatory: `POSTGRES_PASSWORD`, `SESSION_SECRET`, `CREDENTIALS_ENCRYPTION_KEY`, `VERTEX_DOMAIN`, `CERTBOT_EMAIL`.
**Cross-origin (required for the Vercel SPA):** `FRONTEND_ORIGINS=https://syncer-q.vercel.app`, `CROSS_SITE_COOKIES=true`, `ALLOW_VERCEL_PREVIEWS=true` (optional), `GOOGLE_CLIENT_ID=<oauth-web-client-id>`. See `.env.example` for the full list.

**3 · Build & start**
```bash
docker compose build
docker compose up -d                          # 5-service stack (publishes host 80/443)
docker compose --profile ssl up -d certbot    # real Let's Encrypt TLS (after DNS points here)
```
The ingress self-signs a 30-day bootstrap cert on first start, so HTTPS works immediately; the `certbot` profile replaces it. Migrations run on boot via `entrypoint.sh` (`drizzle-kit push`).

**4 · Verify**
```bash
docker compose ps                                                            # all → healthy
docker compose exec vertex-app wget -qO- http://127.0.0.1:5000/api/health    # {"services":{"database":true}}
curl -k https://localhost/api/health                                         # 200 through Nginx
```

### C · Google Sign-In

Add `https://syncer-q.vercel.app` to **Authorized JavaScript origins** in Google Cloud Console (GIS ID-token flow needs no redirect URI). See `GOOGLE_SIGNIN_SETUP.md`.

### CI/CD

**GitHub Actions** (`.github/workflows/production.yml`): type-check + dependency/secret audit → Docker buildx validation → (manual, gated) SSH deploy to the VPS running `infra/vps/deploy.sh` with rolling restart, health-gated **automatic rollback**, and image pruning. Deploy is gated behind `workflow_dispatch` + `vars.DEPLOY_ENABLED=='true'`.

---

## 📄 License

[MIT](LICENSE) © Vertex Command

<div align="center">

**Built for traders who refuse to babysit a dozen dashboards.**

</div>
