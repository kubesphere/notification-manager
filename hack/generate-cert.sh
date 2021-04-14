#!/bin/bash

# This script create a new cert for conversion webhook.

cd hack/

openssl genrsa -out ca.key 2048
openssl req -x509 -new -nodes -key ca.key -subj "/C=CN/ST=HB/O=QC/CN=webhook-ca" -sha256 -days 36500 -out ca.crt
openssl genrsa -out server.key 2048
openssl req -new -nodes -keyout server.key -out server.csr -subj "/C=CN/ST=HB/O=QC/CN=notification-manager-webhook" -config openssl.cnf
#openssl req -new -sha256 -key server.key -subj "/C=CN/ST=HB/O=QC/CN=notification-manager-webhook.kubesphere-monitoring-system.svc" -out server.csr
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -extfile openssl.cnf -out server.crt -days 36500 -sha256 -extensions v3_req

sed -ri "s/(tls.crt: )[^\n]*/\1$(cat server.crt | base64 -w 0)/" ../config/cert/webhook-server-cert.yaml
sed -ri "s/(tls.key: )[^\n]*/\1$(cat server.key | base64 -w 0)/" ../config/cert/webhook-server-cert.yaml
sed -ri "s/(caBundle: )[^\n]*/\1$(cat ca.crt | base64 -w 0)/" ../config/crd/patches/webhook_in_configs.yaml
sed -ri "s/(caBundle: )[^\n]*/\1$(cat ca.crt | base64 -w 0)/" ../config/crd/patches/webhook_in_receivers.yaml
sed -ri "s/(caBundle: )[^\n]*/\1$(cat ca.crt | base64 -w 0)/" ../config/webhook/manifests.yaml

rm -rf ca.* ca.srt server.*
