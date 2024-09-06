package reconciler

import (
	"context"
	"github.com/lburgazzoli/k8s-controller-lib/client"
	"reflect"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ctrl "sigs.k8s.io/controller-runtime"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

type Finalizable interface {
	Finalize(ctx context.Context, obj ctrlClient.Object) error
}

type BaseReconciler[T ctrlClient.Object] struct {
	Log           logr.Logger
	FinalizerName string
	Delegate      reconcile.ObjectReconciler[T]
	Client        *client.Client
}

//nolint:forcetypeassert,wrapcheck,nestif
func (s *BaseReconciler[T]) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	res := reflect.New(reflect.TypeOf(*new(T)).Elem()).Interface().(T)
	if err := s.Client.Get(ctx, req.NamespacedName, res); err != nil {
		return ctrl.Result{}, ctrlClient.IgnoreNotFound(err)
	}

	if res.GetDeletionTimestamp().IsZero() {
		err := AddFinalizer(ctx, s.Client, res, s.FinalizerName)
		if err != nil {
			return ctrl.Result{}, err
		}
	} else {
		f, ok := s.Delegate.(Finalizable)
		if ok {
			err := f.Finalize(ctx, res)
			if err != nil {
				return ctrl.Result{}, err
			}
		}

		err := RemoveFinalizer(ctx, s.Client, res, s.FinalizerName)
		if err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	return s.Delegate.Reconcile(ctx, res)
}
