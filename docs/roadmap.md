# Konfigurator — Delivery Roadmap

## How we work

- **Story format:** *As a `<role>`, I want `<capability>`, so that `<value>`.* Each story has a
  **Done:** line.
- **Lifecycle:** 📝 Draft → 👍 Approved → 🔨 In progress → ✅ Done. See
  [method.md](method.md). Nothing goes from Draft to In progress without explicit approval.
- **One story at a time,** end to end, with `make verify` green.
- Every story links its planning doc (`docs/planning/<code>-<slug>.md`) once one exists. Review
  fixes cite the finding id from [`reviews/2026-11-17-baseline-review.md`](reviews/2026-11-17-baseline-review.md).

---

## Epic H — Hardening (from the 2026-11-17 baseline review)

These are all 📝 **Draft**. They were proposed from the review and aren't agreed yet. The
order follows the review's "suggested order".

### H.1 — Unknown `app.kind` must not crash the operator (B3) — ✅ Done 2026-11-17 · [plan](planning/H.1-reject-malformed-templates.md)
As a **platform operator**, I want a malformed KonfiguratorTemplate to be rejected or reported,
so that one tenant's typo can't crash-loop Konfigurator for everyone.
**Done:** a CRD `enum` on `spec.app.kind` and `spec.renderTarget`; the factory returns an
error instead of `nil`; `RecoverPanic` is on; a table test covers the unknown kind and the
invalid render target (including a delete that doesn't hang).

### H.2 — Safe validation webhook (S2) — ✅ Done 2026-11-17 · [plan](planning/H.2-safe-validation-webhook.md)
As a **platform operator**, I want webhook calls to be bounded and limited to destinations I
approve, so that templates can't be used for SSRF, exfiltration, or stalling the operator.
**Done:** an HTTP client with a timeout and a response-size cap; the call uses `ctx`; the
operator admin allow-lists webhook hosts in operator config; tests cover timeout, deny,
oversized body, and a host that isn't allowed.

### H.3 — Namespace-scoped render context by default (S1) — 📝 Draft · *needs a decision*
As a **tenant on a shared cluster**, I want other tenants' templates to be unable to read my
pod specs, so that cluster-wide mode doesn't leak my configuration or secrets.
**Done:** a template sees only its own namespace unless the operator admin opts in; Pod `env`
values and `managedFields` are redacted from the context; an ADR records the decision.

### H.4 — Least-privilege chart RBAC + pod hardening (S4, S5) — 📝 Draft
As a **cluster admin**, I want Konfigurator to hold only the permissions its mode needs and
to run hardened, so that a compromise of the operator doesn't hand over every Secret in the
cluster.
**Done:** a Role when namespaced and a ClusterRole only when cluster-wide; unused verbs and
resources dropped; a restricted securityContext and resource requests/limits by default;
`helm lint` and a template test for both modes.

### H.5 — Render from the informer cache; delete the hand-rolled context (B2, B1, B5, B6) — 📝 Draft
As a **template author**, I want rendered config to reflect exactly the live cluster objects,
so that deleted pods disappear, restarts never write empty config, and there are no data
races.
**Done:** `pkg/context` mirror and the Pod/Service/Ingress "cache" reconcilers removed;
rendering `List`s from the manager cache after it syncs; tests for deleted objects, a cold
start, and `go test -race` with concurrent reconciles. Also owns: request-scoped state off
the reconciler struct (B10/#166 F4) and the rendered-object name clash between templates for
the same app (S3, #166 X13: ownerRef + refuse to adopt).

### H.6 — Retry + status conditions on KonfiguratorTemplate (B4) — 📝 Draft
As a **template author**, I want to see why my template isn't applied and have it recover by
itself, so that a template created before its Deployment doesn't stay stuck silently.
**Done:** transient errors requeue with backoff; `.status.conditions` carries `Ready`/`Error`
plus `observedGeneration`; tests cover the "app not found yet" case.

### H.7 — Toolchain, dependencies, `make verify` in CI (M1–M4) — ✅ Done 2026-11-17 · [plan](planning/H.7-toolchain-deps-verify.md)
As a **maintainer**, I want the operator built on a supported Go and Kubernetes stack, with
lint and vuln scans in CI, so that we stop shipping known CVEs and broken tooling.
**Done:** a supported Go version in `go.mod`, the Dockerfile, and CI; k8s, controller-runtime,
and controller-tools upgraded as a set; `make verify` (fmt · vet · build · lint · test) plus
`govulncheck` in CI; a kind e2e smoke test.

### H.8 — Remaining correctness + hygiene (B7, B8, B9, B10, B11, S3, S6–S9, M8–M10) — 📝 Draft
To be split into individual stories once H.1–H.7 are agreed. **B9 needs a product decision
first:** either honor `PodMetadataInjector.spec` or remove those fields. Also from the #166
external review: a webhook dialer guard refusing loopback/link-local addresses (X5), pinning
actions to SHAs (X8/S8), and a startup check that warns when the API server is below the
1.30 floor, for kustomize/OLM installs the chart `kubeVersion` cannot guard (X10).

### H.9 — Adopt Stakater shared CI workflows — 🔨 In progress (👍 2026-11-25, option (b): no OLM, gh-pages chart) · [plan](planning/H.9-shared-ci.md)
As a **Stakater maintainer**, I want Konfigurator to build, test, and release through the shared
`stakater/.github` operator workflows like kube-ward, so that CI upkeep and action pinning (S8)
are done once for the org, not per repo.
**Decision first:** release model.
- (a) Move to OLM bundle + OCI Helm chart, like kube-ward. This changes how users install it.
- (b) Keep image + gh-pages chart, using the shared PR/push verification with the bundle and
  chart-push steps turned off.

**Done:** `pull_request`/`push`/`release` (+ e2e) are thin callers of
`stakater/.github/.github/workflows/operator_*.yaml@main`; the Makefile provides the targets those
workflows call (`lint`, `test`, `build`, `docker-build/push`, and per the decision `bundle*`,
`catalog-*`, `helm-test`, `helm-release`, `test-e2e`); `make e2e` keeps running; a release from
a tag is proven end to end. Context: today's workflows are inline (2026-11-17 check), and the
shared ones are driven by Make targets.
