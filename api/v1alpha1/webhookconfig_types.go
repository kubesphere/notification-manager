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

package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// WebhookConfigSpec defines the desired state of WebhookConfig
type WebhookConfigSpec struct {
	// The URL to send HTTP POST requests to.
	// UrlSecret takes precedence over Url. One of UrlSecret and Url should be defined.
	UrlSecret *v1.SecretKeySelector `json:"urlSecret,omitempty"`
	// The endpoint to send HTTP POST requests to.
	Url *string `json:"url,omitempty"`
	// HttpConfig used for webhook
	HttpConfigSelector *metav1.LabelSelector `json:"httpConfigSelector,omitempty"`
}

// WebhookConfigStatus defines the observed state of WebhookConfig
type WebhookConfigStatus struct {
}

// +kubebuilder:object:root=true

// WebhookConfig is the Schema for the webhookconfigs API
type WebhookConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WebhookConfigSpec   `json:"spec,omitempty"`
	Status WebhookConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WebhookConfigList contains a list of WebhookConfig
type WebhookConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WebhookConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WebhookConfig{}, &WebhookConfigList{})
}
