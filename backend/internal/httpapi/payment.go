package httpapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"dragz-admin/backend/internal/app"
	apierrors "dragz-admin/backend/internal/pkg/errors"
	"dragz-admin/backend/internal/store"
	"dragz-admin/backend/platform/billing"
)

// maxWebhookBody bounds what a provider may post. A real event is a few KB; the
// cap keeps an unauthenticated endpoint from being used to allocate memory.
const maxWebhookBody = 1 << 20

// paymentWebhookHandler applies a payment provider callback exactly once.
//
// The idempotency key is (provider, event_id): the first delivery inserts the
// row and applies it, and every retry finds the stored row and answers 200
// without touching the invoice again. A failure leaves the row in "failed" so a
// retry is a real retry rather than a duplicate charge.
func paymentWebhookHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		provider := strings.TrimSpace(pathvarValue(r, "provider"))
		if ctx.Billing == nil || ctx.Tenants == nil {
			fail(w, r, http.StatusServiceUnavailable, apierrors.CodeFeatureDisabled, "errors.payment_provider_unavailable")
			return
		}
		if provider != ctx.Billing.Name() {
			fail(w, r, http.StatusNotFound, apierrors.CodeNotFound, "errors.not_found")
			return
		}
		if _, unsigned := ctx.Billing.(billing.UnsignedVerifier); unsigned && !ctx.AllowDevSecrets {
			// Accepting an unauthenticated callback would let anyone mark an
			// invoice paid. Outside development the operator has to configure a
			// signature.
			fail(w, r, http.StatusServiceUnavailable, apierrors.CodeFeatureDisabled, "errors.payment_provider_unavailable")
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBody))
		if err != nil {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.payment_event_invalid")
			return
		}
		event, err := ctx.Billing.Verify(r.Header, body)
		if err != nil {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.payment_event_invalid")
			return
		}
		stored, inserted, err := ctx.Tenants.RecordPaymentEvent(r.Context(),
			provider, event.ID, event.Type, event.TenantID, body)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		if !inserted {
			switch stored.Status {
			case store.PaymentProcessed, store.PaymentIgnored:
				// A retry of something already handled. 200 stops the provider
				// from retrying again.
				ok(w, r, map[string]any{"received": true, "duplicate": true})
				return
			}
		}
		if err := applyPaymentEvent(r.Context(), ctx, provider, stored, event); err != nil {
			_ = ctx.Tenants.MarkPaymentEvent(r.Context(), stored.ID, store.PaymentFailed, err.Error())
			// 500 asks the provider to retry. The row is already stored, so the
			// retry reuses it instead of creating a second one.
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.payment_apply_failed")
			return
		}
		ok(w, r, map[string]any{"received": true})
	}
}

// applyPaymentEvent is the business side of a callback.
//
// Every branch either applies the event and marks it processed, or marks it
// ignored with a reason. It never returns "there was nothing to do" silently:
// the stored row is what an operator reads when a tenant asks why a payment did
// not land.
func applyPaymentEvent(rowCtx context.Context, ctx *app.Context, provider string, stored store.PaymentEvent, event billing.Event) error {
	switch event.Type {
	case "invoice.paid", "invoice.payment_succeeded", "payment.succeeded":
		// Money arrived: settle the invoice, then lift a grace period that a
		// previous failed payment started.
		subscriptionID, handled, err := settleInvoice(rowCtx, ctx, stored, event, store.InvoicePaid)
		if err != nil {
			return err
		}
		if !handled {
			// settleInvoice already recorded why the event was ignored.
			return nil
		}
		if err := ctx.Tenants.ResumeSubscriptionAfterPayment(rowCtx, event.TenantID, subscriptionID); err != nil {
			return err
		}
		return ctx.Tenants.MarkPaymentEvent(rowCtx, stored.ID, store.PaymentProcessed, "")

	case "invoice.payment_failed", "invoice.payment_action_required", "invoice.past_due":
		// Money did not arrive. Mark the invoice so billing shows the dunning
		// state, and move the subscription to overdue: that starts the grace
		// window (see updateSubscriptionRow) instead of cutting access at once.
		_, handled, err := settleInvoice(rowCtx, ctx, stored, event, store.InvoiceFailed)
		if err != nil {
			return err
		}
		if !handled {
			return nil
		}
		if err := transitionQuietly(rowCtx, ctx, event.TenantID, store.SubscriptionOverdue); err != nil {
			return err
		}
		return ctx.Tenants.MarkPaymentEvent(rowCtx, stored.ID, store.PaymentProcessed, "")

	case "charge.refunded", "invoice.refunded", "refund.succeeded":
		// A refund ends the paid period. The invoice keeps the money trail
		// (status=refunded) and the subscription is cancelled: leaving it active
		// would mean the tenant keeps a plan nobody paid for.
		if _, handled, err := settleInvoice(rowCtx, ctx, stored, event, store.InvoiceRefunded); err != nil {
			return err
		} else if !handled {
			return nil
		}
		if err := transitionQuietly(rowCtx, ctx, event.TenantID, store.SubscriptionCancelled); err != nil {
			return err
		}
		return ctx.Tenants.MarkPaymentEvent(rowCtx, stored.ID, store.PaymentProcessed, "")

	case "subscription.cancelled", "subscription.canceled", "subscription.deleted", "subscription_expired":
		if event.TenantID == 0 {
			return ctx.Tenants.MarkPaymentEvent(rowCtx, stored.ID, store.PaymentIgnored, "missing tenantId")
		}
		_, err := ctx.Tenants.TransitionSubscription(rowCtx, event.TenantID, store.SubscriptionCancelled)
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrInvalidSubscriptionTransition) {
			// Nothing open to cancel: the tenant may already be cancelled.
			return ctx.Tenants.MarkPaymentEvent(rowCtx, stored.ID, store.PaymentIgnored, "no open subscription")
		}
		if err != nil {
			return err
		}
		return ctx.Tenants.MarkPaymentEvent(rowCtx, stored.ID, store.PaymentProcessed, "")

	case "subscription.renewed":
		// A renewal that carries no invoice still lifts a grace period.
		if event.TenantID == 0 {
			return ctx.Tenants.MarkPaymentEvent(rowCtx, stored.ID, store.PaymentIgnored, "missing tenantId")
		}
		if _, err := ctx.Tenants.TransitionSubscription(rowCtx, event.TenantID, store.SubscriptionActive); err != nil {
			if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrInvalidSubscriptionTransition) {
				return ctx.Tenants.MarkPaymentEvent(rowCtx, stored.ID, store.PaymentIgnored, "no open subscription")
			}
			return err
		}
		return ctx.Tenants.MarkPaymentEvent(rowCtx, stored.ID, store.PaymentProcessed, "")

	default:
		return ctx.Tenants.MarkPaymentEvent(rowCtx, stored.ID, store.PaymentIgnored, "unhandled event type")
	}
}

// transitionQuietly applies a subscription status that a payment event implies.
// A tenant with no open subscription (or one that already sits in the target
// state) is not a failure: the callback is still a truthful record of what the
// provider said, so it is applied and answered 200.
func transitionQuietly(rowCtx context.Context, ctx *app.Context, tenantID uint64, status string) error {
	if tenantID == 0 {
		return nil
	}
	_, err := ctx.Tenants.TransitionSubscription(rowCtx, tenantID, status)
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrInvalidSubscriptionTransition) {
		return nil
	}
	return err
}

// settleInvoice moves the event's invoice to status.
//
// handled reports whether the event said enough to act on. When the event names
// no tenant or no invoice, or the invoice does not exist here, the callback is
// recorded as ignored inside this function and handled is false: a provider
// retry cannot do better a second time, so answering 200 is correct.
//
// A handled, successful settle returns the subscription the invoice belongs to,
// which is 0 for a one-off charge that has no subscription to resume.
func settleInvoice(rowCtx context.Context, ctx *app.Context, stored store.PaymentEvent, event billing.Event, status string) (subscriptionID uint64, handled bool, err error) {
	if event.TenantID == 0 || event.InvoiceID == 0 {
		// A provider event that names neither our tenant nor our invoice is
		// recorded for forensics and otherwise ignored: failing it would
		// make the provider retry forever.
		return 0, false, ctx.Tenants.MarkPaymentEvent(rowCtx, stored.ID, store.PaymentIgnored, "missing tenantId or invoiceId")
	}
	subscriptionID, err = ctx.Tenants.SetInvoiceStatus(rowCtx, event.TenantID, event.InvoiceID, status)
	if errors.Is(err, store.ErrNotFound) {
		return 0, false, ctx.Tenants.MarkPaymentEvent(rowCtx, stored.ID, store.PaymentIgnored, "invoice not found")
	}
	if err != nil {
		return 0, false, err
	}
	return subscriptionID, true, nil
}
