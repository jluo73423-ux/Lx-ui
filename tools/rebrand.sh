#!/usr/bin/env bash
# tools/rebrand.sh — reproducible 3x-ui -> Lx-ui rebrand.
#
# * Reads the brand identity from tools/rebrand.env (single source of truth).
# * Renames tracked files whose names carry the upstream brand.
# * Rewrites brand tokens and upstream download/self-update URLs in every
#   tracked text file, while PROTECTING:
#     - the Go module path github.com/mhsanaei/3x-ui/v3 (kept on purpose, D4),
#     - links to upstream issues/PRs/commits and "MHSanaei/3x-ui#123" refs (credit),
#     - the third-party mtg-multi dependency (github.com/mhsanaei/mtg-multi),
#     - any line containing the marker "lxui:keep" (legacy-compat code).
# * Writes the derived identity files (internal/config/repo,
#   frontend/src/lib/brand.ts) and the identity block of the shell scripts.
# * Idempotent: re-running is a no-op; changing LXUI_REPO in rebrand.env and
#   re-running moves every reference to the new repository.
#
# Usage: tools/rebrand.sh [--check]     (--check: report what would change, exit 1 if anything)
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
# shellcheck disable=SC1091
source tools/rebrand.env
export LXUI_REPO LXUI_DISPLAY LXUI_NAME
export REBRAND_CHECK=0
[[ "${1:-}" == "--check" ]] && REBRAND_CHECK=1

python3 - <<'PY'
import os, re, subprocess, sys

ROOT = os.getcwd()
REPO = os.environ["LXUI_REPO"]
DISPLAY = os.environ["LXUI_DISPLAY"]
NAME = os.environ["LXUI_NAME"]
CHECK = os.environ.get("REBRAND_CHECK") == "1"
OWNER_LC = REPO.split("/")[0].lower()

changed = []

def git(*args):
    return subprocess.run(["git", *args], check=True, capture_output=True, text=True).stdout

# Files never touched (hand-written Lx-ui docs, licences, lockfiles of record,
# legacy-compat code that must name the old paths, this tooling itself).
EXCLUDE_EXACT = {
    "LICENSE", "NOTICE", "README.md", "README.zh_CN.md",
    "docs/LX_CHANGES.md", "docs/UPSTREAM_SYNC.md",
    "go.mod", "go.sum",
    "tools/rebrand.sh", "tools/rebrand.env", "tools/check-rebrand.sh",
    "internal/config/legacy.go", "internal/config/legacy_test.go", "legacy_migrate.go",
}
EXCLUDE_PREFIX = ("frontend/node_modules/", "internal/web/dist/", "media/")
BINARY_EXT = {".png", ".jpg", ".jpeg", ".gif", ".ico", ".webp", ".woff", ".woff2",
              ".ttf", ".eot", ".zip", ".gz", ".tgz", ".xz", ".exe", ".dll", ".so",
              ".syso", ".pdf", ".mp4", ".db", ".dat"}

def excluded(path):
    if path in EXCLUDE_EXACT or path.startswith(EXCLUDE_PREFIX):
        return True
    if os.path.splitext(path)[1].lower() in BINARY_EXT:
        return True
    return False

# ---------------------------------------------------------------- 1. renames
RENAME_RULES = [
    (re.compile(r"3x-ui"), NAME),
    (re.compile(r"(?<![A-Za-z0-9])x-ui(?![A-Za-z0-9])"), NAME),
]
for path in git("ls-files").splitlines():
    if path.startswith(EXCLUDE_PREFIX) and not path.startswith("media/"):
        continue
    if os.path.basename(path).startswith("README"):
        continue
    d, b = os.path.split(path)
    nb = b
    for rx, rep in RENAME_RULES:
        nb = rx.sub(rep, nb)
    if nb != b:
        dst = os.path.join(d, nb)
        changed.append(f"rename {path} -> {dst}")
        if not CHECK:
            git("mv", path, dst)

# ------------------------------------------------------- 2. content rewrite
SENT = "\x00LXKEEP{}\x00"
PROTECT = [
    re.compile(r"github\.com/mhsanaei/3x-ui/v3"),                      # Go module path (D4)
    re.compile(r"(?i)github\.com/mhsanaei/mtg-multi"),                 # 3rd-party dependency
    re.compile(r"(?i)mhsanaei/mtg-multi"),
    re.compile(r"(?i)github\.com/mhsanaei/3x-ui/(?:issues|pull|commit|discussions|security|compare)\b[^\s\"')>\]]*"),
    re.compile(r"(?i)mhsanaei/3x-ui#\d+"),
    re.compile(r"(?i)github\.com/(?!mhsanaei/)[\w.-]+/x-ui\b"),        # other projects named x-ui
]

def shell_rules():
    return [
        (re.compile(r"(?i)https://raw\.githubusercontent\.com/mhsanaei/3x-ui"), "${LXUI_RAW_BASE}"),
        (re.compile(r"(?i)https://github\.com/mhsanaei/3x-ui/releases"), "${LXUI_RELEASE_BASE}"),
        (re.compile(r"(?i)https://api\.github\.com/repos/mhsanaei/3x-ui"), "https://api.github.com/repos/${LXUI_REPO}"),
    ]

def common_rules(path):
    ghcr = ("ghcr.io/${{ github.repository_owner }}/" + NAME
            if path.startswith(".github/workflows/") else f"ghcr.io/{OWNER_LC}/{NAME}")
    return [
        (re.compile(r"(?i)ghcr\.io/mhsanaei/3x-ui"), ghcr),
        (re.compile(r"(?i)\bmhsanaei/3x-ui\b"), REPO),
        (re.compile(r"3X-UI|3X-ui"), DISPLAY),
        (re.compile(r"3x-UI|3x-ui"), NAME),
        (re.compile(r"(?<![A-Za-z0-9])X-UI(?![A-Za-z0-9])"), DISPLAY),
        (re.compile(r"(?<![A-Za-z0-9])x-ui(?![A-Za-z0-9])"), NAME),
        (re.compile(r"3xui"), NAME.replace("-", "")),
    ]

def rewrite(text, path):
    out_lines = []
    is_shell = path.endswith(".sh") or path in ("lx-ui.sh",) or text.startswith("#!/bin/bash") or text.startswith("#!/usr/bin/env bash") or text.startswith("#!/bin/sh")
    rules = (shell_rules() if is_shell else []) + common_rules(path)
    for line in text.splitlines(keepends=True):
        if "lxui:keep" in line:
            out_lines.append(line)
            continue
        saved = []
        def stash(m):
            saved.append(m.group(0))
            return SENT.format(len(saved) - 1)
        for rx in PROTECT:
            line = rx.sub(stash, line)
        for rx, rep in rules:
            line = rx.sub(lambda m, rep=rep: rep, line)
        for i, s in enumerate(saved):
            line = line.replace(SENT.format(i), s)
        out_lines.append(line)
    return "".join(out_lines)

files = git("ls-files").splitlines()
if CHECK:
    pass
for path in files:
    if excluded(path) or not os.path.isfile(path):
        continue
    try:
        with open(path, "r", encoding="utf-8", newline="") as f:
            text = f.read()
    except (UnicodeDecodeError, IsADirectoryError):
        continue
    new = rewrite(text, path)
    if new != text:
        changed.append(f"edit {path}")
        if not CHECK:
            with open(path, "w", encoding="utf-8", newline="") as f:
                f.write(new)

# --------------------------------------------- 3. derived identity files
def ensure(path, content):
    old = open(path, encoding="utf-8").read() if os.path.exists(path) else None
    if old != content:
        changed.append(f"write {path}")
        if not CHECK:
            os.makedirs(os.path.dirname(path), exist_ok=True)
            with open(path, "w", encoding="utf-8") as f:
                f.write(content)
    return old

old_repo = None
if os.path.exists("internal/config/repo"):
    old_repo = open("internal/config/repo", encoding="utf-8").read().strip()
ensure("internal/config/repo", REPO + "\n")
ensure("frontend/src/lib/brand.ts",
       "// Generated by tools/rebrand.sh from tools/rebrand.env. Do not edit by hand.\n"
       f"export const BRAND_NAME = '{DISPLAY}';\n"
       f"export const BRAND_ID = '{NAME}';\n"
       f"export const LXUI_REPO = '{REPO}';\n"
       f"export const REPO_URL = `https://github.com/${{LXUI_REPO}}`;\n")

# A repository move: rewrite literal occurrences of the previous value.
if old_repo and old_repo != REPO:
    old_owner_lc = old_repo.split("/")[0].lower()
    for path in git("ls-files").splitlines():
        if excluded(path) or not os.path.isfile(path):
            continue
        try:
            text = open(path, encoding="utf-8", newline="").read()
        except UnicodeDecodeError:
            continue
        new = text.replace(old_repo, REPO).replace(f"ghcr.io/{old_owner_lc}/{NAME}", f"ghcr.io/{OWNER_LC}/{NAME}")
        if new != text:
            changed.append(f"repo-move {path}")
            if not CHECK:
                open(path, "w", encoding="utf-8", newline="").write(new)
    for p in ("README.md", "README.zh_CN.md"):
        if os.path.exists(p):
            t = open(p, encoding="utf-8").read()
            n = t.replace(old_repo, REPO).replace(f"ghcr.io/{old_owner_lc}/{NAME}", f"ghcr.io/{OWNER_LC}/{NAME}")
            if n != t:
                changed.append(f"repo-move {p}")
                if not CHECK:
                    open(p, "w", encoding="utf-8").write(n)

# ------------------------------------------ 4. shell identity block
BEGIN = "# >>> lx-ui identity (managed by tools/rebrand.sh; edit tools/rebrand.env) >>>"
END = "# <<< lx-ui identity <<<"
BLOCK = f'''{BEGIN}
LXUI_REPO="${{LXUI_REPO:-{REPO}}}"
LXUI_RAW_BASE="https://raw.githubusercontent.com/${{LXUI_REPO}}"
LXUI_RELEASE_BASE="https://github.com/${{LXUI_REPO}}/releases"
# LXUI_* variables are canonical; the legacy XUI_* names keep working as aliases.
# An LXUI_ value wins over the XUI_ alias of the same name.
while IFS='=' read -r __lxui_k __lxui_v; do
    case "$__lxui_k" in LXUI_REPO | LXUI_RAW_BASE | LXUI_RELEASE_BASE) continue ;; esac
    export "XUI_${{__lxui_k#LXUI_}}=${{__lxui_v}}"
done < <(env | grep -E '^LXUI_[A-Z0-9_]+=' || true)
unset __lxui_k __lxui_v
{END}
'''
for script in ("install.sh", "update.sh", f"{NAME}.sh"):
    if not os.path.exists(script):
        continue
    text = open(script, encoding="utf-8").read()
    if BEGIN in text:
        new = re.sub(re.escape(BEGIN) + r".*?" + re.escape(END) + r"\n", lambda m: BLOCK, text, flags=re.S)
    else:
        lines = text.splitlines(keepends=True)
        # insert right after the shebang line
        new = lines[0] + "\n" + BLOCK + "".join(lines[1:])
    if new != text:
        changed.append(f"identity-block {script}")
        if not CHECK:
            open(script, "w", encoding="utf-8").write(new)

print(f"rebrand: {len(changed)} change(s)" + (" (check mode)" if CHECK else ""))
for c in changed[:50]:
    print("  " + c)
if len(changed) > 50:
    print(f"  ... and {len(changed) - 50} more")
if CHECK and changed:
    sys.exit(1)
PY
