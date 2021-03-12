package k8s

import (
	"context"
	"fmt"

	"github.com/google/go-cmp/cmp"
	"github.com/stakater/konfigurator/pkg/objects"
	baseutils "github.com/stakater/konfigurator/pkg/utils/base"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	// DEBUG sets debug mode
	DEBUG = baseutils.Getenv("DEBUG", "false")
)

// CreateOrUpdate wraps the function provided by controller-runtime to include
// some additional logging and common functionality across all resources.
func CreateOrUpdate(ctx context.Context, c client.Client, obj client.Object, f controllerutil.MutateFn) (controllerutil.OperationResult, error) {

	return controllerutil.CreateOrUpdate(ctx, c, obj, func() error {
		original := obj.DeepCopyObject()

		if err := f(); err != nil {
			return err
		}

		generateObjectDiff(original, obj)
		return nil
	})
}

func generateObjectDiff(original runtime.Object, modified runtime.Object) {
	if DEBUG == "false" {
		return
	}
	diff := cmp.Diff(original, modified)

	if len(diff) != 0 {
		fmt.Println(diff)
	}
}

func Delete(ctx context.Context, c client.Client, obj client.Object) error {
	return c.Delete(ctx, obj)
}

// GetSecret returns the content of the specified secret.
// Once the key string is not empty, it returns the value of that key.
func GetSecret(r client.Reader, namespace, name string) (map[string][]byte, error) {

	instance := objects.EmptySecret()

	err := r.Get(
		context.Background(),
		types.NamespacedName{
			Namespace: namespace,
			Name:      name,
		},
		instance,
	)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("Secret:%s doesn't exist in %s", name, namespace)
		}
		return nil, err
	}
	return instance.Data, nil
}

func GetSecretSubField(r client.Reader, namespace, name, key string) ([]byte, error) {
	data, err := GetSecret(r, namespace, name)

	if err != nil {
		return nil, err
	}

	if val, ok := data[key]; ok {
		return val, nil
	}
	return nil, fmt.Errorf("Key:%s doesn't exist in Secret:%s", key, name)
}
