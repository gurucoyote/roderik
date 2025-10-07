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
	logA := &EventLog{}
	logB := &EventLog{}

	setActiveEventLog(logA)
	appendEventLog("first")

	setActiveEventLog(logB)
	appendEventLog("second")

	if len(logA.logs) != 1 || logA.logs[0] != "first" {
		t.Fatalf("logA expected single entry 'first', got %#v", logA.logs)
	}
	if len(logB.logs) != 1 || logB.logs[0] != "second" {
		t.Fatalf("logB expected single entry 'second', got %#v", logB.logs)
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
