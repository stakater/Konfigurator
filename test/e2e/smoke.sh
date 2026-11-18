#!/usr/bin/env bash
# e2e smoke test (H.7): the real image + Helm chart on a throwaway kind cluster.
#
# Proves the binary boots and the main loop works end to end:
#   render a template into a ConfigMap -> mount it into a Deployment ->
#   reject an invalid CR at admission (H.1 enum) -> clean up on delete.
#
# Usage: make e2e            (KEEP_CLUSTER=1 to keep the cluster for debugging)
# Needs: docker, kubectl, helm. KIND defaults to `kind` on PATH.
set -euo pipefail

KIND="${KIND:-kind}"
CLUSTER="${CLUSTER:-konfigurator-e2e}"
NS=konfigurator
IMG=konfigurator:e2e
APP=e2e-app
RENDERED="konfigurator-${APP}-rendered"
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
K="kubectl --context kind-${CLUSTER} -n ${NS}"

log()  { printf '\n▶ %s\n' "$*"; }
fail() { printf '\n✖ FAIL: %s\n' "$*" >&2; dump; exit 1; }
dump() {
  $K get all,cm,konfiguratortemplates 2>/dev/null || true
  $K logs deploy/konfigurator --tail=80 2>/dev/null || true
}
cleanup() {
  if [ "${KEEP_CLUSTER:-0}" = "1" ]; then
    echo "KEEP_CLUSTER=1: leaving cluster ${CLUSTER} running"
  else
    "$KIND" delete cluster --name "$CLUSTER" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

# wait_for <seconds> <description> <command...>: retry until the command succeeds.
wait_for() {
  local timeout=$1 what=$2; shift 2
  local deadline=$((SECONDS + timeout))
  until "$@" >/dev/null 2>&1; do
    [ $SECONDS -lt $deadline ] || fail "timed out after ${timeout}s waiting for: ${what}"
    sleep 3
  done
  echo "  ✓ ${what}"
}

log "kind cluster ${CLUSTER}"
"$KIND" get clusters 2>/dev/null | grep -qx "$CLUSTER" || "$KIND" create cluster --name "$CLUSTER" --wait 120s

log "build + load image ${IMG}"
docker build -q -t "$IMG" "$ROOT"
"$KIND" load docker-image "$IMG" --name "$CLUSTER"

log "install chart (with CRDs)"
helm --kube-context "kind-${CLUSTER}" upgrade --install konfigurator "$ROOT/charts/konfigurator" \
  -n "$NS" --create-namespace --wait --timeout 180s \
  --set deployCRD=true --set image.repository=konfigurator --set image.tag=e2e --set image.pullPolicy=Never

log "target app"
$K apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata: {name: ${APP}}
spec:
  replicas: 1
  selector: {matchLabels: {app: ${APP}}}
  template:
    metadata: {labels: {app: ${APP}}}
    spec:
      containers:
      - {name: app, image: registry.k8s.io/pause:3.10}
EOF
$K rollout status deploy/${APP} --timeout=120s

log "KonfiguratorTemplate renders pod names into a ConfigMap"
$K apply -f - <<EOF
apiVersion: konfigurator.stakater.com/v1alpha1
kind: KonfiguratorTemplate
metadata: {name: e2e}
spec:
  renderTarget: ConfigMap
  updateFrequency: 1
  app:
    name: ${APP}
    kind: Deployment
    volumeMounts:
    - {mountPath: /etc/rendered, container: app}
  templates:
    pods.txt: |
      {{- range .Pods }}{{ if eq .Namespace "${NS}" }}{{ .Name }}
      {{ end }}{{ end }}
EOF
rendered_has_app_pod() { $K get cm "$RENDERED" -o jsonpath='{.data.pods\.txt}' | grep -q "^${APP}-"; }
wait_for 180 "ConfigMap ${RENDERED} lists an ${APP} pod" rendered_has_app_pod

app_has_mount() {
  [ "$($K get deploy ${APP} -o jsonpath="{.spec.template.spec.volumes[?(@.name=='${RENDERED}')].configMap.name}")" = "$RENDERED" ] &&
  [ "$($K get deploy ${APP} -o jsonpath="{.spec.template.spec.containers[0].volumeMounts[?(@.name=='${RENDERED}')].mountPath}")" = "/etc/rendered" ]
}
wait_for 60 "Deployment ${APP} has the volume + mount" app_has_mount

log "invalid app.kind is rejected at admission (H.1)"
if out=$($K apply -f - 2>&1 <<EOF
apiVersion: konfigurator.stakater.com/v1alpha1
kind: KonfiguratorTemplate
metadata: {name: e2e-invalid}
spec:
  renderTarget: ConfigMap
  app: {name: ${APP}, kind: Job, volumeMounts: []}
  templates: {a: b}
EOF
); then fail "kind: Job was accepted: ${out}"; fi
echo "$out" | grep -qi 'Unsupported value' || fail "unexpected rejection message: ${out}"
echo "  ✓ rejected: $(echo "$out" | grep -io 'Unsupported value: "Job"' | head -1)"

log "delete cleans up"
$K delete konfiguratortemplate e2e --timeout=60s
configmap_gone() { ! $K get cm "$RENDERED"; }
wait_for 60 "ConfigMap ${RENDERED} deleted" configmap_gone
mount_gone() { [ -z "$($K get deploy ${APP} -o jsonpath="{.spec.template.spec.volumes[?(@.name=='${RENDERED}')].name}")" ]; }
wait_for 60 "volume removed from ${APP}" mount_gone

log "operator stayed healthy"
restarts=$($K get pods -l "$($K get deploy konfigurator -o jsonpath='{.spec.selector.matchLabels}' | python3 -c 'import json,sys;print(",".join(f"{k}={v}" for k,v in json.load(sys.stdin).items()))')" \
  -o jsonpath='{.items[*].status.containerStatuses[0].restartCount}')
[ "$restarts" = "0" ] || fail "operator restarted (${restarts})"
if $K logs deploy/konfigurator | grep -q 'panic'; then fail "panic in operator logs"; fi
echo "  ✓ 0 restarts, no panics"

printf '\n✔ e2e smoke passed\n'
