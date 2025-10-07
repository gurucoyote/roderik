package cmd

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-rod/rod"
)

func TestMCPClickSuccess(t *testing.T) {
	resetNavGlobals()

	el := &rod.Element{}
	CurrentElement = el

	clickCalled := false
	prevClick := clickElementFunc
	clickElementFunc = func(got *rod.Element) error {
		if got != el {
			t.Fatalf("click received unexpected element")
		}
		clickCalled = true
		return nil
	}
	defer func() { clickElementFunc = prevClick }()

	prevFallback := clickFallbackFunc
	clickFallbackFunc = func(error) bool {
		t.Fatalf("fallback should not be invoked on success")
		return false
	}
	defer func() { clickFallbackFunc = prevFallback }()

	msg, err := mcpClick()
	if err != nil {
		t.Fatalf("mcpClick returned error: %v", err)
	}
	if !clickCalled {
		t.Fatalf("expected clickElementFunc to be called")
	}
	if msg != "clicked current element" {
		t.Fatalf("unexpected message: %q", msg)
	}
}

func TestMCPClickFallback(t *testing.T) {
	resetNavGlobals()

	el := &rod.Element{}
	CurrentElement = el

	prevClick := clickElementFunc
	clickElementFunc = func(*rod.Element) error {
		return errors.New("boom")
	}
	defer func() { clickElementFunc = prevClick }()

	fallbackCalled := false
	prevFallback := clickFallbackFunc
	clickFallbackFunc = func(err error) bool {
		fallbackCalled = true
		if err == nil || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("fallback received unexpected error: %v", err)
		}
		return true
	}
	defer func() { clickFallbackFunc = prevFallback }()

	msg, err := mcpClick()
	if err != nil {
		t.Fatalf("mcpClick returned error: %v", err)
	}
	if !fallbackCalled {
		t.Fatalf("expected fallback to run")
	}
	if msg != "click fallback executed" {
		t.Fatalf("unexpected message: %q", msg)
	}
}

func TestMCPClickFailsWhenFallbackDoesNotHandle(t *testing.T) {
	resetNavGlobals()

	CurrentElement = &rod.Element{}

	prevClick := clickElementFunc
	clickElementFunc = func(*rod.Element) error { return errors.New("boom") }
	defer func() { clickElementFunc = prevClick }()

	prevFallback := clickFallbackFunc
	clickFallbackFunc = func(error) bool { return false }
	defer func() { clickFallbackFunc = prevFallback }()

	if _, err := mcpClick(); err == nil {
		t.Fatalf("expected error when fallback cannot handle click")
	}
}

func TestMCPClickRequiresCurrentElement(t *testing.T) {
	resetNavGlobals()
	if _, err := mcpClick(); err == nil {
		t.Fatalf("expected error when no current element")
	}
}

func TestMCPTypeSuccess(t *testing.T) {
	resetNavGlobals()

	el := &rod.Element{}
	CurrentElement = el

	typeCalled := false
	prevType := typeInputFunc
	typeInputFunc = func(got *rod.Element, text string) error {
		if got != el {
			t.Fatalf("type received unexpected element")
		}
		if text != "hello world" {
			t.Fatalf("unexpected text passed to type: %q", text)
		}
		typeCalled = true
		return nil
	}
	defer func() { typeInputFunc = prevType }()

	prevSetValue := setValueFunc
	setValueFunc = func(*rod.Element, string) bool {
		t.Fatalf("setValue should not run on success")
		return false
	}
	defer func() { setValueFunc = prevSetValue }()

	msg, err := mcpType("  \"hello world\"  ")
	if err != nil {
		t.Fatalf("mcpType returned error: %v", err)
	}
	if !typeCalled {
		t.Fatalf("expected typeInputFunc to be invoked")
	}
	if msg != "typed text into current element" {
		t.Fatalf("unexpected message: %q", msg)
	}
}

func TestMCPTypeFallback(t *testing.T) {
	resetNavGlobals()

	el := &rod.Element{}
	CurrentElement = el

	prevType := typeInputFunc
	typeInputFunc = func(*rod.Element, string) error {
		return errors.New("input failed")
	}
	defer func() { typeInputFunc = prevType }()

	setValueCalled := false
	prevSetValue := setValueFunc
	setValueFunc = func(got *rod.Element, text string) bool {
		if got != el {
			t.Fatalf("setValue received unexpected element")
		}
		if text != "fallback" {
			t.Fatalf("unexpected text in fallback: %q", text)
		}
		setValueCalled = true
		return true
	}
	defer func() { setValueFunc = prevSetValue }()

	msg, err := mcpType("fallback")
	if err != nil {
		t.Fatalf("mcpType returned error: %v", err)
	}
	if !setValueCalled {
		t.Fatalf("expected setValue fallback to run")
	}
	if msg != "typed text via javascript fallback" {
		t.Fatalf("unexpected message: %q", msg)
	}
}

func TestMCPTypeFailsWhenFallbackFails(t *testing.T) {
	resetNavGlobals()

	CurrentElement = &rod.Element{}

	prevType := typeInputFunc
	typeInputFunc = func(*rod.Element, string) error { return errors.New("input failed") }
	defer func() { typeInputFunc = prevType }()

	prevSetValue := setValueFunc
	setValueFunc = func(*rod.Element, string) bool { return false }
	defer func() { setValueFunc = prevSetValue }()

	if _, err := mcpType("text"); err == nil {
		t.Fatalf("expected error when both input and fallback fail")
	}
}

func TestMCPTypeRequiresTextAndElement(t *testing.T) {
	resetNavGlobals()

	if _, err := mcpType("hello"); err == nil {
		t.Fatalf("expected error when no current element")
	}

	CurrentElement = &rod.Element{}
	if _, err := mcpType("  "); err == nil {
		t.Fatalf("expected error when text is empty")
	}
}
