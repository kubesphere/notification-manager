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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ReceiverSpec defines the desired state of Receiver
type ReceiverSpec struct {
	// Find EmailReceiver to send notification to for a specific tenant user
	EmailReceiverSelector *metav1.LabelSelector `json:"emailReceiverSelector,omitempty"`
	// Find WechatReceiverSelector to send notification to for a specific tenant user
	WechatReceiverSelector *metav1.LabelSelector `json:"wechatReceiverSelector,omitempty"`
	// Find SlackReceiverSelector to send notification to for a specific tenant user
	SlackReceiverSelector *metav1.LabelSelector `json:"slackReceiverSelector,omitempty"`
	// Find WebhookReceiverSelector to send notification to for a specific tenant user
	WebhookReceiverSelector *metav1.LabelSelector `json:"webhookReceiverSelector,omitempty"`
}

// ReceiverStatus defines the observed state of Receiver
type ReceiverStatus struct {
}

// +kubebuilder:object:root=true

// Receiver is the Schema for the receivers API
type Receiver struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ReceiverSpec   `json:"spec,omitempty"`
	Status ReceiverStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ReceiverList contains a list of Receiver
type ReceiverList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Receiver `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Receiver{}, &ReceiverList{})
}
