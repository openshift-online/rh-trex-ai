# DAO Pattern Specification

**Date:** 2026-07-06
**Status:** Active
**ID:** DA-001
**Related:** [Service Locator](../framework/service-locator.spec.md), [Entity Lifecycle](../framework/entity-lifecycle.spec.md)
**Implements:** `pkg/dao/generic.go`, `plugins/*/dao.go`

---

## Purpose

Define the Data Access Object (DAO) pattern used for all database operations, ensuring consistent GORM usage, query building, and session management.

## Requirements

### Requirement: Entity DAO Interface

Each entity DAO SHALL implement CRUD operations: `Get`, `Create`, `Replace`, `Delete`, `FindByIDs`, and `All`.

#### Scenario: Standard CRUD operations
- GIVEN a `DinosaurDao` implementation
- THEN the following methods SHALL exist:
  - `Get(ctx, id) (*Dinosaur, error)` — retrieve by primary key
  - `Create(ctx, dinosaur) (*Dinosaur, error)` — insert new record
  - `Replace(ctx, dinosaur) (*Dinosaur, error)` — update existing record
  - `Delete(ctx, id) error` — soft-delete by ID
  - `FindByIDs(ctx, ids) (DinosaurList, error)` — batch retrieve by IDs
  - `All(ctx) (DinosaurList, error)` — retrieve all records

### Requirement: Session Factory Pattern

All DAO methods SHALL obtain a GORM session via `(*sessionFactory).New(ctx)` to ensure proper transaction propagation.

#### Scenario: Transaction context propagation
- GIVEN a request context containing a GORM transaction (set by middleware)
- WHEN a DAO method calls `(*sessionFactory).New(ctx)`
- THEN the returned `*gorm.DB` SHALL use the existing transaction
- AND if no transaction exists in context, a new session SHALL be created

### Requirement: Generic DAO for Search

The `GenericDao` SHALL provide composable query building for list/search operations.

#### Scenario: Composable query
- GIVEN a search request with filters, sorting, and pagination
- WHEN the generic DAO is used:
  1. `GetInstanceDao(ctx, model)` creates a scoped DAO
  2. `Where(condition)` adds filter clauses
  3. `OrderBy(field)` adds sorting
  4. `Joins(sql)` adds table joins
  5. `Preload(relation)` eager-loads associations
  6. `Count(model, &total)` gets total count before pagination
  7. `Fetch(offset, limit, &results)` retrieves the page
- THEN each method SHALL chain onto the GORM session without side effects on previous calls

### Requirement: Table Relation Extraction

The `GenericDao` SHALL support extracting GORM model relationships for dynamic join construction.

#### Scenario: Relationship discovery
- GIVEN a model with a `belongs_to` relationship to another table
- WHEN `GetTableRelation(fieldName)` is called
- THEN it SHALL return a `TableRelation` with `TableName`, `ColumnName`, `ForeignTableName`, `ForeignColumnName`
- AND only `belongs_to` and `has_many` relationship types SHALL be supported

### Requirement: Mock DAO for Testing

Each entity SHALL have a mock DAO implementation in `plugins/{kind}/mock_dao.go` for unit testing.

#### Scenario: Mock DAO usage
- GIVEN a unit test for the Widget service
- WHEN the mock DAO is configured with expected returns
- THEN the service SHALL use the mock without database access
- AND the mock SHALL implement the same interface as the real DAO

### Requirement: Error Handling

DAO methods SHALL return raw GORM/database errors. Error translation to `ServiceError` types SHALL occur in the service layer.

#### Scenario: Record not found
- GIVEN a `Get` call for a non-existent ID
- WHEN GORM returns `gorm.ErrRecordNotFound`
- THEN the DAO SHALL propagate this error to the caller
- AND the service layer SHALL translate it to `errors.NotFound`

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| DAO returns raw errors (not ServiceError) | Keeps DAO layer database-focused; service layer handles business error translation |
| Session factory pointer (`*db.SessionFactory`) | Allows late initialization during environment setup |
| Generic DAO for search queries | Avoids duplicating pagination/filtering logic across entity DAOs |
| `Count` preserves WHERE and JOIN clauses | Accurate total count for paginated responses |
| Separate `mock_dao.go` per plugin | Co-located with real DAO; easy to discover and maintain |
