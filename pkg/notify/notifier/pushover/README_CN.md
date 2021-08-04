# Notification Manager 发送通知到 Pushover

## 项目背景

### Notification Manager

Notification Manager 用于在多租户 Kubernetes 环境下管理通知。Notification Manager 从不同的渠道接收告警消息，然后通过各种的渠道发送给租户。目前已实现的通知渠道包括钉钉，邮件，企业微信，Slack，Webhook。

### Pushover

Pushover 是用于发送和接收推送通知的平台。在服务器端，它提供了一个 HTTP API，用于接收通知消息，并传递到可通过用户或组寻址的设备。在设备方面，iOS，Android 和桌面客户端会接收这些推送通知，将其显示给用户，然后将其存储以供离线查看。

Pushover 使用简单的 RESTful API 版本来接收消息并将其广播到运行 Pushover 客户端的设备。使用 Pushover 的  API 可以很便捷的实现用户注册过和消息处理，Pushover 的 API 没有复杂的身份验证机制。可以使用几乎每种语言甚至命令行提供的标准  HTTP 库，而无需任何自定义模块或额外的依赖项。

本项目可以通过调用 Pushover API 来实现发送通知消息到 Pushover 用户设备。

### 项目目标与产出

项目目标是 Notification Manager 可以发送通知消息到 Pushover，然后由 Pushover 将通知推送给用户。

产出内容包括：

- 扩展 Notification Manager CRD 定义，使其支持 Pushover；
- 实现 Notification Manager 发送通知消息到 Pushover；
- 测试并输出测试报告；
- 编写相关文档。

## 项目构建

### 扩展 Notification Manager CRD 以支持 Pushover

#### Config CRD

为Config CRD添加Pushover的配置设定。根据 Pushover 服务的使用要求，需要先创建一个 Application 才可以启用 Pushover 推送渠道。在完成 Pushover APP 的创建之后，可以得到一个属于该应用的 API Token/Key。因此，对于 Pushover 的 Config CRD，需要给出 Pushover APP 的 API Token，并且该 Token 应该是以 加密形式存储的，以避免泄露从而被滥用。

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

在 PushoverConfig 中配置类型为 Credential 的 PushoverTokenSecret。根据 Credential 的定义，用户可以直接在配置 Config 的 YAML 文件中给出该Token（不推荐），也可以将其配置在 Kubernetes 的 Secret 资源中，再通过`name`和`key`指定要包含的 Secret 名称和键名来获取该Token。

此外，若用户选择启用Pushover作为通知渠道，则 Pushover APP 的 API Token 是必需项，因此要对PushoverTokenSecret属性进行验证，保证Config资源可以从 Value 或 ValueFrom 二者中至少一个位置获取该 Token。当 Value 不为空时，ValueFrom 不应该被指定。

#### Notification Manager CRD

为Notification Manager CRD添加Pushover的支持。对于 Pushover 消息的设置，需要给出超时时间`notificationTimeout`和消息模板`template`，如果未指定消息模板，则默认的模板会被启用。

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

为 Receiver CRD 添加 Pushover 消息接收方的配置。

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

接收方配置包括以下内容：

* Enabled：启用开关；
* PushoverConfigSelector：关联 Pushover Config 的选择器；
* AlertSelector：用于筛选告警的选择器；
* Profiles：
  * UserKey：必需项。用户的唯一标识符。每个 Pushover 用户都会被分配一个 user key，它相当于用户名。每一位想要接收 Pushover 消息的用户都要把他们的 user key 配置到这里；
  * Devices：选填。用户可以在此指定接收消息的设备名称，留空则该用户的所有设备均会收到消息；
  * Title：选填。消息的标题，不指定则默认为Pushover应用的名字；
  * Sound：选填。消息提示音，不为空则必须从[支持的声音类型](https://pushover.net/api#sounds)中挑选一个。

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

同样，如果 Pushover 通知渠道被启用，上述配置也需要被验证。主要是对user key的验证。首先，需要保证`Profiles`不能为空，即至少要有一名用户接收消息；其次要对 user key 的合法性进行验证，根据Pushover API文档，user key 是一个长度固定为30字符，由大小写字母或数字构成的字符串，因此这里采用正则表达式的方式，对每一个 user key 进行匹配，任何一个 user key 不符合此格式都是不被允许的。

### Pushover 消息推送功能实现

#### Pushover 消息结构体

根据文档可知，Pushover消息包含了多个选项。

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

必需字段：

* Token：Pushover 应用的 API token；
* UserKey：接收消息的用户user key；
* Message：文本消息。

此外还有很多选填字段，虽然有一部分暂未被使用，但可满足未来扩展的需要，它们是：

* Attachment：附件图片；
* Device：可以在此处指定接收用户所拥有的哪些设备才能接收到消息，多个设备时用逗号将它们分隔开，默认是接收用户的所有设备；
* Title：消息的标题，若留空则被应用名称填补；
* Url：消息携带的链接；
* UrlTitle：消息携带的链接的文本标题；
* Priority：消息优先级，从-2至2的整数，默认是0；
* Sound：声音，默认声音是`pushover`，有多种类型的声音可供选择，详见https://pushover.net/api#sounds；
* Timestamp：Unix时间戳，留空则是Pushover API收到消息的时间；
* Html：启用HTML渲染消息文本，与Monospace互斥；
* Monospace：启用Monospace渲染消息文本，与Html互斥。

如果Priority为最高级2，则代表这是一条紧急消息，需要接收方确认，如下字段也需要被考虑：

* Retry：Priority 为2时的必填项，指定了 Pushover 服务器向用户发送同一通知的频率（以秒为单位）。用户可能在一个嘈杂的环境中或正在睡觉的情况下，这时重试策略（带声音和振动）将有助于引起用户的注意。该参数的值至少要有30秒；
* Expire：Priority 为2时的必填项，指定发送的消息通知将继续重试多少秒（以秒为单位）。如果通知在`Expire`秒内没有被确认，则消息被标记为过期，并将停止向用户发送。请注意，通知过期后仍然会显示给用户，但不会提示用户确认。这个参数的最大值至多是10800秒（3小时）；
* Callback：选填项，可以提供一个可公开访问的URL，当用户确认消息时，Pushover 的服务器将向其发送一个请求。

#### Pushover 消息结构体的验证

在发送Pushover消息之前，这里还会对 Pushover 消息结构体进行校验。校验规则参考 Pushover 文档。当消息未通过验证时发出错误。每个字段的验证策略如下：

* Token：不可为空，且必须是一个长度固定为30字符，由大小写字母或数字构成的字符串；
* UserKey：不可为空，且必须是一个长度固定为30字符，由大小写字母或数字构成的字符串；
* Message：不可为空，且最大长度限制是1024个字符（Rune）；
* Device：如果指定了该字段，则每个设备名称的长度必须介于1和25之间，由大小写字母、数字、短横线（-）或者下划线（_）构成的字符串；
* Title：长度限制在250个字符（Rune）之内；
* Url：长度限制在512个字符（Rune）之内；
* UrlTitle：Url为空时该字段必须为空，且长度限制在100个字符（Rune）之内；
* Priority：必须是在-2至2之间的整数；
* Retry：Priority为2时不可为空；
* Expire：Priority为2时不可为空；
* Sound：若不为空则它必须是受支持的声音类型，详见https://pushover.net/api#sounds；

#### 消息推送流程与策略

消息推送流程会并行地把消息发送给每一位接收消息的租户所注册的每个user key。

发送消息前先对其进行预处理。首先，根据资源定义时给出的 AlertSelector 过滤得到有接收消息需求的租户。其次，因为每一条消息中可能会包含不止一条 Alert，又因为 Pushover 对消息长度存在1024个字符的限制（超出部分会被截断），所以这里采取拆分消息的策略，即为一条消息创建其所拥有的 Alert 数目的副本，然后每个副本只持有一条 Alert，对于这 Alert 数目条消息采取依次发送的策略，以确保它们能被用户完整地接收到。再其次，将消息套入转换成消息字符串，并由此构建 Pushover 消息结构体，对其合法性进行校验，若未通过校验则发出错误。最后，将 Pushover 消息结构体以 JSON 字符串编码，调用 HTTP 的POST方法向 Endpoint（https://api.pushover.net/1/messages.json）发出请求，当返回的响应的状态码不为成功时则发出错误。

## 项目测试

### 生成

测试已经编写的 Pushover 推送通知的功能的第一阶段包含两件事，一方面要生成扩展之后的 Notification Manager CRD 和 Notification Manager Operator，另一方面 CRD 的实现代码也要编译、打包成容器镜像、再上传至一个 Container Registry（这里选择 Docker Hub）供 Kubernetes 拉取。

先借助一个叫做 `controller-gen` 的工具来生成工具代码和 Kubernetes 的 YAML 对象，并通过另一款工具 `kustomize` 重新指定 Docker Hub 里将要存放镜像的目录。

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

然后用`docker build`和`docker push`命令打包并推送镜像至 Docker Hub。

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

### 安装

1. 部署上面生成的 CRD 和 Notification Manager Operator 至 Kubernetes 集群；

```sh
kubectl apply -f config/bundle.yaml
```

2. 部署 Notification Manager；

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
  image: <DOCKERHUB_USERNAME>/notification-manager:latest # 刚才推送镜像的地址
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
      pushover: # Pushover 设置
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

3. 部署 Template；

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

4. 部署 Pushover 的 Config 和 Receiver。User key 作为接收用户的标识要添加到Receiver里。此外 Pushover Application 的 Token 需要存放在 Secret 里，供 Config 读取。这里的Secret选用了Opaque类型，保存经 Base64 编码后的 Token；

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

至此相关资源已经部署完毕。

### 推送消息

Notification Manager使用端口`19093`和API路径`/api/v2/alerts`来接收发送的警报。先通过端口转发使得该 API 可以从集群外的本机调用，再使用项目下提供的 `alert.json` 文件作为告警进行测试。

```sh
kubectl port-forward service/notification-manager-svc 19093:19093 -n kubesphere-monitoring-system

# Another terminal
curl -XPOST -d @alert.json http://127.0.0.1:19093/api/v2/alerts
```

结果：

![pushover.png](https://i.loli.net/2021/07/31/J2ADI7dezqOjQKB.png)

## 参考

1. [Notification Manager](https://github.com/kubesphere/notification-manager)
2. [Pushover](https://pushover.net/)
3. [Pushover API](https://pushover.net/api)

