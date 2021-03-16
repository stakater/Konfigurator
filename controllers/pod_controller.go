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

// PodReconciler reconciles a KonfiguratorTemplate object
type PodReconciler struct {
	client.Client
	Log     logr.Logger
	Context *xContext.Context
}

// +kubebuilder:rbac:groups=v1,resources=pods,verbs=get;list;watch;

func (r *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("pod", req.NamespacedName)

	// your logic here

	log := r.Log.WithValues("template", req.NamespacedName)
	log.Info("Reconciling pod: " + req.Name)
	// Fetch the pod instance
	instance := &corev1.Pod{}

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

func (r *PodReconciler) RemoveFromContext(name, namespace string) error {
	//log := r.Log.WithValues("RemoveFromContext", namespace)
	//log.Info(fmt.Sprintf("Remove Pod %s from the context", name))
	for index, pod := range r.Context.Pods {
		if pod.Name == name && pod.Namespace == namespace {
			// Remove the resource
			r.Context.Pods = append(r.Context.Pods[:index], r.Context.Pods[index+1:]...)
			return nil
		}
	}
	//log.Info(fmt.Sprintf("Pod %s was not in the context", name))
	//NOTE(Jose): Because the upstream resource is not existing, it will fail forever.
	// We dont need to try remove non-existing resources.
	//return fmt.Errorf("Could not find pod resource %v in current context", name)
	return nil
}

func (r *PodReconciler) AddToContext(instance *corev1.Pod) error {
	for index, pod := range r.Context.Pods {
		if pod.Name == instance.Name && pod.Namespace == instance.Namespace {
			// Update the resource
			r.Context.Pods[index] = *instance
			return nil
		}
	}
	r.Context.Pods = append(r.Context.Pods, *instance)
	return nil
}
func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		Complete(r)
}
