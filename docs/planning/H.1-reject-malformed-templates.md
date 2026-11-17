# H.1 — Unknown `app.kind` must not crash the operator

Story: `docs/roadmap.md` H.1 (👍 Approved 2026-11-17). Driver: baseline review
**B3** (crash-loop on unknown kind), plus the closely related parts of **B11**
(invalid `renderTarget`) and **B10** (factory singletons). B10 is included because
the B3 fix rewrites the same function.

## Why (one paragraph)

`pkg/kube/factory.go:31` looks up `app.kind` in a map of global objects. An unknown
kind returns a `nil` interface, and `SetName` on it panics. controller-runtime v0.9
doesn't recover reconciler panics, so the process exits. After the restart it
reconciles the same CR again, so it crash-loops for **every tenant**. The CRD has no
`enum`, so the API server accepts `kind: Job` or a typo. An invalid `renderTarget`
doesn't crash, but it silently renders a Secret. When the CR is deleted, the cleanup
then errors and the finalizer is never removed, so the CR hangs in `Terminating`.

## Governing constraints

- CLAUDE.md invariant 3, **no panics on user input**: the CRD validates, and the
  controller returns errors.
- CLAUDE.md invariant 6, **observable**: this story only makes the rejection visible
  in the operator log, which is the current `ManageError` behavior. Status conditions
  are **H.6**.
- Don't widen into H.5 (render context) or H.7 (toolchain).

## Design

1. **CRD validation.** Add markers on `api/v1alpha1/konfiguratortemplate_types.go`:
   - `AppKind`: `+kubebuilder:validation:Enum=Deployment;DaemonSet;StatefulSet`
   - `RenderTarget`: `+kubebuilder:validation:Enum=ConfigMap;Secret`

   controller-gen v0.5.0 panics under Go 1.27 (verified), so the matching `enum:` is
   **hand-applied** to `config/crd/bases/konfigurator.stakater.com_konfiguratortemplates.yaml`
   and `charts/konfigurator/templates/crd.yaml`. A test pins the YAML to the Go
   constants. They get regenerated properly in H.7.
2. **`KonfiguratorTemplate.IsValid()`** currently always returns `true`. It now
   rejects an unknown `app.kind` and an unknown `renderTarget`, with a descriptive
   error.
3. **Factory.** `kube.CreateObjectFromApp(app, ns) (client.Object, error)` returns a
   **fresh** typed object for each call (closes the B10 factory singleton) and an
   error for an unknown kind.
4. **Reconcile order.** Deletion is handled **before** validation. In
   `handleDelete`, an invalid spec skips unmount and resource deletion and just
   removes the finalizer. After this change nothing is ever created for an invalid
   spec, so there's nothing to clean up. `CreateResources` errors on an unknown
   render target instead of falling through to Secret.
5. **Panic safety net.** `controllers.recoverPanics(r)` is a `reconcile.Reconciler`
   decorator that turns a panic into a returned error and logs it. Every controller
   gets wrapped in `SetupWithManager`. It gets replaced by controller-runtime's
   built-in `RecoverPanic` option when H.7 upgrades the library.

Not changing: rendering, mounting logic, ManageError semantics (H.6), webhook (H.2).

## Tests (red first)

- `api/v1alpha1/konfiguratortemplate_types_test.go`: `IsValid` table. Covers every
  valid kind × target, an unknown kind, an empty kind, lowercase `configmap`, and an
  empty target.
- `pkg/kube/factory_test.go`: each kind returns the right type with name and
  namespace set. Two calls return distinct objects. An unknown kind returns an error
  and doesn't panic.
- `controllers/konfiguratortemplate_controller_test.go`:
  - Reconciling `kind: Job` doesn't panic, and no ConfigMap or Secret is created.
  - Reconciling an invalid CR that has a deletionTimestamp and the finalizer gets the
    finalizer removed.
- `controllers/recover_test.go`: a panicking reconciler returns an error and doesn't
  panic.
- `api/v1alpha1/crd_enum_test.go`: the enums in both CRD YAMLs equal the Go
  constants.

## Out of scope

- `updateFrequency` minimum (B11 remainder) → H.8.
- Status conditions and retry (B4) → H.6.
- Regenerating CRDs with a working controller-gen → H.7.
- Legacy CRs created **before** this fix with an invalid `renderTarget` already have a
  Secret `konfigurator-<app>-rendered` and a mount. Deleting such a CR now removes the
  finalizer without cleaning those up. Accepted limitation: the old code couldn't
  clean them up either (it hung in `Terminating`). Delete them manually.

## Done means

The roadmap Done line is met, `make verify` is green, both CRD copies carry the enums,
B3 is closed in the review ledger, and B10/B11 are marked partially closed.
