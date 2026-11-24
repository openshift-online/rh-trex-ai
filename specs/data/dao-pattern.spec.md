# DAO Pattern Specification

**Date:** 2026-07-06
**Status:** Active
**ID:** DA-001
**Related:** [Service Locator](../framework/service-locator.spec.md), [Entity Lifecycle](../framework/entity-lifecycle.spec.md)
**Implements:** `pkg/db/transaction_middleware.go`, `pkg/db/context.go`, `pkg/server/grpc_interceptors.go`, `pkg/dao/generic.go`, `plugins/*/dao.go`

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

#### Scenario: Request rollback undoes all DAO writes
- GIVEN a request that made several DAO writes through `(*sessionFactory).New(ctx)`
- WHEN `db.MarkForRollback(ctx, err)` is called and the request resolves
- THEN none of those writes SHALL be persisted

#### Scenario: HTTP error response rolls back the request
- GIVEN an HTTP request that made DAO writes inside the request transaction
- WHEN the handler responds with a 4xx or 5xx status
- THEN the transaction middleware SHALL mark the transaction for rollback
- AND none of those writes SHALL be persisted

#### Scenario: gRPC error rolls back the request
- GIVEN a unary gRPC call that made DAO writes inside the request transaction
- WHEN the handler returns a non-nil error
- THEN the transaction interceptor SHALL mark the transaction for rollback
- AND none of those writes SHALL be persisted

#### Scenario: Panic rolls back the request
- GIVEN an HTTP request or unary gRPC call that made DAO writes inside the request transaction
- WHEN the handler panics
- THEN the transaction SHALL be rolled back, not committed
- AND the panic SHALL continue to propagate to outer recovery handlers

#### Scenario: Advisory lock inside a request transaction
- GIVEN a request context containing a transaction
- WHEN a service takes an advisory lock with that context
- THEN the lock SHALL be taken inside the request transaction
- AND it SHALL stay held until the request transaction commits or rolls back

### Requirement: Resolve Before Responding

The request transaction SHALL be committed or rolled back before any part of
the response reaches the client, so a client is never told "success" for a
change that did not persist.

`TransactionMiddleware` SHALL hold the response (status, headers, and body)
until the handler returns, then resolve the transaction, then send it. When
the commit fails, it SHALL discard the held response and the headers the
handler set for it, and send `500 Internal Server Error` with the standard
error body. When the handler flushes, hijacks the connection, writes through
`io.ReaderFrom`, or writes more than `db.MaxBufferedResponseBytes` (1 MiB),
the middleware SHALL resolve the transaction at that moment, before the first
byte is sent, and stream the rest; statements the handler runs afterwards are
outside the request transaction. Writes after a failed commit SHALL return an
error to the handler.

`TransactionUnaryInterceptor` SHALL resolve the transaction before the unary
response is returned. When the commit fails and the handler returned no
error, it SHALL return `codes.Internal` instead of the handler's response.

`db.ResolveErr(ctx) error` SHALL return the commit or rollback error;
`db.Resolve` SHALL keep its signature. The outcome SHALL be recorded in the
context's `db.TxResult`; the middleware and interceptor SHALL add one when the
context has none.

#### Scenario: Commit fails after a successful handler (REST)
- GIVEN a handler that writes rows, sets `Location`, and responds `201`
- WHEN the commit fails (for example, a deferred constraint is violated)
- THEN the client SHALL receive `500` with the standard error body
- AND the response SHALL NOT carry the handler's `Location` or body
- AND no rows SHALL be persisted

#### Scenario: Commit fails after a successful handler (gRPC)
- GIVEN a unary handler that writes rows and returns a response
- WHEN the commit fails
- THEN the client SHALL receive `codes.Internal` and no response

#### Scenario: Streaming handler
- GIVEN a handler that writes rows, writes a chunk, and calls `Flush`
- WHEN `Flush` is called
- THEN the transaction SHALL be committed before the chunk is sent
- AND the rest of the response SHALL stream unbuffered

#### Scenario: Large response
- GIVEN a handler whose body exceeds `db.MaxBufferedResponseBytes`
- WHEN it writes past that size
- THEN the transaction SHALL be resolved before the first byte is sent

#### Scenario: Error response is unchanged
- GIVEN a handler that writes rows and responds `409` with a body
- WHEN the request completes
- THEN the transaction SHALL be rolled back
- AND the client SHALL receive the `409` and its body unchanged

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
  6. `Count(model, &total) error` gets total count before pagination and returns any query error
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

DAO methods SHALL return raw GORM/database errors, including `Count`. Error translation to `ServiceError` types SHALL occur in the service layer, which SHALL NOT silently swallow a failed query.

#### Scenario: Record not found
- GIVEN a `Get` call for a non-existent ID
- WHEN GORM returns `gorm.ErrRecordNotFound`
- THEN the DAO SHALL propagate this error to the caller
- AND the service layer SHALL translate it to `errors.NotFound`

#### Scenario: Count failure surfaces its root cause
- GIVEN a generic list request whose `Count` query fails (e.g. an unknown search column)
- WHEN the DAO executes the COUNT
- THEN `Count` SHALL return the database error rather than discarding it
- AND the service layer SHALL surface it rather than swallowing it
- AND a later statement SHALL NOT mask the root cause with "current transaction is aborted" (SQLSTATE 25P02)

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| DAO returns raw errors (not ServiceError) | Keeps DAO layer database-focused; service layer handles business error translation |
| Session factory pointer (`*db.SessionFactory`) | Allows late initialization during environment setup |
| Generic DAO for search queries | Avoids duplicating pagination/filtering logic across entity DAOs |
| `Count` preserves WHERE and JOIN clauses | Accurate total count for paginated responses |
| `Count` returns its query error | A failing COUNT aborts the request transaction; surfacing the error exposes the real root cause instead of a downstream SQLSTATE 25P02 |
| Resolve before responding, buffering by default | Committing after the response was sent told clients "success" for changes that never persisted. Buffering keeps the status open until the commit is known; the 1 MiB cap and early commit on flush keep memory bounded and streaming working. |
| Separate `mock_dao.go` per plugin | Co-located with real DAO; easy to discover and maintain |
