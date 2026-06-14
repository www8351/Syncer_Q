# CLAUDE_MEMORY

Long-term AI memory: preferences, persistent rules, stack constraints, security posture.

---

## Communication

- Direct, concise. No pleasantries, no hedging, no filler.
- State outcomes plainly. If something failed or was skipped, say so.

## Technical stack (strict)

- **Frontend:** React 19, Vite 7, Tailwind v4, shadcn/ui (Radix), Zustand, TanStack Query, wouter, react-i18next.
- **Backend:** Node.js 24.x, Express 5, TypeScript 5.6, Drizzle ORM, PostgreSQL 15, `ws`, Passport (local), express-session + connect-pg-simple, Zod.
- **Services:** Python FastAPI + Pandas (analytics), Go engine (signal fan-out), Stripe, Gmail API, OpenAI.
- **Infra:** Docker multi-stage, Nginx WAF + TLS, Prometheus + Grafana, GitHub Actions CI/CD, Redis Pub/Sub.

## Architectural constraints

- API + SPA served on **one port** (5000) via Express + Vite middleware.
- Dev DB on port **5433** via `docker-compose.dev.yml`.
- Plan features enforced server-side, fail-closed (`requirePlanFeature()`); admin bypasses.
- Risk enforcement must stay idempotent (auto-flatten lock + audit trail).

## Security (enforce)

- AES-256-GCM for broker credentials + TOTP secrets at rest.
- HMAC-SHA256 (constant-time compare) for TradingView signal webhooks.
- Helmet + strict CSP, CSRF double-submit, rate limiting, account lockout, CSV-injection guard.
- Never commit secrets. Required env: `DATABASE_URL`, `CREDENTIALS_ENCRYPTION_KEY` (64 hex), `SESSION_SECRET`, `SIGNAL_WEBHOOK_SECRET`.

## Operating protocol

- Maintain the 5 lifecycle files autonomously. After each task: update STATUS + append PROGRESS; log architectural shifts in DECISIONS; stack changes in README; rule/persona changes here.
- Read lifecycle files before code changes to align with current state.
