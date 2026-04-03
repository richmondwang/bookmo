package marketplace

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles all database operations for the marketplace module.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a Repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// --- Platform settings ---

// GetPlatformSettings returns the single platform_settings row.
func (r *Repository) GetPlatformSettings(ctx context.Context) (*PlatformSettings, error) {
	const q = `SELECT id, default_fee_percent_bp, dispute_window_hours, updated_at FROM platform_settings LIMIT 1`
	s := &PlatformSettings{}
	err := r.db.QueryRow(ctx, q).Scan(&s.ID, &s.DefaultFeePercentBP, &s.DisputeWindowHours, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("marketplace.GetPlatformSettings: %w", err)
	}
	return s, nil
}

// UpdatePlatformSettings updates the platform_settings row.
func (r *Repository) UpdatePlatformSettings(ctx context.Context, req *UpdatePlatformSettingsRequest) (*PlatformSettings, error) {
	const q = `
		UPDATE platform_settings SET
			default_fee_percent_bp = COALESCE($1, default_fee_percent_bp),
			dispute_window_hours   = COALESCE($2, dispute_window_hours),
			updated_at             = now()
		RETURNING id, default_fee_percent_bp, dispute_window_hours, updated_at`
	s := &PlatformSettings{}
	err := r.db.QueryRow(ctx, q, req.DefaultFeePercentBP, req.DisputeWindowHours).Scan(
		&s.ID, &s.DefaultFeePercentBP, &s.DisputeWindowHours, &s.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("marketplace.UpdatePlatformSettings: %w", err)
	}
	return s, nil
}

// --- Fee overrides ---

// GetOwnerFeeOverride returns the fee override for an owner. Returns ErrNotFound if none.
func (r *Repository) GetOwnerFeeOverride(ctx context.Context, ownerID string) (*OwnerFeeOverride, error) {
	const q = `SELECT id, owner_id, fee_type, fee_value, notes, created_at, updated_at FROM owner_fee_overrides WHERE owner_id = $1`
	o := &OwnerFeeOverride{}
	err := r.db.QueryRow(ctx, q, ownerID).Scan(&o.ID, &o.OwnerID, &o.FeeType, &o.FeeValue, &o.Notes, &o.CreatedAt, &o.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("marketplace.GetOwnerFeeOverride: %w", err)
	}
	return o, nil
}

// UpsertFeeOverride creates or updates a fee override for an owner.
func (r *Repository) UpsertFeeOverride(ctx context.Context, req *UpsertFeeOverrideRequest) (*OwnerFeeOverride, error) {
	const q = `
		INSERT INTO owner_fee_overrides (owner_id, fee_type, fee_value, notes)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (owner_id) DO UPDATE SET
			fee_type   = EXCLUDED.fee_type,
			fee_value  = EXCLUDED.fee_value,
			notes      = EXCLUDED.notes,
			updated_at = now()
		RETURNING id, owner_id, fee_type, fee_value, notes, created_at, updated_at`
	o := &OwnerFeeOverride{}
	err := r.db.QueryRow(ctx, q, req.OwnerID, req.FeeType, req.FeeValue, req.Notes).Scan(
		&o.ID, &o.OwnerID, &o.FeeType, &o.FeeValue, &o.Notes, &o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("marketplace.UpsertFeeOverride: %w", err)
	}
	return o, nil
}

// --- Category fee rates ---

// GetCategoryFeeRate returns the fee rate for a category. Returns ErrNotFound if none.
func (r *Repository) GetCategoryFeeRate(ctx context.Context, categoryID string) (*CategoryFeeRate, error) {
	const q = `SELECT id, category_id, fee_percent_bp, created_at, updated_at FROM category_fee_rates WHERE category_id = $1`
	c := &CategoryFeeRate{}
	err := r.db.QueryRow(ctx, q, categoryID).Scan(&c.ID, &c.CategoryID, &c.FeePercentBP, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("marketplace.GetCategoryFeeRate: %w", err)
	}
	return c, nil
}

// ListCategoryFeeRates returns all category fee rates.
func (r *Repository) ListCategoryFeeRates(ctx context.Context) ([]CategoryFeeRate, error) {
	const q = `SELECT id, category_id, fee_percent_bp, created_at, updated_at FROM category_fee_rates ORDER BY created_at`
	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("marketplace.ListCategoryFeeRates: %w", err)
	}
	defer rows.Close()
	var out []CategoryFeeRate
	for rows.Next() {
		var c CategoryFeeRate
		if err := rows.Scan(&c.ID, &c.CategoryID, &c.FeePercentBP, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("marketplace.ListCategoryFeeRates scan: %w", err)
		}
		out = append(out, c)
	}
	return out, nil
}

// UpsertCategoryFeeRate creates or updates a category fee rate.
func (r *Repository) UpsertCategoryFeeRate(ctx context.Context, categoryID string, feeBP int) (*CategoryFeeRate, error) {
	const q = `
		INSERT INTO category_fee_rates (category_id, fee_percent_bp)
		VALUES ($1, $2)
		ON CONFLICT (category_id) DO UPDATE SET
			fee_percent_bp = EXCLUDED.fee_percent_bp,
			updated_at     = now()
		RETURNING id, category_id, fee_percent_bp, created_at, updated_at`
	c := &CategoryFeeRate{}
	err := r.db.QueryRow(ctx, q, categoryID, feeBP).Scan(&c.ID, &c.CategoryID, &c.FeePercentBP, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("marketplace.UpsertCategoryFeeRate: %w", err)
	}
	return c, nil
}

// --- Payout accounts ---

// ListPayoutAccounts returns all active payout accounts for an owner.
func (r *Repository) ListPayoutAccounts(ctx context.Context, ownerID string) ([]OwnerPayoutAccount, error) {
	const q = `
		SELECT id, owner_id, account_type, account_name, account_number, bank_name,
		       is_default, is_verified, verified_at, created_at, deleted_at
		FROM owner_payout_accounts
		WHERE owner_id = $1 AND deleted_at IS NULL
		ORDER BY is_default DESC, created_at`
	rows, err := r.db.Query(ctx, q, ownerID)
	if err != nil {
		return nil, fmt.Errorf("marketplace.ListPayoutAccounts: %w", err)
	}
	defer rows.Close()
	var out []OwnerPayoutAccount
	for rows.Next() {
		var a OwnerPayoutAccount
		if err := rows.Scan(&a.ID, &a.OwnerID, &a.AccountType, &a.AccountName, &a.AccountNumber,
			&a.BankName, &a.IsDefault, &a.IsVerified, &a.VerifiedAt, &a.CreatedAt, &a.DeletedAt); err != nil {
			return nil, fmt.Errorf("marketplace.ListPayoutAccounts scan: %w", err)
		}
		out = append(out, a)
	}
	return out, nil
}

// GetPayoutAccount returns a single payout account by ID.
func (r *Repository) GetPayoutAccount(ctx context.Context, id string) (*OwnerPayoutAccount, error) {
	const q = `
		SELECT id, owner_id, account_type, account_name, account_number, bank_name,
		       is_default, is_verified, verified_at, created_at, deleted_at
		FROM owner_payout_accounts
		WHERE id = $1 AND deleted_at IS NULL`
	a := &OwnerPayoutAccount{}
	err := r.db.QueryRow(ctx, q, id).Scan(&a.ID, &a.OwnerID, &a.AccountType, &a.AccountName, &a.AccountNumber,
		&a.BankName, &a.IsDefault, &a.IsVerified, &a.VerifiedAt, &a.CreatedAt, &a.DeletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("marketplace.GetPayoutAccount: %w", err)
	}
	return a, nil
}

// CreatePayoutAccount inserts a new payout account.
func (r *Repository) CreatePayoutAccount(ctx context.Context, ownerID string, req *CreatePayoutAccountRequest) (*OwnerPayoutAccount, error) {
	const q = `
		INSERT INTO owner_payout_accounts (owner_id, account_type, account_name, account_number, bank_name)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, owner_id, account_type, account_name, account_number, bank_name,
		          is_default, is_verified, verified_at, created_at, deleted_at`
	a := &OwnerPayoutAccount{}
	err := r.db.QueryRow(ctx, q, ownerID, req.AccountType, req.AccountName, req.AccountNumber, req.BankName).Scan(
		&a.ID, &a.OwnerID, &a.AccountType, &a.AccountName, &a.AccountNumber,
		&a.BankName, &a.IsDefault, &a.IsVerified, &a.VerifiedAt, &a.CreatedAt, &a.DeletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("marketplace.CreatePayoutAccount: %w", err)
	}
	return a, nil
}

// SetDefaultPayoutAccount clears any existing default for the owner, then sets the new one.
// Both operations run in a single transaction.
func (r *Repository) SetDefaultPayoutAccount(ctx context.Context, ownerID, accountID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("marketplace.SetDefaultPayoutAccount begin: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx,
		`UPDATE owner_payout_accounts SET is_default = false WHERE owner_id = $1 AND deleted_at IS NULL`,
		ownerID,
	)
	if err != nil {
		return fmt.Errorf("marketplace.SetDefaultPayoutAccount clear: %w", err)
	}
	_, err = tx.Exec(ctx,
		`UPDATE owner_payout_accounts SET is_default = true WHERE id = $1 AND owner_id = $2 AND deleted_at IS NULL`,
		accountID, ownerID,
	)
	if err != nil {
		return fmt.Errorf("marketplace.SetDefaultPayoutAccount set: %w", err)
	}
	return tx.Commit(ctx)
}

// VerifyPayoutAccount marks an account as verified.
func (r *Repository) VerifyPayoutAccount(ctx context.Context, id string) error {
	now := time.Now().UTC()
	_, err := r.db.Exec(ctx,
		`UPDATE owner_payout_accounts SET is_verified = true, verified_at = $1 WHERE id = $2 AND deleted_at IS NULL`,
		now, id,
	)
	if err != nil {
		return fmt.Errorf("marketplace.VerifyPayoutAccount: %w", err)
	}
	return nil
}

// SoftDeletePayoutAccount soft-deletes a payout account.
func (r *Repository) SoftDeletePayoutAccount(ctx context.Context, id, ownerID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE owner_payout_accounts SET deleted_at = now() WHERE id = $1 AND owner_id = $2 AND deleted_at IS NULL`,
		id, ownerID,
	)
	if err != nil {
		return fmt.Errorf("marketplace.SoftDeletePayoutAccount: %w", err)
	}
	return nil
}

// GetDefaultVerifiedPayoutAccount returns the owner's verified default payout account.
func (r *Repository) GetDefaultVerifiedPayoutAccount(ctx context.Context, ownerID string) (*OwnerPayoutAccount, error) {
	const q = `
		SELECT id, owner_id, account_type, account_name, account_number, bank_name,
		       is_default, is_verified, verified_at, created_at, deleted_at
		FROM owner_payout_accounts
		WHERE owner_id = $1 AND is_default = true AND is_verified = true AND deleted_at IS NULL
		LIMIT 1`
	a := &OwnerPayoutAccount{}
	err := r.db.QueryRow(ctx, q, ownerID).Scan(&a.ID, &a.OwnerID, &a.AccountType, &a.AccountName, &a.AccountNumber,
		&a.BankName, &a.IsDefault, &a.IsVerified, &a.VerifiedAt, &a.CreatedAt, &a.DeletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoVerifiedPayoutAccount
	}
	if err != nil {
		return nil, fmt.Errorf("marketplace.GetDefaultVerifiedPayoutAccount: %w", err)
	}
	return a, nil
}

// ListUnverifiedPayoutAccounts returns all unverified payout accounts (for admin).
func (r *Repository) ListUnverifiedPayoutAccounts(ctx context.Context) ([]OwnerPayoutAccount, error) {
	const q = `
		SELECT id, owner_id, account_type, account_name, account_number, bank_name,
		       is_default, is_verified, verified_at, created_at, deleted_at
		FROM owner_payout_accounts
		WHERE is_verified = false AND deleted_at IS NULL
		ORDER BY created_at`
	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("marketplace.ListUnverifiedPayoutAccounts: %w", err)
	}
	defer rows.Close()
	var out []OwnerPayoutAccount
	for rows.Next() {
		var a OwnerPayoutAccount
		if err := rows.Scan(&a.ID, &a.OwnerID, &a.AccountType, &a.AccountName, &a.AccountNumber,
			&a.BankName, &a.IsDefault, &a.IsVerified, &a.VerifiedAt, &a.CreatedAt, &a.DeletedAt); err != nil {
			return nil, fmt.Errorf("marketplace.ListUnverifiedPayoutAccounts scan: %w", err)
		}
		out = append(out, a)
	}
	return out, nil
}

// --- Earnings ---

// CreateEarning inserts an owner_earnings row.
func (r *Repository) CreateEarning(ctx context.Context, e *OwnerEarning) error {
	const q = `
		INSERT INTO owner_earnings (booking_id, owner_id, gross_amount_centavos, fee_centavos,
		                            net_amount_centavos, fee_type, fee_source, status, released_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`
	_, err := r.db.Exec(ctx, q,
		e.BookingID, e.OwnerID, e.GrossAmountCentavos, e.FeeCentavos,
		e.NetAmountCentavos, e.FeeType, e.FeeSource, e.Status, e.ReleasedAt,
	)
	if err != nil {
		return fmt.Errorf("marketplace.CreateEarning: %w", err)
	}
	return nil
}

// ListEarnings returns earnings for an owner, optionally filtered by status.
func (r *Repository) ListEarnings(ctx context.Context, ownerID, status string) ([]OwnerEarning, error) {
	q := `
		SELECT id, booking_id, owner_id, gross_amount_centavos, fee_centavos, net_amount_centavos,
		       fee_type, fee_source, status, released_at, payout_id, created_at
		FROM owner_earnings
		WHERE owner_id = $1`
	args := []any{ownerID}
	if status != "" {
		q += " AND status = $2"
		args = append(args, status)
	}
	q += " ORDER BY created_at DESC"

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("marketplace.ListEarnings: %w", err)
	}
	defer rows.Close()
	var out []OwnerEarning
	for rows.Next() {
		var e OwnerEarning
		if err := rows.Scan(&e.ID, &e.BookingID, &e.OwnerID, &e.GrossAmountCentavos, &e.FeeCentavos,
			&e.NetAmountCentavos, &e.FeeType, &e.FeeSource, &e.Status, &e.ReleasedAt,
			&e.PayoutID, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("marketplace.ListEarnings scan: %w", err)
		}
		out = append(out, e)
	}
	return out, nil
}

// GetReleasedEarningsForOwner returns all 'released' earnings for an owner (for payout batching).
func (r *Repository) GetReleasedEarningsForOwner(ctx context.Context, ownerID string) ([]OwnerEarning, error) {
	const q = `
		SELECT id, booking_id, owner_id, gross_amount_centavos, fee_centavos, net_amount_centavos,
		       fee_type, fee_source, status, released_at, payout_id, created_at
		FROM owner_earnings
		WHERE owner_id = $1 AND status = 'released'
		ORDER BY created_at`
	rows, err := r.db.Query(ctx, q, ownerID)
	if err != nil {
		return nil, fmt.Errorf("marketplace.GetReleasedEarningsForOwner: %w", err)
	}
	defer rows.Close()
	var out []OwnerEarning
	for rows.Next() {
		var e OwnerEarning
		if err := rows.Scan(&e.ID, &e.BookingID, &e.OwnerID, &e.GrossAmountCentavos, &e.FeeCentavos,
			&e.NetAmountCentavos, &e.FeeType, &e.FeeSource, &e.Status, &e.ReleasedAt,
			&e.PayoutID, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("marketplace.GetReleasedEarningsForOwner scan: %w", err)
		}
		out = append(out, e)
	}
	return out, nil
}

// MarkEarningsPaidOut links earnings to a payout and marks them 'paid_out' in a single transaction.
func (r *Repository) MarkEarningsPaidOut(ctx context.Context, earningIDs []string, payoutID string) error {
	if len(earningIDs) == 0 {
		return nil
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("marketplace.MarkEarningsPaidOut begin: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, eid := range earningIDs {
		_, err := tx.Exec(ctx,
			`UPDATE owner_earnings SET status = 'paid_out', payout_id = $1 WHERE id = $2`,
			payoutID, eid,
		)
		if err != nil {
			return fmt.Errorf("marketplace.MarkEarningsPaidOut update %s: %w", eid, err)
		}
	}
	return tx.Commit(ctx)
}

// SetEarningDisputed marks an earning as 'disputed'.
func (r *Repository) SetEarningDisputed(ctx context.Context, bookingID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE owner_earnings SET status = 'disputed' WHERE booking_id = $1`,
		bookingID,
	)
	if err != nil {
		return fmt.Errorf("marketplace.SetEarningDisputed: %w", err)
	}
	return nil
}

// ReleaseEarning marks an earning as 'released'.
func (r *Repository) ReleaseEarning(ctx context.Context, earningID string) error {
	now := time.Now().UTC()
	_, err := r.db.Exec(ctx,
		`UPDATE owner_earnings SET status = 'released', released_at = $1 WHERE id = $2`,
		now, earningID,
	)
	if err != nil {
		return fmt.Errorf("marketplace.ReleaseEarning: %w", err)
	}
	return nil
}

// DeleteEarning removes an earning (used when admin resolves a dispute with a refund).
func (r *Repository) DeleteEarning(ctx context.Context, bookingID string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM owner_earnings WHERE booking_id = $1`, bookingID)
	if err != nil {
		return fmt.Errorf("marketplace.DeleteEarning: %w", err)
	}
	return nil
}

// --- Disputes ---

// CreateDispute inserts a booking_disputes row.
func (r *Repository) CreateDispute(ctx context.Context, d *BookingDispute) error {
	const q = `
		INSERT INTO booking_disputes (booking_id, customer_id, reason, details)
		VALUES ($1, $2, $3, $4)
		RETURNING id, booking_id, customer_id, reason, details, status, admin_notes, created_at, resolved_at`
	err := r.db.QueryRow(ctx, q, d.BookingID, d.CustomerID, d.Reason, d.Details).Scan(
		&d.ID, &d.BookingID, &d.CustomerID, &d.Reason, &d.Details,
		&d.Status, &d.AdminNotes, &d.CreatedAt, &d.ResolvedAt,
	)
	if err != nil {
		return fmt.Errorf("marketplace.CreateDispute: %w", err)
	}
	return nil
}

// GetDisputeByBooking returns the dispute for a booking. Returns ErrNotFound if none.
func (r *Repository) GetDisputeByBooking(ctx context.Context, bookingID string) (*BookingDispute, error) {
	const q = `
		SELECT id, booking_id, customer_id, reason, details, status, admin_notes, created_at, resolved_at
		FROM booking_disputes WHERE booking_id = $1`
	d := &BookingDispute{}
	err := r.db.QueryRow(ctx, q, bookingID).Scan(
		&d.ID, &d.BookingID, &d.CustomerID, &d.Reason, &d.Details,
		&d.Status, &d.AdminNotes, &d.CreatedAt, &d.ResolvedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("marketplace.GetDisputeByBooking: %w", err)
	}
	return d, nil
}

// GetDisputeByID returns a dispute by its primary key.
func (r *Repository) GetDisputeByID(ctx context.Context, id string) (*BookingDispute, error) {
	const q = `
		SELECT id, booking_id, customer_id, reason, details, status, admin_notes, created_at, resolved_at
		FROM booking_disputes WHERE id = $1`
	d := &BookingDispute{}
	err := r.db.QueryRow(ctx, q, id).Scan(
		&d.ID, &d.BookingID, &d.CustomerID, &d.Reason, &d.Details,
		&d.Status, &d.AdminNotes, &d.CreatedAt, &d.ResolvedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("marketplace.GetDisputeByID: %w", err)
	}
	return d, nil
}

// ListDisputes returns all disputes, optionally filtered by status.
func (r *Repository) ListDisputes(ctx context.Context, status string) ([]BookingDispute, error) {
	q := `SELECT id, booking_id, customer_id, reason, details, status, admin_notes, created_at, resolved_at FROM booking_disputes`
	var args []any
	if status != "" {
		q += " WHERE status = $1"
		args = append(args, status)
	}
	q += " ORDER BY created_at DESC"
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("marketplace.ListDisputes: %w", err)
	}
	defer rows.Close()
	var out []BookingDispute
	for rows.Next() {
		var d BookingDispute
		if err := rows.Scan(&d.ID, &d.BookingID, &d.CustomerID, &d.Reason, &d.Details,
			&d.Status, &d.AdminNotes, &d.CreatedAt, &d.ResolvedAt); err != nil {
			return nil, fmt.Errorf("marketplace.ListDisputes scan: %w", err)
		}
		out = append(out, d)
	}
	return out, nil
}

// ResolveDispute sets a dispute's status to the resolved value and records admin notes.
func (r *Repository) ResolveDispute(ctx context.Context, id, status string, adminNotes *string) error {
	now := time.Now().UTC()
	_, err := r.db.Exec(ctx,
		`UPDATE booking_disputes SET status = $1, admin_notes = $2, resolved_at = $3 WHERE id = $4`,
		status, adminNotes, now, id,
	)
	if err != nil {
		return fmt.Errorf("marketplace.ResolveDispute: %w", err)
	}
	return nil
}

// --- Payouts ---

// CreatePayout inserts an owner_payouts row.
func (r *Repository) CreatePayout(ctx context.Context, p *OwnerPayout) error {
	const q = `
		INSERT INTO owner_payouts (owner_id, payout_account_id, total_amount_centavos, earnings_count, status, scheduled_for)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, owner_id, payout_account_id, total_amount_centavos, earnings_count,
		          paymongo_payout_id, status, scheduled_for, initiated_at, completed_at, failure_reason, created_at`
	err := r.db.QueryRow(ctx, q, p.OwnerID, p.PayoutAccountID, p.TotalAmountCentavos, p.EarningsCount, p.Status, p.ScheduledFor).Scan(
		&p.ID, &p.OwnerID, &p.PayoutAccountID, &p.TotalAmountCentavos, &p.EarningsCount,
		&p.PaymongoPayoutID, &p.Status, &p.ScheduledFor, &p.InitiatedAt, &p.CompletedAt, &p.FailureReason, &p.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("marketplace.CreatePayout: %w", err)
	}
	return nil
}

// UpdatePayoutStatus updates a payout's status and associated timestamps.
func (r *Repository) UpdatePayoutStatus(ctx context.Context, id, status string, paymongoID *string, initiatedAt, completedAt *time.Time, failureReason *string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE owner_payouts SET
			status             = $1,
			paymongo_payout_id = COALESCE($2, paymongo_payout_id),
			initiated_at       = COALESCE($3, initiated_at),
			completed_at       = COALESCE($4, completed_at),
			failure_reason     = COALESCE($5, failure_reason)
		WHERE id = $6`,
		status, paymongoID, initiatedAt, completedAt, failureReason, id,
	)
	if err != nil {
		return fmt.Errorf("marketplace.UpdatePayoutStatus: %w", err)
	}
	return nil
}

// ListPayouts returns payout history for an owner.
func (r *Repository) ListPayouts(ctx context.Context, ownerID string) ([]OwnerPayout, error) {
	const q = `
		SELECT id, owner_id, payout_account_id, total_amount_centavos, earnings_count,
		       paymongo_payout_id, status, scheduled_for, initiated_at, completed_at, failure_reason, created_at
		FROM owner_payouts WHERE owner_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, q, ownerID)
	if err != nil {
		return nil, fmt.Errorf("marketplace.ListPayouts: %w", err)
	}
	defer rows.Close()
	var out []OwnerPayout
	for rows.Next() {
		var p OwnerPayout
		if err := rows.Scan(&p.ID, &p.OwnerID, &p.PayoutAccountID, &p.TotalAmountCentavos, &p.EarningsCount,
			&p.PaymongoPayoutID, &p.Status, &p.ScheduledFor, &p.InitiatedAt, &p.CompletedAt, &p.FailureReason, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("marketplace.ListPayouts scan: %w", err)
		}
		out = append(out, p)
	}
	return out, nil
}

// --- Scheduler queries ---

// FindEarningsToRelease returns bookings eligible to have earnings released:
// completed, dispute window expired, no existing earning, no open/under_review dispute.
func (r *Repository) FindEarningsToRelease(ctx context.Context) ([]bookingEarningCandidate, error) {
	const q = `
		SELECT
			b.id                AS booking_id,
			b.customer_id,
			o.id                AS owner_id,
			s.category_id,
			pi.amount_centavos  AS gross_amount_centavos
		FROM bookings b
		JOIN services s    ON s.id = b.service_id
		JOIN branches br   ON br.id = b.branch_id
		JOIN owners o      ON o.id = br.owner_id
		JOIN payment_intents pi ON pi.booking_id = b.id
		JOIN platform_settings ps ON true
		WHERE b.status = 'completed'
		  AND b.completed_at IS NOT NULL
		  AND b.completed_at + (ps.dispute_window_hours * interval '1 hour') < now()
		  AND b.deleted_at IS NULL
		  AND NOT EXISTS (SELECT 1 FROM owner_earnings  oe WHERE oe.booking_id = b.id)
		  AND NOT EXISTS (SELECT 1 FROM booking_disputes bd
		                  WHERE bd.booking_id = b.id AND bd.status IN ('open','under_review'))`
	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("marketplace.FindEarningsToRelease: %w", err)
	}
	defer rows.Close()
	var out []bookingEarningCandidate
	for rows.Next() {
		var c bookingEarningCandidate
		if err := rows.Scan(&c.BookingID, &c.CustomerID, &c.OwnerID, &c.CategoryID, &c.GrossAmountCentavos); err != nil {
			return nil, fmt.Errorf("marketplace.FindEarningsToRelease scan: %w", err)
		}
		out = append(out, c)
	}
	return out, nil
}

// bookingEarningCandidate is an intermediate struct for scheduler queries.
type bookingEarningCandidate struct {
	BookingID           string
	CustomerID          string
	OwnerID             string
	CategoryID          string
	GrossAmountCentavos int
}

// FindOwnersWithReleasedEarnings returns distinct owner IDs that have 'released' earnings.
func (r *Repository) FindOwnersWithReleasedEarnings(ctx context.Context) ([]string, error) {
	const q = `SELECT DISTINCT owner_id FROM owner_earnings WHERE status = 'released'`
	rows, err := r.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("marketplace.FindOwnersWithReleasedEarnings: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("marketplace.FindOwnersWithReleasedEarnings scan: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// GetOwnerPayoutSchedule returns the payout_schedule value for an owner.
func (r *Repository) GetOwnerPayoutSchedule(ctx context.Context, ownerID string) (string, error) {
	var schedule string
	err := r.db.QueryRow(ctx, `SELECT payout_schedule FROM owners WHERE id = $1`, ownerID).Scan(&schedule)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("marketplace.GetOwnerPayoutSchedule: %w", err)
	}
	return schedule, nil
}

// UpdateOwnerPayoutSchedule sets the payout_schedule for an owner.
func (r *Repository) UpdateOwnerPayoutSchedule(ctx context.Context, ownerID, schedule string) error {
	_, err := r.db.Exec(ctx, `UPDATE owners SET payout_schedule = $1 WHERE id = $2`, schedule, ownerID)
	if err != nil {
		return fmt.Errorf("marketplace.UpdateOwnerPayoutSchedule: %w", err)
	}
	return nil
}

// GetBookingForDispute returns the fields needed to validate a dispute.
type bookingDisputeData struct {
	CustomerID  string
	Status      string
	CompletedAt *time.Time
}

// GetBookingForDispute fetches booking data needed for dispute validation.
func (r *Repository) GetBookingForDispute(ctx context.Context, bookingID string) (*bookingDisputeData, error) {
	const q = `SELECT customer_id, status, completed_at FROM bookings WHERE id = $1 AND deleted_at IS NULL`
	d := &bookingDisputeData{}
	err := r.db.QueryRow(ctx, q, bookingID).Scan(&d.CustomerID, &d.Status, &d.CompletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("marketplace.GetBookingForDispute: %w", err)
	}
	return d, nil
}
