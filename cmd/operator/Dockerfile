ARG GOPROXY="https://goproxy.io"
# Build the manager binary
FROM golang:1.17 as builder
ARG GOPROXY

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN GOPROXY=$GOPROXY go mod download

# Copy the go source
COPY cmd/operator/main.go main.go
COPY controllers/ controllers/
COPY pkg/ pkg/

# Build
RUN GOPROXY=$GOPROXY CGO_ENABLED=0 GO111MODULE=on go build -a -o notification-manager-operator main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM kubesphere/distroless-static:nonroot
WORKDIR /
COPY --from=builder /workspace/notification-manager-operator .
USER nonroot:nonroot

ENTRYPOINT ["/notification-manager-operator"]
