package kube

import (
	"github.com/stakater/konfigurator/api/v1alpha1"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateSecret(name string) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       string(v1alpha1.RenderTargetSecret),
			APIVersion: "v1",
		},
	}
}

func ToSecretData(data map[string]string) (secretData map[string][]byte) {
	secretData = make(map[string][]byte)

	for key, value := range data {
		//NOTE(Jose): Just byte conversion is enough to create a secret
		// This encrypt the data twice so the rendered secret has wrong data.
		//secretData[key] = []byte(base64.StdEncoding.EncodeToString([]byte(value)))
		secretData[key] = []byte(value)
	}
	return
}
