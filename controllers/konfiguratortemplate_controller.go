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
	"github.com/stakater/konfigurator/pkg/kube"
	"github.com/stakater/konfigurator/pkg/template"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"strings"

	"github.com/go-logr/logr"
	k8sutils "github.com/stakater/konfigurator/pkg/utils/k8s"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "github.com/stakater/konfigurator/api/v1alpha1"
	xContext "github.com/stakater/konfigurator/pkg/context"
	"github.com/stakater/konfigurator/pkg/kube/mounts"
	finalizerUtil "github.com/stakater/operator-utils/util/finalizer"
	reconcilerUtil "github.com/stakater/operator-utils/util/reconciler"
)

const (
	TemplateFinalizer     string = "konfigurator.stakater.com/konfiguratortemplate"
	GeneratedByAnnotation string = "konfigurator.stakater.com/generated-by"
)

// KonfiguratorTemplateReconciler reconciles a KonfiguratorTemplate object
type KonfiguratorTemplateReconciler struct {
	client.Client
	Log               logr.Logger
	Scheme            *runtime.Scheme
	RenderedTemplates map[string]string
	XContext          *xContext.Context
	KContext          context.Context
}

// +kubebuilder:rbac:groups=konfigurator.stakater.com,resources=konfiguratortemplates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=konfigurator.stakater.com,resources=konfiguratortemplates/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=konfigurator.stakater.com,resources=konfiguratortemplates/finalizers,verbs=update

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
	_ = r.Log.WithValues("konfiguratortemplate", req.NamespacedName)
	r.KContext = ctx
	// your logic here

	log := r.Log.WithValues("template", req.NamespacedName)
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

	// Validate Custom Resource
	if ok, err := instance.IsValid(); !ok {
		return reconcilerUtil.ManageError(r.Client, instance, err, false)
	}

	// Resource is marked for deletion
	if instance.DeletionTimestamp != nil {
		log.Info("Deletion timestamp found for instance " + req.Name)
		if finalizerUtil.HasFinalizer(instance, TemplateFinalizer) {
			return r.handleDelete(ctx, req, instance)
		}
		// Finalizer doesn't exist so clean up is already done
		return ctrl.Result{}, nil
	}
	// Add finalizer if it doesn't exist
	if !finalizerUtil.HasFinalizer(instance, TemplateFinalizer) {
		log.Info("Adding finalizer for instance " + req.Name)

		finalizerUtil.AddFinalizer(instance, TemplateFinalizer)

		err := r.Client.Update(ctx, instance)
		if err != nil {
			return reconcilerUtil.ManageError(r.Client, instance, err, false)
		}
		return ctrl.Result{}, nil
	}
	return r.handleCreate(ctx, req, instance)
}

func (r *KonfiguratorTemplateReconciler) handleCreate(ctx context.Context, req ctrl.Request, instance *v1alpha1.KonfiguratorTemplate) (ctrl.Result, error) {
	log := r.Log.WithValues("template", req.NamespacedName)
	log.Info(fmt.Sprintf("Initiating sync for KonfiguratorTemplate: %v", instance.Name))

	log.Info("Rendering templates...")
	if err := r.RenderTemplates(ctx, instance.Spec.Templates); err != nil {
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
	return ctrl.Result{}, nil
}
func (r *KonfiguratorTemplateReconciler) handleDelete(ctx context.Context, req ctrl.Request, instance *v1alpha1.KonfiguratorTemplate) (ctrl.Result, error) {
	log := r.Log.WithValues("template", req.NamespacedName)
	log.Info(fmt.Sprintf("Initiating delete for KonfiguratorTemplate: %v", instance.Name))

	// Delegate delete calls to controller
	log.Info("Unmounting volumes...")
	if err := r.UnmountVolumes(instance); err != nil {
		return reconcilerUtil.ManageError(r.Client, instance, err, false)
	}

	log.Info("Deleting resources...")

	err := r.DeleteResources(instance.Spec.RenderTarget, instance.Spec.App.Name, instance.Namespace)
	if err != nil {
		return reconcilerUtil.ManageError(r.Client, instance, err, false)
	}

	log.Info("Deleted KonfiguratorTemplate: %v", instance.Name)

	return ctrl.Result{}, nil
}

func (r *KonfiguratorTemplateReconciler) getGeneratedResourceName(name string) string {
	return strings.ToLower("konfigurator-" + name + "-rendered")
}

func (r *KonfiguratorTemplateReconciler) RenderTemplates(ctx context.Context, templates map[string]string) error {

	r.RenderedTemplates = make(map[string]string)

	for fileName, fileData := range templates {
		rendered, err := template.ExecuteString(fileData, ctx)
		if err != nil {
			return err
		}
		r.RenderedTemplates[fileName] = string(rendered)
	}

	return nil
}

func (r *KonfiguratorTemplateReconciler) CreateResources(name, namespace string, renderTarget v1alpha1.RenderTarget) error {
	// Generate resource name
	resourceName := r.getGeneratedResourceName(name)

	// Check for render target and create resource
	if renderTarget == v1alpha1.RenderTargetConfigMap {
		return r.createConfigMap(resourceName, namespace)
	} else {
		return r.createSecret(resourceName, namespace)
	}
}

func (r *KonfiguratorTemplateReconciler) MountVolumes(instance *v1alpha1.KonfiguratorTemplate) error {
	return r.handleVolumes(
		instance,
		func(mountManager *mounts.MountManager) error {
			err := mountManager.MountVolumes(instance.Spec.App.VolumeMounts)
			if err != nil {
				return fmt.Errorf("Failed to assign volume mounts to the specified resource: %v", err)
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
			if err != nil {
				return fmt.Errorf("Failed to unmount volume mounts from the specified resource: %v", err)
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
	return fmt.Errorf("Invalid render target in KonfiguratorTemplate %v", renderTarget)
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
	appObj := kube.CreateObjectFromApp(app, namespace)

	// Check if the app exists
	if err := r.Get(
		r.KContext,
		types.NamespacedName{Name: app.Name, Namespace: namespace},
		appObj.(client.Object),
	); err != nil {
		return nil, fmt.Errorf("Failed to get the desired app: %v", err)
	}

	return appObj, nil
}

func (r *KonfiguratorTemplateReconciler) createConfigMap(name, namespace string) error {
	configmap := kube.CreateConfigMap(name)

	if _, err := k8sutils.CreateOrUpdate(r.KContext, r.Client, configmap, func() error {

		r.prepareResource(namespace, configmap)

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
	secret := kube.CreateSecret(name)

	if _, err := k8sutils.CreateOrUpdate(r.KContext, r.Client, secret, func() error {

		r.prepareResource(namespace, secret)

		// Add rendered data to resource
		secret.Data = kube.ToSecretData(r.RenderedTemplates)
		//Note(Jose): No need to set owner reference because delete manually
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
