#!/usr/bin/env bash
# tools/check-rebrand.sh — rebrand guard. Fails when tracked files still point
# downloads, self-update, raw scripts or container images at upstream 3x-ui.
# Allowed on purpose: the Go module path (github.com/mhsanaei/3x-ui/v3, D4),
# links to upstream issues/PRs/commits (credit), the third-party mtg-multi
# dependency, and the credit files listed in ALLOW_FILES.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

ALLOW_FILES='^(LICENSE|NOTICE|README\.md|README\.zh_CN\.md|docs/LX_CHANGES\.md|docs/UPSTREAM_SYNC\.md|tools/rebrand\.sh|tools/check-rebrand\.sh|tools/rebrand\.env|go\.mod|go\.sum)$'
PATTERN='(raw\.githubusercontent\.com/mhsanaei/3x-ui|api\.github\.com/repos/mhsanaei/3x-ui|github\.com/mhsanaei/3x-ui/releases|ghcr\.io/mhsanaei/3x-ui|github\.com/mhsanaei/3x-ui(\.git|/(tree|blob|archive|raw)/|["'"'"' )>]|$))'

fail=0
hits=$(git ls-files -z | grep -zvE "$ALLOW_FILES" | grep -zvE '^(frontend/node_modules|internal/web/dist|media)/' \
    | xargs -0 grep -nIiE "$PATTERN" 2>/dev/null \
    | grep -viE 'github\.com/mhsanaei/3x-ui/(issues|pull|commit|discussions|security|compare)' \
    | grep -v 'lxui:keep' || true)
if [[ -n "$hits" ]]; then
    echo "rebrand guard: upstream download/update/image URLs found:" >&2
    echo "$hits" >&2
    fail=1
fi

# The brand identity must be consistent with tools/rebrand.env.
# shellcheck disable=SC1091
source tools/rebrand.env
if [[ "$(tr -d '\n' < internal/config/repo)" != "$LXUI_REPO" ]]; then
    echo "rebrand guard: internal/config/repo does not match LXUI_REPO in tools/rebrand.env (run tools/rebrand.sh)" >&2
    fail=1
fi
if ! tools/rebrand.sh --check > /tmp/lxui-rebrand-check.$$ 2>&1; then
    echo "rebrand guard: tools/rebrand.sh would still change files:" >&2
    cat /tmp/lxui-rebrand-check.$$ >&2
    fail=1
fi
rm -f /tmp/lxui-rebrand-check.$$
[[ $fail -eq 0 ]] && echo "rebrand guard: OK"
exit $fail
