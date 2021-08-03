# Support Send Notifications To Pushover

## Background

### Notification Manager

Notification Manager manages notifications in multi-tenant Kubernetes environments. Notification Manager receives alerts from senders and sends them to the tenant through various receivers, including DingTalk, Email, WeCom, Slack, Webhook, etc.

### Pushover

Pushover is a service to receive instant push notifications on phones or tablets from a variety of sources.  Pushover server provides HTTP APIs to receive notifications and send them to devices accessible by users or groups. Devices including iOS, Android and desktop clients are able to receive these notifications, display them to users and store them for offline review.

Pushover receives messages and broadcasts them to Pushover clients on devices through simple RESTful APIs. They also provides user-friendly registration and message handling methods, without complex authentication. Pushover APIs can be utilized with standard HTTP libraries which are available in almost every language, even in the terminals.

The project aims to send notification messages to Pushover user devices by calling the Pushover API.

### Objectives and Outcomes

Notification Manager can send alerts to Pushover, then Pushover pushes them to users who subscribed. Specifically, this includes:

- Extended Notification Manager CRD to support Pushover.
- Enabling Notification Manager to send alerts via Pushover.
- Tests.
- Documents.

## Code

### Support Send Notifications To Pushover

#### Config CRD

According to Pushover services specification, it is required to create an Application before you can enable the Pushover push messages. This application comes with API Token. In terms of Config CRD of Pushover, the API Token should be given, and the Token should be stored in encrypted manner to avoid leakage and abuse.

```go
type PushoverConfig struct {
   Labels map[string]string `json:"labels,omitempty"`
   // The token of a pushover application.
   PushoverTokenSecret *Credential `json:"pushoverTokenSecret"`
}

type Credential struct {
	Value     string       `json:"value,omitempty" protobuf:"bytes,2,opt,name=value"`
	ValueFrom *ValueSource `json:"valueFrom,omitempty" protobuf:"bytes,3,opt,name=valueFrom"`
}
```

Configure `PushoverTokenSecret` of type `Credential` in `PushoverConfig`. According to the definition of Credential, you can either give the Token explicitly in the Config YAML file (not recommended), or you can configure it in the Secret resource of Kubernetes, and get the Token by referencing the `name` and `key` of that Secret. `PushoverTokenSecret` should be able to retrieve API Token from either `Value` or `ValueFrom`, yet `ValueFrom` should not be configured when `Value` is not empty.

#### Notification Manager CRD

Properties `notificationTimeout` and `template` are configured. Both have their default values.

```go
type PushoverOptions struct {
   // Notification Sending Timeout
   NotificationTimeout *int32 `json:"notificationTimeout,omitempty"`
   // The name of the template to generate pushover message.
   // If the global template is not set, it will use default.
   Template string `json:"template,omitempty"`
}
```

#### Receiver CRD

```go
// PushoverUserProfile includes userKey and other preferences
type PushoverUserProfile struct {
    // UserKey is the user (Pushover User Key) to send notifications to.
    UserKey *string `json:"userKey"`
    // Devices refers to device name to send the message directly to that device, rather than all of the user's devices
    Devices []string `json:"devices,omitempty"`
    // Title refers to message's title, otherwise your app's name is used.
    Title *string `json:"title,omitempty"`
    // Sound refers to the name of one of the sounds (https://pushover.net/api#sounds) supported by device clients
    Sound  *string `json:"sound,omitempty"`
}

type PushoverReceiver struct {
    // whether the receiver is enabled
    Enabled *bool `json:"enabled,omitempty"`
    // PushoverConfig to be selected for this receiver
    PushoverConfigSelector *metav1.LabelSelector `json:"pushoverConfigSelector,omitempty"`
    // Selector to filter alerts.
    AlertSelector *metav1.LabelSelector `json:"alertSelector,omitempty"`
    // The name of the template to generate DingTalk message.
    // If the global template is not set, it will use default.
    Template             *string `json:"template,omitempty"`
    // The users profile.
    Profiles []*PushoverUserProfile `json:"profiles,omitempty"`
}
```

This includes:

* Enabled: Pushover receiver switch.
* PushoverConfigSelector: Selector that associates the Pushover Config.
* AlertSelector: Selector that filters alerts；
* Profiles: 
  * UserKey: Required. A unique identifier for the user. Each Pushover user is assigned a user key, same as an username. Each user who intends to receive alerts via Pushover will have to configure their user key here.
  * Devices: Optional. Device names to send the message directly to that device, rather than all of the user's devices.
  * Title: Optional. Message's title, otherwise your app's name is used.
  * Sound: Optional. Sound refers to the name of one of the [sounds](https://pushover.net/api#sounds) supported by device clients

```go
if r.Spec.Pushover != nil {
		// validate User Profile
		if len(r.Spec.Pushover.Profiles) == 0 {
			// err ...
		} else {
			// requirements
			tokenRegex := regexp.MustCompile(`^[A-Za-z0-9]{30}$`)
			deviceRegex := regexp.MustCompile(`^[A-Za-z0-9_-]{1,25}$`)
			sounds := map[string]bool{"pushover": true, "bike": true, ... , "vibrate": true, "none": true}
			// validate each profile
			for i, profile := range r.Spec.Pushover.Profiles {
				// validate UserKeys
				if profile.UserKey == nil || !tokenRegex.MatchString(*profile.UserKey) {
					// err ...
				}
				// validate Devices
				for _, device := range profile.Devices {
					if !deviceRegex.MatchString(device) {
						// err ...
					}
				}
				// Validate Title
				if profile.Title != nil {
					if l := utf8.RuneCountInString(*profile.Title); l > 250 {
						// err ...
					}
				}
				// Validate Sound
				if profile.Sound != nil {
					if !sounds[*profile.Sound] {
						// err ...
					}
				}
			}
		}
	}
```

If Pushover receiver is enabled, configures above should be validated, especially the `UserKeys`. First, ensure that `Profiles` cannot be empty, i.e., there must be at least one user receiving the message; second, verify the legitimacy of the user key, which is a string of upper and lower case letters or numbers with a fixed length of 30 characters, according to the Pushover API documentation. A regular expression is applied. Any user key that does not match this format is not allowed.

### Implementation

#### Message struct

According to the documentation, Pushover offers a number of options in a message struct.

```go
// Pushover message struct
type pushoverRequest struct {
    // required fields
    // Token is a Pushover application API token, required.
    Token string `json:"token"`
    // UserKey is recipient's Pushover User Key, required.
    UserKey string `json:"user"`
    // Message is your text message, required.
    Message string `json:"message"`
    
    // common optional fields
    // Device specifies a set of user's devices to send the message; all would be sent if empty
    Device string `json:"device,omitempty"`
    // Title is the message's title, otherwise application's name is used.
    Title string `json:"title,omitempty"`
    // Sound is the name of one of the sounds supported by device clients.
    Sound string `json:"sound,omitempty"`
}
```

Required fields:

* Token: the Pushover application's API token.
* UserKey: a user receipt, viewable in user's dashboard.
* Message: text message.

There are also a number of optional fields that parts of them are not being used at this time, but could be used to meet the future needs, they are:

* Attachment: an image file.
* Device: user's device name to send the message directly to that device, rather than all of the user's devices (multiple devices may be separated by a comma).
* Title: message's title, otherwise your app's name is used.
* Url: a [supplementary URL](https://pushover.net/api#urls) to show with the message.
* UrlTitle: a title for the supplementary URL, otherwise just the URL is shown.
* Priority: send as `-2` to generate no notification/alert, `-1` to always send as a quiet notification, `1` to display as high-priority and bypass the user's quiet hours, or `2` to also require confirmation from the user.
* Sound: the name of one of the [sounds](https://pushover.net/api#sounds) supported by device clients.
* Timestamp: a Unix timestamp of the message's date and time.
* Html: enable HTML rendering of message text, mutually exclusive with Monospace.
* Monospace: enable Monospace to render the message text, mutually exclusive with Html.

If Priority is the highest level `2`, it means that this is an urgent message that needs to be acknowledged by the receiver. Thus, the following fields also need to be considered:

* Retry: specifies how often (in seconds) the Pushover servers will send the same notification to the user. This parameter must have a value of at least `30` seconds between retries.
* Expire: specifies how many seconds your notification will continue to be retried for (every `retry` seconds). This parameter must have a maximum value of at most `10800` seconds (3 hours).
* Callback: optional, may be supplied with a publicly-accessible URL that our servers will send a request to when the user has acknowledged your notification.

#### Pushover message validation

The message would be validated before sending it. The validation rules are referred to the documentation. An error is raised when the message does not pass this validation. The rules are:

* Token: required, should be case-sensitive, 30 characters long, and may contain the character set `[A-Za-z0-9]`.
* UserKey: required, should be case-sensitive, 30 characters long, and may contain the character set `[A-Za-z0-9]`.
* Message: required, limited to `1024` 4-byte UTF-8 characters (runes).
* Device: optional. If specified, may be up to 25 characters long, and will contain the character set `[A-Za-z0-9_-]`.
* Title: limited to `250` 4-byte UTF-8 characters (runes).
* Url: limited to `512` 4-byte UTF-8 characters (runes).
* UrlTitle: limited to `1024` 4-byte UTF-8 characters (runes), and should not be set when Url is empty.
* Priority: should be an `int` type value between `-2` and `2`.
* Retry: required when Priority=2.
* Expire: required when Priority=2.
* Sound: If it is not empty, it must be a supported sound type. See also https://pushover.net/api#sounds.

#### Message push process and strategy

The message push sends messages in parallel to each user registered by each tenant that registered Pushover.

Preprocess the message before sending it. First, the tenants that subscribed Pushover messages are selected based on the AlertSelector given at the time of resource definition. Second, Pushover has a limit of 1024 characters on the message length (the exceeded part will be truncated), and each message may contain more than one Alert. Thus, a strategy of splitting the message is applied here, i.e., a message should contain as many Alerts as possible, and each message is sent one after another to ensure that they can be received in an intact manner by the user. Third, fit the message to a template, from which the Pushover message structure is constructed and its legitimacy is verified. Finally, the Pushover message structure is encoded as a JSON string, a POST method is called to send a request to the Endpoint (https://api.pushover.net/1/messages.json), and an error will be raised if the status code of the returned response is not successful.

In addition, the returned response header `X-Limit-App-Remaining` carries information about the limit, which refers to the remaining number of messages the application can send this month. If the number of messages that can be sent in a month is less than 100, the application prints a warning log with the number of messages left.

## Test

### Generate CRDs

On the one hand, the CRDs of extended Notification Manager and Notification Manager Operator have to be generated. On the other hand, the CRD implementation code has to be built, packaged into a container image, and then pushed to a Container Registry (Docker Hub in this case) for Kubernetes Pods to pull.

First, we use a tool called `controller-gen` to generate a series of tool code and Kubernetes YAML objects, and another tool called `kustomize` to re-designate the directory in Docker Hub where the images will be deposited.

```makefile
# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./pkg/apis/v2beta1"
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./pkg/apis/v2beta2"

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=controller-role webhook paths=./pkg/apis/v2beta1 paths=./pkg/apis/v2beta2 output:crd:artifacts:config=config/crd/bases
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=controller-role webhook paths=./pkg/apis/v2beta1 paths=./pkg/apis/v2beta2 output:crd:artifacts:config=helm/crds
	cd config/manager && kustomize edit set image controller=${IMG} && cd ../../
	kustomize build config/default | sed -e '/creationTimestamp/d' > config/bundle.yaml
	kustomize build config/samples | sed -e '/creationTimestamp/d' > config/samples/bundle.yaml
```

Build & Push both images to Docker Hub.

```makefile
# Build the docker image for amd64 and arm64
build-op: test
	docker buildx build --push --platform linux/amd64,linux/arm64 -f cmd/operator/Dockerfile . -t ${IMG}

# Build the docker image for amd64 and arm64
build-nm: test
	docker buildx build --push --platform linux/amd64,linux/arm64 -f cmd/notification-manager/Dockerfile . -t ${NM_IMG}

# Push the docker image
push-amd64:
	docker push ${IMG}
	docker push ${NM_IMG}
```

### Install

1. Deploy generated CRDs and Notification Manager Operator to a Kubernetes cluster.

```sh
kubectl apply -f config/bundle.yaml
```

2. Deploy Notification Manager；

```yaml
apiVersion: notification.kubesphere.io/v2beta2
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
  image: <DOCKERHUB_USERNAME>/notification-manager:latest # Docker hub repo
  imagePullPolicy: Always
  serviceAccountName: notification-manager-sa
  portName: webhook
  defaultConfigSelector:
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
      global:
        templateFile:
        - /etc/notification-manager/template
      email:
        notificationTimeout: 5
        deliveryType: bulk
        maxEmailReceivers: 200
      wechat:
        notificationTimeout: 5
      slack:
        notificationTimeout: 5
      pushover: # Pushover settings
        notificationTimeout: 5
  volumeMounts:
  - mountPath: /etc/notification-manager/
    name: notification-manager-template
  volumes:
  - configMap:
      defaultMode: 420
      name: notification-manager-template
    name: notification-manager-template
```

3. Deploy Template；

```YAML
apiVersion: v1
data:
  template: |2
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

    {{ define "nm.default.html" }}
      <html>
      ......
      </html>
    {{ end }}
kind: ConfigMap
metadata:
  name: notification-manager-template
  namespace: kubesphere-monitoring-system
```

4. Deploy the Config and Receiver of Pushover. UserKeys should be add to Receiver as recipients. In addition, the Pushover Application's token needs to be stored in the Secret for Config to read. The Secret here is of type Opaque, which holds the Base64 encoded Token.

```yaml
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
          name: pushover-token-secret
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
      - userKey: uzggr3m9kw2r5m7im5aicwm1j*****
        title: "test title"
        sound: "bike"
        devices: ["iphone"] # only the user's device called "iphone" can receive messages
---
apiVersion: v1
data:
  token: YXZyZ3BhYjJxZ2I0cnpobTN1bjgyNm83Nm******
kind: Secret
metadata:
  labels:
    app: notification-manager
  name: pushover-token-secret
  namespace: kubesphere-monitoring-system
type: Opaque
```

### Push messages

Notification Manager uses port `19093` and API path `/api/v2/alerts` to receive alerts sent from Alertmanager. The API is first called from a local machine via port forwarding, then tested with `alert.json` files provided under the project as alerts.

```sh
kubectl port-forward service/notification-manager-svc 19093:19093 -n kubesphere-monitoring-system

# Another terminal
curl -XPOST -d @alert.json http://127.0.0.1:19093/api/v2/alerts
```

Result:

![pushover.png](https://i.loli.net/2021/07/31/J2ADI7dezqOjQKB.png)

## Reference

1. [Notification Manager](https://github.com/kubesphere/notification-manager)
2. [Pushover](https://pushover.net/)
3. [Pushover API](https://pushover.net/api)

