package reconciler

import "context"

// controllerNameKey is the context key for the controller name.
// Using a private type prevents collisions with other context keys.
type controllerNameKey struct{}

// WithControllerName returns a new context with the controller name attached.
// The controller name is used for metrics labeling and logging.
//
// Example:
//
//	ctx = reconciler.WithControllerName(ctx, "myapp-controller")
func WithControllerName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, controllerNameKey{}, name)
}

// ControllerNameFromContext retrieves the controller name from the context.
// Returns the controller name and true if found, or empty string and false if not found.
//
// Example:
//
//	if name, ok := reconciler.ControllerNameFromContext(ctx); ok {
//	    // use name
//	}
func ControllerNameFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	name, ok := ctx.Value(controllerNameKey{}).(string)

	return name, ok
}
