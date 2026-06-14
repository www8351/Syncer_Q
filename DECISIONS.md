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

## 2026-06-14 — Rate-limit restart crash — FIXED (user-authorized app-level fix)

- **Decision:** Fix the `@acpr/rate-limit-postgresql` re-init crash via `tablesFilter` + a one-shot pre-migration, not by editing `server/index.ts`.
- **Investigation (systematic debugging):** Two compounding causes — (1) `drizzle-kit push --force` drops `public.migrations` every boot (it's not in the Drizzle schema; the rate-limit objects live in the separate `rate_limit` schema and persist), so tracking desyncs and `init` re-runs into `relation "unique_session_key" already exists`; (2) the 6 `PostgresStore` constructors call `applyMigrations()` un-awaited → concurrent races on first boot.
- **Fix:** `drizzle.config.ts` → `tablesFilter: ["!migrations"]` (push leaves the tracking table alone); `scripts/migrate-ratelimit.cjs` applies the store migrations once, serially, before the app constructs its stores, wired into `entrypoint.sh` + npm `predev`/`prestart`.
- **Rejected:** Editing `server/index.ts` to share/serialize stores (more invasive, reorders middleware); catching the error (symptom only — later migrations would be skipped).
- **Status:** Final. Verified: two restarts → restarts=0, healthy, 0 crashes; tracking survives push; app tables intact.
