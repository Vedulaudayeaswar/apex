package events_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/apex-retail/shared/events"
)

func TestStoreEventRoundTrip(t *testing.T) {
	orig := events.StoreEvent{
		EventID:    "550e8400-e29b-41d4-a716-446655440000",
		StoreID:    "STORE_BLR_002",
		CameraID:   "CAM_ENTRY_01",
		VisitorID:  "VIS_001",
		EventType:  events.EventZoneEnter,
		Timestamp:  time.Now().UTC().Truncate(time.Millisecond),
		ZoneID:     "ELECTRONICS",
		DwellMs:    0,
		IsStaff:    false,
		Confidence: 0.91,
		Metadata: events.Metadata{
			QueueDepth:      4,
			SessionSeq:      2,
			ReentryDetected: false,
			MovementPattern: "NORMAL",
		},
	}

	data, err := json.Marshal(&orig)
	if err != nil {
		t.Fatal(err)
	}

	var decoded events.StoreEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.EventType != events.EventZoneEnter {
		t.Errorf("event_type = %s", decoded.EventType)
	}
	if decoded.Metadata.QueueDepth != 4 {
		t.Errorf("queue_depth = %d", decoded.Metadata.QueueDepth)
	}
}

func TestValidEventTypes(t *testing.T) {
	valid := events.ValidEventTypes()
	if !valid[events.EventReentry] {
		t.Error("REENTRY should be valid")
	}
	if valid["INVALID"] {
		t.Error("INVALID should not be valid")
	}
}
