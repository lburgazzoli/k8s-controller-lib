package reconciler

import (
	"context"
	"fmt"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func AddFinalizer(ctx context.Context, client ctrlClient.Client, o ctrlClient.Object, name string) error {
	if name == "" {
		return nil
	}

	if !ctrlutil.AddFinalizer(o, name) {
		return nil
	}

	err := client.Update(ctx, o)
	if k8serrors.IsConflict(err) {
		return fmt.Errorf("conflict when adding finalizer to %s/%s: %w", o.GetNamespace(), o.GetName(), err)
	}

	if err != nil {
		return fmt.Errorf("failure adding finalizer to %s/%s: %w", o.GetNamespace(), o.GetName(), err)
	}

	return nil
}

func RemoveFinalizer(ctx context.Context, client ctrlClient.Client, o ctrlClient.Object, name string) error {
	if name == "" {
		return nil
	}

	if !ctrlutil.RemoveFinalizer(o, name) {
		return nil
	}

	err := client.Update(ctx, o)
	if k8serrors.IsConflict(err) {
		return fmt.Errorf("conflict when removing finalizer to %s/%s: %w", o.GetNamespace(), o.GetName(), err)
	}

	if err != nil {
		return fmt.Errorf("failure removing finalizer from %s/%s: %w", o.GetNamespace(), o.GetName(), err)
	}

	return nil
}
