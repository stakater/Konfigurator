package volumes

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stakater/konfigurator/pkg/kube/testutil"
)

// controller-runtime's typed client clears TypeMeta after a Get, so dispatch must
// not depend on Kind (H.7 regression: nil func call panicked every mount).
func TestGetFromObjectWithoutTypeMeta(t *testing.T) {
	d, ds, ss := testutil.GetDeployment("test"), testutil.GetDaemonSet("test"), testutil.GetStatefulSet("test")
	d.TypeMeta, ds.TypeMeta, ss.TypeMeta = metav1.TypeMeta{}, metav1.TypeMeta{}, metav1.TypeMeta{}

	tests := []struct {
		name string
		obj  metav1.Object
	}{
		{"deployment", d}, {"daemonset", ds}, {"statefulset", ss},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !verifyVolumesExist(GetFromObject(tt.obj), "test") {
				t.Fatal("volume not found when TypeMeta is empty")
			}
		})
	}
}
