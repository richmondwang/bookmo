package availability

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles all SQL queries for the availability module.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository constructs a Repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// GetDateOverride returns the date override for a branch on a specific date, or
// ErrNotFound if none exists.
func (r *Repository) GetDateOverride(ctx context.Context, branchID string, date time.Time) (*DateOverride, error) {
	const q = `
		SELECT id, branch_id, date, is_closed, open_time, close_time, COALESCE(note, ''), created_at
		FROM date_overrides
		WHERE branch_id = $1
		  AND date = $2::date
	`
	row := r.db.QueryRow(ctx, q, branchID, date)

	var d DateOverride
	err := row.Scan(
		&d.ID,
		&d.BranchID,
		&d.Date,
		&d.IsClosed,
		&d.OpenTime,
		&d.CloseTime,
		&d.Note,
		&d.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("availability.GetDateOverride: %w", err)
	}
	return &d, nil
}

// GetAvailabilityRule returns the active availability rule for a branch on a
// given day of week (0 = Sunday, 6 = Saturday), or ErrNotFound if none exists.
func (r *Repository) GetAvailabilityRule(ctx context.Context, branchID string, dayOfWeek int) (*AvailabilityRule, error) {
	const q = `
		SELECT id, branch_id, day_of_week, start_time, end_time, is_active, created_at
		FROM availability_rules
		WHERE branch_id = $1
		  AND day_of_week = $2
		  AND is_active = true
		LIMIT 1
	`
	row := r.db.QueryRow(ctx, q, branchID, dayOfWeek)

	var a AvailabilityRule
	err := row.Scan(
		&a.ID,
		&a.BranchID,
		&a.DayOfWeek,
		&a.StartTime,
		&a.EndTime,
		&a.IsActive,
		&a.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("availability.GetAvailabilityRule: %w", err)
	}
	return &a, nil
}

// GetActiveBookingsInRange returns active bookings for a service that overlap
// the given time range. Active statuses are pending, awaiting_approval, and
// confirmed.
func (r *Repository) GetActiveBookingsInRange(ctx context.Context, serviceID string, start, end time.Time) ([]BookingSlot, error) {
	const q = `
		SELECT start_time, end_time, quantity
		FROM bookings
		WHERE service_id = $1
		  AND status IN ('pending', 'awaiting_approval', 'confirmed')
		  AND start_time < $3
		  AND end_time   > $2
		  AND deleted_at IS NULL
	`
	rows, err := r.db.Query(ctx, q, serviceID, start, end)
	if err != nil {
		return nil, fmt.Errorf("availability.GetActiveBookingsInRange: %w", err)
	}
	defer rows.Close()

	var slots []BookingSlot
	for rows.Next() {
		var s BookingSlot
		if err := rows.Scan(&s.StartTime, &s.EndTime, &s.Quantity); err != nil {
			return nil, fmt.Errorf("availability.GetActiveBookingsInRange scan: %w", err)
		}
		slots = append(slots, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("availability.GetActiveBookingsInRange rows: %w", err)
	}
	return slots, nil
}

// GetActiveLocksInRange returns unexpired booking locks for a service that
// overlap the given time range.
func (r *Repository) GetActiveLocksInRange(ctx context.Context, serviceID string, start, end time.Time) ([]LockSlot, error) {
	const q = `
		SELECT start_time, end_time, quantity
		FROM booking_locks
		WHERE service_id = $1
		  AND expires_at > now()
		  AND start_time < $3
		  AND end_time   > $2
	`
	rows, err := r.db.Query(ctx, q, serviceID, start, end)
	if err != nil {
		return nil, fmt.Errorf("availability.GetActiveLocksInRange: %w", err)
	}
	defer rows.Close()

	var slots []LockSlot
	for rows.Next() {
		var s LockSlot
		if err := rows.Scan(&s.StartTime, &s.EndTime, &s.Quantity); err != nil {
			return nil, fmt.Errorf("availability.GetActiveLocksInRange scan: %w", err)
		}
		slots = append(slots, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("availability.GetActiveLocksInRange rows: %w", err)
	}
	return slots, nil
}

// GetServiceCapacity returns capacity information for a service.
func (r *Repository) GetServiceCapacity(ctx context.Context, serviceID string) (capacity int, capacityType string, stepMinutes int, minDuration int, maxDuration int, err error) {
	const q = `
		SELECT capacity, capacity_type, step_minutes, min_duration, max_duration
		FROM services
		WHERE id = $1
		  AND deleted_at IS NULL
	`
	row := r.db.QueryRow(ctx, q, serviceID)
	err = row.Scan(&capacity, &capacityType, &stepMinutes, &minDuration, &maxDuration)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = ErrNotFound
			return
		}
		err = fmt.Errorf("availability.GetServiceCapacity: %w", err)
		return
	}
	return
}
