package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestUpdateChromeLocalStateAddsProfileEntry(t *testing.T) {
	tmpDir := t.TempDir()
	localStatePath := filepath.Join(tmpDir, "Local State")
	initial := `{"profile":{"info_cache":{"Default":{"name":"Person 1","is_using_default_name":true}},"last_active_profile":"Default"}}`
	if err := os.WriteFile(localStatePath, []byte(initial), 0644); err != nil {
		t.Fatalf("write initial Local State: %v", err)
	}

	const wantTitle = "WSL2 Profile"
	keys, err := updateChromeLocalState(localStatePath, "WSL2", wantTitle)
	if err != nil {
		t.Fatalf("updateChromeLocalState returned error: %v", err)
	}
	if len(keys) != 1 || keys[0] != "WSL2" {
		t.Fatalf("expected updated keys [WSL2], got %#v", keys)
	}

	infoCache := readInfoCache(t, localStatePath)
	wsEntry, ok := infoCache["WSL2"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected info_cache entry for WSL2, got %T", infoCache["WSL2"])
	}
	if got := wsEntry["name"].(string); got != wantTitle {
		t.Fatalf("expected WSL2 name %q, got %q", wantTitle, got)
	}
	if got, ok := wsEntry["is_using_default_name"].(bool); !ok || got {
		t.Fatalf("expected is_using_default_name false, got %#v", wsEntry["is_using_default_name"])
	}

	defEntry, ok := infoCache["Default"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected info_cache entry for Default, got %T", infoCache["Default"])
	}
	if got := defEntry["name"].(string); got != "Person 1" {
		t.Fatalf("expected Default name unchanged, got %q", got)
	}
}

func TestUpdateChromeLocalStateFallbackDefault(t *testing.T) {
	tmpDir := t.TempDir()
	localStatePath := filepath.Join(tmpDir, "Local State")

	const wantTitle = "Desktop Profile"
	keys, err := updateChromeLocalState(localStatePath, "", wantTitle)
	if err != nil {
		t.Fatalf("updateChromeLocalState returned error: %v", err)
	}
	if len(keys) != 1 || keys[0] != "Default" {
		t.Fatalf("expected updated keys [Default], got %#v", keys)
	}

	infoCache := readInfoCache(t, localStatePath)
	entry, ok := infoCache["Default"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected Default entry, got %T", infoCache["Default"])
	}
	if got := entry["name"].(string); got != wantTitle {
		t.Fatalf("expected Default name %q, got %q", wantTitle, got)
	}
	if got, ok := entry["is_using_default_name"].(bool); !ok || got {
		t.Fatalf("expected is_using_default_name false, got %#v", entry["is_using_default_name"])
	}
}

func readInfoCache(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Local State: %v", err)
	}
	var data map[string]interface{}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("unmarshal Local State: %v", err)
	}
	profileObj, _ := data["profile"].(map[string]interface{})
	if profileObj == nil {
		t.Fatalf("Local State missing profile object")
	}
	infoCache, _ := profileObj["info_cache"].(map[string]interface{})
	if infoCache == nil {
		t.Fatalf("Local State missing info_cache")
	}
	return infoCache
}

func TestUpdateChromePreferences(t *testing.T) {
	tmpDir := t.TempDir()
	preferencesPath := filepath.Join(tmpDir, "Preferences")
	initial := `{"profile":{"name":"Person 1","is_using_default_name":true}}`
	if err := os.WriteFile(preferencesPath, []byte(initial), 0644); err != nil {
		t.Fatalf("write initial Preferences: %v", err)
	}

	const wantTitle = "Friendly"
	if err := updateChromePreferences(preferencesPath, wantTitle); err != nil {
		t.Fatalf("updateChromePreferences returned error: %v", err)
	}

	raw, err := os.ReadFile(preferencesPath)
	if err != nil {
		t.Fatalf("read Preferences: %v", err)
	}
	var data map[string]interface{}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("unmarshal Preferences: %v", err)
	}
	profileObj, _ := data["profile"].(map[string]interface{})
	if profileObj == nil {
		t.Fatalf("Preferences missing profile object")
	}
	if got := profileObj["name"].(string); got != wantTitle {
		t.Fatalf("expected name %q, got %q", wantTitle, got)
	}
	if got, ok := profileObj["is_using_default_name"].(bool); !ok || got {
		t.Fatalf("expected is_using_default_name false, got %#v", profileObj["is_using_default_name"])
	}
}
