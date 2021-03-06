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
	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xContext "github.com/stakater/konfigurator/pkg/context"
	reconcilerUtil "github.com/stakater/operator-utils/util/reconciler"
	corev1 "k8s.io/api/core/v1"
)

// ServiceReconciler reconciles a KonfiguratorTemplate object
type ServiceReconciler struct {
	client.Client
	Log     logr.Logger
	Context *xContext.Context
}

// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;

func (r *ServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("service", req.NamespacedName)
	// log := r.Log.WithValues("service", req.NamespacedName)
	// log.Info("Reconciling service: " + req.Name)
	// Fetch the service instance
	instance := &corev1.Service{}

	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			if err := r.RemoveFromContext(instance.Name, instance.Namespace); err != nil {
				return reconcilerUtil.RequeueWithError(err)
			}
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcilerUtil.RequeueWithError(err)
	}

	// Resource is marked for deletion
	if instance.DeletionTimestamp != nil {
		if err := r.RemoveFromContext(instance.Name, instance.Namespace); err != nil {
			return reconcilerUtil.RequeueWithError(err)
		}
	}

	if err := r.AddToContext(instance); err != nil {
		return reconcilerUtil.RequeueWithError(err)
	}
	return ctrl.Result{}, nil
}

func (r *ServiceReconciler) RemoveFromContext(name, namespace string) error {
	//log := r.Log.WithValues("RemoveFromContext", namespace)
	//log.Info(fmt.Sprintf("Remove Service %s from the context", name))
	for index, service := range r.Context.Services {
		if service.Name == name && service.Namespace == namespace {
			// Remove the resource
			r.Context.Services = append(r.Context.Services[:index], r.Context.Services[index+1:]...)
			return nil
		}
	}
	//log.Info(fmt.Sprintf("Service %s was not in the context", name))
	//NOTE(Jose): Because the upstream resource is not existing, it will fail forever.
	// We dont need to try remove non-existing resources.
	//return fmt.Errorf("Could not find service resource %v in current context", name)
	return nil
}

func (r *ServiceReconciler) AddToContext(instance *corev1.Service) error {
	for index, service := range r.Context.Services {
		if service.Name == instance.Name && service.Namespace == instance.Namespace {
			// Update the resource
			r.Context.Services[index] = *instance
			return nil
		}
	}
	r.Context.Services = append(r.Context.Services, *instance)
	return nil
}
func (r *ServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Service{}).
		Complete(r)
}
