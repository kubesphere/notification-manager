package constants

const (
	HTML     = "html"
	Text     = "text"
	Markdown = "markdown"
	// Post message is Rich Text Format(RTF).  RTF is a file format that lets you exchange text
	// files between different word processors in different operating systems.
	// More info: https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/im-v1/message/create_json#45e0953e
	Post    = "post"
	Aliyun  = "aliyun"
	Tencent = "tencent"
	AWS     = "aws"

	DingTalk = "dingtalk"
	Email    = "email"
	Feishu   = "feishu"
	Pushover = "pushover"
	Slack    = "slack"
	SMS      = "sms"
	Webhook  = "webhook"
	WeChat   = "wechat"
	Discord  = "discord"
	Telegram = "telegram"

	DiscordContent = "content"
	DiscordEmbed   = "embed"

	Cluster   = "cluster"
	Namespace = "namespace"

	AlertFiring   = "firing"
	AlertResolved = "resolved"

	AlertName      = "alertname"
	AlertType      = "alerttype"
	AlertTime      = "alerttime"
	AlertMessage   = "message"
	AlertSummary   = "summary"
	AlertSummaryCN = "summaryCn"

	ReceiverName = "receiver"
	ReceiverType = "receiver_type"

	Verify       = "verify"
	Notification = "notification"

	DefaultWebhookTemplate = `{{ template "webhook.default.message" . }}`
	DefaultHistoryTemplate = `{{ template "nm.default.history" . }}`

	DefaultClusterName = "default"

	KubesphereConfigNamespace = "kubesphere-system"
	KubesphereConfigName      = "kubesphere-config"
	KubesphereConfigKey       = "kubesphere.yaml"
)
