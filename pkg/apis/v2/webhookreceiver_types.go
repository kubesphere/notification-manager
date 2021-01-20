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

package v2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// WebhookReceiverSpec defines the desired state of WebhookReceiver
type WebhookReceiverSpec struct {
	// WebhookConfig to be selected for this receiver
	WebhookConfigSelector *metav1.LabelSelector `json:"webhookConfigSelector,omitempty"`
	// Selector to filter alerts.
	AlertSelector *metav1.LabelSelector `json:"alertSelector,omitempty"`
}

// WebhookReceiverStatus defines the observed state of WebhookReceiver
type WebhookReceiverStatus struct {
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=wr

// WebhookReceiver is the Schema for the webhookreceivers API
type WebhookReceiver struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WebhookReceiverSpec   `json:"spec,omitempty"`
	Status WebhookReceiverStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WebhookReceiverList contains a list of WebhookReceiver
type WebhookReceiverList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WebhookReceiver `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WebhookReceiver{}, &WebhookReceiverList{})
}
