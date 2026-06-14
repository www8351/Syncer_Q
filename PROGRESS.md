# PROGRESS

Dated log of what happened over time.

---

## 2026-06-14 — Lifecycle bootstrap

- **Worked on:** Initialization per CLAUDE.md "Bootstrap on Demand" rule.
- **Changed:** Scanned workspace root. Found existing `README.md` and `claude.md`. Created the 4 missing lifecycle files: `STATUS.md`, `PROGRESS.md`, `DECISIONS.md`, `CLAUDE_MEMORY.md`.
- **Worked:** Targeted creation — did not overwrite existing README.

## 2026-06-14 — Finalize Containerization (Step 1)

Branch: `claude/containerization-setup-review-o40yot`. Order 3→1→2→4.

- **STEP 3 (verify first):** Built the ROOT stack and ran it end-to-end with a generated test `.env` (ports remapped to 18080/18443 to avoid host collisions). Found and fixed real failures:
  - **drizzle-kit missing from runtime image** — it was a devDependency; `npm ci --omit=dev` dropped it, so `entrypoint.sh`'s `npx drizzle-kit push` hit `Cannot find module 'drizzle-kit'` (swallowed by `|| WARNING`) → tables never created → `ensurePlansSeeded()` crashed on missing `plans` → crash loop. **Fix:** moved `drizzle-kit` to `dependencies` + regenerated `package-lock.json`. After fix: `Changes applied`, 49 tables, app serving.
  - **Healthcheck false-unhealthy** — `wget … http://localhost:5000/metrics` failed with "connection refused" while the app was up. Cause: Alpine/musl resolves `localhost`→IPv6 `::1` first, but the app listens IPv4 `0.0.0.0:5000`. Verified `127.0.0.1` works, `localhost` refuses. **Fix:** healthchecks → `127.0.0.1`, `start_period` raised (app boots in ~40s before listen). Same bug fixed in `infra/vps/deploy.sh`.
  - **Analytics import crash** — `main.py` passed `metric_namespace`/`metric_subsystem` to `Instrumentator()`; those kwargs were removed in `prometheus-fastapi-instrumentator` v6 (pinned at 7.0.2). No compatible version exists for the pinned FastAPI (5.x conflicts), so removed the two kwargs (observability config; dashboard filters by job label, not name prefix).
  - **Verified:** fresh `down -v` → `up -d` → all 5 healthchecked services healthy; ingress TLS + routing (`/api/health` 200, `/` 200, `/metrics` 404 by design, `/api/v1/analytics/*` 401 gateway).
  - **Found, NOT fixed (out of scope — app logic):** restart/redeploy over an existing DB crash-loops vertex-app via `@acpr/rate-limit-postgresql` re-running `init` (`unique_session_key already exists`). Documented for app-side follow-up.
- **STEP 1 (consolidate):** `git mv infra/docker-compose.yml infra/docker-compose.microservices.yml` + new `infra/README.md` stating it's future/not-wired/not-prod. Verified no scripts/CI reference the old name (only docs).
- **STEP 2 (harden):** root `env_file` → `path: .env, required: false` (documented `.env` still mandatory in practice); removed obsolete `version: "3.9"` from the microservices compose; annotated every `.env.example` var REQUIRED/OPTIONAL.
- **STEP 4 (docs):** README Deployment section rewritten (build→configure→up→verify, port override, infra=not-prod) in both README copies; `infra/MIGRATION_CHECKLIST.md` rewritten for Path A (containerized Postgres, root stack, auto-migrations, certbot profile); `claude.md` infra section corrected.
- **Did not modify** application/business logic. Two source touches were unavoidable blocking-build fixes: `analytics/main.py` (removed dead kwargs) and `package.json` (dep classification) — neither changes app behavior.

## 2026-06-14 — Rate-limit restart crash FIXED (user-authorized)

Followed systematic-debugging. The crash (`relation "unique_session_key" already exists` on restart) had **two** root causes, found by reading the library source + inspecting live DB state:

- **drizzle-kit push drops the tracking table.** `@acpr/rate-limit-postgresql` tracks migrations in `public.migrations` and keeps its objects in a separate `rate_limit` schema. `entrypoint.sh` runs `drizzle-kit push --force` every boot, which **dropped `public.migrations`** (not a Drizzle table) while the `rate_limit.*` objects persisted → tracking desynced → `init` re-ran and collided. Proven by one-off test: `public.migrations` present (1 row) → `null` after `push --force`.
- **Un-awaited concurrent store migrations.** `PostgresStore`'s constructor calls `applyMigrations()` fire-and-forget; `server/index.ts` builds 6 stores → 6 racing migrations on first boot.

**Fixes:** `drizzle.config.ts` `tablesFilter: ["!migrations"]` (push stops dropping the tracking table) + new `scripts/migrate-ratelimit.cjs` that applies the store migrations once, serially, before the app — wired into `entrypoint.sh` and npm `predev`/`prestart`. Used a `pg.Client` (not `{databaseUrl}`, which postgres-migrations v5 rejected — first attempt failed with "Database config problem", corrected to mirror the library).

**Verified end-to-end:** fresh deploy healthy; **two consecutive restarts → `restarts=0`, healthy, 0 crash lines** (was 0→9 crash-loop); `public.migrations` survives push (8 rows); 49 app tables still managed; all 5 services healthy.

## 2026-06-14 — Fix CI for PR #1 (green checks, no production deploy)

Goal: make PR #1 show green checks WITHOUT enabling a prod deploy (VPS not provisioned). CI/workflow config only.

- **Root cause (bigger than expected):** the only workflow lived at `Vertex_Command-main/.github/workflows/production.yml` — a **nested subdir**. GitHub Actions only discovers workflows in `.github/workflows/` at the **repo root**, so the workflow never ran at all (the missing `pull_request` trigger was secondary). Confirmed: repo root had no `.github/`; tracked paths were `Vertex_Command-main/.github/...` and `Vertex_Command-main/package.json`.
- **Changed:**
  - **Moved** workflow to repo root `.github/workflows/production.yml`; `git rm` the nested copy.
  - **Nested-dir resolution:** `defaults.run.working-directory: Vertex_Command-main` (npm ci / tsc / audit / secret-scan); `setup-node` `cache-dependency-path: Vertex_Command-main/package-lock.json`; docker `context: Vertex_Command-main` (+ `analytics`) with matching `file:`.
  - **PR trigger:** added `pull_request:[main]` (kept `push:[main]` + `workflow_dispatch`); per-ref `concurrency` so PRs don't serialize.
  - **Deploy gated:** `if: github.event_name == 'workflow_dispatch' && vars.DEPLOY_ENABLED == 'true'` + job-level `concurrency: production-deploy`. A merge to main (event `push`) can never match → no auto-deploy. Re-enable steps documented in-file + DECISIONS.md.
  - **tsc non-blocking:** `continue-on-error: true` (user-approved) — 133 pre-existing type errors (29 files); app builds via esbuild+vite regardless. Tracked as tech debt with a TODO to revert.
- **Pre-push verification:** `.stripe-keys.json` is untracked → absent from CI checkout → secret-scan won't false-positive; `git grep` of tracked source = 0 secret-pattern matches. `npm run build` locally → exit 0, `dist/index.cjs` (2.0 MB) + `dist/public` produced (de-risks `docker-build-test`, which does not type-check). New workflow YAML parses; triggers/working-dir/deploy-gate/docker-contexts asserted via PyYAML.
- **Tried/failed:** `actionlint` not installed (validated YAML structurally with PyYAML instead). First combined shell cmd short-circuited the `git rm` (a `&&` after a non-zero `python -c`); re-ran the removal separately.
- **Constraint honored:** no app logic / no infra changes — only the workflow + lifecycle docs.
- **Next:** push branch, watch the run, capture `gh pr checks 1`; STOP before merge (await user approval). PR stays draft.
