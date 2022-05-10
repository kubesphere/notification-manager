VERSION?=$(shell cat VERSION | tr -d " \t\n\r")
# Image URL to use all building/pushing image targets
REGISTRY?=kubesphere
IMG ?= $(REGISTRY)/notification-manager-operator:$(VERSION)
NM_IMG ?= $(REGISTRY)/notification-manager:$(VERSION)
AMD64 ?= -amd64
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manager

# Run tests
test: generate fmt vet manifests
	go test ./... -coverprofile cover.out

# Build binary
binary:
	go build -o bin/notification-manager-operator cmd/operator/main.go
	go build -o bin/notification-manager cmd/notification-manager/main.go

# Verify CRDs
verify: verify-crds

verify-crds: generate
	@if !(git diff --quiet HEAD config/crd); then \
		echo "generated files located at config/crd are out of date, run make generate manifests"; exit 1; \
	fi

# Build manager binary
manager: generate fmt vet
	go build -o bin/notification-manager-operator cmd/operator/main.go

# Build notification-manager binary
nm: fmt vet
	go build -o bin/notification-manager cmd/notification-manager/main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	go run cmd/operator/main.go

# Install CRDs into a cluster
install: manifests
	kustomize build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests
	kustomize build config/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	cd config/manager && kustomize edit set image controller=${IMG} && cd ../../
	kustomize build config/default | kubectl apply -f -

# Deploy custom resources in the configured Kubernetes cluster
deploy-samples: manifests
	kustomize build config/samples | kubectl apply -f -

# Delete samples, crds and operator
undeploy:
	kustomize build config/samples | kubectl delete -f -
	kustomize build config/default | kubectl delete -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=controller-role webhook paths=./pkg/apis/v2beta1 paths=./pkg/apis/v2beta2 output:crd:artifacts:config=config/crd/bases
	cd config/manager && kustomize edit set image controller=${IMG} && cd ../../
	kustomize build config/default | sed -e '/creationTimestamp/d' > config/bundle.yaml
	kustomize build config/samples | sed -e '/creationTimestamp/d' > config/samples/bundle.yaml
	kustomize build config/helm | sed -e '/creationTimestamp/d' > helm/crds/bundle.yaml

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./pkg/apis/v2beta1"
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./pkg/apis/v2beta2"

# Build all docker images for amd64 and arm64
build: test build-op build-nm

# Build the docker image for amd64 and arm64
build-op:
	docker buildx build --push --platform linux/amd64,linux/arm64 -f cmd/operator/Dockerfile . -t ${IMG}

# Build the docker image for amd64 and arm64
build-nm:
	docker buildx build --push --platform linux/amd64,linux/arm64 -f cmd/notification-manager/Dockerfile . -t ${NM_IMG}

# Build all docker images for amd64
build-amd64: test build-op-amd64 build-nm-amd64

# Build the docker image for amd64
build-op-amd64:
	docker build -f cmd/operator/Dockerfile . -t ${IMG}${AMD64}

# Build the docker image for amd64
build-nm-amd64:
	docker build -f cmd/notification-manager/Dockerfile . -t ${NM_IMG}${AMD64}

# Push the docker image
push-amd64:
	docker push ${IMG}${AMD64}
	docker push ${NM_IMG}${AMD64}

#docker-clean:
#	docker rmi `docker image ls|awk '{print $2,$3}'|grep none|awk '{print $2}'|tr "\n" " "`

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.4.1 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif
