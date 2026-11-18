# Changelog

All notable changes to Konfigurator. Every user-visible change gets a line under *Unreleased*,
with upgrade notes for any CRD, RBAC, chart-value or behaviour change. Merging to `master`
publishes a release (`push.yml`), so this section must be current before merge.

## Unreleased

### Upgrade notes (read before upgrading)

1. **Validation webhooks are now opt-in (H.2).** `spec.validationWebhookURL` is only called when
   its host is on the admin allow-list: chart value `validationWebhook.allowedHosts`
   (env `VALIDATION_WEBHOOK_ALLOWED_HOSTS`). The default is empty, i.e. **webhooks disabled**.
   Every template that sets `validationWebhookURL` stops rendering until its host is
   allow-listed; the operator log says so, and the reconcile is retried with backoff.
   Action: set `validationWebhook.allowedHosts` before upgrading, preferably as `host:port`.
2. **Minimum Kubernetes 1.30 / OpenShift 4.17.** The chart enforces it via `kubeVersion`.
3. **CRD enums on `spec.app.kind` and `spec.renderTarget` (H.1).** Existing CRs with other
   values (`Job`, `configmap`, ...) are no longer reconciled and are logged as invalid. Before
   applying the new CRD, find and fix (or delete) them:
   ```sh
   kubectl get konfiguratortemplates -A -o json | jq -r '.items[]
     | select((.spec.app.kind | IN("Deployment","DaemonSet","StatefulSet") | not)
           or (.spec.renderTarget | IN("ConfigMap","Secret") | not))
     | "\(.metadata.namespace)/\(.metadata.name) kind=\(.spec.app.kind) target=\(.spec.renderTarget)"'
   ```
   On Kubernetes < 1.30 such a CR cannot be deleted once the new CRD is applied. Edit it first:
   `spec.app.kind` to `Deployment`/`DaemonSet`/`StatefulSet`, and `spec.renderTarget` to
   **`Secret`** (pre-upgrade operators rendered a Secret for any value other than `ConfigMap`,
   so that is the object the delete must clean up), then delete.
4. **`helm upgrade --reuse-values`** from an older release works (the new value defaults safely).

### Security
- Validation webhook: admin allow-list, 10s timeout, reconcile `ctx`, redirects refused,
  64 KiB / non-2xx rejected (review S2, H.2).
- Toolchain: Go 1.27.4, k8s v0.37.2, controller-runtime v0.25.3; `govulncheck` reports 0
  (was 15). Dead code and the `openshift/api` dependency removed (H.7).

### Fixed
- An unknown `app.kind` no longer crash-loops the operator; invalid CRs are rejected at
  admission and can always be deleted (H.1).
- Deleting a template whose workload is already gone, or whose spec is a legacy invalid value,
  now releases the finalizer and removes the rendered ConfigMap/Secret (PR #166 review).
- Errors are no longer silently dropped, and are handled by kind (PR #166 review):
  transient ones (webhook timeout/5xx, API conflict, app not present yet) are retried with
  backoff; ones that depend on cluster state (template execute error such as `index .Pods 0`
  on an empty list, webhook `allowed: false`, container not present yet) keep the normal
  `updateFrequency` render cycle so the template recovers by itself; permanent ones (template
  does not parse, webhook host not allow-listed, invalid spec) are terminal: logged and
  counted, not retried. Status conditions are still pending (H.6).
- A negative `updateFrequency` no longer causes a hot requeue loop; it means the default.
- Volume/container lookup no longer depends on `TypeMeta`, which newer controller-runtime
  clears after a `Get` (H.7, B12).

### Changed
- Image is multi-arch (`linux/amd64`, `linux/arm64`).
- Build tooling: pinned tools under `./bin`, `make verify` (vet, build, lint, race tests),
  `codegen-check`, `vulncheck`, kind-based `e2e`; committed binaries removed (H.7).
