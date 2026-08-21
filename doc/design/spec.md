# Hierarchical controller-runtime clusters

## Problem

A controller-runtime manager normally gives every controller the same cache and
lifecycle. Larger operators often need controller groups with narrower cache
scopes and an independent shutdown boundary, while still sharing informers for
common Kubernetes types.

The `pkg/hierarchical` package creates child `cluster.Cluster` values under one
parent `manager.Manager`. Each child has a local cache for child-owned types and
falls back to the parent cache for shared types. Children reuse the parent's API
server connection, REST mapper, HTTP client, leader election, and controller
infrastructure; no cluster-side resources are installed.

The hierarchy is deliberately two levels deep. `NewCluster` accepts a
`manager.Manager`, and the returned child is a `cluster.Cluster`, so a child
cannot become another child's parent.

## Public API

```go
type Cluster interface {
    cluster.Cluster
    Parent() manager.Manager
    Stop(context.Context) error
}

func NewCluster(parent manager.Manager, opts ...ClusterOption) (Cluster, error)
func ControllerManagedBy(child Cluster) *builder.Builder

func WithName(name string) ClusterOption
func WithScheme(scheme *runtime.Scheme) ClusterOption
func WithLogger(logger logr.Logger) ClusterOption
func WithByObject(obj client.Object, options cache.ByObject) ClusterOption
func WithNamespaces(namespaces ...string) ClusterOption
```

`ClusterOption` uses the repository's `util.Option[T]` convention. A
`*ClusterOptions` value can therefore also be supplied directly. Names are
required and are expected to be unique within a parent manager.
`ClusterOptions.Validate()` checks the name, scheme, and local namespace scope.

Callers explicitly add every child to the parent before starting the parent:

```go
child, err := hierarchical.NewCluster(mgr,
    hierarchical.WithName("module-a"),
    hierarchical.WithScheme(moduleScheme),
)
if err != nil {
    return err
}
if err := mgr.Add(child); err != nil {
    return err
}

return hierarchical.ControllerManagedBy(child).
    For(&modulev1.Widget{}).
    Owns(&appsv1.Deployment{}).
    Complete(reconciler)
```

The standard controller-runtime builder is returned. A scoped manager adapter
registers the finished controller with the parent manager while exposing the
child’s cache, client, scheme, field indexer, API reader, REST mapper, event
recorder, and logger. This is `sigs.k8s.io/controller-runtime/pkg/builder`, not
the repository’s custom `pkg/builder` package.

## Cache routing

Routing is computed once when the child is constructed:

| Object GVK | Cache |
| --- | --- |
| Present only in the child scheme | Child local cache |
| Present in both schemes | Parent shared cache |
| Present only in the parent scheme | Parent shared cache |
| Configured with `WithByObject` | Child local cache |

For a typed client, the child scheme must contain every type used through the
child. It will normally be the union of shared parent types and child-specific
types. If the same GVK maps to different Go types in the two schemes,
construction fails.

`WithByObject` forces a type into the child cache and passes its selector,
transform, and namespace configuration to controller-runtime. `WithNamespaces`
sets the default namespaces for the local cache only; parent-backed types keep
the parent's cache scope. The option inputs are copied during construction so
later caller mutations do not change cache configuration.

`Get`, `List`, informer lookup, and field indexing route by normalized GVK.
List kinds are normalized to their item kind. `Start` and `WaitForCacheSync`
operate only on the local cache because the parent owns shared informers.
Removing a shared informer would affect unrelated controllers and is rejected
with `ErrSharedInformerRemoval`.

The local cache enables `ReaderFailOnMissingInformer`, preventing accidental
uncached informers from being created by reads.

## Lifecycle

The parent manager starts the child cache through `parent.Add(child)` and starts
child controllers registered by `ControllerManagedBy`. Manager cache readiness
continues to gate controller startup.

`Stop(ctx)` permanently shuts down one child without stopping the parent or
siblings:

1. Reject future controller registration with `ErrClusterStopped`.
2. Cancel child controllers and wait for them to drain.
3. Cancel the child's local cache and wait for it to stop.

The call is idempotent. If its context expires, it returns the context error,
but shutdown continues in the background; a later call can wait for completion.
A stopped child cannot be restarted. Parent cancellation also stops all child
components through their manager-provided contexts.

## Ownership and limitations

- The parent owns shared informers and their scope.
- A child owns only its local cache and its controller cancellation boundary.
- The caller owns child naming and must add children before manager startup.
- Dynamic creation after the parent manager has started is outside the initial
  API contract because controller-runtime manager registration is startup
  oriented.
- Metrics and leader election remain parent-manager concerns.

The scoped-manager and cancellable-runnable approach follows the same broad
composition ideas used by Kubernetes SIGs' multicluster-runtime, adapted here
for multiple cache scopes against one physical API server.
