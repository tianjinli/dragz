package jobasynq

import (
	"encoding/json"
	"testing"
)

func TestJobPayloadRoundTrip(t *testing.T) {
	payload, err := json.Marshal(JobPayload{JobID: 42, TriggerType: "manual"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded JobPayload
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.JobID != 42 || decoded.TriggerType != "manual" {
		t.Fatalf("unexpected payload: %+v", decoded)
	}
}
