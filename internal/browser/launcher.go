package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/mafredri/cdp"
	"github.com/mafredri/cdp/rpcc"
)

// Supported browser identifiers
const (
	BrowserChrome = "chrome"
	BrowserEdge   = "edge"
	BrowserBrave  = "brave"
)

// LaunchConfig contains options for launching a browser instance.
type LaunchConfig struct {
	Browser    string
	BinaryPath string
	ProfileDir string
	Port       int
	Headless   bool
}

// KnownPaths holds standard executable paths per platform.
var knownPaths = map[string]map[string][]string{
	"windows": {
		BrowserChrome: {
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
			filepath.Join(os.Getenv("LOCALAPPDATA"), `Google\Chrome\Application\chrome.exe`),
		},
		BrowserEdge: {
			`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
			`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
			filepath.Join(os.Getenv("LOCALAPPDATA"), `Microsoft\Edge\Application\msedge.exe`),
		},
		BrowserBrave: {
			`C:\Program Files\BraveSoftware\Brave-Browser\Application\brave.exe`,
			`C:\Program Files (x86)\BraveSoftware\Brave-Browser\Application\brave.exe`,
			filepath.Join(os.Getenv("LOCALAPPDATA"), `BraveSoftware\Brave-Browser\Application\brave.exe`),
		},
	},
	"darwin": {
		BrowserChrome: {
			`/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`,
		},
		BrowserEdge: {
			`/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge`,
		},
		BrowserBrave: {
			`/Applications/Brave Browser.app/Contents/MacOS/Brave Browser`,
		},
	},
	"linux": {
		BrowserChrome: {
			"google-chrome",
			"google-chrome-stable",
			"chromium",
			"chromium-browser",
		},
		BrowserEdge: {
			"microsoft-edge",
			"microsoft-edge-stable",
		},
		BrowserBrave: {
			"brave-browser",
			"brave",
		},
	},
}

// IsBrowserSupported returns true if the browser name is known.
func IsBrowserSupported(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case BrowserChrome, BrowserEdge, BrowserBrave:
		return true
	default:
		return false
	}
}

// FindBinary searches for the browser binary on the system.
func FindBinary(browserType string) (string, error) {
	bType := strings.ToLower(strings.TrimSpace(browserType))

	// Check if user gave an exact executable path
	if fi, err := os.Stat(browserType); err == nil && !fi.IsDir() {
		return filepath.Abs(browserType)
	}

	platformPaths, ok := knownPaths[runtime.GOOS]
	if !ok {
		platformPaths = knownPaths["linux"]
	}

	paths, ok := platformPaths[bType]
	if !ok {
		return "", fmt.Errorf("unsupported browser '%s'. Supported: chrome, edge, brave", browserType)
	}

	for _, p := range paths {
		if runtime.GOOS == "linux" {
			if fullPath, err := exec.LookPath(p); err == nil {
				return fullPath, nil
			}
		} else {
			if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
				return p, nil
			}
		}
	}

	return "", fmt.Errorf("could not find %s executable on this system. Specify full path or install %s", bType, bType)
}

// ResolveProfileDir returns the absolute path to the browser user data directory.
// Defaults to ~/.narr/profiles/<browser> unless customDir is provided.
func ResolveProfileDir(browserType string, customDir string) (string, error) {
	bType := strings.ToLower(strings.TrimSpace(browserType))
	if bType == "" {
		bType = "default"
	}

	if customDir != "" {
		absPath, err := filepath.Abs(customDir)
		if err != nil {
			return "", fmt.Errorf("invalid profile directory %q: %w", customDir, err)
		}
		if err := os.MkdirAll(absPath, 0755); err != nil {
			return "", fmt.Errorf("failed to create profile directory %q: %w", absPath, err)
		}
		return absPath, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}

	profileDir := filepath.Join(homeDir, ".narr", "profiles", bType)
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create default profile directory %q: %w", profileDir, err)
	}

	return profileDir, nil
}

// BuildLaunchArgs constructs the command-line arguments for Chromium-based browsers.
func BuildLaunchArgs(cfg LaunchConfig) []string {
	port := cfg.Port
	if port <= 0 {
		port = 9222
	}

	args := []string{
		fmt.Sprintf("--remote-debugging-port=%d", port),
		fmt.Sprintf("--user-data-dir=%s", cfg.ProfileDir),
		"--no-first-run",
		"--no-default-browser-check",
	}

	if cfg.Headless {
		args = append(args, "--headless=new")
	}

	return args
}

// TerminatePreviousInstances ensures no previous debug browser instance is lingering on the given port or profile dir.
// Applies to ALL supported browsers (Chrome, Edge, Brave, etc.).
func TerminatePreviousInstances(browserName string, profileDir string, port int) {
	if port <= 0 {
		port = 9222
	}

	// 1. Try graceful shutdown via DevTools HTTP/WebSocket endpoint if active
	client := &http.Client{Timeout: 1 * time.Second}
	endpoints := []string{
		fmt.Sprintf("http://127.0.0.1:%d", port),
		fmt.Sprintf("http://localhost:%d", port),
	}

	for _, ep := range endpoints {
		resp, err := client.Get(ep + "/json/version")
		if err == nil && resp.StatusCode == http.StatusOK {
			var ver struct {
				WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
			}
			_ = json.NewDecoder(resp.Body).Decode(&ver)
			_ = resp.Body.Close()

			if ver.WebSocketDebuggerURL != "" {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				conn, err := rpcc.DialContext(ctx, ver.WebSocketDebuggerURL)
				if err == nil {
					c := cdp.NewClient(conn)
					_ = c.Browser.Close(ctx)
					_ = conn.Close()
				}
				cancel()
			}
		} else if resp != nil {
			_ = resp.Body.Close()
		}
	}

	// 2. Terminate any lingering debug processes for this port or profile dir across ALL browsers (Chrome, Edge, Brave)
	if runtime.GOOS == "windows" {
		cleanProfile := strings.ReplaceAll(profileDir, `\`, `\\`)
		psCmd := fmt.Sprintf(`Get-NetTCPConnection -LocalPort %d -ErrorAction SilentlyContinue | Select-Object -ExpandProperty OwningProcess -Unique | ForEach-Object { Stop-Process -Id $_ -Force -ErrorAction SilentlyContinue }; Get-CimInstance Win32_Process | Where-Object { ($_.CommandLine -like "*remote-debugging-port=%d*") -or ($_.CommandLine -like "*%s*") } | ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }`, port, port, cleanProfile)
		_ = exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psCmd).Run()
	} else {
		_ = exec.Command("sh", "-c", fmt.Sprintf("lsof -ti :%d | xargs kill -9 2>/dev/null || true", port)).Run()
		_ = exec.Command("pkill", "-f", fmt.Sprintf("remote-debugging-port=%d", port)).Run()
		if profileDir != "" {
			_ = exec.Command("pkill", "-f", profileDir).Run()
		}
	}

	// 3. Remove stale Chromium lock files if present
	if profileDir != "" {
		_ = os.Remove(filepath.Join(profileDir, "lockfile"))
		_ = os.Remove(filepath.Join(profileDir, "SingletonLock"))
		_ = os.Remove(filepath.Join(profileDir, "SingletonCookie"))
		_ = os.Remove(filepath.Join(profileDir, "SingletonSocket"))
	}

	// Allow OS socket & file handles to release
	time.Sleep(500 * time.Millisecond)
}

// Launch starts the browser process with remote debugging enabled.
func Launch(ctx context.Context, cfg LaunchConfig) (*os.Process, error) {
	binaryPath := cfg.BinaryPath
	if binaryPath == "" {
		var err error
		binaryPath, err = FindBinary(cfg.Browser)
		if err != nil {
			return nil, err
		}
	}

	profileDir, err := ResolveProfileDir(cfg.Browser, cfg.ProfileDir)
	if err != nil {
		return nil, err
	}
	cfg.ProfileDir = profileDir

	if cfg.Port <= 0 {
		cfg.Port = 9222
	}

	// Enforce singleton: terminate any stale browser instances on this port or profile
	TerminatePreviousInstances(cfg.Browser, cfg.ProfileDir, cfg.Port)

	args := BuildLaunchArgs(cfg)
	cmd := exec.CommandContext(ctx, binaryPath, args...)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to launch %s (%s): %w", cfg.Browser, binaryPath, err)
	}

	return cmd.Process, nil
}

// WaitForDebugger polls the debugging port until it becomes ready or context expires.
func WaitForDebugger(ctx context.Context, debugURL string, timeout time.Duration) error {
	u, err := url.Parse(debugURL)
	if err != nil {
		return fmt.Errorf("invalid debug URL: %w", err)
	}

	port := u.Port()
	if port == "" {
		port = "9222"
	}

	endpoints := []string{
		fmt.Sprintf("http://%s/json/version", u.Host),
	}
	if strings.HasPrefix(u.Host, "127.0.0.1") {
		endpoints = append(endpoints, fmt.Sprintf("http://localhost:%s/json/version", port))
	}

	client := &http.Client{Timeout: 500 * time.Millisecond}
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		for _, endpoint := range endpoints {
			resp, err := client.Get(endpoint)
			if err == nil && resp.StatusCode == http.StatusOK {
				_ = resp.Body.Close()
				return nil
			}
			if resp != nil {
				_ = resp.Body.Close()
			}
		}

		time.Sleep(200 * time.Millisecond)
	}

	return fmt.Errorf("timed out waiting for browser debugger at %s after %v", debugURL, timeout)
}
