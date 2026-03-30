package availability

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Service implements business logic for the availability module.
type Service struct {
	repo *Repository
}

// NewService constructs a Service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// GetSlotsForDate returns available slots for a service on a given date,
// following ADR-005 resolution priority:
//  1. date_overrides — if is_closed=true return no slots; use override hours if set
//  2. availability_rules — use the rule matching day of week if active
//  3. No slots (branch closed for that date)
//
// Each returned slot has its remaining capacity computed from active bookings
// and locks.
func (s *Service) GetSlotsForDate(ctx context.Context, branchID, serviceID string, date time.Time) ([]Slot, error) {
	capacity, capacityType, stepMinutes, minDuration, _, err := s.repo.GetServiceCapacity(ctx, serviceID)
	if err != nil {
		return nil, fmt.Errorf("availability.GetSlotsForDate: %w", err)
	}

	// ADR-005: check date_overrides first
	override, err := s.repo.GetDateOverride(ctx, branchID, date)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, fmt.Errorf("availability.GetSlotsForDate: %w", err)
	}

	var openTime, closeTime time.Time

	if override != nil {
		if override.IsClosed {
			return nil, nil // branch explicitly closed this day
		}
		if override.OpenTime != nil && override.CloseTime != nil {
			openTime = *override.OpenTime
			closeTime = *override.CloseTime
		} else {
			// override exists but has no hours — treat as closed
			return nil, nil
		}
	} else {
		// Fall back to weekly availability rule
		rule, err := s.repo.GetAvailabilityRule(ctx, branchID, int(date.Weekday()))
		if err != nil {
			// ErrNotFound or any other error — no slots
			return nil, nil
		}
		if !rule.IsActive {
			return nil, nil
		}
		openTime = rule.StartTime
		closeTime = rule.EndTime
	}

	rawSlots := generateSlots(date, openTime, closeTime, stepMinutes, minDuration)
	if len(rawSlots) == 0 {
		return nil, nil
	}

	// Compute remaining capacity for each slot
	dayStart := rawSlots[0].StartTime
	dayEnd := rawSlots[len(rawSlots)-1].EndTime

	bookings, err := s.repo.GetActiveBookingsInRange(ctx, serviceID, dayStart, dayEnd)
	if err != nil {
		return nil, fmt.Errorf("availability.GetSlotsForDate: %w", err)
	}
	locks, err := s.repo.GetActiveLocksInRange(ctx, serviceID, dayStart, dayEnd)
	if err != nil {
		return nil, fmt.Errorf("availability.GetSlotsForDate: %w", err)
	}

	result := make([]Slot, 0, len(rawSlots))
	for _, slot := range rawSlots {
		used := occupiedQuantity(slot.StartTime, slot.EndTime, bookings, locks)
		remaining := capacity - used

		if capacityType == "single" {
			if used > 0 {
				remaining = 0
			} else {
				remaining = 1
			}
		}

		if remaining > 0 {
			slot.RemainingCapacity = remaining
			result = append(result, slot)
		}
	}

	return result, nil
}

// CheckConflict returns true if adding quantity more bookings for serviceID in
// [start, end) would exceed the service's capacity. For single-capacity
// services, any overlap is a conflict. For multi-capacity services, the sum of
// existing quantities plus the new quantity must not exceed capacity.
func (s *Service) CheckConflict(ctx context.Context, serviceID string, start, end time.Time, quantity int) (bool, error) {
	capacity, capacityType, _, _, _, err := s.repo.GetServiceCapacity(ctx, serviceID)
	if err != nil {
		return false, fmt.Errorf("availability.CheckConflict: %w", err)
	}

	bookings, err := s.repo.GetActiveBookingsInRange(ctx, serviceID, start, end)
	if err != nil {
		return false, fmt.Errorf("availability.CheckConflict: %w", err)
	}
	locks, err := s.repo.GetActiveLocksInRange(ctx, serviceID, start, end)
	if err != nil {
		return false, fmt.Errorf("availability.CheckConflict: %w", err)
	}

	used := occupiedQuantity(start, end, bookings, locks)

	if capacityType == "single" {
		return used > 0, nil
	}

	// multi capacity
	return used+quantity > capacity, nil
}

// generateSlots generates slot windows for the given date using the branch's
// open/close times (which carry only the time-of-day component from the DB).
// Step is stepMinutes; each slot spans serviceDuration minutes.
func generateSlots(date, openTime, closeTime time.Time, stepMinutes, serviceDuration int) []Slot {
	// Build wall-clock open/close by combining the date with the time components.
	loc := date.Location()
	y, m, d := date.Date()

	open := time.Date(y, m, d,
		openTime.Hour(), openTime.Minute(), openTime.Second(), 0, loc)
	close := time.Date(y, m, d,
		closeTime.Hour(), closeTime.Minute(), closeTime.Second(), 0, loc)

	if !close.After(open) {
		return nil
	}

	step := time.Duration(stepMinutes) * time.Minute
	duration := time.Duration(serviceDuration) * time.Minute

	var slots []Slot
	for slotStart := open; ; slotStart = slotStart.Add(step) {
		slotEnd := slotStart.Add(duration)
		if slotEnd.After(close) {
			break
		}
		slots = append(slots, Slot{
			StartTime: slotStart,
			EndTime:   slotEnd,
		})
	}
	return slots
}

// occupiedQuantity returns the total quantity already held (by bookings + locks)
// that overlaps the window [start, end).
func occupiedQuantity(start, end time.Time, bookings []BookingSlot, locks []LockSlot) int {
	used := 0
	for _, b := range bookings {
		if b.StartTime.Before(end) && b.EndTime.After(start) {
			used += b.Quantity
		}
	}
	for _, l := range locks {
		if l.StartTime.Before(end) && l.EndTime.After(start) {
			used += l.Quantity
		}
	}
	return used
}
