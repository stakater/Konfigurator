#!/usr/bin/env bash
# Mutation check for the reconciler's error handling (PR #166, round 3, X14).
#
# "Every error site has a test" is proven, not claimed: each `return fail(...)` in
# the KonfiguratorTemplate reconciler is replaced, one at a time, by a call that
# swallows the error; the suite must then fail. fail() itself is then broken both
# ways (never terminal / always terminal); the suite must fail both times.
# A mutant that compiles and passes the suite is an uncovered site: fix the tests.
set -euo pipefail
cd "$(dirname "$0")/.."

f=controllers/konfiguratortemplate_controller.go
cp "$f" "$f.orig"
trap 'mv "$f.orig" "$f"' EXIT

survived=0; total=0
run_mutant() { # $1 = description; the mutated $f is in place
  total=$((total+1))
  if ! go vet ./controllers >/dev/null 2>&1; then
    echo "  ? not compiled: $1"; survived=$((survived+1))
  elif go test -count=1 ./controllers >/dev/null 2>&1; then
    echo "  ✖ SURVIVED:     $1"; survived=$((survived+1))
  else
    echo "  ✓ killed:       $1"
  fi
  cp "$f.orig" "$f"
}

echo "▶ swallowing each error site"
while read -r n; do
  # keep err "used" so the mutant compiles at every site
  sed -i.bak "${n}s/return fail(\(.*\))$/return swallow(\1)/" "$f" && rm -f "$f.bak"
  printf '\nfunc swallow(error) (ctrl.Result, error) { return ctrl.Result{}, nil }\n' >> "$f"
  run_mutant "$f:$n $(sed -n "${n}p" "$f.orig" | sed 's/^[[:space:]]*//')"
done < <(grep -n 'return fail(' "$f" | cut -d: -f1)

echo "▶ breaking fail()"
sed -i.bak 's/return ctrl.Result{}, reconcile.TerminalError(err)/return ctrl.Result{}, err/' "$f" && rm -f "$f.bak"
printf '\nvar _ = reconcile.TerminalError // keep the import used in this mutant\n' >> "$f"
run_mutant "fail(): permanent errors retried"
sed -i.bak 's/if reconcileerr.IsPermanent(err) {/if err != nil {/' "$f" && rm -f "$f.bak"
run_mutant "fail(): every error terminal"

echo "$((total-survived))/$total mutants killed"
[ "$survived" -eq 0 ]
