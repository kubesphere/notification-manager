package sms

import (
	"context"
	"fmt"
	"strings"

	"github.com/kubesphere/notification-manager/pkg/apis/v2beta2"
	"github.com/kubesphere/notification-manager/pkg/notify/config"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/regions"
	smsApi "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sms/v20190711"
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

func (t *TencentNotifier) MakeRequest(ctx context.Context, messages string) error {
	secretId, err := t.NotifierCfg.GetCredential(t.SecretId)
	if err != nil {
		return fmt.Errorf("[Tencent SendSms]cannot get accessKeyId: %s", err.Error())
	}
	secretKey, err := t.NotifierCfg.GetCredential(t.SecretKey)
	if err != nil {
		return fmt.Errorf("[Tencent SendSms] cannot get secretKey: %s", err.Error())
	}
	credential := common.NewCredential(secretId, secretKey)
	client, _ := smsApi.NewClient(credential, regions.Guangzhou, profile.NewClientProfile())

	request := smsApi.NewSendSmsRequest()
	request.SmsSdkAppid = common.StringPtr(t.SmsSdkAppid)
	request.Sign = common.StringPtr(t.Sign)
	// request.SenderId = common.StringPtr("xxx")
	// request.SessionContext = common.StringPtr("xxx")
	// request.ExtendCode = common.StringPtr("0")
	request.TemplateParamSet = common.StringPtrs([]string{messages})
	request.TemplateID = common.StringPtr(t.TemplateID)
	request.PhoneNumberSet = common.StringPtrs(t.PhoneNums)

	response, err := client.SendSms(request)

	if err != nil {
		return fmt.Errorf("[Tencent SendSms] An API error has returned: %s", err.Error())
	}

	sendStatusSet := response.Response.SendStatusSet
	failedPhoneNums := make([]string, 0)
	if len(sendStatusSet) != 0 {
		for _, sendStatus := range sendStatusSet {
			if sendStatus != nil && stringValue(sendStatus.Code) != "OK" {
				failedPhoneNums = append(failedPhoneNums, stringValue(sendStatus.PhoneNumber))
			}
		}
	}

	if len(failedPhoneNums) != 0 {
		return fmt.Errorf("[Tencent SendSms] Some phonenums send failed: %s", strings.Join(failedPhoneNums, ","))
	}

	return nil

}

func handleTencentPhoneNum(phoneNumber []string) []string {
	tencentPhoneNum := make([]string, 0)
	for _, p := range phoneNumber {
		if ok := strings.HasPrefix(p, "+86"); !ok {
			p = fmt.Sprintf("+86%s", p)
		}
		tencentPhoneNum = append(tencentPhoneNum, p)
	}
	return tencentPhoneNum
}
