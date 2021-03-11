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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	finalizerUtil "github.com/stakater/operator-utils/util/finalizer"
	reconcilerUtil "github.com/stakater/operator-utils/util/reconciler"
	corev1 "k8s.io/api/core/v1"
	kContext "github.com/stakater/Konfigurator/pkg/context"
)

// ServiceReconciler reconciles a KonfiguratorTemplate object
type ServiceReconciler struct {
	Log    logr.Logger
	Context  *kContext.Context
}

// +kubebuilder:rbac:groups=v1,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=v1,resources=services/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=v1,resources=services/finalizers,verbs=update

func (r *ServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("service", req.NamespacedName)

	// your logic here

	log := r.Log.WithValues("template", req.NamespacedName)
	log.Info("Reconciling template: " + req.Name)
	// Fetch the service instance
	instance := &corev1.Service{}

	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcilerUtil.RequeueWithError(err)
	}

	// Resource is marked for deletion
	if instance.DeletionTimestamp != nil {
		if err := r.RemoveFromContext(instance.name, instance.Namespace); err != nil {
			return reconcilerUtil.RequeueWithError(err)
		}
	}

	if err := r.AddToContext(instance); err != nil {
		return reconcilerUtil.RequeueWithError(err)
	}
	return ctrl.Result{}, nil
}

func (r *ServiceReconciler) RemoveFromContext(name, namespace string) error {
	for index, service := range r.Context.Services {
		if service.Name == name && service.Namespace == namespace {
			// Remove the resource
			r.Context.Services = append(r.Context.Services[:index], r.Context.Services[index+1:]...)
			return nil
		}
	}
	return fmt.Errorf("Could not find service resource %v in current context", name)
}

func (r *ServiceReconciler) AddToContext(instance *corev1.Service) error {
	for index, service := range r.Context.Services {
		if service.Name == instance.Name && service.Namespace == instance.Namespace {
			// Update the resource
			r.Context.Services[index] = *instance
			return nil
		}
	}
	r.Context.Services = append(r.Context.Services, *return.Resource)
	return nil
}
func (r *ServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Service{}).
		Complete(r)
}
