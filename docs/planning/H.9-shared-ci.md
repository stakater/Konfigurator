# H.9 — Adopt Stakater shared CI workflows (no OLM)

Story: `docs/roadmap.md` H.9 (👍 Approved 2026-11-25, option **(b)**: keep image +
gh-pages chart, no OLM). Driver: the first `push.yml` release run after #166 failed
(run 45600576226) in a step no other Stakater repo has: committing version bumps back
to `master` from inside the release job. Review S8 (floating actions) and the
principle "wrap what is proven" point the same way.

## Why (one paragraph)

Konfigurator's release pipeline is hand-rolled: it derives a tag, builds the image,
`sed`s the chart version, publishes the chart, then **commits the bump to `master`**
and pushes a tag. The commit-back step broke (root-owned `.git` from Docker-based
actions) and is unique to this repo. Every other Stakater repo derives versions from
git tags and commits nothing. Copying their workflows removes the broken part instead
of patching it, and moves action upkeep to one place.

## Facts checked (2026-11-25)

| Item | Finding |
|---|---|
| `stakater/.github` `operator_*` | Always runs `make bundle bundle-build bundle-push` and `catalog-render`; chart via OCI `helm.mk`. **OLM required**; no off switch. Rejected by decision (b) |
| `stakater/.github` `push_container.yaml` | `workflow_call`; inputs `DOCKER_FILE_PATH`, `RELEASE_BRANCH`; secrets `CONTAINER_REGISTRY_{URL,USERNAME,PASSWORD}`, `SLACK_WEBHOOK_URL`, `GH_TOKEN`. Tag from git tags (`anothrNick`, `WITH_V`), multi-stage build, pushes `<registry>/<repo>:<tag>`, then pushes the tag. **No `platforms:`** → amd64 only |
| `stakater/.github` `release.yaml` | Not callable (`on: push tags`); 30 lines creating a GitHub Release. Copied verbatim |
| Forecastle `release-helm.yaml` | Publishes to the gh-pages chart repo with `helm-gh-pages` when `Chart.yaml` changes on the default branch |
| Reloader | Bespoke pinned workflows; chart release gated on a PR label. Not shared |
| Our `pull_request.yml` | Make-driven (`verify codegen-check vulncheck helm-lint mutation-check`, e2e, PR image build). Works; untouched |

## Governing constraints

- CLAUDE.md: `make verify` stays the DoD; the PR workflow is not changed.
- S8: no floating `@master` actions in our files. Pinned where we own the line.
- Honesty: the decision (b) trade-offs are stated in `CHANGELOG.md` and
  `docs/releasing.md`: amd64-only image until the shared workflow gains a
  `PLATFORMS` input; chart released by a `Chart.yaml` PR, not on every merge.

## Design

Three workflow files replace two:

1. **`push.yml`** → a 20-line caller of
   `stakater/.github/.github/workflows/push_container.yaml@main`. Secrets mapped
   from the repo's existing ones: `GHCR_TOKEN` (registry password),
   `STAKATER_DELIVERY_SLACK_WEBHOOK`, `PUBLISH_TOKEN` (`GH_TOKEN`, pushes the tag).
   Registry URL/username are literals passed as secrets, as kube-ward does.
2. **`release-helm.yml`** (new) → Forecastle's job, adapted: trigger on
   `charts/konfigurator/Chart.yaml` changes on `master`; `helm lint`; publish with
   `stefanprodan/helm-gh-pages` **pinned** to `89c6698…` (the SHA that ran last
   week); token `PUBLISH_TOKEN` (worked last week).
3. **`release.yml`** → the shared repo's own `release.yaml`, copied: GitHub Release on
   `v*` tags. Replaces the broken `goreleaser-action@master` (no config existed).

Repo changes:
- Chart bumped to `0.1.40` / image `v0.1.40` so git matches what the failed run
  already published. Merging republishes chart `0.1.40` (same content) and the
  image job creates tag `v0.1.40`; afterwards tag, image, chart and `master` agree.
- `make bump-chart` stays as the helper for the manual chart bump.
- `docs/releasing.md` (new): how a release happens now, step by step.

Not changing: the PR workflow, the Makefile gates, the chart's install path.

## Tests

CI workflows cannot be unit-tested here. What is checked:
- `actionlint` + YAML parse on the three files (syntax, expression and input checks).
- `make helm-lint` on the bumped chart.
- **The merge is the test:** the first `push.yml` run must push image + tag `v0.1.40`;
  the `release-helm.yml` run must republish chart `0.1.40`; `release.yml` must create
  the GitHub Release. All three are watched and reported.

## Out of scope

- OLM bundle/catalog and OCI chart (`operator_*` family): a future story if the team
  decides on OLM.
- `PLATFORMS` input for the shared workflow: follow-up PR to `stakater/.github`
  (H.9.1). Until then the image is amd64-only, as every prior release was.
- Replacing the PR workflow's image build with `pull_request_container_build.yaml`
  (it pushes PR images to the registry; not asked for).

## Done means

The three workflows are thin callers/copies with no commit-back step and no
floating actions in our files; `actionlint` clean; `make helm-lint` green; the
release procedure is documented; after merge, the first runs of all three workflows
are green and `v0.1.40` exists as tag, image and chart.
