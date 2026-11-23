// Package billing is the payment-provider seam. Reconcile writes draft
// invoices; charging them is a later adapter.
package billing

import "context"

// Invoice is the slice of a draft the provider needs in order to charge it.
type Invoice struct {
	ID             uint64
	TenantID       uint64
	SubscriptionID uint64
	Kind           string
	Status         string
	AmountCents    int64
	Currency       string
	Memo           string
}

// Provider charges one invoice. Implementations talk to a payment gateway.
type Provider interface {
	Charge(ctx context.Context, inv Invoice) error
}

// Noop accepts every charge and does not contact a gateway.
type Noop struct{}

// Charge reports success without taking a payment.
func (Noop) Charge(context.Context, Invoice) error { return nil }
