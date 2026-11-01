#!/usr/bin/env bash
# Every message ID the backend hands to a client or stores in the database must
# exist in BOTH locale files, otherwise the API leaks a raw key like
# "errors.foo_bar" to clients (or into the login log / plugin error column).
#
# Covered references:
#   - "errors.*" literals passed to fail()/failData() in internal/httpapi and
#     internal/pkg/errors
#   - message IDs passed to i18n.StoredMessage("...") anywhere in the module
#   - "login_log.*" reasons passed to RecordLoginFailure
#
# Deliberately uses grep rather than ripgrep: rg is not installed on every
# developer machine, and a silently skipped scan is worse than no scan.
# Comparison is done as sorted-set subtraction (one process per file) because
# per-key grep loops are unbearably slow on Windows.
set -euo pipefail

cd "$(dirname "$0")/.."

locales="internal/pkg/i18n/locale"

used=$(
  {
    grep -rhoE '"errors\.[a-zA-Z0-9_.]+"' internal/httpapi internal/pkg/errors || true
    grep -rhoE 'StoredMessage\("[a-zA-Z0-9_.]+"' internal platform || true
    grep -rhoE '"login_log\.[a-zA-Z0-9_.]+"' internal || true
  } | grep -oE '"[a-zA-Z0-9_.]+"' | tr -d '"' | sort -u
)

total=$(printf '%s\n' "$used" | grep -c . || true)
if [ "$total" -eq 0 ]; then
  echo "check-i18n: no message keys found in internal/ platform/ (scan is broken)" >&2
  exit 1
fi

missing=0
for locale in zh en; do
  file="$locales/$locale.json"
  if [ ! -f "$file" ]; then
    echo "check-i18n: missing locale file $file" >&2
    exit 1
  fi
  present=$(grep -oE '"[a-zA-Z0-9_.]+"[[:space:]]*:' "$file" | grep -oE '"[a-zA-Z0-9_.]+"' | tr -d '"' | sort -u)
  absent=$(comm -23 <(printf '%s\n' "$used") <(printf '%s\n' "$present"))
  if [ -n "$absent" ]; then
    while IFS= read -r key; do
      echo "check-i18n: $locale.json is missing key \"$key\"" >&2
      missing=$((missing + 1))
    done <<<"$absent"
  fi
done

if [ "$missing" -gt 0 ]; then
  echo "check-i18n: $missing missing key(s) across zh/en" >&2
  exit 1
fi

echo "check-i18n: ok ($total keys in zh + en)"
