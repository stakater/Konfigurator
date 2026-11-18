/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/stakater/konfigurator/pkg/kube"
	"github.com/stakater/konfigurator/pkg/template"
	"github.com/stakater/konfigurator/pkg/webhook"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	k8sutils "github.com/stakater/konfigurator/pkg/utils/k8s"

	finalizerUtil "github.com/stakater/operator-utils/util/finalizer"
	reconcilerUtil "github.com/stakater/operator-utils/util/reconciler"

	v1alpha1 "github.com/stakater/konfigurator/api/v1alpha1"
	xContext "github.com/stakater/konfigurator/pkg/context"
	"github.com/stakater/konfigurator/pkg/kube/mounts"
)

const (
	TemplateFinalizer     string = "konfigurator.stakater.com/konfiguratortemplate"
	GeneratedByAnnotation string = "konfigurator.stakater.com/generated-by"
	DefaultRequeueTime           = 120 * time.Second
)

// KonfiguratorTemplateReconciler reconciles a KonfiguratorTemplate object
type KonfiguratorTemplateReconciler struct {
	client.Client
	Log               logr.Logger
	Scheme            *runtime.Scheme
	RenderedTemplates map[string]string
	XContext          *xContext.Context
	KContext          context.Context
	// Webhook validates rendered output against spec.validationWebhookURL, limited to
	// admin-allowed hosts. Nil denies all webhook calls (H.2).
	Webhook *webhook.Validator
}

// +kubebuilder:rbac:groups=konfigurator.stakater.com,resources=konfiguratortemplates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=konfigurator.stakater.com,resources=konfiguratortemplates/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=konfigurator.stakater.com,resources=konfiguratortemplates,verbs=get;list;watch;create;update;patch;delete

// +kubebuilder:rbac:groups=apps,resources=deployments;daemonsets;statefulsets,verbs=get;list;update;patch;watch
// +kubebuilder:rbac:groups="",resources=services;configmaps;secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the KonfiguratorTemplate object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.0/pkg/reconcile
func (r *KonfiguratorTemplateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.KContext = ctx
	// your logic here

	log := r.Log.WithValues("konfiguratortemplate", req.NamespacedName)
	log.Info("Reconciling template: " + req.Name)
	// Fetch the Tenant instance
	instance := &v1alpha1.KonfiguratorTemplate{}

	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcilerUtil.RequeueWithError(err)
	}

	// Resource is marked for deletion. Handled before validation so that an
	// invalid spec can still be deleted (H.1).
	if instance.DeletionTimestamp != nil {
		log.Info("Deletion timestamp found for instance " + req.Name)
		if finalizerUtil.HasFinalizer(instance, TemplateFinalizer) {

			return r.handleDelete(ctx, req, instance)
		}
		// Finalizer doesn't exist so clean up is already done
		return ctrl.Result{}, nil
	}

	// An invalid spec is permanent until the user edits it: log it, don't retry.
	// (It is not surfaced in .status yet: review B4, story H.6.)
	if ok, err := instance.IsValid(); !ok {
		return reconcilerUtil.ManageError(r.Client, instance, err, false)
	}

	// Add finalizer if it doesn't exist
	if !finalizerUtil.HasFinalizer(instance, TemplateFinalizer) {
		log.Info("Adding finalizer for instance " + req.Name)

		finalizerUtil.AddFinalizer(instance, TemplateFinalizer)

		err := r.Update(ctx, instance)
		if err != nil {
			return reconcilerUtil.ManageError(r.Client, instance, err, false)
		}
		return ctrl.Result{}, nil
	}
	return r.handleCreate(ctx, req, instance)
}

func (r *KonfiguratorTemplateReconciler) handleCreate(ctx context.Context, req ctrl.Request, instance *v1alpha1.KonfiguratorTemplate) (ctrl.Result, error) {
	log := r.Log.WithValues("konfiguratortemplate", req.NamespacedName)
	log.Info(fmt.Sprintf("Initiating sync for KonfiguratorTemplate: %v", instance.Name))

	log.Info("Rendering templates...")
	if err := r.RenderTemplates(ctx, req, instance); err != nil {
		return reconcilerUtil.ManageError(r.Client, instance, err, false)
	}

	log.Info("Creating resources...")
	if err := r.CreateResources(instance.Spec.App.Name, instance.Namespace, instance.Spec.RenderTarget); err != nil {
		return reconcilerUtil.ManageError(r.Client, instance, err, false)
	}

	log.Info("Mounting volumes...")
	if err := r.MountVolumes(instance); err != nil {
		return reconcilerUtil.ManageError(r.Client, instance, err, false)
	}
	if instance.Spec.UpdateFrequency == 0 {
		return reconcilerUtil.RequeueAfter(DefaultRequeueTime)
	}

	return reconcilerUtil.RequeueAfter(time.Duration(instance.Spec.UpdateFrequency) * time.Minute)
}

func (r *KonfiguratorTemplateReconciler) handleDelete(ctx context.Context, req ctrl.Request, instance *v1alpha1.KonfiguratorTemplate) (ctrl.Result, error) {
	log := r.Log.WithValues("konfiguratortemplate", req.NamespacedName)
	log.Info(fmt.Sprintf("Initiating delete for KonfiguratorTemplate: %v", instance.Name))

	// Each cleanup step is decided on its own, so a CR can always be deleted
	// (H.1; review #166 findings 1 and 2):
	//  - unmount only when the app kind is one we can address; a missing app
	//    means there is nothing left to unmount;
	//  - delete the rendered object whenever the render target is valid, even if
	//    app.kind is not: pre-H.1 operators rendered it before failing on the kind.
	if !instance.Spec.App.Kind.IsSupported() {
		log.Info("Skipping unmount: unsupported app.kind " + string(instance.Spec.App.Kind))
	} else if err := r.UnmountVolumes(instance); err != nil {
		if !errors.IsNotFound(err) {
			return reconcilerUtil.ManageError(r.Client, instance, err, false)
		}
		log.Info("App not found; nothing to unmount")
	}

	if !instance.Spec.RenderTarget.IsSupported() {
		log.Info("Skipping rendered-object deletion: unsupported renderTarget " + string(instance.Spec.RenderTarget))
	} else if err := r.DeleteResources(instance.Spec.RenderTarget, instance.Spec.App.Name, instance.Namespace); err != nil && !errors.IsNotFound(err) {
		return reconcilerUtil.ManageError(r.Client, instance, err, false)
	}

	finalizerUtil.DeleteFinalizer(instance, TemplateFinalizer)
	// Update instance
	if err := r.Update(ctx, instance); err != nil {
		return reconcilerUtil.ManageError(r.Client, instance, err, false)
	}
	log.Info(fmt.Sprintf("Deleted KonfiguratorTemplate: %v", instance.Name))

	return ctrl.Result{}, nil
}

func (r *KonfiguratorTemplateReconciler) getGeneratedResourceName(name string) string {
	return strings.ToLower("konfigurator-" + name + "-rendered")
}

func (r *KonfiguratorTemplateReconciler) RenderTemplates(ctx context.Context, req ctrl.Request, instance *v1alpha1.KonfiguratorTemplate) error {
	log := r.Log.WithValues("KonfiguratorTemplate", req.NamespacedName)
	log.Info("RenderTemplates...")
	templates := instance.Spec.Templates
	r.RenderedTemplates = make(map[string]string)

	for fileName, fileData := range templates {

		rendered, err := template.ExecuteString(fileData, r.XContext)
		if err != nil {
			log.Info("Rendetemplate.ExecuteString error:" + err.Error())
			return err
		}
		r.RenderedTemplates[fileName] = string(rendered)
	}
	return r.validateEngine(ctx, instance.Spec.ValidationWebhookURL)
}

// validateEngine sends the rendered output to the template's validation webhook,
// if one is set, through the admin-configured Validator (H.2, review S2).
func (r *KonfiguratorTemplateReconciler) validateEngine(ctx context.Context, webhookURL string) error {
	if webhookURL == "" {
		return nil
	}
	return r.Webhook.Validate(ctx, webhookURL, r.RenderedTemplates)
}

func (r *KonfiguratorTemplateReconciler) CreateResources(name, namespace string, renderTarget v1alpha1.RenderTarget) error {
	log := r.Log.WithValues("konfiguratortemplate", namespace)
	// Generate resource name
	log.Info("CreateResources...")
	resourceName := r.getGeneratedResourceName(name)
	//log.Info("getGeneratedResourceName ...")
	// Check for render target and create resource
	switch renderTarget {
	case v1alpha1.RenderTargetConfigMap:
		return r.createConfigMap(resourceName, namespace)
	case v1alpha1.RenderTargetSecret:
		return r.createSecret(resourceName, namespace)
	}
	return fmt.Errorf("invalid render target %q", renderTarget)
}

func (r *KonfiguratorTemplateReconciler) MountVolumes(instance *v1alpha1.KonfiguratorTemplate) error {
	return r.handleVolumes(
		instance,
		func(mountManager *mounts.MountManager) error {
			err := mountManager.MountVolumes(instance.Spec.App.VolumeMounts)
			if err != nil {
				return fmt.Errorf("failed to assign volume mounts to the specified resource: %w", err)
			}

			return nil
		},
	)
}

func (r *KonfiguratorTemplateReconciler) UnmountVolumes(instance *v1alpha1.KonfiguratorTemplate) error {
	return r.handleVolumes(
		instance,
		func(mountManager *mounts.MountManager) error {
			err := mountManager.UnmountVolumes()
			if err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("failed to unmount volume mounts from the specified resource: %w", err)
			}
			return nil
		},
	)
}

func (r *KonfiguratorTemplateReconciler) DeleteResources(renderTarget v1alpha1.RenderTarget, name, namespace string) error {
	switch renderTarget {
	case v1alpha1.RenderTargetConfigMap:
		return r.deleteConfigMap(name, namespace)
	case v1alpha1.RenderTargetSecret:
		return r.deleteSecret(name, namespace)
	}
	return fmt.Errorf("invalid render target in KonfiguratorTemplate %v", renderTarget)
}

func (r *KonfiguratorTemplateReconciler) handleVolumes(instance *v1alpha1.KonfiguratorTemplate, handleVolumesFunc func(*mounts.MountManager) error) error {
	mountManager, err := r.createMountManager(instance.Spec.App, instance.Namespace, instance.Spec.RenderTarget)
	if err != nil {
		return err
	}

	_, err = k8sutils.CreateOrUpdate(r.KContext, r.Client, mountManager.Target.(client.Object), func() error {

		return handleVolumesFunc(mountManager)

	})
	return err

}

func (r *KonfiguratorTemplateReconciler) createMountManager(app v1alpha1.App, namespace string, renderTarget v1alpha1.RenderTarget) (*mounts.MountManager, error) {
	appObj, err := r.fetchAppObject(app, namespace)
	if err != nil {
		return nil, err
	}

	// Mount volumes to the specified resource
	return mounts.NewManager(
		r.getGeneratedResourceName(app.Name),
		renderTarget,
		appObj), nil
}

func (r *KonfiguratorTemplateReconciler) fetchAppObject(app v1alpha1.App, namespace string) (metav1.Object, error) {
	appObj, err := kube.CreateObjectFromApp(app, namespace)
	if err != nil {
		return nil, err
	}

	// Check if the app exists
	if err := r.Get(
		r.KContext,
		types.NamespacedName{Name: app.Name, Namespace: namespace},
		appObj,
	); err != nil {
		return nil, err
	}

	return appObj, nil
}

func (r *KonfiguratorTemplateReconciler) createConfigMap(name, namespace string) error {
	log := r.Log.WithValues("konfiguratortemplate", namespace)
	// Generate resource name
	log.Info("createConfigMap...")
	configmap := kube.CreateConfigMap(name)
	r.prepareResource(namespace, configmap)
	log.Info("kube.CreateConfigMap...")

	if _, err := k8sutils.CreateOrUpdate(r.KContext, r.Client, configmap, func() error {

		// Add rendered data to resource
		configmap.Data = r.RenderedTemplates
		//Note(Jose): No need to set owner reference because delete manually
		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (r *KonfiguratorTemplateReconciler) createSecret(name, namespace string) error {
	log := r.Log.WithValues("konfiguratortemplate", namespace)
	log.Info("createSecret...")
	secret := kube.CreateSecret(name)
	r.prepareResource(namespace, secret)

	if _, err := k8sutils.CreateOrUpdate(r.KContext, r.Client, secret, func() error {

		// Add rendered data to resource
		secret.StringData = r.RenderedTemplates

		//Note(Jose): No need to set owner reference because it is deleted manually
		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (r *KonfiguratorTemplateReconciler) deleteConfigMap(name, namespace string) error {
	configmap := kube.CreateConfigMap(r.getGeneratedResourceName(name))
	r.prepareResource(namespace, configmap)

	return r.Delete(r.KContext, configmap)
}

func (r *KonfiguratorTemplateReconciler) deleteSecret(name, namespace string) error {
	secret := kube.CreateSecret(r.getGeneratedResourceName(name))
	r.prepareResource(namespace, secret)

	return r.Delete(r.KContext, secret)
}

func (r *KonfiguratorTemplateReconciler) prepareResource(namespace string, resource metav1.Object) {
	resource.SetNamespace(namespace)

	resource.SetAnnotations(map[string]string{
		GeneratedByAnnotation: "konfigurator",
	})
}

// SetupWithManager sets up the controller with the Manager.
func (r *KonfiguratorTemplateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.KonfiguratorTemplate{}).
		Complete(r)
}
