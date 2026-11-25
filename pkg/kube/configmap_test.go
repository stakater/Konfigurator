package kube

import (
	"testing"

	"github.com/stakater/konfigurator/api/v1alpha1"
)

func TestCreateConfigMap(t *testing.T) {
	name := "test-configmap"
	configmap := CreateConfigMap(name)

	if configmap.Name != name && configmap.Kind != string(v1alpha1.RenderTargetConfigMap) {
		t.Errorf("Configmap creation failed with name: '%s' and kind: '%s'", name, string(v1alpha1.RenderTargetConfigMap))
	}
}
