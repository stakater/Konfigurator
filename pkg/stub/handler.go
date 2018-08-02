package stub

import (
	"context"

	"github.com/stakater/Konfigurator/pkg/apis/konfigurator/v1alpha1"
	"github.com/stakater/Konfigurator/pkg/controllers/konfiguratortemplate"

	"github.com/operator-framework/operator-sdk/pkg/sdk"
)

func NewHandler() sdk.Handler {
	return &Handler{}
}

type Handler struct {
	// Fill me
}

func (h *Handler) Handle(ctx context.Context, event sdk.Event) error {
	switch o := event.Object.(type) {

	case *v1alpha1.KonfiguratorTemplate:
		return h.HandleKonfiguratorTemplate(konfiguratortemplate.NewController(o, event.Deleted))
	}

	return nil
}

func (h *Handler) HandleKonfiguratorTemplate(controller *konfiguratortemplate.Controller) error {
	if err := controller.RenderTemplates(); err != nil {
		return err
	}
	if err := controller.CreateResources(); err != nil {
		return err
	}
	if err := controller.MountVolumes(); err != nil {
		return err
	}

	return nil
}
