package noticenotify

import (
	"context"
	"testing"
	"time"

	"dragz-admin/backend/internal/store"
)

type fakePusher struct {
	calls []any
}

func (p *fakePusher) Push(_ []uint64, payload any) error {
	p.calls = append(p.calls, payload)
	return nil
}

func TestDeliverOneSkipsFuturePublishAt(t *testing.T) {
	future := time.Now().Add(time.Hour)
	pusher := &fakePusher{}
	ok, err := DeliverOne(context.Background(), nil, pusher, store.Notice{
		ID: 1, Title: "later", PublishAt: &future,
	})
	if err != nil {
		t.Fatalf("DeliverOne: %v", err)
	}
	if ok {
		t.Fatal("expected future notice to be skipped")
	}
	if len(pusher.calls) != 0 {
		t.Fatalf("pushes = %d, want 0", len(pusher.calls))
	}
}
