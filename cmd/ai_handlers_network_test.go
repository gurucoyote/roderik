package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-rod/rod"
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

func TestNetworkSaveHandlerDefaultsToServerFile(t *testing.T) {
	prevLog := getActiveEventLog()
	prevBrowser := Browser
	prevPage := Page
	t.Cleanup(func() {
		setActiveEventLog(prevLog)
		Browser = prevBrowser
		Page = prevPage
	})

	Browser = &rod.Browser{}
	Page = &rod.Page{}

	log := newNetworkEventLog()
	body := []byte("server-only-data")
	entry := &NetworkLogEntry{
		RequestID: "req-default",
		URL:       "https://example.com/resource.bin",
		Method:    "GET",
		Response: &NetworkResponseInfo{
			MIMEType: "application/octet-stream",
		},
		Body: &NetworkBody{
			Data: body,
		},
	}
	log.entries["req-default"] = entry
	log.order = append(log.order, "req-default")
	setActiveEventLog(log)

	tmpDir := t.TempDir()
	args := map[string]interface{}{
		"request_id": "req-default",
		"save_dir":   tmpDir,
	}

	res, err := networkSaveHandler(context.Background(), args)
	if err != nil {
		t.Fatalf("networkSaveHandler returned error: %v", err)
	}
	if len(res.Binary) != 0 {
		t.Fatalf("expected no binary payload, got %d bytes", len(res.Binary))
	}
	if res.FilePath == "" {
		t.Fatalf("expected file path in response")
	}
	if filepath.Dir(res.FilePath) != tmpDir {
		t.Fatalf("expected file saved under %q, got %q", tmpDir, filepath.Dir(res.FilePath))
	}

	data, err := os.ReadFile(res.FilePath)
	if err != nil {
		t.Fatalf("reading saved file: %v", err)
	}
	if string(data) != string(body) {
		t.Fatalf("expected saved file contents %q, got %q", string(body), string(data))
	}
	if !strings.Contains(res.Text, res.FilePath) {
		t.Fatalf("expected response text to mention saved path, got %q", res.Text)
	}
	if res.ContentType != "application/octet-stream" {
		t.Fatalf("expected content type application/octet-stream, got %q", res.ContentType)
	}
}

func TestNetworkSaveHandlerBinaryModeReturnsPayload(t *testing.T) {
	prevLog := getActiveEventLog()
	prevBrowser := Browser
	prevPage := Page
	t.Cleanup(func() {
		setActiveEventLog(prevLog)
		Browser = prevBrowser
		Page = prevPage
	})

	Browser = &rod.Browser{}
	Page = &rod.Page{}

	log := newNetworkEventLog()
	body := []byte("inline-binary-data")
	entry := &NetworkLogEntry{
		RequestID: "req-binary",
		URL:       "https://example.com/inline.bin",
		Method:    "GET",
		Response: &NetworkResponseInfo{
			MIMEType: "application/octet-stream",
		},
		Body: &NetworkBody{
			Data: body,
		},
	}
	log.entries["req-binary"] = entry
	log.order = append(log.order, "req-binary")
	setActiveEventLog(log)

	args := map[string]interface{}{
		"request_id": "req-binary",
		"return":     "binary",
	}

	res, err := networkSaveHandler(context.Background(), args)
	if err != nil {
		t.Fatalf("networkSaveHandler returned error: %v", err)
	}
	if res.FilePath != "" {
		t.Fatalf("expected no file path when returning binary, got %q", res.FilePath)
	}
	if string(res.Binary) != string(body) {
		t.Fatalf("expected binary payload %q, got %q", string(body), string(res.Binary))
	}
	if res.ContentType != "application/octet-stream" {
		t.Fatalf("expected content type application/octet-stream, got %q", res.ContentType)
	}
	if !strings.Contains(res.Text, "retrieved") {
		t.Fatalf("expected response text to indicate retrieval, got %q", res.Text)
	}
}
