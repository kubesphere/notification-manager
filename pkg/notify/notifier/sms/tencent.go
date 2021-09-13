package sms

import (
	"context"
	"fmt"
	"strings"

	"github.com/kubesphere/notification-manager/pkg/apis/v2beta2"
	"github.com/kubesphere/notification-manager/pkg/config"
	"github.com/kubesphere/notification-manager/pkg/utils"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/regions"
	smsApi "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sms/v20190711"
)

const (
	tencentMaxPhoneNums = 200
)

type TencentNotifier struct {
	Sign        string
	NotifierCfg *config.Config
	TemplateID  string
	SecretId    *v2beta2.Credential
	SecretKey   *v2beta2.Credential
	PhoneNums   []string
	SmsSdkAppid string
}

func NewTencentProvider(c *config.Config, providers *v2beta2.Providers, phoneNumbers []string) Provider {
	phoneNum := handleTencentPhoneNum(phoneNumbers)
	return &TencentNotifier{
		Sign:        providers.Tencent.Sign,
		NotifierCfg: c,
		TemplateID:  providers.Tencent.TemplateID,
		SecretId:    providers.Tencent.SecretId,
		SecretKey:   providers.Tencent.SecretKey,
		PhoneNums:   phoneNum,
		SmsSdkAppid: providers.Tencent.SmsSdkAppid,
	}
}

func (t *TencentNotifier) MakeRequest(_ context.Context, messages string) error {
	secretId, err := t.NotifierCfg.GetCredential(t.SecretId)
	if err != nil {
		return utils.Errorf("[Tencent SendSms]cannot get accessKeyId: %s", err.Error())
	}
	secretKey, err := t.NotifierCfg.GetCredential(t.SecretKey)
	if err != nil {
		return utils.Errorf("[Tencent SendSms] cannot get secretKey: %s", err.Error())
	}
	credential := common.NewCredential(secretId, secretKey)
	client, _ := smsApi.NewClient(credential, regions.Guangzhou, profile.NewClientProfile())

	req := smsApi.NewSendSmsRequest()
	req.SmsSdkAppid = common.StringPtr(t.SmsSdkAppid)
	req.Sign = common.StringPtr(t.Sign)
	// req.SenderId = common.StringPtr("xxx")
	// req.SessionContext = common.StringPtr("xxx")
	// req.ExtendCode = common.StringPtr("0")
	req.TemplateParamSet = common.StringPtrs([]string{messages})
	req.TemplateID = common.StringPtr(t.TemplateID)
	req.PhoneNumberSet = common.StringPtrs(t.PhoneNums)

	resp, err := client.SendSms(req)

	if err != nil {
		return utils.Errorf("[Tencent SendSms] An API error occurs: %s", err.Error())
	}

	sendStatusSet := resp.Response.SendStatusSet
	failedPhoneNums := make([]string, 0)
	if len(sendStatusSet) != 0 {
		for _, sendStatus := range sendStatusSet {
			if sendStatus != nil && stringValue(sendStatus.Code) != "OK" {
				failedPhoneNums = append(failedPhoneNums, stringValue(sendStatus.PhoneNumber))
			}
		}
	}

	if len(failedPhoneNums) != 0 {
		return utils.Errorf("[Tencent SendSms] Some phonenums send failed: %s", strings.Join(failedPhoneNums, ","))
	}

	return nil

}

func handleTencentPhoneNum(phoneNumbers []string) []string {
	if len(phoneNumbers) > tencentMaxPhoneNums {
		phoneNumbers = phoneNumbers[:tencentMaxPhoneNums]
	}
	tencentPhoneNums := make([]string, 0)
	for _, p := range phoneNumbers {
		if ok := strings.HasPrefix(p, "+86"); !ok {
			p = fmt.Sprintf("+86%s", p)
		}
		tencentPhoneNums = append(tencentPhoneNums, p)
	}
	return tencentPhoneNums
}
