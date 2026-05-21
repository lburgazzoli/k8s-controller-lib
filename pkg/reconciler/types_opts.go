package reconciler

import "github.com/lburgazzoli/k8s-controller-lib/pkg/util"

// ObjectOption configures per-object behavior when added to a Response.
type ObjectOption = util.Option[ObjectOptions]

// ObjectOptions holds per-object configuration.
// Fields use pointer types so nil means "use pipeline default".
type ObjectOptions struct {
	// Ownership controls whether OwnerReferences are set on this object.
	// nil = use pipeline-level default; non-nil = per-object override.
	Ownership *bool
}

// WithOwnership creates an ObjectOption that overrides the pipeline-level ownership setting
// for a specific object. When false, the object will not get OwnerReferences set.
func WithOwnership(enabled bool) ObjectOption {
	return util.FunctionalOption[ObjectOptions](func(opts *ObjectOptions) {
		opts.Ownership = &enabled
	})
}
