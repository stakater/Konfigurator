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

package v1alpha1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"net/url"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

type ValidateRes struct {
	Status bool
	Errors string
}

// log is for logging in this package.
var konfiguratortemplatelog = logf.Log.WithName("konfiguratortemplate-resource")

func (r *KonfiguratorTemplate) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-konfigurator-stakater-com-v1alpha1-konfiguratortemplate,mutating=true,failurePolicy=fail,sideEffects=None,groups=konfigurator.stakater.com,resources=konfiguratortemplates,verbs=create;update,versions=v1alpha1,name=mkonfiguratortemplate.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Defaulter = &KonfiguratorTemplate{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *KonfiguratorTemplate) Default() {
	konfiguratortemplatelog.Info("default", "name", r.Name)

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// +kubebuilder:webhook:path=/validate-konfigurator-stakater-com-v1alpha1-konfiguratortemplate,mutating=false,failurePolicy=fail,sideEffects=None,groups=konfigurator.stakater.com,resources=konfiguratortemplates,verbs=create;update,versions=v1alpha1,name=vkonfiguratortemplate.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &KonfiguratorTemplate{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *KonfiguratorTemplate) ValidateCreate() error {
	konfiguratortemplatelog.Info("validate create", "name", r.Name)

	return r.validateEngine()
}

func (r *KonfiguratorTemplate) validateEngine() error {
	if r.Spec.ValidationWebhookURL == "" {
		return nil
	}
	parsedUrl, err := url.ParseRequestURI(r.Spec.ValidationWebhookURL)
	if err != nil {
		return err
	}
	rawObject, err := json.Marshal(r)
	if err != nil {
		return err
	}
	admissionRequest := v1beta1.AdmissionReview{
		Request: &v1beta1.AdmissionRequest{
			UID: types.UID(r.Name),
			Resource: metav1.GroupVersionResource{
				Group:    r.GroupVersionKind().Group,
				Version:  r.GroupVersionKind().Version,
				Resource: r.GroupVersionKind().Kind,
			},
			Object: runtime.RawExtension{
				Raw:    rawObject,
				Object: r,
			},
		},
	}
	jsonData, err := json.Marshal(admissionRequest)
	if err != nil {
		return err
	}
	konfiguratortemplatelog.Info(fmt.Sprintf("AdmissionRequest: %s",jsonData), "name", r.Name)
	resp, err := http.Post(parsedUrl.String(), "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	var res v1beta1.AdmissionReview
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return err
	}

	if !res.Response.Allowed {
		return fmt.Errorf("Template for %s is invalid: %s", r.Name, res.Response.Result)
	}
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *KonfiguratorTemplate) ValidateUpdate(old runtime.Object) error {
	konfiguratortemplatelog.Info("validate update", "name", r.Name)

	return r.validateEngine()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *KonfiguratorTemplate) ValidateDelete() error {
	konfiguratortemplatelog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
