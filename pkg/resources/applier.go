package resources

import (
	"context"
	"errors"
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	k8serr "k8s.io/apimachinery/pkg/api/errors"
	utilcache "k8s.io/apimachinery/pkg/util/cache"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// ErrObjectTerminating is returned when attempting to apply an object that is being deleted.
var ErrObjectTerminating = errors.New("object is being deleted")

// Applier applies a Kubernetes object using server-side apply.
// Implementations may add caching or other behavior around the core apply operation.
type Applier interface {
	// Client returns the underlying Kubernetes client.
	Client() client.Client

	// Apply applies the given object using server-side apply.
	Apply(ctx context.Context, obj client.Object, opts ...client.ApplyOption) error
}

// DefaultApplier applies objects directly via resources.Apply with no additional logic.
type DefaultApplier struct {
	client client.Client
}

// NewDefaultApplier creates a new DefaultApplier with the given client.
func NewDefaultApplier(cli client.Client) *DefaultApplier {
	return &DefaultApplier{client: cli}
}

// Client returns the underlying Kubernetes client.
func (a *DefaultApplier) Client() client.Client {
	return a.client
}

// Apply delegates directly to the resources.Apply function.
func (a *DefaultApplier) Apply(
	ctx context.Context,
	obj client.Object,
	opts ...client.ApplyOption,
) error {
	return Apply(ctx, a.client, obj, opts...)
}

// CachingApplier wraps an Applier and skips the SSA apply when the object's
// resourceVersion hasn't changed since the last successful apply. It uses
// the informer cache (via client.Get) to cheaply check for changes, and only
// calls the inner Applier when necessary.
//
// It also rejects applies to objects that are being deleted, returning
// ErrObjectTerminating.
type CachingApplier struct {
	options CachingApplierOptions

	inner Applier
	cache *utilcache.LRUExpireCache
}

// NewCachingApplier creates a CachingApplier that wraps the given Applier.
// Options can be used to configure cache size and TTL.
func NewCachingApplier(inner Applier, opts ...CachingApplierOption) *CachingApplier {
	a := CachingApplier{
		options: CachingApplierOptions{
			MaxEntries: 1000,
			TTL:        5 * time.Minute,
		},
		inner: inner,
	}
	a.options.ApplyOptions(opts)
	a.cache = utilcache.NewLRUExpireCache(a.options.MaxEntries)

	return &a
}

// Client returns the underlying Kubernetes client from the inner Applier.
func (a *CachingApplier) Client() client.Client {
	return a.inner.Client()
}

// Apply checks the informer cache for the object and skips the SSA apply if
// the resourceVersion matches the last successful apply. On cache miss, expiry,
// or rv mismatch, it delegates to the inner Applier.
//
// When the apply is skipped, the input object is updated with the current server
// state from the Get result so callers always see up-to-date metadata.
func (a *CachingApplier) Apply(
	ctx context.Context,
	obj client.Object,
	opts ...client.ApplyOption,
) error {
	cli := a.inner.Client()

	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(obj.GetObjectKind().GroupVersionKind())

	err := cli.Get(ctx, client.ObjectKeyFromObject(obj), existing)

	switch {
	case k8serr.IsNotFound(err):
		// Object doesn't exist yet, proceed to apply
	case err != nil:
		return fmt.Errorf("failed to get existing object %s: %w", FormatObjectReference(obj), err)
	default:
		// Object exists — check if it's being deleted
		if existing.GetDeletionTimestamp() != nil {
			return fmt.Errorf(
				"cannot apply %s: %w",
				FormatObjectReference(existing),
				ErrObjectTerminating,
			)
		}

		// Check cache: if rv matches our last successful apply, skip
		if a.isCached(existing.GetUID(), existing.GetResourceVersion()) {
			return a.writeBack(cli, existing, obj)
		}
	}

	// Cache miss, expired, rv mismatch, or new object — delegate to inner Applier
	if err := a.inner.Apply(ctx, obj, opts...); err != nil {
		return err
	}

	// Cache the resourceVersion from the successfully applied object
	a.cache.Add(obj.GetUID(), obj.GetResourceVersion(), a.options.TTL)

	return nil
}

// isCached returns true if the given UID has a cached resourceVersion that
// matches the provided rv.
func (a *CachingApplier) isCached(uid types.UID, rv string) bool {
	cached, ok := a.cache.Get(uid)
	if !ok {
		return false
	}

	cachedRV, ok := cached.(string)

	return ok && cachedRV == rv
}

// writeBack copies the server state from the Get result into the caller's
// input object so it reflects the current metadata (UID, resourceVersion, etc.).
func (a *CachingApplier) writeBack(
	cli client.Client,
	from *unstructured.Unstructured,
	into client.Object,
) error {
	if err := FromUnstructured(cli.Scheme(), from, into); err != nil {
		return fmt.Errorf("failed to write cached object state: %w", err)
	}

	return nil
}
