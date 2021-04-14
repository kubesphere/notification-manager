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

	if r.Spec.DingTalk.Conversation != nil && len(r.Spec.DingTalk.Conversation.ChatIDs) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("dingtalk").Child("conversation").Child("chatids"),
			"must be specify"))
	}

	if r.Spec.Email != nil && len(r.Spec.Email.To) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("email").Child("to"),
			"must be specify"))
	}

	if r.Spec.Slack != nil && len(r.Spec.Slack.Channels) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("slack").Child("channels"),
			"must be specify"))
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
	}

	if allErrs == nil || len(allErrs) == 0 {
		return nil
	}

	return errors.NewInvalid(
		schema.GroupKind{Group: "notification.kubesphere.io", Kind: "Receiver"},
		r.Name, allErrs)
}
