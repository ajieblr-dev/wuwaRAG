#!/bin/sh
# Static smoke check for web/ prototype (no server, no browser).
# - node --check syntax on app.js + inline scripts extracted from ask.html/search.html
# - verifies empty/pending/error state hooks exist in ask.html
# - verifies OD.api mock shapes + endpoint paths
# - verifies local href links resolve to files
set -eu
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
WEB="$ROOT/web"
fail=0
say() { printf '%s\n' "$*"; }
ok() { say "ok: $*"; }
bad() { say "FAIL: $*"; fail=1; }

# 1. files present
for f in app.js ask.html search.html index.html styles.css; do
  [ -f "$WEB/$f" ] && ok "web/$f exists" || bad "web/$f missing"
done

# 2. syntax: node --check app.js
if command -v node >/dev/null 2>&1; then
  if node --check "$WEB/app.js" 2>&1; then ok "node --check web/app.js"
  else bad "node --check web/app.js failed"; fi
  # inline scripts in ask.html / search.html: extract and check
  for page in ask.html search.html; do
    tmp="$(mktemp --suffix=.js)"
    # extract content between <script>...</script> without src=
    awk '/<script>/{flag=1;next}/<\/script>/{flag=0}flag' "$WEB/$page" > "$tmp"
    if [ -s "$tmp" ]; then
      if node --check "$tmp" 2>&1; then ok "node --check $page inline script"
      else bad "node --check $page inline script failed"; fi
    else
      bad "$page: no inline script extracted"
    fi
    rm -f "$tmp"
  done
else
  bad "node not found; skipping syntax checks"
fi

# 3. ask.html states: empty / pending / error (+ offline/aborted)
for pat in 'data-od-id="empty-state"' 'data-od-id="pending"' 'data-od-id="error-state"' 'banner-offline' 'banner-retrieval' 'renderEmpty' 'pendingHTML' 'renderError' 'renderNetworkError' 'renderAborted'; do
  if grep -q "$pat" "$WEB/ask.html"; then ok "ask.html has $pat"
  else bad "ask.html missing $pat"; fi
done

# 4. OD.api mock shapes + endpoint paths
for pat in '/api/v1/chat' '/api/v1/search' 'OD.api' 'mockSources' 'page_title' 'section_title' 'chunk_text'; do
  if grep -q "$pat" "$WEB/app.js"; then ok "app.js has $pat"
  else bad "app.js missing $pat"; fi
done

# 5. local href links resolve (href="*.html" without scheme)
missing=0
for page in "$WEB"/*.html; do
  grep -o 'href="[^"]*"' "$page" | sed 's/href="//;s/"//' | while IFS= read -r href; do
    case "$href" in
      http*|https*|mailto:*|"# "*|"#"'*'|"") continue;;
      \#*) continue;;
      *.html*) target="${href%%\#*}"; target="${target%%\?*}";
        if [ ! -f "$WEB/$target" ]; then echo "BROKEN: $page -> $href"; fi;;
    esac
  done
done > /tmp/smoke-links.txt 2>&1 || true
if grep -q BROKEN /tmp/smoke-links.txt; then cat /tmp/smoke-links.txt; bad "broken local links"; else ok "local href links resolve"; fi

if [ "$fail" -eq 0 ]; then say "SMOKE PASS"; else say "SMOKE FAIL"; fi
exit "$fail"
