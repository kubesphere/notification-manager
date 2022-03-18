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

	DingTalk = "dingtalk"
	Email    = "email"
	Feishu   = "feishu"
	Pushover = "pushover"
	Slack    = "slack"
	SMS      = "sms"
	Webhook  = "webhook"
	WeChat   = "wechat"

	Namespace = "namespace"

	AlertFiring   = "firing"
	AlertResolved = "resolved"

	AlertName      = "alertname"
	AlertType      = "alertype"
	AlertTime      = "alertime"
	AlertMessage   = "message"
	AlertSummary   = "summary"
	AlertSummaryCN = "summaryCn"

	Verify       = "verify"
	Notification = "notification"

	DefaultWebhookTemplate = `{{ template "webhook.default.message" . }}`
)
