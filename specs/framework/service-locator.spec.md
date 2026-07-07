# Service Locator Specification

**Date:** 2026-07-06
**Status:** Active
**ID:** FW-004
**Related:** [Plugin Architecture](plugin-architecture.spec.md)
**Implements:** `pkg/registry/registry.go`, `pkg/environments/`, `plugins/*/plugin.go`

---

## Purpose

Define the service locator pattern used for dependency injection across the framework, enabling plugins to register and consume services without compile-time coupling.

## Requirements

### Requirement: Global Service Registry

A single global `ServiceRegistry` SHALL store service locator functions keyed by name.

#### Scenario: Service registration
- GIVEN a plugin that needs to register a "Dinosaurs" service
- WHEN `registry.RegisterService("Dinosaurs", locatorFunc)` is called during `init()`
- THEN the registry SHALL store the locator function under the key "Dinosaurs"
- AND subsequent calls with the same key SHALL overwrite the previous registration

### Requirement: Lazy Service Instantiation

Service locator functions SHALL create service instances lazily (on first call), not at registration time.

#### Scenario: Service creation deferral
- GIVEN a registered service locator for "Widgets"
- WHEN the locator function is invoked: `locator(env) -> ServiceLocator -> locator() -> WidgetService`
- THEN the `WidgetService` SHALL be constructed with its dependencies (LockFactory, DAO, EventService)
- AND the construction SHALL only occur when `locator()` is called, not during `RegisterService`

### Requirement: Environment-Based Injection

The `LoadDiscoveredServices` function SHALL pass the environment to each locator function for dependency resolution.

#### Scenario: Loading services
- GIVEN three registered service locators
- WHEN `registry.LoadDiscoveredServices(services, env)` is called
- THEN each locator function SHALL receive the `*environments.Env` instance
- AND the resulting service locator SHALL be set on the `Services` struct via `SetService(name, locator)`

### Requirement: Service Access Pattern

Plugins SHALL provide a package-level `Service(s *environments.Services)` helper function for type-safe service access.

#### Scenario: Accessing a service from another plugin
- GIVEN the dinosaurs plugin needs the events service
- WHEN `events.Service(&env.Services)` is called
- THEN the function SHALL retrieve the "Events" service from the registry
- AND cast it to `services.EventServiceLocator`
- AND invoke the locator to return the `EventService` instance
- AND return `nil` if the service is not registered

### Requirement: Plugin Service Locator Type

Each plugin SHALL define its own `ServiceLocator` type as a function returning the plugin's service interface.

#### Scenario: Dinosaur service locator type
- GIVEN the dinosaurs plugin
- THEN `type ServiceLocator func() DinosaurService` SHALL be defined
- AND `NewServiceLocator(env)` SHALL return a closure capturing the environment's dependencies

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| String-keyed registry over interface types | Avoids import cycles between plugins; name-based lookup is simple |
| Locator function returns a closure | Enables lazy construction with captured dependencies |
| `interface{}` for registry values | Plugins define their own locator types; the registry is type-agnostic |
| Package-level `Service()` helper | Provides type-safe access without requiring callers to know the locator type |
| `SetService`/`GetService` on Services struct | Centralized service container that can be passed through the call chain |
