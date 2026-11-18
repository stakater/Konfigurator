package controllers

import (
	"context"
	"testing"

	finalizerUtil "github.com/stakater/operator-utils/util/finalizer"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	konfiguratorv1alpha1 "github.com/stakater/konfigurator/api/v1alpha1"
)

// Review #166 finding 1: deleting a valid template whose target workload is
// already gone must still release the finalizer (nothing left to unmount).
func TestDeleteTemplateWithMissingAppRemovesFinalizer(t *testing.T) {
	tpl := &konfiguratorv1alpha1.KonfiguratorTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "orphan", Namespace: TestNamespace},
		Spec: konfiguratorv1alpha1.KonfiguratorTemplateSpec{
			App:          konfiguratorv1alpha1.App{Name: "gone", Kind: konfiguratorv1alpha1.AppKindDeployment},
			RenderTarget: konfiguratorv1alpha1.RenderTargetConfigMap,
			Templates:    map[string]string{"a": "b"},
		},
	}
	finalizerUtil.AddFinalizer(tpl, TemplateFinalizer)
	now := metav1.Now()
	tpl.DeletionTimestamp = &now
	r, c := newTemplateReconciler(tpl) // no Deployment "gone" exists
	req := reconcile.Request{NamespacedName: types.NamespacedName{Name: "orphan", Namespace: TestNamespace}}

	if _, err := r.Reconcile(context.TODO(), req); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	out := &konfiguratorv1alpha1.KonfiguratorTemplate{}
	err := c.Get(context.TODO(), req.NamespacedName, out)
	if apierrors.IsNotFound(err) {
		return // finalizer released, deletion completed
	}
	if err != nil {
		t.Fatal(err)
	}
	if finalizerUtil.HasFinalizer(out, TemplateFinalizer) {
		t.Fatal("finalizer still present: CR hangs in Terminating when its app is already deleted")
	}
}

// Review #166 finding 2: the pre-upgrade operator rendered the ConfigMap/Secret
// before panicking on an unsupported app.kind. Deleting such a legacy CR must
// still remove the rendered object, not just release the finalizer.
func TestDeleteLegacyInvalidKindTemplateRemovesRenderedObject(t *testing.T) {
	tpl := invalidTemplate("Job", konfiguratorv1alpha1.RenderTargetConfigMap)
	now := metav1.Now()
	tpl.DeletionTimestamp = &now
	rendered := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name: "konfigurator-app-rendered", Namespace: TestNamespace,
		Annotations: map[string]string{GeneratedByAnnotation: "konfigurator"},
	}}
	r, c := newTemplateReconciler(tpl, rendered)
	req := reconcile.Request{NamespacedName: types.NamespacedName{Name: "bad", Namespace: TestNamespace}}

	if _, err := r.Reconcile(context.TODO(), req); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	cmName := types.NamespacedName{Name: rendered.Name, Namespace: TestNamespace}
	if err := c.Get(context.TODO(), cmName, &corev1.ConfigMap{}); !apierrors.IsNotFound(err) {
		t.Fatalf("rendered ConfigMap must be deleted, got err=%v", err)
	}
	out := &konfiguratorv1alpha1.KonfiguratorTemplate{}
	if err := c.Get(context.TODO(), req.NamespacedName, out); err == nil && finalizerUtil.HasFinalizer(out, TemplateFinalizer) {
		t.Fatal("finalizer still present")
	}
}

// PR #166 review (aayeshaarshad): master's CreateResources fell through to a
// Secret for any renderTarget other than "ConfigMap", so a legacy CR with e.g.
// "configmap" left a Secret behind. Deleting it must remove that Secret.
func TestDeleteLegacyInvalidTargetRemovesSecret(t *testing.T) {
	tpl := invalidTemplate(konfiguratorv1alpha1.AppKindDeployment, "configmap")
	now := metav1.Now()
	tpl.DeletionTimestamp = &now
	leftover := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Name: "konfigurator-app-rendered", Namespace: TestNamespace,
		Annotations: map[string]string{GeneratedByAnnotation: "konfigurator"},
	}}
	r, c := newTemplateReconciler(tpl, leftover)
	req := reconcile.Request{NamespacedName: types.NamespacedName{Name: "bad", Namespace: TestNamespace}}

	if _, err := r.Reconcile(context.TODO(), req); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	name := types.NamespacedName{Name: leftover.Name, Namespace: TestNamespace}
	if err := c.Get(context.TODO(), name, &corev1.Secret{}); !apierrors.IsNotFound(err) {
		t.Fatalf("leftover Secret must be deleted, got err=%v", err)
	}
}
