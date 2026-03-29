# Write tests for a module

Write comprehensive tests for the `$ARGUMENTS` module.

## What to create

### Unit tests — `internal/$ARGUMENTS/service_test.go`

- Table-driven tests for every public method on `Service`
- Mock the `Repository` interface — define the mock in `internal/$ARGUMENTS/mock_repository_test.go`
- Cover all happy paths and all error paths
- For the booking module: test every state machine transition, including illegal transitions that should return errors
- For the payment module: test both card (auth/capture) and e-wallet (immediate capture + refund) paths separately
- For the availability module: test the resolution priority (date_overrides > availability_rules > closed)

### Integration tests — `internal/$ARGUMENTS/integration_test.go`

- Build tag: `//go:build integration` on the first line
- Use `testcontainers-go` to spin up PostgreSQL with PostGIS
- Run all migrations before tests
- Test the repository layer directly against a real DB
- Clean up test data after each test using `t.Cleanup`

## Conventions

- Test function names: `TestServiceName_MethodName_Scenario`
- Use `errors.Is` to assert sentinel errors, not string matching
- Never use `time.Sleep` — use channels or context cancellation for async
- Run with: `go test -race ./internal/$ARGUMENTS/...`
- Integration tests: `go test -tags integration -race ./internal/$ARGUMENTS/...`

## After writing tests

Run them and fix any failures:
```
go test -race ./internal/$ARGUMENTS/...
```