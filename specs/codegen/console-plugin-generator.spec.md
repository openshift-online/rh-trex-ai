# Console Plugin Generator Specification

**Date:** 2026-07-06
**Status:** Active
**ID:** CG-004
**Related:** [REST Conventions](../api/rest-conventions.spec.md)
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
  - List and detail views for each entity
  - CRUD forms matching the OpenAPI schema
  - OpenShift Console extension points

### Requirement: API Client Integration

The generated console plugin SHALL include a typed API client matching the OpenAPI specification.

#### Scenario: API calls from UI
- GIVEN the generated console plugin
- WHEN a user creates a new entity via the UI form
- THEN the plugin SHALL call the correct REST endpoint with proper request body

### Requirement: Standalone Project

The generated console plugin SHALL be a standalone Node.js project with its own `package.json`.

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| React/TypeScript | Required by OpenShift Console dynamic plugin architecture |
| OpenAPI-driven forms | Ensures UI fields match API schema; single source of truth |
| Standalone project | Console plugins are deployed independently from the API server |
