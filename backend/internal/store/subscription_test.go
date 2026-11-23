package store

import (
	"context"
	"errors"
	"testing"
)

func TestValidSubscriptionTransition(t *testing.T) {
	allowed := [][2]string{
		{SubscriptionTrial, SubscriptionTrial},
		{SubscriptionTrial, SubscriptionActive},
		{SubscriptionTrial, SubscriptionOverdue},
		{SubscriptionTrial, SubscriptionCancelled},
		{SubscriptionActive, SubscriptionActive},
		{SubscriptionActive, SubscriptionOverdue},
		{SubscriptionActive, SubscriptionCancelled},
		{SubscriptionOverdue, SubscriptionOverdue},
		{SubscriptionOverdue, SubscriptionActive},
		{SubscriptionOverdue, SubscriptionCancelled},
	}
	for _, pair := range allowed {
		if !ValidSubscriptionTransition(pair[0], pair[1]) {
			t.Fatalf("%s → %s should be allowed", pair[0], pair[1])
		}
	}
	denied := [][2]string{
		{SubscriptionActive, SubscriptionTrial},
		{SubscriptionOverdue, SubscriptionTrial},
		{SubscriptionCancelled, SubscriptionActive},
		{SubscriptionCancelled, SubscriptionTrial},
		{SubscriptionCancelled, SubscriptionOverdue},
		{SubscriptionCancelled, SubscriptionCancelled},
		{"ended", SubscriptionActive},
		{SubscriptionActive, "ended"},
		{"", SubscriptionActive},
	}
	for _, pair := range denied {
		if ValidSubscriptionTransition(pair[0], pair[1]) {
			t.Fatalf("%s → %s should be rejected", pair[0], pair[1])
		}
	}
}

func TestReplaceTenantSubscriptionRejectsUnknownStatus(t *testing.T) {
	repo := NewTenantRepository(&DB{})
	_, err := repo.ReplaceTenantSubscription(context.Background(), 2, 1, "ended")
	if !errors.Is(err, ErrInvalidSubscriptionTransition) {
		t.Fatalf("expected invalid transition, got %v", err)
	}
}
