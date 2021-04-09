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
	finalizerUtil "github.com/stakater/operator-utils/util/finalizer"
	ctrl "sigs.k8s.io/controller-runtime"

	konfiguratorv1alpha1 "github.com/stakater/konfigurator/api/v1alpha1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/types"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	TestNamespace               = "default"
	TestPodName                 = "testPod"
	TestPodMetadataInjectorName = "testPodInjector"
	testAnnotations             = map[string]string{
		"config/fluentd.config": "fluentd configuration",
	}
)

func TestPodAnnotationInject(t *testing.T) {

	// 1. Prepare Test data
	// 1.1 Make test objects, injector and pod
	injector := &konfiguratorv1alpha1.PodMetadataInjector{
		ObjectMeta: metav1.ObjectMeta{
			Name:      TestPodMetadataInjectorName,
			Namespace: TestNamespace,
			Labels: map[string]string{
				"kind": "build",
			},
		},
	}
	injector.SetAnnotations(testAnnotations)

	finalizerUtil.AddFinalizer(injector, PodMetadataInjectorFinalizer)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      TestPodName,
			Namespace: TestNamespace,
			Labels: map[string]string{
				"kind": "build",
			},
		},
	}
	// 1.2. Expected value
	expectedPod := pod.DeepCopy()
	expectedPod.SetAnnotations(testAnnotations)

	// 2. Initiate Fake client and reconciler
	var resourceContext xContext.Context
	objs := []runtime.Object{
		injector,
		pod,
	}
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(konfiguratorv1alpha1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()
	r := &PodMetadataInjectorReconciler{
		Client:  fakeClient,
		Scheme:  scheme,
		Log:     ctrl.Log.WithName("PodMetadataInjector"),
		Context: &resourceContext,
	}
	// 3. Reconcile
	_, err := r.Reconcile(
		context.TODO(),
		reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      TestPodMetadataInjectorName,
				Namespace: TestNamespace,
			},
		},
	)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}

	if !apiequality.Semantic.DeepEqual(expectedPod.Annotations, resourceContext.Pods[0].Annotations) {
		t.Fatalf("Pod in the context cache has different annnotations.\n  %#+v != %#+v", resourceContext.Pods[0].Annotations, expectedPod.Annotations)
	}

}
