# ADR-011: API documentation with go-swagger annotations

## Status
Accepted

## Context

The OpenAPI spec for Kadto — a Booking Platform must stay in sync with the actual Go implementation. Maintaining a separate `docs/openapi.yaml` by hand diverges from the code over time. go-swagger generates the spec directly from structured comments in Go source files, making the spec a byproduct of writing code rather than a separate maintenance burden.

## Decision

Use go-swagger (`github.com/go-swagger/go-swagger`) to generate `docs/openapi.yaml` from annotations in Go source files. The spec is never edited by hand — it is always regenerated.

## Annotation locations

| What | Where | Annotation type |
|---|---|---|
| Global API metadata | `cmd/api/main.go` | `swagger:meta` |
| Request/response structs | `internal/*/model.go` | `swagger:model` |
| Request body structs | `internal/*/model.go` | `swagger:parameters` |
| Route handlers | `internal/*/handler.go` | `swagger:route` |
| Response wrappers | `internal/*/model.go` | `swagger:response` |

## Regeneration command

```bash
# From project root
swagger generate spec -o docs/openapi.yaml --scan-models

# Validate the generated spec
swagger validate docs/openapi.yaml
```

Add both to the Makefile:

```makefile
.PHONY: swagger
swagger:
	swagger generate spec -o docs/openapi.yaml --scan-models
	swagger validate docs/openapi.yaml

.PHONY: swagger-serve
swagger-serve:
	swagger serve docs/openapi.yaml --port 8082
```

## Annotation conventions for this project

### 1. Global meta — `cmd/api/main.go`

```go
// Package main Kadto API
//
// # Kadto — a Booking Platform
//
// A two-sided booking marketplace for the Philippines market.
// Customers discover and book services. Owners manage listings
// and approve bookings.
//
//	Schemes: https, http
//	Host: api.kadto.ph
//	BasePath: /v1
//	Version: 1.0.0
//	License: Proprietary
//
//	Consumes:
//	  - application/json
//
//	Produces:
//	  - application/json
//
//	SecurityDefinitions:
//	  bearer:
//	    type: apiKey
//	    in: header
//	    name: Authorization
//	    description: "JWT token. Format: Bearer {token}"
//
// swagger:meta
package main
```

### 2. Model structs — `internal/*/model.go`

Every struct used as a request body or response must have a `swagger:model` annotation. Use field-level comments for descriptions, `required`, `example`, and `enum` tags.

```go
// Booking represents a customer's reservation of a service slot.
//
// swagger:model Booking
type Booking struct {
    // UUID of the booking
    // required: true
    // example: 550e8400-e29b-41d4-a716-446655440000
    ID uuid.UUID `json:"id"`

    // Current booking status
    // required: true
    // enum: pending,awaiting_approval,confirmed,rejected,cancelled,rescheduled,completed
    Status string `json:"status"`

    // Booking start time in UTC ISO 8601
    // required: true
    // example: 2026-01-13T03:00:00Z
    StartTime time.Time `json:"start_time"`

    // Booking end time in UTC ISO 8601
    // required: true
    // example: 2026-01-13T04:00:00Z
    EndTime time.Time `json:"end_time"`

    // Total amount in centavos. Divide by 100 for display in PHP.
    // required: true
    // minimum: 0
    // example: 35000
    AmountCentavos int `json:"amount_centavos"`

    // ISO 4217 currency code. Always PHP for this market.
    // required: true
    // example: PHP
    Currency string `json:"currency"`

    // Deadline for owner to approve or reject, in UTC.
    // Only present when status is awaiting_approval.
    // example: 2026-01-14T03:00:00Z
    OwnerResponseDeadline *time.Time `json:"owner_response_deadline,omitempty"`
}
```

### 3. Request parameter structs — `internal/*/model.go`

Request bodies use `swagger:parameters` on a wrapper struct. Name the struct `{OperationID}Body`.

```go
// swagger:parameters approveBooking
type approveBookingParams struct {
    // Booking UUID
    // in: path
    // required: true
    // example: 550e8400-e29b-41d4-a716-446655440000
    ID string `json:"id"`
}

// swagger:parameters createBookingLock
type createBookingLockBody struct {
    // in: body
    // required: true
    Body struct {
        // UUID of the service to book
        // required: true
        ServiceID uuid.UUID `json:"service_id"`

        // UUID of the branch
        // required: true
        BranchID uuid.UUID `json:"branch_id"`

        // Desired start time in UTC
        // required: true
        // example: 2026-01-13T03:00:00Z
        StartTime time.Time `json:"start_time"`

        // Desired end time in UTC
        // required: true
        // example: 2026-01-13T04:00:00Z
        EndTime time.Time `json:"end_time"`

        // Number of spots to reserve. Defaults to 1.
        // minimum: 1
        // example: 1
        Quantity int `json:"quantity"`
    }
}
```

### 4. Response wrapper structs — `internal/*/model.go`

Define named response types so handlers can reference them. This gives the generated spec clean, reusable response schemas.

```go
// Standard error response returned on all 4xx and 5xx responses.
//
// swagger:response errorResponse
type errorResponse struct {
    // in: body
    Body struct {
        // Machine-readable error code in snake_case
        // example: slot_unavailable
        Error string `json:"error"`

        // Human-readable description safe to display to the user
        // example: The selected time slot is no longer available
        Message string `json:"message"`
    }
}

// swagger:response bookingResponse
type bookingResponse struct {
    // in: body
    Body Booking
}

// swagger:response bookingListResponse
type bookingListResponse struct {
    // in: body
    Body []Booking
}
```

### 5. Route handlers — `internal/*/handler.go`

The `swagger:route` comment goes directly above the handler function. Format:

```
// swagger:route METHOD /path tag operationID
//
// Summary line (used as the operation title)
//
// Longer description of what this endpoint does,
// including important business rules.
//
// Security:
//   bearer:
//
// Parameters:
//   + name: paramName
//     ...
//
// Responses:
//   200: responseName
//   400: errorResponse
//   401: errorResponse
//   403: errorResponse
//   404: errorResponse
```

Full example:

```go
// swagger:route POST /owner/bookings/{id}/approve owner approveBooking
//
// Approve a booking
//
// Approves a booking in awaiting_approval status. Re-validates slot
// availability inside the DB transaction before confirming. Captures
// payment for card bookings. GCash/Maya bookings were already captured
// at booking creation time.
//
// Security:
//   bearer:
//
// Responses:
//   200: bookingResponse
//   403: errorResponse
//   404: errorResponse
//   409: errorResponse
func (h *Handler) ApproveBooking(c *gin.Context) {
    // ...
}
```

## Tags used in this project

Tags group endpoints in the generated spec. Use exactly these values:

| Tag | Endpoints |
|---|---|
| `auth` | /auth/* |
| `search` | /search, /categories |
| `services` | /services/* |
| `availability` | /availability |
| `bookings` | /bookings/* (customer) |
| `payments` | /payments/* |
| `reviews` | /reviews/*, /customer-reviews/* |
| `profiles` | /users/* |
| `participants` | /bookings/*/participants |
| `notifications` | /notifications/* |
| `owner` | /owner/* |

## Consequences

- `docs/openapi.yaml` is gitignored — it is generated, not committed
- Add `swagger generate spec -o docs/openapi.yaml --scan-models` to CI so the spec is always fresh
- Frontend consumes the generated spec directly — never the hand-maintained version
- Every new handler must include a `swagger:route` comment before it is merged
- Every new model struct used in a request or response must include `swagger:model`
- The `annotate-swagger` slash command adds annotations to existing handlers that are missing them
