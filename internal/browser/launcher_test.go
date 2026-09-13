package browser

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestIsBrowserSupported(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"chrome", true},
		{"Chrome", true},
		{"edge", true},
		{"Edge", true},
		{"brave", true},
		{"firefox", false},
		{"safari", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := IsBrowserSupported(tt.name); got != tt.expected {
			t.Errorf("IsBrowserSupported(%q) = %v, want %v", tt.name, got, tt.expected)
		}
	}
}

func TestResolveProfileDir_Custom(t *testing.T) {
	tmpDir := t.TempDir()
	customPath := filepath.Join(tmpDir, "my-custom-profile")

	got, err := ResolveProfileDir("chrome", customPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != customPath {
		t.Errorf("got %q, want %q", got, customPath)
	}

	if fi, err := os.Stat(got); err != nil || !fi.IsDir() {
		t.Errorf("profile dir was not created or is not a directory")
	}
}

func TestResolveProfileDir_Default(t *testing.T) {
	got, err := ResolveProfileDir("chrome", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !filepath.IsAbs(got) {
		t.Errorf("expected absolute path, got %q", got)
	}

	if fi, err := os.Stat(got); err != nil || !fi.IsDir() {
		t.Errorf("default profile dir was not created: %v", err)
	}
}

func TestBuildLaunchArgs(t *testing.T) {
	cfg := LaunchConfig{
		Browser:    "chrome",
		ProfileDir: "C:\\tmp\\profile",
		Port:       9222,
		Headless:   true,
	}

	args := BuildLaunchArgs(cfg)

	hasArg := func(prefix string) bool {
		for _, a := range args {
			if len(a) >= len(prefix) && a[:len(prefix)] == prefix {
				return true
			}
		}
		return false
	}

	if !hasArg("--remote-debugging-port=9222") {
		t.Errorf("missing remote-debugging-port arg in %v", args)
	}
	if !hasArg("--user-data-dir=C:\\tmp\\profile") {
		t.Errorf("missing user-data-dir arg in %v", args)
	}
	if !hasArg("--headless=new") {
		t.Errorf("missing headless arg in %v", args)
	}
}

func TestFindBinary_CurrentOS(t *testing.T) {
	if runtime.GOOS == "windows" {
		// Verify finding at least one known browser on Windows if installed
		path, err := FindBinary("chrome")
		if err == nil {
			if fi, err := os.Stat(path); err != nil || fi.IsDir() {
				t.Errorf("found binary path %q is invalid", path)
			}
		}
	}
}
