# k8s-controller-lib - Claude Code Instructions

## Project Context

This is a Kubernetes controller library designed to provide reusable utilities and patterns for building Kubernetes controllers. The library emphasizes consistency with controller-runtime, minimal dependencies, and pragmatic design choices.

## Development Phase

**This library is currently in the design phase.**

- Breaking changes are acceptable and expected
- No migration paths or backward compatibility required
- Focus on finding the right APIs and patterns rather than maintaining stability
- Prioritize clean design over incremental changes

## Documentation Map

All project documentation is located in the `docs/` directory:

**Essential reading:**
- @docs/architecture.md
- @docs/development.md

**Feature-specific documentation:**
- @docs/design/conditions.md
- @docs/design/auto-watch.md
- @docs/design/configuration.md

## Development Guidelines

**IMPORTANT:** Before starting any development work:

1. Read @docs/architecture.md to understand the high-level system design and component relationships
2. Read @docs/development.md for detailed coding standards and patterns

All development work must follow the guidelines specified in @docs/development.md:

- Code organization and patterns
- Functional options design (using `util.Option[T]` with `With*` functions)
- Testing practices (vanilla Gomega with dot imports)
- Dependency management (reuse k8s.io libraries, minimize external dependencies)
- Comment and documentation standards (clarify intent, not obvious code)

## Key Reminders

- **Controller-runtime consistency:** Follow patterns from the Kubernetes controller-runtime library
- **Functional options:** Use the generic `util.Option[T]` pattern with `With*` functions; place options in `*_opts.go` files
- **Testing:** Use vanilla Gomega with dot imports, no Ginkgo BDD style
- **Design:** Prefer functions over interfaces unless there's a clear need for abstraction
- **Comments:** Clarify intent and edge cases, don't describe obvious code
- **Dependencies:** Reuse k8s.io libraries; avoid unnecessary external dependencies

## Package-Specific Guidelines

### Architecture and Component Relationships

For understanding how components interact and design decisions, refer to @docs/architecture.md

### Conditions Package

When working with the `pkg/conditions` package, refer to @docs/design/conditions.md

### Auto-Watch Feature

When working with auto-watch functionality, refer to @docs/design/auto-watch.md

### Configuration Management

When adding configuration to controllers, refer to @docs/design/configuration.md

## Before Making Changes

1. Read @docs/architecture.md to understand the high-level design and how components fit together
2. Read @docs/development.md for detailed coding guidelines
3. For feature-specific work, consult the relevant design doc:
   - Conditions: @docs/design/conditions.md
   - Auto-watch: @docs/design/auto-watch.md
   - Configuration: @docs/design/configuration.md
4. Check existing code for established patterns
5. Ensure changes align with controller-runtime conventions
6. Minimize new dependencies and justify any additions

## Before Committing

**CRITICAL: Update documentation to reflect code changes.**

Documentation serves as context for LLMs (Claude, GitHub Copilot, etc.) and must remain accurate:

1. **Review docs for accuracy** - Check if your changes invalidate existing documentation
2. **Update affected docs** - Fix examples, API references, design rationale
3. **Add new docs if needed** - Document new features, patterns, or breaking changes
4. **Verify cross-references** - Ensure links between docs remain valid

**Why this matters:** Inaccurate documentation misleads both humans and AI assistants, causing incorrect code generation and confusion. Documentation accuracy is as critical as code correctness.
