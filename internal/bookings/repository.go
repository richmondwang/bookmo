package bookings

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles all SQL queries for the bookings module.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository constructs a Repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Create inserts a new booking and populates b.ID and b.CreatedAt via RETURNING.
func (r *Repository) Create(ctx context.Context, b *Booking) error {
	const q = `
		INSERT INTO bookings (
			service_id, branch_id, customer_id,
			start_time, end_time, quantity, status,
			payment_method, owner_response_deadline,
			rescheduled_from_booking_id, reschedule_attempt_count,
			rejected_reason, cancelled_by, currency
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12, $13, $14
		) RETURNING id, created_at`

	row := r.db.QueryRow(ctx, q,
		b.ServiceID, b.BranchID, b.CustomerID,
		b.StartTime, b.EndTime, b.Quantity, b.Status,
		b.PaymentMethod, b.OwnerResponseDeadline,
		b.RescheduledFromBookingID, b.RescheduleAttemptCount,
		b.RejectedReason, b.CancelledBy, b.Currency,
	)
	return row.Scan(&b.ID, &b.CreatedAt)
}

// GetByID fetches a booking by ID, respecting soft deletes.
func (r *Repository) GetByID(ctx context.Context, id string) (*Booking, error) {
	const q = `
		SELECT id, service_id, branch_id, customer_id,
		       start_time, end_time, quantity, status,
		       payment_method, owner_response_deadline,
		       rescheduled_from_booking_id, reschedule_attempt_count,
		       rejected_reason, cancelled_by, currency, deleted_at, created_at
		FROM bookings
		WHERE id = $1 AND deleted_at IS NULL`

	b := &Booking{}
	row := r.db.QueryRow(ctx, q, id)
	err := row.Scan(
		&b.ID, &b.ServiceID, &b.BranchID, &b.CustomerID,
		&b.StartTime, &b.EndTime, &b.Quantity, &b.Status,
		&b.PaymentMethod, &b.OwnerResponseDeadline,
		&b.RescheduledFromBookingID, &b.RescheduleAttemptCount,
		&b.RejectedReason, &b.CancelledBy, &b.Currency, &b.DeletedAt, &b.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrBookingNotFound
		}
		return nil, fmt.Errorf("bookings.GetByID: %w", err)
	}
	return b, nil
}

// UpdateStatus updates a booking's status and optional extra fields.
// Supported extra keys: "rejected_reason" (string), "cancelled_by" (string),
// "reschedule_attempt_count" (int), "owner_response_deadline" (*time.Time).
func (r *Repository) UpdateStatus(ctx context.Context, id, status string, extra map[string]any) error {
	// Build SET clause dynamically based on extra fields.
	args := []any{id, status}
	set := "status = $2"
	i := 3

	for k, v := range extra {
		switch k {
		case "rejected_reason", "cancelled_by", "reschedule_attempt_count", "owner_response_deadline":
			set += fmt.Sprintf(", %s = $%d", k, i)
			args = append(args, v)
			i++
		}
	}

	q := fmt.Sprintf("UPDATE bookings SET %s WHERE id = $1 AND deleted_at IS NULL", set)
	ct, err := r.db.Exec(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("bookings.UpdateStatus: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrBookingNotFound
	}
	return nil
}

// CreateLock inserts a booking lock and populates lock.ID.
func (r *Repository) CreateLock(ctx context.Context, lock *BookingLock) error {
	const q = `
		INSERT INTO booking_locks (service_id, branch_id, start_time, end_time, quantity, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at`

	row := r.db.QueryRow(ctx, q,
		lock.ServiceID, lock.BranchID,
		lock.StartTime, lock.EndTime,
		lock.Quantity, lock.ExpiresAt,
	)
	return row.Scan(&lock.ID, &lock.CreatedAt)
}

// GetLock fetches a booking lock by ID if it has not expired.
func (r *Repository) GetLock(ctx context.Context, lockID string) (*BookingLock, error) {
	const q = `
		SELECT id, service_id, branch_id, start_time, end_time, quantity, expires_at, created_at
		FROM booking_locks
		WHERE id = $1 AND expires_at > now()`

	l := &BookingLock{}
	err := r.db.QueryRow(ctx, q, lockID).Scan(
		&l.ID, &l.ServiceID, &l.BranchID,
		&l.StartTime, &l.EndTime, &l.Quantity,
		&l.ExpiresAt, &l.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrLockNotFound
		}
		return nil, fmt.Errorf("bookings.GetLock: %w", err)
	}
	return l, nil
}

// DeleteLock deletes a booking lock by ID.
func (r *Repository) DeleteLock(ctx context.Context, lockID string) error {
	_, err := r.db.Exec(ctx, "DELETE FROM booking_locks WHERE id = $1", lockID)
	if err != nil {
		return fmt.Errorf("bookings.DeleteLock: %w", err)
	}
	return nil
}

// CreateRescheduleRequest inserts a reschedule request and populates req.ID.
func (r *Repository) CreateRescheduleRequest(ctx context.Context, req *RescheduleRequest) error {
	const q = `
		INSERT INTO reschedule_requests (booking_id, requested_by, new_start_time, new_end_time, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`

	row := r.db.QueryRow(ctx, q,
		req.BookingID, req.RequestedBy,
		req.NewStartTime, req.NewEndTime, req.Status,
	)
	return row.Scan(&req.ID, &req.CreatedAt)
}

// GetPendingRescheduleForBooking fetches the pending reschedule request for a booking.
func (r *Repository) GetPendingRescheduleForBooking(ctx context.Context, bookingID string) (*RescheduleRequest, error) {
	const q = `
		SELECT id, booking_id, requested_by, new_start_time, new_end_time, status, created_at
		FROM reschedule_requests
		WHERE booking_id = $1 AND status = 'pending'`

	rr := &RescheduleRequest{}
	err := r.db.QueryRow(ctx, q, bookingID).Scan(
		&rr.ID, &rr.BookingID, &rr.RequestedBy,
		&rr.NewStartTime, &rr.NewEndTime, &rr.Status, &rr.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRescheduleNotFound
		}
		return nil, fmt.Errorf("bookings.GetPendingRescheduleForBooking: %w", err)
	}
	return rr, nil
}

// GetRescheduleByID fetches a reschedule request by ID.
func (r *Repository) GetRescheduleByID(ctx context.Context, id string) (*RescheduleRequest, error) {
	const q = `
		SELECT id, booking_id, requested_by, new_start_time, new_end_time, status, created_at
		FROM reschedule_requests
		WHERE id = $1`

	rr := &RescheduleRequest{}
	err := r.db.QueryRow(ctx, q, id).Scan(
		&rr.ID, &rr.BookingID, &rr.RequestedBy,
		&rr.NewStartTime, &rr.NewEndTime, &rr.Status, &rr.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRescheduleNotFound
		}
		return nil, fmt.Errorf("bookings.GetRescheduleByID: %w", err)
	}
	return rr, nil
}

// UpdateRescheduleStatus updates the status of a reschedule request.
func (r *Repository) UpdateRescheduleStatus(ctx context.Context, rescheduleID, status string) error {
	const q = `UPDATE reschedule_requests SET status = $2 WHERE id = $1`
	ct, err := r.db.Exec(ctx, q, rescheduleID, status)
	if err != nil {
		return fmt.Errorf("bookings.UpdateRescheduleStatus: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrRescheduleNotFound
	}
	return nil
}

// ApproveReschedule runs the approval transaction per ADR-002:
//  1. Lock reschedule request row FOR UPDATE
//  2. Re-check slot availability for the new time
//  3. Set reschedule_requests.status='approved'
//  4. Set original booking status='rescheduled'
//  5. Insert new booking with status='confirmed'
func (r *Repository) ApproveReschedule(ctx context.Context, rescheduleID string, newBooking *Booking) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("bookings.ApproveReschedule: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// 1. Lock reschedule request row and load it.
	var rr RescheduleRequest
	err = tx.QueryRow(ctx,
		`SELECT id, booking_id, new_start_time, new_end_time, status FROM reschedule_requests WHERE id = $1 FOR UPDATE`,
		rescheduleID,
	).Scan(&rr.ID, &rr.BookingID, &rr.NewStartTime, &rr.NewEndTime, &rr.Status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRescheduleNotFound
		}
		return fmt.Errorf("bookings.ApproveReschedule: lock reschedule: %w", err)
	}
	if rr.Status != "pending" {
		return ErrIllegalStateTransition
	}

	// 2. Load original booking.
	var orig Booking
	err = tx.QueryRow(ctx,
		`SELECT id, service_id, branch_id, quantity FROM bookings WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`,
		rr.BookingID,
	).Scan(&orig.ID, &orig.ServiceID, &orig.BranchID, &orig.Quantity)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrBookingNotFound
		}
		return fmt.Errorf("bookings.ApproveReschedule: load original: %w", err)
	}

	// 3. Re-check slot availability for the new time.
	if err := checkSlotAvailabilityTx(ctx, tx, orig.ServiceID, rr.NewStartTime, rr.NewEndTime, orig.Quantity, rr.BookingID); err != nil {
		return err
	}

	// 4. Mark reschedule as approved.
	if _, err := tx.Exec(ctx,
		`UPDATE reschedule_requests SET status = 'approved' WHERE id = $1`,
		rescheduleID,
	); err != nil {
		return fmt.Errorf("bookings.ApproveReschedule: approve reschedule: %w", err)
	}

	// 5. Set original booking to 'rescheduled'.
	if _, err := tx.Exec(ctx,
		`UPDATE bookings SET status = 'rescheduled' WHERE id = $1`,
		rr.BookingID,
	); err != nil {
		return fmt.Errorf("bookings.ApproveReschedule: set rescheduled: %w", err)
	}

	// 6. Insert new booking as confirmed.
	newBooking.ServiceID = orig.ServiceID
	newBooking.BranchID = orig.BranchID
	newBooking.StartTime = rr.NewStartTime
	newBooking.EndTime = rr.NewEndTime
	newBooking.Quantity = orig.Quantity
	newBooking.Status = "confirmed"
	origID := orig.ID
	newBooking.RescheduledFromBookingID = &origID

	err = tx.QueryRow(ctx, `
		INSERT INTO bookings (
			service_id, branch_id, customer_id,
			start_time, end_time, quantity, status,
			payment_method, rescheduled_from_booking_id,
			reschedule_attempt_count, currency
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at`,
		newBooking.ServiceID, newBooking.BranchID, newBooking.CustomerID,
		newBooking.StartTime, newBooking.EndTime, newBooking.Quantity, newBooking.Status,
		newBooking.PaymentMethod, newBooking.RescheduledFromBookingID,
		0, newBooking.Currency,
	).Scan(&newBooking.ID, &newBooking.CreatedAt)
	if err != nil {
		return fmt.Errorf("bookings.ApproveReschedule: insert new booking: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("bookings.ApproveReschedule: commit: %w", err)
	}
	return nil
}

// GetOwnerQueue returns all awaiting_approval bookings for an owner's branches,
// joined with service, branch, customer, and trust data.
func (r *Repository) GetOwnerQueue(ctx context.Context, ownerID string) ([]QueueItem, error) {
	const q = `
		SELECT
			b.id, b.service_id, b.branch_id, b.customer_id,
			b.start_time, b.end_time, b.quantity, b.status,
			b.payment_method, b.owner_response_deadline,
			b.rescheduled_from_booking_id, b.reschedule_attempt_count,
			b.rejected_reason, b.cancelled_by, b.currency, b.deleted_at, b.created_at,
			s.name AS service_name,
			br.name AS branch_name,
			u.full_name AS customer_name,
			ctp.total_bookings,
			ctp.completed_bookings,
			ctp.completion_rate,
			ctp.avg_owner_rating,
			ctp.total_owner_reviews
		FROM bookings b
		JOIN services s ON s.id = b.service_id AND s.deleted_at IS NULL
		JOIN branches br ON br.id = b.branch_id AND br.deleted_at IS NULL
		JOIN users u ON u.id = b.customer_id
		LEFT JOIN customer_trust_profiles ctp ON ctp.customer_id = b.customer_id
		WHERE b.status = 'awaiting_approval'
		  AND b.deleted_at IS NULL
		  AND br.owner_id = (SELECT id FROM owners WHERE user_id = $1 LIMIT 1)
		ORDER BY b.created_at ASC`

	rows, err := r.db.Query(ctx, q, ownerID)
	if err != nil {
		return nil, fmt.Errorf("bookings.GetOwnerQueue: %w", err)
	}
	defer rows.Close()

	var items []QueueItem
	for rows.Next() {
		var item QueueItem
		var (
			totalBookings     *int
			completedBookings *int
			completionRate    *float64
			avgOwnerRating    *float64
			totalOwnerReviews *int
		)
		err := rows.Scan(
			&item.ID, &item.ServiceID, &item.BranchID, &item.CustomerID,
			&item.StartTime, &item.EndTime, &item.Quantity, &item.Status,
			&item.PaymentMethod, &item.OwnerResponseDeadline,
			&item.RescheduledFromBookingID, &item.RescheduleAttemptCount,
			&item.RejectedReason, &item.CancelledBy, &item.Currency, &item.DeletedAt, &item.CreatedAt,
			&item.ServiceName,
			&item.BranchName,
			&item.CustomerName,
			&totalBookings, &completedBookings, &completionRate,
			&avgOwnerRating, &totalOwnerReviews,
		)
		if err != nil {
			return nil, fmt.Errorf("bookings.GetOwnerQueue: scan: %w", err)
		}
		if totalBookings != nil {
			item.CustomerTrust = &CustomerTrustData{
				TotalBookings:     *totalBookings,
				CompletedBookings: *completedBookings,
				CompletionRate:    *completionRate,
				AvgOwnerRating:    *avgOwnerRating,
				TotalOwnerReviews: *totalOwnerReviews,
			}
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// GetBookingsByCustomer returns all bookings for a customer, newest first.
func (r *Repository) GetBookingsByCustomer(ctx context.Context, customerID string) ([]Booking, error) {
	const q = `
		SELECT id, service_id, branch_id, customer_id,
		       start_time, end_time, quantity, status,
		       payment_method, owner_response_deadline,
		       rescheduled_from_booking_id, reschedule_attempt_count,
		       rejected_reason, cancelled_by, currency, deleted_at, created_at
		FROM bookings
		WHERE customer_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC`

	rows, err := r.db.Query(ctx, q, customerID)
	if err != nil {
		return nil, fmt.Errorf("bookings.GetBookingsByCustomer: %w", err)
	}
	defer rows.Close()

	var bs []Booking
	for rows.Next() {
		var b Booking
		err := rows.Scan(
			&b.ID, &b.ServiceID, &b.BranchID, &b.CustomerID,
			&b.StartTime, &b.EndTime, &b.Quantity, &b.Status,
			&b.PaymentMethod, &b.OwnerResponseDeadline,
			&b.RescheduledFromBookingID, &b.RescheduleAttemptCount,
			&b.RejectedReason, &b.CancelledBy, &b.Currency, &b.DeletedAt, &b.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("bookings.GetBookingsByCustomer: scan: %w", err)
		}
		bs = append(bs, b)
	}
	return bs, rows.Err()
}

// OwnsBookingBranch checks that the given ownerID owns the branch associated with the booking.
func (r *Repository) OwnsBookingBranch(ctx context.Context, bookingID, ownerID string) (bool, error) {
	const q = `
		SELECT COUNT(*) > 0
		FROM bookings b
		JOIN branches br ON br.id = b.branch_id
		JOIN owners o ON o.id = br.owner_id
		WHERE b.id = $1 AND o.user_id = $2 AND b.deleted_at IS NULL`

	var owns bool
	err := r.db.QueryRow(ctx, q, bookingID, ownerID).Scan(&owns)
	if err != nil {
		return false, fmt.Errorf("bookings.OwnsBookingBranch: %w", err)
	}
	return owns, nil
}

// OwnsRescheduleBranch checks that the given ownerID owns the branch for the reschedule's booking.
func (r *Repository) OwnsRescheduleBranch(ctx context.Context, rescheduleID, ownerID string) (bool, error) {
	const q = `
		SELECT COUNT(*) > 0
		FROM reschedule_requests rr
		JOIN bookings b ON b.id = rr.booking_id
		JOIN branches br ON br.id = b.branch_id
		JOIN owners o ON o.id = br.owner_id
		WHERE rr.id = $1 AND o.user_id = $2 AND b.deleted_at IS NULL`

	var owns bool
	err := r.db.QueryRow(ctx, q, rescheduleID, ownerID).Scan(&owns)
	if err != nil {
		return false, fmt.Errorf("bookings.OwnsRescheduleBranch: %w", err)
	}
	return owns, nil
}

// CheckSlotConflict returns true if the slot is unavailable (i.e., capacity would be exceeded).
func (r *Repository) CheckSlotConflict(ctx context.Context, serviceID string, start, end time.Time, quantity int) (bool, error) {
	capacity, err := r.getServiceCapacity(ctx, serviceID)
	if err != nil {
		return false, err
	}

	var bookingQty int
	err = r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(quantity), 0)
		FROM bookings
		WHERE service_id = $1
		  AND status IN ('pending', 'awaiting_approval', 'confirmed')
		  AND start_time < $3
		  AND end_time > $2
		  AND deleted_at IS NULL`,
		serviceID, start, end,
	).Scan(&bookingQty)
	if err != nil {
		return false, fmt.Errorf("bookings.CheckSlotConflict: bookings query: %w", err)
	}

	var lockQty int
	err = r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(quantity), 0)
		FROM booking_locks
		WHERE service_id = $1
		  AND start_time < $3
		  AND end_time > $2
		  AND expires_at > now()`,
		serviceID, start, end,
	).Scan(&lockQty)
	if err != nil {
		return false, fmt.Errorf("bookings.CheckSlotConflict: locks query: %w", err)
	}

	return bookingQty+lockQty+quantity > capacity, nil
}

// getServiceCapacity returns the capacity for a service.
func (r *Repository) getServiceCapacity(ctx context.Context, serviceID string) (int, error) {
	var capacity int
	err := r.db.QueryRow(ctx,
		`SELECT capacity FROM services WHERE id = $1 AND deleted_at IS NULL`,
		serviceID,
	).Scan(&capacity)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, fmt.Errorf("bookings.getServiceCapacity: service not found")
		}
		return 0, fmt.Errorf("bookings.getServiceCapacity: %w", err)
	}
	return capacity, nil
}

// checkSlotAvailabilityTx performs slot overlap check inside a transaction,
// excluding the given bookingIDToExclude from the overlap count (used during reschedule).
func checkSlotAvailabilityTx(ctx context.Context, tx pgx.Tx, serviceID string, start, end time.Time, quantity int, excludeBookingID string) error {
	// Get service capacity.
	var capacity int
	err := tx.QueryRow(ctx,
		`SELECT capacity FROM services WHERE id = $1 AND deleted_at IS NULL`,
		serviceID,
	).Scan(&capacity)
	if err != nil {
		return fmt.Errorf("bookings.checkSlotAvailabilityTx: capacity: %w", err)
	}

	var bookingQty int
	err = tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(quantity), 0)
		FROM bookings
		WHERE service_id = $1
		  AND id != $4
		  AND status IN ('pending', 'awaiting_approval', 'confirmed')
		  AND start_time < $3
		  AND end_time > $2
		  AND deleted_at IS NULL`,
		serviceID, start, end, excludeBookingID,
	).Scan(&bookingQty)
	if err != nil {
		return fmt.Errorf("bookings.checkSlotAvailabilityTx: bookings: %w", err)
	}

	var lockQty int
	err = tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(quantity), 0)
		FROM booking_locks
		WHERE service_id = $1
		  AND start_time < $3
		  AND end_time > $2
		  AND expires_at > now()`,
		serviceID, start, end,
	).Scan(&lockQty)
	if err != nil {
		return fmt.Errorf("bookings.checkSlotAvailabilityTx: locks: %w", err)
	}

	if bookingQty+lockQty+quantity > capacity {
		return ErrSlotUnavailable
	}
	return nil
}
