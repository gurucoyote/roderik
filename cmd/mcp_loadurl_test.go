package cmd

import "testing"

func TestLoadURLToolEnabledDefaultTrue(t *testing.T) {
	t.Setenv("RODERIK_ENABLE_LOAD_URL", "")
	if !loadURLToolEnabled() {
		t.Fatalf("expected load_url tool to be enabled by default")
	}
}

func TestLoadURLToolEnabledFalse(t *testing.T) {
	t.Setenv("RODERIK_ENABLE_LOAD_URL", "false")
	if loadURLToolEnabled() {
		t.Fatalf("expected load_url tool to be disabled when env set to false")
	}
}

func TestLoadURLToolEnabledZero(t *testing.T) {
	t.Setenv("RODERIK_ENABLE_LOAD_URL", "0")
	if loadURLToolEnabled() {
		t.Fatalf("expected load_url tool to be disabled when env set to 0")
	}
}

func TestLoadURLToolEnabledExplicitTrue(t *testing.T) {
	t.Setenv("RODERIK_ENABLE_LOAD_URL", "true")
	if !loadURLToolEnabled() {
		t.Fatalf("expected load_url tool to be enabled when env set to true")
	}
}

func TestNavigationToolsFollowLoadURLToggle(t *testing.T) {
	t.Setenv("RODERIK_ENABLE_LOAD_URL", "")
	if !navigationToolsEnabled() {
		t.Fatalf("navigation tools should be enabled when load_url is enabled")
	}

	t.Setenv("RODERIK_ENABLE_LOAD_URL", "false")
	if navigationToolsEnabled() {
		t.Fatalf("navigation tools should be disabled when load_url is disabled")
	}
}
