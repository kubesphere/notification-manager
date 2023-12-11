# Use of this source code is governed by a Apache license
# that can be found in the LICENSE file.

FROM golang:1.20 as tenant-sidecar

COPY / /
WORKDIR /
ENV GOPROXY=https://goproxy.io
RUN CGO_ENABLED=0 GO111MODULE=on go build -a -o tenant-sidecar main.go backend.go

FROM kubesphere/distroless-static:nonroot
WORKDIR /
COPY --from=tenant-sidecar /tenant-sidecar .
USER nonroot:nonroot

ENTRYPOINT ["/tenant-sidecar"]

