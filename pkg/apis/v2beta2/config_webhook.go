/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v2beta2

import (
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (r *Config) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,mutating=false,failurePolicy=fail,groups=notification.kubesphere.io,resources=configs,versions=v2beta2
var _ webhook.Validator = &Config{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Config) ValidateCreate() error {

	return r.validateConfig()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Config) ValidateUpdate(_ runtime.Object) error {
	return r.validateConfig()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Config) ValidateDelete() error {
	return nil
}

func (r *Config) validateConfig() error {
	var allErrs field.ErrorList
	var credentials []map[string]interface{}

	if r.Spec.DingTalk != nil && r.Spec.DingTalk.Conversation != nil {
		credentials = append(credentials, map[string]interface{}{
			"credential": r.Spec.DingTalk.Conversation.AppKey,
			"path":       field.NewPath("spec", "dingtalk", "conversation", "appkey"),
		})
		credentials = append(credentials, map[string]interface{}{
			"credential": r.Spec.DingTalk.Conversation.AppSecret,
			"path":       field.NewPath("spec", "dingtalk", "conversation", "appsecret"),
		})
	}

	if r.Spec.Email != nil {
		credentials = append(credentials, map[string]interface{}{
			"credential": r.Spec.Email.AuthSecret,
			"path":       field.NewPath("spec", "email", "authSecret"),
		})
		credentials = append(credentials, map[string]interface{}{
			"credential": r.Spec.Email.AuthPassword,
			"path":       field.NewPath("spec", "email", "authPassword"),
		})

		if r.Spec.Email.TLS != nil {
			credentials = append(credentials, map[string]interface{}{
				"credential": r.Spec.Email.TLS.RootCA,
				"path":       field.NewPath("spec", "email", "tls", "rootCA"),
			})

			if r.Spec.Email.TLS.ClientCertificate != nil {
				credentials = append(credentials, map[string]interface{}{
					"credential": r.Spec.Email.TLS.Cert,
					"path":       field.NewPath("spec", "email", "tls", "clientCertificate", "cert"),
				})
				credentials = append(credentials, map[string]interface{}{
					"credential": r.Spec.Email.TLS.Key,
					"path":       field.NewPath("spec", "email", "tls", "clientCertificate", "key"),
				})
			}
		}
	}

	if r.Spec.Slack != nil {
		credentials = append(credentials, map[string]interface{}{
			"credential": r.Spec.Slack.SlackTokenSecret,
			"path":       field.NewPath("spec", "slack", "slackTokenSecret"),
		})
	}

	if r.Spec.Telegram != nil {
		credentials = append(credentials, map[string]interface{}{
			"credential": r.Spec.Telegram.TelegramTokenSecret,
			"path":       field.NewPath("spec", "telegram", "telegramTokenSecret"),
		})
	}

	if r.Spec.Wechat != nil {
		credentials = append(credentials, map[string]interface{}{
			"credential": r.Spec.Wechat.WechatApiSecret,
			"path":       field.NewPath("spec", "wechat", "wechatApiSecret"),
		})
	}

	if r.Spec.Sms != nil {
		providers := r.Spec.Sms.Providers
		defaultProvider := r.Spec.Sms.DefaultProvider
		if (defaultProvider == "aliyun" && providers.Aliyun == nil) ||
			(defaultProvider == "tencent" && providers.Tencent == nil) ||
			(defaultProvider == "huawei" && providers.Huawei == nil) ||
			(defaultProvider == "aws" && providers.AWS == nil) {
			err := field.Invalid(field.NewPath("spec", "sms", "defaultProvider"),
				defaultProvider,
				"cannot find provider")
			allErrs = append(allErrs, err)
		}

		// Sms aliyun provider parameters validation
		if providers.Aliyun != nil {
			if providers.Aliyun.AccessKeyId != nil {
				credentials = append(credentials, map[string]interface{}{
					"credential": r.Spec.Sms.Providers.Aliyun.AccessKeyId,
					"path":       field.NewPath("spec", "sms", "providers", "aliyun", "accessKeyId"),
				})
			}
			if providers.Aliyun.AccessKeySecret != nil {
				credentials = append(credentials, map[string]interface{}{
					"credential": r.Spec.Sms.Providers.Aliyun.AccessKeySecret,
					"path":       field.NewPath("spec", "sms", "providers", "aliyun", "accessKeySecret"),
				})
			}
		}

		// Sms tencent provider parameters validation
		if providers.Tencent != nil {
			if providers.Tencent.SecretId != nil {
				credentials = append(credentials, map[string]interface{}{
					"credential": r.Spec.Sms.Providers.Tencent.SecretId,
					"path":       field.NewPath("spec", "sms", "providers", "tencent", "secretId"),
				})
			}
			if providers.Tencent.SecretKey != nil {
				credentials = append(credentials, map[string]interface{}{
					"credential": r.Spec.Sms.Providers.Tencent.SecretKey,
					"path":       field.NewPath("spec", "sms", "providers", "tencent", "secretKey"),
				})
			}
		}

		// Sms huawei provider parameters validation
		if providers.Huawei != nil {
			if providers.Huawei.AppKey != nil {
				credentials = append(credentials, map[string]interface{}{
					"credential": r.Spec.Sms.Providers.Huawei.AppKey,
					"path":       field.NewPath("spec", "sms", "providers", "huawei", "appKey"),
				})
			}
			if providers.Huawei.AppSecret != nil {
				credentials = append(credentials, map[string]interface{}{
					"credential": r.Spec.Sms.Providers.Huawei.AppSecret,
					"path":       field.NewPath("spec", "sms", "providers", "huawei", "appSecret"),
				})
			}
		}

		// Sms AWS provider parameters validation
		if providers.AWS != nil {
			if providers.AWS.AccessKeyId != nil {
				credentials = append(credentials, map[string]interface{}{
					"credential": r.Spec.Sms.Providers.AWS.AccessKeyId,
					"path":       field.NewPath("spec", "sms", "providers", "aws", "accessKeyId"),
				})
			}
			if providers.AWS.SecretAccessKey != nil {
				credentials = append(credentials, map[string]interface{}{
					"credential": r.Spec.Sms.Providers.AWS.SecretAccessKey,
					"path":       field.NewPath("spec", "sms", "providers", "aws", "secretAccessKey"),
				})
			}
		}

	}

	if r.Spec.Pushover != nil {
		credentials = append(credentials, map[string]interface{}{
			"credential": r.Spec.Pushover.PushoverTokenSecret,
			"path":       field.NewPath("spec", "pushover", "pushoverTokenSecret"),
		})
	}

	if r.Spec.Feishu != nil {
		credentials = append(credentials, map[string]interface{}{
			"credential": r.Spec.Feishu.AppID,
			"path":       field.NewPath("spec", "feishu", "appID"),
		})
		credentials = append(credentials, map[string]interface{}{
			"credential": r.Spec.Feishu.AppSecret,
			"path":       field.NewPath("spec", "feishu", "appSecret"),
		})
	}

	for _, v := range credentials {
		err := validateCredential(v["credential"].(*Credential), v["path"].(*field.Path))
		if err != nil {
			allErrs = append(allErrs, err)
		}
	}

	if allErrs == nil || len(allErrs) == 0 {
		return nil
	}

	return errors.NewInvalid(
		schema.GroupKind{Group: "notification.kubesphere.io", Kind: "Config"},
		r.Name, allErrs)
}

func validateCredential(c *Credential, fldPath *field.Path) *field.Error {

	if c == nil {
		return nil
	}

	if len(c.Value) == 0 && c.ValueFrom == nil {
		return field.Required(fldPath, "must specify one of: `value` or `valueFrom`")
	}

	if len(c.Value) != 0 && c.ValueFrom != nil {
		return field.Invalid(fldPath.Child("valueFrom"), "", "may not be specified when `value` is not empty")
	}

	if c.ValueFrom != nil {
		if c.ValueFrom.SecretKeyRef == nil {
			return field.Required(fldPath.Child("valueFrom").Child("SecretKeyRef"), "must be specified")
		}
	}

	return nil
}
