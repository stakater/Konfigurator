# VERSION defines the project version for the bundle and image.
# Override on the command line, e.g. `make bundle VERSION=0.0.2`.
VERSION ?= 0.0.1

# CHANNELS / DEFAULT_CHANNEL configure the OLM bundle (see `make bundle`).
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)
BUNDLE_IMG ?= controller-bundle:$(VERSION)

# Image URL to use all building/pushing image targets
IMG ?= stakater/konfigurator:v$(VERSION)

## Pinned tool versions (H.7). Tools install into ./bin (gitignored) with `go install`.
LOCALBIN ?= $(shell pwd)/bin
CONTROLLER_TOOLS_VERSION ?= v0.23.0
GOLANGCI_LINT_VERSION    ?= v2.15.0
GOVULNCHECK_VERSION      ?= v1.10.0
KIND_VERSION             ?= v0.32.0
KUSTOMIZE_VERSION        ?= v5.8.3

CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
GOLANGCI_LINT  ?= $(LOCALBIN)/golangci-lint
GOVULNCHECK    ?= $(LOCALBIN)/govulncheck
KIND           ?= $(LOCALBIN)/kind
KUSTOMIZE      ?= $(LOCALBIN)/kustomize

CHART_CRDS := charts/konfigurator/templates/crd.yaml

.PHONY: all
all: manager

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate CRDs and RBAC from +kubebuilder markers, and sync the chart's CRD copy.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	$(MAKE) chart-crds

.PHONY: chart-crds
chart-crds: ## Copy the generated CRDs into the Helm chart (installed only when deployCRD=true).
	@{ echo '{{- if .Values.deployCRD }}'; cat config/crd/bases/*.yaml; echo '{{- end }}'; } > $(CHART_CRDS)

.PHONY: generate
generate: controller-gen ## Generate deepcopy code.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: codegen-check
codegen-check: generate manifests ## Fail if generated code/manifests are out of date.
	@git diff --exit-code -- api config $(CHART_CRDS) || { echo "generated files are stale: run 'make generate manifests' and commit"; exit 1; }

.PHONY: fmt
fmt: ## Format the code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet.
	go vet ./...

.PHONY: lint
lint: golangci-lint ## Run golangci-lint (config: .golangci.yml).
	$(GOLANGCI_LINT) run --timeout=10m ./...

.PHONY: test
test: ## Run unit tests with the race detector.
	go test -race -count=1 ./...

.PHONY: unit-test
unit-test: test

# Definition of Done gate (docs/method.md). Run by the Claude Code PostToolUse hook
# (hack/verify-hook.sh) after every Go edit, and in CI.
.PHONY: verify
verify: vet ## vet, build, lint (includes gofmt/goimports), test.
	go build ./...
	$(MAKE) lint
	$(MAKE) test

.PHONY: vulncheck
vulncheck: govulncheck ## Report reachable known vulnerabilities.
	$(GOVULNCHECK) ./...

.PHONY: e2e
e2e: kind ## Smoke-test the real image + chart on a throwaway kind cluster.
	KIND=$(KIND) test/e2e/smoke.sh

##@ Build

.PHONY: manager
manager: generate fmt vet lint helm-lint ## Build the manager binary.
	go build -o bin/manager main.go

.PHONY: run
run: generate fmt vet manifests ## Run against the cluster in ~/.kube/config.
	go run ./main.go

.PHONY: docker-build
docker-build: test ## Build the docker image.
	docker build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push the docker image.
	docker push ${IMG}

.PHONY: helm-lint
helm-lint: ## Lint the chart and render it as an upgrade with --reuse-values from a pre-H.2 release would.
	helm lint --kube-version 1.30.0 charts/konfigurator
	helm template charts/konfigurator --kube-version 1.30.0 --set validationWebhook=null >/dev/null
	helm template charts/konfigurator --kube-version 1.30.0 --set 'validationWebhook.allowedHosts={a.svc,b.svc:8443}' | grep -q 'a.svc,b.svc:8443'

.PHONY: mutation-check
mutation-check: ## Prove every reconciler error site is covered: swallow each one and require a failing test.
	hack/mutation-check.sh

.PHONY: bump-chart
bump-chart: ## Set chart version/appVersion/image tag to $(VERSION) (used by CI).
	sed -i "s/^version:.*/version:  $(VERSION)/" charts/konfigurator/Chart.yaml
	sed -i "s/^appVersion:.*/appVersion:  $(VERSION)/" charts/konfigurator/Chart.yaml
	sed -i "s/tag:.*/tag:  v$(VERSION)/" charts/konfigurator/values.yaml

##@ Deployment (kustomize)

.PHONY: install
install: manifests kustomize ## Install CRDs into the cluster in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs.
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy the controller via kustomize.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

.PHONY: undeploy
undeploy: kustomize ## Undeploy the controller.
	$(KUSTOMIZE) build config/default | kubectl delete -f -

# OLM bundle targets: unchanged legacy scaffolding, not exercised in CI (H.7 plan, out of scope).
.PHONY: bundle
bundle: manifests kustomize
	operator-sdk generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | operator-sdk generate bundle -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)
	operator-sdk bundle validate ./bundle

.PHONY: bundle-build
bundle-build:
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

##@ Tools (pinned; reinstalled when the version changes)

$(LOCALBIN):
	mkdir -p $(LOCALBIN)

# go-install-tool installs $(2)@$(3) as $(1), versioned so a version bump reinstalls.
define go-install-tool
@[ -f "$(1)-$(3)" ] || { \
	set -e; \
	echo "Installing $(2)@$(3)"; \
	GOBIN=$(LOCALBIN) go install $(2)@$(3); \
	mv "$(LOCALBIN)/$$(basename $(1))" "$(1)-$(3)"; \
}; \
ln -sf "$$(basename $(1))-$(3)" "$(1)"
endef

.PHONY: controller-gen golangci-lint govulncheck kind kustomize
controller-gen: $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))
golangci-lint: $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))
govulncheck: $(LOCALBIN)
	$(call go-install-tool,$(GOVULNCHECK),golang.org/x/vuln/cmd/govulncheck,$(GOVULNCHECK_VERSION))
kind: $(LOCALBIN)
	$(call go-install-tool,$(KIND),sigs.k8s.io/kind,$(KIND_VERSION))
kustomize: $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: help
help: ## Show this help.
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ { printf "  %-16s %s\n", $$1, $$2 } /^##@/ { printf "\n%s\n", substr($$0, 5) }' $(MAKEFILE_LIST)
