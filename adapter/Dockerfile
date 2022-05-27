# Copyright 2018 The KubeSphere Authors. All rights reserved.
# Use of this source code is governed by a Apache license
# that can be found in the LICENSE file.

# Copyright 2018 The KubeSphere Authors. All rights reserved.
# Use of this source code is governed by a Apache license
# that can be found in the LICENSE file.

FROM golang:1.17 as notification-adapter

COPY / /
WORKDIR /
ENV GOPROXY=https://goproxy.io
RUN CGO_ENABLED=0 GO111MODULE=on go build -i -ldflags '-w -s' -o notification-adapter cmd/main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM kubesphere/distroless-static:nonroot
WORKDIR /
COPY --from=notification-adapter /notification-adapter .
USER nonroot:nonroot

ENTRYPOINT ["/notification-adapter"]
