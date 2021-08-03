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
	"regexp"
	"unicode/utf8"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (r *Receiver) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,mutating=false,failurePolicy=fail,groups=notification.kubesphere.io,resources=configs,versions=v2beta2
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
			"path":       field.NewPath("spec").Child("dingtalk").Child("chatbot").Child("webhook"),
		})
		credentials = append(credentials, map[string]interface{}{
			"credential": r.Spec.DingTalk.ChatBot.Secret,
			"path":       field.NewPath("spec").Child("dingtalk").Child("chatbot").Child("secret"),
		})
	}

	if r.Spec.Webhook != nil && r.Spec.Webhook.HTTPConfig != nil {
		httpConfig := r.Spec.Webhook.HTTPConfig
		credentials = append(credentials, map[string]interface{}{
			"credential": httpConfig.BearerToken,
			"path":       field.NewPath("spec").Child("webhook").Child("httpConfig").Child("bearerToken"),
		})

		if httpConfig.BasicAuth != nil {
			credentials = append(credentials, map[string]interface{}{
				"credential": httpConfig.BasicAuth.Password,
				"path":       field.NewPath("spec").Child("webhook").Child("httpConfig").Child("basicAuth").Child("password"),
			})
		}

		if httpConfig.TLSConfig != nil {
			credentials = append(credentials, map[string]interface{}{
				"credential": httpConfig.TLSConfig.RootCA,
				"path":       field.NewPath("spec").Child("webhook").Child("httpConfig").Child("tlsConfig").Child("rootCA"),
			})

			if httpConfig.TLSConfig.ClientCertificate != nil {
				credentials = append(credentials, map[string]interface{}{
					"credential": httpConfig.TLSConfig.Cert,
					"path":       field.NewPath("spec").Child("webhook").Child("httpConfig").Child("tlsConfig").Child("clientCertificate").Child("cert"),
				})
				credentials = append(credentials, map[string]interface{}{
					"credential": httpConfig.TLSConfig.Key,
					"path":       field.NewPath("spec").Child("webhook").Child("httpConfig").Child("tlsConfig").Child("clientCertificate").Child("key"),
				})
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
			allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("dingtalk").Child("conversation").Child("chatids"),
				"must be specified"))
		}

		if r.Spec.DingTalk.TmplType != nil {
			if *r.Spec.DingTalk.TmplType != "text" && *r.Spec.DingTalk.TmplType != "markdown" {
				allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("dingtalk").Child("tmplType"),
					"must be one of: `text` or `markdown`"))
			}
		}
	}

	if r.Spec.Email != nil {
		if len(r.Spec.Email.To) == 0 {
			allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("email").Child("to"),
				"must be specified"))
		}

		if r.Spec.Email.TmplType != nil {
			if *r.Spec.Email.TmplType != "text" && *r.Spec.Email.TmplType != "html" {
				allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("email").Child("tmplType"),
					"must be one of: `text` or `html`"))
			}
		}
	}

	if r.Spec.Slack != nil && len(r.Spec.Slack.Channels) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("slack").Child("channels"),
			"must be specified"))
	}

	if r.Spec.Webhook != nil {
		if r.Spec.Webhook.URL == nil && r.Spec.Webhook.Service == nil {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("webhook"), "",
				"must specify one of: `url` or `service`"))
		} else if r.Spec.Webhook.URL != nil && r.Spec.Webhook.Service != nil {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("webhook").Child("url"), "",
				"may not be specified when `service` is not empty"))
		}
	}

	if r.Spec.Wechat != nil {
		wechat := r.Spec.Wechat
		if (wechat.ToUser == nil || len(wechat.ToUser) == 0) &&
			(wechat.ToParty == nil || len(wechat.ToParty) == 0) &&
			(wechat.ToTag == nil || len(wechat.ToTag) == 0) {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("wechat"), "",
				"must specify one of: `toUser`, `toParty` or `toTag`"))
		}

		if wechat.TmplType != nil {
			if *wechat.TmplType != "text" && *wechat.TmplType != "markdown" {
				allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("wechat").Child("tmplType"),
					"must be one of: `text` or `html`"))
			}
		}
	}

	if r.Spec.Pushover != nil {
		// validate User Profile
		if len(r.Spec.Pushover.Profiles) == 0 {
			allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("pushover").Child("profiles"),
				"must be specified"))
		} else {
			// requirements
			tokenRegex := regexp.MustCompile(`^[A-Za-z0-9]{30}$`)
			deviceRegex := regexp.MustCompile(`^[A-Za-z0-9_-]{1,25}$`)
			sounds := map[string]bool{"pushover": true, "bike": true, "bugle": true, "cashregister": true, "classical": true, "cosmic": true, "falling": true, "gamelan": true, "incoming": true, "intermission": true, "magic": true, "mechanical": true, "pianobar": true, "siren": true, "spacealarm": true, "tugboat": true, "alien": true, "climb": true, "persistent": true, "echo": true, "updown": true, "vibrate": true, "none": true}
			// validate each profile
			for i, profile := range r.Spec.Pushover.Profiles {
				// validate UserKeys
				if profile.UserKey == nil || !tokenRegex.MatchString(*profile.UserKey) {
					allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("pushover").Child("profiles").Child("userKey"),
						fmt.Sprintf("found invalid Pushover User Key: %s, profile: %d", *profile.UserKey, i)))
				}
				// validate Devices
				for _, device := range profile.Devices {
					if !deviceRegex.MatchString(device) {
						allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("pushover").Child("devices"),
							fmt.Sprintf("found invalid Pushover device name: %s, profile: %d", device, i)))
					}
				}
				// Validate Title
				if profile.Title != nil {
					if l := utf8.RuneCountInString(*profile.Title); l > 250 {
						allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("pushover").Child("title"),
							fmt.Sprintf("found invalid Pushover title: %s, please limit your title within 250 characters, profile: %d", *profile.Title, i)))
					}
				}
				// Validate Sound
				if profile.Sound != nil {
					if !sounds[*profile.Sound] {
						allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("pushover").Child("title"),
							fmt.Sprintf("found invalid Pushover sound: %s, please refer to https://pushover.net/api#sounds, profile: %d", *profile.Sound, i)))
					}
				}
			}
		}
	}

	if allErrs == nil || len(allErrs) == 0 {
		return nil
	}

	return errors.NewInvalid(
		schema.GroupKind{Group: "notification.kubesphere.io", Kind: "Receiver"},
		r.Name, allErrs)
}
