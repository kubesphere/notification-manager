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
	"github.com/robfig/cron/v3"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (r *Silence) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,mutating=false,failurePolicy=fail,groups=notification.kubesphere.io,resources=silences,versions=v2beta2
var _ webhook.Validator = &Silence{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (s *Silence) ValidateCreate() (warnings admission.Warnings, err error) {

	return s.validateSilence()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (s *Silence) ValidateUpdate(_ runtime.Object) (warnings admission.Warnings, err error) {
	return s.validateSilence()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (s *Silence) ValidateDelete() (warnings admission.Warnings, err error) {
	return admission.Warnings{}, nil
}

func (s *Silence) validateSilence() (warnings admission.Warnings, err error) {
	var allErrs field.ErrorList

	if err := validateSelector(s.Spec.Matcher); err != nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "matcher"), s.Spec.Matcher, err.Error()))
	}

	if s.Spec.Schedule != "" {
		if _, err := cron.ParseStandard(s.Spec.Schedule); err != nil {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "schedule"), s.Spec.Schedule, err.Error()))
		}
	}

	if allErrs == nil || len(allErrs) == 0 {
		return admission.Warnings{}, nil
	}

	return admission.Warnings{}, errors.NewInvalid(
		schema.GroupKind{Group: "notification.kubesphere.io", Kind: "Receiver"},
		s.Name, allErrs)
}
