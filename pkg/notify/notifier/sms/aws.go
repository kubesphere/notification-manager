package sms

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/kubesphere/notification-manager/pkg/apis/v2beta2"
	"github.com/kubesphere/notification-manager/pkg/controller"
	"github.com/kubesphere/notification-manager/pkg/utils"
)

const (
	awsMaxPhoneNums = 1000
)

type AWSNotifier struct {
	notifierCtl     *controller.Controller
	AccessKeyId     *v2beta2.Credential
	SecretAccessKey *v2beta2.Credential
	PhoneNums       string
	Region          string
}

func NewAWSProvider(c *controller.Controller, providers *v2beta2.Providers, phoneNumbers []string) Provider {
	phoneNums := handleAWSPhoneNums(phoneNumbers)
	return &AWSNotifier{
		notifierCtl:     c,
		AccessKeyId:     providers.AWS.AccessKeyId,
		SecretAccessKey: providers.AWS.SecretAccessKey,
		PhoneNums:       phoneNums,
		Region:          providers.AWS.Region,
	}
}

type SNSPublishAPI interface {
	Publish(ctx context.Context,
		params *sns.PublishInput,
		optFns ...func(*sns.Options)) (*sns.PublishOutput, error)
}

func PublishMessage(c context.Context, api SNSPublishAPI, input *sns.PublishInput) (*sns.PublishOutput, error) {
	return api.Publish(c, input)
}

func (a *AWSNotifier) MakeRequest(ctx context.Context, messages string) error {
	accessKeyId, err := a.notifierCtl.GetCredential(a.AccessKeyId)
	if err != nil {
		return utils.Errorf("[AWS  SendSms] cannot get accessKeyId: %s", err.Error())
	}
	secretAccessKey, err := a.notifierCtl.GetCredential(a.SecretAccessKey)
	if err != nil {
		return utils.Errorf("[AWS  SendSms] cannot get secretAccessKey: %s", err.Error())
	}

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
			Value: aws.Credentials{
				AccessKeyID: accessKeyId, SecretAccessKey: secretAccessKey,
			},
		}),
		config.WithRegion(a.Region))
	if err != nil {
		return utils.Errorf("[AWS SendSms]configuration error: %s", err.Error())
	}

	client := sns.NewFromConfig(cfg)

	input := &sns.PublishInput{
		Message:     &messages,
		PhoneNumber: &a.PhoneNums,
	}

	_, err = PublishMessage(ctx, client, input)
	if err != nil {
		return utils.Errorf("[AWS  SendSms] Send failed: %s", err.Error())
	}

	return nil
}

func handleAWSPhoneNums(phoneNumbers []string) string {
	if len(phoneNumbers) > awsMaxPhoneNums {
		phoneNumbers = phoneNumbers[:awsMaxPhoneNums]
	}
	return strings.Join(phoneNumbers, ",")
}
