package services

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/nizigama/linux-server-monitor/structs"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupSyncTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	if err := db.AutoMigrate(&structs.Cpu{}, &structs.Memory{}, &structs.Disk{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}
	return db
}

// capturedRequest holds the last request seen by a stub server.
type capturedRequest struct {
	mu     sync.Mutex
	Method string
	Path   string
	Header http.Header
	Body   []byte
}

func (c *capturedRequest) set(r *http.Request, body []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Method = r.Method
	c.Path = r.URL.Path
	c.Header = r.Header.Clone()
	c.Body = body
}

func (c *capturedRequest) get() (method, path string, header http.Header, body []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Method, c.Path, c.Header.Clone(), append([]byte(nil), c.Body...)
}

// stubServer creates an httptest.Server that records the last request and returns the given status.
// body is read and stored so the server can respond quickly.
func stubServer(t *testing.T, statusCode int) (*httptest.Server, *capturedRequest) {
	captured := &capturedRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		captured.set(r, body)
		w.WriteHeader(statusCode)
	}))
	t.Cleanup(server.Close)
	return server, captured
}

func TestSyncOnce_TimeWindowAndPayloadShape(t *testing.T) {
	db := setupSyncTestDB(t)
	now := time.Now()
	// Use a timestamp inside the sync window (start = now-15s, end = now); end is exclusive so use now-5s
	ts := now.Add(-5 * time.Second).Unix()

	tsStr := time.Unix(ts, 0).Format("2006-01-02 15:04:05 MST")
	// Seed DB with one of each type
	err := db.Create(&structs.Cpu{
		Datetime: ts,
		Metrics:  [][]string{{tsStr, "core 0", "25.50"}},
	}).Error
	if err != nil {
		t.Fatalf("create cpu: %v", err)
	}
	err = db.Create(&structs.Memory{
		Datetime: ts,
		Metrics:  [][]string{{tsStr, "45.25"}},
	}).Error
	if err != nil {
		t.Fatalf("create memory: %v", err)
	}
	err = db.Create(&structs.Disk{
		Datetime: ts,
		Metrics:  [][]string{{tsStr, "/dev/sda1", "60%"}},
	}).Error
	if err != nil {
		t.Fatalf("create disk: %v", err)
	}

	server, captured := stubServer(t, http.StatusOK)
	client := server.Client()

	err = SyncOnce(db, client, server.URL+"/api/metrics")
	if err != nil {
		t.Fatalf("SyncOnce: %v", err)
	}

	_, _, _, body := captured.get()
	var metrics []structs.Metrics
	if err := json.Unmarshal(body, &metrics); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if len(metrics) != 3 {
		t.Fatalf("expected 3 metric types, got %d", len(metrics))
	}
	types := map[string]bool{}
	for _, m := range metrics {
		types[m.Type] = true
	}
	if !types["Cpu"] || !types["Memory"] || !types["Disk"] {
		t.Errorf("expected Cpu, Memory, Disk types; got %v", metrics)
	}
	// At least one type should have data (we seeded all three)
	var hasData bool
	for _, m := range metrics {
		if len(m.Data) > 0 {
			hasData = true
			break
		}
	}
	if !hasData {
		t.Error("expected at least one metric type to have Data")
	}
}

func TestSyncOnce_POSTToStubServer(t *testing.T) {
	db := setupSyncTestDB(t)
	server, captured := stubServer(t, http.StatusOK)
	client := server.Client()

	err := SyncOnce(db, client, server.URL+"/api/metrics")
	if err != nil {
		t.Fatalf("SyncOnce: %v", err)
	}

	method, path, header, body := captured.get()
	if method != http.MethodPost {
		t.Errorf("expected method POST, got %s", method)
	}
	if path != "/api/metrics" {
		t.Errorf("expected path /api/metrics, got %s", path)
	}
	if ct := header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}
	var metrics []structs.Metrics
	if err := json.Unmarshal(body, &metrics); err != nil {
		t.Fatalf("body not valid JSON: %v", err)
	}
	if len(metrics) != 3 {
		t.Errorf("expected 3 metric types, got %d", len(metrics))
	}
}

func TestSyncOnce_EmptyDatabase(t *testing.T) {
	db := setupSyncTestDB(t)
	server, captured := stubServer(t, http.StatusOK)
	client := server.Client()

	err := SyncOnce(db, client, server.URL+"/api/metrics")
	if err != nil {
		t.Fatalf("SyncOnce: %v", err)
	}

	_, _, _, body := captured.get()
	var metrics []structs.Metrics
	if err := json.Unmarshal(body, &metrics); err != nil {
		t.Fatalf("body not valid JSON: %v", err)
	}
	if len(metrics) != 3 {
		t.Fatalf("expected 3 metric types, got %d", len(metrics))
	}
	for i, m := range metrics {
		if len(m.Data) != 0 {
			t.Errorf("metric type %s (index %d): expected empty Data, got %d entries", m.Type, i, len(m.Data))
		}
	}
}

func TestSyncOnce_HTTPErrorHandling(t *testing.T) {
	db := setupSyncTestDB(t)
	server, captured := stubServer(t, http.StatusInternalServerError)
	client := server.Client()

	err := SyncOnce(db, client, server.URL+"/api/metrics")
	if err == nil {
		t.Error("expected error when server returns 500")
	}

	method, _, _, _ := captured.get()
	if method != http.MethodPost {
		t.Errorf("expected one POST request, got method %s", method)
	}
}

func TestSyncOnce_ClientTimeout(t *testing.T) {
	db := setupSyncTestDB(t)
	block := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block // block until test unlocks
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	t.Cleanup(func() { close(block) })

	client := &http.Client{Timeout: 50 * time.Millisecond}

	done := make(chan error, 1)
	go func() {
		done <- SyncOnce(db, client, server.URL+"/api/metrics")
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Error("expected timeout error")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("SyncOnce did not return within timeout")
	}
}

func TestSyncOnce_UsesProvidedURL(t *testing.T) {
	db := setupSyncTestDB(t)
	server, captured := stubServer(t, http.StatusOK)
	client := server.Client()
	customPath := "/custom/metrics/endpoint"

	err := SyncOnce(db, client, server.URL+customPath)
	if err != nil {
		t.Fatalf("SyncOnce: %v", err)
	}

	_, path, _, _ := captured.get()
	if path != customPath {
		t.Errorf("expected path %q, got %q", customPath, path)
	}
}

// countingStubServer returns a server that returns 500 for the first failCount
// requests and 200 thereafter. The returned getter returns the total request count.
func countingStubServer(t *testing.T, failCount int) (*httptest.Server, func() int) {
	var state struct {
		mu    sync.Mutex
		count int
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		state.mu.Lock()
		state.count++
		c := state.count
		state.mu.Unlock()
		if c <= failCount {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	getCount := func() int {
		state.mu.Lock()
		defer state.mu.Unlock()
		return state.count
	}
	return server, getCount
}

func TestSyncWithRetry_SuccessOnFirstTry(t *testing.T) {
	db := setupSyncTestDB(t)
	server, getCount := countingStubServer(t, 0) // succeed from first request
	client := server.Client()
	backoffs := []time.Duration{1 * time.Millisecond, 2 * time.Millisecond}

	err := SyncWithRetry(db, client, server.URL+"/api/metrics", 3, backoffs)
	if err != nil {
		t.Fatalf("SyncWithRetry: %v", err)
	}
	if getCount() != 1 {
		t.Errorf("expected 1 request, got %d", getCount())
	}
}

func TestSyncWithRetry_SuccessOnSecondTry(t *testing.T) {
	db := setupSyncTestDB(t)
	server, getCount := countingStubServer(t, 1) // fail once, then succeed
	client := server.Client()
	backoffs := []time.Duration{1 * time.Millisecond, 2 * time.Millisecond}

	err := SyncWithRetry(db, client, server.URL+"/api/metrics", 3, backoffs)
	if err != nil {
		t.Fatalf("SyncWithRetry: %v", err)
	}
	if getCount() != 2 {
		t.Errorf("expected 2 requests, got %d", getCount())
	}
}

func TestSyncWithRetry_SuccessOnThirdTry(t *testing.T) {
	db := setupSyncTestDB(t)
	server, getCount := countingStubServer(t, 2) // fail twice, then succeed
	client := server.Client()
	backoffs := []time.Duration{1 * time.Millisecond, 2 * time.Millisecond}

	err := SyncWithRetry(db, client, server.URL+"/api/metrics", 3, backoffs)
	if err != nil {
		t.Fatalf("SyncWithRetry: %v", err)
	}
	if getCount() != 3 {
		t.Errorf("expected 3 requests, got %d", getCount())
	}
}

func TestSyncWithRetry_AllRetriesExhausted(t *testing.T) {
	db := setupSyncTestDB(t)
	server, getCount := countingStubServer(t, 10) // always fail (500)
	client := server.Client()
	backoffs := []time.Duration{1 * time.Millisecond, 2 * time.Millisecond}

	err := SyncWithRetry(db, client, server.URL+"/api/metrics", 3, backoffs)
	if err == nil {
		t.Fatal("expected error when all retries fail")
	}
	if getCount() != 3 {
		t.Errorf("expected exactly 3 requests (max retries), got %d", getCount())
	}
}

func TestSyncWithRetry_BackoffBetweenAttempts(t *testing.T) {
	db := setupSyncTestDB(t)
	server, getCount := countingStubServer(t, 2) // fail twice, succeed on 3rd
	client := server.Client()
	backoffs := []time.Duration{5 * time.Millisecond, 10 * time.Millisecond}

	start := time.Now()
	err := SyncWithRetry(db, client, server.URL+"/api/metrics", 3, backoffs)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("SyncWithRetry: %v", err)
	}
	if getCount() != 3 {
		t.Errorf("expected 3 requests, got %d", getCount())
	}
	// Should have slept at least 5ms + 10ms between the three attempts
	if elapsed < 14*time.Millisecond {
		t.Errorf("expected elapsed >= 14ms (backoff 5ms+10ms), got %v", elapsed)
	}
}

func TestSyncWithRetry_EmptyBackoffsStillRetries(t *testing.T) {
	db := setupSyncTestDB(t)
	server, getCount := countingStubServer(t, 10) // always 500
	client := server.Client()

	err := SyncWithRetry(db, client, server.URL+"/api/metrics", 3, nil)
	if err == nil {
		t.Fatal("expected error when all retries fail")
	}
	if getCount() != 3 {
		t.Errorf("expected 3 attempts with nil backoffs, got %d", getCount())
	}
}

func TestSyncWithRetry_SingleRetryNoBackoff(t *testing.T) {
	db := setupSyncTestDB(t)
	server, getCount := countingStubServer(t, 1) // fail once then succeed
	client := server.Client()

	// maxRetries=2, empty backoffs: one retry with 0 sleep
	err := SyncWithRetry(db, client, server.URL+"/api/metrics", 2, []time.Duration{})
	if err != nil {
		t.Fatalf("SyncWithRetry: %v", err)
	}
	if getCount() != 2 {
		t.Errorf("expected 2 requests, got %d", getCount())
	}
}
