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
	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kContext "github.com/stakater/konfigurator/pkg/context"
	reconcilerUtil "github.com/stakater/operator-utils/util/reconciler"
	"k8s.io/api/extensions/v1beta1"
)

// IngressReconciler reconciles a KonfiguratorTemplate object
type IngressReconciler struct {
	client.Client
	Log     logr.Logger
	Context *kContext.Context
}

// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;

func (r *IngressReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("ingress", req.NamespacedName)

	// your logic here

	log := r.Log.WithValues("template", req.NamespacedName)
	log.Info("Reconciling template: " + req.Name)
	// Fetch the ingress instance
	instance := &v1beta1.Ingress{}

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
		if err := r.RemoveFromContext(instance.Name, instance.Namespace); err != nil {
			return reconcilerUtil.RequeueWithError(err)
		}
	}

	if err := r.AddToContext(instance); err != nil {
		return reconcilerUtil.RequeueWithError(err)
	}
	return ctrl.Result{}, nil
}

func (r *IngressReconciler) RemoveFromContext(name, namespace string) error {
	for index, ingress := range r.Context.Ingresses {
		if ingress.Name == name && ingress.Namespace == namespace {
			// Remove the resource
			r.Context.Ingresses = append(r.Context.Ingresses[:index], r.Context.Ingresses[index+1:]...)
			return nil
		}
	}
	return fmt.Errorf("Could not find ingress resource %v in current context", name)
}

func (r *IngressReconciler) AddToContext(instance *v1beta1.Ingress) error {
	for index, ingress := range r.Context.Ingresses {
		if ingress.Name == instance.Name && ingress.Namespace == instance.Namespace {
			// Update the resource
			r.Context.Ingresses[index] = *instance
			return nil
		}
	}
	r.Context.Ingresses = append(r.Context.Ingresses, *instance)
	return nil
}
func (r *IngressReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.Ingress{}).
		Complete(r)
}
