package controllers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	konfiguratorv1alpha1 "github.com/stakater/konfigurator/api/v1alpha1"
	"github.com/stakater/konfigurator/pkg/webhook"
	finalizerUtil "github.com/stakater/operator-utils/util/finalizer"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// S2/H.2: the reconciler must route validation through its admin-configured
// Validator, never call an arbitrary user-supplied URL.
func TestTemplateWebhookHonorsAllowList(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		_, _ = w.Write([]byte(`{"allowed":true}`))
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)

	tests := []struct {
		name       string
		validator  *webhook.Validator
		wantSecret bool
		wantHits   int32
	}{
		{"host not allow-listed: nothing sent, nothing rendered", &webhook.Validator{AllowedHosts: []string{"validator.example.svc"}}, false, 0},
		{"no validator configured: webhooks disabled", nil, false, 0},
		{"allow-listed host: validated and rendered", &webhook.Validator{AllowedHosts: []string{u.Host}}, true, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			atomic.StoreInt32(&hits, 0)
			tpl := &konfiguratorv1alpha1.KonfiguratorTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "tpl", Namespace: TestNamespace},
				Spec: konfiguratorv1alpha1.KonfiguratorTemplateSpec{
					App:                  konfiguratorv1alpha1.App{Name: "app", Kind: konfiguratorv1alpha1.AppKindDeployment},
					RenderTarget:         konfiguratorv1alpha1.RenderTargetSecret,
					Templates:            map[string]string{"a.conf": "x"},
					ValidationWebhookURL: srv.URL + "/validate",
				},
			}
			finalizerUtil.AddFinalizer(tpl, TemplateFinalizer)
			r, c := newTemplateReconciler(tpl)
			r.Webhook = tt.validator

			_, _ = r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "tpl", Namespace: TestNamespace}})

			err := c.Get(context.TODO(), types.NamespacedName{Name: r.getGeneratedResourceName("app"), Namespace: TestNamespace}, &corev1.Secret{})
			if tt.wantSecret && err != nil {
				t.Fatalf("want rendered Secret, got %v", err)
			}
			if !tt.wantSecret && !apierrors.IsNotFound(err) {
				t.Fatalf("want no Secret, got err=%v", err)
			}
			if got := atomic.LoadInt32(&hits); got != tt.wantHits {
				t.Fatalf("webhook hits = %d, want %d", got, tt.wantHits)
			}
		})
	}
}
