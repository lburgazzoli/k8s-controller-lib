# k8s-controller-lib - Claude Code Instructions

## Project Context

This is a Kubernetes controller library designed to provide reusable utilities and patterns for building Kubernetes controllers. The library emphasizes consistency with controller-runtime, minimal dependencies, and pragmatic design choices.

## Development Guidelines

**IMPORTANT:** All development work on this project must follow the guidelines specified in `docs/development.md`. Read and adhere to those guidelines for:

- Code organization and patterns
- Functional options design
- Testing practices
- Dependency management
- Comment and documentation standards

## Key Reminders

- **Controller-runtime consistency:** Follow patterns from the Kubernetes controller-runtime library
- **Functional options:** Use the With* pattern and struct-based options; place options in `*_opts.go` files
- **Testing:** Use vanilla Gomega with dot imports, no Ginkgo BDD style
- **Design:** Prefer functions over interfaces unless there's a clear need for abstraction
- **Comments:** Clarify intent and edge cases, don't describe obvious code
- **Dependencies:** Reuse k8s.io libraries; avoid unnecessary external dependencies

## Package-Specific Guidelines

### Conditions Package

When working with the `pkg/conditions` package, refer to `docs/design/conditions.md` for:
- API design and usage patterns
- Aggregation logic and customization
- Integration with `metav1.Condition` and `meta` package
- Examples and best practices

## Before Making Changes

1. Read `docs/development.md` for detailed guidelines
2. For conditions work, consult `docs/design/conditions.md`
3. Check existing code for established patterns
4. Ensure changes align with controller-runtime conventions
5. Minimize new dependencies and justify any additions
