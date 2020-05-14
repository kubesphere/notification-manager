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
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NotificationManagerSpec defines the desired state of NotificationManager
type NotificationManagerSpec struct {
	// Compute Resources required by container.
	Resources v1.ResourceRequirements `json:"resources,omitempty"`
	// Docker Image used to start Notification Manager container,
	// for example kubesphere/notification-manager:v0.1.0
	Image *string `json:"image,omitempty"`
	// Image pull policy. One of Always, Never, IfNotPresent.
	// Defaults to IfNotPresent if not specified
	ImagePullPolicy *v1.PullPolicy `json:"imagePullPolicy,omitempty"`
	// Number of instances to deploy for Notification Manager deployment.
	Replicas *int32 `json:"replicas,omitempty"`
	// Define which Nodes the Pods will be scheduled to.
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// Pod's scheduling constraints.
	Affinity *v1.Affinity `json:"affinity,omitempty"`
	// Pod's tolerations.
	Tolerations []v1.Toleration `json:"tolerations,omitempty"`
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
	// Various receiver options
	Options *Options `json:"options,omitempty"`
}

type GlobalOptions struct {
	// The file which template be in, must be a absolute path.
	TemplateFiles []string `json:"templateFile,omitempty"`
}

type EmailOptions struct {
	// Notification Sending Timeout
	NotificationTimeout *int32 `json:"notificationTimeout,omitempty"`
	// Type of sending email, bulk or single
	DeliveryType string `json:"deliveryType,omitempty"`
	// The maximum size of receivers in one email.
	MaxEmailReceivers int `json:"maxEmailReceivers,omitempty"`
	// The name of the template to generate email message,like '{{ template "email.default.html" . }}'
	Template string `json:"template,omitempty"`
	// The name of the template to generate email subject,like `{{ template "email.default.subject" . }}`
	SubjectTemplate string `json:"subjectTemplate,omitempty"`
}

type WechatOptions struct {
	// Notification Sending Timeout
	NotificationTimeout *int32 `json:"notificationTimeout,omitempty"`
	// The name of the template to generate wechat message,like '{{ template "wechat.default" . }}'
	Template string `json:"template,omitempty"`
}

type SlackOptions struct {
	// Notification Sending Timeout
	NotificationTimeout *int32 `json:"notificationTimeout,omitempty"`
	// The name of the template to generate slack message,like '{{ template "slack.default" . }}'
	Template string `json:"template,omitempty"`
}

type WebhookOptions struct {
	// Notification Sending Timeout
	NotificationTimeout *int32 `json:"notificationTimeout,omitempty"`
	// The name of the template to generate webhook message,like '{{ template "webhook.default" . }}'
	Template string `json:"template,omitempty"`
}

type Options struct {
	Global  *GlobalOptions  `json:"global,omitempty"`
	Email   *EmailOptions   `json:"email,omitempty"`
	Wechat  *WechatOptions  `json:"wechat,omitempty"`
	Slack   *SlackOptions   `json:"slack,omitempty"`
	Webhook *WebhookOptions `json:"webhook,omitempty"`
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
