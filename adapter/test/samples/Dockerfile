# Copyright 2018 The KubeSphere Authors. All rights reserved.
# Use of this source code is governed by a Apache license
# that can be found in the LICENSE file.

# Copyright 2018 The KubeSphere Authors. All rights reserved.
# Use of this source code is governed by a Apache license
# that can be found in the LICENSE file.

FROM golang:1.13 as socket-server

COPY / /
WORKDIR /
ENV GOPROXY=https://goproxy.io
RUN CGO_ENABLED=0 GO111MODULE=on go build -i -ldflags '-w -s' -o socket-server main.go

FROM alpine:3.9

COPY --from=socket-server /socket-server /usr/local/bin/

RUN apk add --update ca-certificates && update-ca-certificates
RUN apk add curl
RUN adduser -D -g kubesphere -u 1002 kubesphere
RUN chown -R kubesphere:kubesphere /usr/local/bin/socket-server
RUN apk add libcap
RUN setcap 'CAP_NET_BIND_SERVICE=+ep' /usr/local/bin/socket-server

USER kubesphere
CMD ["sh"]
