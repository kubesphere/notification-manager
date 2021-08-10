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

## v1.3.0 / 2021-08-10

### FEATURES
- Add support to send notifications to Huawei SMS platform (#90 #94). @zhu733756
- Add support to send notifications to Pushover (#91). @txfs19260817

### CHANGE 
- Adjust alertmanager integration guide.