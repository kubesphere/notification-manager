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
	// Docker Image used to start Notification Manager container,
	// for example kubesphere/notification-manager:v0.1.0
	Image *string `json:"image,omitempty"`
	// Number of instances to deploy for Notification Manager deployment.
	Replicas *int32 `json:"replicas,omitempty"`
	// ServiceAccountName is the name of the ServiceAccount to use to run Notification Manager Pods.
	// ServiceAccount 'default' in notification manager's namespace will be used if not specified.
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
	// Port name used for the pods and service, defaults to webhook
	PortName string `json:"portName,omitempty"`
	// Global default Email/Wechat/Slack/Webhook Config to be selected
	GlobalConfigSelector *metav1.LabelSelector `json:"globalConfigSelector,omitempty"`
	// Receivers to send notifications to
	Receivers *ReceiversSpec `json:"receivers"`
}

type ReceiversSpec struct {
	// Key used to identify tenant, default to be "namespace" if not specified
	TenantKey string `json:"tenantKey"`
	// Selector to find global notification receivers
	// which will be used when tenant receivers cannot be found.
	// Only matchLabels expression is allowed.
	GlobalReceiverSelector *metav1.LabelSelector `json:"globalReceiverSelector"`
	// Selector to find tenant notification receivers.
	// Only matchLabels expression is allowed.
	TenantReceiverSelector *metav1.LabelSelector `json:"tenantReceiverSelector"`
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
