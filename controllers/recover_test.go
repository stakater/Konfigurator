package controllers

import (
	"context"
	"testing"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestRecoverPanics(t *testing.T) {
	tests := []struct {
		name    string
		inner   reconcileFunc
		wantErr bool
	}{
		{"panic becomes error", func(context.Context, ctrl.Request) (ctrl.Result, error) {
			var m map[string]int
			m["boom"] = 1 // nil map write panics
			return ctrl.Result{}, nil
		}, true},
		{"normal result passes through", func(context.Context, ctrl.Request) (ctrl.Result, error) {
			return ctrl.Result{Requeue: true}, nil
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := recoverPanics(tt.inner, ctrl.Log).Reconcile(context.TODO(), reconcile.Request{})
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !res.Requeue {
				t.Fatal("inner result was not passed through")
			}
		})
	}
}
