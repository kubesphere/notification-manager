# Copyright 2018 The KubeSphere Authors. All rights reserved.
# Use of this source code is governed by a Apache license
# that can be found in the LICENSE file.

# Copyright 2018 The KubeSphere Authors. All rights reserved.
# Use of this source code is governed by a Apache license
# that can be found in the LICENSE file.

FROM golang:1.13 as tenant-sidecar

COPY / /
COPY pkg/ pkg/
WORKDIR /
ENV GOPROXY=https://goproxy.io
RUN CGO_ENABLED=0 GO111MODULE=on go build -a -i -ldflags '-w -s' -o tenant-sidecar cmd/main.go

FROM kubesphere/distroless-static:nonroot
WORKDIR /
COPY --from=tenant-sidecar /tenant-sidecar .
USER nonroot:nonroot

ENTRYPOINT ["/tenant-sidecar"]


