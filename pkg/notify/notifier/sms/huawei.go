package sms

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

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
		Signature:     providers.Huawei.Signature,
		NotifierCfg:   c,
		TemplateId:    providers.Huawei.TemplateId,
		AppKey:        providers.Huawei.AppKey,
		AppSecret:     providers.Huawei.AppSecret,
		PhoneNums:     phoneNum,
		Sender:        providers.Huawei.Sender,
		TemplateParas: providers.Huawei.TemplateParas,
		Url:           u,
	}
}

func (h *HuaweiNotifier) MakeRequest(ctx context.Context, messages string) error {
	appKey, err := h.NotifierCfg.GetCredential(h.AppKey)
	if err != nil {
		return fmt.Errorf("[Huawei SendSms]cannot get appKey: %s", err.Error())
	}
	appSecret, err := h.NotifierCfg.GetCredential(h.AppSecret)
	if err != nil {
		return fmt.Errorf("[Huawei SendSms] cannot get appSecret: %s", err.Error())
	}

	// body
	body := strings.NewReader(url.Values{
		"from":           {h.Sender},
		"to":             {h.PhoneNums},
		"templateId":     {h.TemplateId},
		"templateParas":  {`[" + Messages + "]`},
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
		return fmt.Errorf("[Huawei  SendSms] send failed: %s, code: %s", r.Description, r.Code)
	}
	return nil

}

func buildWSSEHeader(appKey string, appSecret string) string {
	now := time.Now().Format("2006-01-02T15:04:05Z")
	nonce := strings.ReplaceAll(string(uuid.NewUUID()), "-", "")
	sha256 := getSha256Code(nonce + now + appSecret)
	passwordDigest := base64.StdEncoding.EncodeToString([]byte(sha256))
	return fmt.Sprintf(`"UsernameToken Username="%s",PasswordDigest="%s",Nonce="%s",Created="%s"`, appKey, passwordDigest, nonce, now)
}

func handleHuaweiUrl(u string) string {
	if _, err := url.ParseRequestURI(u); err != nil {
		return DefaultUrl
	}
	return u
}

func handleHuaweiPhoneNums(phoneNumbers []string) string {
	if len(phoneNumbers) > aliyunMaxPhoneNums {
		phoneNumbers = phoneNumbers[:aliyunMaxPhoneNums]
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
