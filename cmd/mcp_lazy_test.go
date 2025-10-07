package cmd

import (
	"testing"

	"github.com/go-rod/rod"
)

func TestDesktopLazyInitSkipsStartupForMCP(t *testing.T) {
	desktopConnectorBackup := desktopConnector
	prepareBrowserBackup := prepareBrowserFunc
	defer func() {
		desktopConnector = desktopConnectorBackup
		prepareBrowserFunc = prepareBrowserBackup
	}()

	Browser = nil
	Page = nil
	CurrentElement = nil
	Desktop = true

	RootCmd.PersistentPreRun(mcpCmd, nil)
	if Browser != nil || Page != nil {
		t.Fatalf("expected browser/page to remain nil until first MCP action when --desktop is used")
	}

	called := false
	desktopConnector = func(func(string, ...interface{})) (string, string, error) {
		called = true
		Browser = rod.New()
		Page = &rod.Page{}
		return "ws://fake", "localhost", nil
	}

	_, err := withPage(func() (string, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("withPage returned error: %v", err)
	}
	if !called {
		t.Fatalf("expected desktop connector to be invoked lazily on first MCP access")
	}
}
