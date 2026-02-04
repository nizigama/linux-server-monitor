package services

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"gorm.io/gorm"
)

const (
	defaultSyncURL       = "https://kubana.cloud/api/metrics"
	defaultSyncSeconds   = 10
	syncWindowSeconds    = 15
	httpClientTimeout    = 10 * time.Second
	syncMaxRetries       = 3
	syncBackoffCondensed = 3 // log condensed message every N consecutive failures
)

// SyncOnce fetches recent metrics from the database, marshals them to JSON,
// and POSTs to the given URL using the provided client. Used by the periodic
// sync loop and by tests (with an injectable client and URL).
func SyncOnce(db *gorm.DB, client *http.Client, url string) error {
	if client == nil {
		client = &http.Client{Timeout: httpClientTimeout}
	}

	end := time.Now()
	start := end.Add(-syncWindowSeconds * time.Second)
	startStr := start.Format(time.DateTime)
	endStr := end.Format(time.DateTime)

	metrics, err := GetMetrics(db, startStr, endStr)
	if err != nil {
		return err
	}

	body, err := json.Marshal(metrics)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &syncHTTPError{statusCode: resp.StatusCode}
	}
	return nil
}

type syncHTTPError struct {
	statusCode int
}

func (e *syncHTTPError) Error() string {
	return "metrics sync HTTP error: status " + strconv.Itoa(e.statusCode)
}

func getSyncConfig() (url string, interval time.Duration) {
	url = os.Getenv("METRICS_SYNC_URL")
	if url == "" {
		url = defaultSyncURL
	}

	sec := defaultSyncSeconds
	if s := os.Getenv("METRICS_SYNC_INTERVAL_SECONDS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			sec = n
		}
	}
	interval = time.Duration(sec) * time.Second
	return url, interval
}

// SyncWithRetry runs SyncOnce up to maxRetries times, sleeping after each failure
// using the given backoff durations (index 0 after first failure, index 1 after
// second, etc.; if there are fewer backoffs than retries, the last backoff is
// reused). Returns the last error if all attempts fail; returns nil on success.
// Used by SyncMetrics and by tests.
func SyncWithRetry(db *gorm.DB, client *http.Client, url string, maxRetries int, backoffs []time.Duration) error {
	var lastErr error
	try := 0
	for try < maxRetries {
		lastErr = SyncOnce(db, client, url)
		if lastErr == nil {
			return nil
		}
		try++
		if try < maxRetries {
			backoff := time.Duration(0)
			if len(backoffs) > 0 {
				idx := try - 1
				if idx >= len(backoffs) {
					idx = len(backoffs) - 1
				}
				backoff = backoffs[idx]
			}
			time.Sleep(backoff)
		}
	}
	return lastErr
}

// SyncMetrics runs a loop that every METRICS_SYNC_INTERVAL_SECONDS (default 10)
// fetches recent metrics and POSTs them to METRICS_SYNC_URL. On connection or
// 5xx errors it retries with exponential backoff (1s, 2s) up to syncMaxRetries
// times, then logs and continues. After syncBackoffCondensed consecutive
// failures it logs a single condensed message to avoid log spam.
func SyncMetrics(db *gorm.DB) {
	logger := log.New(os.Stdout, "SYNC METRICS: ", log.LstdFlags)
	url, interval := getSyncConfig()

	client := &http.Client{Timeout: httpClientTimeout}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	backoffs := []time.Duration{1 * time.Second, 2 * time.Second}
	consecutiveFailures := 0

	for range ticker.C {
		err := SyncWithRetry(db, client, url, syncMaxRetries, backoffs)
		if err != nil {
			consecutiveFailures++
			if consecutiveFailures >= syncBackoffCondensed {
				logger.Printf("sync repeatedly failing (%d consecutive failures): %v", consecutiveFailures, err)
				consecutiveFailures = 0
			} else {
				logger.Printf("sync failed after %d attempt(s): %v", syncMaxRetries, err)
			}
		} else {
			consecutiveFailures = 0
		}
	}
}
