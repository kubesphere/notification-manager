# Receiver

## Overview

`Receiver` is used to define the notification format and destinations to which notifications will be sent.
`Receiver` can be categorized into 2 types `global` and `tenant` by label like `type = global`, `type = tenant` :
- A global receiver receives all notifications and then send notifications regardless tenant info(user or namespace).
- A tenant receiver only receives notifications from the namespaces that the tenant has access to.

A receiver resource allows the user to define:

- [dingtalk](#DingTalk-Receiver)
- [email](#Email-Receiver)
- [feishu](#Feishu-Receiver)
- [pushover](#Pushover-Receiver)
- [slack](#Slack-Receiver)
- [sms](#SMS-Receiver)
- [webhook](#Webhook-Receiver)
- [wechat](#WeChat-Receiver)
- [discord](#Discord-Receiver)

## DingTalk Receiver

A dingtalk receiver is like this

```yaml
apiVersion: notification.kubesphere.io/v2beta2
kind: Receiver
metadata:
  name: global-receiver
  labels:
    type: tenant
    user: admin
spec:
  dingtalk:
    alertSelector:
      matchExpressions:
        - key: namespace
          operator: DoesNotExist
    conversation:
      chatids:
        - chat894f9f4d634eb283933af6c7102977b2
    chatbot:
      webhook:
        valueFrom:
          secretKeyRef:
            key: webhook
            name: global-receiver-secret
            namespace: kubesphere-monitoring-system
      keywords:
        - kubesphere
      secret:
        valueFrom:
          secretKeyRef:
            key: secret
            name: global-receiver-secret
            namespace: kubesphere-monitoring-system  
    dingtalkConfigSelector:
      matchLabels:
        type: tenant
        user: admin
    enabled: true
    template: nm.default.text
    titleTemplate: nm.default.subject
    tmplType: text
    tmplText:
      name: notification-manager-template
      namespace: kubesphere-monitoring-system
```

A dingtalk receiver allows the user to define:

- `alertSelector` - The label selector used to filter notifications. For more information, you can refer to [notification filter](#Notification-filter).
- `conversation.chatid` - The id of dingtalk conversation. For more information, you can refer to [this](https://open.dingtalk.com/document/orgapp-server/create-group-session).
- [chatbot](#Chatbot) - The configuration of dingtalk chatbot.
- `dingtalkConfigSelector` - The label selector used to get `Config`. For more information, you can refer to [this](#How-to-select-config).
- `enabled` - Whether to enable receiver.
- `template` - The name of the template that generated notifications. For more information, you can refer to [template](../template.md).
- `titleTemplate` - The name of the template that generated markdown message title.
- `tmplText` - The configmap that the template text file be in. For more information, you can refer to [template](../template.md).
- `tmplType` - The type of the message send to the dingtalk, `text` or `markdown`, default type is `text`.

### Chatbot

The dingtalk chatbot is a webhook that receives messages and forwards them to the conversation. For more information, you can refer to [this](https://open.dingtalk.com/document/group/custom-robot-access).

A chatbot allows the user to define:

- `webhook` - The webhook url of chatbot, and `type` is [credential](./credential.md).
- `keywords` - The keywords of the chatbot, and the notifications send to the chatbot must include one of the keywords.
- `secret` - Secret of ChatBot, you can get it after enabling Additional Signature of ChatBot, and `type` is [credential](./credential.md).

## Email Receiver

An email receiver is like this.

```yaml
apiVersion: notification.kubesphere.io/v2beta2
kind: Receiver
metadata:
  name: global-receiver
  labels:
    type: tenant
    user: test
spec:
  email:
    alertSelector:
      matchExpressions:
      - key: namespace
        operator: DoesNotExist
    emailConfigSelector:
      matchLabels:
        type: tenant
        user: test
    enabled: true
    template: nm.default.html
    subjectTemplate: nm.default.subject
    tmplType: html
    tmplText:
      name: notification-manager-template
      namespace: kubesphere-monitoring-system
    to:
    - test@kubesphere.io
```

An email receiver allows the user to define:

- `alertSelector` - The label selector used to filter notifications. For more information, please refer to [notification filter](#Notification-filter).
- `emailConfigSelector` - The label selector used to get `Config`. For more information, please refer to [this](#How-to-select-config).
- `enabled` - Whether to enable receiver.
- `template` - The name of the template that generated notifications. For more information, please refer to [template](../template.md).
- `subjectTemplate` - The name of the template that generated email subject.
- `tmplText` - The configmap that the template text file be in. For more information, please refer to [template](../template.md). 
- `tmplType` - The type of the email content, `html` or `text`, default type is `html`.
- `to` - Who will receiver the email.

## Feishu Receiver

A feishu receiver is like this

```yaml
apiVersion: notification.kubesphere.io/v2beta2
kind: Receiver
metadata:
  name: global-receiver
  labels:
    type: tenant
    user: admin
spec:
  feishu:
    alertSelector:
      matchExpressions:
        - key: namespace
          operator: DoesNotExist
    chatbot:
      webhook:
        valueFrom:
          secretKeyRef:
            key: webhook
            name: global-receiver-secret
            namespace: kubesphere-monitoring-system
      keywords:
        - kubesphere
      secret:
        valueFrom:
          secretKeyRef:
            key: secret
            name: global-receiver-secret
            namespace: kubesphere-monitoring-system  
    department:
      - dev
    feishuConfigSelector:
      matchLabels:
        type: tenant
        user: admin
    enabled: true
    template: nm.default.post
    tmplType: text
    tmplText:
      name: notification-manager-template
      namespace: kubesphere-monitoring-system
    user:
      - test
```
A feishu receiver allows the user to define:

- `alertSelector` - The label selector used to filter notifications. For more information, please refer to [notification filter](#Notification-filter).
- [chatbot](#Feishu-Chatbot) - The configuration of feishu chatbot.
- `department` - The department of feishu, all the users in the department will receive the notifications. Note that the notification to the department are sent asynchronously, there will be a delay.
- `enabled` - Whether to enable receiver.
- `feishuConfigSelector` - The label selector used to get `Config`. For more information, please refer to [this](#How-to-select-config).
- `template` - The name of the template that generated notifications. For more information, please refer to [template](../template.md).
- `tmplText` - The configmap that the template text file be in. For more information, please refer to [template](../template.md).
- `tmplType` - The type of message sent to feishu, `post` or `text`, default type is `post`.
- `user` - Who will receiver notifications. Note that the notifications to the user sent asynchronously, there will be a delay.

### Feishu Chatbot

The feishu chatbot is a webhook that receives messages and forwards them to the conversation, you can refer to [this](https://open.feishu.cn/document/ukTMukTMukTM/ucTM5YjL3ETO24yNxkjN) for more information.

A chatbot allows the user to define:

- `webhook` - The webhook url of chatbot, and `type` is [credential](./credential.md).
- `keywords` - The keywords of the chatbot, the notifications sent to the chatbot must include one of the keywords.
- `secret` - Secret of ChatBot, you can get it after enabled Additional Signature of ChatBot, and `type` is [credential](./credential.md).

## Pushover Receiver

A pushover receiver is like this.

```yaml
apiVersion: notification.kubesphere.io/v2beta2
kind: Receiver
metadata:
  name: global-receiver
  labels:
    type: tenant
    user: admin
spec:
  pushover:
    alertSelector:
      matchExpressions:
      - key: namespace
        operator: DoesNotExist
    pushoverConfigSelector:
      matchLabels:
        type: tenant
        user: admin
    enabled: true
    profiles:
      - userKey: uQiRzpo4DXghDmr9QzzfQu27cmVRsG
        devices:
          - droid2
        title: "notification"
        sound: bike
    template: nm.default.text
    titleTemplate: nm.default.subject
    tmplText:
      name: notification-manager-template
      namespace: kubesphere-monitoring-system
```

A pushover receiver allows the user to define:

- `alertSelector` - The label selector used to filter notifications. For more information, please refer to [notification filter](#Notification-filter).
- `enabled` - Whether to enable receiver.
- [profiles](#User-Profile) - The profile of users who will receive the notifications.
- `pushoverConfigSelector` - The label selector used to get `Config`. For more information, please refer to [this](#How-to-select-config).
- `template` - The name of the template that generated notifications. For more information, please refer to [template](../template.md).
- `titleTemplate` - The name of the template that generated message title.
- `tmplText` - The configmap that the template text file be in. For more information, please refer to [template](../template.md).

### User Profile 

A profile allows the user to define:

- `userKey` - Who will receive the notifications. `userKey`  is 30 characters long, case-sensitive, and may contain the character set [A-Za-z0-9].
- `devices` - The name of some of your devices to send. Device names are optional, may be up to 25 characters long, and will contain the character set [A-Za-z0-9_-]. 
  If device name does not specify, or the specified device name is no longer enabled/valid, notifications will be sent to all active devices for that user to avoid losing messages
- `sound` - The name of a supported sound to override your default sound choice, more information see [sounds](https://pushover.net/api#sounds).

## Slack Receiver

A slack receiver is like this.

```yaml
apiVersion: notification.kubesphere.io/v2beta2
kind: Receiver
metadata:
  name: global-receiver
  labels:
    type: tenant
    user: admin
spec:
  slack:
    alertSelector:
      matchExpressions:
      - key: namespace
        operator: DoesNotExist
    slackConfigSelector:
      matchLabels:
        type: tenant
        user: admin
    enabled: true
    template: nm.default.text
    tmplText:
      name: notification-manager-template
      namespace: kubesphere-monitoring-system
    channels:
      - dev
      - test
```

A slack receiver allows the user to define:

- `alertSelector` - The label selector used to filter notifications. For more information, please refer to [notification filter](#Notification-filter).
- `enabled` - Whether to enable receiver.
- `slackConfigSelector` - The label selector used to get `Config`. For more information, please refer to [this](#How-to-select-config).
- `template` - The name of the template that generated notifications. For more information, please refer to [template](../template.md).
- `tmplText` - The configmap that the template text file be in. For more information, please refer to [template](../template.md).
- `channels` - Channels that the notification will send to.

## SMS Receiver

An SMS receiver is like this.

```yaml
apiVersion: notification.kubesphere.io/v2beta2
kind: Receiver
metadata:
  name: global-receiver
  labels:
    type: tenant
    user: admin
spec:
  sms:
    alertSelector:
      matchExpressions:
      - key: namespace
        operator: DoesNotExist
    smsConfigSelector:
      matchLabels:
        type: tenant
        user: admin
    enabled: true
    template: nm.default.text
    tmplText:
      name: notification-manager-template
      namespace: kubesphere-monitoring-system
    phoneNumbers:
      - 13612345678
```

An SMS receiver allows the user to define:

- `alertSelector` - The label selector used to filter notifications. For more information, please refer to [notification filter](#Notification-filter).
- `enabled` - Whether to enable receiver.
- `smsConfigSelector` - The label selector used to get `Config`. For more information, please refer to [this](#How-to-select-config).
- `template` - The name of the template that generated notifications. For more information, please refer to [template](../template.md).
- `tmplText` - The configmap that the template text file be in. For more information, please refer to [template](../template.md).
- `phoneNumbers` - PhoneNumbers that the notification will send to.

## Webhook Receiver

A webhook receiver is like this

```yaml
apiVersion: notification.kubesphere.io/v2beta2
kind: Receiver
metadata:
  name: global-receiver
  labels:
    type: tenant
    user: admin
spec:
  webhook:
    alertSelector:
      matchExpressions:
        - key: namespace
          operator: DoesNotExist
    enabled: true
    template: webhook.default.message
    tmplText:
      name: notification-manager-template
      namespace: kubesphere-monitoring-system
    url: "https://192.168.0.2:443/notifications"
    service: []
    httpConfig: []
```
A webhook receiver allows the user to define:

- `alertSelector` - The label selector used to filter notifications. For more information, please refer to [notification filter](#Notification-filter).
- `enabled` - Whether to enable receiver.
- `template` - The name of the template that generated notifications. For more information, please refer to [template](../template.md).
- `tmplText` - The configmap that the template text file be in. For more information, please refer to [template](../template.md).
- `url` - Url of the webhook.
- [service](#Service) - `service` is a reference to the service for this webhook. Either `service` or `url` must be specified. If the webhook is running within the cluster, then you should use `service`.
- [httpConfig](#HttpConfig) - The http/s client configuration.

> A webhook receiver has no need a config.
> The default template for webhook is `webhook.default.message`, which is a special template that serializes alert to json as notification message

### Service

`Service` allows the user to define:

- `namespace` - The namespace of the service.
- `name` - The name of the service.
- `port` - The port on the service that hosting webhook, it should be a valid port number (1-65535, inclusive).
- `path` - An optional URL path to which the request will be sent.
- `scheme` - Http scheme. The default value is http.

### HttpConfig

`HttpConfig` allows the user to define:

- `basicAuth.username` - The username for basic auth.
- `basicAuth.password` - Password for basic auth, and `type` is [credential](./credential.md).
- `bearerToken` - Token for bearer authentication, and `type` is [credential](./credential.md).
- `proxyUrl` - HTTP proxy server used to connect to the targets.  
- [tlsConfig](#TlsConfig) - TLS configuration to use to connect to the targets, it needs to be set when the scheme is https.

#### TlsConfig

`tlsConfig` allows the user to define:

- `rootCA` - RootCA defines the root certificate authorities that clients use when verifying server certificates, and `type` is [credential](./credential.md).
- `clientCertificate.cert` - The client cert file for the targets, and `type` is [credential](./credential.md).
- `clientCertificate.key` - The client key file for the targets, and `type` is [credential](./credential.md).
- `serverName` - Used to verify the hostname for the targets.
- `insecureSkipVerify` - Disable target certificate validation.

#### 

## WeChat Receiver

A WeChat receiver is like this

```yaml
apiVersion: notification.kubesphere.io/v2beta2
kind: Receiver
metadata:
  name: global-receiver
  labels:
    type: tenant
    user: admin
spec:
  wechat:
    alertSelector:
      matchExpressions:
        - key: namespace
          operator: DoesNotExist
    wechatConfigSelector:
      matchLabels:
        type: tenant
        user: admin
    enabled: true
    template: nm.default.text
    tmplType: text
    tmplText:
      name: notification-manager-template
      namespace: kubesphere-monitoring-system
    toUser:
      - user1
    toParty:
      - "1"
    toTag:
      - "2"
```

A WeChat receiver allows the user to define:

- `alertSelector` - The label selector used to filter notifications, more information see [notification filter](#Notification-filter).
- `wechatkConfigSelector` - The label selector used to get `Config`, more information see [this](#How-to-select-config).
- `enabled` - Whether to enable receiver.
- `template` - The name of the template that generated notifications, more information see [template](../template.md).
- `tmplText` - The configmap that the template text file be in, more information see [template](../template.md).
- `tmplType` - The type of the message send to the WeChat, text or markdown, default type is text.
- `toUser` - The id of users who will receive the notifications.
- `toPart` - The id of the party, all users in the party will receive notifications.
- `toTag` - The id of the tag, all users who have this tag will receive notifications.

### WeChat Chatbot

The wechat chatbot is a webhook that receives messages and forwards them to the conversation.
> Note: When using the markdown type, the WeChat robot only supports @userid.

A chatbot allows the user to define:

- `webhook` - The webhook url of chatbot, and `type` is [credential](./credential.md).

```yaml
apiVersion: notification.kubesphere.io/v2beta2
kind: Receiver
metadata:
  name: test-wechat-receiver
  labels:
    type: global
spec:
  wechat:
    enabled: true
    template: nm.default.text
    tmplType: text
    tmplText:
      name: notification-manager-template
      namespace: kubesphere-monitoring-system
    chatbot:
      atMobiles:
        - "13455431234"
      atUsers:
        - "@all"
        - "userid"
      webhook:
        valueFrom:
          secretKeyRef:
            key: test
            name: wechat-bot-secret
            namespace: kubesphere-monitoring-federated
---
kind: Secret
apiVersion: v1
metadata:
  name: wechat-bot-secret
  namespace: kubesphere-monitoring-federated
  labels:
    notification.kubesphere.io/managed: 'true'
    type: global
  annotations:
    kubesphere.io/creator: admin
data:
  test: aHR0cHfgbftyjiuyfdfgnzZW5kP2tleT05ZWY5ZDAyZC0xOTcwLTRhM2ItOTY5Ni1hMWIwOGUxOTdlMzc=
```

### Discord Receiver

Discord receiver allows the user to define:

- `webhook` - The webhook url of channel, and `type` is [credential](./credential.md).
- `mentionedUsers` - Users who need to be mentioned.
- `mentionedRoles` - Roles that need to be mentioned.

```yaml
apiVersion: notification.kubesphere.io/v2beta2
kind: Receiver
metadata:
  name: test-discord-receiver
  labels:
    type: global
spec:
  discord:
    enabled: true
    template: nm.default.text
    type: embed   # content or embed
    tmplText:
      name: notification-manager-template
      namespace: kubesphere-monitoring-system
    enabled: true
    mentionedUsers:
      - everyone
      - "1045280620097572914"
    mentionedRoles:
      - "1057234958281887744"
    webhook:
      valueFrom:
        secretKeyRef:
          key: webhook
          name: discord-secret
          namespace: kubesphere-monitoring-federated
---
kind: Secret
apiVersion: v1
metadata:
  name: discord-secret
  namespace: kubesphere-monitoring-federated
  labels:
    notification.kubesphere.io/managed: 'true'
    type: global
  annotations:
    kubesphere.io/creator: admin
data:
  webhook: aHR0cHM6Ly9kaXNjETRWERViaG9va3MvMTA0NTMxNTk5MzQ2NTAxMjI0NC9KYjBvRk9qVnpkUldsSGZURWROWFZJNXBDbUxKN1NtVUJtYkpOcEhSTFFGTl9YSXhQbW1xMWNrVTlEZlM1N1NuT0VKSg==
```

## How to select config

The `Receiver` defines where to send notifications, and the [Config](config.md) define how to send notifications to receiver.
A `Receiver` selects a `Config` by `xxxConfigSelector`.

The relationship between receivers and configs can be demonstrated as below:

![Receivers & Configs](../images/receivers_configs.png)

For a tenant receiver, it will use the config selector to select config if config selector has been set, else it will try to find a `default` config.
For a global receiver, it can only use default config.

## Notification filter

A receiver can only receive the notifications it needs by setting `alertSelector`. The `alertSelector` is label selector,
the receiver will only receive the notifications that match the selector.

An email receiver only receives "warning" notifications.

```yaml
email:
  alertSelector:
    matchExpressions:
      - key: severity
        operator: In
        values:
          - error
          - critical
```

An email receiver only receives cluster-level notifications.

```yaml
email:
  alertSelector:
    matchExpressions:
      - key: namespace
        operator: DoesNotExist
```