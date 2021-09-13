package sms

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kubesphere/notification-manager/pkg/apis/v2beta2"
	"github.com/kubesphere/notification-manager/pkg/config"
	"github.com/kubesphere/notification-manager/pkg/utils"
	"k8s.io/apimachinery/pkg/util/uuid"
)

const (
	HuaweiMaxPhoneNums = 200
	DefaultUrl         = "https://rtcsms.cn-north-1.myhuaweicloud.com:10743/sms/batchSendSms/v1"
)

var (
	allowedTemplateKeys = []string{"alertname", "severity", "message", "summary", "alerttype", "cluster", "node", "namespace", "workload", "pod"}
)

type HuaweiNotifier struct {
	Signature     string
	NotifierCfg   *config.Config
	TemplateId    string
	AppKey        *v2beta2.Credential
	AppSecret     *v2beta2.Credential
	PhoneNums     string
	Sender        string
	TemplateParas string
	Url           string
}

type HuaweiResponse struct {
	Code        string `json:"code,omitempty"`
	Description string `json:"description,omitempty"`
}

func NewHuaweiProvider(c *config.Config, providers *v2beta2.Providers, phoneNumbers []string) Provider {
	phoneNum := handleHuaweiPhoneNums(phoneNumbers)
	u := handleHuaweiUrl(providers.Huawei.Url)
	return &HuaweiNotifier{
		Signature:   providers.Huawei.Signature,
		NotifierCfg: c,
		TemplateId:  providers.Huawei.TemplateId,
		AppKey:      providers.Huawei.AppKey,
		AppSecret:   providers.Huawei.AppSecret,
		PhoneNums:   phoneNum,
		Sender:      providers.Huawei.Sender,
		Url:         u,
	}
}

func (h *HuaweiNotifier) MakeRequest(_ context.Context, messages string) error {
	appKey, err := h.NotifierCfg.GetCredential(h.AppKey)
	if err != nil {
		return utils.Errorf("[Huawei SendSms]cannot get appKey: %s", err.Error())
	}
	appSecret, err := h.NotifierCfg.GetCredential(h.AppSecret)
	if err != nil {
		return utils.Errorf("[Huawei SendSms]cannot get appSecret: %s", err.Error())
	}

	m := strings.Join(generateParams(messages), ",")
	// body
	body := strings.NewReader(url.Values{
		"from":           {h.Sender},
		"to":             {h.PhoneNums},
		"templateId":     {h.TemplateId},
		"templateParas":  {"[" + m + "]"},
		"signature":      {h.Signature},
		"statusCallback": {""},
	}.Encode())

	// xheader
	xheader := buildWSSEHeader(appKey, appSecret)

	// client
	client := &http.Client{}
	req, err := http.NewRequest("POST", h.Url, body)
	if err != nil {
		return utils.Errorf("[Huawei SendSms] Create request failed: %s", err.Error())
	}
	req.Header.Set("Authorization", `WSSE realm="SDP",profile="UsernameToken",type="Appkey"`)
	req.Header.Set("X-WSSE", xheader)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return utils.Errorf("[Huawei SendSms] An API error occurs: %s", err.Error())
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	res, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return utils.Errorf("[Huawei  SendSms] read body failed: %s", err.Error())
	}

	var r HuaweiResponse
	if err = json.Unmarshal(res, &r); err != nil {
		return utils.Errorf("[Huawei  SendSms] unmarshal failed: %s", err.Error())
	}
	if r.Code != "000000" {
		return utils.Errorf("[Huawei  SendSms] send failed: %s, code: %s, raw messages: %s, handled messages: %s", r.Description, r.Code, messages, m)
	}
	return nil
}

func buildWSSEHeader(appKey string, appSecret string) string {
	now := time.Now().Format("2006-01-02T15:04:05Z")
	nonce := strings.ReplaceAll(string(uuid.NewUUID()), "-", "")
	sha256 := getSha256Code(nonce + now + strings.ReplaceAll(appSecret, "\n", ""))
	passwordDigest := base64.StdEncoding.EncodeToString([]byte(sha256))
	return fmt.Sprintf(`UsernameToken Username="%s",PasswordDigest="%s",Nonce="%s",Created="%s"`, strings.ReplaceAll(appKey, "\n", ""), passwordDigest, nonce, now)
}

// Note here, pls make your custom SMS template containing ten placeholders(the keys stored in the field allowedTemplateKeys).
// BTW, you can create an SMS template in Huawei Cloud's SMS console.
// i.e: Received notifications: alertname=${TEXT}, alerttype=${TEXT}, message=${TEXT}, summary=${TEXT}... (10 placeholders).
func generateParams(messages string) []string {
	// sample: 1 alert for  alertname=test  rule_id=d556b8d429c631f8 \nAlerts Firing:\nLabels:\n- alertname = test\n- alerttype = metric\n- cluster = default\n- host_ip = 192.168.88.6\n- node = node1\n- role = master\n- rule_id = d556b8d429c631f8\n- severity = critical\nAnnotations:\n- kind = Node\n- message = test message\n- resources = [\"node1\"]\n- rule_update_time = 2021-07-27T13:48:32Z\n- rules = [{\"_metricType\":\"node:node_memory_utilisation:{$1}\",\"condition_type\":\">=\",\"thresholds\":\"10\",\"unit\":\"%\"}]\n- summary = node node1 memory utilization > = 10%
	re := regexp.MustCompile(`[a-zA-Z0-9_]+\s+=\s+.*?[^\n]+`)
	sepRe := regexp.MustCompile(`\s+=\s+`)

	matches := re.FindAllString(messages, -1)
	sa := make([]string, 0)
	if len(matches) == 0 {
		// Keep the length of the string less than 20 characters.
		if utf8.RuneCountInString(messages) > 20 {
			messages = string([]rune(messages)[:20])
		}
		sa = append(sa, messages)
		return sa
	}

	extractedPairs := make(map[string]string)
	for _, m := range matches {
		mn := sepRe.Split(m, -1)
		if len(mn) > 1 {
			if mn[0] == "deployment" || mn[0] == "statfulset" || mn[0] == "daemonset" {
				mn[0] = "workload"
			}
			extractedPairs[mn[0]] = mn[1]
		}
	}

	for _, key := range allowedTemplateKeys {
		v := ""
		if val, ok := extractedPairs[key]; ok {
			v = val
		}
		// Keep the length of the string less than 20 characters.
		if utf8.RuneCountInString(v) > 20 {
			v = string([]rune(v)[:20])
		}
		sa = append(sa, `"`+v+`"`)
	}

	return sa
}

func handleHuaweiUrl(u string) string {
	if _, err := url.ParseRequestURI(u); err != nil {
		return DefaultUrl
	}
	return u
}

func handleHuaweiPhoneNums(phoneNumbers []string) string {
	if len(phoneNumbers) > HuaweiMaxPhoneNums {
		phoneNumbers = phoneNumbers[:HuaweiMaxPhoneNums]
	}
	huaweiPhoneNums := make([]string, 0)
	for _, p := range phoneNumbers {
		if ok := strings.HasPrefix(p, "+86"); !ok {
			p = fmt.Sprintf("+86%s", p)
		}
		huaweiPhoneNums = append(huaweiPhoneNums, p)
	}
	return strings.Join(huaweiPhoneNums, ",")
}
