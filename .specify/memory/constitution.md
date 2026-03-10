<!--
Sync Impact Report
- Version change: (none) → 1.0.0 (initial ratification)
- Added principles:
  - I. Backward Compatibility
  - II. Multi-Language Correctness
  - III. Test Discipline
  - IV. Minimal Complexity
  - V. Observable & Debuggable
- Added sections:
  - Code Standards
  - Development Workflow
  - Governance
- Removed sections: (none — initial creation)
- Templates requiring updates:
  - .specify/templates/plan-template.md — ✅ no changes needed (Constitution Check section is generic)
  - .specify/templates/spec-template.md — ✅ no changes needed
  - .specify/templates/tasks-template.md — ✅ no changes needed
- Follow-up TODOs: none
-->

# Pulumi Engine Constitution

## Core Principles

### I. Backward Compatibility

All changes to the Pulumi CLI, engine, SDKs, and gRPC protocol MUST
preserve backward compatibility unless a major version bump is planned
and explicitly approved. Breaking changes MUST go through a deprecation
cycle with clear migration guidance. State file format changes MUST be
forward-readable by at least one prior minor version.

**Rationale**: Pulumi manages production infrastructure. A breaking
change can strand users mid-deployment or corrupt state.

### II. Multi-Language Correctness

Features MUST work correctly across all supported language SDKs
(Go, Node.js/TypeScript, Python, .NET, Java, YAML). Code generation
from schemas MUST produce idiomatic output for each target language.
Integration tests MUST cover at least the primary SDK languages
affected by a change.

**Rationale**: Pulumi's core value proposition is language choice.
A feature that works in one SDK but breaks in another violates the
fundamental contract with users.

### III. Test Discipline

- Unit tests MUST accompany all new core logic in `pkg/` and `sdk/`.
- Integration tests MUST cover cross-SDK and engine lifecycle scenarios
  when behavior spans process boundaries or gRPC calls.
- Lifecycle/fuzz tests MUST be updated when resource state machine
  transitions change.
- Tests MUST pass locally before opening a PR; CI enforces this gate.

**Rationale**: The engine orchestrates real cloud resources. Untested
paths risk data loss, orphaned resources, or silent corruption.

### IV. Minimal Complexity

Start with the simplest correct implementation. Abstractions MUST be
justified by at least two concrete use sites. New packages, interfaces,
and indirection layers MUST demonstrate why the existing structure is
insufficient. Prefer clear, linear code over clever patterns.

**Rationale**: A large multi-language codebase compounds complexity.
Each unnecessary abstraction multiplies maintenance cost across every
SDK and codegen target.

### V. Observable & Debuggable

All engine operations MUST produce structured logs via OpenTelemetry
spans. Error messages MUST include enough context (resource URN,
provider, operation) for a user to diagnose failures without reading
source code. Panic recovery MUST capture and report stack traces.

**Rationale**: Users debug infrastructure failures in production.
Opaque errors waste hours and erode trust.

## Code Standards

- All Go files MUST include the Apache 2.0 license header.
- Go code MUST pass `golangci-lint` with the project's `.golangci.yml`
  configuration (includes gosec, revive, misspell, and others).
- Go code MUST be formatted with `gofumpt`.
- Protobuf imports MUST use `google.golang.org/protobuf`, not the
  deprecated `github.com/golang/protobuf`.
- Code SHOULD be self-documenting. Comments are reserved for
  non-obvious logic and public API documentation.
- Every PR MUST include a changelog entry (imperative mood, no
  trailing period) unless labeled `impact/no-changelog-required`.

## Development Workflow

- All changes enter via pull request with squash merge.
- PR descriptions MUST explain **why** the change is needed; the
  **what** is captured by the diff.
- CI MUST pass: lint, build, unit tests, and integration tests for
  affected SDK languages.
- Internal branches MUST use the `{username}/{feature}` naming
  convention.
- Semantic versioning governs all releases. Version is tracked in
  `sdk/.version`.

## Governance

This constitution captures the non-negotiable engineering standards
for the `pulumi/pulumi` repository. It supersedes informal conventions
when they conflict.

- **Amendments** require a PR updating this file with a clear
  rationale. The version MUST be bumped per semver rules (see below).
- **Version policy**: MAJOR for principle removal or redefinition,
  MINOR for new principles or material expansion, PATCH for
  clarifications and typo fixes.
- **Compliance**: PR reviewers SHOULD verify that changes align with
  these principles. The Constitution Check section in plan templates
  codifies this gate for planned features.

**Version**: 1.0.0 | **Ratified**: 2026-03-10 | **Last Amended**: 2026-03-10
