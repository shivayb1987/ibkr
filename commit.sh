#!/usr/bin/env zsh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

git add **/*.go *.sh
git add .env
git add commit.sh
git add contracts.txt
git add main.go
git add zshrc.txt

STAGED=$(git diff --cached --name-only 2>/dev/null || true)
if [[ -z "$STAGED" ]]; then
  echo "Nothing staged."
  exit 0
fi

REVIEW_FILES=$(echo "$STAGED" | grep -E '\.go$' || true)

echo "Staged:"
echo "$STAGED"
echo ""

if [[ -z "$REVIEW_FILES" ]]; then
  echo "No .go files staged; skipping Cursor review."
  exit 0
fi

echo "Reviewing (.go only):"
echo "$REVIEW_FILES"
echo ""

read_cursor_api_key_from_file() {
  local file=$1
  [[ -f "$file" ]] || return 1

  local line
  line=$(grep -E '^[[:space:]]*(export[[:space:]]+)?CURSOR_API_KEY=' "$file" | tail -1 || true)
  [[ -n "$line" ]] || return 1

  CURSOR_API_KEY="${line#*=}"
  CURSOR_API_KEY="${CURSOR_API_KEY#export }"
  CURSOR_API_KEY="${CURSOR_API_KEY#CURSOR_API_KEY=}"
  CURSOR_API_KEY="${CURSOR_API_KEY//\"/}"
  CURSOR_API_KEY="${CURSOR_API_KEY//\'/}"
  CURSOR_API_KEY="${CURSOR_API_KEY##[[:space:]]}"
  CURSOR_API_KEY="${CURSOR_API_KEY%%[[:space:]]}"
  [[ -n "$CURSOR_API_KEY" ]] || return 1
}

load_cursor_api_key() {
  if [[ -n "${CURSOR_API_KEY:-}" ]]; then
    export CURSOR_API_KEY
    return 0
  fi

  # Empty export blocks agent login fallback.
  unset CURSOR_API_KEY

  # ./commit.sh is non-interactive, so ~/.zshrc is not auto-sourced.
  read_cursor_api_key_from_file "$ROOT/.env" \
    || read_cursor_api_key_from_file "${HOME}/.zshenv" \
    || read_cursor_api_key_from_file "${HOME}/.zshrc" \
    || true

  if [[ -n "${CURSOR_API_KEY:-}" ]]; then
    export CURSOR_API_KEY
  fi
}

review_staged() {
  if ! command -v agent >/dev/null 2>&1; then
    echo "warning: cursor agent CLI not found; skipping review" >&2
    return 0
  fi

  load_cursor_api_key

  local agent_args=(--print --trust --workspace "$ROOT")
  if [[ -n "${CURSOR_API_KEY:-}" ]]; then
    agent_args+=(--api-key "$CURSOR_API_KEY")
  fi

  local review_paths
  review_paths=$(echo "$REVIEW_FILES" | tr '\n' ' ')

  echo "Cursor reviewing staged changes..."
  if ! agent "${agent_args[@]}" \
    "Bugbot-style review of staged changes only.

Full Repository Path: $ROOT
Run: git diff --cached -- $review_paths

Review only these staged .go files. Ignore all other staged files.
Do not edit files.
Output:
1. One-line summary
2. Markdown table: Severity | Location | Finding
   Use file:line in Location, or 'No issues found' if clean."; then
    echo "error: Cursor review failed. Set CURSOR_API_KEY in .env or run: agent login" >&2
    return 1
  fi
}

review_staged
