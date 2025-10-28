package cmd

import (
	"testing"

	"github.com/go-rod/rod"
)

func TestEnsurePageEventHandlersRegistersOncePerPage(t *testing.T) {
	originalRegister := registerPageEvents
	defer func() { registerPageEvents = originalRegister }()

	registerCalls := 0
	registerPageEvents = func(*rod.Page) {
		registerCalls++
	}

	pageEventMu.Lock()
	pageEventPage = nil
	pageEventMu.Unlock()

	page := &rod.Page{}
	ensurePageEventHandlers(page)
	ensurePageEventHandlers(page)

	if registerCalls != 1 {
		t.Fatalf("expected registerPageEvents to be called once, got %d", registerCalls)
	}

	ensurePageEventHandlers(&rod.Page{})
	if registerCalls != 2 {
		t.Fatalf("expected registerPageEvents to be called for new page, got %d", registerCalls)
	}
}

func TestSetActiveEventLogControlsAppend(t *testing.T) {
	original := getActiveEventLog()
	defer setActiveEventLog(original)

	logA := newNetworkEventLog()
	logB := newNetworkEventLog()

	setActiveEventLog(logA)
	appendEventLog("first")

	setActiveEventLog(logB)
	appendEventLog("second")

	msgsA := logA.Messages()
	if len(msgsA) != 1 || msgsA[0] != "first" {
		t.Fatalf("logA expected single entry 'first', got %#v", msgsA)
	}
	msgsB := logB.Messages()
	if len(msgsB) != 1 || msgsB[0] != "second" {
		t.Fatalf("logB expected single entry 'second', got %#v", msgsB)
	}
}

func TestEnsurePageEventHandlersNilSafe(t *testing.T) {
	pageEventMu.Lock()
	pageEventPage = nil
	pageEventMu.Unlock()

	ensurePageEventHandlers(nil)

	pageEventMu.Lock()
	defer pageEventMu.Unlock()
	if pageEventPage != nil {
		t.Fatalf("expected pageEventPage to remain nil when ensurePageEventHandlers called with nil")
	}
}

func TestSetNetworkActivityEnabled(t *testing.T) {
	original := isNetworkActivityEnabled()
	defer setNetworkActivityEnabled(original)

	changed := setNetworkActivityEnabled(!original)
	current := isNetworkActivityEnabled()
	if current == original {
		t.Fatalf("expected network activity state to flip from %t", original)
	}
	if !changed {
		t.Fatalf("expected change flag when toggling state")
	}
	unchanged := setNetworkActivityEnabled(current)
	if unchanged {
		t.Fatalf("expected change flag to be false when setting same state")
	}
}
