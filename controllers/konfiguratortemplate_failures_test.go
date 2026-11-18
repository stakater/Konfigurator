package controllers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	finalizerUtil "github.com/stakater/operator-utils/util/finalizer"
	appsv1 "k8s.io/api/apps/v1"
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
	xContext "github.com/stakater/konfigurator/pkg/context"
	"github.com/stakater/konfigurator/pkg/webhook"
)

// Every failure point of the KonfiguratorTemplate reconciler, with an injected
// fault and the expected outcome (PR #166 review, rounds 3 and 4):
//   - terminal:  only a CR edit or restart can fix it, and both re-trigger the
//     template, so it is returned as reconcile.TerminalError (logged, counted,
//     NOT requeued);
//   - nextCycle: depends on cluster state nothing watches (empty Pods list,
//     webhook allowed:false, container not there yet): nil error and
//     RequeueAfter(updateFrequency), so the template keeps its render rhythm and
//     recovers by itself;
//   - retry:     transient, returned plainly so controller-runtime backs off.
//
// A retry storm starves the single worker; a swallowed error leaves a CR stuck; a
// terminal error on state freezes a template until a human acts. All three are
// bugs, so all three directions are pinned. hack/mutation-check.sh proves each
// error site is covered.

const appName = "app"

type outcome int

const (
	retry outcome = iota
	terminal
	nextCycle
)

func (o outcome) String() string { return [...]string{"retry", "terminal", "nextCycle"}[o] }

var (
	templateGR = schema.GroupResource{Group: "konfigurator.stakater.com", Resource: "konfiguratortemplates"}
	internal   = apierrors.NewInternalError(errors.New("injected"))
)

type fault struct {
	match func(client.Object) bool
	err   error
}

func onTemplate(o client.Object) bool {
	_, ok := o.(*konfiguratorv1alpha1.KonfiguratorTemplate)
	return ok
}
func onDeployment(o client.Object) bool { _, ok := o.(*appsv1.Deployment); return ok }
func onSecret(o client.Object) bool     { _, ok := o.(*corev1.Secret); return ok }
func onConfigMap(o client.Object) bool  { _, ok := o.(*corev1.ConfigMap); return ok }

// faults builds interceptors; a nil fault leaves that verb untouched.
func faults(get, update, create, del *fault) interceptor.Funcs {
	f := interceptor.Funcs{}
	if get != nil {
		f.Get = func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if get.match(obj) {
				return get.err
			}
			return c.Get(ctx, key, obj, opts...)
		}
	}
	if update != nil {
		f.Update = func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			if update.match(obj) {
				return update.err
			}
			return c.Update(ctx, obj, opts...)
		}
	}
	if create != nil {
		f.Create = func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			if create.match(obj) {
				return create.err
			}
			return c.Create(ctx, obj, opts...)
		}
	}
	if del != nil {
		f.Delete = func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			if del.match(obj) {
				return del.err
			}
			return c.Delete(ctx, obj, opts...)
		}
	}
	return f
}

type tplOpt func(*konfiguratorv1alpha1.KonfiguratorTemplate)

func validTemplate(opts ...tplOpt) *konfiguratorv1alpha1.KonfiguratorTemplate {
	tpl := &konfiguratorv1alpha1.KonfiguratorTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "tpl", Namespace: TestNamespace},
		Spec: konfiguratorv1alpha1.KonfiguratorTemplateSpec{
			App: konfiguratorv1alpha1.App{Name: appName, Kind: konfiguratorv1alpha1.AppKindDeployment,
				VolumeMounts: []konfiguratorv1alpha1.VolumeMount{{MountPath: "/etc/x", Container: appName}}},
			RenderTarget: konfiguratorv1alpha1.RenderTargetSecret,
			Templates:    map[string]string{"a.conf": "x"},
		},
	}
	finalizerUtil.AddFinalizer(tpl, TemplateFinalizer)
	for _, o := range opts {
		o(tpl)
	}
	return tpl
}

func deleting(tpl *konfiguratorv1alpha1.KonfiguratorTemplate) {
	now := metav1.Now()
	tpl.DeletionTimestamp = &now
}
func noFinalizer(tpl *konfiguratorv1alpha1.KonfiguratorTemplate) { tpl.Finalizers = nil }
func withWebhook(tpl *konfiguratorv1alpha1.KonfiguratorTemplate) {
	tpl.Spec.ValidationWebhookURL = "http://validator.example.svc/validate"
}
func templates(text string) tplOpt {
	return func(tpl *konfiguratorv1alpha1.KonfiguratorTemplate) {
		tpl.Spec.Templates = map[string]string{"a": text}
	}
}
func every(minutes int) tplOpt {
	return func(tpl *konfiguratorv1alpha1.KonfiguratorTemplate) { tpl.Spec.UpdateFrequency = minutes }
}
func toConfigMap(tpl *konfiguratorv1alpha1.KonfiguratorTemplate) {
	tpl.Spec.RenderTarget = konfiguratorv1alpha1.RenderTargetConfigMap
}

func legacyInvalidTarget() *konfiguratorv1alpha1.KonfiguratorTemplate {
	tpl := invalidTemplate(konfiguratorv1alpha1.AppKindDeployment, "configmap")
	deleting(tpl)
	return tpl
}

// deployment returns the target workload; mounted=true means the rendered
// volume is already attached, so an unmount actually changes (and Updates) it.
func deployment(containerName string, mounted bool) *appsv1.Deployment {
	rendered := "konfigurator-" + appName + "-rendered"
	c := corev1.Container{Name: containerName}
	d := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: appName, Namespace: TestNamespace}}
	if mounted {
		c.VolumeMounts = []corev1.VolumeMount{{Name: rendered, MountPath: "/etc/x"}}
		d.Spec.Template.Spec.Volumes = []corev1.Volume{{Name: rendered}}
	}
	d.Spec.Template.Spec.Containers = []corev1.Container{c}
	return d
}

func newReconciler(f interceptor.Funcs, objs ...runtime.Object) *KonfiguratorTemplateReconciler {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(konfiguratorv1alpha1.AddToScheme(scheme))
	c := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).WithInterceptorFuncs(f).Build()
	// An empty render context: the operator just started, or every pod is gone.
	return &KonfiguratorTemplateReconciler{Client: c, Scheme: scheme, Log: ctrl.Log.WithName("test"), XContext: &xContext.Context{}}
}

func webhookServer(t *testing.T, h http.HandlerFunc) (*webhook.Validator, string) {
	t.Helper()
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)
	u, _ := url.Parse(s.URL)
	return &webhook.Validator{AllowedHosts: []string{u.Host}, Timeout: 100 * time.Millisecond}, s.URL
}

func replying(body string, status int) func(*testing.T) (*webhook.Validator, string) {
	return func(t *testing.T) (*webhook.Validator, string) {
		return webhookServer(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		})
	}
}

func hanging(t *testing.T) (*webhook.Validator, string) {
	return webhookServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Drain the body so the server detects the client hanging up (it only
		// starts its background read after the body is consumed); bound the wait
		// so a failing test cannot hang Server.Close.
		_, _ = io.Copy(io.Discard, r.Body)
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	})
}

func TestReconcileFailureClassification(t *testing.T) {
	tests := []struct {
		name    string
		tpl     *konfiguratorv1alpha1.KonfiguratorTemplate
		objs    []runtime.Object
		faults  interceptor.Funcs
		webhook func(t *testing.T) (*webhook.Validator, string) // optional; a URL overrides the spec
		want    outcome
		after   time.Duration // nextCycle only; 0 means DefaultRequeueTime
	}{
		// ---- create path
		{name: "template Get fails", tpl: validTemplate(),
			faults: faults(&fault{onTemplate, internal}, nil, nil, nil), want: retry},
		{name: "invalid spec", tpl: invalidTemplate("Job", konfiguratorv1alpha1.RenderTargetSecret), want: terminal},
		{name: "finalizer add fails", tpl: validTemplate(noFinalizer),
			faults: faults(nil, &fault{onTemplate, internal}, nil, nil), want: retry},
		{name: "template parse error", tpl: validTemplate(templates("{{ .Pods ")), want: terminal},
		{name: "template execute error on empty live state (index .Pods 0)",
			tpl: validTemplate(templates("{{ (index .Pods 0).Name }}")), want: nextCycle},
		{name: "template execute error honours updateFrequency",
			tpl: validTemplate(templates("{{ (index .Pods 0).Name }}"), every(3)), want: nextCycle, after: 3 * time.Minute},
		{name: "webhook host refused", tpl: validTemplate(withWebhook),
			webhook: func(*testing.T) (*webhook.Validator, string) { return &webhook.Validator{}, "" }, want: terminal},
		{name: "webhook says allowed:false (depends on rendered content)", tpl: validTemplate(withWebhook),
			webhook: replying(`{"allowed":false,"message":"bad"}`, http.StatusOK), want: nextCycle},
		{name: "webhook non-JSON 2xx reply", tpl: validTemplate(withWebhook),
			webhook: replying("<html>maintenance</html>", http.StatusOK), want: nextCycle},
		{name: "webhook timeout", tpl: validTemplate(withWebhook), webhook: hanging, want: retry},
		{name: "webhook 5xx", tpl: validTemplate(withWebhook),
			webhook: replying("", http.StatusInternalServerError), want: retry},
		{name: "rendered Secret create fails", tpl: validTemplate(), objs: []runtime.Object{deployment(appName, false)},
			faults: faults(nil, nil, &fault{onSecret, internal}, nil), want: retry},
		{name: "rendered ConfigMap create fails", tpl: validTemplate(toConfigMap), objs: []runtime.Object{deployment(appName, false)},
			faults: faults(nil, nil, &fault{onConfigMap, internal}, nil), want: retry},
		{name: "app Get fails", tpl: validTemplate(), objs: []runtime.Object{deployment(appName, false)},
			faults: faults(&fault{onDeployment, internal}, nil, nil, nil), want: retry},
		{name: "app not found yet", tpl: validTemplate(), want: retry},
		{name: "container not there yet (GitOps may add it)", tpl: validTemplate(),
			objs: []runtime.Object{deployment("other", false)}, want: nextCycle},
		{name: "app update conflict", tpl: validTemplate(), objs: []runtime.Object{deployment(appName, false)},
			faults: faults(nil, &fault{onDeployment, apierrors.NewConflict(templateGR, appName, nil)}, nil, nil), want: retry},
		// ---- delete path
		{name: "delete: unmount update conflict", tpl: validTemplate(deleting), objs: []runtime.Object{deployment(appName, true)},
			faults: faults(nil, &fault{onDeployment, apierrors.NewConflict(templateGR, appName, nil)}, nil, nil), want: retry},
		{name: "delete: rendered object delete fails", tpl: validTemplate(deleting),
			objs:   []runtime.Object{&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "konfigurator-app-rendered", Namespace: TestNamespace}}},
			faults: faults(nil, nil, nil, &fault{onSecret, internal}), want: retry},
		{name: "delete: legacy secret Get fails", tpl: legacyInvalidTarget(),
			faults: faults(&fault{onSecret, internal}, nil, nil, nil), want: retry},
		{name: "delete: legacy secret delete fails", tpl: legacyInvalidTarget(),
			objs: []runtime.Object{&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "konfigurator-app-rendered", Namespace: TestNamespace,
				Annotations: map[string]string{GeneratedByAnnotation: "konfigurator"}}}},
			faults: faults(nil, nil, nil, &fault{onSecret, internal}), want: retry},
		{name: "delete: finalizer removal conflict", tpl: validTemplate(deleting),
			faults: faults(nil, &fault{onTemplate, apierrors.NewConflict(templateGR, "tpl", nil)}, nil, nil), want: retry},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tpl := tt.tpl.DeepCopy()
			r := newReconciler(tt.faults, append([]runtime.Object{tpl}, tt.objs...)...)
			if tt.webhook != nil {
				v, u := tt.webhook(t)
				r.Webhook = v
				if u != "" {
					tpl.Spec.ValidationWebhookURL = u
					if err := r.Update(context.TODO(), tpl); err != nil {
						t.Fatal(err)
					}
				}
			}
			req := reconcile.Request{NamespacedName: types.NamespacedName{Name: tpl.Name, Namespace: TestNamespace}}

			res, err := r.Reconcile(context.TODO(), req)
			var got outcome
			switch {
			case err == nil && res.RequeueAfter > 0:
				got = nextCycle
			case err == nil:
				t.Fatal("nil error and no RequeueAfter: the failure was swallowed")
			case errors.Is(err, reconcile.TerminalError(nil)):
				got = terminal
			default:
				got = retry
			}
			if got != tt.want {
				t.Fatalf("outcome = %s, want %s; res=%+v err=%v", got, tt.want, res, err)
			}
			if tt.want == nextCycle {
				want := DefaultRequeueTime
				if tt.after > 0 {
					want = tt.after
				}
				if res.RequeueAfter != want {
					t.Fatalf("RequeueAfter = %v, want the render interval %v", res.RequeueAfter, want)
				}
			}
		})
	}
}

// On Kubernetes < 1.30 (no CRD validation ratcheting) the API server rejects the
// finalizer-removal update of a legacy invalid CR with 422. The operator cannot fix
// that, but it must say what the admin has to do, and name Secret for renderTarget:
// pre-H.1 operators rendered a Secret for every non-"ConfigMap" value.
func TestDeleteRejectedByValidationExplainsTheFix(t *testing.T) {
	invalid := apierrors.NewInvalid(schema.GroupKind{Group: "konfigurator.stakater.com", Kind: "KonfiguratorTemplate"}, "tpl",
		field.ErrorList{field.NotSupported(field.NewPath("spec", "app", "kind"), "Job", []string{"Deployment"})})
	r := newReconciler(faults(nil, &fault{onTemplate, invalid}, nil, nil), validTemplate(deleting))

	_, err := r.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "tpl", Namespace: TestNamespace}})
	if err == nil {
		t.Fatal("a 422 on finalizer removal must be returned, not swallowed")
	}
	for _, want := range []string{"spec.app.kind", "renderTarget", "Secret", "1.30"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error must mention %q, got: %v", want, err)
		}
	}
}
