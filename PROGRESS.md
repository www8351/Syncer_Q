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

## 2026-06-15 — Repo/disk cleanup (Tiers A + B + C)

Inventory first: `.git` only 4.3 MB (repo NOT bloated); disk hogs were local/ignored (`node_modules` 707 MB, `attached_assets` 47 MB, `dist` 6.2 MB).

- **Tier A (local rm, not in git):** logs (`dev-server.log`, `vite-dev.log`), `dist/`, `.cache/ .agents/ .canvas/`, `artifacts/`, `.replit`, empty `_bmad-output/ backups/`, editor swap file, and 3 stale untracked duplicate `STATUS/PROGRESS/DECISIONS.md` inside the nested dir. Verified none were git-tracked.
- **Tier B (`git rm`):** `python_fixes/` (4 files, no refs), `PRESENTATION.md`, `vertex-command-spec.md`.
- **Tier C:** `git rm _bmad/` (12 tracked files — recoverable from history; disables `bmad-*` skills until reinstalled). `attached_assets/` pruned **120 → 7 files** (deleted 113 old prompts/screenshots/images/mp4; kept 6 `logo-vertex-*.png` + `.gitkeep`), 47 MB → 3.1 MB.
- **Kept by user choice:** `node_modules` (707 MB, `npm ci` to rebuild), `_bmad` ignored remnants (10 KB).
- **Safety gates honored:** secrets `.env` + `.stripe-keys.json` untouched; irreversible local media deletion done only after explicit per-bucket confirmation (auto-classifier blocked the first bundled attempt); zero essential build/app files removed.
- **Note:** pre-existing uncommitted working-tree changes (Google Sign-In WIP: `useAuth.ts`, `AuthPage.tsx`, `locales/*`, `server/{index,routes,storage}.ts`, `package.json`, `.env.example`) were left untouched.
