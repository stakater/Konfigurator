/*
1. If tenant is not existing, don't requeue again
2. Space labels are inherited to the namespace
3. Check if rolebinding created
4.
*/

package controllers

import (
	"context"
	finalizerUtil "github.com/stakater/operator-utils/util/finalizer"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	ctrl "sigs.k8s.io/controller-runtime"

	konfiguratorv1alpha1 "github.com/stakater/konfigurator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	TestKonfiguratorTemplateName = "testTemplate"
	TestDaemonsetName            = "fluentd"
	TestContainerName            = "fluentd"
)

func TestTemplateRendering(t *testing.T) {

	// 1. Prepare Test data
	// 1.1 Make test objects, injector and pod
	konfiguratorTemplate := &konfiguratorv1alpha1.KonfiguratorTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      TestKonfiguratorTemplateName,
			Namespace: TestNamespace,
		},
		Spec: konfiguratorv1alpha1.KonfiguratorTemplateSpec{
			RenderTarget: konfiguratorv1alpha1.RenderTargetSecret,
			App: konfiguratorv1alpha1.App{
				Name: TestDaemonsetName,
				Kind: konfiguratorv1alpha1.AppKindDaemonSet,
				VolumeMounts: []konfiguratorv1alpha1.VolumeMount{
					{
						MountPath: "/fluentd/etc/conf",
						Container: TestContainerName,
					},
				},
			},
			Templates: map[string]string{
				"fluent.conf": `
<source>
	@type tail
	path /var/log/containers/*.log
	pos_file /var/log/es-containers.log.pos
	time_format %Y-%m-%dT%H:%M:%S.%N
	tag kubernetes.*
	format json
	read_from_head true
</source>

<filter kubernetes.var.log.containers.**.log>
	@type kubernetes_metadata
</filter>

<match **>
	@type stdout
</match>
`,
			},
		},
	}

	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      TestDaemonsetName,
			Namespace: TestNamespace,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "fluentd",
				},
			},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: TestContainerName,
						},
					},
				},
			},
		},
	}

	finalizerUtil.AddFinalizer(konfiguratorTemplate, TemplateFinalizer)

	// 2. Initiate Fake client and reconciler
	objs := []runtime.Object{
		konfiguratorTemplate,
		ds,
	}
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(konfiguratorv1alpha1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()
	r := &KonfiguratorTemplateReconciler{
		Client: fakeClient,
		Scheme: scheme,
		Log:    ctrl.Log.WithName("KonfiguratorTemplate"),
	}

	// 1.2. Expected value
	expectedVolumeName := r.getGeneratedResourceName(konfiguratorTemplate.Spec.App.Name)
	expectedSecretName := r.getGeneratedResourceName(konfiguratorTemplate.Spec.App.Name)
	// 3. Reconcile
	_, err := r.Reconcile(
		context.TODO(),
		reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      TestKonfiguratorTemplateName,
				Namespace: TestNamespace,
			},
		},
	)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}

	outSecret := &corev1.Secret{}
	err = fakeClient.Get(context.TODO(), types.NamespacedName{Name: expectedSecretName, Namespace: TestNamespace}, outSecret)
	if err != nil {
		t.Fatal(err)
	}
	if !apiequality.Semantic.DeepEqual(konfiguratorTemplate.Spec.Templates, outSecret.StringData) {
		t.Fatalf("The rendered secret has different content with the template.\n  %#+v", outSecret.StringData)
	}

	outDs := &appsv1.DaemonSet{}
	err = fakeClient.Get(context.TODO(), types.NamespacedName{Name: TestDaemonsetName, Namespace: TestNamespace}, outDs)
	if err != nil {
		t.Fatal(err)
	}
	if len(outDs.Spec.Template.Spec.Volumes) == 0 {
		t.Fatalf("The template is not rendered in the volume.\n  %#+v", outDs.Spec.Template.Spec.Volumes)
	}
	if !apiequality.Semantic.DeepEqual(outDs.Spec.Template.Spec.Volumes[0].Name, expectedVolumeName) {
		t.Fatalf("The template is rendered incorrectly in the volume.\n  %#+v", outDs.Spec.Template.Spec.Volumes)
	}
}
