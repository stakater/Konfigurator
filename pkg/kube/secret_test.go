package kube

import (
	"testing"

	"github.com/stakater/konfigurator/api/v1alpha1"
)

func TestCreateSecret(t *testing.T) {
	name := "test-secret"
	secret := CreateSecret(name)

	if secret.ObjectMeta.Name != name && secret.TypeMeta.Kind != string(v1alpha1.RenderTargetSecret) {
		t.Errorf("Secret creation failed with name: '%s' and kind: '%s'", name, string(v1alpha1.RenderTargetSecret))
	}
}
