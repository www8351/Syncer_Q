# VERTEX COMMAND - Trading Command Center

## Overview
Professional Hebrew-language (RTL) prop trading SaaS platform for managing multiple trading accounts across prop firms. Features multi-user authentication with email verification/password reset, account tracking, firm rule engine with multi-tier support, trailing drawdown, withdrawal management, balance history, alerts, trading priorities, analytics, integrations with trading platforms, billing/subscriptions, admin module, and data export.

## Architecture
- **Frontend**: React + Vite, Tailwind CSS v4, shadcn/ui, Framer Motion, Recharts
- **Backend**: Express.js with RESTful API
- **Database**: PostgreSQL with Drizzle ORM (21 tables across 3 schema files)
- **Auth**: express-session + connect-pg-simple (PostgreSQL sessions), bcryptjs, TOTP-based 2FA (otplib + qrcode)
- **Email**: Gmail API via Replit google-mail integration
- **Language**: Multi-language (Hebrew default, English, Arabic, Spanish) via react-i18next with RTL/LTR support
- **Trading Platform Integrations**: Real API clients for Tradovate, TopstepX, and Rithmic

## Key Files
### Shared (Schema)
- `shared/schema.ts` - Core schema (accounts, firms, firm_tiers, withdrawals, balance_history, alerts, monthly_reports, audit_log, settings, users, risk_interventions, equity_ticks)
- `shared/integrations-schema.ts` - Integration tables (integration_providers, integration_connections, integration_accounts, imported_trades, sync_jobs, sync_logs)
- `shared/billing-schema.ts` - Billing tables (plans, subscriptions, invoices, payment_methods, billing_events)
- `shared/copy-trading-schema.ts` - Copy trading tables (copy_trading_groups, copy_trading_followers, copy_trading_orders)
- `shared/journal-schema.ts` - Journal tables (journal_entries, journal_psychology, journal_daily_summary, trading_goals, shared_reports, journal_alerts, journal_alert_settings, playbooks, trade_tags, trade_screenshots)

### Server
- `server/db.ts` - Database connection (imports all 4 schema files)
- `server/storage.ts` - CRUD storage interface with all data methods
- `server/routes.ts` - Main API routes, auth, password reset
- `server/rule-engine.ts` - Account status, drawdown, trading priority computation
- `server/integrations-routes.ts` - Integration provider/connection CRUD, sync jobs, connect endpoints for Tradovate/TopstepX/Rithmic
- `server/tradovate-client.ts` - Tradovate REST API client (auth, account discovery, balance, Market/Limit/Stop orders)
- `server/tradovate-websocket.ts` - Tradovate WebSocket client (real-time position/order/balance streaming, heartbeat, reconnection, SSE broadcasting)
- `server/rithmic-websocket.ts` - Rithmic persistent WebSocket client (real-time position streaming with 2s fast-poll, heartbeat, reconnect, EventEmitter)
- `server/topstepx-streaming.ts` - TopstepX fast-poll streaming client (2s polling with smart-diff position detection, reconnect, EventEmitter)
- `server/futures-tick-sizes.ts` - Futures contract tick size mapping (ES=0.25, NQ=0.25, CL=0.01, etc.) for accurate slippage calculations
- `server/topstepx-client.ts` - TopstepX (ProjectX One) REST API client (auth via API key, account discovery, Market/Limit/Stop orders)
- `server/rithmic-client.ts` - Rithmic API client (auth, account discovery via protobuf WebSocket)
- `server/trading-routes.ts` - Trading API (place Market/Limit/Stop orders, get positions/orders via broker, SSE stream for all brokers, WebSocket management)
- `server/journal-routes.ts` - Trading journal API (CRUD, analytics, psychology, calendar, daily summaries, CSV import/export, tax reports, shared reports, market data endpoint)
- `server/playbook-routes.ts` - Playbook API (CRUD for trading strategies, custom tags, trade screenshots with stats per playbook)
- `server/market-data.ts` - Market data service: fetches historical OHLCV candles from Yahoo Finance (equities/futures) and CoinGecko (crypto fallback), with DB caching layer and symbol mapping for platform-specific symbols (MES→ES=F, MNQZ4→NQ=F, BTCUSD→BTC-USD, etc.)
- `server/copy-trading-engine.ts` - Copy trading engine with WebSocket-first + polling fallback (position monitoring, order replication, risk engine integration, advanced sizing, heartbeat monitor, panic flatten, orphaned trade detection)
- `server/copy-trading-risk-engine.ts` - Unified risk pipeline (restricted symbols, daily loss limit tracking, drawdown check, margin check, news embargo, slippage computation, order type mapping, error normalization)
- `server/copy-trading-routes.ts` - Copy trading API (groups CRUD, followers CRUD, order log with pagination, flatten endpoint, orphaned trades detection, webhook/heartbeat endpoints, slippage stats, webhook token management)
- `server/billing-routes.ts` - Plans, subscriptions, invoices, plan change, cancel/resume
- `server/affiliate-routes.ts` - Affiliate/referral program (code generation, tracking, admin stats, reward management)
- `server/admin-routes.ts` - Admin-only user/billing/metrics/health endpoints
- `server/system-health-stream.ts` - System Health SSE streaming endpoint (admin-only, real-time health data every 5s via shared sampler, Stripe/OpenAI ping probes, circular error buffer capturing console.error/warn)
- `server/gmail.ts` - Gmail API client for verification + password reset emails (googleapis lazy-loaded)
- `server/help-routes.ts` - Help Center API (AI chat via OpenAI streaming, system status health checks with real DB/engine/broker probes) (OpenAI SDK lazy-loaded)
- `server/stripeClient.ts` - Stripe SDK lazy-loaded via promise-memoized dynamic import
- `server/webhookHandlers.ts` - Stripe webhook handler (uses type-only Stripe import for zero runtime cost)
- `server/equity-tick-processor.ts` - Real-time equity tick processing: records granular equity changes, computes HWM/trailing stop for both static and trailing drawdown types, detects breach, creates drawdown alerts at 80%/90%/breach thresholds, triggers auto-flatten on breach, auto-cleanup of old ticks (30-day retention)
- `server/risk-enforcer.ts` - Automated risk intervention service: on drawdown breach, executes emergency flatten (cancel all working orders + close all positions at market) via Tradovate/TopstepX broker APIs. Features in-memory idempotency lock (Set<accountId>) with 60s cooldown, full audit trail to risk_interventions table, and alert generation. Exports flattenAccount/flattenTradovate/flattenTopstepX for external use by reconciliation daemon.
- `server/provider-core.ts` - Shared provider utilities extracted from copy-trading-engine: authenticateProvider, getProviderPositions, resolveMasterConnection, resolveFollowerConnection, resolveMasterAccountInfo, resolveFollowerAccountInfo. Consumed by both copy-trading-engine and reconciliation-daemon to avoid circular dependencies.
- `server/reconciliation-daemon.ts` - Orphaned position recovery daemon: runs on boot (10s delay) and on WS reconnection. Iterates active copy groups, fetches master positions via REST, detects follower orphans when master is flat, auto-flattens via risk-enforcer, writes sync_recovery audit trail to risk_interventions table. Dispatches webhook notifications (Discord/Telegram) for sync_recovery and flatten_failed events.
- `server/webhook-dispatcher.ts` - Webhook notification dispatcher: Discord embed + Telegram HTML formatters, async fire-and-forget via Promise.allSettled, reads from settings.notificationPreferences JSONB. Exports dispatchRiskIntervention() and testWebhookConfig(). Integrated into risk-enforcer and reconciliation-daemon.
- `server/trailing-drawdown.ts` - Trailing drawdown computation engine (evaluateTrailingDrawdown, evaluateAccountsBatch, computeTrailingDrawdownState)

### Analytics Microservice (`analytics/`)
- `analytics/main.py` - FastAPI application with 3 analytics endpoints: `/api/analytics/trade-metrics`, `/api/analytics/risk-summary`, `/api/analytics/copy-performance`. All endpoints require `user_id` (injected by Node.js gateway). Date validation, health check at `/health`.
- `analytics/metrics.py` - Vectorized Pandas computation engine: Profit Factor, Win Rate, Expectancy, R-Multiples (binned distribution), MAE (Maximum Adverse Excursion by side), MFE (Maximum Favorable Excursion), equity curve, daily stats, win/loss streaks, trade duration, symbol breakdown, hourly distribution, copy order latency/slippage analysis.
- `analytics/queries.py` - SQL query layer using SQLAlchemy `text()` with bound parameters. Queries: imported_trades, journal_entries, risk_interventions, copy_trading_orders. All enforce user_id tenant isolation.
- `analytics/db.py` - Async SQLAlchemy engine with connection pooling (5+5).
- `analytics/config.py` - Pydantic settings: `ANALYTICS_DATABASE_URL`, `ANALYTICS_LOG_LEVEL`, `ANALYTICS_MAX_TRADE_ROWS` (default 100k).
- `analytics/Dockerfile` - Multi-stage Python 3.11-slim build, non-root `analytics` user (uid 1001), 2 Uvicorn workers on port 8100.
- **Routing**: Node.js gateway at `/api/v1/analytics/:endpoint` (requireAuth + user_id injection) → proxies to `http://analytics:8100/api/analytics/...`. Nginx routes `/api/v1/analytics` through the Node gateway (not direct to Python). In production, all analytics traffic is authenticated.

### Client
- `client/src/App.tsx` - Router with auth gating (20+ authenticated routes + 2 public routes)
- `client/src/pages/Home.tsx` - Home page with instrument prices ticker (NQ, ES, GC, CL, RTY, YM), community stats, recommended firms
- `client/src/pages/GetStarted.tsx` - Step-by-step onboarding wizard (Choose Plan, Add Connection, Enable Accounts, Import Contract, Set Leader)
- `client/src/pages/InvestorShowcase.tsx` - Public investor showcase page at /investor (no auth required)
- `client/src/pages/Reports.tsx` - Tax reports + shareable performance reports
- `client/src/pages/PublicReport.tsx` - Public shared report view at /report/:token (no auth required)
- `client/src/components/AnalyticsDashboard.tsx` - Full analytics visualization: Equity Curve (AreaChart), R-Multiple Distribution (BarChart), Hourly P&L Distribution, MAE/MFE stats, daily stats, streaks, symbol breakdown table, risk intervention summary, copy trading latency/slippage/fill rate with latency distribution chart. Fetches from /api/v1/analytics/* gateway. Date range picker (7d/30d/90d/1y/all) + source toggle (imported/journal).
- `client/src/pages/Dashboard.tsx` - Main dashboard with KPIs, charts, alerts, priorities, analytics (AnalyticsDashboard + DrawdownEvaluator), settings
- `client/src/pages/AccountDetail.tsx` - Account detail with history chart, rules, drawdown
- `client/src/pages/TradingPriority.tsx` - Dedicated "What to Trade Today" page
- `client/src/pages/Integrations.tsx` - Provider cards, connection management, sync
- `client/src/pages/CopyTrading.tsx` - Copy trading groups, followers, order execution log
- `client/src/pages/Journal.tsx` - Journaling dashboard with P&L charts, win rate, drawdown, calendar, trade replay
- `client/src/pages/Trades.tsx` - Trade history page with stats cards (win rate, profit factor, avg win/loss) and full trade table
- `client/src/pages/DailyJournal.tsx` - Daily journal view with per-day breakdown (trades, wins, losses, P&L, commission)
- `client/src/pages/WeeklyJournal.tsx` - Weekly journal view with per-week breakdown
- `client/src/pages/StrategyManagement.tsx` - Strategy/playbook management with win/loss tracking per strategy
- `client/src/pages/ManageData.tsx` - Data management (auto-sync, CSV import, data history)
- `client/src/pages/Billing.tsx` - Plan comparison, subscription management, invoices
- `client/src/pages/Affiliates.tsx` - Affiliate program page (referral link, stats, history)
- `client/src/pages/Admin.tsx` - User list, subscription metrics, provider health, KPIs, affiliate stats
- `client/src/pages/SystemHealth.tsx` - Admin-only real-time system health dashboard with SSE streaming
- `client/src/pages/WebGL3D.tsx` - Interactive 3D scene page (React Three Fiber, TorusKnot, OrbitControls, WebGL error/context-loss handling, SSE telemetry HUD)
- `client/src/components/webgl/Scene.tsx` - 3D scene component (TorusKnot geometry, studio lighting, orbit controls, memory cleanup, real-time risk-driven visual mutations via Zustand)
- `client/src/components/webgl/WebGLErrorBoundary.tsx` - WebGL-specific error boundary with styled fallback UI
- `client/src/stores/telemetryStore.ts` - Zustand store for volatile trading telemetry (positions, balances, drawdown risk, daily P&L, account status)
- `client/src/hooks/useTelemetryStream.ts` - SSE ingestion hook with exponential backoff reconnection, parses trading stream events into Zustand store
- `client/src/pages/AuthPage.tsx` - Login/Register with Hebrew RTL + Google Sign-In (GSI)
- `client/src/pages/Onboarding.tsx` - 5-step onboarding wizard (experience, account type, instruments, goals, referral)
- `client/src/hooks/useAuth.ts` - Auth hook (includes onboardingCompleted, avatarUrl)
- `client/src/hooks/useCurrency.tsx` - Global currency formatting context (USD/EUR/% toggle), used by all pages
- `client/src/hooks/useTheme.tsx` - Theme context provider: light/dark/system mode + 10 accent color presets, persisted to localStorage. Wraps entire app via ThemeProvider in App.tsx. CSS variables `--ring`, `--accent-brand`, `--accent-brand-light`, `--accent-brand-dark` are dynamically updated.
- `client/src/hooks/useTradingStream.ts` - SSE hook for real-time WebSocket state and trading data updates
- `client/src/components/WebhookSettings.tsx` - Webhook notification settings panel (Discord + Telegram configuration, test send, enable/disable toggle)
- `client/src/components/AccountDialogs.tsx` - Add/Edit account dialogs
- `client/src/components/BrokerConnectDialog.tsx` - Broker connection popup (Tradovate/TopStepX) with multi-step flow, uses safeBrokerFetch for error-safe API calls
- `client/src/components/FirmCombobox.tsx` - Firm selector with free-text

## Data Model (26 Tables)
### Core
- **users** - Auth (name, email, passwordHash, role, status, emailVerified, verificationToken, resetToken, googleId, avatarUrl, tradingExperience, accountType, instruments[], goals[], referralSource, onboardingCompleted)
- **firms** - Prop firm definitions with default rules
- **firm_tiers** - Account types per firm with tier-specific rules
- **accounts** - Trading accounts (balance, drawdown, status, dataSource, integrationConnectionId, notes)
- **withdrawals** - Withdrawal lifecycle tracking
- **balance_history** - Daily snapshots (equity, pnl, drawdown remaining, risk percent)
- **alerts** - System alerts with title, severity, resolution tracking
- **monthly_reports** - Monthly performance summaries with metadataJson (auto-generated from imported_trades during sync)
- **audit_log** - Action audit trail with ipAddress, userAgent
- **settings** - User preferences

### Integrations
- **integration_providers** - TopstepX, Tradovate, Rithmic, CSV Import
- **integration_connections** - User connections with status tracking and integrationMode (sync | full)
- **integration_accounts** - External accounts with linking to internal accounts
- **imported_trades** - Trade data from integrations
- **sync_jobs** - Sync job tracking with status
- **sync_logs** - Sync operation logs

### Billing
- **plans** - Free ($0), Basic ($29/mo), Pro ($79/mo), Unlimited ($199/mo) with feature flags + maxCopyTradingAccounts + maxJournalAccounts
- **subscriptions** - User subscription tracking
- **invoices** - Invoice history
- **payment_methods** - Stored payment methods
- **billing_events** - Billing event log

### Copy Trading
- **copy_trading_groups** - Master account, name, status, poll interval, settings
- **copy_trading_followers** - Follower accounts with multiplier, sizingMode (proportional | fixed), max position, symbol filters, enable toggle
- **copy_trading_orders** - Execution log (master/follower order refs, status, fill details, latency)

## Auth & Trial System
- Sessions stored in PostgreSQL via connect-pg-simple
- `requireAuth` middleware on all `/api/*` routes except `/api/auth/*` and `/api/billing/webhook`
- Trial middleware returns 402 when trial expired (bypassed for auth, billing, admin, public routes; admin role bypasses entirely)
- New users registered with `role: "user"` and auto-created 7-day trial subscription (status="trialing", trialEndsAt=now+7d)
- TrialGuard component in App.tsx checks `/api/billing/status` and shows paywall when expired
- All data queries filtered by `req.session.userId`
- Password reset via email with 60-minute token expiry

## Plan-Based Feature Enforcement
- `requirePlanFeature(feature)` middleware in `billing-routes.ts` gates routes by plan feature flags (fail-closed on errors)
- Admin role bypasses all plan limits
- Plans table has boolean feature flags: `hasIntegrations`, `hasAutoSync`, `hasExports`, `hasPriorityEngine`, `hasTeamSupport`, `hasAiChatbot`, `hasCopyTrading`
- Feature gating: integrations (Basic+), copy_trading (Basic+), ai_chatbot (Pro+), priority_engine (Basic+), exports (Basic+), team_support (Unlimited), auto_sync (Pro+)
- Account creation enforces `maxAccounts` per plan (Free: 2, Basic: 5, Pro: 15, Unlimited: unlimited)
- 403 responses include `{code: "plan_limit", feature, requiredPlan}` for client-side upgrade modal
- `PlanLimitModal` in Dashboard.tsx shows upgrade prompt with navigation to billing page
- Sidebar lock icons (🔒) in both desktop and mobile views for locked features
- i18n keys for plan limit messages in all 4 locales (he, en, ar, es)

## API Endpoints
### Auth
- POST `/api/auth/register`, `/api/auth/login`, `/api/auth/logout`
- GET `/api/auth/me`, `/api/auth/verify/:token`
- POST `/api/auth/resend-verification`, `/api/auth/forgot-password`, `/api/auth/reset-password`

### Core
- `/api/firms` - CRUD (returns embedded tiers)
- `/api/firm-tiers` - CRUD
- `/api/accounts` - CRUD with computed status
- `/api/withdrawals` - CRUD with lifecycle
- `/api/balance-history/:accountId` - Snapshots
- `/api/alerts` - CRUD + unread/read-all
- `/api/trading-priorities` - Priority engine
- `/api/monthly-reports` - Monthly reports (auto-generates from trades if needed)
- `/api/trades/monthly-pnl` - Aggregated P&L by month from imported trades (fallback: balance diff)
- `/api/audit-log`, `/api/settings`, `/api/export/:type`

### Integrations
- GET `/api/integrations/providers`, `/api/integrations/connections`
- POST `/api/integrations/connections` (create)
- POST `/api/integrations/connections/:id/test`, `/sync`, `/reconnect`
- DELETE `/api/integrations/connections/:id`
- GET `/api/integrations/connections/:id/accounts`
- POST `/api/integrations/accounts/:externalId/link`, `/unlink`
- GET `/api/integrations/sync-jobs`, `/api/integrations/sync-jobs/:id`, `/api/integrations/sync-logs`

### Copy Trading
- GET `/api/copy-trading/groups` - List user's copy groups with followers + master account
- GET `/api/copy-trading/groups/:id` - Group detail with enriched followers
- POST `/api/copy-trading/groups` - Create group (name, masterAccountId, pollIntervalMs)
- PATCH `/api/copy-trading/groups/:id` - Update group (status: active/paused, name, pollInterval)
- DELETE `/api/copy-trading/groups/:id` - Delete group + all followers/orders
- POST `/api/copy-trading/groups/:id/followers` - Add follower (multiplier, maxPositionSize, allowedSymbols)
- PATCH `/api/copy-trading/followers/:id` - Update follower settings
- DELETE `/api/copy-trading/followers/:id` - Remove follower
- GET `/api/copy-trading/groups/:id/orders` - Order execution log (paginated)
- GET `/api/copy-trading/engine-status` - Active polling groups count

### Billing
- GET `/api/billing/plans`, `/api/billing/subscription`, `/api/billing/invoices`
- GET `/api/billing/status` - Returns trial/subscription status (status, daysLeft, trialEndsAt, planName)
- POST `/api/billing/checkout`, `/api/billing/change-plan`, `/api/billing/cancel`, `/api/billing/resume`
- POST `/api/billing/portal`, `/api/billing/webhook`

### Admin (admin role only)
- GET `/api/admin/users`, `/api/admin/billing`, `/api/admin/metrics`, `/api/admin/integrations/health`

## Rule Engine (`server/rule-engine.ts`)
- `resolveRules(account, firm, tiers)` - Tier-first, then firm fallback
- `computeAccountStatus(account, rules)` - Returns: healthy, buffer_building, near_target, ready_to_withdraw, consistency_risk, drawdown_risk, violated, inactive
- `computeDrawdownInfo(account, rules)` - Static/trailing drawdown calculations
- `computeTradingPriority(account, rules)` - Score 0-100, recommendation: trade/light_trading/avoid/do_not_trade/ready_to_withdraw
- `recalculateAccountStatus(accountId)` / `recalculateAccountsForUser(userId)` - Batch recalculation
- `isAccountSyncEligible(account)` - Returns false for violated/inactive accounts (used by sync + WS streams)

## Broker Rules Validation (`server/broker-rules.ts`)
Pre-import validation checklist per broker. Accounts failing validation are skipped before entering the system.
- **TopstepX** (full rule set with tier resolution):
  - Hard checks: `account_active`, `status_valid`, `max_loss_limit` (trailing drawdown breach), `consistency_40pct`
  - Tier params auto-resolved: $50K (target $3K, daily $1K, max loss $2K, 5 contracts), $100K ($6K/$2K/$3K/10), $150K ($9K/$3K/$4.5K/15)
  - Policy rules (informational): no_overnight_positions (15:10 CT), permitted_products (CME/CBOT/NYMEX/COMEX), min_trading_days (5), professional_conduct
  - `resolveTopstepXTier()` matches tier by profitTarget → maxContracts → dailyLossLimit → trailingDrawdown
- **Tradovate**: `account_active`, `legal_status_valid`, `consistency_40pct`, plus informational: balance, accountType, marginAccountType
- **Rithmic**: `account_exists`, `rp_code_valid`, `name_not_burned`, `consistency_40pct`
- **40% Consistency Rule** applied across ALL brokers: topDayProfit / totalProfit must be ≤ 40%
- Central dispatcher: `validateBrokerAccount(providerKey, rawAccount)` routes to correct validator
- All discovery functions return `{ accounts, skipped }` — skipped includes validation details and reason

## Billing Plans
- **Free** ($0): 2 accounts, 1 connection, 0 copy/0 journal
- **Basic** ($29/mo): 5 accounts, 3 connections, 3 copy/1 journal, exports, priority engine
- **Pro** ($79/mo): 15 accounts, 10 connections, 10 copy/5 journal, AI chatbot, auto sync
- **Unlimited** ($199/mo): Unlimited accounts, 100 connections, unlimited copy/journal, team support

## Pages
- **Dashboard** (/) - KPIs, charts, account tables, filters, alerts, priorities, analytics, settings
- **Trading Priority** (/trading-priority) - Priority-sorted account cards with scores
- **Integrations** (/integrations) - Provider cards, connection management, sync
- **Copy Trading** (/copy-trading) - Copy groups, master/follower management, order execution log
- **Billing** (/billing) - Plan comparison, subscription, invoices
- **Admin** (/admin) - User management, metrics, provider health
- **Account Detail** (/account/:id) - Full account view with charts and rules
- **Auth Page** - Login/Register with verification + Google Sign-In
- **Onboarding** - 5-step wizard (experience, account type, instruments, goals, referral)

## Security & Production
- **API Versioning**: All routes under `/api/v1/` prefix, backward-compatible redirect from `/api/*`
- **AES-256-GCM Encryption**: `server/encryption.ts` for credentials and TOTP secrets at rest
- **2FA/TOTP**: `otplib` + `qrcode` for authenticator app setup, backup codes, encrypted storage
- **Helmet.js + CSP**: Strict Content Security Policy with whitelisted sources per environment
- **CSRF Protection**: Double-submit cookie pattern, `/api/v1/csrf-token` endpoint
- **XSS Sanitization**: Input sanitization on all user-provided data
- **CORS**: Whitelist-based origin policy derived from REPLIT_DOMAINS
- **Account Lockout**: 5 failed attempts → 15-minute lockout, admin unlock available
- **DDoS Protection**: `express-slow-down` progressive delays + `express-rate-limit` with PostgresStore persistence
- **Error Sanitization**: `safeErrorResponse()` utility strips internal details, returns generic Hebrew messages
- **Response Logging**: Only byte count logged (no PII), sensitive paths excluded
- **CSV Injection Protection**: `sanitizeCsvCell()` neutralizes formula injection in imports/exports
- **Dependency Scanning**: `scripts/audit.sh` runs on startup, results shown in admin panel
- **DB Backup**: Daily automated pg_dump backups, admin manual trigger, configurable retention
- **Security Event Monitoring**: `security_events` table, admin panel with filters/pagination
- Rate limiting on auth endpoints (login: 5/min, register: 3/min, password reset: 3/min)
- General API rate limit (100/min)
- Session secret enforced in production (fails startup if missing)
- Secure cookies in production (httpOnly, secure, sameSite)
- Trust proxy enabled for proper IP detection behind load balancer
- Health check at GET `/api/health` (returns DB + Stripe status, 503 if unhealthy)
- Deployment: autoscale target, build: `npm run build`, run: `node dist/index.cjs`

## Memory Optimization
- Heavy SDKs lazy-loaded via promise-memoized dynamic `import()`: OpenAI, googleapis, Stripe, QRCode, otplib
- Pattern: singleton promise cached on first call, race-safe (concurrent requests share the same promise)
- Map cleanup: API queue timestamps/stats capped, latency store stale-key sweep, equity map periodic sweep, orphan client sweeps in TopstepX/Tradovate, connectionAccountCache TTL+max-size
- TopstepX poll interval: 500ms → 2500ms to reduce API queue pressure
- Response body capture removed from logging middleware (was re-serializing all JSON responses)
- EventEmitter maxListeners set to 20 (prevents memory leak warnings)
- `/api/v1/system/memory` endpoint for runtime heap/Map diagnostics (auth bypass for debugging)
- Workflow runs with `NODE_OPTIONS='--max-old-space-size=256'`
- Baseline heap: ~52 MB (down from ~160 MB), well under 120 MB target

## Production Migration Infrastructure (`infra/`)
- `infra/docker-compose.yml` - Orchestrates all services: Go routing engine, Node.js API, Redis, frontend (Nginx), reverse proxy (Nginx), Certbot
- `infra/backend-go/` - Go-based low-latency routing engine with: TradingView webhook handler, WebSocket connection manager (persistent broker connections with auto-reconnect), Redis Pub/Sub subscriber for Master→Slave signal fan-out
- `infra/frontend/Dockerfile` - Multi-stage build: Vite build → Nginx static serving
- `infra/nodejs-api/Dockerfile` - Multi-stage build: npm build → Node.js production runtime
- `infra/nginx/` - Reverse proxy config with SSL/TLS (Let's Encrypt), WebSocket keep-alive (`proxy_read_timeout 86400s`), subdomain routing (`api.` → Go backend, `www.` → frontend)
- `infra/.env.template` - Environment variable template for production deployment
- `infra/MIGRATION_CHECKLIST.md` - Step-by-step migration guide (server provisioning, Docker setup, DNS, SSL, deployment, smoke tests, rollback)

### Docker / Production Containerization
- `Dockerfile` - Multi-stage build: Stage 1 (builder) installs all deps, builds Vite frontend + esbuild backend; Stage 2 (runner) copies dist/public assets, installs prod deps only, runs as unprivileged `nodeuser` (uid 1001) via tini init
- `docker-compose.yml` - Orchestrates 7 services: postgres → vertex-app + analytics → prometheus + grafana → ingress (Nginx) → certbot (ssl profile). Node app, analytics, prometheus, and grafana on internal network only (no host port exposure). Separate volumes for SSL certs, certbot webroot, prometheus_data, and grafana_data.
- `entrypoint.sh` - Startup sequence: pg_isready wait loop (30 retries), drizzle-kit push for schema sync, then exec node dist/index.cjs
- `.dockerignore` - Excludes node_modules, dist, .git, .local, .env, artifacts, nginx

### Nginx Ingress / WAF Layer (`nginx/`)
- `nginx/Dockerfile` - Nginx 1.27-alpine with openssl for self-signed cert bootstrap
- `nginx/nginx.conf` - Production reverse proxy config with:
  - **SSL/TLS**: TLS 1.2/1.3 only, strong cipher suite, OCSP stapling, session cache, auto-redirect HTTP→HTTPS
  - **Security headers**: HSTS (2yr preload), X-Content-Type-Options, X-Frame-Options DENY, X-XSS-Protection, Referrer-Policy, Permissions-Policy, Content-Security-Policy
  - **Rate limiting**: 3 zones — auth endpoints (5r/min, burst 3), API routes (30r/s, burst 50), global (60r/s, burst 30); 429 JSON error responses
  - **Payload limits**: 2MB max body, 16K body buffer, 8K header buffers, 50 connections/IP
  - **SSE passthrough**: Proxy buffering disabled for /api/v1/stream and /api/v1/system-health with 1hr read timeout
  - **Static asset caching**: /assets/ proxied with 30-day cache + immutable header
  - **Error pages**: Structured JSON 429/502/503/504 responses
- `nginx/ssl-init.sh` - Self-signed certificate bootstrapper (30-day, runs on first startup only)
- `nginx/certbot-renew.sh` - Let's Encrypt certificate acquisition via webroot challenge
- Certbot renewal container runs on `ssl` profile (`docker compose --profile ssl up certbot`), auto-renews every 12 hours

### VPS Provisioning & CI/CD
- `infra/vps/provision.sh` - Zero-trust VPS hardening script (run as root on fresh Debian 12 / Ubuntu 24.04 LTS):
  - Creates `vertex` deploy user with passwordless sudo
  - SSH hardening: non-standard port (default 2222), Ed25519 keys only, root login disabled, password auth disabled, 3 max auth tries
  - UFW firewall: deny-all default, allow SSH port + 80 + 443 only
  - fail2ban: SSH brute-force protection (3 attempts, 2h ban)
  - Docker Engine + Compose plugin installation
  - Kernel hardening (sysctl: SYN cookies, reverse-path filtering, ASLR, no ICMP redirects)
  - Log rotation for app logs (14-day retention)
- `infra/vps/deploy.sh` - Production deployment script (called by CI or manually):
  - Builds containers in parallel, rolling restart (postgres untouched)
  - Health check: HTTP status code validation (200/401/302 all valid)
  - Automatic rollback to previous images on health check failure
  - Docker image pruning (48h retention)
  - Service status verification
- `.github/workflows/production.yml` - GitHub Actions CI/CD pipeline:
  - Trigger: push to main or manual dispatch
  - Job 1 (lint-and-audit): TypeScript type check, dependency audit, secrets-in-code scan
  - Job 2 (docker-build-test): Buildx validation of vertex-app + analytics images with GHA cache
  - Job 3 (deploy): SSH to VPS, rsync codebase, atomic .env write, deploy.sh execution, internal + external health checks
  - SSH host key pinning via VPS_HOST_FINGERPRINT secret (fallback to ssh-keyscan with warning)
  - Concurrency: single deploy at a time, no cancel-in-progress
- `.env.example` - Template for production environment variables

**Required GitHub Actions Secrets:**
VPS_HOST, VPS_SSH_PRIVATE_KEY, VPS_SSH_PORT (default 2222), VPS_HOST_FINGERPRINT (recommended),
POSTGRES_USER, POSTGRES_PASSWORD, SESSION_SECRET, VERTEX_DOMAIN, CERTBOT_EMAIL,
TOPSTEPX_API_KEY, TOPSTEPX_USERNAME, TOPSTEPX_PASSWORD, TRADOVATE_CID, TRADOVATE_SECRET,
STRIPE_SECRET_KEY, STRIPE_PUBLISHABLE_KEY, STRIPE_WEBHOOK_SECRET, OPENAI_API_KEY, CREDENTIALS_ENCRYPTION_KEY,
SIGNAL_WEBHOOK_SECRET

### Algorithmic Signal Ingestion
- `server/signal-routes.ts` - Signal webhook and mapping management endpoints:
  - `POST /api/v1/signals` - External webhook endpoint (HMAC SHA-256 authenticated, CSRF-exempt)
    - Accepts: `{source, symbol, action, quantity, group_id?, group_name?, price?, order_type?, limit_price?, stop_price?, timestamp?}`
    - Validates payload via Zod schema
    - Atomic dedup via SHA-256 hash of raw body → `INSERT ON CONFLICT DO NOTHING` on `processed_signals.signal_hash`
    - Dynamic symbol resolution via `signal_mappings` table (e.g., TradingView `NQ1!` → broker `NQ`)
    - Bridges into `executeWebhookOrderForFollower()` fan-out — all risk-enforcer rules apply
    - Returns execution results per follower with latency tracking
  - `GET /api/v1/signals/mappings` - List symbol mappings (session auth)
  - `POST /api/v1/signals/mappings` - Create mapping (session auth)
  - `PUT /api/v1/signals/mappings/:id` - Update mapping (session auth)
  - `DELETE /api/v1/signals/mappings/:id` - Delete mapping (session auth)
  - `GET /api/v1/signals/history` - Processed signal audit log (session auth)
- `shared/copy-trading-schema.ts` - Signal tables:
  - `signal_mappings` - Dynamic symbol mapping (source + external_symbol → broker_contract_id). Update front month here without code push.
  - `processed_signals` - Dedup log with unique `signal_hash`, status tracking, execution results JSONB
- **HMAC Auth**: `x-signature-256` header = `sha256=<HMAC-SHA-256(body, SIGNAL_WEBHOOK_SECRET)>`. Uses `crypto.timingSafeEqual` for constant-time comparison.
- **Security**: CSRF-exempt (external webhook), bypasses session auth middleware, but requires valid HMAC signature. Without `SIGNAL_WEBHOOK_SECRET` set, all signals are rejected (500).

### Infrastructure Telemetry (Prometheus + Grafana)
- `server/prometheus-metrics.ts` - Prometheus metric definitions using prom-client:
  - `vertex_http_request_duration_seconds` (histogram) - HTTP request latency by method/route/status
  - `vertex_http_requests_total` (counter) - Total HTTP requests by method/route/status
  - `vertex_ws_connections_active` (gauge) - WebSocket/streaming connections by provider/type
  - `vertex_broker_api_latency_ms` (histogram) - Broker API call latency by provider/operation
  - `vertex_copy_order_latency_ms` (histogram) - Copy trading execution latency
  - `vertex_risk_interventions_total` (counter) - Risk interventions by type/status
  - `vertex_eventloop_lag_ms` (gauge) - Node.js event loop lag
  - `vertex_inflight_orders` (gauge) - Orders currently being executed
  - `vertex_sse_clients_active` (gauge) - SSE streaming clients by stream type
  - `vertex_db_query_latency_ms` (histogram) - Database query latency
  - `vertex_reconciliation_runs_total` (counter) - Reconciliation daemon runs by result
  - `vertex_orphans_detected` (gauge) - Orphan positions from last scan
  - Default Node.js process metrics (CPU, memory, GC, event loop) with `vertex_` prefix
- **Bridge**: `recordLatency()` in latency-monitor.ts emits to both in-memory store AND Prometheus histograms. WS/SSE gauges synced every 5s via `syncPrometheusGauges()`.
- **Node.js endpoint**: `GET /metrics` (registered in server/index.ts before routes)
- **Python endpoint**: `GET /metrics` on analytics service via `prometheus-fastapi-instrumentator` (auto-instruments request duration, status codes)
- `infra/prometheus/prometheus.yml` - Scrape config: vertex-app (5s interval), analytics (15s interval), self (15s)
- `infra/prometheus/alerts.yml` - Alert rules:
  - Critical: event loop lag >500ms, broker API p95 >2s, no WS connections
  - Warning: event loop lag >100ms, broker API p95 >500ms, copy order p95 >1s, DB p95 >200ms, Node >512MB RAM, analytics >256MB RAM, analytics p95 >10s
  - Info: risk intervention triggered
- `infra/grafana/provisioning/datasources/prometheus.yml` - Auto-provisions Prometheus as default datasource
- `infra/grafana/provisioning/dashboards/dashboard.yml` - Auto-provisions dashboard JSON from file
- `infra/grafana/dashboards/vertex-overview.json` - Pre-built 14-panel dashboard:
  - Event Loop Lag, HTTP Request Rate, Memory Usage (Node + Python)
  - Broker API Latency p95, WebSocket Connections (stat), In-Flight Orders
  - Copy Order Latency p95, Database Latency p95
  - Risk Interventions (bar), HTTP Duration p95, Analytics Duration p95
  - Reconciliation Runs (stat), Orphan Positions (stat)
- **Nginx routing**: Grafana served at `/grafana/` path with WebSocket upgrade support for live features
- **Access**: `GRAFANA_ADMIN_USER` / `GRAFANA_ADMIN_PASSWORD` env vars (default admin/vertex_grafana_change_me)

## Mobile Responsiveness
- Hamburger menu button opens slide-out sidebar (replaces bottom nav bar)
- Mobile sidebar has: logo, user profile, categorized navigation (Platform, Copy Trading, Journaling, Tools)
- Admin link visible only to users with admin role
- RTL-aware sidebar positioning (slides from right in RTL, left in LTR)
- Trial countdown banner shown in sidebar for trialing users
- Compact header and KPI cards on mobile
- Dialog forms extracted into separate memoized components

## Local Development
- **DB**: dev PostgreSQL runs via `docker compose -f docker-compose.dev.yml up -d` → `postgres:15-alpine` on host port **5433** (container 5432), user/pass/db = `vertex`/`vertex`/`vertex_command`, healthcheck via `pg_isready`.
- **Schema**: `npm run db:push` (drizzle-kit). First run only; existing DB already holds the full 26-table schema. `db:push` warns on data-loss drops — abort if the DB already has tables.
- **Run**: `npm run dev` (`cross-env NODE_ENV=development tsx server/index.ts`). Express + Vite middleware serve API and SPA together on **port 5000**. Health at `GET /api/health`, `/health`, `/healthz`; metrics at `GET /metrics`.
- **Required `.env` keys**: `DATABASE_URL` (`postgres://vertex:vertex@localhost:5433/vertex_command`), `CREDENTIALS_ENCRYPTION_KEY` (64 hex / 32 bytes), `SESSION_SECRET`, `SIGNAL_WEBHOOK_SECRET`.
- **Credential decrypt note**: `server/encryption.ts` uses AES-256-GCM. A blob encrypted under a previous `CREDENTIALS_ENCRYPTION_KEY` fails with `Unsupported state or unable to authenticate data` (GCM auth-tag mismatch) and is **unrecoverable** — the auto-connect handler sets the connection to `error` + "reconnect" and the startup dedup merge (`server/linked-users.ts`) prunes duplicate connections.

## Repository Docs
- `README.md` — public-facing project README (overview, feature matrix, tech stack, architecture diagram, quick start, scripts, data model, security, deployment).
- `PRESENTATION.md` — 12-slide project presentation deck (Marp/reveal/preview-ready).
- `claude.md` — this file: deep architecture + AI context reference (renamed from `replit.md`).
- `vertex-command-spec.md` — original product spec.

## Repository Security Posture
- Git history audited (all commits, all blobs): **no secrets ever committed** — `.env`, `.stripe-keys.json`, broker keys, encryption/session secrets, webhook URLs are absent from history. No `git filter-repo` purge needed.
- `.env`, `.stripe-keys.json`, logs, `node_modules`, `dist`, `__pycache__`, `.venv`, `backups/`, `*.sql.gz` are git-ignored.
- The git root is the **outer wrapper dir**; the project lives in the `Vertex_Command-main/` subdir. The detailed `.gitignore` (5.4 KB) is in the subdir; the **root** `.gitignore` carries a full-tree `**/` secret backstop so coverage holds regardless of subdir.
- `.replit` secrets were removed (set via Replit Secrets). `.env.example` holds placeholders only.
- Pre-push GitHub checklist: private repo, Secret Scanning + Push Protection, Dependabot, branch protection on `main` (PR review, required status checks `lint-and-audit`/`docker-build-test`, signed commits, linear history, no force-push).
