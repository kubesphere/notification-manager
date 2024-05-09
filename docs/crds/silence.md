# Silence

## Overview

`Silence` CRD is used to define policies to mute notifications for a given time. A silence is configured based on a label selector.
If the incoming alert matches the label selector of an active silence, no notifications will be sent out for that alert.

`Silence` can be categorized into 2 types `global` and `tenant` by label like `type = global`, `type = tenant` :
- A global silence will mute all notifications that match the label selector. The global silence will take effect in the [silence](../../README.md#silence) step.
- A tenant silence only mutes the notifications that will send to receivers of this tenant. The tenant silence will take effect in the [filter](../../README.md#filter) step.

A silence resource allows the user to define:

- `enabled` - whether the silence enabled.
- `matcher` - The label selector used to match alert.
- `startsAt` - The start time during which the silence is active.
- `schedule` - The schedule in Cron format. If set, the silence will be active periodicity, and the startsAt will be invalid.
- `duration` - The time range during which the silence is active. If not set, the silence will be active ever.

> If the `startsAt` and `schedule` are not set, the silence will be active for ever.

### Examples

A silence that mutes all notifications in namespace `test` and is active for ever.

```yaml
apiVersion: notification.kubesphere.io/v2beta2
kind: Silence
metadata:
  name: silence1
  labels:
    type: global
spec:
  matcher:
    matchExpressions:
      - key: namespace
        operator: In
        values:
        - test
```

A silence that mutes all notifications in namespace `test` and is activated at `2022-02-29T00:00:00Z` for 24 hours.

```yaml
apiVersion: notification.kubesphere.io/v2beta2
kind: Silence
metadata:
  name: silence1
  labels:
    type: global
spec:
  matcher:
    matchExpressions:
      - key: namespace
        operator: In
        values:
        - test
  startsAt: "2022-02-29T00:00:00Z"
  duration: 24h
```

A silence that mutes all tenant notifications and is activated at 10 PM for 8 hours every day.

```yaml
apiVersion: notification.kubesphere.io/v2beta2
kind: Silence
metadata:
  name: silence1
  labels:
    type: global
spec:
  matcher:
    matchExpressions:
      - key: namespace
        operator: In
        values:
        - test
  schedule: "0 22 * * *"
  duration: 8h
```
