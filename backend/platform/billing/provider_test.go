package billing

import (
	"context"
	"testing"
)

func TestNoopCharge(t *testing.T) {
	if err := (Noop{}).Charge(context.Background(), Invoice{ID: 1, Currency: "CNY"}); err != nil {
		t.Fatal(err)
	}
}
