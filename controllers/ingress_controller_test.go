/*
1. If tenant is not existing, don't requeue again
2. Space labels are inherited to the namespace
3. Check if rolebinding created
4.
*/

package controllers

import (
	"context"

	xContext "github.com/stakater/konfigurator/pkg/context"
	ctrl "sigs.k8s.io/controller-runtime"

	"testing"

	konfiguratorv1alpha1 "github.com/stakater/konfigurator/api/v1alpha1"
	networkingv1 "k8s.io/api/networking/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/types"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	TestIngressName = "testIngress"
)

func TestIngressCache(t *testing.T) {

	// 1. Prepare Test data
	// 1.1 Make test objects, injector and pod

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      TestIngressName,
			Namespace: TestNamespace,
			Labels: map[string]string{
				"kind": "build",
			},
		},
	}
	// 1.2. Expected value

	// 2. Initiate Fake client and reconciler
	var resourceContext xContext.Context
	objs := []runtime.Object{
		ingress,
	}
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(konfiguratorv1alpha1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()
	r := &IngressReconciler{
		Client:  fakeClient,
		Log:     ctrl.Log.WithName("Service"),
		Context: &resourceContext,
	}
	// 3. Reconcile
	_, err := r.Reconcile(
		context.TODO(),
		reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      TestIngressName,
				Namespace: TestNamespace,
			},
		},
	)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}

	if len(resourceContext.Ingresses) == 0 {
		t.Fatalf("Service cache is not working for %s", ingress.Name)
	}
	if !apiequality.Semantic.DeepEqual(ingress.Name, resourceContext.Ingresses[0].Name) {
		t.Fatalf("Cached service is not equal to the created one.%s!=%s", ingress.Name, resourceContext.Services[0].Name)
	}

}
