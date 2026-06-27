# Publishing the Clean Public Mirror

This document explains how to publish **Vertex Command** as a clean, public
portfolio repository — without exposing any secrets and without carrying over
the private git history.

## Why a fresh history?

The private repo's git history is already free of committed secrets (`.env`
files were always gitignored). However, earlier commits did contain personal
data (a real admin email and password in the seed scripts). Those values have
since been replaced with environment variables, but they still live in the
*old* history. Publishing a brand-new history drops them entirely, which is the
safest way to share the project publicly.

## What gets excluded

The mirror script (`scripts/make-public-mirror.sh`) copies the app and strips:

- **Secrets / runtime** — `.env`, `.env.*` (keeps `.env.example`), `backups/`,
  `*.sql.gz`, `*.dump`, `*.log`
- **Dependencies / build output** — `node_modules/`, `dist/`, `build/`,
  `.cache/`, `.venv/`, `__pycache__/`
- **Internal working docs / tooling** — `claude.md`, `CLAUDE_MEMORY.md`,
  `DECISIONS.md`, `PROGRESS.md`, `STATUS.md`, `.claude/`, `_bmad/`,
  `.obsidian/`, `attached_assets/`

It keeps `.env.example`, the source code, `README.md`, `LICENSE`, Docker/infra
config, and the `.github/` CI workflows (copied from the repo root).

The script then runs a secret scan and **aborts** if it finds anything that
looks like a key (`sk_live_`, `sk_test_`, `AKIA…`, private keys) or the old
personal data — so a contaminated mirror can never be created by accident.

## Option A — automated (recommended)

From a machine that has push access to GitHub:

```bash
# 1. Build the clean mirror (fresh git history) next to the repo
bash Vertex_Command-main/scripts/make-public-mirror.sh ~/vertex-command-public

# 2. Create an EMPTY public repo on GitHub (no README/license), e.g. "vertex-command"

# 3. Push
cd ~/vertex-command-public
git remote add origin git@github.com:<you>/vertex-command.git
git push -u origin main
```

## Option B — fully manual

```bash
# 1. Copy the app out, without .git
rsync -a --exclude='.git' --exclude='node_modules' --exclude='dist' \
  --exclude='.env' --exclude='.env.*' --include='.env.example' \
  Vertex_Command-main/ ~/vertex-command-public/

# 2. Remove internal docs you don't want public
cd ~/vertex-command-public
rm -f claude.md CLAUDE_MEMORY.md DECISIONS.md PROGRESS.md STATUS.md
rm -rf .claude _bmad .obsidian attached_assets

# 3. Fresh history + push
git init -b main
git add -A
git commit -m "Initial public release of Vertex Command"
git remote add origin git@github.com:<you>/vertex-command.git
git push -u origin main
```

## Before you flip it to Public — final checklist

- [ ] `.env` is **not** present in the mirror (only `.env.example`).
- [ ] `git log` in the mirror shows a single clean initial commit.
- [ ] Search returns nothing:
      `grep -rIn "www8351\|yosefrabitrade\|refael100A\|sk_live_\|sk_test_" .`
- [ ] Set the real values via environment variables when running locally
      (`ADMIN_EMAILS`, `SEED_ADMIN_*`, `SEED_DEMO_PASSWORD`, etc.).
