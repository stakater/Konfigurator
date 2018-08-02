package configtemplate

import (
	api "github.com/stakater/Konfigurator/pkg/apis/konfigurator/v1"
)

type ConfigTemplateController struct {
	ConfigTemplate *api.ConfigTemplate
	Deleted        bool
}

func NewController(configTemplate *api.ConfigTemplate, deleted bool) *ConfigTemplateController {
	return &ConfigTemplateController{
		ConfigTemplate: configTemplate,
		Deleted:        deleted,
	}
}

func Reconcile(vr *api.ConfigTemplate) {

}
