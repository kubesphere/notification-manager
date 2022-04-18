# Config

## Overview

`Config` is used to define the information needed to send notifications, such as SMTP server, DingTalk setting, slack token, etc. 
`Config` can be categorized into 2 types `tenant` and `default` by label like `type = tenant`, `type = default`:
- The `tenant` config can only be selected by tenant receivers with the same tenant label  .
- The `default` config can be selected by all receivers. Usually admin will set a global default config.

A receiver will select a config according [this](receiver.md#how-to-select-config).

A config resource allows the user to define:

- [dingtalk](#DingTalk-Config)
- [email](#Email-Config)
- [feishu](#Feishu-Config)
- [pushover](#Pushover-Config)
- [slack](#Slack-Config)
- [sms](#SMS-Config)
- [wechat](#WeChat-Config)

## DingTalk Config

A dingtalk config is like this.

```yaml
apiVersion: notification.kubesphere.io/v2beta2
kind: Config
metadata:
  name: default-config
  labels:
    type: default
spec:
  dingtalk:
    conversation:
      appkey: 
        valueFrom:
          secretKeyRef: 
            key: appkey
            name: defalut-config-secret
            namespace: kubesphere-monitoring-system
      appsecret:
        valueFrom:
          secretKeyRef: 
            key: appsecret
            name: defalut-config-secret
            namespace: kubesphere-monitoring-system
```

A dingtalk config allows the user to define:

- `conversation.appkey` - Refers to the key of the application with which to send messages, and `type` is [credential](./credential.md). For more information, please refer to [this](https://open.dingtalk.com/document/orgapp-server/used-to-obtain-the-application-authorization-without-a-logon-user).
- `conversation.appsecret` - Refers to the secret of the application with which to send messages, and `type` is [credential](./credential.md).

> The application used to send notifications must have the authority `Enterprise conversation`,
and the IP which Notification Manager used to send messages must be in the allowlist of the application. Usually, it is the IP of Kubernetes nodes, you can simply add all Kubernetes nodes to the white list.

## Email Config

An email config is like this.

```yaml
apiVersion: notification.kubesphere.io/v2beta2
kind: Config
metadata:
  name: default-config
  labels:
    type: default
spec:
  email:
    hello: "hello"
    authIdentify: nil
    authPassword:
      valueFrom:
        secretKeyRef: 
          key: password
          name: defalut-config-secret
          namespace: kubesphere-monitoring-system
    authUsername: admin
    from: admin@kubesphere.io
    requireTLS: true
    smartHost:
      host: imap.kubesphere.io
      port: 25
    tls: []
```

An email config allows the user to define:

- `authIdentify` - The identity for PLAIN authentication.
- `authUsername` - The username for CRAM-MD5, LOGIN and PLAIN authentications.
- `authPassword` - The password for CRAM-MD5, LOGIN and PLAIN authentications, and `type` is [credential](./credential.md).
- `from` - Email address to send notifications to.
- `hello` - The domain name of the sending host, It will register to SMTP server using the `HELO` command before the MAIL FROM command.
- `smartHost.host` - The host of the SMTP server.
- `smartHost.port` - The port of the SMTP server.
- `tls` - TLS configuration to use to connect to the targets. For more information, please refer to [TlsConfig](./receiver.md#TlsConfig).

## Feishu Config

A feishu config is like this.

```yaml
apiVersion: notification.kubesphere.io/v2beta2
kind: Config
metadata:
  name: default-config
  labels:
    type: default
spec:
  feishu:
    appID: 
      valueFrom:
        secretKeyRef: 
          key: appkey
          name: defalut-config-secret
          namespace: kubesphere-monitoring-system
    appSecret:
      valueFrom:
        secretKeyRef: 
          key: appsecret
          name: defalut-config-secret
          namespace: kubesphere-monitoring-system
```

A feishu config allows the user to define:

- `appID` - The key of the application with which to send messages, and `type` is [credential](./credential.md). For more information, please refer to [this](https://open.feishu.cn/document/home/develop-a-gadget-in-5-minutes/create-an-app).
- `appSecret` - The secret of the application with which to send messages, and `type` is [credential](./credential.md).

> The application used to send notifications must have authorities `Read and send messages in private and group chats`, `Send batch messages to multiple users`, and `Send batch messages to members from one or more departments`.

## Pushover Config

A pushover config is like this.

```yaml
apiVersion: notification.kubesphere.io/v2beta2
kind: Config
metadata:
  name: default-config
  labels:
    type: default
spec:
  pushover:
    pushoverTokenSecret: 
      valueFrom:
        secretKeyRef: 
          key: token
          name: defalut-config-secret
          namespace: kubesphere-monitoring-system
```

A pushover config allows the user to define:

- `pushoverTokenSecret` - The token of a pushover application, and `type` is [credential](./credential.md).

## Slack Config

A slack config is like this.

```yaml
apiVersion: notification.kubesphere.io/v2beta2
kind: Config
metadata:
  name: default-config
  labels:
    type: default
spec:
  slack: 
    slackTokenSecret: 
      valueFrom:
        secretKeyRef: 
          key: token
          name: defalut-config-secret
          namespace: kubesphere-monitoring-system
```

A slack config allows the user to define:

- `slackTokenSecret` - The token of slack user or bot, and `type` is [credential](./credential.md).

> Slack token is the OAuth Access Token or Bot User OAuth Access Token when you create a Slack app. The application used to send notifications must have scope chat:write. The application must be in the channel which you want to send notifications to.

## SMS Config

An SMS config is like this.

```yaml
apiVersion: notification.kubesphere.io/v2beta2
kind: Config
metadata:
  labels:
    type: default
  name: default-config
spec:
  sms:
    defaultProvider: huawei
    providers:
      huawei:
        url: https://rtcsms.cn-north-1.myhuaweicloud.com:10743/sms/batchSendSms/v1
        signature: xxx
        templateId: xxx
        templateParas: xxx
        sender: kubesphere
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
```

An SMS config allows the user to define:

- `defaultProvider` - The default SMS provider. It must be one of `aliyun`, `huawei`, and `tencent`. The first provider will be used if not set.
- `providers` - The SMS provider, which supports `aliyun`, `huawei` and `tencent`.

### Aliyun SMS provider

The Aliyun SMS provider allows the user to define:

- `signName` - SMS signature name.
- `templateCode` - The code of the SMS template.
- `accessKeyId` - The ID of the access key. For more information, please refer to [this](https://help.aliyun.com/document_detail/53045.htm?spm=a2c4g.11186623.0.0.71f36cf0OtJmES#task-354412).
- `accessKeySecret` - The secret of the access key. For more information, please refer to [this](https://help.aliyun.com/document_detail/53045.htm?spm=a2c4g.11186623.0.0.71f36cf0OtJmES#task-354412).

### Huawei SMS provider

A Huawei SMS provider allows the user to define:

- `url` - The url used to send SMS.
- `signature` - SMS signature name.
- `templateId` - The ID of SMS template.
- `appSecret` - The secret of SMS application. For more information, please refer to [SMS application](https://support.huaweicloud.com/usermanual-msgsms/sms_03_0001.html).
- `appKey` - The key of SMS application. For more information, please refer to [SMS application](https://support.huaweicloud.com/usermanual-msgsms/sms_03_0001.html).

### Tencent SMS provider

A Tencent SMS provider allows the user to define:

- `sign` - SMS signature name.
- `templateID` - The ID of SMS template.
- `smsSdkAppid` - The SMS SdkAppId generated after adding the app in the SMS console.
- `secretId` - The id of API secret, and `type` is [credential](./credential.md). You can get it from [here](https://cloud.tencent.com/login?s_url=https%3A%2F%2Fconsole.cloud.tencent.com%2Fcapi).
- `secretKey` - The key of API secret, and `type` is [credential](./credential.md). . You can get it from [here](https://cloud.tencent.com/login?s_url=https%3A%2F%2Fconsole.cloud.tencent.com%2Fcapi).

## WeChat Config

A WeChat config is like this.

```yaml
apiVersion: notification.kubesphere.io/v2beta2
kind: Config
metadata:
  name: default-config
  labels:
    app: notification-manager
    type: default
spec:
  wechat:
    wechatApiUrl: https://qyapi.weixin.qq.com/cgi-bin/
    wechatApiSecret:
      valueFrom:
        secretKeyRef:
          key: wechat
          name: defalut-config-secret
          namespace: kubesphere-monitoring-system
    wechatApiCorpId: "********"
    wechatApiAgentId: "1000003"
```

A WeChat config allows the user to define:

- `wechatApiUrl` - The WeChat API server, and the default value is `https://qyapi.weixin.qq.com/cgi-bin/`.
- `wechatApiCorpId` - The corporation ID for authentication. For more information, please refer to [corpid](https://developer.work.weixin.qq.com/document/path/90665#corpid).
- `wechatApiSecret` - The secret of the application which to send messages. For more information, please refer to [secret](https://developer.work.weixin.qq.com/document/path/90665#secret).
- `wechatApiAgentId` - The id of the application which to send messages. For more information, please refer to [agentid](https://developer.work.weixin.qq.com/document/path/90665#agentid).

> Any user, party or tag who wants to receive notifications must be in the allowed users list of the application which to send message.