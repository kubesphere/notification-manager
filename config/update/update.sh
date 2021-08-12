#!/bin/bash

backup_dir="./backup"
output_dir="./crds"

notification_name=$1
if [ "$notification_name" = "" ]; then
  notification_name="notification-manager"
fi

notification_namespace=$2
if [ "$notification_namespace" = "" ]; then
  notification_namespace="kubesphere-monitoring-system"
fi

mkdir -p $backup_dir
mkdir -p $output_dir

backup_v1alpha1() {
  # export notification manager
  src=$(kubectl get notificationmanagers.notification.kubesphere.io -n "$notification_namespace" "$notification_name" -o json)
  # shellcheck disable=SC2046
  echo "$src" | jq >"${backup_dir}"/notification-manager-"$(echo "$src" | jq -r '.metadata.namespace')"-"$(echo "$src" | jq -r '.metadata.name')".json

  # export dingtalk config
  # shellcheck disable=SC2207
  local ns=($(kubectl get dingtalkconfigs.notification.kubesphere.io -A | sed -n '1!p' | awk '{print $1}'))
  # shellcheck disable=SC2207
  dingtalkconfigs=($(kubectl get dingtalkconfigs.notification.kubesphere.io -A | sed -n '1!p' | awk '{print $2}'))

  for ((i = 0; i < ${#ns[@]}; i++)); do
    src=$(kubectl get dingtalkconfigs.notification.kubesphere.io "${dingtalkconfigs[i]}" -n "${ns[i]}" -ojson)
    echo "$src" | jq >"${backup_dir}/dingtalkconfig-$(echo "$src" | jq -r '.metadata.namespace')-$(echo "$src" | jq -r '.metadata.name').json"
  done

  # export email config
  # shellcheck disable=SC2207
  local ns=($(kubectl get emailconfigs.notification.kubesphere.io -A | sed -n '1!p' | awk '{print $1}'))
  # shellcheck disable=SC2207
  emailconfigs=($(kubectl get emailconfigs.notification.kubesphere.io -A | sed -n '1!p' | awk '{print $2}'))

  for ((i = 0; i < ${#ns[@]}; i++)); do
    src=$(kubectl get emailconfigs.notification.kubesphere.io "${emailconfigs[i]}" -n "${ns[i]}" -ojson)
    echo "$src" | jq >"${backup_dir}/emailconfig-$(echo "$src" | jq -r '.metadata.namespace')-$(echo "$src" | jq -r '.metadata.name').json"
  done

  # export email receiver
  # shellcheck disable=SC2207
  local ns=($(kubectl get emailreceivers.notification.kubesphere.io -A | sed -n '1!p' | awk '{print $1}'))
  # shellcheck disable=SC2207
  emailreceivers=($(kubectl get emailreceivers.notification.kubesphere.io -A | sed -n '1!p' | awk '{print $2}'))

  for ((i = 0; i < ${#ns[@]}; i++)); do
    src=$(kubectl get emailreceivers.notification.kubesphere.io "${emailreceivers[i]}" -n "${ns[i]}" -ojson)
    echo "$src" | jq >"${backup_dir}/emailreceiver-$(echo "$src" | jq -r '.metadata.namespace')-$(echo "$src" | jq -r '.metadata.name').json"
  done

  # export slack config
  # shellcheck disable=SC2207
  local ns=($(kubectl get slackconfigs.notification.kubesphere.io -A | sed -n '1!p' | awk '{print $1}'))
  # shellcheck disable=SC2207
  slackconfigs=($(kubectl get slackconfigs.notification.kubesphere.io -A | sed -n '1!p' | awk '{print $2}'))

  for ((i = 0; i < ${#ns[@]}; i++)); do
    src=$(kubectl get slackconfigs.notification.kubesphere.io "${slackconfigs[i]}" -n "${ns[i]}" -ojson)
    echo "$src" | jq >"${backup_dir}/slackconfig-$(echo "$src" | jq -r '.metadata.namespace')-$(echo "$src" | jq -r '.metadata.name').json"
  done

  # export slack receiver
  # shellcheck disable=SC2207
  local ns=($(kubectl get slackreceivers.notification.kubesphere.io -A | sed -n '1!p' | awk '{print $1}'))
  # shellcheck disable=SC2207
  slackreceivers=($(kubectl get slackreceivers.notification.kubesphere.io -A | sed -n '1!p' | awk '{print $2}'))

  for ((i = 0; i < ${#ns[@]}; i++)); do
    src=$(kubectl get slackreceivers.notification.kubesphere.io "${slackreceivers[i]}" -n "${ns[i]}" -ojson)
    echo "$src" | jq >"${backup_dir}/slackreceiver-$(echo "$src" | jq -r '.metadata.namespace')-$(echo "$src" | jq -r '.metadata.name').json"
  done

  # export webhook config
  # shellcheck disable=SC2207
  local ns=($(kubectl get webhookconfigs.notification.kubesphere.io -A | sed -n '1!p' | awk '{print $1}'))
  # shellcheck disable=SC2207
  webhookconfigs=($(kubectl get webhookconfigs.notification.kubesphere.io -A | sed -n '1!p' | awk '{print $2}'))

  for ((i = 0; i < ${#ns[@]}; i++)); do
    src=$(kubectl get webhookconfigs.notification.kubesphere.io "${webhookconfigs[i]}" -n "${ns[i]}" -ojson)
    echo "$src" | jq >"${backup_dir}"/webhookconfig-"$(echo "$src" | jq -r '.metadata.namespace')"-"$(echo "$src" | jq -r '.metadata.name')".json
  done

  # export wechat config
  # shellcheck disable=SC2207
  local ns=($(kubectl get wechatconfigs.notification.kubesphere.io -A | sed -n '1!p' | awk '{print $1}'))
  # shellcheck disable=SC2207
  wechatconfigs=($(kubectl get wechatconfigs.notification.kubesphere.io -A | sed -n '1!p' | awk '{print $2}'))

  for ((i = 0; i < ${#ns[@]}; i++)); do
    src=$(kubectl get wechatconfigs.notification.kubesphere.io "${wechatconfigs[i]}" -n "${ns[i]}" -ojson)
    echo "$src" | jq >"${backup_dir}/wechatconfig-$(echo "$src" | jq -r '.metadata.namespace')-$(echo "$src" | jq -r '.metadata.name').json"
  done

  # export wechat receiver
  # shellcheck disable=SC2207
  local ns=($(kubectl get wechatreceivers.notification.kubesphere.io -A | sed -n '1!p' | awk '{print $1}'))
  # shellcheck disable=SC2207
  wechatreceivers=($(kubectl get wechatreceivers.notification.kubesphere.io -A | sed -n '1!p' | awk '{print $2}'))

  for ((i = 0; i < ${#ns[@]}; i++)); do
    src=$(kubectl get wechatreceivers.notification.kubesphere.io "${wechatreceivers[i]}" -n "${ns[i]}" -ojson)
    echo "$src" | jq >"${backup_dir}/wechatreceiver-$(echo "$src" | jq -r '.metadata.namespace')-$(echo "$src" | jq -r '.metadata.name').json"
  done
}

backup_v2alpha1() {

  # shellcheck disable=SC2207
  array=($(kubectl get notification-manager | sed -n '1!p' | awk '{print $1}'))
  for ((i = 0; i < ${#array[@]}; i++)); do

    if [ "name" = "${array[i]}" ]; then
      continue
    fi

    # shellcheck disable=SC2206
    names=(${array[i]//\// })

    if [ "${#names[@]}" = "1" ]; then
      continue
    fi

    src=$(kubectl get "${names[0]}" "${names[1]}" -ojson)
    kind=$(echo "$src" | jq -r '.kind')$(echo "$src" | jq -r '.kind' | awk '{ print tolower($0) }')
    echo "$src" | jq >"${backup_dir}/$kind-$(echo "$src" | jq -r '.metadata.name').json"
  done
}

delete_invalid_info() {
  src=$1
  dest=${src%\"}
  dest=${dest#\"}
  dest=$(echo "$dest" |
    jq 'del(.metadata.namespace)' |
    jq 'del(.metadata.creationTimestamp)' |
    jq 'del(.metadata.generation)' |
    jq 'del(.metadata.managedFields)' |
    jq 'del(.metadata.resourceVersion)' |
    jq 'del(.metadata.selfLink)' |
    jq 'del(.metadata.uid)' |
    jq 'delpaths([["metadata","annotations", "kubectl.kubernetes.io/last-applied-configuration"]])' |
    jq 'delpaths([["metadata","annotations", "reloadtimestamp"]])')
  echo "$dest"
}

update_v1alpha1() {

  files=$(ls "$backup_dir")
  for file in $files; do

    src=$(jq '.' "$backup_dir/$file")
    namespace=$(echo "$src" | jq -r '.metadata.namespace')
    kind=$(echo "$src" | jq -r '.kind')

    if [ "$kind" = "NotificationManager" ]; then
      resource=$(echo "$src" |
        jq 'setpath(["apiVersion"]; "notification.kubesphere.io/v2beta1")' |
        jq 'setpath(["spec", "defaultSecretNamespace"]; "kubesphere-monitoring-federated")' |
        jq 'del(.metadata.namespace)')
      resource=$(delete_invalid_info "\"$(echo "$resource" | jq -c)\"")

      if [ "$(echo "$resource" | jq '.spec.receivers.options' | jq 'has("global")')" != "true" ]; then
        resource=$(echo "$resource" | jq '.spec.receivers.options.global.templateFile |= .+ ["/etc/notification-manager/template"]')
      fi

      if [ "$(echo "$resource" | jq '.spec' | jq 'has("volumes")')" != "true" ]; then
        resource=$(echo "$resource" |
          jq 'setpath(["spec", "volumes"]; [{"configMap": {"defaultMode": 420,"name": "notification-manager-template"},"name": "notification-manager-template"}])')
      fi

      if [ "$(echo "$resource" | jq '.spec' | jq 'has("volumeMounts")')" != "true" ]; then
        resource=$(echo "$resource" |
          jq 'setpath(["spec", "volumeMounts"]; [{"mountPath": "/etc/notification-manager/","name": "notification-manager-template","readOnly": true}])')
      fi

      echo "$resource" | jq >"${output_dir}/notification-manager-$(echo "$resource" | jq -r '.metadata.name').json"
      continue
    fi

    resource=$(echo "$src" | jq 'setpath(["apiVersion"]; "notification.kubesphere.io/v2beta1")')
    name=$(echo "$kind" | awk '{ print tolower($0) }')-$(echo "$resource" | jq -r '.metadata.namespace')-$(echo "$resource" | jq -r '.metadata.name')
    resource=$(echo "$resource" | jq --arg name "$name" 'setpath(["metadata", "name"]; $name)' | jq 'del(.metadata.namespace)')
    resource=$(delete_invalid_info "\"$(echo "$resource" | jq -c)\"")

    if [[ "$kind" == *Config ]]; then
      resource=$(echo "$resource" | jq 'setpath(["kind"]; "Config")')
    elif [[ "$kind" == *Receiver ]]; then
      resource=$(echo "$resource" | jq 'setpath(["kind"]; "Receiver")')
    else
      continue
    fi

    type=""
    if [[ "$kind" == DingTalk* ]]; then
      type="dingtalk"
    elif [[ "$kind" == Email* ]]; then
      type="email"
    elif [[ "$kind" == Slack* ]]; then
      type="slack"
    elif [[ "$kind" == Webhook* ]]; then
      type="webhook"
    elif [[ "$kind" == Wechat* ]]; then
      type="wechat"
    else
      continue
    fi

    resource=$(echo "$resource" | jq --arg type $type 'setpath(["spec", $type]; .spec)')
    str=$(echo "$resource" | jq '.spec' | jq 'keys')
    # shellcheck disable=SC2001
    str=$(echo "$str" | sed 's/\"//g')
    # shellcheck disable=SC2001
    str=$(echo "$str" | sed 's/\[//g')
    # shellcheck disable=SC2001
    str=$(echo "$str" | sed 's/\]//g')
    # shellcheck disable=SC2206
    keys=(${str//,/ })

    for ((j = 0; j < ${#keys[@]}; j++)); do
      if [ "${keys[j]}" = "$type" ]; then
        continue
      fi

      resource=$(echo "$resource" | jq --arg key "${keys[j]}" 'delpaths([["spec", $key]])')
    done

    if [ "$kind" = "DingTalkConfig" ]; then

      if [ "$(echo "$resource" | jq '.spec.dingtalk' | jq 'has("conversation")')" = "true" ]; then
        config=$(echo "$resource" |
          jq --arg ns "$namespace" 'setpath(["spec", "dingtalk", "conversation", "appkey", "namespace"]; $ns)' |
          jq --arg ns "$namespace" 'setpath(["spec", "dingtalk", "conversation", "appsecret", "namespace"]; $ns)' |
          jq 'del(.spec.dingtalk.conversation.chatid)' |
          jq 'del(.spec.dingtalk.chatbot)')

        echo "$config" | jq >"${output_dir}/$(echo "$config" | jq -r '.metadata.name').json"
      fi

      name=dingtalkreceiver-$(echo "$src" | jq -r '.metadata.namespace')-$(echo "$src" | jq -r '.metadata.name')
      resource=$(echo "$resource" |
        jq 'setpath(["kind"]; "Receiver")' |
        jq --arg name "$name" 'setpath(["metadata", "name"]; $name)')

      if [ "default" = "$(echo "$receiver" | jq -r '.metadata.labels.type')" ]; then
        resource=$(echo "$resource" |
          jq 'setpath(["metadata","labels", "type"]; "global")')
      fi

      if [ "$(echo "$resource" | jq '.spec.dingtalk' | jq 'has("chatbot")')" = "true" ]; then
        resource=$(echo "$resource" |
          jq --arg ns "$namespace" 'setpath(["spec", "dingtalk", "chatbot", "webhook", "namespace"]; $ns)')

        if [ "$(echo "$resource" | jq '.spec.dingtalk.chatbot' | jq 'has("secret")')" = "true" ]; then
          resource=$(echo "$resource" |
            jq --arg ns "$namespace" 'setpath(["spec", "dingtalk", "chatbot", "secret", "namespace"]; $ns)')
        fi
      fi

      if [ "$(echo "$resource" | jq '.spec.dingtalk' | jq 'has("conversation")')" = "true" ]; then
        resource=$(echo "$resource" |
          jq 'setpath(["spec", "dingtalk", "conversation", "chatids"]; [.spec.dingtalk.conversation.chatid])' |
          jq 'del(.spec.dingtalk.conversation.chatid)' |
          jq 'del(.spec.dingtalk.conversation.appkey)' |
          jq 'del(.spec.dingtalk.conversation.appsecret)')
      fi
    fi

    if [ "$kind" = "EmailConfig" ]; then

      resource=$(echo "$resource" |
        jq --arg ns "$namespace" 'setpath(["spec", "email", "authPassword", "namespace"]; $ns)')

      if [ "$(echo "$resource" | jq '.spec.email' | jq 'has("authSecret")')" = "true" ]; then
        resource=$(echo "$resource" |
          jq --arg ns "$namespace" 'setpath(["spec", "email", "authSecret", "namespace"]; $ns)')
      fi

      if [ "$(echo "$config" | jq '.spec.email' | jq 'has("tls")')" = "true" ]; then
        resource=$(echo "$resource" |
          jq --arg ns "$namespace" 'setpath(["spec", "email", "tls", "rootCA", "namespace"]; $ns)' |
          jq --arg ns "$namespace" 'setpath(["spec", "email", "tls", "clientCertificate", "cert", "namespace"]; $ns)' |
          jq --arg ns "$namespace" 'setpath(["spec", "email", "tls", "clientCertificate", "key", "namespace"]; $ns)')
      fi

      port=$(echo "$resource" | jq -r '.spec.email.smartHost.port')
      resource=$(echo "$resource" | jq --argjson port "$port" 'setpath(["spec", "email", "smartHost", "port"]; $port)')
    fi

    if [ "$kind" = "SlackConfig" ]; then
      resource=$(echo "$resource" |
        jq --arg ns "$namespace" 'setpath(["spec", "slack", "slackTokenSecret", "namespace"]; $ns)')
    fi

    if [ "$kind" = "SlackReceiver" ]; then
      resource=$(echo "$resource" |
        jq 'setpath(["spec", "slack", "channels"]; [.spec.slack.channel])' |
        jq 'del(.spec.slack.channel)')
    fi

    if [ "$kind" = "WebhookConfig" ]; then

      name=webhookreceiver-$(echo "$src" | jq -r '.metadata.namespace')-$(echo "$src" | jq -r '.metadata.name')
      resource=$(echo "$resource" | jq 'setpath(["kind"]; "Receiver")' |
        jq --arg name "$name" 'setpath(["metadata", "name"]; $name)')

      if [ "$(echo "$resource" | jq '.spec.webhook' | jq 'has("httpConfig")')" = "true" ]; then
        if [ "$(echo "$resource" | jq '.spec.webhook.httpConfig' | jq 'has("basicAuth")')" = "true" ]; then
          resource=$(echo "$resource" |
            jq --arg ns "$namespace" 'setpath(["spec", "webhook", "httpConfig", "basicAuth", "password", "namespace"]; $ns)')
        fi

        if [ "$(echo "$resource" | jq '.spec.webhook.httpConfig' | jq 'has("bearerToken")')" = "true" ]; then
          resource=$(echo "$resource" |
            jq --arg ns "$namespace" 'setpath(["spec", "webhook", "httpConfig", "bearerToken", "namespace"]; $ns)')
        fi

        if [ "$(echo "$resource" | jq '.spec.webhook.httpConfig' | jq 'has("tlsConfig")')" = "true" ]; then
          if [ "$(echo "$resource" | jq '.spec.webhook.httpConfig.tlsConfig' | jq 'has("rootCA")')" = "true" ]; then
            resource=$(echo "$resource" |
              jq --arg ns "$namespace" 'setpath(["spec", "webhook", "httpConfig", "tlsConfig", "rootCA", "namespace"]; $ns)')
          fi

          if [ "$(echo "$resource" | jq '.spec.webhook.httpConfig.tlsConfig' | jq 'has("clientCertificate")')" = "true" ]; then
            resource=$(echo "$resource" |
              jq --arg ns "$namespace" 'setpath(["spec", "webhook", "httpConfig", "tlsConfig", "clientCertificate", "cert", "namespace"]; $ns)' |
              jq --arg ns "$namespace" 'setpath(["spec", "webhook", "httpConfig", "tlsConfig", "clientCertificate", "key", "namespace"]; $ns)')
          fi
        fi
      fi

      if [ "default" = "$(echo "$resource" | jq -r '.metadata.labels.type')" ]; then
        resource=$(echo "$resource" |
          jq 'setpath(["metadata","labels", "type"]; "global")')
      fi

    fi

    if [ "$kind" = "WechatConfig" ]; then
      resource=$(echo "$resource" |
        jq --arg ns "$namespace" 'setpath(["spec", "wechat", "wechatApiSecret", "namespace"]; $ns)')
    fi

    if [ "$kind" = "WechatReceiver" ]; then

      if [ "$(echo "$resource" | jq '.spec.wechat' | jq 'has("toUser")')" = "true" ]; then
        toUser=$(echo "$resource" | jq -r '.spec.wechat.toUser')
        # shellcheck disable=SC2206
        users=(${toUser//|/ })

        receiver=$(echo "$resource" | jq 'setpath(["spec", "wechat", "toUser"]; [])')
        for ((j = 0; j < ${#users[@]}; j++)); do
          resource=$(echo "$receiver" | jq --arg user "${users[j]}" '.spec.wechat.toUser |= .+ [$user]')
        done
      fi

      if [ "$(echo "$resource" | jq '.spec.wechat' | jq 'has("toParty")')" = "true" ]; then
        toParty=$(echo "$resource" | jq -r '.spec.wechat.toParty')
        # shellcheck disable=SC2206
        parties=(${toParty//|/ })

        resource=$(echo "$resource" | jq 'setpath(["spec", "wechat", "toParty"]; [])')
        for ((j = 0; j < ${#parties[@]}; j++)); do
          resource=$(echo "$resource" | jq --arg party "${parties[j]}" '.spec.wechat.toParty |= .+ [$party]')
        done
      fi

      if [ "$(echo "$resource" | jq '.spec.wechat' | jq 'has("toTag")')" = "true" ]; then
        toTag=$(echo "$resource" | jq -r '.spec.wechat.toTag')
        # shellcheck disable=SC2206
        tags=(${toTag//|/ })

        resource=$(echo "$resource" | jq 'setpath(["spec", "wechat", "toTag"]; [])')
        for ((j = 0; j < ${#tags[@]}; j++)); do
          resource=$(echo "$resource" | jq --arg tag "${tags[j]}" '.spec.wechat.toTag |= .+ [$tag]')
        done
      fi
    fi

    echo "$resource" | jq >"${output_dir}/$(echo "$resource" | jq -r '.metadata.name').json"
  done
}

update_v2alpha1() {

  files=$(ls "$backup_dir")
  for file in $files; do

    src=$(jq '.' "$backup_dir/$file")
    local namespace="$notification_namespace"
    kind=$(echo "$src" | jq -r '.kind')

    if [ "$kind" = "NotificationManager" ]; then

      default_ns=$(echo "$src" | jq '.spec.defaultSecretNamespace')
      if [ "$default_ns" != "null" ] && [ "$default_ns" != "" ]; then
        namespace"$default_ns"
      fi

      resource=$(echo "$src" |
        jq 'setpath(["apiVersion"]; "notification.kubesphere.io/v2beta1")' |
        jq 'setpath(["spec", "defaultSecretNamespace"]; "kubesphere-monitoring-federated")')
      resource=$(delete_invalid_info "\"$(echo "$resource" | jq -c)\"")

      echo "$resource" | jq >"${output_dir}/notification-manager-$(echo "$resource" | jq -r '.metadata.name').json"
      continue
    fi

    resource=$(echo "$src" | jq 'setpath(["apiVersion"]; "notification.kubesphere.io/v2beta1")')
    name=$(echo "$kind" | awk '{ print tolower($0) }')-$(echo "$resource" | jq -r '.metadata.name')
    resource=$(echo "$resource" | jq --arg name "$name" 'setpath(["metadata", "name"]; $name)')
    resource=$(delete_invalid_info "\"$(echo "$resource" | jq -c)\"")

    if [[ "$kind" == *Config ]]; then
      resource=$(echo "$resource" | jq 'setpath(["kind"]; "Config")')
    elif [[ "$kind" == *Receiver ]]; then
      resource=$(echo "$resource" | jq 'setpath(["kind"]; "Receiver")')
    else
      continue
    fi

    type=""
    if [[ "$kind" == DingTalk* ]]; then
      type="dingtalk"
    elif [[ "$kind" == Email* ]]; then
      type="email"
    elif [[ "$kind" == Slack* ]]; then
      type="slack"
    elif [[ "$kind" == Webhook* ]]; then
      type="webhook"
    elif [[ "$kind" == Wechat* ]]; then
      type="wechat"
    else
      continue
    fi

    resource=$(echo "$resource" | jq --arg type $type 'setpath(["spec", $type]; .spec)')
    str=$(echo "$resource" | jq '.spec' | jq 'keys')
    # shellcheck disable=SC2001
    str=$(echo "$str" | sed 's/\"//g')
    # shellcheck disable=SC2001
    str=$(echo "$str" | sed 's/\[//g')
    # shellcheck disable=SC2001
    str=$(echo "$str" | sed 's/\]//g')
    # shellcheck disable=SC2206
    keys=(${str//,/ })

    for ((j = 0; j < ${#keys[@]}; j++)); do
      if [ "${keys[j]}" = "$type" ]; then
        continue
      fi

      resource=$(echo "$resource" | jq --arg key "${keys[j]}" 'delpaths([["spec", $key]])')
    done

    if [[ "$kind" == "DingTalkConfig" ]]; then
      if [ "$(echo "$resource" | jq '.spec.dingtalk' | jq 'has("conversation")')" = "true" ]; then
        resource=$(echo "$resource" |
          jq --arg ns "$namespace" 'setpath(["spec", "dingtalk", "conversation", "appkey", "namespace"]; $ns)' |
          jq --arg ns "$namespace" 'setpath(["spec", "dingtalk", "conversation", "appsecret", "namespace"]; $ns)')
      fi

      if [ "$(echo "$resource" | jq '.spec.dingtalk')" = "null" ]; then
        continue
      fi
    fi

    if [[ "$kind" == "DingTalkReceiver" ]]; then
      if [ "$(echo "$resource" | jq '.spec.dingtalk' | jq 'has("chatbot")')" = "true" ]; then
        resource=$(echo "$resource" |
          jq --arg ns "$namespace" 'setpath(["spec", "dingtalk", "chatbot", "webhook", "namespace"]; $ns)')
        if [ "$(echo "$resource" | jq '.spec.dingtalk.chatbot' | jq 'has("secret")')" = "true" ]; then
          resource=$(echo "$resource" |
            jq --arg ns "$namespace" 'setpath(["spec", "dingtalk", "chatbot", "secret", "namespace"]; $ns)')
        fi
      fi

      if [ "$(echo "$resource" | jq '.spec.dingtalk' | jq 'has("conversation")')" = "true" ]; then
        resource=$(
          echo "$resource" |
            jq 'setpath(["spec", "dingtalk", "conversation", "chatids"]; [.spec.dingtalk.conversation.chatid])'
          jq 'del(.spec.dingtalk.conversation.chatid)'
        )
      fi
    fi

    if [[ "$kind" == "EmailConfig" ]]; then
      port=$(echo "$resource" | jq -r '.spec.email.smartHost.port')
      resource=$(echo "$resource" |
        jq --argjson port "$port" 'setpath(["spec", "email", "smartHost", "port"]; $port)' |
        jq --arg ns "$namespace" 'setpath(["spec", "email", "authPassword", "namespace"]; $ns)')

      if [ "$(echo "$resource" | jq '.spec.email' | jq 'has("authSecret")')" = "true" ]; then
        resource=$(echo "$resource" |
          jq --arg ns "$namespace" 'setpath(["spec", "email", "authSecret", "namespace"]; $ns)')
      fi

      if [ "$(echo "$resource" | jq '.spec.email' | jq 'has("tls")')" = "true" ]; then
        resource=$(echo "$resource" |
          jq --arg ns "$namespace" 'setpath(["spec", "email", "tls", "rootCA", "namespace"]; $ns)' |
          jq --arg ns "$namespace" 'setpath(["spec", "email", "tls", "clientCertificate", "cert", "namespace"]; $ns)' |
          jq --arg ns "$namespace" 'setpath(["spec", "email", "tls", "clientCertificate", "key", "namespace"]; $ns)')
      fi
    fi

    if [[ "$kind" == "SlackConfig" ]]; then
      resource=$(echo "$resource" |
        jq --arg ns "$namespace" 'setpath(["spec", "slack", "slackTokenSecret", "namespace"]; $ns)')
    fi

    if [[ "$kind" == "SlackReceiver" ]]; then
      resource=$(echo "$resource" |
        jq 'setpath(["spec", "slack", "channels"]; [.spec.slack.channel])' |
        jq 'del(.spec.slack.channel)')
    fi

    if [[ "$kind" == "WebhookConfig" ]]; then
      continue
    fi

    if [[ "$kind" == "WebhookReceiver" ]]; then
      if [ "$(echo "$resource" | jq '.spec.webhook' | jq 'has("httpConfig")')" = "true" ]; then
        if [ "$(echo "$resource" | jq '.spec.webhook.httpConfig' | jq 'has("basicAuth")')" = "true" ]; then
          resource=$(echo "$resource" |
            jq --arg ns "$namespace" 'setpath(["spec", "webhook", "httpConfig", "basicAuth", "password", "namespace"]; $ns)')
        fi

        if [ "$(echo "$resource" | jq '.spec.webhook.httpConfig' | jq 'has("bearerToken")')" = "true" ]; then
          resource=$(echo "$resource" |
            jq --arg ns "$namespace" 'setpath(["spec", "webhook", "httpConfig", "bearerToken", "namespace"]; $ns)')
        fi

        if [ "$(echo "$resource" | jq '.spec.webhook.httpConfig' | jq 'has("tlsConfig")')" = "true" ]; then
          if [ "$(echo "$resource" | jq '.spec.webhook.httpConfig.tlsConfig' | jq 'has("rootCA")')" = "true" ]; then
            resource=$(echo "$resource" |
              jq --arg ns "$namespace" 'setpath(["spec", "webhook", "httpConfig", "tlsConfig", "rootCA", "namespace"]; $ns)')
          fi

          if [ "$(echo "$resource" | jq '.spec.webhook.httpConfig.tlsConfig' | jq 'has("clientCertificate")')" = "true" ]; then
            resource=$(echo "$resource" |
              jq --arg ns "$namespace" 'setpath(["spec", "webhook", "httpConfig", "tlsConfig", "clientCertificate", "cert", "namespace"]; $ns)' |
              jq --arg ns "$namespace" 'setpath(["spec", "webhook", "httpConfig", "tlsConfig", "clientCertificate", "key", "namespace"]; $ns)')
          fi
        fi
      fi
    fi

    if [[ "$kind" == "WechatConfig" ]]; then
      resource=$(echo "$resource" |
        jq --arg ns "$namespace" 'setpath(["spec", "wechat", "wechatApiSecret", "namespace"]; $ns)')
    fi

    if [[ "$kind" == "WechatReceiver" ]]; then

      if [ "$(echo "$resource" | jq '.spec.wechat' | jq 'has("toUser")')" = "true" ]; then
        toUser=$(echo "$resource" | jq -r '.spec.wechat.toUser')
        # shellcheck disable=SC2206
        users=(${toUser//|/ })

        resource=$(echo "$resource" | jq 'setpath(["spec", "wechat", "toUser"]; [])')
        for ((j = 0; j < ${#users[@]}; j++)); do
          resource=$(echo "$resource" | jq --arg user "${users[j]}" '.spec.wechat.toUser |= .+ [$user]')
        done
      fi

      if [ "$(echo "$resource" | jq '.spec.wechat' | jq 'has("toParty")')" = "true" ]; then
        toParty=$(echo "$resource" | jq -r '.spec.wechat.toParty')
        # shellcheck disable=SC2206
        parties=(${toParty//|/ })

        resource=$(echo "$resource" | jq 'setpath(["spec", "wechat", "toParty"]; [])')
        for ((j = 0; j < ${#parties[@]}; j++)); do
          resource=$(echo "$resource" | jq --arg party "${parties[j]}" '.spec.wechat.toParty |= .+ [$party]')
        done
      fi

      if [ "$(echo "$resource" | jq '.spec.wechat' | jq 'has("toTag")')" = "true" ]; then
        toTag=$(echo "$resource" | jq -r '.spec.wechat.toTag')
        # shellcheck disable=SC2206
        tags=(${toTag//|/ })

        resource=$(echo "$resource" | jq 'setpath(["spec", "wechat", "toTag"]; [])')
        for ((j = 0; j < ${#tags[@]}; j++)); do
          resource=$(echo "$resource" | jq --arg tag "${tags[j]}" '.spec.wechat.toTag |= .+ [$tag]')
        done
      fi
    fi

    echo "$resource" | jq >"${output_dir}/$(echo "$resource" | jq -r '.metadata.name').json"

  done
}

version=$(kubectl get crd notificationmanagers.notification.kubesphere.io -o jsonpath='{.spec.versions[0].name}')
if [ "${version}" = "v1alpha1" ]; then
  backup_v1alpha1
  update_v1alpha1
elif [ "${version}" = "v2alpha1" ]; then
  backup_v2alpha1
  update_v2alpha1
elif [ "${version}" = "v2beta1" ]; then
  echo "This is the latest version"
else
  echo "Unknown version"
fi
