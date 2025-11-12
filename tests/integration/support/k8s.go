package support

import "context"

// AsError wraps a function that returns (value, error) to return only the error.
// This is useful for adapting gomega-matchers/k8s functions to work with
// Gomega's Succeed() matcher.
//
// Example:
//
//	k := k8s.New(client, scheme)
//	g.Eventually(support.AsError(k.Get(&configMap))).WithContext(ctx).Should(Succeed())
func AsError[T any](fn func(context.Context) (T, error)) func(context.Context) error {
	return func(ctx context.Context) error {
		_, err := fn(ctx)

		return err
	}
}
