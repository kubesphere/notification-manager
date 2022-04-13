# Router

## Overview

`Router` CRD is used to send the specified notifications to the specified receivers.

```yaml
apiVersion: notification.kubesphere.io/v2beta2
kind: Router
metadata:
  name: router1
spec:
  alertSelector:
    matchExpressions:
      - key: alertname
        operator: In
        values:
        - CPUThrottlingHigh
  receivers:
    name: 
      - user1
    regexName: "user1.*?"
    selector: [] 
    type: email
```

A router resource allows user to define:

- `alertSelector` - A label selector used to match alert. The matched alert will send to the `receivers`.
- `receivers.name` - The name of receivers which notifications will send to.
- `receivers.regexName` - A regular expression to match the receiver name.
- `receivers.selector` - A label selector used to select receivers.
- `type` - The type of receiver, known values are dingtalk, email, feishu, pushover, sms, slack, webhook, WeChat.

## Examples

A router that routes all notifications to the all receivers of tenant `user1`.

```yaml
apiVersion: notification.kubesphere.io/v2beta2
kind: Router
metadata:
  name: router1
spec:
  receivers:
    selector:
      matchLabels:
        tenant: user1
```

A router that routes cluster-level notifications to the email receivers `user1`.

```yaml
apiVersion: notification.kubesphere.io/v2beta2
kind: Router
metadata:
  name: router1
spec:
  alertSelector:
    matchExpressions:
      - key: namespace
        operator: DoesNotExist
  receivers:
    name: 
      - user1
    type: email
```
