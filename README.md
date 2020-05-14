# Notification Manager
Notification Manager manages notifications in multi-tenant K8s environment. It receives alerts or notifications from different senders and then send notifications to various tenant receivers based on alerts/notifications' tenant label like "namespace". 

Supported senders includes:
- Prometheus Alertmanager
- Custom sender (Coming soon)

Supported receivers includes:
- Email
- [Wechat Work](https://work.weixin.qq.com/)
- Slack (Coming soon)
- Webhook (Coming soon)

## CustomResourceDefinitions
Notification Manager uses the following CRDs to define the desired alerts/notifications webhook and receiver configs.
- NotificationManager: Defines the desired alerts/notification webhook deployment. The Notification Manager Operator ensures a deployment meeting the resource requirements is running.
- EmailConfig: Defines the email configs like SmartHost, AuthUserName, AuthPassword, From, RequireTLS etc. There are also global options like NotificationTimeout, DeliveryType, MaxEmailReceivers to define email configs.
- EmailReceiver: Define email receiver's mail addresses and the EmailConfig selector.
- WechatConfig: Define the wechat configs like ApiUrl, ApiCorpId, AgentId and ApiSecret. There are also global options like NotificationTimeout to define email configs.
- WechatReceiver: Define the wechat receiver related info like ToUser, ToParty, ToTag as well as WechatConfig Selector.
- SlackConfig: Define the slack configs like ApiUrl.
- SlackReceiver: Define the slack channel or user to send notifications to and the SlackConfig selector.
- WebhookConfig: Define the webhook Url, HttpConfig.
- WebhookReceiver: Define the WebhookConfig selector.

Receiver CRDs like EmailReceiver, WechatReceiver, SlackReceiver and WebhookReceiver can be categorized into 2 types `global` and `tenant` by label like `type = global`, `type = tenant` .
- A global EmailReceiver receives all alerts and then send notifications regardless tenant info(user or namespace).
- A tenant EmailReceiver receives alerts with specified tenant label like `user` or `namespace` 

Usually alerts received from Alertmanager contains a `namespace` label, Notification Manager uses this label to decide which receiver to use for sending notifications. 
- For KubeSphere, Notification Manager will try to find workspace `user` in that `namespace`'s rolebinding and then find receivers with `user = xxx` label.
- For other Kubernetes cluster, Notification Manager will try to find receivers with `namespace = xxx` label. 

For alerts without a `namespace` label, for example alerts of node or kubelet, user can setup a receiver with `type = global` label to receive alerts without a `namespace` label. A global receiver sends notifications for all alerts received regardless any label. A global receiver is usually set for a admin role.

Config CRDs like EmailConfig, WechatConfig, SlackConfig, WebhookConfig can be categorized into 3 types `global`, `tenant` and `default` by label like `type = global`, `type = tenant`, `type = default`. 
- Global EmailConfig is to be selected by a Global EmailReceiver. 
- Tenant EmailConfig is to be selected by a tenant EmailReceiver which means each tenant can have his own EmailConfig. 
- If no EmailConfig selector is configured in a EmailReceiver, then this EmailReceiver will try to find a `default` EmailConfig. Usually admin will set a global default config.

## QuickStart

Deploy CRDs and the Notification Manager Operator:

```shell
kubectl apply -f config/bundle.yaml
```

### Deploy Notification Manager in KubeSphere (Uses `workspace` to distinguish each tenant user):

Deploy Notification Manager
```shell
cat <<EOF | kubectl apply -f -
apiVersion: notification.kubesphere.io/v1alpha1
kind: NotificationManager
metadata:
  name: notificationmanager-sample
spec:
  replicas: 1
  resources:
    limits:
      cpu: 500m
      memory: 1Gi
    requests:
      cpu: 100m
      memory: 20Mi
  image: kubesphere/notification-manager:latest
  imagePullPolicy: Always
  serviceAccountName: notification-manager-sa
  portName: webhook
  globalConfigSelector:
    matchLabels:
      type: default
  receivers:
    tenantKey: user
    globalReceiverSelector:
      matchLabels:
        type: global
    tenantReceiverSelector:
      matchLabels:
        type: tenant
    options:
      email:
        notificationTimeout: 5
        deliveryType: bulk
        maxEmailReceivers: 200
      wechat:
        notificationTimeout: 5
EOF
```

Deploy EmailConfig and EmailReceivers
```
cat <<EOF | kubectl apply -f -
apiVersion: notification.kubesphere.io/v1alpha1
kind: EmailConfig
metadata:
  labels:
    app: notification-manager
    type: tenant
    user: admin
  name: admin-email-config
  namespace: kubesphere-monitoring-system
spec:
  authPassword:
    key: password
    name: global-email-secret
    namespace: kubesphere-monitoring-system
  authUsername: abc1
  from: abc1@xyz.com
  requireTLS: true
  smartHost:
    host: imap.xyz.com
    port: "25"
---
apiVersion: notification.kubesphere.io/v1alpha1
kind: EmailReceiver
metadata:
  labels:
    app: notification-manager
    type: tenant
    user: admin
  name: admin-email
  namespace: kubesphere-monitoring-system
spec:
  emailConfigSelector:
    matchLabels:
      type: tenant
      user: admin
  to:
  - abc2@xyz.com
  - abc3@xyz.com
---
apiVersion: v1
data:
  password: dGVzdA==
kind: Secret
metadata:
  labels:
    app: notification-manager
  name: global-email-secret
  namespace: kubesphere-monitoring-system
type: Opaque
EOF
```
Deploy WechatConfig and WechatReceivers

```
cat <<EOF | kubectl apply -f -
apiVersion: notification.kubesphere.io/v1alpha1
kind: WechatConfig
metadata:
  name: admin-wechat-config
  namespace: kubesphere-monitoring-system
  labels:
    app: notification-manager
    type: tenant
    user: admin
spec:
  wechatApiUrl: < wechat-api-url >
  wechatApiSecret:
    key: wechat
    name: < wechat-api-secret >
  wechatApiCorpId: < wechat-api-corp-id >
  wechatApiAgentId: < wechat-api-agent-id >
---
apiVersion: notification.kubesphere.io/v1alpha1
kind: WechatReceiver
metadata:
  name: admin-wechat
  namespace: kubesphere-monitoring-system
  labels:
    app: notification-manager
    type: tenant
    user: admin
spec:
  wechatConfigSelector:
    matchLabels:
      type: tenant
      user: admin
  toUser: < wechat-user >
  toParty: < wechat-party >
  toTag: < wechat-tag >
---
apiVersion: v1
data:
  wechat: dGVzdA==
kind: Secret
metadata:
  labels:
    app: notification-manager
  name: < wechat-api-secret >
  namespace: kubesphere-monitoring-system
type: Opaque
EOF
```

>WechatApiAgentId is the id of app which sending message to user in your Wechat Work, wechatApiSecret is the secret of this app, you can get these two parameters in App Managerment of your Wechat Work. Note that any user, party or tag who wants to rerceive notifications must be in the allowed users list of this app.

### Deploy Notification Manager in any other Kubernetes cluster (Uses `namespace` to distinguish each tenant user):
Deploy Notification Manager
```shell
cat <<EOF | kubectl apply -f -
apiVersion: notification.kubesphere.io/v1alpha1
kind: NotificationManager
metadata:
  name: notificationmanager-sample
spec:
  replicas: 1
  resources:
    limits:
      cpu: 500m
      memory: 1Gi
    requests:
      cpu: 100m
      memory: 20Mi
  image: kubesphere/notification-manager:latest
  imagePullPolicy: Always
  serviceAccountName: notification-manager-sa
  portName: webhook
  globalConfigSelector:
    matchLabels:
      type: default
  receivers:
    tenantKey: namespace
    globalReceiverSelector:
      matchLabels:
        type: global
    tenantReceiverSelector:
      matchLabels:
        type: tenant
    options:
      notificationTimeout:
        email: 5
EOF
```

Deploy EmailConfig and EmailReceivers
```
cat <<EOF | kubectl apply -f -
apiVersion: notification.kubesphere.io/v1alpha1
kind: EmailConfig
metadata:
  labels:
    app: notification-manager
    type: tenant
    namespace: default
  name: admin-email-config
  namespace: default
spec:
  authPassword:
    key: password
    name: global-email-secret
    namespace: default
  authUsername: abc1
  from: abc1@xyz.com
  requireTLS: true
  smartHost:
    host: imap.xyz.com
    port: "25"
---
apiVersion: notification.kubesphere.io/v1alpha1
kind: EmailReceiver
metadata:
  labels:
    app: notification-manager
    type: tenant
    namespace: default
  name: admin-email
  namespace: default
spec:
  emailConfigSelector:
    matchLabels:
      type: tenant
      namespace: default
  to:
  - abc2@xyz.com
  - abc3@xyz.com
---
apiVersion: v1
data:
  password: dGVzdA==
kind: Secret
metadata:
  labels:
    app: notification-manager
  name: global-email-secret
  namespace: default
type: Opaque
EOF
```
Deploy WechatConfig and WechatReceivers

```
cat <<EOF | kubectl apply -f -
apiVersion: notification.kubesphere.io/v1alpha1
kind: WechatConfig
metadata:
  name: admin-wechat-config
  namespace: default
  labels:
    app: notification-manager
    type: tenant
    namespace: default
spec:
  wechatApiUrl: < wechat-api-url >
  wechatApiSecret:
    key: wechat
    name: < wechat-api-secret >
  wechatApiCorpId: < wechat-api-corp-id >
  wechatApiAgentId: < wechat-api-agent-id >
---
apiVersion: notification.kubesphere.io/v1alpha1
kind: WechatReceiver
metadata:
  name: admin-wechat
  namespace: default
  labels:
    app: notification-manager
    type: tenant
    namespace: default
spec:
  wechatConfigSelector:
    matchLabels:
      type: tenant
      namespace: default
  toUser: < wechat-user >
  toParty: < wechat-party >
  toTag: < wechat-tag >
---
apiVersion: v1
data:
  wechat: dGVzdA==
kind: Secret
metadata:
  labels:
    app: notification-manager
  name: < wechat-api-secret >
  namespace: default
type: Opaque
EOF
```

### Config Prometheus Alertmanager to send alerts to Notification Manager
Notification Manager use port `19093` and API path `/api/v2/alerts` to receive alerts sending from Prometheus Alertmanager.
To receive Alertmanager alerts, add webhook config like below to the `receivers` section of Alertmanager configuration file:

```shell
    "receivers":
     - "name": "notification-manager"
       "webhook_configs":
       - "url": "http://notificationmanager-sample-svc.kubesphere-monitoring-system.svc:19093/api/v2/alerts"
```

## Development

```
# Build notification-manager-operator and notification-manager docker images
make build 
# Push built docker images to docker registry
make push
```
