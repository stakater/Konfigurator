/*
Copyright 2021.

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

type Phase string

const (
	PhaseInitial           Phase = ""
	PhaseRendering         Phase = "Rendering"
	PhaseCreatingConfigMap Phase = "CreatingConfigMap"
	PhaseRendered          Phase = "Rendered"
)

type RenderTarget string

const (
	RenderTargetConfigMap RenderTarget = "ConfigMap"
	RenderTargetSecret    RenderTarget = "Secret"
)

type AppKind string

const (
	AppKindDeployment  AppKind = "Deployment"
	AppKindDaemonSet   AppKind = "DaemonSet"
	AppKindStatefulSet AppKind = "StatefulSet"
)

type App struct {
	Name         string        `json:"name"`
	Kind         AppKind       `json:"kind"`
	VolumeMounts []VolumeMount `json:"volumeMounts"`
}

type VolumeMount struct {
	MountPath string `json:"mountPath"`
	Container string `json:"container"`
}

// KonfiguratorTemplateSpec defines the desired state of KonfiguratorTemplate
type KonfiguratorTemplateSpec struct {
	App          App               `json:"app"`
	RenderTarget RenderTarget      `json:"renderTarget"`
	Templates    map[string]string `json:"templates"`
	// Rendering frequency in minutes
	UpdateFrequency      int    `json:"updateFrequency,omitempty"`
	ValidationWebhookURL string `json:"validationWebhookURL,omitempty"`
}

// KonfiguratorTemplateStatus defines the observed state of KonfiguratorTemplate
type KonfiguratorTemplateStatus struct {
	//CurrentPhase Phase `json:"currentPhase"`
	// Status conditions
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// KonfiguratorTemplate is the Schema for the konfiguratortemplates API
type KonfiguratorTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              KonfiguratorTemplateSpec   `json:"spec"`
	Status            KonfiguratorTemplateStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +kubebuilder:object:root=true

// KonfiguratorTemplateList contains a list of KonfiguratorTemplate
type KonfiguratorTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KonfiguratorTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KonfiguratorTemplate{}, &KonfiguratorTemplateList{})
}

func (k *KonfiguratorTemplate) IsValid() (bool, error) {
	return true, nil
}

//NOTE:(Jose) To avoid reconcile loop because of status update in its reconcile

////GetReconcileStatus - returns conditions, required for making Channel ConditionsStatusAware
//func (k *KonfiguratorTemplate) GetReconcileStatus() []metav1.Condition {
//	return k.Status.Conditions
//}
//
//// SetReconcileStatus - sets status, required for making Channel ConditionsStatusAware
//func (k *KonfiguratorTemplate) SetReconcileStatus(reconcileStatus []metav1.Condition) {
//	k.Status.Conditions = reconcileStatus
//}
