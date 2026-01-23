# Auto-Watch Design

## Overview

The auto-watch feature in the Pipeline provides automatic watch registration for provisioned resources. When enabled, the pipeline automatically sets up watches for all objects created during reconciliation, ensuring the controller is notified when those resources change.

## Core Concepts

### Controller Name from Context

The controller name used for metrics labeling is **retrieved from the context at runtime**, not from configuration.

```go
// Inject controller name into context before reconciliation
ctx = reconciler.WithControllerName(ctx, "myapp-controller")
result, err := pipeline.Reconcile(ctx, obj)
```

The watcher retrieves the controller name from context during watch setup:

```go
// In watch.go:setupWatch()
controllerName := "unknown"
if name, ok := reconciler.ControllerNameFromContext(ctx); ok {
    controllerName = name
}
```

**Rationale:**
- Allows dynamic controller naming based on runtime conditions
- Separates configuration (what to watch) from runtime state (who is watching)
- Avoids the need to pass controller name through multiple layers
- Aligns with Kubernetes patterns where context carries request-scoped data

### Watch Configuration

Auto-watch is configured via `pipeline.WithAutoWatch()`:

```go
p, err := pipeline.NewPipeline(client,
    pipeline.WithFieldOwner("myapp-controller"),
    pipeline.WithAutoWatch(ctrl, cache,
        watch.For(gvks.Deployment,
            watch.WithPredicates(predicates.GenerationChanged()),
        ),
        watch.For(gvks.Secret, watch.Disabled()),
    ),
    pipeline.WithActions(/* ... */),
)
```

**Key points:**
- Watch configuration is static (defined at pipeline creation)
- Controller name is dynamic (provided at reconciliation time)
- Custom predicates and handlers can be specified per GVK
- Watches can be explicitly disabled for specific resource types

### Partial Watching (Metadata Only)

Use `watch.Partial()` to watch only object metadata (labels, annotations, ownership), which is more efficient when spec/status are not needed:

```go
watch.For(gvks.Deployment,
    watch.Partial(),
    watch.WithPredicates(predicates.LabelChanged()),
)
```

**Important caveats:**
- Default predicates are NOT applied with `Partial()` - you must explicitly provide predicates
- Only metadata is available in predicates and event handlers
- Useful for watching resources where only metadata changes matter (e.g., tracking ownership)

## Watch Handlers

### Default Handler: EnqueueRequestForOwnerOrLabel

The auto-watch system uses a unified handler that automatically handles both ownership models:

**Owner References (Standard)**
- Checks for OwnerReferences on the watched object
- Enqueues reconciliation for the owner if found
- Respects the `controller` flag (only controller owners trigger reconciliation)

**Label Fallback (Custom)**
- If no owner reference is found, checks for labels:
  - `controller-lib.k8s.io/owner-name`
  - `controller-lib.k8s.io/owner-namespace`
- Defaults namespace to the object's namespace if label not present

**Behavior**
- OwnerReferences take priority when present
- Seamlessly handles mixed scenarios (some objects owned, some labeled)
- No configuration needed - works automatically

**Example:**

```go
pipeline.WithAutoWatch(ctrl, cache,
    watch.For(gvks.ConfigMap),  // Uses default handler
    watch.For(gvks.Deployment), // Uses default handler
)
```

Both ConfigMaps and Deployments will trigger reconciliation whether they have:
- OwnerReferences set (via `WithOwnership(true)`)
- Labels only (via `WithOwnership(false)` + `WithOwnerLabels(true)`)
- A mix of both

### Custom Handlers

You can still provide custom handlers when needed:

```go
watch.For(gvk, watch.WithHandler(myCustomHandler))
```

## Integration Testing Patterns

### Testing Auto-Watch with Custom Resources

When testing auto-watch with custom resource types, you must:

1. **Create actual CRDs in the test cluster**
   - Cannot use fake objects with fake UIDs
   - Status updates require the CRD to exist in the cluster

2. **Register types in the scheme**
   - Add the custom resource and its list type
   - Add apiextensions types for CRD creation
   - Call `metav1.AddToGroupVersion()` to register CreateOptions, etc.

3. **Create objects in the cluster**
   - Use `client.Create()` to persist objects
   - Add `t.Cleanup()` to delete objects after tests

**Example:**

```go
func TestAutoWatch(t *testing.T) {
    g := NewWithT(t)
    ctx := context.Background()

    // 1. Register all required types
    s := runtime.NewScheme()
    g.Expect(corev1.AddToScheme(s)).To(Succeed())
    g.Expect(apiextensionsv1.AddToScheme(s)).To(Succeed())

    schemeBuilder := runtime.NewSchemeBuilder(func(s *runtime.Scheme) error {
        gv := schema.GroupVersion{Group: "test.example.com", Version: "v1"}
        s.AddKnownTypes(gv, &MyResource{}, &MyResourceList{})
        metav1.AddToGroupVersion(s, gv)
        return nil
    })
    g.Expect(schemeBuilder.AddToScheme(s)).To(Succeed())

    // 2. Start k3s environment
    env, err := k3senv.New(k3senv.WithScheme(s))
    g.Expect(err).ToNot(HaveOccurred())
    g.Expect(env.Start(ctx)).To(Succeed())
    t.Cleanup(func() { _ = env.Stop(ctx) })

    // 3. Create CRD
    crd := &apiextensionsv1.CustomResourceDefinition{
        ObjectMeta: metav1.ObjectMeta{
            Name: "myresources.test.example.com",
        },
        Spec: apiextensionsv1.CustomResourceDefinitionSpec{
            Group: "test.example.com",
            Names: apiextensionsv1.CustomResourceDefinitionNames{
                Kind:     "MyResource",
                ListKind: "MyResourceList",
                Plural:   "myresources",
                Singular: "myresource",
            },
            Scope: apiextensionsv1.NamespaceScoped,
            Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{
                Name:    "v1",
                Served:  true,
                Storage: true,
                Schema: &apiextensionsv1.CustomResourceValidation{
                    OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
                        Type: "object",
                        Properties: map[string]apiextensionsv1.JSONSchemaProps{
                            "status": {
                                Type: "object",
                                XPreserveUnknownFields: func() *bool {
                                    b := true
                                    return &b
                                }(),
                            },
                        },
                    },
                },
                Subresources: &apiextensionsv1.CustomResourceSubresources{
                    Status: &apiextensionsv1.CustomResourceSubresourceStatus{},
                },
            }},
        },
    }
    g.Expect(env.CreateCRD(ctx, crd)).To(Succeed())

    // 4. Create test object in cluster
    owner := &MyResource{
        TypeMeta: metav1.TypeMeta{
            APIVersion: "test.example.com/v1",
            Kind:       "MyResource",
        },
        ObjectMeta: metav1.ObjectMeta{
            Name:      "test-owner",
            Namespace: "default",
        },
    }
    g.Expect(client.Create(ctx, owner)).To(Succeed())
    t.Cleanup(func() { _ = client.Delete(ctx, owner) })

    // 5. Inject controller name and reconcile
    ctx = reconciler.WithControllerName(ctx, "myresource")
    _, err = pipeline.Reconcile(ctx, owner)
    g.Expect(err).ToNot(HaveOccurred())
}
```

### Required Type Methods

Custom resources used in tests must implement:

1. **status.Accessor interface:**
   ```go
   func (r *MyResource) GetStatus() *status.Status { return &r.Status }
   func (r *MyResource) SetStatus(s *status.Status) { r.Status = *s }
   ```

2. **runtime.Object interface:**
   ```go
   func (r *MyResource) DeepCopyObject() runtime.Object
   func (r *MyResource) DeepCopyInto(out *MyResource)
   ```

3. **schema.ObjectKind interface:**
   ```go
   func (r *MyResource) GetObjectKind() schema.ObjectKind
   func (r *MyResource) GroupVersionKind() schema.GroupVersionKind
   func (r *MyResource) SetGroupVersionKind(gvk schema.GroupVersionKind)
   ```

4. **List type with DeepCopy methods:**
   ```go
   type MyResourceList struct {
       metav1.TypeMeta
       metav1.ListMeta
       Items []MyResource
   }
   // + DeepCopyObject() and DeepCopyInto() implementations
   ```

## Historical Context: Why ControllerName Was Removed

### The Problem

Earlier versions included a `ControllerName` field in `AutoWatchOptions`:

```go
// REMOVED - This was misleading
type AutoWatchOptions struct {
    ControllerName string  // Never actually used!
    Controller     controller.Controller
    Cache          cache.Cache
    WatchConfigs   []watch.Config
}
```

This field was **write-only** - it could be set via `pipeline.WithControllerName()` but was never read by the implementation. The actual controller name came from the context.

### Why It Was Confusing

1. **False expectations:** Users believed setting this would configure the controller name
2. **Redundant configuration:** Tests had to set it AND inject via context
3. **No actual effect:** The field was stored but never used
4. **Misleading documentation:** Examples showed usage that had no impact

### The Solution

Removed the field entirely and clarified in documentation:

```go
// WithAutoWatch enables automatic watch setup for provisioned objects.
// The controller and cache parameters are required to register watches.
// Optional Config parameters customize watch behavior for specific GVKs.
//
// Controller name for metrics is retrieved from the context via
// reconciler.WithControllerName(). If not present in context,
// "unknown" will be used as fallback.
```

This makes it clear that:
- Controller name comes from context, not configuration
- It's the caller's responsibility to inject the controller name
- The auto-watch feature focuses on watch configuration, not naming

## Best Practices

### Do: Use Context for Controller Name

```go
// Inject controller name before reconciliation
ctx = reconciler.WithControllerName(ctx, "myapp-controller")
result, err := pipeline.Reconcile(ctx, obj)
```

### Don't: Try to Configure Controller Name Statically

```go
// This pattern no longer exists (and never worked)
pipeline.WithAutoWatch(ctrl, cache,
    pipeline.WithControllerName("myapp"),  // REMOVED
)
```

### Do: Test with Public APIs

```go
// Use the public Reconcile() method
_, err := pipeline.Reconcile(ctx, owner)
```

### Don't: Test with Private Methods

```go
// Don't access private methods like execute()
err := p.execute(ctx, req, resp)  // Private API, subject to change
```

### Do: Create Real Objects in Integration Tests

```go
// Create in cluster
g.Expect(client.Create(ctx, owner)).To(Succeed())
t.Cleanup(func() { _ = client.Delete(ctx, owner) })
```

### Don't: Use Fake UIDs

```go
// This won't work with status updates
owner := &MyResource{
    ObjectMeta: metav1.ObjectMeta{
        UID: "fake-uid",  // Status update will fail
    },
}
```

## Metrics

Auto-watch tracks registered watches via Prometheus metrics:

```go
DynamicWatchedResourcesTotal.WithLabelValues(
    controllerName,  // From context
    gvk.GroupVersion().String(),
    gvk.Kind,
).Set(1)
```

**Labels:**
- `controller`: Controller name from context
- `group_version`: Resource group/version (e.g., "apps/v1")
- `kind`: Resource kind (e.g., "Deployment")

This allows monitoring which resources each controller is watching.
