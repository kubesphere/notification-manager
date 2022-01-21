package sms

import (
	"context"
	"strings"

	openapi "github.com/alibabacloud-go/darabonba-openapi/client"
	dysmsapi "github.com/alibabacloud-go/dysmsapi-20170525/v2/client"
	"github.com/kubesphere/notification-manager/pkg/apis/v2beta2"
	"github.com/kubesphere/notification-manager/pkg/controller"
	"github.com/kubesphere/notification-manager/pkg/utils"
)

const (
	aliyunMaxPhoneNums = 1000
)

type AliyunNotifier struct {
	SignName        string
	notifierCtl     *controller.Controller
	TemplateCode    string
	AccessKeyId     *v2beta2.Credential
	AccessKeySecret *v2beta2.Credential
	PhoneNums       string
}

func NewAliyunProvider(c *controller.Controller, providers *v2beta2.Providers, phoneNumbers []string) Provider {
	phoneNums := handleAliyunPhoneNums(phoneNumbers)
	return &AliyunNotifier{
		SignName:        providers.Aliyun.SignName,
		notifierCtl:     c,
		TemplateCode:    providers.Aliyun.TemplateCode,
		AccessKeyId:     providers.Aliyun.AccessKeyId,
		AccessKeySecret: providers.Aliyun.AccessKeySecret,
		PhoneNums:       phoneNums,
	}
}

func (a *AliyunNotifier) MakeRequest(_ context.Context, messages string) error {
	accessKeyId, err := a.notifierCtl.GetCredential(a.AccessKeyId)
	if err != nil {
		return utils.Errorf("[Aliyun  SendSms] cannot get accessKeyId: %s", err.Error())
	}
	accessKeySecret, err := a.notifierCtl.GetCredential(a.AccessKeySecret)
	if err != nil {
		return utils.Errorf("[Aliyun  SendSms] cannot get accessKeySecret: %s", err.Error())
	}
	c := &openapi.Config{}
	c.AccessKeyId = &accessKeyId
	c.AccessKeySecret = &accessKeySecret
	client, err := dysmsapi.NewClient(c)
	if err != nil {
		return utils.Errorf("[Aliyun  SendSms] cannot make a client with accessKeyId:%s,accessKeySecret:%s",
			a.AccessKeyId.ValueFrom.SecretKeyRef.Name, a.AccessKeySecret.ValueFrom.SecretKeyRef.Name)
	}

	templateParam := `{"code":"` + messages + `"}`
	req := &dysmsapi.SendSmsRequest{
		PhoneNumbers:  &a.PhoneNums,
		SignName:      &a.SignName,
		TemplateCode:  &a.TemplateCode,
		TemplateParam: &templateParam,
	}
	resp, err := client.SendSms(req)
	if err != nil {
		return utils.Errorf("[Aliyun  SendSms] An API error occurs: %s", err.Error())
	}

	if stringValue(resp.Body.Code) != "OK" {
		return utils.Errorf("[Aliyun  SendSms] Send failed: %s", stringValue(resp.Body.Message))
	}

	return nil
}

func handleAliyunPhoneNums(phoneNumbers []string) string {
	if len(phoneNumbers) > aliyunMaxPhoneNums {
		phoneNumbers = phoneNumbers[:aliyunMaxPhoneNums]
	}
	return strings.Join(phoneNumbers, ",")
}
