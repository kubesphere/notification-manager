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
	"fmt"
	"github.com/kubesphere/notification-manager/pkg/utils"
	"regexp"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	PushoverDeviceRegex = regexp.MustCompile(`^[A-Za-z0-9_-]{1,25}$`)
	PushoverSounds      = []string{
		"pushover",
		"bike",
		"bugle",
		"cashregister",
		"classical",
		"cosmic",
		"falling",
		"gamelan",
		"incoming",
		"intermission",
		"magic",
		"mechanical",
		"pianobar",
		"siren",
		"spacealarm",
		"tugboat",
		"alien",
		"climb",
		"persistent",
		"echo",
		"updown",
		"vibrate",
		"none",
	}
)

func (r *Receiver) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,mutating=false,failurePolicy=fail,groups=notification.kubesphere.io,resources=receivers,versions=v2beta2
var _ webhook.Validator = &Receiver{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Receiver) ValidateCreate() error {

	return r.validateReceiver()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Receiver) ValidateUpdate(_ runtime.Object) error {
	return r.validateReceiver()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Receiver) ValidateDelete() error {
	return nil
}

func (r *Receiver) validateReceiver() error {
	var allErrs field.ErrorList
	var credentials []map[string]interface{}

	if r.Spec.DingTalk != nil && r.Spec.DingTalk.ChatBot != nil {
		credentials = append(credentials, map[string]interface{}{
			"credential": r.Spec.DingTalk.ChatBot.Webhook,
			"path":       field.NewPath("spec", "dingtalk", "chatbot", "webhook"),
		})
		credentials = append(credentials, map[string]interface{}{
			"credential": r.Spec.DingTalk.ChatBot.Secret,
			"path":       field.NewPath("spec", "dingtalk", "chatbot", "secret"),
		})
	}

	if r.Spec.Wechat != nil && r.Spec.Wechat.ChatBot != nil {
		credentials = append(credentials, map[string]interface{}{
			"credential": r.Spec.Wechat.ChatBot.Webhook,
			"path":       field.NewPath("spec", "wechat", "chatbot", "webhook"),
		})
	}

	if r.Spec.Webhook != nil && r.Spec.Webhook.HTTPConfig != nil {
		httpConfig := r.Spec.Webhook.HTTPConfig
		credentials = append(credentials, map[string]interface{}{
			"credential": httpConfig.BearerToken,
			"path":       field.NewPath("spec", "webhook", "httpConfig", "bearerToken"),
		})

		if httpConfig.BasicAuth != nil {
			credentials = append(credentials, map[string]interface{}{
				"credential": httpConfig.BasicAuth.Password,
				"path":       field.NewPath("spec", "webhook", "httpConfig", "basicAuth", "password"),
			})
		}

		if httpConfig.TLSConfig != nil {
			credentials = append(credentials, map[string]interface{}{
				"credential": httpConfig.TLSConfig.RootCA,
				"path":       field.NewPath("spec", "webhook", "httpConfig", "tlsConfig", "rootCA"),
			})

			if httpConfig.TLSConfig.ClientCertificate != nil {
				credentials = append(credentials, map[string]interface{}{
					"credential": httpConfig.TLSConfig.Cert,
					"path":       field.NewPath("spec", "webhook", "httpConfig", "tlsConfig", "clientCertificate", "cert"),
				})
				credentials = append(credentials, map[string]interface{}{
					"credential": httpConfig.TLSConfig.Key,
					"path":       field.NewPath("spec", "webhook", "httpConfig", "tlsConfig", "clientCertificate", "key"),
				})
			}
		}
	}

	if r.Spec.Feishu != nil && r.Spec.Feishu.ChatBot != nil {
		credentials = append(credentials, map[string]interface{}{
			"credential": r.Spec.Feishu.ChatBot.Webhook,
			"path":       field.NewPath("spec", "feishu", "chatbot", "webhook"),
		})
		credentials = append(credentials, map[string]interface{}{
			"credential": r.Spec.Feishu.ChatBot.Secret,
			"path":       field.NewPath("spec", "feishu", "chatbot", "secret"),
		})
	}

	if r.Spec.Discord != nil && r.Spec.Discord.Webhook != nil {
		credentials = append(credentials, map[string]interface{}{
			"credential": r.Spec.Discord.Webhook,
			"path":       field.NewPath("spec", "discord", "webhook"),
		})
		if r.Spec.Discord.Type != nil {
			if *r.Spec.Discord.Type != "content" && *r.Spec.Discord.Type != "embed" {
				allErrs = append(allErrs,
					field.NotSupported(field.NewPath("spec", "discord", "type"),
						*r.Spec.Email.TmplType,
						[]string{"content", "embed"}))
			}
		}
	}

	for _, v := range credentials {
		err := validateCredential(v["credential"].(*Credential), v["path"].(*field.Path))
		if err != nil {
			allErrs = append(allErrs, err)
		}
	}

	if r.Spec.DingTalk != nil {
		if r.Spec.DingTalk.Conversation != nil && len(r.Spec.DingTalk.Conversation.ChatIDs) == 0 {
			allErrs = append(allErrs,
				field.Required(field.NewPath("spec", "dingtalk", "conversation", "chatids"),
					"must be specified"))
		}

		if r.Spec.DingTalk.TmplType != nil {
			if *r.Spec.DingTalk.TmplType != "text" && *r.Spec.DingTalk.TmplType != "markdown" {
				allErrs = append(allErrs,
					field.NotSupported(field.NewPath("spec", "dingtalk", "tmplType"),
						*r.Spec.DingTalk.TmplType,
						[]string{"text", "markdown"}))
			}
		}

		if err := validateSelector(r.Spec.DingTalk.AlertSelector); err != nil {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec", "dingtalk", "alertSelector"),
					r.Spec.DingTalk.AlertSelector,
					err.Error()))
		}
	}

	if r.Spec.Email != nil {
		if len(r.Spec.Email.To) == 0 {
			allErrs = append(allErrs, field.Required(field.NewPath("spec", "email", "to"),
				"must be specified"))
		}

		if r.Spec.Email.TmplType != nil {
			if *r.Spec.Email.TmplType != "text" && *r.Spec.Email.TmplType != "html" {
				allErrs = append(allErrs,
					field.NotSupported(field.NewPath("spec", "email", "tmplType"),
						*r.Spec.Email.TmplType,
						[]string{"text", "html"}))
			}
		}

		if err := validateSelector(r.Spec.Email.AlertSelector); err != nil {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec", "email", "alertSelector"),
					r.Spec.Email.AlertSelector,
					err.Error()))
		}
	}

	if r.Spec.Slack != nil {
		if len(r.Spec.Slack.Channels) == 0 {
			allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("slack").Child("channels"),
				"must be specified"))
		}

		if err := validateSelector(r.Spec.Slack.AlertSelector); err != nil {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec", "slack", "alertSelector"),
					r.Spec.Slack.AlertSelector,
					err.Error()))
		}
	}

	if r.Spec.Webhook != nil {
		if r.Spec.Webhook.URL == nil && r.Spec.Webhook.Service == nil {
			allErrs = append(allErrs, field.Required(field.NewPath("spec", "webhook"),
				"must specify one of: `url` or `service`"))
		} else if r.Spec.Webhook.URL != nil && r.Spec.Webhook.Service != nil {
			allErrs = append(allErrs, field.Duplicate(field.NewPath("spec", "webhook", "url"),
				"url should not set when service set"))
		}

		if err := validateSelector(r.Spec.Webhook.AlertSelector); err != nil {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec", "webhook", "alertSelector"),
					r.Spec.Webhook.AlertSelector,
					err.Error()))
		}
	}

	if r.Spec.Wechat != nil {
		wechat := r.Spec.Wechat
		if len(wechat.ToUser) == 0 && len(wechat.ToParty) == 0 && len(wechat.ToTag) == 0 && wechat.ChatBot == nil {
			allErrs = append(allErrs, field.Required(field.NewPath("spec", "wechat"),
				"must specify one of: `toUser`, `toParty`, `toTag` or `chatbot`"))
		}

		if wechat.TmplType != nil {
			if *wechat.TmplType != "text" && *wechat.TmplType != "markdown" {
				allErrs = append(allErrs,
					field.NotSupported(field.NewPath("spec", "wechat", "tmplType"),
						*wechat.TmplType,
						[]string{"text", "markdown"}))
			}
		}

		if err := validateSelector(r.Spec.Wechat.AlertSelector); err != nil {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec", "wechat", "alertSelector"),
					wechat.AlertSelector,
					err.Error()))
		}
	}

	if r.Spec.Discord != nil {
		if r.Spec.Discord.Webhook == nil {
			allErrs = append(allErrs, field.Required(field.NewPath("spec", "discord", "webhook"),
				"must be specified"))
		}

		if err := validateSelector(r.Spec.Discord.AlertSelector); err != nil {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec", "discord", "alertSelector"),
					r.Spec.Discord.AlertSelector,
					err.Error()))
		}
	}

	if r.Spec.Pushover != nil {
		// validate User Profile
		if len(r.Spec.Pushover.Profiles) == 0 {
			allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("pushover").Child("profiles"),
				"must be specified"))
		} else {
			// requirements
			// validate each profile
			for i, profile := range r.Spec.Pushover.Profiles {
				// validate UserKeys
				if profile.UserKey == nil {
					allErrs = append(allErrs,
						field.Required(field.NewPath("spec", "pushover", fmt.Sprintf("profiles[%d]", i), "userKey"),
							"must be specified"))
				}
				// validate Devices
				for _, device := range profile.Devices {
					if !PushoverDeviceRegex.MatchString(device) {
						allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "pushover", fmt.Sprintf("profiles[%d]", i), "devices"),
							device,
							"length must less than 25 characters and can only contain character set [A-Za-z0-9_-]"))
					}
				}

				// Validate Sound
				if profile.Sound != nil {
					flag := false
					for _, v := range PushoverSounds {
						if v == *profile.Sound {
							flag = true
							break
						}
					}
					if !flag {
						allErrs = append(allErrs,
							field.NotSupported(field.NewPath("spec", "pushover", fmt.Sprintf("profiles[%d]", i), "sound"),
								*profile.Sound, PushoverSounds))
					}
				}
			}
		}

		if err := validateSelector(r.Spec.Pushover.AlertSelector); err != nil {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec", "pushover", "alertSelector"),
					r.Spec.Pushover.AlertSelector,
					err.Error()))
		}
	}

	if r.Spec.Sms != nil {
		if len(r.Spec.Sms.PhoneNumbers) == 0 {
			allErrs = append(allErrs, field.Required(field.NewPath("spec", "sms", "phoneNumbers"), "must be specified"))
		}

		if err := validateSelector(r.Spec.Sms.AlertSelector); err != nil {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec", "sms", "alertSelector"),
					r.Spec.Sms.AlertSelector,
					err.Error()))
		}
	}

	if r.Spec.Feishu != nil {
		feishu := r.Spec.Feishu
		if len(feishu.User) == 0 && len(feishu.Department) == 0 && feishu.ChatBot == nil {
			allErrs = append(allErrs, field.Required(field.NewPath("spec", "feishu"),
				"must specify one of: `user`, `department` or `chatbot`"))
		}

		if err := validateSelector(r.Spec.Feishu.AlertSelector); err != nil {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec", "feishu", "alertSelector"),
					feishu.AlertSelector,
					err.Error()))
		}
	}

	if r.Spec.Telegram != nil {
		if len(r.Spec.Telegram.Channels) == 0 {
			allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("telegram").Child("channels"),
				"must be specified"))
		}

		if err := validateSelector(r.Spec.Telegram.AlertSelector); err != nil {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("spec", "telegram", "alertSelector"),
					r.Spec.Slack.AlertSelector,
					err.Error()))
		}
	}

	if len(allErrs) == 0 {
		return nil
	}

	return errors.NewInvalid(
		schema.GroupKind{Group: "notification.kubesphere.io", Kind: "Receiver"},
		r.Name, allErrs)
}

func validateSelector(selector *LabelSelector) error {

	if selector == nil {
		return nil
	}

	_, err := utils.LabelSelectorDeal(selector)
	return err
}
