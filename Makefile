VERSION?=$(shell cat VERSION | tr -d " \t\n\r")
# Image URL to use all building/pushing image targets
IMG ?= kubespheredev/notification-manager-operator:$(VERSION)
NM_IMG ?= kubespheredev/notification-manager:$(VERSION)
# Image URL for arm64 to use all building/pushing image targets
ARM64 ?= -arm64
IMG_ARM64 ?= kubespheredev/notification-manager-operator:$(VERSION)$(ARM64)
NM_IMG_ARM64 ?= kubespheredev/notification-manager:$(VERSION)$(ARM64)
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
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=controller-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	cd config/manager && kustomize edit set image controller=${IMG} && cd ../../
	kustomize build config/default | sed -e '/creationTimestamp/d' > config/bundle.yaml
	kustomize build config/samples | sed -e '/creationTimestamp/d' > config/samples/bundle.yaml

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build all docker images
build: build-op build-nm

# Build the docker image
build-op: test
	docker build -f cmd/operator/Dockerfile . -t ${IMG} --network host

# Build the docker image
build-nm: test
	docker build -f cmd/notification-manager/Dockerfile . -t ${NM_IMG} --network host

# Push the docker image
push:
	docker push ${IMG}
	docker push ${NM_IMG}

# Build all docker images for arm64
build-arm64: build-op-arm64 build-nm-arm64

# Build the docker image for arm64
build-op-arm64: test
	docker buildx build --push --platform linux/arm64 -f cmd/operator/Dockerfile . -t ${IMG_ARM64}

# Build the docker image for arm64
build-nm-arm64: test
	docker buildx build --push --platform linux/arm64 -f cmd/notification-manager/Dockerfile . -t ${NM_IMG_ARM64}

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
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.5 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif
