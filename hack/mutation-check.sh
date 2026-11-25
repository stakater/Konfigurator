#!/usr/bin/env bash
# Mutation check for the KonfiguratorTemplate reconciler's error handling
# (PR #166, rounds 3-4). "Every error site has a test" is proven, not claimed.
#
# 1. The unchanged suite must pass (a red suite would hide survivors).
# 2. Every error return in the controller file is swallowed, one at a time, and
#    the suite must fail. Covers `return r.fail(...)`, `return err`,
#    `return nil, err` and `return ctrl.Result{}, err`.
# 3. fail() is broken four ways (permanent retried, next-cycle swallowed,
#    next-cycle ignores updateFrequency, everything terminal); each must fail.
# 4. At least MIN_MUTANTS mutants must have run, or the check is trivially green.
#
# Mutants are applied with `go test -overlay`: the tracked source is never edited,
# so an interrupted run cannot leave a mutant behind.
set -euo pipefail
cd "$(dirname "$0")/.."

f=controllers/konfiguratortemplate_controller.go
abs="$PWD/$f"
MIN_MUTANTS=${MIN_MUTANTS:-20}

tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT
mutant="$tmp/mutant.go"; overlay="$tmp/overlay.json"
printf '{"Replace":{"%s":"%s"}}\n' "$abs" "$mutant" > "$overlay"

echo "▶ baseline (unchanged suite must pass)"
go test -count=1 ./controllers >/dev/null 2>&1 || { echo "  ✖ the suite already fails; fix that before mutation testing"; exit 1; }
echo "  ✓ green"

survived=0; total=0
run_mutant() { # $1 = description; $mutant already written
  total=$((total+1))
  if ! go vet -overlay "$overlay" ./controllers >/dev/null 2>&1; then
    echo "  ? not compiled: $1"; survived=$((survived+1))
  elif go test -count=1 -overlay "$overlay" ./controllers >/dev/null 2>&1; then
    echo "  ✖ SURVIVED:     $1"; survived=$((survived+1))
  else
    echo "  ✓ killed:       $1"
  fi
}
mutate() { sed "$1" "$f" > "$mutant"; }          # $1 = sed program
mutate_append() { mutate "$1"; printf '%s\n' "$2" >> "$mutant"; }

echo "▶ swallowing each error return"
grep -nE 'return (r\.fail\(|err$|nil, err$|ctrl\.Result\{\}, err$)' "$f" | cut -d: -f1 | while read -r n; do
  line=$(sed -n "${n}p" "$f")
  case "$line" in
    *"return r.fail("*)   mutate "${n}s/return r\.fail(.*)$/return ctrl.Result{}, func() error { _ = err; return nil }()/" ;;
    *"return nil, err")    mutate "${n}s/return nil, err$/return nil, nil/" ;;
    *"return err")         mutate "${n}s/return err$/return nil/" ;;
    *"ctrl.Result{}, err")  mutate "${n}s/return ctrl\.Result{}, err$/return ctrl.Result{}, nil/" ;;
  esac
  run_mutant "$f:$n $(printf '%s' "$line" | sed 's/^[[:space:]]*//')"
done > "$tmp/sites.log"; cat "$tmp/sites.log"
# the while loop ran in a subshell; recover its counts from the log
total=$(grep -c '^  ' "$tmp/sites.log"); survived=$(grep -cE '^  (✖|\?)' "$tmp/sites.log" || true)

echo "▶ breaking fail()"
mutate_append 's/return ctrl.Result{}, reconcile.TerminalError(err)/return ctrl.Result{}, err/' 'var _ = reconcile.TerminalError // keep the import used in this mutant'
run_mutant "fail(): permanent errors retried"
mutate 's/return ctrl.Result{RequeueAfter: after}, nil/return ctrl.Result{}, nil/'
run_mutant "fail(): next-cycle errors swallowed"
mutate 's/after := renderInterval(instance)/after := DefaultRequeueTime/'
run_mutant "fail(): next-cycle ignores updateFrequency"
mutate 's/case reconcileerr.IsPermanent(err):/case err != nil:/'
run_mutant "fail(): every error terminal"

echo "$((total-survived))/$total mutants killed (minimum $MIN_MUTANTS)"
[ "$survived" -eq 0 ] && [ "$total" -ge "$MIN_MUTANTS" ]
