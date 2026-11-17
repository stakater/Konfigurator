package kube

import (
	"testing"

	"github.com/stakater/konfigurator/api/v1alpha1"
	"github.com/stakater/konfigurator/pkg/kube/testutil"
	appsv1 "k8s.io/api/apps/v1"
)

func TestCreateObjectFromApp(t *testing.T) {
	tests := []struct {
		name    string
		kind    v1alpha1.AppKind
		wantErr bool
		check   func(t *testing.T, obj interface{})
	}{
		{name: "deployment", kind: v1alpha1.AppKindDeployment, check: func(t *testing.T, obj interface{}) {
			if _, ok := obj.(*appsv1.Deployment); !ok {
				t.Fatalf("want *Deployment, got %T", obj)
			}
		}},
		{name: "daemonset", kind: v1alpha1.AppKindDaemonSet, check: func(t *testing.T, obj interface{}) {
			if _, ok := obj.(*appsv1.DaemonSet); !ok {
				t.Fatalf("want *DaemonSet, got %T", obj)
			}
		}},
		{name: "statefulset", kind: v1alpha1.AppKindStatefulSet, check: func(t *testing.T, obj interface{}) {
			if _, ok := obj.(*appsv1.StatefulSet); !ok {
				t.Fatalf("want *StatefulSet, got %T", obj)
			}
		}},
		{name: "unknown kind is an error, not a panic (B3)", kind: "Job", wantErr: true},
		{name: "empty kind is an error", kind: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj, err := CreateObjectFromApp(testutil.GetApp(tt.kind), testutil.Namespace)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("want error for kind %q, got object %T", tt.kind, obj)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.check(t, obj)
			if obj.GetName() != "testapp" || obj.GetNamespace() != testutil.Namespace {
				t.Fatalf("got %s/%s, want %s/testapp", obj.GetNamespace(), obj.GetName(), testutil.Namespace)
			}
		})
	}
}

// B10: each call must return a fresh object, never a shared global.
func TestCreateObjectFromAppReturnsDistinctObjects(t *testing.T) {
	a, err := CreateObjectFromApp(testutil.GetApp(v1alpha1.AppKindDeployment), "ns-a")
	if err != nil {
		t.Fatal(err)
	}
	b, err := CreateObjectFromApp(testutil.GetApp(v1alpha1.AppKindDeployment), "ns-b")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("CreateObjectFromApp returned the same object twice")
	}
	if a.GetNamespace() != "ns-a" {
		t.Fatalf("first object mutated by second call: namespace=%q", a.GetNamespace())
	}
}
