# H.2 — Safe validation webhook

Story: `docs/roadmap.md` H.2 (👍 Approved 2026-11-17). Driver: baseline review **S2**.

## Why (one paragraph)

`controllers/konfiguratortemplate_controller.go` (`validateEngine`) calls `http.Post`
with `http.DefaultClient` on `spec.validationWebhookURL`, and that URL is supplied by
the template author. That causes three problems:

- **SSRF:** the operator's network identity reaches any address, including cloud
  metadata endpoints and internal services.
- **Exfiltration:** the request body is the rendered output, which can contain
  cluster-wide pod data (S1).
- **DoS:** there's no timeout. One slow endpoint blocks the only template worker,
  which stalls every template. The response is also decoded without a size limit.

## Governing constraints

- CLAUDE.md invariant 4: **outbound calls are bounded** (timeout, size cap, `ctx`,
  admin allow-list).
- CLAUDE.md invariant 1: cross-boundary reach is decided by the **operator admin in
  operator config, never in the CR**. The allow-list therefore lives in an env var
  set from Helm values, not in a CR field.

## Design

New package **`pkg/webhook`**, which has no controller-runtime imports:

```go
type Validator struct {
    AllowedHosts     []string      // exact host or host:port, case-insensitive; empty = webhooks disabled
    Timeout          time.Duration // default 10s
    MaxResponseBytes int64         // default 64 KiB
    Client           *http.Client  // optional; built from Timeout, redirects refused
}
func (v *Validator) Validate(ctx context.Context, rawURL string, rendered map[string]string) error
func ParseAllowedHosts(csv string) []string
```

`Validate` checks these rules in order, and **no request is sent** unless the first
three pass:

1. Parse the URL. The scheme must be `http` or `https`.
2. The allow-list must be non-empty.
3. The host must be on the allow-list.
4. POST the JSON with `http.NewRequestWithContext`.
5. Redirects are refused, so an allowed host can't bounce the request to a
   non-allowed one.
6. A non-2xx status is an error.
7. The body goes through `io.LimitReader(MaxResponseBytes+1)`. More than the cap is
   an error.
8. A response with `allowed:false` is an error that includes the message.

Wiring:

- `KonfiguratorTemplateReconciler` gets a `Webhook *webhook.Validator` field.
  `validateEngine` delegates to it with the reconcile `ctx`. A nil field behaves
  like an empty allow-list (deny).
- `main.go` builds the validator from `VALIDATION_WEBHOOK_ALLOWED_HOSTS`
  (comma-separated).
- Chart: `validationWebhook.allowedHosts: []` in values, rendered into that env var
  in `operator.yaml`. Default empty means disabled.

Docs: the `validationWebhookURL` row in `docs/konfigurator-template.md` explains the
allow-list requirement, with an upgrade note.

Not changing: the rendering itself, the wire format sent to the webhook (still the
JSON map of rendered files), and ManageError semantics (H.6).

**Behavior change (intended):** existing users of `validationWebhookURL` must
allow-list their webhook host when they upgrade. Otherwise their templates stop
rendering, and the operator log explains why.

## Tests (red first)

`pkg/webhook/validator_test.go`. It's table-driven against `httptest.Server`, and
every case asserts both the error and whether the server was hit:

| Case | Expectation |
|---|---|
| allowed + `allowed:true` | nil |
| allowed + `allowed:false` | error containing the message |
| host not on the allow-list | error, server **not** hit |
| empty allow-list | error, server **not** hit |
| scheme `file`/`ftp`, or an unparsable URL | error, nothing sent |
| server slower than the timeout | error within the timeout |
| body larger than the cap | error |
| 302 to a second server | error, second server **not** hit |
| 500 | error |
| cancelled `ctx` | error |

Also:

- `ParseAllowedHosts` table: blanks, spaces, and case.
- Controller: a template whose webhook host isn't allowed gets **no** Secret or
  ConfigMap rendered.

## Out of scope

- Restricting allowed hosts to resolve only to in-cluster IPs, and DNS-rebinding
  defense. The allow-list is an exact-host decision made by the admin, which is
  acceptable for now.
- A configurable timeout or cap. Fixed defaults until someone needs to change them
  (YAGNI).
- The kustomize `config/manager` env (the chart is the shipped install path).
- Status conditions (H.6).

## Done means

The roadmap Done line is met, `make verify` is green, `helm lint` passes and
`helm template` shows the env var when it's set, the user docs are updated, and S2 is
closed in the review ledger.
