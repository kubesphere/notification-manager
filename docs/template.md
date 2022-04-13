# Template

## Overview

The notifications are sent to receivers constructed via templates. The Notification Manager comes with default templates which also support customization.
User can define global template and define template for receiver.

## Add global template

User can define global template by configuring Notification Manager CR like this.

```yaml
apiVersion: notification.kubesphere.io/v2beta2
kind: NotificationManager
metadata:
  name: notification-manager
spec:
  template:
    reloadCycle: 1m
    text:
      name: notification-manager-template
      namespace: kubesphere-monitoring-system
```

The `template.text` is a `ConfigmapKeySelector` that specifying a configmap where the template text in. 
Notification manager webhook will reload the template text on every `template.reloadCycle`.

A `ConfigmapKeySelector` allows user to define:

- `name` - The name of the configmap.
- `naemspace` - The namespace of the configmap, if not set, [DefaultSecretNamespace](./crds/notification-manager.md#DefaultSecretNamespace) will be used.
- `key` - The key of the configmap, if not set, all files in configmap are parsed as template file.

## Add receiver template

User can define template for receiver like this.

```yaml
apiVersion: notification.kubesphere.io/v2beta2
kind: Receiver
metadata:
  name: global-receiver
  labels:
    type: tenant
    user: admin
spec:
  wechat:
    tmplText:
      name: notification-manager-template
      namespace: kubesphere-monitoring-system
```

The `template.text` is a `ConfigmapKeySelector` that specifying a configmap where the template text in. 

## How to use template

A template may like this.

```
    {{ define "nm.default.text" }}{{ range .Alerts }}{{ template "nm.default.message" . }}
    {{ range .Labels.SortedPairs }}  {{ .Name | translate }}: {{ .Value }}
    {{ end }}
    {{ end }}{{- end }}
```

The `nm.default.text` is the name of template. A receiver can be set like this to use this template.

```yaml
apiVersion: notification.kubesphere.io/v2beta2
kind: Receiver
metadata:
  name: global-receiver
  labels:
    type: tenant
    user: admin
spec:
  wechat:
    template: nm.default.text
```

We can also set all WeChat receivers to use this template like this.

```yaml
apiVersion: notification.kubesphere.io/v2beta2
kind: NotificationManager
metadata:
  name: notification-manager
spec:
  receivers:
    options:
      wechat:
        template: nm.default.text
```

We can also set all receivers to use this template like this.

```yaml
apiVersion: notification.kubesphere.io/v2beta2
kind: NotificationManager
metadata:
  name: notification-manager
spec:
  receivers:
    options:
      global:
        template: nm.default.text
```

The priority of these templates is:

```
default template < global template < template for each type of receivers < receiver template
```

The default template of receivers.

|          | text type | default template        | default subject template |
|----------|-----------|-------------------------|--------------------------|
| dingtalk | markdown  | nm.default.markdown     | nm.default.subject       |
| &nbsp;   | text      | nm.default.text         | &nbsp;                   |
| email    | html      | nm.default.html         | nm.default.subject       |
| &nbsp;   | text      | nm.default.text         | &nbsp;                   |
| feishu   | post      | nm.feishu.post          | &nbsp;                   |
| &nbsp;   | text      | nm.feishu.text          | &nbsp;                   |
| pushover | &nbsp;    | nm.default.text         | nm.default.subject       |
| sms      | &nbsp;    | nm.default.text         | &nbsp;                   |
| slack    | &nbsp;    | nm.default.text         | &nbsp;                   |
| wechat   | markdown  | nm.default.markdown     | &nbsp;                   |
| &nbsp;   | text      | nm.default.text         | &nbsp;                   |
| webhook  | &nbsp;    | webhook.default.message | &nbsp;                   |

> You can find these default templates at [here](../config/samples/template.yaml).

> `webhook.default.message` is a special template, it will not generate notifications, 
> notification manager will serialize the data to json and send it to webhook.

## Customize template

The notification template is based on the [Go templating](https://pkg.go.dev/text/template) system.
So we need to know what is the data structure passed to notification template firstly.

### Data

`Data` is the structure passed to notification template.

```yaml
{
  "alerts": [
    {
      "status": "firing",
      "labels": {
        "alertname": "KubePodCrashLooping",
        "container": "busybox-3jb7u6",
        "instance": "10.233.71.230:8080",
        "job": "kube-state-metrics",
        "namespace": "pp1",
        "pod": "dd1-0",
        "prometheus": "kubesphere-monitoring-system/k8s",
        "severity": "critical"
      },
      "annotations": {
        "message": "Pod pp1/dd1-0 (busybox-3jb7u6) is restarting 1.07 times / 5 minutes.",
      },
      "startsAt": "2020-02-26T07:05:04.989876849Z",
      "endsAt": "0001-01-01T00:00:00Z",
    }
  ],
  "groupLabels": {
    "alertname": "KubePodCrashLooping",
    "namespace": "pp1"
  },
  "commonLabels": {
    "alertname": "KubePodCrashLooping",
    "container": "busybox-3jb7u6",
    "instance": "10.233.71.230:8080",
    "job": "kube-state-metrics",
    "namespace": "pp1",
    "pod": "dd1-0",
    "prometheus": "kubesphere-monitoring-system/k8s",
    "severity": "critical"
  },
  "commonAnnotations": {
    "message": "Pod pp1/dd1-0 (busybox-3jb7u6) is restarting 1.07 times / 5 minutes.",
  },
}
```

| Name              | Type            | Notes                                           | 
|-------------------|-----------------|-------------------------------------------------|
| Alerts            | [Alert](#Alert) | List of alert.                                  |
| GroupLabels       | [KV](#KV)       | Labels used to group alerts in `Alerts`.        |
| CommonLabels      | [KV](#KV)       | The labels common to all of the alerts.         |
| CommonAnnotations | [KV](#KV)       | The annotations common to all of the alerts. |

The `Data` type exposes a function for getting status.

- `Status` returns the status of `Alerts`. if there has a firing alert in `Alerts`, it returns `firing`, else it returns `resolved`.

The `Alerts` type exposes functions for filtering alerts:

- `Firing` returns a list of currently firing alert objects in `Alerts`.
- `Resolved` returns a list of resolved alert objects in `Alerts`.

#### Alert

`Alert` holds one alert for notification templates.

| Name        | Type      | Notes                                          | 
|-------------|-----------|------------------------------------------------|
| Status      | string    | The status of the alert, firing or resolved.   |
| Labels      | [KV](#KV) | A set of labels to for the alert.              |
| Annotations | [KV](#KV) | A set of annotations for the alert.            |
| StartsAt    | time.Time | The time the alert started firing.             |
| EndsAt      | time.Time | Only set if the end time of an alert is known. |

The `Alert` type exposes functions for getting message of alert:

- `Message` returns the `message` in `Annotations`, if `message` is not set, `summary` in `Annotations` will be used, else `summaryCn` in `Annotations` will be used.
- `MessageCN` returns the `summaryCn` in `Annotations`, if `summaryCn` is not set, `message` in `Annotations` will be used, else `summary` in `Annotations` will be used.

#### KV

`KV` is a set of key/value string pairs used to represent labels and annotations.

```shell
  type KV map[string]string
```

The `KV` type exposes these functions:

| Name        | Arguments | Returns         | Notes                                                       | 
|-------------|-----------|-----------------|-------------------------------------------------------------|
| SortedPairs | &nbsp;    | [Pairs](#Pairs) | Returns a sorted list of key/value pairs.                   |
| Remove      | []string  | KV              | Returns a copy of the key/value map without the given keys. |
| Names       | &nbsp;    | []string        | Returns the names of the label names in the LabelSet.       |
| Values      | &nbsp;    | []string        | Returns a list of the values in the LabelSet.               |

#### Pairs 

`Pairs` list of key/value string pairs.

```shell
type Pair struct {
	Name, Value string
}

type Pairs []Pair
```

The `pairs` type exposes these functions:

| Name        | Returns  | Notes                                                 | 
|-------------|----------|-------------------------------------------------------|
| Names       | []string | Returns the names of the label names in the LabelSet. |
| Values      | []string | Returns a list of the values in the LabelSet.         |


### Functions

In addition to the [built-in functions](https://pkg.go.dev/text/template#hdr-Functions) of go templating, notification manager also has the following functions:

| Name         | Arguments                   | Returns | Notes                                                                    | 
|--------------|-----------------------------|---------|--------------------------------------------------------------------------|
| toUpper      | string                      | string  | Converts all characters to upper case.                                   |
| toLower      | string                      | string  | Converts all characters to lower case.                                   |
| join         | sep string, s []string      | string  | Similar to `strings.Join`.                                               |
| match        | pattern, string             | bool    | Match a string using Regexp.                                             |
| reReplaceAll | pattern, replacement, text  | string  | Regexp substitution.                                                     |
| translate    | string                      | string  | Translate the string, see [Multilingual support](#Multilingual-support). |

## Multilingual support

Notification manager supports language customization.

```yaml
apiVersion: notification.kubesphere.io/v2beta2
kind: NotificationManager
metadata:
  name: notification-manager
spec:
  template:
    language: zh-cn
    languagePack:
      - name: zh-cn
        namespace: kubesphere-monitoring-system
```
The `languagePack` is a list of `ConfigmapKeySelector`. The configmap is like this.

```yaml
apiVersion: v1
data:
  zh-cn: |
    - name: zh-cn
      dictionary:
        alert: "告警"
kind: ConfigMap
metadata:
  name: zh-cn
  namespace: kubesphere-monitoring-system
```

The `name` if language name, the `dictionary` containing all the words to be translated. Users can define dictionaries in any language.
The user can switch the notification language by changing the `template.language`.

User can translate a word by call `translate` function in the template like this.

```yaml
{{ .Status | translate }}
```