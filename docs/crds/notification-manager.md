# NotificationManager CRD

## Overview

NotificationManager CRD defines the desired Notification Manager deployment. 
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

Properties of Notification Manager webhook deployment.
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

Parameters of the Notification Manager webhook.

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

`DefaultConfigSelector` is a label selector used to select the default config. The default configuration is as follows.

```yaml
  defaultConfigSelector:
    matchLabels:
      type: default
```

### DefaultSecretNamespace

`defaultSecretNamespace` is used to define the default namespace where the secret and configmap are located.For more information, please refer to [Credential](credential.md).

### Receivers 

The `receivers` supports the following fields:

- `tenantKey` -  key used to identify tenant. The default value is `namespace` if not specified.
- `globalReceiverSelector` - Selector to find global receivers which will be used when tenant receivers cannot be found. Only the matchLabels expression allowed.
- `tenantReceiverSelector` - Selector to find tenant receivers. Only the matchLabels expression allowed.
- `options` - Global options and options for all kind of receivers.

#### Options

##### Global options

- `templateFile` - Template file path, which must be an absolute path. This field is deprecated in v2.0.0 and will be removed in a future release.
- `template` - The name of the template that generates the notification.
- `cluster` - The name of the cluster where Notification Manager is deployed, and the default value is `default`, Notification Manager will add a cluster label to the notification using this value if the notification does not contain a cluster label. 
  If Notification Manager deployed in KubeSphere(v3.3+), it will try to get the cluster name automatically if the `cluster` is not set.
##### DingTalk options

- `notificationTimeout` - Timeout when sending notifications to DingTalk, and the default value is `3s`.
- `template` - The name of the template that generates the notification for all dingtalk receivers. For more information, please refer to [template](../template.md).
- `titleTemplate` - The name of the template that generates the markdown title.
- `tmplType` - The type of message sent to dingtalk, The value can be `text` or `markdown`.
- `tokenExpires` - The expiry time of the token, and the default value is `2h`.
- `conversationMessageMaxSize` - The maximum message size of a single request to a conversation, and the default value is `5000 bytes`.
- `chatbotMessageMaxSize` - The maximum message size of a single request to a chatbot, and the default value is `19960 bytes`.
- `chatBotThrottle` - The throttle is used to control how often notifications can be sent to the same conversation.
- `conversationThrottle` - The throttle is used to control how often notifications can be sent to the same chatbot.

A throttle supports the following fields:

- `threshold` - The maximum calls in one `unit`.
- `unit` - The time slot for flow control.
- `maxWaitTime` - The maximum waiting time that can be tolerated when notification calls trigger flow control. If the actual waiting time exceeds this time, 
  an error will be returned, otherwise waits for the traffic limit to be lifted. Nil means do not wait, and the maximum value is `unit`.

##### Email options

- `notificationTimeout` - Timeout when sending an email, and the default value is `3s`.
- `template` - The name of the template that generates the notification for all email receivers. For more information, please refer to [template](../template.md).
- `subjectTemplate` - The name of the template that generates email subject.
- `tmplType` - The type of the email content. The value can be `html` or `text`. 
- `maxEmailReceivers` - The maximum size of recipients in one email, and the default value is unlimited.

##### Feishu options

- `notificationTimeout` - Timeout when sending notifications to feishu, and the default value is `3s`.
- `template` - The name of the template that generates the notification for all feishu receivers. For more information, please refer to [template](../template.md).
- `tmplType` - The type of message sent to feishu. The value can be `text` or `post`. The `post` is Rich Text Format. For more information, please refer to [this](https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/im-v1/message/create_json#45e0953e)  for more information.
- `tokenExpires` - The expiry time of the token, and the default value is `2h`.

##### Pushover options

- `notificationTimeout` - Timeout when sending notifications to pushover, and the default value is `3s`.
- `template` - The name of the template that generates the notification for all pushover receivers. For more information, please refer to [template](../template.md).

##### Slack options

- `notificationTimeout` - Timeout when sending notifications to slack, and the default value is `3s`.
- `template` - The name of the template that generates the notification for all slack receivers. For more information, please refer to [template](../template.md).
- `titleTemplate` - The name of the template that generates message title.

##### SMS options

- `notificationTimeout` - Timeout when sending notifications to short message service, and the default value is `3s`.
- `template` - The name of the template that generates the notification for all sms receivers. For more information, please refer to [template](../template.md).

##### Webhook options

- `notificationTimeout` - Timeout when sending notifications to webhook, and the default value is `3s`.
- `template` - The name of the template that generates the notification for all webhook receivers. For more information, please refer to [template](../template.md).

##### WeChat options

- `notificationTimeout` - Timeout when sending notifications to WeChat, and the default value is `3s`.
- `template` - The name of the template that generates the notification for all WeChat receivers. For more information, please refer to [template](../template.md).
- `tmplType` - The type of message sent to WeChat. The value can be `text` or `markdown`. 
- `tokenExpires` - The expiry time of the token, and the default value is `2h`.
- `messageMaxSize` - The maximum message size that a single request can send, and the default value is `2048 bytes`.

### Args

`args` is the startup parameters of the Notification Manager webhook. It contains the following parameters.

- `--log.level` - The log level to use. Possible values are `debug`, `info`, `warn`, `error`, and the default value is `info`.
- `--log.format` - The log format to use, Possible values are `logfmt`, `json`, and the default value is `logfmt`.
- `--webhook.address` - The address to listen on for incoming data, and the default value is `19093`.
- `--webhook.timeout` - The timeout for each incoming request, and the default value is `3s`.
- `--worker.timeout` - Processing timeout for each batch data, and the default value is `30s`.
- `--worker.queue` -- Notification worker queue capacity, i.e., the maximum number of goroutines that process notifications.
- `--store.type` -- Type of store which is used to cache the data. Now it only supports `memory`.

### BatchMaxSize and BatchMaxWait

The incoming data will be pushed into the cache first, Notification Manager will batch out data from the cache and process them.
`batchMaxSize` defines the maximum number of data fetched from the cache at a time, `batchMaxWait` defines the maximum time to fetch data from the cache.

### RoutePolicy

There are two ways to determine to which receivers a notification will be sent: router or namespace matching.
The `routePolicy` determines the priority of these two ways. Valid RoutePolicy include All, RouterFirst, and RouterOnly.
and will also be sent to the receivers whose tenants have the right to access the namespace the notifications belong to.
- `All` - The notifications will be sent to the receivers that match any router, and will also be sent to the receivers whose tenants have the right to access the namespace the notifications belong to.
- `RouterFirst` - The notifications will be sent to the receivers that match any router first. If no receivers match any router, notifications will be sent to the receivers whose tenants have the right to access the namespace the notifications belong to.
- `RouterOnly` - The notifications will only be sent to the receivers that match any router.

### GroupLabels

`groupLabels` is used to group the notifications, and notifications with the same label will be sent together. By default, Notification Manager groups notifications with `alertname` and `namespace`.
If notifications grouping is not require, it can be set to nil.

### Template

`template` is used to set the template file and language package that Notification Manager used, and you can refer to [template](../template.md) for more information.

### Sidecars

`sidecars` will add sidecars to the Notification Manager pod.

```yaml
  sidecars:
    tenant:
      image: kubesphere/notification-tenant-sidecar:v3.2.0
      name: tenant
      type: kubesphere
```

The key `tenant` is the type of the sidecar which is the only supported sidecar type for now.

#### Tenant sidecar

A tenant sidecar is used to receive all users who have the right to access a namespace.
A tenant sidecar must provide `/api/v2/tenant?namespace=<namespace>` API on port 19094, Notification Manager calls this API to receive all users who have the right to access a namespace.
The request parameter is the namespace, the response body is a list of users.

A tenant sidecar supports the following fields:

- `type` - The type of the sidecar. Now it only supports `kubesphere`.
- `container` - A pod container, you can refer to [this](https://github.com/kubernetes/kubernetes/blob/master/staging/src/k8s.io/api/core/v1/types.go#L2290) for more information.

### History

`history` defines a webhook to receive history of all sent notifications. For more information, please refer to [webhook receiver](receiver.md#Webhook-receiver).

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
