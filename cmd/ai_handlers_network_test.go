package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

func TestNetworkListHandlerDefaultLimit(t *testing.T) {
	prev := getActiveEventLog()
	t.Cleanup(func() {
		setActiveEventLog(prev)
	})

	log := newNetworkEventLog()
	total := networkListDefaultLimit + 30
	for i := 0; i < total; i++ {
		id := fmt.Sprintf("req-%03d", i)
		log.entries[id] = &NetworkLogEntry{
			RequestID: id,
			URL:       fmt.Sprintf("https://example.com/%d", i),
			Method:    "GET",
		}
		log.order = append(log.order, id)
	}
	setActiveEventLog(log)

	res, err := networkListHandler(context.Background(), nil)
	if err != nil {
		t.Fatalf("networkListHandler returned error: %v", err)
	}

	var payload networkListResponse
	if err := json.Unmarshal([]byte(res.Text), &payload); err != nil {
		t.Fatalf("expected JSON response, got error: %v", err)
	}

	if payload.Total != total {
		t.Fatalf("expected total %d, got %d", total, payload.Total)
	}
	if payload.Returned != networkListDefaultLimit {
		t.Fatalf("expected returned %d, got %d", networkListDefaultLimit, payload.Returned)
	}
	if len(payload.Entries) != networkListDefaultLimit {
		t.Fatalf("expected entries length %d, got %d", networkListDefaultLimit, len(payload.Entries))
	}
	expectedStart := total - networkListDefaultLimit
	if payload.Entries[0].RequestID != fmt.Sprintf("req-%03d", expectedStart) {
		t.Fatalf("unexpected first entry request id %q", payload.Entries[0].RequestID)
	}
	if payload.Entries[len(payload.Entries)-1].RequestID != fmt.Sprintf("req-%03d", total-1) {
		t.Fatalf("unexpected last entry request id %q", payload.Entries[len(payload.Entries)-1].RequestID)
	}
	if !payload.HasMore {
		t.Fatalf("expected has_more to be true when results exceed default limit")
	}
	if !payload.Tail {
		t.Fatalf("expected tail=true by default")
	}
	expectedOffset := total - payload.Returned
	if payload.Offset != expectedOffset {
		t.Fatalf("expected offset %d, got %d", expectedOffset, payload.Offset)
	}
}

func TestNetworkListHandlerOffsetAndLimit(t *testing.T) {
	prev := getActiveEventLog()
	t.Cleanup(func() {
		setActiveEventLog(prev)
	})

	log := newNetworkEventLog()
	total := 10
	for i := 0; i < total; i++ {
		id := fmt.Sprintf("req-%03d", i)
		log.entries[id] = &NetworkLogEntry{
			RequestID: id,
			URL:       fmt.Sprintf("https://example.com/%d", i),
			Method:    "GET",
		}
		log.order = append(log.order, id)
	}
	setActiveEventLog(log)

	args := map[string]interface{}{
		"limit":  3,
		"offset": 4,
		"tail":   false,
	}
	res, err := networkListHandler(context.Background(), args)
	if err != nil {
		t.Fatalf("networkListHandler returned error: %v", err)
	}

	var payload networkListResponse
	if err := json.Unmarshal([]byte(res.Text), &payload); err != nil {
		t.Fatalf("expected JSON response, got error: %v", err)
	}

	if payload.Total != total {
		t.Fatalf("expected total %d, got %d", total, payload.Total)
	}
	if payload.Offset != 4 {
		t.Fatalf("expected offset 4, got %d", payload.Offset)
	}
	if payload.Returned != 3 {
		t.Fatalf("expected returned 3, got %d", payload.Returned)
	}
	if !payload.HasMore {
		t.Fatalf("expected has_more to be true with remaining entries")
	}
	if payload.Tail {
		t.Fatalf("expected tail=false when explicitly disabled")
	}

	expectedFirstID := "req-004"
	if payload.Entries[0].RequestID != expectedFirstID {
		t.Fatalf("expected first entry id %q, got %q", expectedFirstID, payload.Entries[0].RequestID)
	}
	expectedLastID := "req-006"
	if payload.Entries[len(payload.Entries)-1].RequestID != expectedLastID {
		t.Fatalf("expected last entry id %q, got %q", expectedLastID, payload.Entries[len(payload.Entries)-1].RequestID)
	}
}

func TestNetworkListHandlerInvalidOffset(t *testing.T) {
	prev := getActiveEventLog()
	t.Cleanup(func() {
		setActiveEventLog(prev)
	})

	setActiveEventLog(newNetworkEventLog())

	args := map[string]interface{}{
		"offset": -1,
		"tail":   false,
	}
	if _, err := networkListHandler(context.Background(), args); err == nil {
		t.Fatalf("expected error for negative offset")
	}
}
