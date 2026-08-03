# Console Plugin Generator Specification

**Date:** 2026-08-03
**Status:** Active
**ID:** CG-004
**Related:** [REST Conventions](../api/rest-conventions.spec.md), [Testing Standards](../standards/testing.spec.md), [OpenAPI Intermediate Representation](openapi-ir.spec.md)
**Implements:** `scripts/console-plugin-generator/`

---

## Purpose

Define the generator that scaffolds OpenShift Console UI plugins (React/TypeScript) from OpenAPI specifications.

## Requirements

### Requirement: OpenShift Console Plugin Structure

The generated plugin SHALL conform to the OpenShift Console dynamic plugin architecture.

#### Scenario: Plugin scaffold
- GIVEN a project's OpenAPI specification
- WHEN the console plugin generator runs
- THEN a React/TypeScript project SHALL be generated with:
  - List and detail views for each displayable resource view
  - Forms only for operations documented by that resource view
  - OpenShift Console extension points

### Requirement: Shared IR Consumption

The console plugin generator SHALL consume the shared normalized OpenAPI IR and SHALL NOT maintain an independent raw OpenAPI parser or create pages from schema names alone.

#### Scenario: Helper and error schemas
- GIVEN patch-request and error schemas are present but have no resource views
- WHEN the console plugin is generated
- THEN neither schema SHALL produce a navigation entry, list page, or detail page

### Requirement: Scoped View and Action Fidelity

Generated pages, forms, and API calls SHALL preserve the selected resource view's exact operation paths, required scope parameters, and documented capabilities. The plugin SHALL NOT display a CRUD action that the view does not support.

#### Scenario: Parent-scoped read-only view
- GIVEN an Agent inbox view requires `agent_id` and exposes list and interrupt operations but no create operation
- WHEN that view is generated
- THEN its API calls SHALL include `agent_id`
- AND the UI SHALL expose list and interrupt capabilities
- AND it SHALL NOT render a create form

### Requirement: Generated Console Acceptance Tests

The console plugin generator SHALL have acceptance tests that generate a plugin into an isolated temporary directory, install only pinned dependencies, and build and type-check the project. Component and API-client tests SHALL verify navigation and pages for documented resource views, forms and actions for supported capabilities only, required scope propagation, exact request construction, response rendering, and authentication integration. Covered legacy cases SHALL use the same assertions before and after migration to the canonical IR.

#### Scenario: Generated scoped read-only view
- GIVEN a fixture defines a scoped read-only view plus one non-CRUD action
- WHEN the generated plugin's components and API client are tested
- THEN navigation and requests SHALL retain the required scope
- AND the supported action SHALL be present
- AND create, update, or delete controls absent from the fixture SHALL NOT be rendered

### Requirement: API Client Integration

The generated console plugin SHALL include a typed API client matching the OpenAPI specification.

#### Scenario: API calls from UI
- GIVEN the generated console plugin
- WHEN a user creates a new entity via the UI form
- THEN the plugin SHALL call the correct REST endpoint with proper request body

### Requirement: Standalone Project

The generated console plugin SHALL be a standalone Node.js project with its own `package.json`.

#### Scenario: Independent plugin build
- GIVEN a generated console plugin directory
- WHEN its pinned dependencies are installed and its production build is run
- THEN the build SHALL succeed without importing API server source code

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| React/TypeScript | Required by OpenShift Console dynamic plugin architecture |
| OpenAPI-driven forms | Ensures UI fields match API schema; single source of truth |
| Standalone project | Console plugins are deployed independently from the API server |
| Pages project resource views | Supports multiple scopes per kind without treating helper schemas as navigation objects |
| Test rendered capabilities and requests | A successful TypeScript build does not prove that navigation, controls, or API calls match the documented view |
