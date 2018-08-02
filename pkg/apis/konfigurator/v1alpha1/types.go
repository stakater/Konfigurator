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

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type KonfiguratorTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []KonfiguratorTemplate `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type KonfiguratorTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              KonfiguratorTemplateSpec   `json:"spec"`
	Status            KonfiguratorTemplateStatus `json:"status,omitempty"`
}

type KonfiguratorTemplateSpec struct {
	RenderTarget string            `json:"renderTarget"`
	VolumeMounts []VolumeMount     `json:"volumeMounts"`
	Templates    map[string]string `json:"templates"`
}

type VolumeMount struct {
	MountPath string `json:"mountPath"`
	Container string `json:"container"`
}

type KonfiguratorTemplateStatus struct {
	CurrentPhase Phase `json:"currentPhase"`
}
