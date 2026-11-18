# Baseline code review — 2026-11-17

Scope: whole repo at `d7b514d` (master). Method: read every non-generated Go
file, the CRDs, the Helm chart, the Dockerfile, and the CI workflows; ran
`go build`, `go vet`, `go test -race`, and `govulncheck` locally.

**Severity:** 🔴 high (crash, data loss/leak, privilege boundary) · 🟠 medium
(wrong behavior users will hit) · 🟡 low (hygiene, latent).
**Confidence:** *Confirmed* = traced in code, and checked in a library where
relevant · *Plausible* = reasoned from the design, not reproduced.
Each fix becomes a 📝 Draft story in [`../roadmap.md`](../roadmap.md). **None
of these is approved yet.**

## Baseline signals

| Check | Result |
|---|---|
| `go build ./...` | ✅ green (local Go 1.27.1, module declares `go 1.17`) |
| `go vet ./...` | ✅ green |
| `go test -race ./...` | ✅ green. **This doesn't prove the code is race-free:** no test runs reconcilers concurrently (see B2). |
| `golangci-lint` | ⚪ not assessed: not installed, and the repo has no config |
| `govulncheck` | ✅ **0** after H.7 (Go 1.27.4, x/net v0.63.0, govulncheck v1.10.0). Baseline was ❌ **15 reachable vulnerabilities.** 9 are in `golang.org/x/net@v0.17.0` / `x/text@v0.14.0`; the rest are in the stdlib of the *local* toolchain. The shipped image builds on **`golang:1.17`** (EOL Aug 2022), so production carries far more stdlib CVEs than this scan shows. |
| `make test` | ⚪ not run. It downloads `setup-envtest.sh` from controller-runtime v0.7, a mechanism that has since been removed. |
| Repo hygiene | `bin/manager` (44 MB) and `bin/controller-gen` (21 MB) are **committed** |

## Correctness bugs

### 🔴 B1. Deleted Pods/Services/Ingresses are never evicted from the render context. *Confirmed*
`controllers/pod_controller.go:51`, `service_controller.go:51`, `ingress_controller.go:52`.
When `Get` returns NotFound, the code calls `RemoveFromContext(instance.Name, instance.Namespace)`.
But `instance` is the empty struct that `Get` failed to fill, so both fields are `""` and
nothing matches. Every deleted object stays in the in-memory context **forever**.
**Effect:** templates such as the fluentd example keep rendering dead pods and endpoints,
and memory grows with pod churn.
**Fix:** use `req.Name` / `req.Namespace`.

### 🔴 B2. The shared render context is mutated from 4 goroutines with no lock. *Confirmed by design*
`pkg/context/context.go`, wired in `main.go`.
One `xContext.Context` (plain slices) is written by the Pod, Service, Ingress, **and**
PodMetadataInjector reconcilers. Each one runs in its own controller goroutine. The
template reconciler reads it while they write. Two goroutines append to `Pods` at once
(Pod and PodMetadataInjector), and reads overlap with in-place `append`-deletes. That's a
textbook data race: corrupted slices, lost updates, possible panics.
**Fix (preferred, simpler):** delete the hand-rolled mirror. At render time, `List` from
the manager's informer cache, which is already thread-safe and already kept current by
the same watches. This also fixes B1, B5, and B6 at the root.

### 🔴 B3. An unknown `app.kind` crashes the operator in a loop. *Confirmed* — ✅ **closed by H.1**
`pkg/kube/factory.go:31` + CRD has no `enum` on `spec.app.kind`.
`appKindToObjectMap[kind]` returns a `nil` interface for any kind except
Deployment/DaemonSet/StatefulSet. The next line, `appObject.SetName(...)`, panics.
controller-runtime v0.9 doesn't recover reconciler panics, so the process exits.
On restart it reconciles the same CR and crashes again.
**Effect:** anyone who can create a KonfiguratorTemplate (`kind: Job`, or a typo) takes
Konfigurator down **for every tenant**.
**Fix:** add a CRD enum, return an error instead of panicking, and turn on `RecoverPanic`.

### 🟠 B4. Template failures are never retried, and nobody sees them. *Confirmed*
`controllers/konfiguratortemplate_controller.go:105-178`, all through `ManageError(..., false)`.
In operator-utils v0.1.14, `isRetriable=false` returns `(Result{}, nil)`, so the template
isn't requeued. It sets status only if the object implements `ConditionsStatusAware`, and
that code is commented out (`konfiguratortemplate_types.go`). So a template created
*before* its Deployment, a webhook blip, or an API conflict leaves the template **silently
stuck** until someone edits it or the operator restarts.
**Fix:** requeue transient errors with backoff, and publish `Ready`/`Error` conditions in
`.status` (observedGeneration).

### 🟠 B5. On a cold start, empty or partial config can be written over good config. *Plausible*
The render context fills asynchronously as Pod/Service/Ingress reconcilers process their
initial lists. Nothing makes the template reconciler wait for that. After an operator
restart, a template can render against an empty context and overwrite the good ConfigMap.
Every consuming pod (e.g. fluentd) then reloads broken config.
**Fix:** falls out of B2. Reading from the synced cache means rendering never starts
before the caches are synced.

### 🟠 B6. Objects being deleted are put straight back into the context. *Confirmed*
`pod_controller.go:61-68` (same in service/ingress). When `DeletionTimestamp` is set, the
code removes the object, then falls through to `AddToContext`, because the branch has no
`return`.

### 🟠 B7. Secret target: keys removed from a template are never deleted. *Confirmed*
`konfiguratortemplate_controller.go:367` sets `secret.StringData` on top of the existing
`Data`. A key removed from `spec.templates` stays in `Data` forever. The ConfigMap path
replaces `Data` and is correct. Side effect: `StringData` never round-trips, so every
reconcile is an `Update`.
**Fix:** build `Data` (`[]byte`) as a full replacement.

### 🟠 B8. PodMetadataInjector: finalizer is never removed, so the CR hangs in `Terminating`. *Confirmed*
`controllers/podmetadatainjector_controller.go:72-81`. The deletion branch calls
`RemoveFromContext` and returns. It never calls `DeleteFinalizer` + `Update`.

### 🟠 B9. PodMetadataInjector ignores its own spec, and an empty selector matches every pod. *Confirmed*
`podmetadatainjector_controller.go:105,121,128`. The CRD declares `spec.selector` and
`spec.objectMeta`. The controller selects with the CR's **own metadata labels** and
injects the CR's **own metadata annotations**. The example and the test do the same, so
this is a contract mismatch, not a typo. Consequences:
- An injector without labels gets `MatchingLabels({})`, which matches **all pods in all
  watched namespaces**.
- `SetAnnotations` **replaces** the pod's annotations instead of merging.
- The next Pod event overwrites the injection (see B2), and the 120s requeue puts it back.
  The result is flapping.

**Decision needed:** honor the spec (a breaking change for current users) or remove the spec
fields. Whichever we choose, the List must be namespace-scoped.

### 🟡 B10. Per-request state is stored on the shared reconciler. *Confirmed, latent* — ◐ factory singleton fixed in H.1; reconciler fields remain
`RenderedTemplates` and `KContext` are struct fields (`konfiguratortemplate_controller.go:64-66,86`).
`appKindToObjectMap` hands out **one global object per kind** (`factory.go:9`).
This is only safe because `MaxConcurrentReconciles` is 1. Raising it corrupts state.

### 🟡 B11. Other correctness nits — ◐ invalid `renderTarget` fixed in H.1; the rest remain
- An invalid `renderTarget` (e.g. lowercase `configmap`) renders as a **Secret**
  (`CreateResources`, else-branch). Then `createVolume` and `DeleteResources` both reject
  it, so the finalizer is never removed (stuck `Terminating`). The CRD has no enum here
  either.
- Changing a `volumeMounts[].mountPath` never takes effect: `volumeMountExists` matches by
  volume name only (`pkg/kube/mounts/manager.go:121`).
- A negative `updateFrequency` makes the template requeue in a hot loop. It needs
  `minimum: 1` in the CRD.
- Template rendering only runs on a timer (default 2 min) and never on Pod/Service/Ingress
  events. The README says "when resources change", which isn't what happens.

### ✅ B12. *(found and fixed in H.7)* Volume/container accessors dispatched on `TypeMeta.Kind`
`pkg/kube/lists/{volumes,containers}`. Modern controller-runtime clears TypeMeta after a typed
`Get`. The kind lookup then returned a nil func, and every mount panicked. It was caught by the
existing rendering test during the upgrade, and fixed with a Go type switch plus a regression
test. A sweep found no other TypeMeta-dependent code.

## Security

### 🔴 S1. Cross-namespace information disclosure through templates. *Confirmed by design*
In cluster-wide mode (`WATCH_NAMESPACE=""`, which the README recommends), every template
renders against **all** pods, services, and ingresses in the cluster, and writes the result
into a ConfigMap in its **own** namespace. A tenant who can create a KonfiguratorTemplate
can dump other tenants' full Pod specs. That includes literal `env` values, which often hold
credentials. A one-line template does it: `{{ json .Pods }}`.
**Fix options:** restrict the context to the template's namespace by default, and make
cross-namespace visibility an explicit opt-in controlled by the operator admin (for example
a namespace allow-list in operator config, never in the CR). Redact `env` values and drop
`managedFields`.

### 🔴 S2. The validation webhook is an SSRF and exfiltration channel with no timeout. *Confirmed* — ✅ **closed by H.2** (residual: no DNS-rebinding defense; allow-list is exact-host)
`konfiguratortemplate_controller.go:227`. `http.Post` to `spec.validationWebhookURL`, which
comes from the user:
- **SSRF:** the operator's network identity (in-cluster, often privileged egress) POSTs to
  any URL, including cloud metadata endpoints and internal services.
- **Exfiltration:** the body is the rendered output, which can include S1 data.
- **No timeout:** `http.DefaultClient` has no deadline, so one slow endpoint blocks the
  only template worker. That stalls **all** templates (DoS).
- The response body is decoded without a size limit.

**Fix:** use a client with a timeout and a body limit. Let the operator admin allow-list
webhook hosts (or require in-cluster Service references). Pass `ctx`.

### 🟠 S3. Confused deputy: template authors get write access through the operator's identity. *Confirmed*
Someone who can create a KonfiguratorTemplate but can't edit Deployments or Secrets can
still use the operator to:
- patch any Deployment, DaemonSet, or StatefulSet in their namespace (add volumes/mounts);
- overwrite **any** existing ConfigMap or Secret named `konfigurator-<x>-rendered`
  (`CreateOrUpdate` never checks the `generated-by` annotation or an ownerRef);
- delete that object when the template is deleted.

**Fix:** set an ownerRef and refuse to adopt objects the operator didn't create. Document the
RBAC implication, or use a SubjectAccessReview to check that the template creator could do
this themselves.

### 🟠 S4. The chart grants cluster-wide privileges it doesn't need. *Confirmed*
`charts/konfigurator/templates/clusterrole.yaml` is installed **unconditionally**, even in the
default namespace-scoped mode (`WATCH_NAMESPACE = release namespace`). It grants cluster-wide
`create/update/delete` on **Secrets**, ConfigMaps, and Services, and `update/patch` on all
Deployments, DaemonSets, and StatefulSets. If the operator is compromised, the attacker owns
every Secret in the cluster.
**Fix:** use a Role when namespaced and a ClusterRole only when cluster-wide. Drop `services`
write and `delete` where it isn't used.

### 🟠 S5. The pod runs without hardening. *Confirmed*
`values.yaml`: `podSecurityContext: {}`, `securityContext: {}`, `resources: {}`. The pod is
missing `runAsNonRoot`, `readOnlyRootFilesystem`, `allowPrivilegeEscalation: false`,
`capabilities.drop: [ALL]`, `seccompProfile: RuntimeDefault`, and requests/limits. The image
does run as 65532 (distroless), which is good.

### 🟡 S6. Template functions can probe the operator's filesystem. *Confirmed*
`"dir"` and `"exists"` (`pkg/template/funcs/funcs.go:15-16`) let any template author list
directories and check paths inside the operator container. Remove them, or restrict them to
an allow-listed root.

### 🟡 S7. No limit on template output size. *Plausible*
Rendering has no output cap, so a nested `range` over a large context can drive the operator
out of memory. ConfigMaps are capped at 1 MiB anyway, so output can be capped there.

### 🟠 S8. CI/CD supply chain. *Confirmed*
- Third-party actions pinned to **`@master`** while they hold `PUBLISH_TOKEN`:
  `stefanprodan/helm-gh-pages@master`, `ad-m/github-push-action@master`
  (`push.yml:105,128`), `goreleaser/goreleaser-action@master` (`release.yml:29`). Pin to a
  commit SHA.
- `push.yml:72` passes `PUBLISH_TOKEN` as a Docker **build-arg** (`GIT_ACCESS_TOKEN`). The
  Dockerfile never declares the ARG, so it's unused today, but build-args can end up in build
  metadata. Remove it.

### 🟡 S9. Runtime nits
- `zap.Options{Development: true}` in production (`main.go:62`) gives a console encoder and
  stack traces on warnings.
- The metrics endpoint on `:8080` is unauthenticated. That was the kubebuilder v3 default;
  `kube-rbac-proxy` exists in `config/` but the chart doesn't use it.
- Logs are verbose at Info on every reconcile.

## Modernization / maintainability

| # | Item |
|---|---|
| ✅ M1 | *(closed by H.7)* **Toolchain:** `go 1.17`, `golang:1.17` builder. Move to a supported Go (one of the two newest minors), and keep `go.mod`, the Dockerfile, and CI in lockstep. |
| ✅ M2 | *(closed by H.7: k8s v0.37.2, controller-runtime v0.25.3, controller-tools v0.23.0)* **Kubernetes stack:** k8s libs **v0.22** + controller-runtime **v0.9** (2021). Upgrade them as a set (k8s + controller-runtime + controller-tools), with e2e on kind. This brings `RecoverPanic`, a modern envtest, and current `x/net`. |
| ✅ M3 | *(closed by H.7)* **Build tooling:** the Makefile installs tools with `go get` (broken since Go 1.18). `crd:trivialVersions=true` was removed from controller-gen. The envtest bootstrap is dead. Needed: `go install` with pinned versions, `setup-envtest`, and a `make verify` target (fmt-check · vet · build · lint · test). |
| ✅ M4 | *(closed by H.7: golangci-lint v2.15.0, 0 issues)* **Lint:** add `.golangci.yml` and wire it into `make verify` and CI. |
| ✅ M5 | *(closed by H.7: lint forced it; `openshift/api` dependency dropped)* **Dead code:** `pkg/objects` (it's the only reason `github.com/openshift/api v3.9.0+incompatible` is a dependency), `pkg/utils/base` (`Compress`, `Output`, `MergeMapString`), `k8s_utils.GetSecret*`. All unused. |
| ✅ M6 | *(closed by H.7)* **Committed binaries** in `bin/`: remove them and gitignore the folder. |
| ✅ M7 | *(closed by H.7: `TARGETOS/TARGETARCH`)* **Dockerfile** hardcodes `GOARCH=amd64` while CI sets up QEMU/buildx. Use `TARGETARCH` for real multi-arch. Drop `-a`. |
| M8 | **Tests:** controller tests only cover the happy path through the fake client. Missing: NotFound/delete paths (they'd have caught B1, B6, B8), unknown kind (B3), Secret key removal (B7), concurrency (B2), webhook timeout/deny (S2). No envtest or e2e. |
| M10 | *(found in H.7)* Several legacy tests join their assertions with `&&`, so they fail only when *every* check fails (`pkg/kube/configmap_test.go`, `secret_test.go`, `objects/volume/volume_test.go`; the factory test was rewritten in H.1). |
| M9 | **Docs vs. reality:** the README says "when kubernetes resources change", but rendering is timer-driven (B11). The CRD's `PodMetadataInjector.spec` is dead (B9). |

## Suggested order

1. **Stop the bleeding (security + crash):** B3, S2, S1, S4. Each is small and self-contained,
   except S1, which needs a design decision on cross-namespace opt-in.
2. **Collapse the render context into the informer cache:** B2. This also fixes B1, B5, and
   B6. It's the highest-leverage change in the repo. Retry and status (B4) land with it.
3. **Toolchain + deps + `make verify`:** M1–M4. Before or alongside step 2, so the fixes land
   on a supported Go, with lint and govulncheck in CI.
4. Then the rest: B7, B8, B9 (needs a decision), S3, S5–S8, M5–M9.

## External review of PR #166 (aayeshaarshad, 2026-11-18)

Independent review of the hardening PR; two findings reproduced on real clusters.

| # | Finding | Outcome |
|---|---|---|
| X1 | On k8s < 1.30 (no CRD validation ratcheting) a legacy invalid CR can't be deleted after the CRD upgrade | **Decision (a):** minimum k8s 1.30 / OCP 4.17, enforced by chart `kubeVersion`; actionable 422 message; pre-upgrade check in `CHANGELOG.md` |
| X2 | Delete/create errors never retried (`ManageError(..., false)`) | ✅ fixed: retriable on both paths; `IsValid` stays permanent. Status half of B4 → H.6 |
| X3 | `helm upgrade --reuse-values` nil pointer on the new value | ✅ fixed (`default dict`); `make helm-lint` renders that case |
| X4 | Legacy non-standard `renderTarget` left its rendered Secret | ✅ fixed: deleted on finalize |
| X5 | Allow-list matches names not addresses; bare host = any port; proxy env applies | ✅ documented. Dialer guard (refuse loopback/link-local) → **H.8** (S2 residual) |
| X6 | CI built amd64 only | ✅ fixed: `platforms: linux/amd64,linux/arm64` |
| X7 | Committed `.claude/settings.json` hook affects every agent user | kept by design (method enforcement); opt-out documented in `CLAUDE.md` |
| X8 | `@master` actions with `PUBLISH_TOKEN` | pre-existing → S8 / H.8 |

### Round 3 (re-review of 0b99afe / 6a0c9b9)

| # | Finding | Outcome |
|---|---|---|
| X9 | `make helm-lint` fails on Helm ≤ 3.14 (default kube version 1.29 < new floor); not run in CI | ✅ `--kube-version 1.30.0` on lint and both renders; `helm-lint` + `mutation-check` run in CI |
| X10 | `kubeVersion` guards Helm installs only; kustomize/OLM users get no check | → **H.8**: startup check logging a warning when the server version is < 1.30 |
| X11 | Finalizer-add `Update` still `isRetriable=false` | ✅ all sites go through `fail()`; covered by the classification table |
| X12 | Retrying permanent webhook errors can starve the single worker | ✅ `pkg/reconcileerr` marks permanent errors at the source; `fail()` maps them to `reconcile.TerminalError`. Classification pinned for 15 failure points |
| X13 | Legacy Secret deleted by name only | ✅ deleted only when it carries the `generated-by` annotation. The name clash with a valid template for the same app is S3 → **H.5** (ownerRef) |
| X14 | Tests covered 2 of 7 retry sites | ✅ `TestReconcileFailureClassification` (15 rows) + `make mutation-check` (swallows each site, breaks `fail()` both ways; all mutants must die) |
| X15 | 422 hint: `IsInvalid` is broader than "< 1.30"; "supported value" ambiguous for `renderTarget` | ✅ "If this is Kubernetes < 1.30 …", names the values, says `Secret`; CHANGELOG matches |
