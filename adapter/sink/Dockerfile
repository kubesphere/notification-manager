# Copyright 2018 The KubeSphere Authors. All rights reserved.
# Use of this source code is governed by a Apache license
# that can be found in the LICENSE file.

# Copyright 2018 The KubeSphere Authors. All rights reserved.
# Use of this source code is governed by a Apache license
# that can be found in the LICENSE file.

FROM golang:1.13 as notification-adapter

COPY cmd/main.go /
COPY cmd/type.go /
WORKDIR /
ENV GOPROXY=https://goproxy.io
RUN CGO_ENABLED=0 GO111MODULE=on go build -i -ldflags '-w -s' -o notification-adapter main.go type.go

FROM alpine:3.9

COPY --from=notification-adapter /notification-adapter /usr/local/bin/

RUN apk add --update ca-certificates && update-ca-certificates
RUN apk add curl
RUN adduser -D -g kubesphere -u 1002 kubesphere
RUN chown -R kubesphere:kubesphere /usr/local/bin/notification-adapter
RUN apk add libcap
RUN setcap 'CAP_NET_BIND_SERVICE=+ep' /usr/local/bin/notification-adapter

USER kubesphere
CMD ["sh"]
