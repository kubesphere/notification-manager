package sms

import (
	"context"
	"fmt"
	"strings"

	openapi "github.com/alibabacloud-go/darabonba-openapi/client"
	dysmsapi "github.com/alibabacloud-go/dysmsapi-20170525/v2/client"
	"github.com/kubesphere/notification-manager/pkg/apis/v2beta2"
	"github.com/kubesphere/notification-manager/pkg/notify/config"
)

type AliyunNotifier struct {
	SignName        string
	NotifierCfg     *config.Config
	TemplateCode    string
	AccessKeyId     *v2beta2.Credential
	AccessKeySecret *v2beta2.Credential
	PhoneNums       string
}

func NewAliyunProvider(c *config.Config, providers *v2beta2.Providers, phoneNumbers []string) Provider {
	phoneNum := strings.Join(phoneNumbers, ",")
	return &AliyunNotifier{
		SignName:        providers.Aliyun.SignName,
		NotifierCfg:     c,
		TemplateCode:    providers.Aliyun.TemplateCode,
		AccessKeyId:     providers.Aliyun.AccessKeyId,
		AccessKeySecret: providers.Aliyun.AccessKeySecret,
		PhoneNums:       phoneNum,
	}
}

func (a *AliyunNotifier) MakeRequest(ctx context.Context, messages string) error {
	accessKeyId, err := a.NotifierCfg.GetCredential(a.AccessKeyId)
	if err != nil {
		return fmt.Errorf("cannot get accessKeyId: %s", err.Error())
	}
	accessKeySecret, err := a.NotifierCfg.GetCredential(a.AccessKeySecret)
	if err != nil {
		return fmt.Errorf("cannot get accessKeySecret")
	}
	config := &openapi.Config{}
	config.AccessKeyId = &accessKeyId
	config.AccessKeySecret = &accessKeySecret
	client, err := dysmsapi.NewClient(config)
	if err != nil {
		return fmt.Errorf("cannot make a client with accessKeyId:%s,accessKeySecret:%s", accessKeyId, accessKeySecret)
	}

	templateParam := `{"code":"` + messages + `"}`
	sendReq := &dysmsapi.SendSmsRequest{
		PhoneNumbers:  &a.PhoneNums,
		SignName:      &a.SignName,
		TemplateCode:  &a.TemplateCode,
		TemplateParam: &templateParam,
	}
	sendResp, err := client.SendSms(sendReq)
	if err != nil {
		return err
	}

	if stringValue(sendResp.Body.Code) != "OK" {
		return fmt.Errorf(stringValue(sendResp.Body.Message))
	}

	return nil
}

func stringValue(a *string) string {
	if a == nil {
		return ""
	}
	return *a
}
