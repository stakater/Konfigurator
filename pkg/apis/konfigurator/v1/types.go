package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Phase string

const (
	PhaseInitial  Phase = ""
	PhaseRendered       = "Rendered"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ConfigTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []ConfigTemplate `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ConfigTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              ConfigTemplateSpec   `json:"spec"`
	Status            ConfigTemplateStatus `json:"status,omitempty"`
}

type ConfigTemplateSpec struct {
	RenderTarget string            `json:"renderTarget"`
	VolumeMounts []VolumeMount     `json:"volumeMounts"`
	Templates    map[string]string `json:"templates"`
}

type VolumeMount struct {
	MountPath string `json:"mountPath"`
	Container string `json:"container"`
}

type ConfigTemplateStatus struct {
	// Fill me
}
