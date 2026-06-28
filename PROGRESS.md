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

## 2026-06-14 — CI iteration to GREEN (3 latent blockers found by actually running the workflow)

Once the workflow ran from the repo root for the first time, three pre-existing bugs surfaced in sequence (none were ever visible before because the nested workflow never executed). Fixed each, re-pushed, re-watched:

1. **Startup failure (0s, no jobs):** `deploy.environment.url` used `${{ secrets.VERTEX_DOMAIN }}`; `secrets` isn't an allowed context there → whole-workflow compile failure. Installed `actionlint` (v1.7.7 win binary) which pinpointed it (`context "secrets" is not allowed here`). Fixed → `vars.VERTEX_DOMAIN`. Re-ran clean.
2. **Docker build — `Could not resolve "./lib/queryClient"`:** `.gitignore` `lib/` (a Python-venv pattern) over-matched `client/src/lib/`, so `queryClient.ts` + `utils.ts` were never committed → absent from the CI checkout. Anchored the rule (`/lib/`, `/lib64/`) and committed the 2 files (user-approved scope cross). Build advanced.
3. **Docker build — `/app/attached_assets: not found`:** runner stage `COPY attached_assets` failed because the 47 MB media dir is deliberately gitignored. Confirmed unused (vite built without it; no server refs). User-approved fix: `attached_assets/.gitkeep` placeholder (`/attached_assets/*` + `!.gitkeep`) — dir present, media uncommitted, Dockerfile untouched.

- **Verification (fresh evidence):** `gh pr checks 1` → exit 0; `Static Analysis & Audit` = pass, `Docker Build Validation` = pass (both images), `Deploy to VPS` = skipping. Deploy job `conclusion=skipped` across all 3 runs — **never executed**. PR #1 remains `isDraft=true`. tsc step shows as a tolerated (non-blocking) annotation, as designed.
- **Did NOT** merge, mark ready, or run the deploy. Stopped for user approval.
- **Commits:** `bc86a52` (relocate + gate), `124902c` (startup-failure fix), `bf89b7c` (client/src/lib), `a398cb5` (attached_assets .gitkeep).

## 2026-06-14 — PR #1 merged to main (deploy verified NOT fired)

- **Action:** User approved merge. Marked PR ready, merged via merge commit `4561dac`.
- **Safety verification (the whole point):** the merge produced a `push` to main → workflow ran (`27505627776`). Deploy job `if: workflow_dispatch && DEPLOY_ENABLED` evaluated false on a `push` event → **`Deploy to VPS`=skipped**. `Static Analysis & Audit`=success, `Docker Build Validation`=success. No rsync / no prod secrets / no containers — confirmed no production deploy on merge to main.
- **State:** `main` now carries the root-level CI workflow with deploy gated OFF. VPS deploy remains disabled until provisioned + `DEPLOY_ENABLED=true`.

## 2026-06-15 — Cleanup: feature branch deleted

- Deleted `claude/containerization-setup-review-o40yot` (remote + local; local `-d` confirmed it was merged). Merged commits preserved in `main` via merge commit `4561dac`.
- CI/containerization work fully closed. No open work items; next action is VPS provisioning (then enable deploy) + paying down the 133 `tsc` errors.

## 2026-06-15 — Step 2: AWS infra as Terraform (branch claude/aws-terraform-step2)

- **Worked on:** Greenfield `infra/terraform/` for the Go routing engine across 3 regions.
- **Created:** root config (`versions/providers/variables/main/ecr/iam/outputs.tf`), reusable `modules/region-host` (key pair + SG + EC2, Ubuntu 24.04 ARM64 via Canonical SSM), `templates/user_data.cloud-init.yaml.tftpl` (embeds provision.sh b64 + installs `amazon-ecr-credential-helper`), `bootstrap/` (S3+DynamoDB state), `backend.hcl.example`, `terraform.tfvars.example`, `.gitignore`, `README.md`. Added `infra/backend-go/build-push.sh`.
- **Edited:** `infra/backend-go/Dockerfile` → ARM64-capable (`TARGETOS/TARGETARCH`, default arm64).
- **Decisions (user-confirmed):** Go engine image; S3+DynamoDB state; SSH 2222; imported public key. SG: SSH←admin IP, 443←0.0.0.0/0 (interim until Global Accelerator). IMDSv2 enforced; single ECR + cross-region pull.
- **Verified:** downloaded terraform 1.9.8; `terraform fmt -recursive -check` clean; `terraform init -backend=false` + `terraform validate` = "configuration is valid" for BOTH root and bootstrap. **Not applied** (billable AWS resources — user runs apply per README).
- **Not done (later steps):** AWS Global Accelerator + 443 lockdown; engine app-wiring (Redis/secrets/run); custom VPC.

## 2026-06-16 — PR #2 merged to main (Step 2 AWS infra + cleanup)

- **Action:** User approved merge. Pre-merge check: `mergeable=MERGEABLE`, `mergeStateStatus=CLEAN`. Checks green — `Static Analysis & Audit`=success, `Docker Build Validation`=success; `Deploy to VPS`=skipped, `Supabase Preview`=skipped. Merged via merge commit `b2d3ddc` (base `main` ← `claude/aws-terraform-step2`). +720 / −1789.
- **Content:** `infra/terraform/` (ECR + 3 ARM64 Graviton EC2 across us-east-1/eu-central-1/ap-northeast-1, IAM ECR-readonly, per-region SG, S3+DynamoDB state, IMDSv2); ARM64-capable Dockerfile + `build-push.sh`; repo cleanup (`python_fixes/`, `_bmad/`, stale docs, `attached_assets` 47M→3.1M).
- **Safety:** Terraform NOT applied (billable). No app logic changed; secrets untouched.
- **Next:** operator runs `terraform apply` per `infra/terraform/README.md`; later steps — Global Accelerator + 443 lockdown, engine app-wiring.

## 2026-06-16 — Vercel split deploy: frontend on Vercel, backend on Render/Railway/Fly

Branch: `feat/google-signin`. Diagnosed the `404: NOT_FOUND` on the Vercel domain and implemented the SPA-only split.

- **Root cause of 404 (two):** (1) git root is the outer wrapper dir; the app lives in the `Vertex_Command-main/` subdir, so Vercel scanned root, found no framework/output → 404. (2) Wrong tool: the Express server is long-lived/stateful (broker WebSockets, SSE, background daemons, in-memory rate-limit/idempotency locks, PG sessions) — cannot run on Vercel serverless.
- **Decision (user):** deploy SPA static to Vercel; backend Docker stack to Render/Railway/Fly; frontend calls backend cross-origin. See DECISIONS.md.
- **Frontend refactor (configurable API base):** new `client/src/lib/apiBase.ts` (`API_BASE` from `VITE_API_URL`, `apiUrl()` helper). Wrapped all 5 fetch sites in `queryClient.ts` and all 3 `EventSource` sites (`useTradingStream.ts`, `useTelemetryStream.ts`, `SystemHealth.tsx`) — SSE also gained `withCredentials: true` (cookies don't cross origin otherwise).
- **Backend (`server/index.ts`):** `buildAllowedOrigins()` now reads `FRONTEND_ORIGINS` (comma list) + optional `ALLOW_VERCEL_PREVIEWS` regex for `*.vercel.app`; session cookie `sameSite` is `none` when `CROSS_SITE_COOKIES=true` (else `lax` for local/same-origin). `cors({credentials:true})` already present.
- **Config:** new `Vertex_Command-main/vercel.json` (framework vite, `buildCommand: npx vite build`, `outputDirectory: dist/public`, SPA rewrite). `.env.example` documents `VITE_API_URL`/`FRONTEND_ORIGINS`/`ALLOW_VERCEL_PREVIEWS`/`CROSS_SITE_COOKIES`.
- **Verified:** `npx vite build` → `✓ built in 13.84s`, `dist/public/` emitted. `npm run check` still red but ONLY pre-existing errors (tracked 133-error tech debt) — none in touched files. Build command Vercel runs (esbuild/vite) does not type-check.
- **Caveat flagged:** cross-site cookie (`SameSite=None`) is dropped by browsers blocking third-party cookies (Safari ITP, Brave, Chrome incognito / 3p-cookie phase-out) → login may 401. Robust fix later = shared parent domain (`app.` + `api.` same registrable domain → `SameSite=Lax`).
- **Manual ops left to operator:** Vercel project Root Directory = `Vertex_Command-main` + `VITE_API_URL` env; deploy backend Docker to host + set its env; add Vercel origin to Google OAuth Authorized JS origins.
- **Not committed** — code changes only; awaiting user.

## 2026-06-15 — Repo/disk cleanup (Tiers A + B + C)

Inventory first: `.git` only 4.3 MB (repo NOT bloated); disk hogs were local/ignored (`node_modules` 707 MB, `attached_assets` 47 MB, `dist` 6.2 MB).

- **Tier A (local rm, not in git):** logs (`dev-server.log`, `vite-dev.log`), `dist/`, `.cache/ .agents/ .canvas/`, `artifacts/`, `.replit`, empty `_bmad-output/ backups/`, editor swap file, and 3 stale untracked duplicate `STATUS/PROGRESS/DECISIONS.md` inside the nested dir. Verified none were git-tracked.
- **Tier B (`git rm`):** `python_fixes/` (4 files, no refs), `PRESENTATION.md`, `vertex-command-spec.md`.
- **Tier C:** `git rm _bmad/` (12 tracked files — recoverable from history; disables `bmad-*` skills until reinstalled). `attached_assets/` pruned **120 → 7 files** (deleted 113 old prompts/screenshots/images/mp4; kept 6 `logo-vertex-*.png` + `.gitkeep`), 47 MB → 3.1 MB.
- **Kept by user choice:** `node_modules` (707 MB, `npm ci` to rebuild), `_bmad` ignored remnants (10 KB).
- **Safety gates honored:** secrets `.env` + `.stripe-keys.json` untouched; irreversible local media deletion done only after explicit per-bucket confirmation (auto-classifier blocked the first bundled attempt); zero essential build/app files removed.
- **Note:** pre-existing uncommitted working-tree changes (Google Sign-In WIP: `useAuth.ts`, `AuthPage.tsx`, `locales/*`, `server/{index,routes,storage}.ts`, `package.json`, `.env.example`) were left untouched.

## 2026-06-16 — PIVOT: kill AWS multi-region; finish Vercel SPA + single-VPS split

Branch `feat/vercel-vps-split` (off `feat/google-signin`). User mandate: permanently abandon the AWS multi-region Terraform stack; ship hybrid Vercel (`syncer-q.vercel.app`) + single VPS.

- **Explored first (3 parallel agents):** mapped IaC inventory, frontend Vercel-readiness, backend CORS/cookie/Docker state. **Key finding:** the prior "Vercel split" commit (`8fab244`) was **incomplete** — it wrapped only `queryClient.ts` + 3 SSE hooks in `apiUrl()`, leaving ~41 relative `fetch("/api/…")` across 16 files + 6 `window.open`/`window.location.href` backend nav/downloads un-prefixed. On Vercel those resolve to `syncer-q.vercel.app/api/…` → SPA rewrite serves `index.html` → `res.json()` throws. App was NOT actually cross-origin-ready.
- **Cleanup (`git rm`):** deleted `infra/terraform/` (all `.tf` + modules/bootstrap/templates/ECR/IAM/state/locks/examples/README), `infra/backend-go/` (Go engine + ECR `build-push.sh`), `infra/frontend/`, `infra/nodejs-api/`, `infra/nginx/`, `infra/prometheus/`, `infra/grafana/`, `infra/docker-compose.microservices.yml`, `infra/MIGRATION_CHECKLIST.md`, `infra/README.md`. Kept `infra/vps/{provision.sh,deploy.sh}`.
- **Compose:** removed `prometheus` + `grafana` services + `prometheus_data`/`grafana_data` volumes (they mounted the now-deleted `infra/prometheus`+`infra/grafana` — a blind `rm -rf infra/` would have broken the kept stack). `nginx/nginx.conf`: removed `grafana_backend` upstream + `/grafana/` + `/grafana/api/live/` locations. `infra/vps/deploy.sh`: dropped prometheus/grafana from the health loop.
- **Frontend (the real work):** 3 parallel agents wrapped the 41 `fetch` sites (added `import { apiUrl } from "@/lib/apiBase";` per file); then wrapped the 6 `window.open`/`window.location.href` OAuth/export/download sites by hand. **Verified:** grep → 0 un-wrapped `/api` fetches; `npx vite build` exit 0.
- **CORS/cookies:** 12-Factor per user — **no `server/index.ts` change** (already env-driven). `.env.example`: promoted `FRONTEND_ORIGINS`/`CROSS_SITE_COOKIES`/`ALLOW_VERCEL_PREVIEWS` from OPTIONAL to active (example `https://syncer-q.vercel.app`, `CROSS_SITE_COOKIES=true`); removed the Grafana env block.
- **CI:** `production.yml` had no Terraform/ECR refs to strip. Added `FRONTEND_ORIGINS`/`CROSS_SITE_COOKIES`/`ALLOW_VERCEL_PREVIEWS`/`GOOGLE_CLIENT_ID`/`SIGNAL_WEBHOOK_SECRET` to the atomic `.env` write; dropped prometheus/grafana from the health-check loop.
- **Worked / didn't:** `vercel.json` was already correct (framework vite, `outputDirectory dist/public`, SPA rewrite) — left as-is. No remaining hardcoded `localhost`/ports in `client/src` (grep clean). Backend code untouched (already supported cross-site).
- **Not committed** — awaiting user.

## 2026-06-28 — Vercel "build Success but blank/404" diagnosis + branch consolidation to single `main`

Task: resolve the Vercel runtime failure (passes build, serves blank screen / 404), collapse the repo to one canonical branch, sync lifecycle docs. Explored with 3 parallel agents (frontend config, git topology, lifecycle docs), then verified every claimed file by hand.

- **Diagnostic — config already correct (no rewrite).** Read `Vertex_Command-main/vercel.json` (framework vite, `buildCommand npx vite build`, `outputDirectory dist/public`, rewrite `[{source:"/(.*)",destination:"/index.html"}]`), `vite.config.ts` (`root client`, `build.outDir dist/public`, no `base` ⇒ defaults `/` which is correct for a root-domain SPA; replit plugins gated behind `NODE_ENV!=='production' && REPL_ID`), `client/src/lib/apiBase.ts` (`API_BASE = (VITE_API_URL ?? "").replace(/\/$/,"")`, `apiUrl()` passes absolute URLs through). No hardcoded host, no `http://` ⇒ no mixed-content. **The Task A prescription to "create/rewrite vercel.json" was already satisfied** — rewriting was rejected as regression risk that can't fix a dashboard-level cause.
- **Root cause = Vercel dashboard/infra, not repo.** A successful build + blank runtime points to (1) `VITE_API_URL` unset in Vercel ⇒ `API_BASE=""` ⇒ API calls resolve to `syncer-q.vercel.app/api/*` (no backend) ⇒ app stalls on first `/api/auth/me`; (2) Root Directory (likely already `Vertex_Command-main`, else build would fail); (3) VPS backend unreachable / not HTTPS. Documented as operator steps in STATUS.
- **Code change — `client/index.html` hardened.** Removed a debug global error trap (`window.onerror` + `unhandledrejection` both `return true` / `preventDefault()`) that **swallowed all runtime errors** — turning a crash-on-mount into a silent blank screen and actively masking the very failure being diagnosed. Kept only a benign ResizeObserver-noise suppressor; real errors now reach console + React overlay. Repointed `og:image`/`twitter:image` from `replit.com` → `https://syncer-q.vercel.app/opengraph.jpg`; removed `twitter:site=@replit`.
- **Config — Node pin.** Added `"engines": { "node": ">=20.19" }` to `package.json` (Vite 7 minimum).
- **Build verification — exit 0.** First `npx vite build` **failed**: `Rollup failed to resolve "@vercel/analytics/react"`. Cause: the dep is declared (`^2.0.1`) but local `node_modules` was stale (not installed). `npm install` synced it; rebuild → `✓ built in 14.19s`, 3909 modules, `dist/public/index.html` (2.08 kB) + `assets/index-*.{js,css}` (JS 3.30 MB / gzip 898 kB — large-chunk warning noted, non-blocking). Served via `vite preview`: `/`=200, `/dashboard`=200 (SPA fallback), JS asset=200; served HTML has `<div id="root">`, zero TRAP/replit matches.
- **Branch consolidation — prune only (no merges).** `git branch --merged main` confirmed all 4 are ancestors of `main` (zero unique commits, `git diff main...<branch>` empty) ⇒ Task B's "sequential merge + conflict resolution" was moot. Deleted 3 local (`feat/google-signin` @15f114e, `feat/vercel-vps-split` @e60407d, `vercel/vercel-web-analytics-integrati-e5zi0k` @0dd24e9) with `git branch -d`; deleted all 4 on origin (those + remote-only `claude/secure-portfolio-repo-wkzhag`) with `git push origin --delete`; `git remote prune origin`. Final `git branch -a`: only `main` + `remotes/origin/main`.
- **Did NOT:** touch `vercel.json`/`vite.config.ts`/`apiBase.ts` (verified correct), backend code, or app logic. README/CLAUDE_MEMORY unchanged (no stack/persona shift).
- **Next:** commit + push `main` (triggers Vercel webhook redeploy); operator sets `VITE_API_URL` in Vercel + provisions/points the VPS backend (HTTPS) — the live site stays blank until then.

## 2026-06-28 — AWS EC2 Copy Trading Execution Engine: provisioning + deploy guide (advisory)

Task: produce an exact, cut-and-pasteable 4-phase guide to stand up a hardened AWS EC2 host as the dedicated low-latency Copy Trading Execution Engine (WS + rule-engine + Tradovate/Topstep order routing), with the Vercel SPA/API router calling it cross-origin. **Guide only — no AWS resources created, no repo files changed besides these lifecycle docs.**

- **Worked on:** Phase 1 (AWS CLI: `copy-trading-sg`, ingress 22-from-admin-IP / 80 / 443, t3.medium Ubuntu 22.04 + 30 GB gp3 encrypted, IMDSv2-required, T3-unlimited, Elastic IP). Phase 2 (`bootstrap.sh`: apt upgrade, `deployer` unprivileged user, Node 20 via NodeSource, Docker Engine+Compose plugin, PM2, UFW deny-in/allow 22+80+443, fail2ban, unattended-upgrades, post-verify SSH lockdown). Phase 3 (Path A: multi-stage Dockerfile → `dist/index.cjs` non-root uid 1001 + tini; `docker-compose.yml` with `env_file` 600, `restart: always`, json-file 10m×5 log cap, 65536 nofile ulimit, Caddy auto-TLS reverse proxy; Path B: `ecosystem.config.cjs` **fork/instances:1**, `node --env-file=.env`, pm2-logrotate). Phase 4 (bare repo + `post-receive` hook for `git push production main`; Vercel→AWS HMAC **reusing the existing `server/signal-routes.ts` scheme**; reboot via Docker `restart:always` / `pm2 startup` / a hardened systemd unit).
- **Aligned to real repo facts (not generic):** app listens `:5000`; build `tsx script/build.ts` → `dist/index.cjs`, start `node dist/index.cjs`, Node `>=20.19`; health `/healthz`; HMAC header `x-signature-256: sha256=<HMAC-SHA256(rawBody, SIGNAL_WEBHOOK_SECRET)>` with `timingSafeEqual` + `processed_signals` dedup already implemented — guide signs from Vercel, does not reinvent the verifier.
- **Key correctness calls:** (1) **fork/single-instance mandatory** — PM2 cluster would duplicate WS streams + the risk-enforcer in-memory idempotency `Set<accountId>` → double orders. (2) **Elastic IP = egress source** for broker whitelisting; Vercel serverless has no static egress IP, so ingress is gated by **TLS + HMAC, not IP allowlist**. (3) App port 5000 never opened at SG/UFW — reverse proxy reaches it internally.
- **Worked / didn't:** N/A (advisory; nothing executed). Recommended **Path A (Docker + Caddy)** as primary (matches repo's containerized backend + 2-line auto-TLS); systemd unit over pm2-startup for Path B.
- **Relation to the 2026-06-16 pivot:** this is the **single-VPS** host made concrete on EC2 (EIP model) — it does **not** revive the deleted multi-region Terraform/microservices stack. See DECISIONS (2026-06-28 — EC2 execution-engine host).

## 2026-06-28 — Production was 404 on EVERY path: Vercel building the repo root, not the app subdir (FIXED)

User: "syncer-q.vercel.app עדיין לא עובד" (still down). Ran systematic-debugging against the **live** site instead of re-asserting the prior session's theory — and the prior theory was **wrong**.

- **Evidence (live `curl`, not assumptions):** `GET /` → `HTTP 404`, `X-Vercel-Error: NOT_FOUND`, `Server: Vercel`, body "The page could not be found / NOT_FOUND". Probed 6 paths (`/`, `/favicon.ico`, `/opengraph.jpg`, `/assets/`, `/__does_not_exist__`, `/Vertex_Command-main/`) — **all 404**. The random path returning 404 (not the index) proved the **SPA rewrite was not applied** ⇒ `vercel.json` was **never read**.
- **Root cause:** Vercel project **Root Directory was the repo root**, not the `Vertex_Command-main/` app subdir. The repo root has no `package.json`, no app, and no `vercel.json`; Vercel auto-detected no framework and shipped an **empty static deploy** ⇒ a live deployment that 404s everywhere (`NOT_FOUND`, not `DEPLOYMENT_NOT_FOUND`). **The repo was correct all along** — subdir `vercel.json` (`outputDirectory dist/public`, SPA rewrite) is valid and `npx vite build` produces `dist/public/index.html`. Wrong layer was a **Vercel project setting**.
- **Corrects the prior misdiagnosis:** the 2026-06-28 "blank screen / set `VITE_API_URL`" entries assumed the bundle loaded and only API calls failed. False — the site returned **404**, the bundle never loaded. `VITE_API_URL` is a *later* concern (API calls), not the cause of "doesn't work."
- **Fix (committed `b1c6ba9`, pushed to `main`):** added a **repo-root `vercel.json`** that drives the nested build — `installCommand`/`buildCommand` `cd Vertex_Command-main && npm install` / `npx vite build`, `outputDirectory: Vertex_Command-main/dist/public`, `framework: null`, SPA rewrite. Works even with Root Directory left at the repo root; harmlessly ignored if Root Directory is set to the subdir (then the subdir `vercel.json` governs). User also setting dashboard Root Directory = `Vertex_Command-main` in parallel ("do both").
- **Verified live (post-redeploy):** `/` → **200** with real shell (`<html lang="he">`, `<div id="root">`, hashed `/assets/index-*.{js,css}`); `/dashboard` deep link → **200** (SPA fallback); JS asset → **200** `application/javascript`. Served bundle hash `index-D_E2tDmD.js` matches the local build ⇒ built from this source. Redeploy was live by the first poll.
- **Still pending (separate layer, not "down"):** for in-app API/auth calls to succeed, `VITE_API_URL` must point at a reachable **HTTPS backend** (the EC2 engine). Until then the shell loads but data calls fail — expected, and distinct from the 404 that's now resolved.
