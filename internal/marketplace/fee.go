package marketplace

import (
	"context"
	"fmt"
)

// FeeResult holds the resolved fee in centavos and its source.
type FeeResult struct {
	FeeCentavos int
	FeeType     string // "percent" or "flat"
	FeeSource   string // "owner_override", "category_rate", or "platform_default"
}

// ResolveFee computes the platform fee in centavos for a booking.
// Priority: owner_fee_overrides → category_fee_rates → platform_settings.
// All arithmetic uses integers (basis points) to avoid floating-point errors.
func ResolveFee(ctx context.Context, repo *Repository, ownerID, categoryID string, grossCentavos int) (*FeeResult, error) {
	// 1. Owner-specific override.
	override, err := repo.GetOwnerFeeOverride(ctx, ownerID)
	if err == nil {
		var fee int
		if override.FeeType == "flat" {
			fee = override.FeeValue
		} else {
			// percent: fee_value is basis points
			fee = grossCentavos * override.FeeValue / 10000
		}
		return &FeeResult{
			FeeCentavos: fee,
			FeeType:     override.FeeType,
			FeeSource:   "owner_override",
		}, nil
	}
	if err != ErrNotFound {
		return nil, fmt.Errorf("marketplace.ResolveFee owner override: %w", err)
	}

	// 2. Category default rate.
	if categoryID != "" {
		catRate, err := repo.GetCategoryFeeRate(ctx, categoryID)
		if err == nil {
			fee := grossCentavos * catRate.FeePercentBP / 10000
			return &FeeResult{
				FeeCentavos: fee,
				FeeType:     "percent",
				FeeSource:   "category_rate",
			}, nil
		}
		if err != ErrNotFound {
			return nil, fmt.Errorf("marketplace.ResolveFee category rate: %w", err)
		}
	}

	// 3. Platform-wide default.
	settings, err := repo.GetPlatformSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("marketplace.ResolveFee platform settings: %w", err)
	}
	fee := grossCentavos * settings.DefaultFeePercentBP / 10000
	return &FeeResult{
		FeeCentavos: fee,
		FeeType:     "percent",
		FeeSource:   "platform_default",
	}, nil
}
