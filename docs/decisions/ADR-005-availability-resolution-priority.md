# ADR-005: Availability resolution priority — overrides before rules

## Status
Accepted

## Context

Branch availability is defined by two data sources:

1. **`availability_rules`** — recurring weekly schedule (e.g. "open Monday–Saturday 9am–6pm")
2. **`date_overrides`** — one-off exceptions (e.g. "closed December 25", "open until 8pm this Saturday only")

When generating slots for a given date, the availability engine must decide which source wins when both exist for the same date.

## Decision

`date_overrides` always takes priority over `availability_rules`. If a `date_override` row exists for a branch + date, it is used exclusively — the weekly rule for that day of week is ignored entirely.

Resolution order (highest to lowest):
1. `date_overrides` — if `is_closed = true`, return no slots; if hours are set, use those hours
2. `availability_rules` — use the rule matching the day of week, if active
3. No slots (branch is closed for that date)

## Reasoning

This is the mental model owners already have. When an owner blocks Christmas Day, they expect it to override their normal Monday schedule without having to also edit the Monday rule. Any other resolution order would require owners to understand the interaction between two systems — a source of support issues.

The override-first model also makes edge cases unambiguous: if an owner sets a `date_override` with `is_closed = false` and specific hours, those hours are used regardless of what the weekly rule says. There is no merging or intersection logic.

## Consequences

- The availability engine must query `date_overrides` first, before touching `availability_rules`
- If a `date_override` row is found, short-circuit — do not load `availability_rules` for that date
- The Go implementation follows this structure:
  ```go
  func (s *Service) GetSlotsForDate(ctx context.Context, branchID uuid.UUID, date time.Time) ([]Slot, error) {
      override, err := s.repo.GetDateOverride(ctx, branchID, date)
      if err != nil && !errors.Is(err, ErrNotFound) {
          return nil, err
      }
      if override != nil {
          if override.IsClosed {
              return nil, nil // no slots
          }
          return s.generateSlots(date, override.OpenTime, override.CloseTime), nil
      }

      rule, err := s.repo.GetAvailabilityRule(ctx, branchID, int(date.Weekday()))
      if err != nil || !rule.IsActive {
          return nil, nil // no slots
      }
      return s.generateSlots(date, rule.StartTime, rule.EndTime), nil
  }
  ```
- The owner-facing `/owner/branches/:id/calendar` endpoint must merge both sources into a single month view so owners see exactly what customers will see — the merged result of overrides + rules, not two separate lists
- Tests must cover: override present + rule present (override wins), override absent + rule present (rule used), both absent (no slots), override with `is_closed = true` (no slots regardless of rule)
