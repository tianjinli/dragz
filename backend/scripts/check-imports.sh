#!/usr/bin/env bash
# Enforce the three-layer dependency rule:
#
#     plugins/  ->  platform/  ->  kernel/
#
# Dependencies point one way only. Plugins never import each other and never
# reach into platform internals; platform and kernel never know about plugins.
#
# Two documented exceptions:
#   1. plugins/all is the single registration point, so it MAY import plugins.
#   2. plugins/system re-exports internal/httpapi declarations until P6 moves
#      the handlers into the plugin itself.
#
# Uses .Imports (not .TestImports) so tests may freely import plugins for
# baseline/golden purposes.
set -euo pipefail

cd "$(dirname "$0")/.."

module="dragz-admin/backend"
violations=0

report() {
  echo "check-imports: $*" >&2
  violations=$((violations + 1))
}

while IFS='|' read -r pkg imports; do
  [ -n "$pkg" ] || continue

  case "$pkg" in
    "$module"/plugins/*)
      owner="${pkg#"$module"/plugins/}"
      owner="${owner%%/*}"
      for imp in $imports; do
        case "$imp" in
          "$module"/plugins/*)
            if [ "$owner" != "all" ]; then
              other="${imp#"$module"/plugins/}"
              other="${other%%/*}"
              if [ "$other" != "$owner" ]; then
                report "$pkg must not import $imp (plugins are isolated; talk through platform interfaces)"
              fi
            fi
            ;;
          "$module/internal/httpapi")
            if [ "$owner" != "system" ]; then
              report "$pkg must not import $imp (only plugins/system may, and only until P6)"
            fi
            ;;
        esac
      done
      ;;

    "$module"/platform|"$module"/platform/*|"$module"/internal/kernel|"$module"/internal/kernel/*)
      for imp in $imports; do
        case "$imp" in
          "$module"/plugins/*)
            report "$pkg must not import $imp (platform and kernel are plugin-agnostic)"
            ;;
        esac
      done
      ;;
  esac
done < <(go list -f '{{.ImportPath}}|{{join .Imports " "}}' ./...)

if [ "$violations" -gt 0 ]; then
  echo "check-imports: $violations violation(s)" >&2
  exit 1
fi

echo "check-imports: ok"
