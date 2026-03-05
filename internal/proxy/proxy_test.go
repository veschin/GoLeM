package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// proxy.go tests
// ---------------------------------------------------------------------------

func TestNewProxyDefaultsConcurrency(t *testing.T) {
	tests := []struct {
		name        string
		concurrency int
		want        int
	}{
		{"zero defaults to 1", 0, 1},
		{"negative defaults to 1", -5, 1},
		{"positive kept as-is", 3, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(Config{
				TargetURL:   "http://localhost:9999",
				Concurrency: tt.concurrency,
			})
			if got := cap(p.sem); got != tt.want {
				t.Errorf("semaphore capacity = %d, want %d", got, tt.want)
			}
			if p.cfg.Concurrency != tt.want {
				t.Errorf("cfg.Concurrency = %d, want %d", p.cfg.Concurrency, tt.want)
			}
		})
	}
}

// startTestProxy starts a proxy pointed at the given backend URL on a random
// port and returns the proxy, its base URL, and a cleanup function.
func startTestProxy(t *testing.T, backendURL string, concurrency int) (*Proxy, string, func()) {
	t.Helper()
	p := New(Config{
		TargetURL:   backendURL,
		Concurrency: concurrency,
		Port:        0, // random port
	})

	started := make(chan struct{})
	var startErr error

	go func() {
		defer close(started)
		_, startErr = p.Start()
	}()

	// Wait until the proxy is listening.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if port := p.Port(); port > 0 {
			baseURL := fmt.Sprintf("http://localhost:%d", port)
			return p, baseURL, func() { p.Stop() }
		}
		time.Sleep(10 * time.Millisecond)
	}

	// If we get here, Start failed or listener never bound.
	select {
	case <-started:
		if startErr != nil {
			t.Fatalf("proxy Start failed: %v", startErr)
		}
	default:
	}
	t.Fatal("proxy did not start listening within 3s")
	return nil, "", nil // unreachable
}

func TestProxyStartAndStop(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	p, baseURL, cleanup := startTestProxy(t, backend.URL, 1)
	defer cleanup()

	// Verify /health returns 200.
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("/health status = %d, want 200", resp.StatusCode)
	}

	port := p.Port()

	// Stop the proxy.
	p.Stop()

	// Give the listener a moment to close.
	time.Sleep(50 * time.Millisecond)

	// Verify the port is released — connection should fail.
	client := &http.Client{Timeout: 500 * time.Millisecond}
	_, err = client.Get(fmt.Sprintf("http://localhost:%d/health", port))
	if err == nil {
		t.Error("expected connection error after Stop, but request succeeded")
	}
}

func TestProxyHealthEndpointReturnsJSON(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	_, baseURL, cleanup := startTestProxy(t, backend.URL, 2)
	defer cleanup()

	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}

	// Check required fields.
	if status, ok := body["status"].(string); !ok || status != "ok" {
		t.Errorf("status = %v, want \"ok\"", body["status"])
	}
	for _, key := range []string{"port", "active", "queued"} {
		if _, ok := body[key]; !ok {
			t.Errorf("missing key %q in health response", key)
		}
	}
	// Port should match actual port (non-zero).
	if portVal, ok := body["port"].(float64); !ok || portVal <= 0 {
		t.Errorf("port = %v, want >0 number", body["port"])
	}
}

func TestProxyConcurrencySemaphore(t *testing.T) {
	// Track how many requests are concurrently active in the backend.
	var active int64
	var maxActive int64

	gate := make(chan struct{}) // keeps backend requests blocked until we release

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cur := atomic.AddInt64(&active, 1)
		defer atomic.AddInt64(&active, -1)

		// Track the maximum concurrency observed.
		for {
			old := atomic.LoadInt64(&maxActive)
			if cur <= old || atomic.CompareAndSwapInt64(&maxActive, old, cur) {
				break
			}
		}

		<-gate // wait until test releases
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	_, baseURL, cleanup := startTestProxy(t, backend.URL, 1) // concurrency = 1
	defer cleanup()

	// Send 2 concurrent requests through the proxy.
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := http.Get(baseURL + "/test")
			if err != nil {
				return // proxy may have stopped
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}()
	}

	// Wait a bit for both requests to arrive at the proxy.
	time.Sleep(200 * time.Millisecond)

	// With concurrency=1, only one should be active in the backend.
	cur := atomic.LoadInt64(&active)
	if cur != 1 {
		t.Errorf("active backend requests = %d, want 1 (concurrency=1)", cur)
	}

	// Also verify via /health that active=1.
	resp, err := http.Get(baseURL + "/health")
	if err == nil {
		var body map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&body)
		resp.Body.Close()
		if a, ok := body["active"].(float64); ok && a != 1 {
			t.Errorf("/health active = %v, want 1", a)
		}
	}

	// Release both requests.
	close(gate)
	wg.Wait()

	// Max active should have been exactly 1.
	if m := atomic.LoadInt64(&maxActive); m != 1 {
		t.Errorf("max concurrent backend requests = %d, want 1", m)
	}
}

func TestJoinPaths(t *testing.T) {
	tests := []struct {
		base, suffix, want string
	}{
		{"", "/foo", "/foo"},
		{"/api", "/v1", "/api/v1"},
		{"/api/", "/v1", "/api/v1"},
		{"/api/", "v1", "/api/v1"},
		{"/api", "", "/api"},
		{"", "", ""},
		{"/a/b/", "/c/d", "/a/b/c/d"},
	}
	for _, tt := range tests {
		name := fmt.Sprintf("joinPaths(%q,%q)", tt.base, tt.suffix)
		t.Run(name, func(t *testing.T) {
			if got := joinPaths(tt.base, tt.suffix); got != tt.want {
				t.Errorf("joinPaths(%q, %q) = %q, want %q", tt.base, tt.suffix, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// lifecycle.go tests
// ---------------------------------------------------------------------------

func TestWriteAndReadPIDPort(t *testing.T) {
	dir := t.TempDir()

	wantPID := 12345
	wantPort := 54321

	if err := WritePIDFile(dir, wantPID, wantPort); err != nil {
		t.Fatalf("WritePIDFile: %v", err)
	}

	gotPID, gotPort, err := readPIDPort(dir)
	if err != nil {
		t.Fatalf("readPIDPort: %v", err)
	}
	if gotPID != wantPID {
		t.Errorf("pid = %d, want %d", gotPID, wantPID)
	}
	if gotPort != wantPort {
		t.Errorf("port = %d, want %d", gotPort, wantPort)
	}
}

func TestCleanPIDFile(t *testing.T) {
	dir := t.TempDir()

	// Write PID and port files.
	if err := WritePIDFile(dir, 1, 2); err != nil {
		t.Fatalf("WritePIDFile: %v", err)
	}

	// Verify files exist.
	for _, name := range []string{pidFile, portFile} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("expected %s to exist: %v", name, err)
		}
	}

	// Clean up.
	if err := CleanPIDFile(dir); err != nil {
		t.Fatalf("CleanPIDFile: %v", err)
	}

	// Verify files are removed.
	for _, name := range []string{pidFile, portFile} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Errorf("expected %s to be removed, got err: %v", name, err)
		}
	}
}

func TestCleanPIDFileIdempotent(t *testing.T) {
	dir := t.TempDir()
	// Calling CleanPIDFile on a directory with no PID files should not error.
	if err := CleanPIDFile(dir); err != nil {
		t.Fatalf("CleanPIDFile on empty dir: %v", err)
	}
}

func TestIsRunningReturnsFalseWhenNoFiles(t *testing.T) {
	dir := t.TempDir()
	port, alive := IsRunning(dir)
	if alive {
		t.Error("IsRunning returned true for empty directory")
	}
	if port != 0 {
		t.Errorf("port = %d, want 0", port)
	}
}

func TestWriteAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	content := "hello, world"
	if err := writeAtomic(path, content); err != nil {
		t.Fatalf("writeAtomic: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != content {
		t.Errorf("content = %q, want %q", string(got), content)
	}

	// Overwrite with new content — should replace atomically.
	content2 := "updated content"
	if err := writeAtomic(path, content2); err != nil {
		t.Fatalf("writeAtomic (overwrite): %v", err)
	}
	got, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile after overwrite: %v", err)
	}
	if string(got) != content2 {
		t.Errorf("content after overwrite = %q, want %q", string(got), content2)
	}

	// No temp files should be left behind.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if e.Name() != "test.txt" {
			t.Errorf("unexpected leftover file: %s", e.Name())
		}
	}
}

func TestWriteAtomicBadDir(t *testing.T) {
	// Writing to a non-existent directory should fail.
	err := writeAtomic("/nonexistent-dir-12345/file.txt", "data")
	if err == nil {
		t.Error("expected error writing to non-existent directory")
	}
}

func TestReadPIDPortParseErrors(t *testing.T) {
	dir := t.TempDir()

	// Write non-numeric PID.
	os.WriteFile(filepath.Join(dir, pidFile), []byte("notanumber"), 0o644)
	os.WriteFile(filepath.Join(dir, portFile), []byte("8080"), 0o644)

	_, _, err := readPIDPort(dir)
	if err == nil {
		t.Error("expected error for non-numeric PID")
	}

	// Write valid PID but non-numeric port.
	os.WriteFile(filepath.Join(dir, pidFile), []byte(strconv.Itoa(os.Getpid())), 0o644)
	os.WriteFile(filepath.Join(dir, portFile), []byte("notaport"), 0o644)

	_, _, err = readPIDPort(dir)
	if err == nil {
		t.Error("expected error for non-numeric port")
	}
}
