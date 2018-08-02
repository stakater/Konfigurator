package configtemplate

import (
	"github.com/stakater/Konfigurator/pkg/apis/konfigurator/v1alpha1"
)

type Controller struct {
	ConfigTemplate *v1alpha1.ConfigTemplate
	Deleted        bool
}

func NewController(configTemplate *v1alpha1.ConfigTemplate, deleted bool) *Controller {
	return &Controller{
		ConfigTemplate: configTemplate,
		Deleted:        deleted,
	}
}

func Reconcile(vr *v1alpha1.ConfigTemplate) {

}
