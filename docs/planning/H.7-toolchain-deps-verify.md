# H.7 — Toolchain, dependencies, `make verify` in CI

Story: `docs/roadmap.md` H.7 (👍 Approved 2026-11-17). Driver: baseline review
**M1–M4**, plus **M6** (committed binaries) and **M7** (amd64-only image), because
those two block this story (see Design). It also removes the H.1 stopgaps
(hand-applied CRD enums, the `recoverPanics` decorator).

## Why (one paragraph)

The operator builds on **Go 1.17**, which has been end-of-life since 2022, against
**k8s v0.22 / controller-runtime v0.9** from 2021. govulncheck reports 15 reachable
vulnerabilities locally, and the shipped image (built with `golang:1.17`) carries more.
The tooling is broken:

- `controller-gen` v0.5.0 panics on a current Go, so CRDs can't be regenerated (H.1
  had to hand-edit them).
- `make test` downloads an envtest bootstrap that no longer exists.
- Tool installs use `go get`, which stopped installing binaries in Go 1.18.
- CI pipes `install.sh` from golangci-lint's `master` branch and pins lint v1.44.2.

Every hardening story after this one needs a working, current toolchain.

## Facts checked (2026-11-17)

| Item | Version | Source |
|---|---|---|
| Supported Go | **1.27.4**, 1.26.11 | go.dev/dl JSON |
| `golang` image | `golang:1.27.4` exists | Docker Hub API |
| controller-runtime | **v0.25.3** (requires k8s **v0.37.0**, go 1.26) | its go.mod |
| k8s.io/api, apimachinery, client-go | **v0.37.2** | `go list -m -versions` |
| controller-tools | **v0.23.0** | `go list -m -versions` |
| golangci-lint | **v2.15.0** | `go list -m -versions` |
| govulncheck (x/vuln) | **v1.10.0** | `go list -m -versions` |
| operator-utils | latest v0.1.19 still targets controller-runtime **v0.8.1** | its go.mod |
| kind (local) | v0.32.0 | `kind version` |

## Governing constraints

- Rasheed's global default: **always build on a supported Go** (one of the two newest
  minors). `go.mod`, the Dockerfile, and CI move **together**.
- k8s libs, controller-runtime, and controller-tools move **as a set** (controller-runtime
  pins a k8s minor). Run e2e on kind.
- **The tests must build what the binary builds.** Manager options get extracted into a
  function that `main` and a test both call.
- Behavior stays the same, except for the deliberate removal of the H.1 stopgaps.

## Design

1. **Go:** `go 1.27.4` in `go.mod`, `golang:1.27.4` in the Dockerfile,
   `GOLANG_VERSION: 1.27.4` in all three workflows.
2. **Dependencies:** k8s v0.37.2, controller-runtime v0.25.3, logr v1, then
   `go mod tidy`. **operator-utils:** keep it if it compiles and passes against
   controller-runtime v0.25. If not, replace the helpers we use (finalizer
   Add/Has/Delete → `controllerutil`; `ManageError`/`RequeueWithError`/`RequeueAfter` →
   a small local `controllers/result.go` with identical semantics, since H.6 changes
   the semantics, not this story).
3. **Manager API migration** (`main.go`): `Metrics: metricsserver.Options{BindAddress}`,
   `Cache.DefaultNamespaces` for `WATCH_NAMESPACE`, and
   `Controller.RecoverPanic: true`. These are built in a new `managerOptions(...)`,
   which a test asserts on. **Delete** `controllers/recover.go` and its test: the
   built-in option replaces them (planned in H.1).
4. **Makefile:**
   - Tools are pinned and installed with `go install` into `./bin`: controller-gen,
     golangci-lint, govulncheck, kind, kustomize.
   - `manifests` drops `trivialVersions`/`preserveUnknownFields`, which no longer exist.
   - `chart-crds` copies the generated CRDs into
     `charts/konfigurator/templates/crd.yaml` (wrapped in `deployCRD`).
   - `codegen-check` regenerates everything and fails on any git diff.
   - `verify` = fmt-check · vet · build · **lint (no longer optional)** · test.
   - `vulncheck` and `e2e` targets.
   - `test` becomes plain `go test` (the dead envtest bootstrap is removed; there are
     no envtest tests).
5. **M6:** untrack the committed `bin/manager` (44 MB) and `bin/controller-gen` (21 MB,
   Linux) and gitignore `/bin`, because the tools now install there.
6. **M7:** the Dockerfile builds for `TARGETOS/TARGETARCH`, so the image runs on
   arm64 kind nodes (needed for local e2e).
7. **Lint:** a `.golangci.yml` (v2) with the default set (errcheck, govet, ineffassign,
   staticcheck, unused) plus gofmt/goimports. Fix every finding. No blanket excludes.
8. **CRDs:** regenerate with controller-tools v0.23.0. The H.1 enum test stays as a
   guard, and the enums now come from the markers.
9. **CI:**
   - PR workflow: `make verify codegen-check vulncheck`, plus an `e2e` job
     (`make e2e`).
   - Push workflow: the lint step becomes `make verify`.
   - The curl-pipe-from-master install is removed.
10. **e2e smoke** (`test/e2e/smoke.sh`, `make e2e`):
    - Set up a kind cluster, build the image, load it into kind, and install the chart
      with CRDs.
    - Apply a Deployment and a KonfiguratorTemplate (ConfigMap target) that renders
      pod names.
    - Assert the ConfigMap appears with a pod's name in it, and the Deployment has the
      volume and mount.
    - Assert a `kind: Job` template is **rejected by the API server** (the H.1 enum).
    - Delete the template, then assert the ConfigMap is gone and the Deployment's
      volume is removed.

## Tests (red first)

- `main_test.go`:
  - `managerOptions` sets `RecoverPanic=true`, the metrics/probe addresses, leader
    election, and `DefaultNamespaces` only when `WATCH_NAMESPACE` is set (table).
  - Red until `managerOptions` exists.
- The existing unit suite must stay green across the upgrade. That's the behavior
  guard for the migration.
- The H.1 enum test must stay green on the **regenerated** CRDs, which proves the
  markers produce the enums.
- `make codegen-check` must be clean after regeneration.
- e2e smoke, as above. It's the only test that boots the real binary in a real cluster.

## Out of scope

- Pinning third-party actions to SHAs (S8) → H.8.
- The broken `release.yml` (goreleaser with no config) and the
  bundle/operator-sdk targets → unchanged; noted.
- Removing dead code (M5), unless lint or the upgrade forces it (if so, it's recorded
  here).
- Renovate grouping policy.
- Any behavior change to reconcilers (H.5/H.6).

## Done means

The roadmap Done line is met:

- `make verify` (with lint) is green.
- `make codegen-check` is clean.
- `make vulncheck` reports **0** reachable vulnerabilities, or each remaining one is
  listed with the reason it can't be fixed.
- `make e2e` passes locally on kind.
- CI workflows are updated.
- M1–M4, M6, M7 are closed in the review ledger.
