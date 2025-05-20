# Copyright 2018 The KubeSphere Authors. All rights reserved.
# Use of this source code is governed by a Apache license
# that can be found in the LICENSE file.

# Copyright 2018 The KubeSphere Authors. All rights reserved.
# Use of this source code is governed by a Apache license
# that can be found in the LICENSE file.

FROM golang:1.16 AS notification-adapter

COPY cmd/main.go /
COPY cmd/types.go /
COPY go.* /
WORKDIR /
ENV GOPROXY=https://goproxy.io
RUN CGO_ENABLED=0 GO111MODULE=on go build -i -ldflags '-w -s' -o notification-adapter main.go types.go

FROM alpine:3.9

COPY --from=notification-adapter /notification-adapter /usr/local/bin/

RUN adduser -D -g kubesphere -u 1002 kubesphere && \
    chown -R kubesphere:kubesphere /usr/local/bin/notification-adapter && \
    apk add libcap && \
    setcap 'CAP_NET_BIND_SERVICE=+ep' /usr/local/bin/notification-adapter

USER kubesphere
CMD ["sh"]
