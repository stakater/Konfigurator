package konfiguratortemplate

import (
	"strings"

	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/stakater/Konfigurator/pkg/apis/konfigurator/v1alpha1"
	"github.com/stakater/Konfigurator/pkg/kube"
	"github.com/stakater/Konfigurator/pkg/template"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	GeneratedByAnnotation = "konfigurator.stakater.com/generated-by"
)

type Controller struct {
	Resource          *v1alpha1.KonfiguratorTemplate
	Deleted           bool
	RenderedTemplates map[string]string
}

func NewController(konfiguratorTemplate *v1alpha1.KonfiguratorTemplate, deleted bool) *Controller {
	return &Controller{
		Resource: konfiguratorTemplate,
		Deleted:  deleted,
	}
}

func (controller *Controller) getGeneratedResourceName() string {
	return strings.ToLower("konfigurator-" + controller.Resource.Spec.App.Name + "-rendered")
}

func (controller *Controller) RenderTemplates() error {
	templates := controller.Resource.Spec.Templates

	controller.RenderedTemplates = make(map[string]string)

	for fileName, fileData := range templates {
		// TODO: Inject Context to template
		rendered, err := template.ExecuteString(fileData, nil)
		if err != nil {
			return err
		}
		controller.RenderedTemplates[fileName] = string(rendered)
	}

	return nil
}

func (controller *Controller) CreateResources() error {
	// Generate resource name
	resourceName := controller.getGeneratedResourceName()

	var resourceToCreate metav1.Object

	// Check for render target and create resource
	if controller.Resource.Spec.RenderTarget == v1alpha1.RenderTargetConfigMap {
		resourceToCreate = controller.createConfigMap(resourceName)
	} else {
		resourceToCreate = controller.createSecret(resourceName)
	}

	// Try to create the resource
	if err := sdk.Create(resourceToCreate.(runtime.Object)); err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	// Update the resource if it already exists
	if err := sdk.Update(resourceToCreate.(runtime.Object)); err != nil {
		return err
	}

	return nil
}

func (controller *Controller) createConfigMap(name string) metav1.Object {
	configmap := kube.CreateConfigMap(name)
	controller.prepareResource(configmap)

	// Add rendered data to resource
	configmap.Data = controller.RenderedTemplates

	return configmap
}

func (controller *Controller) createSecret(name string) metav1.Object {
	secret := kube.CreateSecret(name)
	controller.prepareResource(secret)

	// Add rendered data to resource
	secret.Data = kube.ToSecretData(controller.RenderedTemplates)

	return secret
}

func (controller *Controller) prepareResource(resource metav1.Object) {
	resource.SetNamespace(controller.Resource.Namespace)

	resource.SetAnnotations(map[string]string{
		GeneratedByAnnotation: "konfigurator",
	})
}

func (controller *Controller) MountVolumes() error {
	// TODO: Check if the app exists
	// TODO: Mount the generated resource to the app's containers
	return nil
}
