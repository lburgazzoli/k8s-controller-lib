package resources

import (
	"time"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/util"
)

// CachingApplierOption configures a CachingApplier.
type CachingApplierOption = util.Option[CachingApplierOptions]

// CachingApplierOptions holds configuration for CachingApplier.
type CachingApplierOptions struct {
	MaxEntries int
	TTL        time.Duration
}

// ApplyOptions applies all provided options to this CachingApplierOptions instance.
func (o *CachingApplierOptions) ApplyOptions(opts []CachingApplierOption) *CachingApplierOptions {
	for _, opt := range opts {
		opt.ApplyTo(o)
	}

	return o
}

// WithMaxEntries sets the maximum number of entries in the LRU cache.
func WithMaxEntries(n int) CachingApplierOption {
	return util.FunctionalOption[CachingApplierOptions](func(opts *CachingApplierOptions) {
		opts.MaxEntries = n
	})
}

// WithTTL sets the time-to-live for cache entries. After this duration,
// entries expire and the next apply will perform a full SSA apply
// regardless of resourceVersion.
func WithTTL(ttl time.Duration) CachingApplierOption {
	return util.FunctionalOption[CachingApplierOptions](func(opts *CachingApplierOptions) {
		opts.TTL = ttl
	})
}
