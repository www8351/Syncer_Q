# DECISIONS

Decision log. Why things were chosen, what was rejected, whether final.

---

## 2026-06-14 — Adopt file-based lifecycle protocol

- **Decision:** Enforce 5-file lifecycle (README, STATUS, PROGRESS, DECISIONS, CLAUDE_MEMORY) as source of truth, per CLAUDE.md.
- **Why:** Persistent cross-session state; avoid re-explaining context to each new AI session.
- **Rejected:** Single monolithic notes file — loses separation between timeline / current-state / rationale.
- **Status:** Final.

## 2026-06-14 — Preserve existing README on bootstrap

- **Decision:** Bootstrap creates only missing files; existing `README.md` left untouched.
- **Status:** Final.

## 2026-06-14 — Production stack = ROOT monolith (Path A); infra/ = future only

- **Decision:** The root `docker-compose.yml` (postgres + vertex-app + analytics + prometheus + grafana + ingress + certbot) is the production target. `infra/docker-compose.*` (Go engine + Redis microservices) stays as documented future scaffolding, not wired to the app.
- **Why:** The application code is wired to the root stack only; it uses no Redis and does not publish to the Go engine.
- **Status:** Final (locked by task).

## 2026-06-14 — Prevent accidental use of the microservices compose by RENAME (+ README)

- **Decision:** `git mv infra/docker-compose.yml infra/docker-compose.microservices.yml` and add `infra/README.md`.
- **Why:** Rename is the real guard — a bare `docker compose up` in `infra/` finds no default file and can't launch by accident. README explains intent.
- **Rejected:** README-only (a bare `up` still auto-finds `docker-compose.yml`); rename-only (no explanation).
- **Status:** Final.

## 2026-06-14 — `drizzle-kit` moved to production dependencies

- **Decision:** Move `drizzle-kit` from `devDependencies` to `dependencies`.
- **Why:** The runtime image installs `--omit=dev`; `entrypoint.sh` runs `drizzle-kit push` at boot, so the tool must ship in the image. A runtime `npx` download is fragile/offline-unsafe and was failing.
- **Rejected:** Runtime `npx` fetch; baking migrations at build time (DB not available at build).
- **Status:** Final.

## 2026-06-14 — Healthchecks use 127.0.0.1, not localhost

- **Decision:** All container healthchecks (and `deploy.sh` health gate) probe `127.0.0.1`; raised `start_period`.
- **Why:** Alpine/musl resolves `localhost`→IPv6 `::1` first, but the app listens IPv4-only → permanent false `unhealthy`. App's `~40s` boot needs a longer start window.
- **Rejected:** Making the app listen on `::` (dual-stack) — that's an app-code change; out of scope.
- **Status:** Final.

## 2026-06-14 — Analytics: drop removed Instrumentator kwargs (vs downgrade)

- **Decision:** Remove `metric_namespace`/`metric_subsystem` from `Instrumentator()` in `analytics/main.py`; keep pinned `prometheus-fastapi-instrumentator==7.0.2`.
- **Why:** Those kwargs were removed in v6.0 → hard import crash. No version both has them AND supports the pinned FastAPI 0.115 (5.x conflicts). Dashboard filters analytics metrics by Prometheus job label, not an `analytics_` name prefix, so dropping them is safe.
- **Rejected:** Pin to 5.x/6.x (dependency conflict / kwarg still absent in 6.x).
- **Status:** Final.

## 2026-06-14 — Rate-limit restart crash left for app-side follow-up

- **Decision:** Do NOT fix the `@acpr/rate-limit-postgresql` re-init crash in this PR.
- **Why:** Root cause is in application code (`server/index.ts:265` constructs multiple `PostgresStore`s, each re-running a non-idempotent `init`); the task constrains changes to containerization + docs. Fresh deploys work; only restart/redeploy over an existing DB is affected.
- **Status:** Open — flagged in PR, STATUS.md, and MIGRATION_CHECKLIST Phase 8. Revisit with an app-side idempotency fix before relying on rolling restarts.
