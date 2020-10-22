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

// Configuration of ChatBot
type DingTalkChatBot struct {
	// The webhook of ChatBot which the message will send to.
	Webhook *v1.SecretKeySelector `json:"webhook"`

	// Custom keywords of ChatBot
	Keywords []string `json:"keywords,omitempty"`

	// Secret of ChatBot, you can get it after enabled Additional Signature of ChatBot.
	Secret *v1.SecretKeySelector `json:"secret,omitempty"`
}

// Configuration of conversation
type DingTalkConversation struct {
	// The key of the application which sending message.
	AppKey *v1.SecretKeySelector `json:"appkey"`
	// The secret of the application which sending message.
	AppSecret *v1.SecretKeySelector `json:"appsecret"`
	// The id of the conversation.
	ChatID string `json:"chatid"`
}

// DingTalkConfigSpec defines the desired state of DingTalkConfig
type DingTalkConfigSpec struct {

	// Be careful, a ChatBot only can send 20 message per minute.
	ChatBot *DingTalkChatBot `json:"chatbot,omitempty"`

	// The conversation which message will send to.
	Conversation *DingTalkConversation `json:"conversation,omitempty"`
}

// DingTalkConfigStatus defines the observed state of DingTalkConfig
type DingTalkConfigStatus struct {
}

// +kubebuilder:object:root=true

// DingTalkConfig is the Schema for the dingtalkconfigs API
type DingTalkConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DingTalkConfigSpec   `json:"spec,omitempty"`
	Status DingTalkConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DingTalkConfigList contains a list of DingTalkConfig
type DingTalkConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DingTalkConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DingTalkConfig{}, &DingTalkConfigList{})
}
