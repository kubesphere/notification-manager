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
	"github.com/kubesphere/notification-manager/pkg/notify/config"
	"k8s.io/apimachinery/pkg/util/uuid"
)

const (
	HuaweiMaxPhoneNums = 200
	DefaultUrl         = "https://rtcsms.cn-north-1.myhuaweicloud.com:10743/sms/batchSendSms/v1"
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

func (h *HuaweiNotifier) MakeRequest(ctx context.Context, messages string) error {
	appKey, err := h.NotifierCfg.GetCredential(h.AppKey)
	if err != nil {
		return fmt.Errorf("[Huawei SendSms]cannot get appKey: %s", err.Error())
	}
	appSecret, err := h.NotifierCfg.GetCredential(h.AppSecret)
	if err != nil {
		return fmt.Errorf("[Huawei SendSms]cannot get appSecret: %s", err.Error())
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
		return fmt.Errorf("[Huawei SendSms] Create request failed: %s", err.Error())
	}
	req.Header.Set("Authorization", `WSSE realm="SDP",profile="UsernameToken",type="Appkey"`)
	req.Header.Set("X-WSSE", xheader)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("[Huawei SendSms] An API error occurs: %s", err.Error())
	}

	defer resp.Body.Close()

	res, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("[Huawei  SendSms] read body failed: %s", fmt.Errorf(err.Error()))
	}

	var r HuaweiResponse
	if err = json.Unmarshal(res, &r); err != nil {
		return fmt.Errorf("[Huawei  SendSms] unmarshal failed: %s", fmt.Errorf(err.Error()))
	}
	if r.Code != "000000" {
		return fmt.Errorf("[Huawei  SendSms] send failed: %s, code: %s, raw messages: %s, handled messages: %s", r.Description, r.Code, messages, m)
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

// Note here, the keys corresponding to the return values are alertname,alerttype,severity,message,summary.
// Pls make your custom SMS template containing five placeholders, you can create an SMS template in Huawei Cloud's SMS console.
// i.e: Received notifications: alertname:${TEXT}, alerttype:${TEXT}, serverity:${TEXT}, message:${TEXT}, summary:${TEXT}
func generateParams(messages string) []string {
	messagePat := `alertname=(?P<alertname>.*?)\s+(?s).*alerttype\s?=\s?(?P<alerttype>[a-zA-Z]+)\s+(?s).*severity\s?=\s?(?P<severity>[a-zA-Z]+)\s+(?s).*message\s?=\s?(?P<message>.*?)\s+(?s).*summary\s?=\s?(?P<summary>.*[^\"])`
	re := regexp.MustCompile(messagePat)

	matches := re.FindStringSubmatch(messages)
	sa := make([]string, 0)
	if len(matches) == 0 {
		// Keep the length of the string less than 20 characters.
		if utf8.RuneCountInString(messages) > 20 {
			messages = string([]rune(messages)[:20])
		}
		sa = append(sa, messages)
		return sa
	}

	stripedRe := regexp.MustCompile(`\s+`)

	for i, m := range matches {
		if i > 0 {
			mn := stripedRe.ReplaceAllString(m, "")
			// Keep the length of the string less than 20 characters.
			if utf8.RuneCountInString(mn) > 20 {
				mn = string([]rune(mn)[:20])
			}
			sa = append(sa, `"`+mn+`"`)
		}
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
