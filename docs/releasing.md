# Releasing Konfigurator

Since H.9 (2026-11-25) the pipeline is the shared Stakater one. Versions come from git
tags; CI never commits to `master`.

## What happens on its own

| Trigger | Workflow | Result |
|---|---|---|
| PR to `master` | `pull_request.yml` (ours) | `make verify codegen-check vulncheck helm-lint mutation-check`, kind e2e, image build (not pushed) |
| Merge to `master` | `push.yml` → shared `push_container.yaml` | next patch tag computed from existing tags (`v0.1.40` → `v0.1.41`), image `ghcr.io/stakater/konfigurator:<tag>` pushed, tag pushed |
| Tag `v*` pushed | `release.yml` | GitHub Release for the tag |
| `charts/konfigurator/Chart.yaml` changes on `master` | `release-helm.yml` | chart published to `https://stakater.github.io/stakater-charts` |

## Releasing a new operator version to chart users

1. Merge the code. `push.yml` publishes image `vX.Y.Z` and the tag.
2. Open a PR that bumps the chart to that image: `make bump-chart VERSION=X.Y.Z`
   (sets `Chart.yaml` `version`/`appVersion` and `values.yaml` `image.tag`), plus the
   `CHANGELOG.md` section for the release. Review it like any PR.
3. Merge. `release-helm.yml` publishes chart `X.Y.Z`.

A chart bump can skip operator versions, and a chart-only fix bumps the chart alone.
`[skip-ci]` in a commit message skips `push.yml`.

## Known limits

- **Image is `linux/amd64` only.** The shared `push_container.yaml` sets no `platforms:`.
  The Dockerfile cross-compiles via `TARGETARCH`, so a `PLATFORMS` input upstream in
  `stakater/.github` is all that is missing (follow-up H.9.1).
- `push.yml` calls the shared workflow at `@main`, as every Stakater repo does: upkeep of
  the actions inside it is upstream's. Actions in our own files are pinned (S8).
- Secrets used: `GHCR_TOKEN` (registry push), `PUBLISH_TOKEN` (git tag, chart repo push),
  `STAKATER_DELIVERY_SLACK_WEBHOOK`. Rotating one of them means checking all three
  workflows.

## Why not OLM

`stakater/.github`'s `operator_*` workflows always build an OLM bundle and catalog and
publish the chart to an OCI registry. Konfigurator has no bundle and its users install
from the gh-pages chart repo, so that family was rejected (H.9 decision (b),
2026-11-25). If the team adopts OLM later, switching is a separate story.
