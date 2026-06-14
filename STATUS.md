# STATUS

_Last updated: 2026-06-14_

## Where the project stands

Containerization (Step 1) finalized on branch `claude/containerization-setup-review-o40yot`. The ROOT monolith stack (Path A) builds and runs end-to-end; ambiguity vs the `infra/` microservices stack removed; hardening + docs applied. Draft PR pending review.

## Done

- **End-to-end verified (fresh deploy):** `docker compose build` (Node/Python/nginx) + `up -d` → postgres, vertex-app, analytics, prometheus, grafana all **healthy**; ingress routes over TLS. `/api/health`=200 (`database:true`), `/metrics` served, analytics `/health` healthy, 49 tables migrated.
- **3 blocking build/run bugs found & fixed:** (1) `drizzle-kit` was a devDep → not in runtime image → migrations silently skipped; moved to prod deps. (2) healthchecks probed `localhost` → musl resolves IPv6 first while app listens IPv4 → false `unhealthy`; switched to `127.0.0.1` + longer `start_period`. (3) analytics crashed on import — `Instrumentator(metric_namespace=…)` removed in instrumentator v6; dropped the kwargs.
- **Consolidation:** `infra/docker-compose.yml` → `infra/docker-compose.microservices.yml` + new `infra/README.md` (future/not-prod). Removed obsolete `version:` key.
- **Hardening:** `env_file` now optional (`required:false`); `.env.example` annotated REQUIRED/OPTIONAL; deploy.sh health gate fixed to `127.0.0.1`.
- **Docs:** README deployment section (both copies), `MIGRATION_CHECKLIST.md` rewritten for Path A, `claude.md` infra section corrected.

## Open / In progress

- Draft PR review + merge.

## Next best action

- Open the draft PR; address review.

## Blockers / Waiting

- None blocking the PR.

## Needs review

- **Known app-level bug (out of containerization scope):** `vertex-app` crash-loops on **restart/redeploy over an existing DB** — `@acpr/rate-limit-postgresql` (`server/index.ts:265`) re-runs its `init` migration → `relation "unique_session_key" already exists`. Fresh deploys unaffected. Needs an app-side fix (idempotent store init / single shared store). Flagged in the PR and `MIGRATION_CHECKLIST.md` Phase 8.
