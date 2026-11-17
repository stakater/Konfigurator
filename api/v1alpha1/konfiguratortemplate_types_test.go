package v1alpha1

import "testing"

func TestKonfiguratorTemplateIsValid(t *testing.T) {
	tests := []struct {
		name   string
		kind   AppKind
		target RenderTarget
		valid  bool
	}{
		{"deployment/configmap", AppKindDeployment, RenderTargetConfigMap, true},
		{"daemonset/secret", AppKindDaemonSet, RenderTargetSecret, true},
		{"statefulset/configmap", AppKindStatefulSet, RenderTargetConfigMap, true},
		{"unknown kind (B3)", "Job", RenderTargetConfigMap, false},
		{"empty kind", "", RenderTargetSecret, false},
		{"lowercase target (B11)", AppKindDeployment, "configmap", false},
		{"empty target", AppKindDeployment, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := &KonfiguratorTemplate{Spec: KonfiguratorTemplateSpec{
				App:          App{Name: "app", Kind: tt.kind},
				RenderTarget: tt.target,
			}}
			ok, err := k.IsValid()
			if ok != tt.valid {
				t.Fatalf("IsValid() = %v (err %v), want %v", ok, err, tt.valid)
			}
			if !tt.valid && err == nil {
				t.Fatal("invalid spec must return a descriptive error")
			}
		})
	}
}
