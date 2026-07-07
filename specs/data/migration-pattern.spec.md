# Migration Pattern Specification

**Date:** 2026-07-06
**Status:** Active
**ID:** DA-002
**Related:** [DAO Pattern](dao-pattern.spec.md), [Plugin Architecture](../framework/plugin-architecture.spec.md)
**Implements:** `pkg/db/migration_registry.go`, `plugins/*/migration.go`

---

## Purpose

Define the database migration pattern using gormigrate with decentralized auto-registration, ensuring deterministic ordering and idempotent execution.

## Requirements

### Requirement: Decentralized Migration Registration

Each plugin SHALL register its migration via `db.RegisterMigration(migration())` in its `init()` function.

#### Scenario: Plugin migration registration
- GIVEN a widgets plugin with a migration function
- WHEN the plugin's `init()` runs during import
- THEN the migration SHALL be appended to the global `migrationRegistry` slice

### Requirement: Deterministic Migration Ordering

Migrations SHALL be sorted by their string ID before execution, ensuring consistent ordering across environments.

#### Scenario: Migration sort order
- GIVEN migrations with IDs "202507010001", "202506150001", "202507020001"
- WHEN `LoadDiscoveredMigrations()` is called
- THEN the returned slice SHALL be ordered: "202506150001", "202507010001", "202507020001"

### Requirement: Migration ID Format

Migration IDs SHALL follow the format `YYYYMMDDHHMM{hash}` where the hash is derived from the kind name.

#### Scenario: Migration ID generation
- GIVEN a kind "Widget" generated on 2026-07-06 at 14:30
- WHEN the generator creates the migration
- THEN the ID SHALL be `20260706143{4-digit-kind-hash}`
- AND the hash SHALL be deterministic for the same kind name (SHA256-based mod 10000)

### Requirement: Migration Function Structure

Each migration SHALL define a local struct matching the desired table schema and use `tx.AutoMigrate`.

#### Scenario: Migration with custom fields
- GIVEN a Widget with fields `Name string`, `MaxSpeed *int`
- WHEN the migration runs
- THEN `tx.AutoMigrate(&Widget{})` SHALL create the `widgets` table
- AND the table SHALL include columns: `id`, `created_at`, `updated_at`, `deleted_at`, `name`, `max_speed`
- AND the local struct SHALL embed `db.Model` (not the API model) to avoid import cycles

### Requirement: Idempotent Execution

Migrations SHALL be idempotent — running `./trex migrate` multiple times SHALL produce the same result.

#### Scenario: Re-running migrations
- GIVEN all migrations have been applied
- WHEN `./trex migrate` is run again
- THEN gormigrate SHALL detect already-applied migrations via its tracking table
- AND no migrations SHALL be re-applied

### Requirement: Field Addition via New Migration

Adding fields to an existing entity SHALL always create a new migration file, never modify an existing one.

#### Scenario: Adding a field to Widgets
- GIVEN the widgets table exists with columns `name`, `max_speed`
- WHEN a `description` field is added
- THEN a new migration file SHALL be created with `tx.AutoMigrate` on the extended struct
- AND the original migration file SHALL remain unchanged

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| Decentralized registry over centralized list | Plugins own their migrations; no central file to merge-conflict |
| Sort by string ID | Timestamp-based IDs naturally sort chronologically |
| Local struct in migration function | Avoids coupling migration to current model state; migration is a snapshot |
| `AutoMigrate` for schema changes | Additive-only (adds columns, doesn't drop); safe for production |
| Kind-name hash in migration ID | Prevents ID collisions when generating multiple kinds at the same time |
