package marketplace

import "time"

// OwnerPayoutAccount is a row from owner_payout_accounts.
//
// swagger:model OwnerPayoutAccount
type OwnerPayoutAccount struct {
	// UUID of the payout account.
	// example: 550e8400-e29b-41d4-a716-446655440000
	ID string `db:"id" json:"id"`

	// Owner who owns this account.
	// example: 550e8400-e29b-41d4-a716-446655440001
	OwnerID string `db:"owner_id" json:"owner_id"`

	// Type of account.
	// enum: gcash,maya,bank
	// example: gcash
	AccountType string `db:"account_type" json:"account_type"`

	// Display name or account holder name.
	// example: Juan dela Cruz
	AccountName string `db:"account_name" json:"account_name"`

	// Mobile number or bank account number.
	// example: 09171234567
	AccountNumber string `db:"account_number" json:"account_number"`

	// Bank name. Only present for bank accounts.
	// example: BDO
	BankName *string `db:"bank_name" json:"bank_name,omitempty"`

	// Whether this is the default payout account.
	// example: true
	IsDefault bool `db:"is_default" json:"is_default"`

	// Whether this account has been verified.
	// example: true
	IsVerified bool `db:"is_verified" json:"is_verified"`

	// Timestamp when the account was verified.
	VerifiedAt *time.Time `db:"verified_at" json:"verified_at,omitempty"`

	// Timestamp when the account was created.
	CreatedAt time.Time `db:"created_at" json:"created_at"`

	DeletedAt *time.Time `db:"deleted_at" json:"-"`
}

// OwnerEarning is a row from owner_earnings.
//
// swagger:model OwnerEarning
type OwnerEarning struct {
	// UUID of the earning record.
	// example: 550e8400-e29b-41d4-a716-446655440000
	ID string `db:"id" json:"id"`

	// Booking this earning is for.
	// example: 550e8400-e29b-41d4-a716-446655440001
	BookingID string `db:"booking_id" json:"booking_id"`

	// Owner who earned this.
	// example: 550e8400-e29b-41d4-a716-446655440002
	OwnerID string `db:"owner_id" json:"owner_id"`

	// Full booking amount in centavos. Divide by 100 for display in PHP.
	// example: 35000
	GrossAmountCentavos int `db:"gross_amount_centavos" json:"gross_amount_centavos"`

	// Platform fee deducted in centavos. Divide by 100 for display in PHP.
	// example: 3500
	FeeCentavos int `db:"fee_centavos" json:"fee_centavos"`

	// Net amount the owner receives in centavos. Divide by 100 for display in PHP.
	// example: 31500
	NetAmountCentavos int `db:"net_amount_centavos" json:"net_amount_centavos"`

	// How the fee was calculated.
	// enum: percent,flat
	// example: percent
	FeeType string `db:"fee_type" json:"fee_type"`

	// Which fee configuration level was used.
	// enum: owner_override,category_rate,platform_default
	// example: platform_default
	FeeSource string `db:"fee_source" json:"fee_source"`

	// Current status of the earning.
	// enum: pending,released,disputed,paid_out
	// example: released
	Status string `db:"status" json:"status"`

	// Timestamp when the earning was released from the dispute window.
	ReleasedAt *time.Time `db:"released_at" json:"released_at,omitempty"`

	// UUID of the payout batch this earning is included in.
	PayoutID *string `db:"payout_id" json:"payout_id,omitempty"`

	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// OwnerPayout is a row from owner_payouts.
//
// swagger:model OwnerPayout
type OwnerPayout struct {
	// UUID of the payout batch.
	// example: 550e8400-e29b-41d4-a716-446655440000
	ID string `db:"id" json:"id"`

	// Owner receiving this payout.
	// example: 550e8400-e29b-41d4-a716-446655440001
	OwnerID string `db:"owner_id" json:"owner_id"`

	// Payout account used for this batch.
	// example: 550e8400-e29b-41d4-a716-446655440002
	PayoutAccountID string `db:"payout_account_id" json:"payout_account_id"`

	// Total amount transferred in centavos. Divide by 100 for display in PHP.
	// example: 94500
	TotalAmountCentavos int `db:"total_amount_centavos" json:"total_amount_centavos"`

	// Number of earnings included in this payout.
	// example: 3
	EarningsCount int `db:"earnings_count" json:"earnings_count"`

	// PayMongo payout transfer ID for reconciliation.
	PaymongoPayoutID *string `db:"paymongo_payout_id" json:"paymongo_payout_id,omitempty"`

	// Current payout status.
	// enum: pending,processing,paid,failed
	// example: paid
	Status string `db:"status" json:"status"`

	// When the payout is scheduled to run.
	ScheduledFor time.Time `db:"scheduled_for" json:"scheduled_for"`

	// When the PayMongo transfer was initiated.
	InitiatedAt *time.Time `db:"initiated_at" json:"initiated_at,omitempty"`

	// When the payout completed.
	CompletedAt *time.Time `db:"completed_at" json:"completed_at,omitempty"`

	// Reason for failure if status is 'failed'.
	FailureReason *string `db:"failure_reason" json:"failure_reason,omitempty"`

	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// BookingDispute is a row from booking_disputes.
//
// swagger:model BookingDispute
type BookingDispute struct {
	// UUID of the dispute.
	// example: 550e8400-e29b-41d4-a716-446655440000
	ID string `db:"id" json:"id"`

	// Booking being disputed.
	// example: 550e8400-e29b-41d4-a716-446655440001
	BookingID string `db:"booking_id" json:"booking_id"`

	// Customer who raised the dispute.
	// example: 550e8400-e29b-41d4-a716-446655440002
	CustomerID string `db:"customer_id" json:"customer_id"`

	// Reason for the dispute.
	// enum: service_not_rendered,quality_issue,wrong_service,unauthorized_charge,other
	// example: service_not_rendered
	Reason string `db:"reason" json:"reason"`

	// Detailed description of the dispute (max 1000 chars).
	Details *string `db:"details" json:"details,omitempty"`

	// Current status of the dispute.
	// enum: open,under_review,resolved_release,resolved_refund
	// example: open
	Status string `db:"status" json:"status"`

	// Admin notes added during resolution.
	AdminNotes *string `db:"admin_notes" json:"admin_notes,omitempty"`

	CreatedAt  time.Time  `db:"created_at" json:"created_at"`
	ResolvedAt *time.Time `db:"resolved_at" json:"resolved_at,omitempty"`
}

// PlatformSettings is the single row from platform_settings.
//
// swagger:model PlatformSettings
type PlatformSettings struct {
	// UUID of the settings row.
	ID string `db:"id" json:"id"`

	// Default platform fee in basis points. 1000 = 10%.
	// example: 1000
	DefaultFeePercentBP int `db:"default_fee_percent_bp" json:"default_fee_percent_bp"`

	// Dispute window in hours after booking completion.
	// example: 48
	DisputeWindowHours int `db:"dispute_window_hours" json:"dispute_window_hours"`

	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// CategoryFeeRate is a row from category_fee_rates.
//
// swagger:model CategoryFeeRate
type CategoryFeeRate struct {
	ID string `db:"id" json:"id"`
	// Category this rate applies to.
	CategoryID string `db:"category_id" json:"category_id"`
	// Fee in basis points. 1000 = 10%.
	// example: 800
	FeePercentBP int        `db:"fee_percent_bp" json:"fee_percent_bp"`
	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt    *time.Time `db:"updated_at" json:"updated_at,omitempty"`
}

// OwnerFeeOverride is a row from owner_fee_overrides.
//
// swagger:model OwnerFeeOverride
type OwnerFeeOverride struct {
	ID      string `db:"id" json:"id"`
	OwnerID string `db:"owner_id" json:"owner_id"`
	// Fee type — percent (basis points) or flat (centavos).
	// enum: percent,flat
	// example: percent
	FeeType string `db:"fee_type" json:"fee_type"`
	// Fee value. For percent: basis points (500 = 5%). For flat: centavos.
	// example: 500
	FeeValue  int        `db:"fee_value" json:"fee_value"`
	Notes     *string    `db:"notes" json:"notes,omitempty"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt *time.Time `db:"updated_at" json:"updated_at,omitempty"`
}

// --- Request DTOs ---

// CreatePayoutAccountRequest is the request body for POST /owner/payout-accounts.
//
// swagger:model CreatePayoutAccountRequest
type CreatePayoutAccountRequest struct {
	// required: true
	// enum: gcash,maya,bank
	// example: gcash
	AccountType string `json:"account_type" binding:"required,oneof=gcash maya bank"`
	// required: true
	// example: Juan dela Cruz
	AccountName string `json:"account_name" binding:"required"`
	// required: true
	// example: 09171234567
	AccountNumber string `json:"account_number" binding:"required"`
	// Required when account_type is bank.
	// example: BDO
	BankName *string `json:"bank_name"`
}

// SetDefaultAccountRequest is the request body for PUT /owner/payout-accounts/:id.
//
// swagger:model SetDefaultAccountRequest
type SetDefaultAccountRequest struct {
	// required: true
	// example: true
	IsDefault bool `json:"is_default" binding:"required"`
}

// VerifyOTPRequest is the request body for POST /owner/payout-accounts/:id/verify-otp.
//
// swagger:model PayoutVerifyOTPRequest
type VerifyOTPRequest struct {
	// 6-digit OTP sent to the account mobile number.
	// required: true
	// example: 123456
	OTP string `json:"otp" binding:"required"`
}

// UpdatePayoutScheduleRequest is the request body for PUT /owner/payout-schedule.
//
// swagger:model UpdatePayoutScheduleRequest
type UpdatePayoutScheduleRequest struct {
	// required: true
	// enum: daily,weekly
	// example: weekly
	Schedule string `json:"schedule" binding:"required,oneof=daily weekly"`
}

// RaiseDisputeRequest is the request body for POST /bookings/:id/dispute.
//
// swagger:model RaiseDisputeRequest
type RaiseDisputeRequest struct {
	// required: true
	// enum: service_not_rendered,quality_issue,wrong_service,unauthorized_charge,other
	// example: service_not_rendered
	Reason string `json:"reason" binding:"required"`
	// Optional dispute details (max 1000 chars).
	// example: The service was not available when I arrived.
	Details *string `json:"details"`
}

// ResolveDisputeRequest is the request body for POST /admin/disputes/:id/resolve.
//
// swagger:model ResolveDisputeRequest
type ResolveDisputeRequest struct {
	// required: true
	// enum: release,refund
	// example: release
	Resolution string `json:"resolution" binding:"required,oneof=release refund"`
	// Admin notes explaining the resolution.
	// example: Reviewed evidence; service was rendered.
	AdminNotes *string `json:"admin_notes"`
}

// UpsertFeeOverrideRequest is the request body for POST/PUT /admin/fee-overrides.
//
// swagger:model UpsertFeeOverrideRequest
type UpsertFeeOverrideRequest struct {
	// required: true
	// example: 550e8400-e29b-41d4-a716-446655440001
	OwnerID string `json:"owner_id" binding:"required"`
	// required: true
	// enum: percent,flat
	// example: percent
	FeeType string `json:"fee_type" binding:"required,oneof=percent flat"`
	// Fee value. For percent: basis points. For flat: centavos.
	// required: true
	// example: 500
	FeeValue int     `json:"fee_value" binding:"required,min=0"`
	Notes    *string `json:"notes"`
}

// UpdateCategoryFeeRateRequest is the request body for PUT /admin/category-fee-rates/:category_id.
//
// swagger:model UpdateCategoryFeeRateRequest
type UpdateCategoryFeeRateRequest struct {
	// Fee in basis points. 1000 = 10%.
	// required: true
	// minimum: 0
	// maximum: 10000
	// example: 800
	FeePercentBP int `json:"fee_percent_bp" binding:"required,min=0,max=10000"`
}

// UpdatePlatformSettingsRequest is the request body for PUT /admin/platform-settings.
//
// swagger:model UpdatePlatformSettingsRequest
type UpdatePlatformSettingsRequest struct {
	// Default platform fee in basis points. 1000 = 10%.
	// minimum: 0
	// maximum: 10000
	// example: 1000
	DefaultFeePercentBP *int `json:"default_fee_percent_bp"`
	// Dispute window duration in hours.
	// minimum: 1
	// example: 48
	DisputeWindowHours *int `json:"dispute_window_hours"`
}

// --- swagger:response wrappers ---

// swagger:response payoutAccountResponse
type payoutAccountResponse struct {
	// in: body
	Body OwnerPayoutAccount
}

// swagger:response payoutAccountListResponse
type payoutAccountListResponse struct {
	// in: body
	Body []OwnerPayoutAccount
}

// swagger:response earningListResponse
type earningListResponse struct {
	// in: body
	Body []OwnerEarning
}

// swagger:response payoutListResponse
type payoutListResponse struct {
	// in: body
	Body []OwnerPayout
}

// swagger:response disputeResponse
type disputeResponse struct {
	// in: body
	Body BookingDispute
}

// swagger:response disputeListResponse
type disputeListResponse struct {
	// in: body
	Body []BookingDispute
}

// swagger:response platformSettingsResponse
type platformSettingsResponse struct {
	// in: body
	Body PlatformSettings
}

// swagger:response categoryFeeRateListResponse
type categoryFeeRateListResponse struct {
	// in: body
	Body []CategoryFeeRate
}

// swagger:response feeOverrideResponse
type feeOverrideResponse struct {
	// in: body
	Body OwnerFeeOverride
}
