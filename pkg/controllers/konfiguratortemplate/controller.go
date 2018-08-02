package konfiguratortemplate

import (
	"github.com/stakater/Konfigurator/pkg/apis/konfigurator/v1alpha1"
)

type Controller struct {
	KonfiguratorTemplate *v1alpha1.KonfiguratorTemplate
	Deleted              bool
}

func NewController(konfiguratorTemplate *v1alpha1.KonfiguratorTemplate, deleted bool) *Controller {
	return &Controller{
		KonfiguratorTemplate: konfiguratorTemplate,
		Deleted:              deleted,
	}
}

func Reconcile(vr *v1alpha1.KonfiguratorTemplate) {

}
