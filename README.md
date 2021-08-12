# Notification Manager
Notification Manager manages notifications in multi-tenant K8s environment. It receives alerts or notifications from different senders and then send notifications to various tenant receivers based on alerts/notifications' tenant label like "namespace". 

Supported senders includes:
- Prometheus Alertmanager
- Custom sender (Coming soon)

Supported receivers includes:
- Email
- [WeCom](https://work.weixin.qq.com/)
- Slack 
- Webhook 
- DingTalk

## Architecture
Notification Manager uses CRDs to store notification configs like email, WeCom and slack. It also includes an operator to create and reconcile NotificationManager CRD which watches all notification config CRDs, updates notification settings accordingly and sends notifications to users.

![Architecture](docs/images/architecture.png)

## Integration with Alertmanager
Notification Manager could receive webhook notifications from Alertmanager and then send notifications to users in a multi-tenancy way.

![Notification Manager](docs/images/notification-manager.png)

### Config Alertmanager to send alerts to Notification Manager
Notification Manager uses port `19093` and API path `/api/v2/alerts` to receive alerts sent from Alertmanager.
To receive Alertmanager alerts, add webhook config like below to the `receivers` section of Alertmanager configuration file:

```shell
    "receivers":
     - "name": "notification-manager"
       "webhook_configs":
       - "url": "http://notification-manager-svc.kubesphere-monitoring-system.svc:19093/api/v2/alerts"
```

## CustomResourceDefinitions
Notification Manager uses the following CRDs to define the desired alerts/notifications webhook and receiver configs:
- NotificationManager: Defines the desired alerts/notification webhook deployment. The Notification Manager Operator ensures a deployment meeting the resource requirements is running.
- Config: Defines the dingtalk, email, slack, webhook, wechat configs. 
- Receiver: Define dingtalk, email, slack, webhook, wechat receivers.

The relationship between receivers and configs can be demonstrated as below:

![Receivers & Configs](docs/images/receivers_configs.png)

Receiver CRDs like EmailReceiver, WechatReceiver, SlackReceiver and WebhookReceiver can be categorized into 2 types `global` and `tenant` by label like `type = global`, `type = tenant` :
- A global EmailReceiver receives all alerts and then send notifications regardless tenant info(user or namespace).
- A tenant EmailReceiver receives alerts with specified tenant label like `user` or `namespace` 

Usually alerts received from Alertmanager contains a `namespace` label, Notification Manager uses this label to decide which receiver to use for sending notifications:
- For KubeSphere, Notification Manager will try to find workspace `user` in that `namespace`'s rolebinding and then find receivers with `user = xxx` label.
- For other Kubernetes cluster, Notification Manager will try to find receivers with `namespace = xxx` label. 

For alerts without a `namespace` label, for example alerts of node or kubelet, user can set up a receiver with `type = global` label to receive alerts without a `namespace` label. A global receiver sends notifications for all alerts received regardless any label. A global receiver is usually set for an admin role.

Config CRDs can be categorized into 2 types `tenant` and `default` by label like `type = tenant`, `type = default`:
- Tenant EmailConfig is to be selected by a tenant EmailReceiver which means each tenant can have his own EmailConfig. 
- If no EmailConfig selector is configured in a EmailReceiver, then this EmailReceiver will try to find a `default` EmailConfig. Usually admin will set a global default config.

A receiver could be configured without xxxConfigSelector, in which case Notification Manager will try to find a default xxxConfigSelector with `type = default` label, for example:
- A global EmailReceiver with `type = global` label should always use the default EmailConfig which means emailConfigSelector needn't to be configured for a global EmailReceiver and one default EmailConfig with `type = default` label needs to be configured for all global EmailReceivers.  
- Usually a tenant EmailReceiver with `type = tenant` label could have its own tenant emailConfigSelector to find its tenant EmailConfig with `type = tenant` label.
- A tenant EmailReceiver with `type = tenant` label can also be configured without a emailConfigSelector, in which case Notification Manager will try to find the default EmailConfig with `type = default` label for this tenant EmailReceiver.

## QuickStart

Deploy CRDs and the Notification Manager Operator:

```shell
kubectl apply -f https://raw.githubusercontent.com/kubesphere/notification-manager/master/config/bundle.yaml
```

### Deploy Notification Manager in KubeSphere (Uses `workspace` to distinguish each tenant user):

#### Deploy Notification Manager
```shell
cat <<EOF | kubectl apply -f -
apiVersion: notification.kubesphere.io/v2beta1
kind: NotificationManager
metadata:
  name: notification-manager
spec:
  replicas: 1
  resources:
    limits:
      cpu: 500m
      memory: 1Gi
    requests:
      cpu: 100m
      memory: 20Mi
  image: kubesphere/notification-manager:v1.3.0
  imagePullPolicy: IfNotPresent
  serviceAccountName: notification-manager-sa
  portName: webhook
  defaultConfigSelector:
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
      global:
        templateFile:
        - /etc/notification-manager/template
      email:
        notificationTimeout: 5
        maxEmailReceivers: 200
      wechat:
        notificationTimeout: 5
      slack:
        notificationTimeout: 5
      pushover:
        notificationTimeout: 5
  volumeMounts:
  - mountPath: /etc/notification-manager/
    name: template
  volumes:
  - configMap:
      defaultMode: 420
      name: template
    name: template
EOF
```

#### Deploy the default EmailConfig and global EmailReceiver
```
cat <<EOF | kubectl apply -f -
apiVersion: notification.kubesphere.io/v2beta1
kind: Config
metadata:
  labels:
    app: notification-manager
    type: default
  name: default-email-config
spec:
  email:
    authPassword:
      key: password
      name: default-email-secret
    authUsername: sender1 
    from: sender1@xyz.com
    requireTLS: true
    smartHost:
      host: imap.xyz.com
      port: 25
---
apiVersion: notification.kubesphere.io/v2beta1
kind: Receiver
metadata:
  labels:
    app: notification-manager
    type: global
  name: global-email-receiver
spec:
  email:
    enabled: true
    # emailConfigSelector needn't to be configured for a global receiver
    to:
    - receiver1@xyz.com
    - receiver2@xyz.com
---
apiVersion: v1
data:
  password: dGVzdA==
kind: Secret
metadata:
  labels:
    app: notification-manager
  name: default-email-secret
  namespace: kubesphere-monitoring-system
type: Opaque
EOF
```

#### Deploy a tenant EmailConfig and a EmailReceiver
```
cat <<EOF | kubectl apply -f -
apiVersion: notification.kubesphere.io/v2beta1
kind: Config
metadata:
  labels:
    app: notification-manager
    type: tenant
    user: user1 
  name: user1-email-config
spec:
  email:
    authPassword:
      key: password
      name: default-email-secret
    authUsername: sender1 
    from: sender1@xyz.com
    requireTLS: true
    smartHost:
      host: imap.xyz.com
      port: 25
---
apiVersion: notification.kubesphere.io/v2beta1
kind: Receiver
metadata:
  labels:
    app: notification-manager
    type: tenant
    user: user1
  name: user1-email-receiver
spec:
  email:
    # This emailConfigSelector could be omitted in which case a defalut EmailConfig should be configured
    emailConfigSelector:
      matchLabels:
        type: tenant
        user: user1 
    to:
    - receiver1@xyz.com
    - receiver2@xyz.com
---
apiVersion: v1
data:
  password: dGVzdA==
kind: Secret
metadata:
  labels:
    app: notification-manager
  name: default-email-secret
  namespace: kubesphere-monitoring-system
type: Opaque
EOF
```

#### Deploy the default WechatConfig and global WechatReceivers

```
cat <<EOF | kubectl apply -f -
apiVersion: notification.kubesphere.io/v2beta1
kind: Config
metadata:
  name: default-wechat-config
  labels:
    app: notification-manager
    type: default
spec:
  wechat:
    wechatApiUrl: < wechat-api-url >
    wechatApiSecret:
      key: wechat
      name: < wechat-api-secret >
    wechatApiCorpId: < wechat-api-corp-id >
    wechatApiAgentId: < wechat-api-agent-id >
---
apiVersion: notification.kubesphere.io/v2beta1
kind: Receiver
metadata:
  name: global-wechat-receiver
  labels:
    app: notification-manager
    type: global 
spec:
  wechat:
    # wechatConfigSelector needn't to be configured for a global receiver
    # optional
    # One of toUser, toParty, toParty should be specified.
    toUser: 
      - user1
      - user2
    toParty: 
      - party1
      - party2
    toTag:
      - tag1
      - tag2
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
> - wechatApiAgentId is the id of app which sends messages to user in your WeCom.
> - wechatApiSecret is the secret of this app.
> - You can get these two parameters in App Management of your WeCom. 
> - Note that any user, party or tag who wants to receive notifications must be in the allowed users list of this app.

#### Deploy the default SlackConfig and global SlackReceiver

```
cat <<EOF | kubectl apply -f -
apiVersion: notification.kubesphere.io/v2beta1
kind: Config
metadata:
  name: default-slack-config
  labels:
    app: notification-manager
    type: default
spec:
  slack:
    slackTokenSecret: 
      key: token
      name: < slack-token-secret >
---
apiVersion: notification.kubesphere.io/v2beta1
kind: Receiver
metadata:
  name: global-slack-receiver
  labels:
    app: notification-manager
    type: global
spec:
  slack:
    # slackConfigSelector needn't to be configured for a global receiver
    channels: 
      - channel1
      - channel2
---
apiVersion: v1
data:
  token: dGVzdA==
kind: Secret
metadata:
  labels:
    app: notification-manager
  name: < slack-token-secret >
  namespace: kubesphere-monitoring-system
type: Opaque
EOF
```
> Slack token is the OAuth Access Token or Bot User OAuth Access Token when you create a Slack app. This app must have the scope chat:write. The user who creates the app or bot user must be in the channel which you want to send notification to.

#### Deploy the default WebhookConfig and global WebhookReceiver

```
cat <<EOF | kubectl apply -f -
apiVersion: v1
data:
  ca: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJzekNDQVZpZ0F3SUJBZ0lRWHNmaU9QTUdNVXZnVkhoNTgyV1BoREFLQmdncWhrak9QUVFEQWpBaE1STXcKRVFZRFZRUUxFd3ByZFdKbGMzQm9aWEpsTVFvd0NBWURWUVFEREFFcU1DQVhEVEl3TVRBeE5EQXpORE0xT0ZvWQpEekl6TVRNd01USTBNRE16TVRFMVdqQWhNUk13RVFZRFZRUUxFd3ByZFdKbGMzQm9aWEpsTVFvd0NBWURWUVFECkRBRXFNRmt3RXdZSEtvWkl6ajBDQVFZSUtvWkl6ajBEQVFjRFFnQUVNK0pSdzBSUjZJa2RueDB1U3FnSUtSRG8KdGErMzNMSWtRektHc1dWVzNmcStjQnk0Q3duVGR5aHN1SnIycVh0YVNXeVd1ekJIWENqTWYyTllSZG9KK2FOdwpNRzR3RGdZRFZSMFBBUUgvQkFRREFnR21NQThHQTFVZEpRUUlNQVlHQkZVZEpRQXdEd1lEVlIwVEFRSC9CQVV3CkF3RUIvekFwQmdOVkhRNEVJZ1FnYnU4R3o4bmlKNUo2SnI4ZVVDUW5YR2ZMSUhNOUhVcnRTcnBLdUYzTVhlOHcKRHdZRFZSMFJCQWd3Qm9jRWZ3QUFBVEFLQmdncWhrak9QUVFEQWdOSkFEQkdBaUVBclI4ZC9vaE5aRm81dEsvMwphMEJHTXRsQTBjZHh5bldWenBQZXY3Q05qVDBDSVFEQjFzN2h6dXZPM1dMWis4MG9XWFFiSDR3bE83em9MVUhQCnJGVTF3ZWtSb0E9PQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==
  cert: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJ6akNDQVhTZ0F3SUJBZ0lSQUk4VjkwMDViZGJNc2psUzYxRTliTm93Q2dZSUtvWkl6ajBFQXdJd0lURVQKTUJFR0ExVUVDeE1LYTNWaVpYTndhR1Z5WlRFS01BZ0dBMVVFQXd3QktqQWdGdzB5TURFd01UUXdNek00TlRoYQpHQTh5TXpFek1ERXlOREF6TWpZeE5Wb3dJVEVUTUJFR0ExVUVDaE1LYTNWaVpYTndhR1Z5WlRFS01BZ0dBMVVFCkF3d0JLakJaTUJNR0J5cUdTTTQ5QWdFR0NDcUdTTTQ5QXdFSEEwSUFCTGwxS3MyTlJueXhmUDZGQzYzTHhobWoKZ2RRTlB1MDlLKzIwZmdkM3Q3NW9GVXdDSzkrSXNlaHRTRzlnSzhSNWhiejBoZ082RGZoM0hyQ3RCMm1ZS1RpagpnWW93Z1ljd0RnWURWUjBQQVFIL0JBUURBZ0dtTUE4R0ExVWRKUVFJTUFZR0JGVWRKUUF3REFZRFZSMFRBUUgvCkJBSXdBREFwQmdOVkhRNEVJZ1FnUnc5ZXBQN1BMODhtSHBXNzh3ekJtTFBqMkhqMTZYa1pJdFJub0dPK3VUMHcKS3dZRFZSMGpCQ1F3SW9BZ1Q1Z09zSmQrajdzY2NpY3RXM0JINjVpM2owb3FrSGdaQ2gvMDVzYW5kNWN3Q2dZSQpLb1pJemowRUF3SURTQUF3UlFJaEFQT2hJUjRnQ0wxUTdCT1Y2cXNYUWIyTjhsanZzTjhYTmxzY1FsVkhsRlE4CkFpQndaWlphWGMyeC9CVEd0alhnU3pHaStTbEVVTDE3SUVaZmdZYjNkQ2tweVE9PQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==
  key: LS0tLS1CRUdJTiBFQyBQUklWQVRFIEtFWS0tLS0tCk1IY0NBUUVFSVBpMEpMYnlUMjdNczl2RWJ4WHNCckhlYjAyZ3VLY2NNVHRQSWI5RXRyZXFvQW9HQ0NxR1NNNDkKQXdFSG9VUURRZ0FFdVhVcXpZMUdmTEY4L29VTHJjdkdHYU9CMUEwKzdUMHI3YlIrQjNlM3ZtZ1ZUQUlyMzRpeAo2RzFJYjJBcnhIbUZ2UFNHQTdvTitIY2VzSzBIYVpncE9BPT0KLS0tLS1FTkQgRUMgUFJJVkFURSBLRVktLS0tLQo=
  password: ZmYwZDM4YWItN2IzMC00ODE1LWI5OTMtMjQwYTc3YjQwZmMw
  bearer: ZXlKaGJHY2lPaUpJVXpJMU5pSXNJblI1Y0NJNklrcFhWQ0o5LmV5SjFjMlZ5SWpvaU5XSmlZemxrT0RFdFpqYzJNUzAwWVdFNUxXSmlNekl0WkRCaU9UVmtaR05rTXpkbUlpd2laWGh3SWpveE9UWXlOalUxT0RjNUxDSnBZWFFpT2pFMk1ESTJOVFU0Tnprc0ltbHpjeUk2SW10MVltVnpjR2hsY21VaUxDSnVZbVlpT2pFMk1ESTJOVFU0TnpsOS40Rk4wS3FIRF91Q1AtRmFIMmFpT3ZPUjFsY2wtVjFyS0Z4d2RQXzNuRmY0
kind: Secret
metadata:
  labels:
    app: notification-manager
  name: default-webhook-secret
  namespace: kubesphere-monitoring-system
type: Opaque

---
apiVersion: notification.kubesphere.io/v2beta1
kind: Config
metadata:
  name: default-webhook-config
  labels:
    app: notification-manager
    type: default

---
apiVersion: notification.kubesphere.io/v2beta1
kind: Receiver
metadata:
  name: global-webhook-receiver
  labels:
    app: notification-manager
    type: global
spec:
  webhook:
    url: http://127.0.0.1:8080/
    httpConfig: 
      bearerToken
        key: password
        name: default-webhook-secret
      tlsConfig:
        rootCA:
          key: ca
          name: default-webhook-secret
        clientCertificate:
          cert:
            key: cert
            name: default-webhook-secret
          key:
            key: key
            name: default-webhook-secret
        insecureSkipVerify: false
EOF
```

> - The `rootCA` is the server root certificate.
> - The `certificate` is the clientCertificate of client.
> - The format of bearerToken is `Authorization <bearerToken>`.

#### Deploy the default DingTalkConfig and a global DingTalkReceiver

```
cat <<EOF | kubectl apply -f -
apiVersion: v1
data:
  appkey: ZGluZ2Jla3UxR2enAyeHQ=
  appsecret: dnRFNWt2RWppOWdiZF9x
  webhook: aHR0cHM6Ly9vYXBpLmRpbmd0YWxrLmNvbS9yb2JvdC9zZW5kP2FjY2Vzc190b2tlbj0zNjUxO
  secret: U0VDZjJiMTkyOGUwOGY5ZjM4YzIwMmZGNiN2VhMjk1MTMyNDI0YTgxMDljMjFkYzYwNGU3MDkzNQ==
kind: Secret
metadata:
  labels:
    app: notification-manager
  name: default-dingtalk-secret
  namespace: kubesphere-monitoring-system
type: Opaque

---
apiVersion: notification.kubesphere.io/v2beta1
kind: Config
metadata:
  name: default-dingtalk-config
  labels:
    app: notification-manager
    type: default
spec:
  dingtalk:
    conversation:
      appkey: 
        key: appkey
        name: default-dingtalk-secret
      appsecret:
        key: appsecret
        name: default-dingtalk-secret

---
apiVersion: notification.kubesphere.io/v2beta1
kind: Receiver
metadata:
  name: global-dingtalk-receiver
  labels:
    app: notification-manager
    type: global
spec:
  dingtalk:
    conversation:
      chatids: 
        - chat1
        - chat2
    chatbot:
      webhook:
        key: webhook
        name: default-dingtalk-secret
      keywords: 
      - kubesphere
      secret:
        key: secret
        name: default-dingtalk-secret
EOF
```

> - DingTalkReceiver can both send messages to `conversation` and `chatbot`.
> - If you want to send messages to conversation, the application used to send messages to conversation must have the authority `Enterprise conversation`, and the IP which notification manager used to send messages must be in the white list of the application. Usually, it is the IP of Kubernetes nodes, you can simply add all Kubernetes nodes to the white list.
> - The `appkey` is the key of the application, the `appsecret` is the secret of the application.
> - The `chatids` is the id of the conversation, it can only be obtained from the response of [creating conversation](https://ding-doc.dingtalk.com/document#/org-dev-guide/create-chat).
> - The `webhook` is the URL of a chatbot, the `keywords` is the keywords of a chatbot, The `secret` is the secret of chatbot, you can get them in the setting page of chatbot.

#### Deploy the default SmsConfig and a global SmsReceiver
```
apiVersion: notification.kubesphere.io/v2beta2
kind: Receiver
metadata:
  labels:
    app: notification-manager
    type: global
  name: global-sms-receiver
spec:
  sms:
    enabled: true
    phoneNumbers: ["13612345678"]
---
apiVersion: notification.kubesphere.io/v2beta2
kind: Config
metadata:
  labels:
    app: notification-manager
    type: default
  name: default-sms-config
spec:
  sms:
    defaultProvider: huawei
    providers:
      huawei:
        url: https://rtcsms.cn-north-1.myhuaweicloud.com:10743/sms/batchSendSms/v1
        signature: "xxx"
        templateId: "xxx"
        sender: "12323313"
        appSecret:
          valueFrom:
            secretKeyRef:
              namespace: "default"
              key: huawei.appSecret
              name: default-sms-secret
        appKey:
          valueFrom:
            secretKeyRef:
              namespace: "default"
              key: huawei.appKey
              name: default-sms-secret
      aliyun: 
        signName: xxxx 
        templateCode: xxx
        accessKeyId: 
          valueFrom:
            secretKeyRef:
              namespace: "default"
              key: aliyun.accessKeyId
              name: default-sms-secret
        accessKeySecret:
          valueFrom:
            secretKeyRef:
              namespace: "default"
              key: aliyun.accessKeySecret
              name: default-sms-secret
      tencent:
        templateID: xxx
        smsSdkAppid: xxx
        sign: xxxx
        secretId:
          valueFrom:
            secretKeyRef:
              namespace: "default"
              key: tencent.secretId
              name: default-sms-secret
        secretKey:
          valueFrom:
            secretKeyRef:
              namespace: "default"
              key: tencent.secretKey
              name: default-sms-secret

---
apiVersion: v1
data:
  aliyun.accessKeyId: eHh4eA==
  aliyun.accessKeySecret: eHh4eA==
  tencent.secretId: eHh4eA==
  tencent.secretKey: eHh4eA==
  huawei.appKey: eHh4eA==
  huawei.appSecret: eHh4eA==
kind: Secret
metadata:
  labels:
    app: notification-manager
  name: default-sms-secret
type: Opaque
```
For SMS templates,  you can create them in your SMS provider's SMS console.
For Huawei Cloud SMS provider, pls make your custom SMS template containing ten placeholders. 
For a detailed description, if you have a template like this: 
```
[KubeSphere alerts] alertname = ${TEXT}, severity = ${TEXT}, message = ${TEXT}, summary = ${TEXT}, alerttype = ${TEXT}, cluster = ${TEXT}, node = ${TEXT}, namespace = ${TEXT}, workload = ${TEXT}, pod = ${TEXT}
```
Then you will receive the notification as below:
```
[KubeSphere alerts] alertname = test, severity = warning, message = this is a test message, summary = node node1 memory utilization >= 10%, alerttype = metric, cluster = default, node = node1, namespace = kube-system, workload = nginx-deployment, pod = pod1
```

#### Deploy the default PushoverConfig and global PushoverReceiver

```
cat <<EOF | kubectl apply -f -
apiVersion: notification.kubesphere.io/v2beta2
kind: Config
metadata:
  name: default-pushover-config
  labels:
    app: notification-manager
    type: default
spec:
  pushover:
    pushoverTokenSecret:
      valueFrom:
        secretKeyRef:
          key: token
          name: < pushover-token-secret-name >
---
apiVersion: notification.kubesphere.io/v2beta2
kind: Receiver
metadata:
  name: global-pushover-receiver
  labels:
    app: notification-manager
    type: global
spec:
  pushover:
    # pushoverConfigSelector needn't to be configured for a global receiver
    profiles:
    - userKey: < Pushover-User-Key >
      title: "test title" # optional
      sound: "classical" # optional
      devices: ["iphone"] # optional
---
apiVersion: v1
data:
  token: < Pushover-Application-Token >
kind: Secret
metadata:
  labels:
    app: notification-manager
  name: < pushover-token-secret-name >
  namespace: kubesphere-monitoring-system
type: Opaque
EOF
```
> You can add a profile object for each user under `profiles`. This object includes:
>* UserKey: Required. A unique identifier for the user. Each Pushover user is assigned a user key, same as an username. Each user who intends to receive alerts via Pushover will have to configure their user key here.
>* Devices: Optional. Device names to send the message directly to that device, rather than all of the user's devices.
>* Title: Optional. Message's title, otherwise your app's name is used.
>* Sound: Optional. Sound refers to the name of one of the [sounds](https://pushover.net/api#sounds) supported by device clients


### Deploy Notification Manager in any other Kubernetes cluster (Uses `namespace` to distinguish each tenant user):

Deploying Notification Manager in Kubernetes is similar to deploying it in KubeSphere, the differences are:

Firstly, change the `tenantKey` to `namespace` like this.

```
apiVersion: notification.kubesphere.io/v2beta1
kind: NotificationManager
metadata:
  name: notification-manager
spec:
  receivers:
    tenantKey: namespace
```

Secondly, change the label of receiver and config from `user: ${user}` to `namespace: ${namespace}` like this.

```
cat <<EOF | kubectl apply -f -
apiVersion: notification.kubesphere.io/v2beta1
kind: Config
metadata:
  labels:
    app: notification-manager
    type: tenant
    namespace: default
  name: user1-email-config
spec:
  email:
    authPassword:
      key: password
      name: user1-email-secret
    authUsername: sender1 
    from: sender1@xyz.com
    requireTLS: true
    smartHost:
      host: imap.xyz.com
      port: 25
---
apiVersion: notification.kubesphere.io/v2beta1
kind: Receiver
metadata:
  labels:
    app: notification-manager
    type: tenant
    namespace: default
  name: user1-email-receiver
spec:
  email:
    emailConfigSelector:
      matchLabels:
        type: tenant
    to:
    - receiver3@xyz.com
    - receiver4@xyz.com
EOF
```

### Notification filter

A receiver can filter alerts by setting a label selector, only alerts that match the label selector will be sent to this receiver.
Here is a sample, this receiver will only receive alerts from auditing.

```
apiVersion: notification.kubesphere.io/v2beta1
kind: Receiver
metadata:
  labels:
    app: notification-manager
    type: global
  name: global-email-receiver
spec:
  email:
    to:
    - receiver1@xyz.com
    - receiver2@xyz.com
    alertSelector:
      matchLabels:
        alerttype: auditing
```

### Customize template

You can customize the notifications by customizing the template. You need to create a template file include the template that you customized, and mount it to `NotificationManager`. Then you can change the template to the template which you defined.

It can set a global template, or set a template for each type of receivers, or set a template for each receiver, or use default template.  
The priority of these templates is:

```
default template < global template < template for each type of receivers < receiver template
```

To set a global template:

```yaml
apiVersion: notification.kubesphere.io/v2beta1
kind: NotificationManager
metadata:
  name: notification-manager
spec:
  receivers:
    options:
      global:
        template: <template>
```

To set template for each type of receivers:

```yaml
apiVersion: notification.kubesphere.io/v2beta1
kind: NotificationManager
metadata:
  name: notification-manager
spec:
  receivers:
    options:
      email:
        subjectTemplate:  <subject-template>
        template: <template>
      wechat:
        template: <template>
      slack:
        template: <template>
      webhook:
        template: <template>
      dingtalk:
        template: <template>
      sms:
        template: <template>
      pushover:
        template: <template>
```

> The email receiver can set template for text and template for subject.

To set receiver template:

```yaml
apiVersion: notification.kubesphere.io/v2beta1
kind: Receiver
metadata:
  labels:
    app: notification-manager
    type: global
  name: global-email-receiver
spec:
  email:
    subjectTemplate:  <subject-template>
    template: <template>
```

The dingtalk and wechat support two message formats: `text` and `markdown`, the email supports two message formats: `html` and `text`
You can set a global message format for dingtalk, wechat, or email, or set a message format for each dingtalk receiver, wechat receiver, or email receiver.
The priority of these message format is:

```
default message format < global message format < receiver message format
```


To set a global message format:

```yaml
apiVersion: notification.kubesphere.io/v2beta1
kind: NotificationManager
metadata:
  name: notification-manager
spec:
  receivers:
    options:
      email:
        tmplType: html
      wechat:
        tmplType: markdown
      dingtalk:
        tmplType: markdown
        titleTemplate: <title-template>
```

> When the message format of dingtalk is `markdown`, you can set the message format of title for dingtalk.

To set receiver message format:

```yaml
apiVersion: notification.kubesphere.io/v2beta1
kind: Receiver
metadata:
  labels:
    app: notification-manager
    type: global
  name: global-email-receiver
spec:
  email:
    tmplType: html
  wechat:
    tmplType: markdown
  dingtalk:
    tmplType: markdown
    titleTemplate: <title-template>
```

Here is the template `nm.default.text`. For more information about templates, you can see [here](https://prometheus.io/docs/alerting/latest/notifications/).

```
    {{ define "nm.default.subject" }}{{ .Alerts | len }} alert{{ if gt (len .Alerts) 1 }}s{{ end }} for {{ range .GroupLabels.SortedPairs }} {{ .Name }}={{ .Value }} {{ end }}
    {{- end }}

    {{ define "__nm_alert_list" }}{{ range . }}Labels:
    {{ range .Labels.SortedPairs }}{{ if ne .Name "runbook_url" }}- {{ .Name }} = {{ .Value }}{{ end }}
    {{ end }}Annotations:
    {{ range .Annotations.SortedPairs }}{{ if ne .Name "runbook_url"}}- {{ .Name }} = {{ .Value }}{{ end }}
    {{ end }}
    {{ end }}{{ end }}

    {{ define "nm.default.text" }}{{ template "nm.default.subject" . }}
    {{ if gt (len .Alerts.Firing) 0 -}}
    Alerts Firing:
    {{ template "__nm_alert_list" .Alerts.Firing }}
    {{- end }}
    {{ if gt (len .Alerts.Resolved) 0 -}}
    Alerts Resolved:
    {{ template "__nm_alert_list" .Alerts.Resolved }}
    {{- end }}
    {{- end }}
```

## Update

There are some breaking changes in v1.0.0 :

- All config crds are aggregated into a crd named `Config`.
- All receivers crds are aggregated into a crd named `Receiver`.
- Now the `Config`, `Receiver`, and `NotificationManager` are cluster scoped crd.
- The `NotificationManager` crd add a property named `defaultSecretNamespace` which defines the default namespace to which notification manager secrets belong.
- Now the namespace of the secret can be specified in `SecretKeySelector` like this. 
  If the `namespace` of `SecretKeySelector` has be set, notification manager will get the secret in this namespace, 
  else, notification manager will get the secret in the `defaultSecretNamespace`,
  if the `defaultSecretNamespace` does not set, will get the secret from the namespace of notification manager operator.

```yaml
    kind: Config
    metadata:
      labels:
        type: tenant
        namespace: default
      name: user1-email-config
    spec:
      email:
        authPassword:
          key: password
          name: user1-email-secret
          namespace: kubesphere-monitoring-system
```

- Move the configuration of DingTalk chatbot from dingtalk config to dingtalk receiver.
- Move the chatid of DingTalk conversation from dingtalk config to dingtalk receiver.
- Now the `chatid` of DingTalk conversation is an array types, and renamed to `chatids`.
- Now the `port` of email `smartHost` is an integer type.
- Now the `channel` fo slack is an array types, and renamed to `channels`.
- Move the configuration of webhook from webhook config to webhook receiver.
- Now the `toUser`, `toParty`, `toTag` of wechat receiver are array type.

### Steps to migrate crds from v0.x to latest

You can update the v0.x to the latest version by following this.

Firstly, backup the old crds and converts to new crds.

```shell
curl -o update.sh https://raw.githubusercontent.com/kubesphere/notification-manager/master/config/update/update.sh && sh update.sh
```

>This command will generate two directories, backup and crds. The `backup` directory store the old crds, and the `crds` directory store the new crds

Secondly, delete old crds.

```shell
kubectl delete --ignore-not-found=true crd notificationmanagers.notification.kubesphere.io
kubectl delete --ignore-not-found=true crd dingtalkconfigs.notification.kubesphere.io
kubectl delete --ignore-not-found=true crd dingtalkreceivers.notification.kubesphere.io
kubectl delete --ignore-not-found=true crd emailconfigs.notification.kubesphere.io
kubectl delete --ignore-not-found=true crd emailreceivers.notification.kubesphere.io
kubectl delete --ignore-not-found=true crd slackconfigs.notification.kubesphere.io
kubectl delete --ignore-not-found=true crd slackreceivers.notification.kubesphere.io
kubectl delete --ignore-not-found=true crd webhookconfigs.notification.kubesphere.io
kubectl delete --ignore-not-found=true crd webhookreceivers.notification.kubesphere.io
kubectl delete --ignore-not-found=true crd wechatconfigs.notification.kubesphere.io
kubectl delete --ignore-not-found=true crd wechatreceivers.notification.kubesphere.io
```

Thirdly, deploy the notification-manager of the latest version.

```shell
kubectl apply -f https://raw.githubusercontent.com/kubesphere/notification-manager/master/config/bundle.yaml
```

Finally, deploy configs and receivers.

```shell
kubectl apply -f crds/
```

## Development

```
# Build notification-manager-operator and notification-manager docker images
make build 
# Push built docker images to docker registry
make push
```

## Documentation

- [API documentation](./docs/api/_index.md).