package constants

const (
	HTML     = "html"
	Text     = "text"
	Markdown = "markdown"
	Post     = "post"
	Aliyun   = "aliyun"
	Tencent  = "tencent"

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
