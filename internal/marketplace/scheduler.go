package marketplace

import (
	"context"
	"fmt"
	"log"
	"time"
)

// Scheduler runs periodic marketplace jobs: earnings release and payout batching.
type Scheduler struct {
	repo   *Repository
	client *PayoutClient
}

// NewScheduler creates a Scheduler.
func NewScheduler(repo *Repository, client *PayoutClient) *Scheduler {
	return &Scheduler{repo: repo, client: client}
}

// ReleaseEarnings finds all completed bookings whose dispute window has expired
// with no open dispute, and creates owner_earnings rows for them.
// Called by the background worker on a schedule (e.g. every hour).
func (s *Scheduler) ReleaseEarnings(ctx context.Context) error {
	candidates, err := s.repo.FindEarningsToRelease(ctx)
	if err != nil {
		return fmt.Errorf("marketplace.ReleaseEarnings: find candidates: %w", err)
	}

	for _, c := range candidates {
		if err := s.releaseOne(ctx, c); err != nil {
			// Log and continue — one failure should not block the rest.
			log.Printf("marketplace.ReleaseEarnings: booking %s: %v", c.BookingID, err)
		}
	}
	return nil
}

func (s *Scheduler) releaseOne(ctx context.Context, c bookingEarningCandidate) error {
	fee, err := ResolveFee(ctx, s.repo, c.OwnerID, c.CategoryID, c.GrossAmountCentavos)
	if err != nil {
		return fmt.Errorf("resolve fee: %w", err)
	}

	now := time.Now().UTC()
	e := &OwnerEarning{
		BookingID:           c.BookingID,
		OwnerID:             c.OwnerID,
		GrossAmountCentavos: c.GrossAmountCentavos,
		FeeCentavos:         fee.FeeCentavos,
		NetAmountCentavos:   c.GrossAmountCentavos - fee.FeeCentavos,
		FeeType:             fee.FeeType,
		FeeSource:           fee.FeeSource,
		Status:              "released",
		ReleasedAt:          &now,
	}
	if err := s.repo.CreateEarning(ctx, e); err != nil {
		return fmt.Errorf("create earning: %w", err)
	}
	return nil
}

// ProcessPayouts batches all 'released' earnings per owner and initiates PayMongo transfers.
// Only owners with a verified default payout account receive payouts.
// Called by the background worker according to each owner's payout_schedule.
func (s *Scheduler) ProcessPayouts(ctx context.Context) error {
	ownerIDs, err := s.repo.FindOwnersWithReleasedEarnings(ctx)
	if err != nil {
		return fmt.Errorf("marketplace.ProcessPayouts: find owners: %w", err)
	}

	for _, ownerID := range ownerIDs {
		if err := s.processOwnerPayout(ctx, ownerID); err != nil {
			log.Printf("marketplace.ProcessPayouts: owner %s: %v", ownerID, err)
		}
	}
	return nil
}

func (s *Scheduler) processOwnerPayout(ctx context.Context, ownerID string) error {
	account, err := s.repo.GetDefaultVerifiedPayoutAccount(ctx, ownerID)
	if err != nil {
		// No verified default — flag and skip.
		log.Printf("marketplace.processOwnerPayout: owner %s has no verified default account", ownerID)
		return nil
	}

	earnings, err := s.repo.GetReleasedEarningsForOwner(ctx, ownerID)
	if err != nil {
		return fmt.Errorf("get released earnings: %w", err)
	}
	if len(earnings) == 0 {
		return nil
	}

	var total int
	var earningIDs []string
	for _, e := range earnings {
		total += e.NetAmountCentavos
		earningIDs = append(earningIDs, e.ID)
	}

	payout := &OwnerPayout{
		OwnerID:             ownerID,
		PayoutAccountID:     account.ID,
		TotalAmountCentavos: total,
		EarningsCount:       len(earnings),
		Status:              "processing",
		ScheduledFor:        time.Now().UTC(),
	}
	if err := s.repo.CreatePayout(ctx, payout); err != nil {
		return fmt.Errorf("create payout: %w", err)
	}

	// Initiate the PayMongo transfer.
	paymongoID, err := s.client.InitiatePayout(ctx, account.AccountType, account.AccountNumber, account.AccountName, total)
	now := time.Now().UTC()
	if err != nil {
		failReason := err.Error()
		_ = s.repo.UpdatePayoutStatus(ctx, payout.ID, "failed", nil, nil, nil, &failReason)
		return fmt.Errorf("initiate payout: %w", err)
	}

	// Mark earnings as paid_out and link them to the payout.
	if err := s.repo.MarkEarningsPaidOut(ctx, earningIDs, payout.ID); err != nil {
		log.Printf("marketplace.processOwnerPayout: mark paid_out failed for payout %s: %v", payout.ID, err)
	}

	// Update payout to 'paid'.
	if err := s.repo.UpdatePayoutStatus(ctx, payout.ID, "paid", &paymongoID, &now, &now, nil); err != nil {
		return fmt.Errorf("update payout status: %w", err)
	}
	return nil
}
