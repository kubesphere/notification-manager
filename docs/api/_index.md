
# Notification Manager HTTP API

The Notification Manager Deployment exposes an HTTP API for receiving alerts, sending notifications, and verifing config.

> The default port of Notification Manager is `19093`.

- [`Receive alerts`](#Receive-alerts)
- [`Send notifications`](#Send-notifications)
- [`Verify`](#Verify)

## Receive alerts

> Post /api/v2/alerts

This API is used to send alerts to Notification Manager. Notification Manager will then send notifications to the receivers defined in the cluster.

Request:

```
{
  "receiver": "Critical",
  "status": "firing",
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
        "runbook_url": "https://github.com/kubernetes-monitoring/kubernetes-mixin/tree/master/runbook.md#alert-name-kubepodcrashlooping"
      },
      "startsAt": "2020-02-26T07:05:04.989876849Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "http://prometheus-k8s-0:9090/graph?g0.expr=rate%28kube_pod_container_status_restarts_total%7Bjob%3D%22kube-state-metrics%22%7D%5B15m%5D%29+%2A+60+%2A+5+%3E+0\u0026g0.tab=1",
      "fingerprint": "a4c6c4f7a49ca0ae"
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
    "runbook_url": "https://github.com/kubernetes-monitoring/kubernetes-mixin/tree/master/runbook.md#alert-name-kubepodcrashlooping"
  },
  "externalURL": "http://alertmanager-main-2:9093"
}
```

> You can find the details about the request at [here](https://github.com/prometheus/alertmanager/blob/main/template/template.go#L231).

Response:

```
{
  "Status":200,
  "Message":"Notification request accepted"
}
```

> Status code `200` means Notification Manager has received the alerts and begins to send notifications, it doesn't mean the notifications have been sent successfully.

## Send notifications

> Post /api/v2/notifications

This API is used to send notifications directly.

Request:

```
{
  "config":{
    "apiVersion":"notification.kubesphere.io/v2beta2",
    "kind":"Config",
    "metadata":{
      "name":"test-user-config",
      "labels":{
        "app":"notification-manager",
        "type":"default",
      }
    },
    "spec":{
      "email":{
        "authPassword":{
          "value": <password>
        },
        "authUsername": <user>,
        "from": <email>,
        "requireTLS":true,
        "smartHost":{
          "host": <smtp-host>,
          "port": <smtp port>
        }
      }
    }
  },
  "receiver":{
    "apiVersion":"notification.kubesphere.io/v2beta2",
    "kind":"Receiver",
    "metadata":{
      "name":"test-user-receiver",
      "labels":{
        "app":"notification-manager",
        "type":"global",
      }
    },
    "spec":{
      "email":{
        "to":[
          <email>
        ]
      }
    }
  },
  "alert": {
    "receiver": "Critical",
    "status": "firing",
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
          "runbook_url": "https://github.com/kubernetes-monitoring/kubernetes-mixin/tree/master/runbook.md#alert-name-kubepodcrashlooping"
        },
        "startsAt": "2020-02-26T07:05:04.989876849Z",
        "endsAt": "0001-01-01T00:00:00Z",
        "generatorURL": "http://prometheus-k8s-0:9090/graph?g0.expr=rate%28kube_pod_container_status_restarts_total%7Bjob%3D%22kube-state-metrics%22%7D%5B15m%5D%29+%2A+60+%2A+5+%3E+0\u0026g0.tab=1",
        "fingerprint": "a4c6c4f7a49ca0ae"
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
      "runbook_url": "https://github.com/kubernetes-monitoring/kubernetes-mixin/tree/master/runbook.md#alert-name-kubepodcrashlooping"
    },
    "externalURL": "http://alertmanager-main-2:9093"
  }
}
```

- `config`: The corresponding config for the `receiver`. If `config` is not set, the `receiver` will use the default config defined in the cluster. If the `config` is set, the config type must match the receiver type.
- `receiver`: Receiver that receives notifications. It must be set.
- `alert`: Alerts that will be sent to the receiver.

Response:

```
{
  "Status":200,
  "Message":"Send alerts successfully"
}
```

## Verify

> Post /api/v2/verify

This API used to verify the config and receiver are correct. If the config and receiver are correct, receiver will receive a test notification.

Request:

```
{
  "config":{
    "apiVersion":"notification.kubesphere.io/v2beta2",
    "kind":"Config",
    "metadata":{
      "name":"test-user-config",
      "labels":{
        "app":"notification-manager",
        "type":"default",
      }
    },
    "spec":{
      "email":{
        "authPassword":{
          "value": <password>
        },
        "authUsername": <user>,
        "from": <email>,
        "requireTLS":true,
        "smartHost":{
          "host": <smtp-host>,
          "port": <smtp port>
        }
      }
    }
  },
  "receiver":{
    "apiVersion":"notification.kubesphere.io/v2beta2",
    "kind":"Receiver",
    "metadata":{
      "name":"test-user-receiver",
      "labels":{
        "app":"notification-manager",
        "type":"global",
      }
    },
    "spec":{
      "email":{
        "to":[
          <email>
        ]
      }
    }
  }
}
```

- `config`: The corresponding config for the `receiver`. If `config` is not set, the `receiver` will use the default config defined in the cluster. If the `config` is set, the config type must match the receiver type.
- `receiver`: Receiver that receives notifications. It must be set.

Response:

```
{
  "Status":200,
  "Message":"Verify successfully"
}
```