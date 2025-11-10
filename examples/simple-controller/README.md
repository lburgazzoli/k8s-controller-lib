# Simple Controller Example

A complete example demonstrating pipeline-based reconciliation with the k8s-controller-lib.

## What It Demonstrates

- **Pipeline Pattern**: Orchestrating reconciliation through a pipeline of actions
- **Template-Based Resource Generation**: Using Go templates to generate Kubernetes manifests
- **Server-Side Apply**: Applying resources with field manager tracking
- **Status Management**: Automatic status updates with conditions and ObservedGeneration
- **Auto-Watch**: Dynamically watching generated Deployment resources

## Custom Resource

The `SimpleApp` CRD defines a simple application with:
- **`spec.image`** - Container image to deploy
- **`spec.replicas`** - Number of replicas (default: 1)
- **`spec.port`** - Container port (default: 8080)

The controller generates a Deployment based on these specs using Go templates.

## Running the Example

```bash
# Install CRDs
kubectl apply -f config/crds/bases

# Run the controller locally
go run cmd/main.go

# Create a SimpleApp
kubectl apply -f config/samples/simpleapp_v1alpha1_simpleapp.yaml
```

## Tests

The example includes comprehensive integration tests using k3s-envtest (400+ lines):

```bash
# Run all tests
go test -v ./internal/controller/simple/...

# Specific test
go test -v ./internal/controller/simple/... -run TestSimpleControllerIntegration
```

### Test Coverage

- Create SimpleApp creates Deployment
- Update SimpleApp updates Deployment
- Status updates with ObservedGeneration and conditions
- Default values are applied correctly
- Field manager is set correctly
- Multiple SimpleApps in different namespaces
- Template renders resource limits correctly

See [`internal/controller/simple/controller_test.go`](internal/controller/simple/controller_test.go) for implementation details.

## Architecture

1. **Controller Setup** ([`internal/controller/simple/controller.go`](internal/controller/simple/controller.go))
   - Initializes Go template engine with embedded templates
   - Creates Pipeline with auto-watch for Deployments
   - Registers with controller-runtime manager

2. **Reconciliation** (`Reconcile` function)
   - Delegates to Pipeline which orchestrates actions
   - Automatically updates status with conditions

3. **Resource Generation** (`manifests` action)
   - Renders Go templates with SimpleApp spec as values
   - Applies labels via transformer function
   - Returns generated Deployments to Pipeline

4. **Pipeline Execution**
   - Applies resources using server-side apply
   - Automatically watches Deployments for changes
   - Updates status with ProvisioningSucceeded condition

## Key Patterns

**Template-Based Generation**:
```go
e, err := gotemplate.NewEngine(gotemplate.Source{
    FS:   templates,  // Embedded FS with *.yaml.tmpl files
    Path: "templates/*.yaml.tmpl",
})
```

**Pipeline Actions**:
```go
pipeline.WithActions(s.manifests)  // Function returning resources
```

**Auto-Watch**:
```go
pipeline.WithAutoWatch(c, mgr.GetCache())  // Watches generated resources
```

For more details on these patterns, see:
- [Pipeline Design](../../docs/design/auto-watch.md)
- [Development Guidelines](../../docs/development.md)
