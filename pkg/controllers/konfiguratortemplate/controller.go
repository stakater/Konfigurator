package konfiguratortemplate

import (
	"github.com/stakater/Konfigurator/pkg/apis/konfigurator/v1alpha1"
)

type Controller struct {
	Resource *v1alpha1.KonfiguratorTemplate
	Deleted  bool
}

func NewController(konfiguratorTemplate *v1alpha1.KonfiguratorTemplate, deleted bool) *Controller {
	return &Controller{
		Resource: konfiguratorTemplate,
		Deleted:  deleted,
	}
}

func (controller *Controller) getGeneratedResourceName() string {
	return "konfigurator-" + controller.Resource.Spec.AppName + "-rendered"
}

func (controller *Controller) RenderTemplates() error {
	return nil
}

func (controller *Controller) CreateResources() error {
	// Generate resource name
	//resourceName := controller.getGeneratedResourceName()
	// Check for render target
	if controller.Resource.Spec.RenderTarget == v1alpha1.RenderTargetConfigMap {
	} else {

	}
	// Create resource based on render target
	// Add rendered data to resource
	return nil
}

func (controller *Controller) MountVolumes() error {
	return nil
}
