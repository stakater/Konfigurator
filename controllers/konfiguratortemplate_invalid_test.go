package controllers

import (
	"context"
	"testing"

	finalizerUtil "github.com/stakater/operator-utils/util/finalizer"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	konfiguratorv1alpha1 "github.com/stakater/konfigurator/api/v1alpha1"
)

func newTemplateReconciler(objs ...runtime.Object) (*KonfiguratorTemplateReconciler, client.Client) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(konfiguratorv1alpha1.AddToScheme(scheme))
	c := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()
	return &KonfiguratorTemplateReconciler{Client: c, Scheme: scheme, Log: ctrl.Log.WithName("test")}, c
}

func invalidTemplate(kind konfiguratorv1alpha1.AppKind, target konfiguratorv1alpha1.RenderTarget) *konfiguratorv1alpha1.KonfiguratorTemplate {
	tpl := &konfiguratorv1alpha1.KonfiguratorTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "bad", Namespace: TestNamespace},
		Spec: konfiguratorv1alpha1.KonfiguratorTemplateSpec{
			App:          konfiguratorv1alpha1.App{Name: "app", Kind: kind},
			RenderTarget: target,
			Templates:    map[string]string{"a": "b"},
		},
	}
	finalizerUtil.AddFinalizer(tpl, TemplateFinalizer)
	return tpl
}

// B3/B11: an invalid spec must neither panic nor render anything.
func TestReconcileInvalidTemplateDoesNotPanicOrRender(t *testing.T) {
	tests := []struct {
		name   string
		kind   konfiguratorv1alpha1.AppKind
		target konfiguratorv1alpha1.RenderTarget
	}{
		{"unknown app kind", "Job", konfiguratorv1alpha1.RenderTargetConfigMap},
		{"lowercase render target", konfiguratorv1alpha1.AppKindDeployment, "configmap"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, c := newTemplateReconciler(invalidTemplate(tt.kind, tt.target))
			req := reconcile.Request{NamespacedName: types.NamespacedName{Name: "bad", Namespace: TestNamespace}}

			if _, err := r.Reconcile(context.TODO(), req); err != nil {
				t.Fatalf("invalid spec is permanent, should not be retried: %v", err)
			}
			name := types.NamespacedName{Name: r.getGeneratedResourceName("app"), Namespace: TestNamespace}
			if err := c.Get(context.TODO(), name, &corev1.Secret{}); !apierrors.IsNotFound(err) {
				t.Fatalf("no Secret may be rendered for an invalid spec, got err=%v", err)
			}
			if err := c.Get(context.TODO(), name, &corev1.ConfigMap{}); !apierrors.IsNotFound(err) {
				t.Fatalf("no ConfigMap may be rendered for an invalid spec, got err=%v", err)
			}
		})
	}
}

// B3/B11: deleting an invalid template must release the finalizer, not hang in Terminating.
func TestDeleteInvalidTemplateRemovesFinalizer(t *testing.T) {
	tests := []struct {
		name   string
		kind   konfiguratorv1alpha1.AppKind
		target konfiguratorv1alpha1.RenderTarget
	}{
		{"unknown app kind", "Job", konfiguratorv1alpha1.RenderTargetConfigMap},
		{"lowercase render target", konfiguratorv1alpha1.AppKindDeployment, "configmap"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tpl := invalidTemplate(tt.kind, tt.target)
			now := metav1.Now()
			tpl.DeletionTimestamp = &now
			r, c := newTemplateReconciler(tpl)
			req := reconcile.Request{NamespacedName: types.NamespacedName{Name: "bad", Namespace: TestNamespace}}

			if _, err := r.Reconcile(context.TODO(), req); err != nil {
				t.Fatalf("reconcile: %v", err)
			}
			out := &konfiguratorv1alpha1.KonfiguratorTemplate{}
			err := c.Get(context.TODO(), req.NamespacedName, out)
			if apierrors.IsNotFound(err) {
				return // finalizer released and deletion completed
			}
			if err != nil {
				t.Fatal(err)
			}
			if finalizerUtil.HasFinalizer(out, TemplateFinalizer) {
				t.Fatal("finalizer still present: CR would hang in Terminating")
			}
		})
	}
}
