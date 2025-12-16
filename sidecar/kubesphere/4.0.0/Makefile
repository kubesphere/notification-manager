# Copyright 2018 The KubeSphere Authors. All rights reserved.
# Use of this source code is governed by a Apache license
# that can be found in the LICENSE file.

IMG ?= kubesphere/notification-tenant-sidecar:v4.2.0
AMD64 ?= -amd64

all: docker-build

# Build tenant sidecar binary
tenant-sidecar:
	go build -o tenant-sidecar main.go backend.go

# Build the docker image
docker-build:
	docker buildx build --platform linux/amd64,linux/arm64 --push -f Dockerfile -t ${IMG} .

# Build the docker image for arm64
docker-build-amd64:
	docker build -f Dockerfile -t ${IMG}${AMD64} .

# Push the docker image
push-amd64:
	docker push ${IMG}${AMD64}
