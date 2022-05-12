## v2.0.1 / 2022-05-13

### BUGFIX

- Fix bug that the dispatcher worker routine is not released (#136). @gliffcheung
- Fix bug that the template could not generate notifications correctly (#139). @wanjunlei
- Fix bug that the namespace cannot be specified when installing with helm (#141). @wanjunlei

## v2.0.0 / 2022-04-18

### FEATURES

- Support to route the specific notifications to the specific receivers using `Router` CRD (#121). @wanjunlei
- Support to mute specific notifications for a given period using `Silence` CRD (#121). @wanjunlei
- Add support to send notifications to Feishu (#124). @wanjunlei
- Support the dynamic modification of the template (#123). @wanjunlei
- Add e2e tests with GitHub action (#111). @zhu733756
- Add support to build and push a container image after PR is merged (#122). @wenchajun

### Enhancement

- Refactor the cache mechanism of receiver and config (#118). @wanjunlei
- Add support for custom pushover title using template (#128). @wanjunlei
- Optimize the mechanism of reloading the receivers and configs after Notification Manager CR changed (#127). @wanjunlei
- Support to get KubeSphere cluster name from configmap (#125). @wanjunlei

### BUGFIX

- Resolve the problem of filtered notifications being recorded in the notification history (#117). @wanjunlei

## v1.4.0 / 2021-10-14

### FEATURES
- Support collecting notification history (#102). @wanjunlei
- Add tenant sidecar for KubeSphere v3.2.0 (#105). @wanjunlei

### UPGRADE & BUGFIX
- Fix the bug that the error is not returned to the caller during notification setting verification (#104). @wanjunlei
- Fix WeChat alert selector doesn't work (#109). @wanjunlei

## v1.3.0 / 2021-08-10

### FEATURES
- Add support to send notifications to Huawei SMS platform (#90 #94). @zhu733756
- Add support to send notifications to Pushover (#91). @txfs19260817

### CHANGE 
- Adjust alertmanager integration guide.
- Adjust hostpath host-time (mounts /etc/localtime) to read-only mode.

## v1.2.0 / 2021-07-15

### FEATURES
- Add support to verify the receivers and configs. @wanjunlei
- Add support to get tenant info from a sidecar. @wanjunlei
- Add support to send notifications to SMS platforms (Aliyun & Tencent). @zhu733756
- Add support to send `DingTalk` notifications in `markdown` format. @happywzy
- Add support to send `Wechat` notifications in `markdown` format. @wanjunlei
- Add support to send `Email` notifications in `text` format. @wanjunlei
- Now DingTalk chatbot can `@` someone in the notification messages. @happywzy
- Now Every receiver can set a template for itself. @wanjunlei
- Add an API to send notifications directly. @wanjunlei

### CHANGE
- Upgrade the crd version to v2beta2.

## v1.0.0 / 2021-04-22

### FEATURES
- All config crds are aggregated into a crd named `Config`. 
- All receivers crds are aggregated into a crd named `Receiver`.
- Now the `Config`, `Receiver`, and `NotificationManager` are cluster scoped crd.
- Now the namespace of the secret can be specified in `SecretKeySelector` .
- Move the configuration of DingTalk chatbot from dingtalk config to dingtalk receiver.
- Move the `chatid` of DingTalk conversation from dingtalk config to dingtalk receiver.
- Now the `chatid` of DingTalk conversation is an array types, and renamed to `chatids`.
- Now the `port` of email `smartHost` is an integer type.
- Now the `channel` fo slack is an array types, and renamed to `channels`.
- Move the configuration of webhook from webhook config to webhook receiver.
- Now the `toUser`, `toParty`, `toTag` of wechat receiver are array type.

