package kube

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/stakater/konfigurator/api/v1alpha1"
)

// CreateObjectFromApp returns a new, empty typed workload object for app.Kind,
// named and namespaced for a Get. Every call returns a fresh object (B10); an
// unsupported kind is an error, never a nil object (B3).
func CreateObjectFromApp(app v1alpha1.App, namespace string) (client.Object, error) {
	var obj client.Object
	switch app.Kind {
	case v1alpha1.AppKindDeployment:
		obj = &appsv1.Deployment{TypeMeta: typeMeta(app.Kind)}
	case v1alpha1.AppKindDaemonSet:
		obj = &appsv1.DaemonSet{TypeMeta: typeMeta(app.Kind)}
	case v1alpha1.AppKindStatefulSet:
		obj = &appsv1.StatefulSet{TypeMeta: typeMeta(app.Kind)}
	default:
		return nil, fmt.Errorf("unsupported app kind %q", app.Kind)
	}
	obj.SetName(app.Name)
	obj.SetNamespace(namespace)
	return obj, nil
}

func typeMeta(kind v1alpha1.AppKind) metav1.TypeMeta {
	return metav1.TypeMeta{Kind: string(kind), APIVersion: "apps/v1"}
}
