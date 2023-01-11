
# Image URL to use all building/pushing image targets
IMG ?= docker.io/cluster-api-provider-kind/kind-controller:latest
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.25.0

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

GOARCH  := $(shell go env GOARCH)
GOOS    := $(shell go env GOOS)

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

management-cluster:
	sudo kind create cluster --name capi-test --config hack/kind-with-docker.yaml

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test ./... -coverprofile cover.out

##@ Build

.PHONY: build
build: manifests generate fmt vet ## Build manager binary.
	go build -o bin/manager main.go

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./main.go

# If you wish built the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64 ). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build
docker-build: test ## Build docker image with the manager.
	docker build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	docker push ${IMG}

# PLATFORMS defines the target platforms for  the manager image be build to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - able to use docker buildx . More info: https://docs.docker.com/build/buildx/
# - have enable BuildKit, More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image for your registry (i.e. if you do not inform a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To properly provided solutions that supports more than one platform you should use this option.
PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
.PHONY: docker-buildx
docker-buildx: test ## Build and push docker image for the manager for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- docker buildx create --name project-v3-builder
	docker buildx use project-v3-builder
	- docker buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
	- docker buildx rm project-v3-builder
	rm Dockerfile.cross

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

##@ Build Dependencies

# .PHONY: controller-gen
# controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary. If wrong version is installed, it will be overwritten.
# $(CONTROLLER_GEN): $(LOCALBIN)
# 	test -s $(LOCALBIN)/controller-gen && $(LOCALBIN)/controller-gen --version | grep -q $(CONTROLLER_TOOLS_VERSION) || \
# 	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

# .PHONY: envtest
# envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
# $(ENVTEST): $(LOCALBIN)
# 	test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

TOOLS_DIR := hack/tools
TOOLS_BIN_DIR := $(abspath $(TOOLS_DIR)/bin)

.PHONY: envtest
ENVTEST_VER := v0.0.0-20211110210527-619e6b92dab9
ENVTEST_BIN := envtest
ENVSUBST := $(TOOLS_BIN_DIR)/$(ENVTEST_BIN)
envtest: $(CONTROLLER_GEN) 
$(ENVTEST): ## Build controller-gen from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) go install -tags tools sigs.k8s.io/controller-runtime/tools/setup-envtest@$(CONTROLLER_GEN_VER)

.PHONY: controller-gen
CONTROLLER_GEN_VER := v0.9.2
CONTROLLER_GEN_BIN := controller-gen
CONTROLLER_GEN := $(TOOLS_BIN_DIR)/$(CONTROLLER_GEN_BIN)
controller-gen: $(CONTROLLER_GEN) 
$(CONTROLLER_GEN): ## Build controller-gen from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) go install -tags tools sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VER)

ENVSUBST_VER := v2.0.0-20210730161058-179042472c46
ENVSUBST_BIN := envsubst
ENVSUBST := $(TOOLS_BIN_DIR)/$(ENVSUBST_BIN)
$(ENVSUBST): ## Build envsubst from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) go install -tags tools github.com/drone/envsubst/v2/cmd/envsubst@$(ENVSUBST_VER)

KUSTOMIZE_VER := v4.5.2
KUSTOMIZE_BIN := kustomize
KUSTOMIZE := $(TOOLS_BIN_DIR)/$(KUSTOMIZE_BIN)
$(KUSTOMIZE): ## Build kustomize from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) go install -tags tools sigs.k8s.io/kustomize/kustomize/v4@$(KUSTOMIZE_VER)

KIND_VER := v0.17.0
KIND_BIN := kind
KIND :=  $(TOOLS_BIN_DIR)/$(KIND_BIN)
$(KIND): ## Build kind into tools folder.
	GOBIN=$(TOOLS_BIN_DIR) go install -tags tools sigs.k8s.io/kind@$(KIND_VER)

KUBECTL_VER := v1.22.4
KUBECTL_BIN := kubectl
KUBECTL := $(TOOLS_BIN_DIR)/$(KUBECTL_BIN)-$(KUBECTL_VER)
$(KUBECTL): ## Build kubectl from tools folder.
	mkdir -p $(TOOLS_BIN_DIR)
	rm -f "$(TOOLS_BIN_DIR)/$(KUBECTL_BIN)*"
	curl --retry 2 -fsL https://storage.googleapis.com/kubernetes-release/release/$(KUBECTL_VER)/bin/$(GOOS)/$(GOARCH)/kubectl -o $(KUBECTL)
	ln -sf $(KUBECTL) $(TOOLS_BIN_DIR)/$(KUBECTL_BIN)
	chmod +x $(KUBECTL) $(TOOLS_BIN_DIR)/$(KUBECTL_BIN)
	
install-tools: $(ENVTEST) $(CONTROLLER_GEN) $(KUBECTL) $(ENVSUBST) $(KUSTOMIZE) $(KIND)
