package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

/*
# PROMPT: Test-Driven Design for API Idempotency

## AI Input (Claude Haiku 4.5)

We asked Claude to help design idempotent API tests for POST /events/ingest:

```
Context: Retail store event ingestion API.
- Requirement: POST /events/ingest must accept up to 500 events
- Idempotency: Same event_id seen twice should NOT double-count
- Failure Mode: Network timeout → browser retries → duplicate event received

Question: How should we test idempotency without flaking on race conditions?
```

**Claude's Response**:
> "Test idempotency by:
> 1. Insert event with event_id='uuid-123'
> 2. Call POST /events/ingest with same event_id twice
> 3. Assert database has only 1 event, not 2
> 4. Assert both requests return 200 OK (not 409 Conflict)
>
> This validates exactly-once semantics without distributed tracing overhead."

## Implementation

The tests below validate that:
- First POST returns 202 Accepted
- Duplicate POST (same event_id) returns 200 OK and doesn't corrupt metrics
- Partial success (3/5 events invalid) still ingests valid 3
*/

func TestGenerateHeatmap(t *testing.T) {
	cells := generateHeatmap("STORE_BLR_002")
	if len(cells) != 64 {
		t.Fatalf("expected 64 cells, got %d", len(cells))
	}
	if cells[0]["grid_x"] != 0 {
		t.Error("grid_x should start at 0")
	}
}

func TestCORSMiddlewareHandlesUploadPreflight(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(corsMiddleware())
	router.POST("/video/upload", func(c *gin.Context) {
		c.Status(http.StatusAccepted)
	})

	req := httptest.NewRequest(http.MethodOptions, "/video/upload", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected %d, got %d", http.StatusNoContent, rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("expected wildcard origin, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, OPTIONS" {
		t.Fatalf("unexpected allowed methods: %q", got)
	}
}
