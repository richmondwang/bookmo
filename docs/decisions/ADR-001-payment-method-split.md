# ADR-001: Payment method split — immediate capture vs authorize/capture

## Status
Accepted

## Context

The platform uses PayMongo as its payment gateway for the Philippines market. Because owners manually approve bookings within a 24-hour window, there is a gap between when the customer pays and when the booking is confirmed. This creates a problem: how do you hold a customer's money without fully charging them until the owner approves?

Cards (Visa, Mastercard) support a two-step payment flow:
1. **Authorize** — the bank reserves the funds on the customer's card but does not transfer them
2. **Capture** — the funds are actually moved to the merchant

GCash and Maya (the dominant e-wallets in the Philippines) do **not** support authorization holds. When a GCash or Maya payment is initiated, the money leaves the customer's wallet immediately and permanently — there is no "hold" state.

## Decision

Split payment handling by method type:

**Cards (`method_type = 'auth_capture'`)**
- On booking: create a PayMongo payment intent and authorize (hold) the funds
- On owner approval: capture the funds
- On rejection / timeout / cancellation: void the hold — no charge, no refund needed

**GCash / Maya (`method_type = 'immediate_capture'`)**
- On booking: capture immediately — funds leave the customer's wallet
- On owner approval: booking confirmed, funds already held by PayMongo
- On rejection / timeout / cancellation: issue a full refund via PayMongo's refund API

## Consequences

- The `payment_intents` table stores `method_type` so the payment service always knows which path to take — never infer from `method` alone at runtime
- Refund processing is async — GCash/Maya refunds take 3–7 banking days; the `refunds` table tracks the refund lifecycle separately
- The customer-facing UI must communicate the refund timeline clearly for e-wallet rejections — "your refund will appear in 3–7 banking days" must be shown at rejection time, not buried in terms
- The reconciliation job must handle both paths: for cards, check authorized vs captured; for e-wallets, check captured vs refunded
- Never attempt to void a GCash/Maya payment — it will fail. Always use the refund endpoint.
- Never attempt to refund a card authorization that hasn't been captured — void it instead.

## How to determine method_type in code

```go
func methodType(method string) string {
    switch method {
    case "gcash", "maya", "bank_transfer":
        return "immediate_capture"
    default: // card
        return "auth_capture"
    }
}
```
