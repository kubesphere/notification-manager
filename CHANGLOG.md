## v2.5.3 / 2025-03-10

### Bugfix
- update template (#280) @Gentleelephant

## v2.5.2 / 2024-05-14

### Enhancements

- support to get tenant info from multiple cluster (#263) @wanjunlei
- trim space (#267) @Gentleelephant
- update template (#264) @Gentleelephant
- update helm template (#266) @wanjunlei
- add alerttime to metric alert (#265) @wanjunlei

### Bugfix

- Fix Typo in documentation (#262) @mohamed-rafraf

### Update

- Bump google.golang.org/protobuf from 1.30.0 to 1.33.0 (#252) @dependabot
- Bump google.golang.org/protobuf from 1.31.0 to 1.33.0 in /sidecar/kubesphere/4.0.0 (#253) @dependabot
- Bump golang.org/x/net from 0.17.0 to 0.23.0 in /sidecar/kubesphere/4.0.0 (#260) @dependabot
- Bump golang.org/x/net from 0.17.0 to 0.23.0 (#261) @dependabot

## v2.5.1 / 2024-04-03

### Bugfix

- fix bug regex silence will silence all alerts(#257). @wanjunlei


## v2.5.0 / 2024-03-21

### FEATURES

-  Support sending notification history without adaptor(#227). @wanjunlei
-  New tenant sidecar for kubesphere v4.0.0(#231). @wanjunlei

### Enhancements

- Add receiver name to the notification(#235). @wanjunlei
- Add annotation of alert to the notification(#238). @wanjunlei  
- Update go version to 1.20(#236). @wanjunlei

### Deprecations

- Delete the v2beta1 version of the CRD(#230). @wanjunlei

### Bugfix

- Fix a bug that notification manager will crash when the smtp server is not available(#245). @Gentleelephant

## v2.4.0 / 2023-09-20

### FEATURES

-  Support sending notifications to telegram(#210). @mangoGoForward

### Enhancements

- Supports regular expression matching in Receiver, Silence, and Router(#215). @wenchajun

## v2.3.0 / 2023-04-12

### Enhancements

- Optimize the logic of notification history(#193). @wanjunlei

## v2.3.0-rc.0 / 2023-02-07

### Enhancements

- Optimize the logic of notification history(#193). @wanjunlei

## v2.2.0 / 2023-01-06

### FEATURES

- Support Wechat bot receiver(#179). @Gentleelephant
- Support discord receiver(#185). @Gentleelephant

### Enhancements & Updates

- Update kube-rbac-proxy to 0.11.0(#189). @Gentleelephant

## v2.1.0 / 2022-11-29

### FEATURES

- Support sending notifications to AWS SMS (#159). @Bennu-Li
- Support routing notifications to specified tenants (#163). @wanjunlei

### Enhancements
- Support using the `TZ` environment variable instead of the host path `/etc/localtime` to set the time zone (#148). @ctrought
- Enhanced notification template supports automatic message selection based on notification language (#160). @wanjunlei

### BUGFIX

- Fixed `Feishu` does not support a short template name (#152). @wanjunlei

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

