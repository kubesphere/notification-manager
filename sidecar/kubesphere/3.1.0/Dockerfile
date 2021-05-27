# Copyright 2018 The KubeSphere Authors. All rights reserved.
# Use of this source code is governed by a Apache license
# that can be found in the LICENSE file.

# Copyright 2018 The KubeSphere Authors. All rights reserved.
# Use of this source code is governed by a Apache license
# that can be found in the LICENSE file.

FROM golang:1.13 as kubesphere-sidecar

COPY / /
COPY pkg/ pkg/
WORKDIR /
ENV GOPROXY=https://goproxy.io
RUN CGO_ENABLED=0 GO111MODULE=on go build -a -i -ldflags '-w -s' -o kubesphere-sidecar cmd/main.go

FROM kubespheredev/distroless-static:nonroot
WORKDIR /
COPY --from=kubesphere-sidecar /kubesphere-sidecar .
USER nonroot:nonroot

ENTRYPOINT ["/kubesphere-sidecar"]


