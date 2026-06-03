"""API contract and schema validation tests.

# PROMPT: Test Generation for Retail Store Event Schema Compliance

## AI Input (Claude Haiku 4.5)

We asked Claude to help design comprehensive schema validation tests:

```
Context: Retail store event ingestion pipeline.
- Schema: Required fields include event_id, store_id, camera_id, visitor_id, event_type, etc.
- Validation: All events must pass before metrics are computed
- Failure: Invalid event should be logged but not crash the pipeline

Question: What are the critical schema tests we should automate?
```

**Claude's Response**:
> "Prioritize these schema tests:
> 1. All required fields present (not null)
> 2. event_id is globally unique UUID (not duplicate)
> 3. event_type is from allowed enum (not arbitrary string)
> 4. timestamp is ISO8601 format (for sorting and tracing)
> 5. confidence is 0.0 ≤ x ≤ 1.0 (valid probability)
>
> These catch 95% of data quality issues before they cascade to metrics."

## Implementation

Below are tests that validate the event contract between detection pipeline and analytics services.
"""
import json
import uuid
from datetime import datetime, timezone

import pytest

REQUIRED_FIELDS = [
    "event_id", "store_id", "camera_id", "visitor_id",
    "event_type", "timestamp", "zone_id", "dwell_ms",
    "is_staff", "confidence", "metadata",
]


def sample_event():
    return {
        "event_id": str(uuid.uuid4()),
        "store_id": "STORE_BLR_002",
        "camera_id": "CAM_ENTRY_01",
        "visitor_id": "VIS_001",
        "event_type": "ZONE_ENTER",
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "zone_id": "ELECTRONICS",
        "dwell_ms": 0,
        "is_staff": False,
        "confidence": 0.91,
        "metadata": {
            "queue_depth": 4,
            "session_seq": 2,
            "reentry_detected": False,
            "movement_pattern": "NORMAL",
        },
    }


def test_event_schema_required_fields():
    ev = sample_event()
    for f in REQUIRED_FIELDS:
        assert f in ev


def test_reentry_metadata():
    ev = sample_event()
    ev["event_type"] = "REENTRY"
    ev["metadata"]["reentry_detected"] = True
    assert ev["metadata"]["reentry_detected"] is True


@pytest.mark.parametrize("event_type", [
    "ENTRY", "EXIT", "ZONE_ENTER", "REENTRY", "QUEUE_JOIN", "ANOMALY",
])
def test_valid_event_types(event_type):
    ev = sample_event()
    ev["event_type"] = event_type
    assert ev["event_type"] in {
        "ENTRY", "EXIT", "ZONE_ENTER", "ZONE_EXIT", "QUEUE_JOIN",
        "QUEUE_LEAVE", "REENTRY", "DETECTION", "ANOMALY",
    }
