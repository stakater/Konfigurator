package controllers

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// recoverPanics wraps a reconciler so that a panic fails one reconcile (logged,
// returned as an error, retried with backoff) instead of crash-looping the whole
// operator (review B3). Stand-in for controller-runtime's RecoverPanic option,
// which v0.9 lacks; replace it when H.7 upgrades the library.
func recoverPanics(inner reconcile.Reconciler, log logr.Logger) reconcile.Reconciler {
	return reconcileFunc(func(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
		defer func() {
			if p := recover(); p != nil {
				log.Error(fmt.Errorf("%v", p), "recovered panic in reconciler",
					"request", req.NamespacedName, "stack", string(debug.Stack()))
				res, err = ctrl.Result{}, fmt.Errorf("panic reconciling %s: %v", req.NamespacedName, p)
			}
		}()
		return inner.Reconcile(ctx, req)
	})
}

// reconcileFunc adapts a function to reconcile.Reconciler.
type reconcileFunc func(context.Context, ctrl.Request) (ctrl.Result, error)

func (f reconcileFunc) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return f(ctx, req)
}
