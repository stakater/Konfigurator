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
	xContext "github.com/stakater/konfigurator/pkg/context"
	finalizerUtil "github.com/stakater/operator-utils/util/finalizer"
	reconcilerUtil "github.com/stakater/operator-utils/util/reconciler"
	"k8s.io/apimachinery/pkg/api/errors"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	konfiguratorv1alpha1 "github.com/stakater/konfigurator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// PodMetadataInjectorReconciler reconciles a PodMetadataInjector object
type PodMetadataInjectorReconciler struct {
	client.Client
	Log     logr.Logger
	Scheme  *runtime.Scheme
	Context *xContext.Context
}

const (
	RequeTime                    = 120 * time.Second
	PodMetadataInjectorFinalizer = "konfigurator.stakater.com/PodMetadataInjector"
)

// +kubebuilder:rbac:groups=konfigurator.stakater.com,resources=podmetadatainjectors,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=konfigurator.stakater.com,resources=podmetadatainjectors/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=konfigurator.stakater.com,resources=podmetadatainjectors/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;

func (r *PodMetadataInjectorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("PodMetadataInjector", req.NamespacedName)

	log.Info("Reconciling PodMetadataInjector: " + req.Name)
	instance := &konfiguratorv1alpha1.PodMetadataInjector{}

	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcilerUtil.RequeueWithError(err)
	}
	// your logic here

	// Resource is marked for deletion
	if instance.DeletionTimestamp != nil {
		log.Info("Deletion timestamp found for instance " + req.Name)
		// Finalizer doesn't exist so clean up is already done
		if !finalizerUtil.HasFinalizer(instance, PodMetadataInjectorFinalizer) {
			return ctrl.Result{}, nil
		}

		if err := r.RemoveFromContext(ctx, instance); err != nil {
			return reconcilerUtil.ManageError(r.Client, instance, err, false)
		}
		return ctrl.Result{}, nil
	}
	// Add finalizer if it doesn't exist
	if !finalizerUtil.HasFinalizer(instance, PodMetadataInjectorFinalizer) {
		log.Info("Adding finalizer " + req.Name)

		finalizerUtil.AddFinalizer(instance, PodMetadataInjectorFinalizer)

		err := r.Client.Update(ctx, instance)
		if err != nil {
			return reconcilerUtil.ManageError(r.Client, instance, err, false)
		}
		return ctrl.Result{}, nil
	}

	if err := r.AddToContext(ctx, instance); err != nil {
		return reconcilerUtil.RequeueWithError(err)
	}

	return reconcilerUtil.RequeueAfter(RequeTime)
}

func (r *PodMetadataInjectorReconciler) RemoveFromContext(ctx context.Context, injector *konfiguratorv1alpha1.PodMetadataInjector) error {
	podList := &corev1.PodList{}
	if err := r.List(ctx, podList, client.MatchingLabels(injector.Labels)); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	for _, pod := range podList.Items {
		r.RemoveOnePodFromContext(&pod)
	}
	return nil
}

func (r *PodMetadataInjectorReconciler) AddToContext(ctx context.Context, injector *konfiguratorv1alpha1.PodMetadataInjector) error {
	//log := r.Log.WithValues("RemoveFromContext", namespace)
	//log.Info(fmt.Sprintf("Remove Pod %s from the context", name))
	podList := &corev1.PodList{}
	if err := r.List(ctx, podList, client.MatchingLabels(injector.Labels)); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	for _, pod := range podList.Items {
		pod.SetAnnotations(injector.Annotations)
		r.AddOnePodToContext(&pod)
	}
	return nil
}

func (r *PodMetadataInjectorReconciler) AddOnePodToContext(instance *corev1.Pod) {
	log := r.Log.WithValues("podmetadatainjector", instance.Namespace)

	for index, pod := range r.Context.Pods {
		if pod.Name == instance.Name && pod.Namespace == instance.Namespace {
			r.Context.Pods[index] = *instance
			log.Info("Pod manifest updated in the xcontext:" + instance.Name)
			return
		}
	}
	log.Info("Pod manifest appended in the xcontext:" + instance.Name)
	r.Context.Pods = append(r.Context.Pods, *instance)
}

func (r *PodMetadataInjectorReconciler) RemoveOnePodFromContext(instance *corev1.Pod) {
	log := r.Log.WithValues("podmetadatainjector", instance.Namespace)

	for index, pod := range r.Context.Pods {
		if pod.Name == instance.Name && pod.Namespace == instance.Namespace {
			log.Info("Pod manifest removed from the xcontext:" + instance.Name)
			r.Context.Pods = append(r.Context.Pods[:index], r.Context.Pods[index+1:]...)
			return
		}
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodMetadataInjectorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&konfiguratorv1alpha1.PodMetadataInjector{}).
		Complete(r)
}
