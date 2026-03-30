package payments

import "errors"

var (
	ErrPaymentNotFound         = errors.New("payments: not found")
	ErrInvalidWebhookSignature = errors.New("payments: invalid webhook signature")
	ErrDuplicateWebhookEvent   = errors.New("payments: duplicate webhook event")
	ErrRefundFailed            = errors.New("payments: refund failed")
	ErrVoidFailed              = errors.New("payments: void failed")
	ErrCaptureFailed           = errors.New("payments: capture failed")
)
