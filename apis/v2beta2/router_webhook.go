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
	"regexp"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (r *Router) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,mutating=false,failurePolicy=fail,groups=notification.kubesphere.io,resources=routers,versions=v2beta2
var _ webhook.Validator = &Router{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Router) ValidateCreate() (warnings admission.Warnings, err error) {

	return r.validateRouter()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Router) ValidateUpdate(_ runtime.Object) (warnings admission.Warnings, err error) {
	return r.validateRouter()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Router) ValidateDelete() (warnings admission.Warnings, err error) {
	return admission.Warnings{}, nil
}

func (r *Router) validateRouter() (warnings admission.Warnings, err error) {
	var allErrs field.ErrorList

	if err := validateSelector(r.Spec.AlertSelector); err != nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "alertSelector"), r.Spec.AlertSelector, err.Error()))
	}

	if err := validateSelector(r.Spec.Receivers.Selector); err != nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "receivers", "alertSelector"), r.Spec.Receivers.Selector, err.Error()))
	}

	if r.Spec.Receivers.RegexName != "" {
		if _, err := regexp.Compile(r.Spec.Receivers.RegexName); err != nil {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "receivers", "regexName"), r.Spec.Receivers.RegexName, err.Error()))
		}
	}

	if allErrs == nil || len(allErrs) == 0 {
		return admission.Warnings{}, nil
	}

	return admission.Warnings{}, errors.NewInvalid(
		schema.GroupKind{Group: "notification.kubesphere.io", Kind: "Receiver"},
		r.Name, allErrs)
}
