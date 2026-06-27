#!/usr/bin/env bash
#
# make-public-mirror.sh — build a clean, fresh-history public mirror of the app.
#
# Produces a standalone copy of the Vertex Command project (the contents of
# Vertex_Command-main/ at the new repo root) with a brand-new git history, so
# none of the original private history is carried over. Secrets, build
# artifacts, dependencies, and internal working docs are excluded; .env.example
# is kept as the configuration template.
#
# Usage:
#   bash scripts/make-public-mirror.sh [TARGET_DIR]
#
# Default TARGET_DIR: ../vertex-command-public (sibling of the repo).
# The script refuses to overwrite a non-empty existing directory.
#
# After it finishes, push from your machine that has GitHub access:
#   cd <TARGET_DIR>
#   git remote add origin git@github.com:<you>/<public-repo>.git
#   git push -u origin main
#
set -euo pipefail

# --- Resolve paths -----------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"   # Vertex_Command-main
REPO_ROOT="$(cd "$PROJECT_ROOT/.." && pwd)"    # repo root (holds .github/)
TARGET_DIR="${1:-$REPO_ROOT/../vertex-command-public}"

echo "==> Source project : $PROJECT_ROOT"
echo "==> Target mirror  : $TARGET_DIR"

# --- Safety check ------------------------------------------------------------
if [ -e "$TARGET_DIR" ] && [ -n "$(ls -A "$TARGET_DIR" 2>/dev/null || true)" ]; then
  echo "ERROR: target '$TARGET_DIR' exists and is not empty. Aborting." >&2
  exit 1
fi
mkdir -p "$TARGET_DIR"

# Internal working files / tooling that must NOT ship in the public mirror.
# (Patterns are matched against the git-relative path of each file.)
EXCLUDE_RE='^(claude\.md|CLAUDE\.md|CLAUDE_MEMORY\.md|DECISIONS\.md|PROGRESS\.md|STATUS\.md|PUBLIC_MIRROR\.md|scripts/make-public-mirror\.sh|\.claude/|_bmad/|\.obsidian/|attached_assets/)'

# Copy the git-relative file list from $1 (a repo path) into $TARGET_DIR.
# Uses `git ls-files` so .gitignore is honoured automatically (node_modules,
# .env, dist, backups, etc. are excluded) while new untracked-but-not-ignored
# files (LICENSE, …) are still included via --others --exclude-standard.
copy_tracked() {
  local src="$1" prefix="${2:-}" f
  ( cd "$src" && git ls-files --cached --others --exclude-standard ) \
  | while IFS= read -r f; do
      case "$prefix$f" in
        */node_modules/*|node_modules/*) continue ;;
      esac
      if printf '%s\n' "$f" | grep -qE "$EXCLUDE_RE"; then continue; fi
      mkdir -p "$TARGET_DIR/$prefix$(dirname "$f")"
      cp -p "$src/$f" "$TARGET_DIR/$prefix$f"
    done
}

# --- Copy project files (clean) ---------------------------------------------
copy_tracked "$PROJECT_ROOT"
echo "==> Copied project source (git-tracked, secrets/junk excluded)"

# --- Bring in CI workflows from the repo root, if present --------------------
if [ -d "$REPO_ROOT/.github" ]; then
  ( cd "$REPO_ROOT" && git ls-files --cached --others --exclude-standard -- .github ) \
  | while IFS= read -r f; do
      mkdir -p "$TARGET_DIR/$(dirname "$f")"
      cp -p "$REPO_ROOT/$f" "$TARGET_DIR/$f"
    done
  echo "==> Copied .github/ (CI workflows)"
fi

# --- Guard: fail loudly if any secret-looking content slipped through --------
echo "==> Scanning mirror for secrets..."
# Prefixes require trailing key-like chars so we don't flag tools that merely
# reference the patterns (e.g. the CI secret-scan step in production.yml).
if grep -rInE 'sk_live_[A-Za-z0-9]{16,}|sk_test_[A-Za-z0-9]{16,}|AKIA[A-Z0-9]{16}|-----BEGIN [A-Z ]*PRIVATE KEY-----|www8351|yosefrabitrade|refael100A' "$TARGET_DIR" \
     --exclude-dir=node_modules 2>/dev/null; then
  echo "ERROR: potential secret/personal data found in mirror (see above). Aborting." >&2
  exit 1
fi
echo "    clean."

# --- Fresh git history -------------------------------------------------------
cd "$TARGET_DIR"
git init -q -b main
git add -A
git -c user.name="Vertex Command" -c user.email="noreply@example.com" \
    commit -q -m "Initial public release of Vertex Command

Clean public mirror: full React 19 + TypeScript + Vite frontend and
Node/Express + PostgreSQL + Python FastAPI backend. All secrets are
sourced from environment variables (see .env.example)."

echo ""
echo "================================================================"
echo " Clean public mirror ready at: $TARGET_DIR"
echo "================================================================"
echo " Next steps (from a machine with GitHub access):"
echo "   cd \"$TARGET_DIR\""
echo "   git remote add origin git@github.com:<you>/<public-repo>.git"
echo "   git push -u origin main"
echo "================================================================"
