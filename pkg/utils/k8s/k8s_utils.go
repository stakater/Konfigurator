package k8s

import (
	"context"
	"fmt"
	"os"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// DEBUG enables printing object diffs on CreateOrUpdate (env DEBUG, default "false").
var DEBUG = debugFromEnv()

func debugFromEnv() string {
	if v := os.Getenv("DEBUG"); v != "" {
		return v
	}
	return "false"
}

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
