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
	RequeTime = 120 * time.Second
)

// +kubebuilder:rbac:groups=konfigurator.stakater.com,resources=podmetadatainjectors,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=konfigurator.stakater.com,resources=podmetadatainjectors/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=konfigurator.stakater.com,resources=podmetadatainjectors/finalizers,verbs=update

func (r *PodMetadataInjectorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("podmetadatainjector", req.NamespacedName)
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

	if err := r.AddToContext(ctx, instance); err != nil {
		return reconcilerUtil.RequeueWithError(err)
	}

	return reconcilerUtil.RequeueAfter(RequeTime)
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
	for index, pod := range r.Context.Pods {
		if pod.Name == instance.Name && pod.Namespace == instance.Namespace {
			r.Context.Pods[index] = *instance
		}
	}
	r.Context.Pods = append(r.Context.Pods, *instance)
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodMetadataInjectorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&konfiguratorv1alpha1.PodMetadataInjector{}).
		Complete(r)
}
