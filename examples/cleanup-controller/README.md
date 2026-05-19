# Cleanup Controller Example

This example demonstrates how to use **finalizers and cleanup actions** in the k8s-controller-lib pipeline to properly clean up resources when a custom resource is deleted.

## What This Example Demonstrates

- **Finalizer management**: Automatically adding and removing finalizers
- **Cleanup actions**: Executing cleanup logic during resource deletion
- **External resource cleanup**: Deleting resources not managed by owner references
- **Proper deletion flow**: Ensuring cleanup completes before finalizer removal

## Overview

### The Problem

When a Kubernetes resource is deleted, any resources with owner references pointing to it are automatically garbage collected. However, there are scenarios where you need custom cleanup logic:

1. **External resources**: Resources without owner references (e.g., ConfigMaps created in different namespaces, external services)
2. **Stateful cleanup**: Releasing locks, updating external systems, or performing cleanup operations
3. **Complex dependencies**: Resources that shouldn't be deleted via owner references

### The Solution: Finalizers

Finalizers allow controllers to perform cleanup before a resource is permanently deleted. The k8s-controller-lib pipeline provides automatic finalizer management:

```go
pipeline.NewPipeline(
    client,
    pipeline.WithFieldOwner(fieldManager),
    pipeline.WithActions(createResources),      // Regular reconciliation
    pipeline.WithCleanupActions(cleanup),       // Cleanup during deletion
)
```

## How It Works

### 1. Normal Reconciliation

When a `CleanupApp` resource is created or updated:
1. The pipeline adds a finalizer automatically (default: `reconciler.k8s-controller-lib/finalizer`)
2. Regular actions execute (in this example: creating a ConfigMap)
3. The resource enters a ready state

### 2. Deletion Flow

When a `CleanupApp` resource is deleted:
1. Kubernetes sets `.metadata.deletionTimestamp`
2. The pipeline detects deletion and **skips regular actions**
3. Cleanup actions execute in **reverse order** (last registered runs first)
4. If all cleanup actions succeed, the finalizer is removed
5. The resource is permanently deleted by Kubernetes

### 3. Cleanup Actions

Cleanup actions have the same signature as regular actions:

```go
func (c *Cleanup) cleanup(
    ctx context.Context,
    req *reconciler.Request,
) error {
    // Perform cleanup logic
    // Delete external resources, release locks, etc.
    return nil
}
```

**Key characteristics:**
- Execute **only during deletion** (when `.metadata.deletionTimestamp` is set)
- Run in **reverse order** of registration
- **All errors are collected** - finalizer is only removed if all succeed
- Regular actions **do not run** during deletion

## Example: CleanupApp Controller

This controller creates a ConfigMap **without owner references** to demonstrate finalizer-based cleanup.

### What the Controller Does

**On Create/Update:**
- Renders a ConfigMap from a template
- Creates the ConfigMap in the same namespace
- **Does not set owner references** (to simulate external resources)

**On Delete:**
- Executes cleanup action
- Deletes the ConfigMap by name
- Removes finalizer after successful cleanup
- Resource is permanently deleted

### Why This Matters

Without a finalizer, when you delete the `CleanupApp`:
- ❌ The ConfigMap would be orphaned (no owner reference means no garbage collection)
- ❌ You'd have to manually clean up the ConfigMap

With a finalizer:
- ✅ The cleanup action deletes the ConfigMap
- ✅ The finalizer ensures cleanup completes before deletion
- ✅ No orphaned resources

## Running the Example

### Prerequisites

- Go 1.24+
- A Kubernetes cluster (or kind/k3s/minikube)
- kubectl configured

### Installation

1. **Generate CRDs and code:**
   ```bash
   make generate
   ```

2. **Install CRDs:**
   ```bash
   make install
   ```

3. **Run the controller:**
   ```bash
   make run KUBECONFIG=~/.kube/config
   ```

### Testing the Controller

1. **Create a CleanupApp:**
   ```bash
   kubectl apply -f config/samples/cleanupapp.yaml
   ```

2. **Verify the ConfigMap was created:**
   ```bash
   kubectl get configmap my-external-config -n default
   ```

   Note: The ConfigMap has **no owner references** - it won't be garbage collected automatically.

3. **Check the finalizer:**
   ```bash
   kubectl get cleanupapp sample-cleanup -o yaml | grep -A 2 finalizers
   ```

   You should see:
   ```yaml
   finalizers:
   - reconciler.k8s-controller-lib/finalizer
   ```

4. **Delete the CleanupApp:**
   ```bash
   kubectl delete cleanupapp sample-cleanup
   ```

5. **Watch the cleanup process:**
   ```bash
   # In the controller logs, you'll see:
   # "running cleanup" namespace="default" name="sample-cleanup"
   # "deleted configmap during cleanup" name="my-external-config" namespace="default"
   ```

6. **Verify the ConfigMap was deleted:**
   ```bash
   kubectl get configmap my-external-config -n default
   # Error from server (NotFound): configmaps "my-external-config" not found
   ```

## Code Walkthrough

### Controller Setup

```go
p, err := pipeline.NewPipeline(
    mgr.GetClient(),
    pipeline.WithFieldOwner(fieldManager),
    pipeline.WithActions(s.manifests),        // Create ConfigMap
    pipeline.WithCleanupActions(s.cleanup),   // Delete ConfigMap on deletion
    pipeline.WithPostApply(watch.New().Watch),
)
```

### Cleanup Action

```go
func (c *Cleanup) cleanup(
    ctx context.Context,
    req *reconciler.Request,
) error {
    app := req.Object.(*cleanupApi.CleanupApp)

    // Delete the ConfigMap by name
    cm := &corev1.ConfigMap{}
    cm.Name = app.Spec.ConfigMapName
    cm.Namespace = app.Namespace

    err := req.Client.Delete(ctx, cm)
    if errors.IsNotFound(err) {
        return nil  // Already deleted
    }

    return err
}
```

## Key Concepts

### Automatic Finalizer Management

The pipeline automatically:
- Adds the finalizer on first reconciliation
- Detects deletion via `.metadata.deletionTimestamp`
- Runs cleanup actions instead of regular actions
- Removes the finalizer only after all cleanup actions succeed

### Cleanup Execution Order

If you have multiple cleanup actions:

```go
pipeline.WithCleanupActions(cleanup1, cleanup2, cleanup3)
```

They execute in **reverse order**: `cleanup3` → `cleanup2` → `cleanup1`

This ensures you can clean up dependencies in the correct order (e.g., delete child resources before parents).

### Error Handling

If any cleanup action returns an error:
- ❌ The finalizer is **not removed**
- ❌ The resource is **not deleted**
- ✅ Cleanup will be retried on the next reconciliation

This prevents orphaned resources if cleanup fails.

### Custom Finalizer Names

By default, the finalizer is `reconciler.k8s-controller-lib/finalizer`. You can customize it:

```go
pipeline.WithCleanupActions(cleanup),
pipeline.WithFinalizer("my-custom-finalizer.example.com"),
```

## Comparison with simple-controller

| Feature | simple-controller | cleanup-controller |
|---------|------------------|-------------------|
| **Resource** | Deployment | ConfigMap |
| **Owner References** | Yes (automatic GC) | No (manual cleanup) |
| **Finalizer** | No | Yes (automatic) |
| **Cleanup Logic** | Not needed | Required |
| **Deletion** | Automatic | Controlled |

## Common Use Cases

### When to Use Finalizers

1. **External resources**: Cloud resources (S3 buckets, RDS instances, load balancers)
2. **Cross-namespace resources**: Resources in other namespaces can't use owner references
3. **State cleanup**: Releasing distributed locks, updating registries
4. **Graceful shutdown**: Draining connections, completing in-flight operations
5. **Notification**: Sending deletion events to external systems

### When NOT to Use Finalizers

1. **Owner references work**: If resources are in the same namespace and can use owner references
2. **Simple resources**: Resources that can be garbage collected automatically
3. **No cleanup needed**: Resources that don't require special cleanup logic

## Troubleshooting

### Resource stuck in "Terminating"

If your CleanupApp is stuck with `.metadata.deletionTimestamp` set:

1. **Check cleanup action errors:**
   ```bash
   kubectl logs <controller-pod> | grep cleanup
   ```

2. **Manually remove the finalizer (emergency only):**
   ```bash
   kubectl patch cleanupapp sample-cleanup -p '{"metadata":{"finalizers":[]}}' --type=merge
   ```

   ⚠️  **Warning**: This may orphan resources! Only use if cleanup is genuinely impossible.

### Cleanup action not executing

- Verify the finalizer is present on the resource
- Check controller logs for errors
- Ensure `WithCleanupActions()` is called when creating the pipeline

## Next Steps

- Explore the [simple-controller](../simple-controller/) for basic pipeline usage
- Review the [pipeline documentation](../../docs/) for advanced features
- Check the integration tests in the library for more examples

## References

- [Kubernetes Finalizers Documentation](https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers/)
- [k8s-controller-lib Pipeline Package](../../pkg/reconciler/pipeline/)
