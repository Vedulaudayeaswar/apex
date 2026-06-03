package events

import (
	"encoding/json"
	"time"
)

const (
	EventEntry      = "ENTRY"
	EventExit       = "EXIT"
	EventZoneEnter  = "ZONE_ENTER"
	EventZoneExit   = "ZONE_EXIT"
	EventQueueJoin  = "QUEUE_JOIN"
	EventQueueLeave = "QUEUE_LEAVE"
	EventReentry    = "REENTRY"
	EventDetection  = "DETECTION"
	EventAnomaly    = "ANOMALY"
	EventReset      = "RESET"
)

type Metadata struct {
	QueueDepth      int       `json:"queue_depth,omitempty"`
	SessionSeq      int       `json:"session_seq,omitempty"`
	ReentryDetected bool      `json:"reentry_detected,omitempty"`
	MovementPattern string    `json:"movement_pattern,omitempty"`
	BBox            []float64 `json:"bbox,omitempty"`
	TrackID         int       `json:"track_id,omitempty"`
	AnomalyType     string    `json:"anomaly_type,omitempty"`
	Severity        string    `json:"severity,omitempty"`
	ActiveVisitors  int       `json:"active_visitors,omitempty"`
	ConversionRate  float64   `json:"conversion_rate,omitempty"`
}

type StoreEvent struct {
	EventID    string    `json:"event_id"`
	StoreID    string    `json:"store_id"`
	CameraID   string    `json:"camera_id"`
	VisitorID  string    `json:"visitor_id"`
	EventType  string    `json:"event_type"`
	Timestamp  time.Time `json:"timestamp"`
	ZoneID     string    `json:"zone_id"`
	DwellMs    int64     `json:"dwell_ms"`
	IsStaff    bool      `json:"is_staff"`
	Confidence float64   `json:"confidence"`
	Metadata   Metadata  `json:"metadata"`
}

func (e *StoreEvent) MarshalJSON() ([]byte, error) {
	type Alias StoreEvent
	return json.Marshal(&struct {
		Timestamp string `json:"timestamp"`
		*Alias
	}{
		Timestamp: e.Timestamp.UTC().Format(time.RFC3339Nano),
		Alias:     (*Alias)(e),
	})
}

func (e *StoreEvent) UnmarshalJSON(data []byte) error {
	type Alias StoreEvent
	aux := &struct {
		Timestamp string `json:"timestamp"`
		*Alias
	}{
		Alias: (*Alias)(e),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	t, err := time.Parse(time.RFC3339Nano, aux.Timestamp)
	if err != nil {
		t, err = time.Parse(time.RFC3339, aux.Timestamp)
		if err != nil {
			return err
		}
	}
	e.Timestamp = t
	return nil
}

func ValidEventTypes() map[string]bool {
	return map[string]bool{
		EventEntry: true, EventExit: true, EventZoneEnter: true,
		EventZoneExit: true, EventQueueJoin: true, EventQueueLeave: true,
		EventReentry: true, EventDetection: true, EventAnomaly: true,
		EventReset: true,
	}
}
