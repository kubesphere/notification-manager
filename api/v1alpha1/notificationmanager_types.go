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

// NotificationManagerSpec defines the desired state of NotificationManager
type NotificationManagerSpec struct {
	// Global default receiver configs, used if corresponding items in Receivers are not specified
	Global *GlobalSpec `json:"global,omitempty"`
	// Receivers to send notifications to
	Receivers *ReceiversSpec `json:"receivers"`
}

// Global default receiver configs
type GlobalSpec struct {
	// Global default EmailConfig to be selected
	EmailConfigSelector *metav1.LabelSelector `json:"emailConfigSelector,omitempty"`
	// Global default WechatConfig to be selected
	WechatConfigSelector *metav1.LabelSelector `json:"wechatConfigSelector,omitempty"`
	// Global default SlackConfig to be selected
	SlackConfigSelector *metav1.LabelSelector `json:"slackConfigSelector,omitempty"`
	// Global default WebhookConfig to be selected
	WebhookConfigSelector *metav1.LabelSelector `json:"webhookConfigSelector,omitempty"`
}

type ReceiversSpec struct {
	// Key used to identify tenant, default to be "namespace" if not specified
	TenantKey string `json:"tenantKey,omitempty"`
	// Selector to find all notification receivers
	ReceiverSelector *metav1.LabelSelector `json:"receiverSelector"`
}

// NotificationManagerStatus defines the observed state of NotificationManager
type NotificationManagerStatus struct {
}

// +kubebuilder:object:root=true

// NotificationManager is the Schema for the notificationmanagers API
type NotificationManager struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NotificationManagerSpec   `json:"spec,omitempty"`
	Status NotificationManagerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NotificationManagerList contains a list of NotificationManager
type NotificationManagerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NotificationManager `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NotificationManager{}, &NotificationManagerList{})
}
