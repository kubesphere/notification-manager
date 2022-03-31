# NotificationManager CRD

## Overview

NotificationManager CRD Defines the desired notification manager webhook deployment. 
The Notification Manager Operator ensures a deployment meeting the resource requirements is running.

```yaml
apiVersion: notification.kubesphere.io/v2beta2
kind: NotificationManager
metadata:
  name: notification-manager
spec:
  args:
    - --log.level=debug
  batchMaxSize: 100
  batchMaxWait: 6s
  defaultConfigSelector:
    matchLabels:
      type: default
  groupLabels:
    - alertname
    - namespace
    - pod
  image: kubesphere/notification-manager:latest
  imagePullPolicy: Always
  portName: webhook
  receivers:
    globalReceiverSelector:
      matchLabels:
        type: global
    options:
      dingtalk:
        notificationTimeout: 5
      email:
        notificationTimeout: 5
      global:
        cluster: host
      slack:
        notificationTimeout: 5
      webhook:
        notificationTimeout: 5
      wechat:
        notificationTimeout: 5
    tenantKey: user
    tenantReceiverSelector:
      matchLabels:
        type: tenant
  replicas: 1
  resources:
    limits:
      cpu: 500m
      memory: 1Gi
    requests:
      cpu: 100m
      memory: 20Mi
  routePolicy: All
  serviceAccountName: notification-manager-sa
  template:
    language: English
    reloadCycle: 1m
    text:
      name: notification-manager-template
      namespace: kubesphere-monitoring-system
```

A NotificationManager resource allows the user to define:

Properties of notification manager webhook deployment.
- `resources`
- `image`
- `imagePullPolicy`
- `replicas`
- `nodeSelector`
- `affinity`
- `tolerations`
- `ServiceAccountName`
- `portName`
- `volumes`
- `volumeMounts`

Properties of Receiver and Config.

- [defaultConfigSelector](#DefaultConfigSelector)
- [defaultSecretNamespace](#DefaultSecretNamespace)
- [receivers](#Receivers)

Parameters of the notification manager webhook.

- [args](#args)
- [batchMaxSize](#BatchMaxSize-and-BatchMaxWait)
- [batchMaxWait](#BatchMaxSize-and-BatchMaxWait)
- [routePolicy](#RoutePolicy)

Parameters for generating and organizing notifications.

- [groupLabels](#GroupLabels)
- [template](#Template)

Others

- [sidecars](#Sidecars)
- [history](#History)

## Configuring NotificationManager

### DefaultConfigSelector

`DefaultConfigSelector` is a label selector used to select the default config. Default is this

```yaml
  defaultConfigSelector:
    matchLabels:
      type: default
```

### DefaultSecretNamespace

`defaultSecretNamespace` used to define the default namespace where the secret and configmap located, more information ser [Credential](credential.md).

### Receivers 

The `receivers` supports the following fields:

- `tenantKey` -  key used to identify tenant, default to be "namespace" if not specified.
- `globalReceiverSelector` - Selector to find global receivers which will be used when tenant receivers cannot be found. Only matchLabels expression allowed.
- `tenantReceiverSelector` - Selector to find tenant receivers. Only matchLabels expression allowed.
- `options` - Global options and options for all kind of receiver.

#### Options

##### Global options

- `templateFile` - Template file path, must be an absolute path. This field deprecated in v2.0.0 and will be removed in a future release.
- `template` - The name of the template that generated the notification.
- `cluster` - The name of the cluster where the notification manager deployed, default is `default`, Notification Manager will add a cluster label to the notification using this value if the notification does not contain a cluster label. 
  If notification manager deployed in KubeSphere(v3.3+), notification manager will try to get the cluster name automatically if the `cluster` is not set.

##### DingTalk options

- `notificationTimeout` - Maximum time to send notifications to DingTalk, default is `3s`.
- `template` - The name of the template that generated the notification for all dingtalk receiver, more information see [template](../template.md).
- `titleTemplate` - The name of the template that generated markdown title.
- `tmplType` - The type of message sent to dingtalk, text or markdown.
- `tokenExpires` - The expiry time of the token, default is 2 hours.
- `conversationMessageMaxSize` - The maximum message size that a single request can send to a conversation, default is 5000 bytes.
- `chatbotMessageMaxSize` - The maximum message size that a single request can send to a chatbot, default is 19960 bytes.
- `chatBotThrottle` - The throttle used to control how often notification sent to the same conversation.
- `conversationThrottle` - The throttle used to control how often notification sent to the same chatbot.

A throttle supports the following fields:

- `threshold` - The maximum calls in a `unit`.
- `unit` - The time unit for flow control.
- `maxWaitTime` - The maximum waiting time that can be tolerated when calling trigger flow control. If the actual waiting time exceeds this time, it will
  return an error, otherwise waits for the traffic limit to be lifted. Nil means do not wait, the maximum value is `unit`.

##### Email options

- `notificationTimeout` - Maximum time to send an email, default is `3s`.
- `template` - The name of the template that generated the notification for all email receiver, more information see [template](../template.md).
- `subjectTemplate` - The name of the template that generated email subject.
- `tmplType` - The type of the email content, html or text. 
- `maxEmailReceivers` - The maximum size of recipients in one email, default unlimited.

##### Feishu options

- `notificationTimeout` - Maximum time to send notifications to feishu, default is `3s`.
- `template` - The name of the template that generated the notification for all feishu receiver, more information see [template](../template.md).
- `tmplType` - The type of message sent to feishu, text or post. The `post` is Rich Text Format, you can refer to [this](https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/im-v1/message/create_json#45e0953e)  for more information.
- `tokenExpires` - The expiry time of the token, default is 2 hours.

##### Pushover options

- `notificationTimeout` - Maximum time to send notifications to pushover, default is `3s`.
- `template` - The name of the template that generated the notification for all pushover receiver, more information see [template](../template.md).

##### Slack options

- `notificationTimeout` - Maximum time to send notifications to slack, default is `3s`.
- `template` - The name of the template that generated the notification for all slack receiver, more information see [template](../template.md).
- `titleTemplate` - The name of the template that generated message title.

##### SMS options

- `notificationTimeout` - Maximum time to send notifications to short message service, default is `3s`.
- `template` - The name of the template that generated the notification for all sms receiver, more information see [template](../template.md).

##### Webhook options

- `notificationTimeout` - Maximum time to send notifications to webhook, default is `3s`.
- `template` - The name of the template that generated the notification for all webhook receiver, more information see [template](../template.md).

##### WeChat options

- `notificationTimeout` - Maximum time to send notifications to WeChat, default is `3s`.
- `template` - The name of the template that generated the notification for all WeChat receiver, more information see [template](../template.md).
- `tmplType` - The type of message sent to WeChat, text or markdown. 
- `tokenExpires` - The expiry time of the token, default is 2 hours.
- `messageMaxSize` - The maximum message size that a single request can send, default is 2048 bytes.

### Args

`args` is the startup parameters of the notification manager webhook, contains the following parameters.

- `--log.level` - The log level to use. Possible values are `debug`, `info`, `warn`, `error`, default is `info`.
- `--log.format` - The log format to use, Possible values are `logfmt`, `json`, efault is `logfmt`.
- `--webhook.address` - The address to listen on for incoming data, default is `19093`.
- `--webhook.timeout` - The timeout for each incoming request, default is `3s`.
- `--worker.timeout` - Processing timeout for each batch data, default is `30s`.
- `--worker.queue` -- Notification worker queue capacity, i.e., the maximum number of goroutines that process notifications.
- `--store.type` -- Type of store which used to cache the data, now only support `memory`.

### BatchMaxSize and BatchMaxWait

The incoming data will be pushed into the cache first, notification manager will batch out data from the cache and process them.
`batchMaxSize` defines the maximum number of data fetched from the cache at a time, `batchMaxWait` defines the maximum time to fetch data from the cache.

### RoutePolicy

There are two ways to determine which receivers the notifications will send to, router, or via namespace matching.
The `routePolicy` determines the priority of these two ways. Valid RoutePolicy include All, RouterFirst, and RouterOnly.
- `All` - The notifications will be sent to the receivers that match any router, and also send to the receivers of those tenants with the right to access the namespace to which the notifications belong.
- `RouterFirst` - The notifications will be sent to the receivers that match any router first. If no receivers match any router, notifications will send to the receivers of those tenants with the right to access the namespace to which the notifications belong.
- `RouterOnly` - The notifications will only be sent to the receivers that match any router.

### GroupLabels

`groupLabels` used to group the notifications, notifications with the same label will send together, by default, notification manager group notifications with `alertname` and `namespace`.
If notifications grouping does not require, it can be set to nil.

### Template

`template` used to set the template file and language package that notification manager used, you can refer to [this](../template.md) for more information.

### Sidecars

`sidecars` will add sidecars to the notification manager webhook pod.

```yaml
  sidecars:
    tenant:
      image: kubesphere/notification-tenant-sidecar:v3.2.0
      name: tenant
      type: kubesphere
```

The key `tenant` is the type of the sidecar, now it only supports tenant sidecar.

#### Tenant sidecar

A tenant sidecar used to get all user who has the right to access a namespace.
A tenant sidecar must provide `/api/v2/tenant?namespace=<namespace>` API on port 19094, notification manager calls this API to get all user who has the right to access a namespace.
The request parameter is the namespace, the response body is a list of user.

A tenant sidecar supports the following fields:

- `type` - The type of the sidecar, support value is `kubesphere`.
- `container` - A pod container, you can refer to [this](https://github.com/kubernetes/kubernetes/blob/master/staging/src/k8s.io/api/core/v1/types.go#L2290) for more information.

### History

`history` defines a webhook to receive all sent notifications as notification history, more information see [webhook receiver](receiver.md#Webhook-receiver)

```yaml
  history:
    webhook:
      enabled: true
      service:
        name: notification-adapter
        namespace: kubesphere-monitoring-system
        path: alerts
        port: 8080
```
