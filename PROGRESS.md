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
