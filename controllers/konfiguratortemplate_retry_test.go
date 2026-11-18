package controllers

import (
	"context"
	"strings"
	"testing"

	finalizerUtil "github.com/stakater/operator-utils/util/finalizer"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	konfiguratorv1alpha1 "github.com/stakater/konfigurator/api/v1alpha1"
	"github.com/stakater/konfigurator/pkg/webhook"
)

func newTemplateReconcilerWith(funcs interceptor.Funcs, objs ...runtime.Object) (*KonfiguratorTemplateReconciler, client.Client) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(konfiguratorv1alpha1.AddToScheme(scheme))
	c := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).WithInterceptorFuncs(funcs).Build()
	return &KonfiguratorTemplateReconciler{Client: c, Scheme: scheme, Log: ctrl.Log.WithName("test")}, c
}

func deletingTemplate(name string) *konfiguratorv1alpha1.KonfiguratorTemplate {
	tpl := &konfiguratorv1alpha1.KonfiguratorTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: TestNamespace},
		Spec: konfiguratorv1alpha1.KonfiguratorTemplateSpec{
			App:          konfiguratorv1alpha1.App{Name: "gone", Kind: konfiguratorv1alpha1.AppKindDeployment},
			RenderTarget: konfiguratorv1alpha1.RenderTargetConfigMap,
			Templates:    map[string]string{"a": "b"},
		},
	}
	finalizerUtil.AddFinalizer(tpl, TemplateFinalizer)
	now := metav1.Now()
	tpl.DeletionTimestamp = &now
	return tpl
}

var templateGR = schema.GroupResource{Group: "konfigurator.stakater.com", Resource: "konfiguratortemplates"}

// PR #166 review (aayeshaarshad): transient errors on the delete path must be
// returned so controller-runtime retries; today ManageError(..., false) swallows them.
func TestDeleteTransientErrorIsRetried(t *testing.T) {
	conflict := interceptor.Funcs{Update: func(context.Context, client.WithWatch, client.Object, ...client.UpdateOption) error {
		return apierrors.NewConflict(templateGR, "tpl", nil)
	}}
	r, _ := newTemplateReconcilerWith(conflict, deletingTemplate("tpl"))
	req := reconcile.Request{NamespacedName: types.NamespacedName{Name: "tpl", Namespace: TestNamespace}}

	_, err := r.Reconcile(context.TODO(), req)
	if err == nil || !apierrors.IsConflict(err) {
		t.Fatalf("a 409 on finalizer removal must be returned for retry, got err=%v", err)
	}
}

// On Kubernetes < 1.30 (no CRD validation ratcheting) the API server rejects the
// finalizer-removal update of a legacy invalid CR with 422. The operator cannot fix
// that, but it must say what the admin has to do.
func TestDeleteRejectedByValidationExplainsTheFix(t *testing.T) {
	invalid := interceptor.Funcs{Update: func(context.Context, client.WithWatch, client.Object, ...client.UpdateOption) error {
		return apierrors.NewInvalid(schema.GroupKind{Group: "konfigurator.stakater.com", Kind: "KonfiguratorTemplate"}, "tpl",
			field.ErrorList{field.NotSupported(field.NewPath("spec", "app", "kind"), "Job", []string{"Deployment"})})
	}}
	r, _ := newTemplateReconcilerWith(invalid, deletingTemplate("tpl"))
	req := reconcile.Request{NamespacedName: types.NamespacedName{Name: "tpl", Namespace: TestNamespace}}

	_, err := r.Reconcile(context.TODO(), req)
	if err == nil {
		t.Fatal("a 422 on finalizer removal must be returned, not swallowed")
	}
	if !strings.Contains(err.Error(), "edit spec.app.kind") || !strings.Contains(err.Error(), "1.30") {
		t.Fatalf("error must tell the admin how to free the CR, got: %v", err)
	}
}

// A webhook failure (timeout, 5xx, refused host) must be retried with backoff, not
// silently stop the template's periodic re-render.
func TestWebhookFailureIsRetried(t *testing.T) {
	tpl := &konfiguratorv1alpha1.KonfiguratorTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "tpl", Namespace: TestNamespace},
		Spec: konfiguratorv1alpha1.KonfiguratorTemplateSpec{
			App:                  konfiguratorv1alpha1.App{Name: "app", Kind: konfiguratorv1alpha1.AppKindDeployment},
			RenderTarget:         konfiguratorv1alpha1.RenderTargetSecret,
			Templates:            map[string]string{"a": "b"},
			ValidationWebhookURL: "http://validator.example.svc/validate",
		},
	}
	finalizerUtil.AddFinalizer(tpl, TemplateFinalizer)
	r, c := newTemplateReconciler(tpl)
	r.Webhook = &webhook.Validator{} // no allowed hosts: refused
	req := reconcile.Request{NamespacedName: types.NamespacedName{Name: "tpl", Namespace: TestNamespace}}

	_, err := r.Reconcile(context.TODO(), req)
	if err == nil {
		t.Fatal("a refused webhook must return an error so the reconcile is retried")
	}
	name := types.NamespacedName{Name: r.getGeneratedResourceName("app"), Namespace: TestNamespace}
	if err := c.Get(context.TODO(), name, &corev1.Secret{}); !apierrors.IsNotFound(err) {
		t.Fatalf("nothing may be rendered when validation fails, got err=%v", err)
	}
}
