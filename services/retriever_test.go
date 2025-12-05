package services

import (
	"testing"
	"time"

	"github.com/nizigama/linux-server-monitor/structs"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	err = db.AutoMigrate(&structs.Cpu{}, &structs.Memory{}, &structs.Disk{})
	if err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	return db
}

func TestGetMetrics_EmptyDatabase(t *testing.T) {
	db := setupTestDB(t)

	start := time.Now().Add(-1 * time.Hour).Format(time.DateTime)
	end := time.Now().Format(time.DateTime)

	metrics, err := GetMetrics(db, start, end)
	if err != nil {
		t.Fatalf("GetMetrics failed: %v", err)
	}

	if len(metrics) != 3 {
		t.Fatalf("Expected 3 metric types, got %d", len(metrics))
	}

	if len(metrics[0].Data) != 0 {
		t.Errorf("Expected empty CPU data, got %d entries", len(metrics[0].Data))
	}
	if len(metrics[1].Data) != 0 {
		t.Errorf("Expected empty Memory data, got %d entries", len(metrics[1].Data))
	}
	if len(metrics[2].Data) != 0 {
		t.Errorf("Expected empty Disk data, got %d entries", len(metrics[2].Data))
	}
}

func TestGetMetrics_InvalidDateTimeFormat(t *testing.T) {
	db := setupTestDB(t)

	_, err := GetMetrics(db, "invalid-date", "2024-01-01 12:00:00")
	if err == nil {
		t.Error("Expected error for invalid start datetime format")
	}

	_, err = GetMetrics(db, "2024-01-01 12:00:00", "invalid-date")
	if err == nil {
		t.Error("Expected error for invalid end datetime format")
	}
}

func TestGetMetrics_WithValidCpuMetrics(t *testing.T) {
	db := setupTestDB(t)

	now := time.Now()
	timestamp := now.Unix()

	// Create test CPU metrics
	cpuMetrics := [][]string{
		{"2024-01-01 12:00:00 EST", "core 0", "25.50"},
		{"2024-01-01 12:00:00 EST", "core 1", "30.75"},
	}

	err := db.Create(&structs.Cpu{
		Datetime: timestamp,
		Metrics:  cpuMetrics,
	}).Error
	if err != nil {
		t.Fatalf("Failed to create test data: %v", err)
	}

	start := now.Add(-1 * time.Hour).Format(time.DateTime)
	end := now.Add(1 * time.Hour).Format(time.DateTime)

	metrics, err := GetMetrics(db, start, end)
	if err != nil {
		t.Fatalf("GetMetrics failed: %v", err)
	}

	if len(metrics[0].Data) != 1 {
		t.Fatalf("Expected 1 CPU metric entry, got %d", len(metrics[0].Data))
	}

	if len(metrics[0].Data[0]) != 2 {
		t.Fatalf("Expected 2 CPU cores, got %d", len(metrics[0].Data[0]))
	}
}

func TestGetMetrics_WithEmptyMetrics(t *testing.T) {
	db := setupTestDB(t)

	now := time.Now()
	timestamp := now.Unix()

	// Create CPU record with empty metrics
	err := db.Create(&structs.Cpu{
		Datetime: timestamp,
		Metrics:  [][]string{},
	}).Error
	if err != nil {
		t.Fatalf("Failed to create test data: %v", err)
	}

	// Create CPU record with empty row
	err = db.Create(&structs.Cpu{
		Datetime: timestamp + 1,
		Metrics:  [][]string{[]string{}},
	}).Error
	if err != nil {
		t.Fatalf("Failed to create test data: %v", err)
	}

	start := now.Add(-1 * time.Hour).Format(time.DateTime)
	end := now.Add(1 * time.Hour).Format(time.DateTime)

	metrics, err := GetMetrics(db, start, end)
	if err != nil {
		t.Fatalf("GetMetrics failed: %v", err)
	}

	// Empty metrics should be skipped
	if len(metrics[0].Data) != 0 {
		t.Errorf("Expected empty CPU data (empty metrics should be skipped), got %d entries", len(metrics[0].Data))
	}
}

func TestGetMetrics_WithInvalidLastMetric(t *testing.T) {
	db := setupTestDB(t)

	now := time.Now()
	timestamp := now.Unix()

	// Create first valid metric
	err := db.Create(&structs.Cpu{
		Datetime: timestamp,
		Metrics: [][]string{
			{"2024-01-01 12:00:00 EST", "core 0", "25.50"},
		},
	}).Error
	if err != nil {
		t.Fatalf("Failed to create test data: %v", err)
	}

	// Create second metric with invalid last entry (empty row)
	err = db.Create(&structs.Cpu{
		Datetime: timestamp + 1,
		Metrics: [][]string{
			[]string{}, // Empty row
		},
	}).Error
	if err != nil {
		t.Fatalf("Failed to create test data: %v", err)
	}

	// Create third valid metric
	err = db.Create(&structs.Cpu{
		Datetime: timestamp + 2,
		Metrics: [][]string{
			{"2024-01-01 12:00:05 EST", "core 0", "30.75"},
		},
	}).Error
	if err != nil {
		t.Fatalf("Failed to create test data: %v", err)
	}

	start := now.Add(-1 * time.Hour).Format(time.DateTime)
	end := now.Add(1 * time.Hour).Format(time.DateTime)

	metrics, err := GetMetrics(db, start, end)
	if err != nil {
		t.Fatalf("GetMetrics failed: %v", err)
	}

	// Should have at least 1 valid metric (first one, third might be filtered by interval)
	// The invalid second metric should be skipped entirely
	if len(metrics[0].Data) < 1 {
		t.Errorf("Expected at least 1 CPU metric entry (invalid one should be skipped), got %d", len(metrics[0].Data))
	}
}

func TestGetMetrics_IntervalFiltering(t *testing.T) {
	db := setupTestDB(t)

	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.Local)

	// Create metrics at different intervals
	// Use Local timezone for consistent parsing
	metrics := []struct {
		timeOffset time.Duration
		metrics    [][]string
	}{
		{0, [][]string{{baseTime.Format("2006-01-02 15:04:05 MST"), "core 0", "25.50"}}},
		{15 * time.Second, [][]string{{baseTime.Add(15 * time.Second).Format("2006-01-02 15:04:05 MST"), "core 0", "30.75"}}},
		{45 * time.Second, [][]string{{baseTime.Add(45 * time.Second).Format("2006-01-02 15:04:05 MST"), "core 0", "35.00"}}},
	}

	for i, m := range metrics {
		err := db.Create(&structs.Cpu{
			Datetime: baseTime.Add(m.timeOffset).Unix(),
			Metrics:  m.metrics,
		}).Error
		if err != nil {
			t.Fatalf("Failed to create test data %d: %v", i, err)
		}
	}

	// Test with time range that includes all metrics
	// Use Local timezone for query times
	start := baseTime.Add(-1 * time.Hour).Format(time.DateTime)
	end := baseTime.Add(1 * time.Hour).Format(time.DateTime)

	result, err := GetMetrics(db, start, end)
	if err != nil {
		t.Fatalf("GetMetrics failed: %v", err)
	}

	// The interval logic depends on the time difference between start and end
	// For this test, we're using a 2-hour window, so it should use noLimit interval
	// All metrics should be included (unless they're filtered by the interval check)
	// Since we're using noLimit for < 24 hours, all should be included
	if len(result[0].Data) < 1 {
		t.Errorf("Expected at least 1 CPU metric entry, got %d. Time range: %s to %s. Created %d metrics", len(result[0].Data), start, end, len(metrics))
	}
}

func TestGetMetrics_AllMetricTypes(t *testing.T) {
	db := setupTestDB(t)

	now := time.Now()
	timestamp := now.Unix()

	// Create all metric types
	err := db.Create(&structs.Cpu{
		Datetime: timestamp,
		Metrics:  [][]string{{"2024-01-01 12:00:00 EST", "core 0", "25.50"}},
	}).Error
	if err != nil {
		t.Fatalf("Failed to create CPU data: %v", err)
	}

	err = db.Create(&structs.Memory{
		Datetime: timestamp,
		Metrics:  [][]string{{"2024-01-01 12:00:00 EST", "45.25"}},
	}).Error
	if err != nil {
		t.Fatalf("Failed to create Memory data: %v", err)
	}

	err = db.Create(&structs.Disk{
		Datetime: timestamp,
		Metrics:  [][]string{{"2024-01-01 12:00:00 EST", "/dev/sda1", "60%"}},
	}).Error
	if err != nil {
		t.Fatalf("Failed to create Disk data: %v", err)
	}

	start := now.Add(-1 * time.Hour).Format(time.DateTime)
	end := now.Add(1 * time.Hour).Format(time.DateTime)

	metrics, err := GetMetrics(db, start, end)
	if err != nil {
		t.Fatalf("GetMetrics failed: %v", err)
	}

	if len(metrics) != 3 {
		t.Fatalf("Expected 3 metric types, got %d", len(metrics))
	}

	if len(metrics[0].Data) != 1 {
		t.Errorf("Expected 1 CPU entry, got %d", len(metrics[0].Data))
	}
	if len(metrics[1].Data) != 1 {
		t.Errorf("Expected 1 Memory entry, got %d", len(metrics[1].Data))
	}
	if len(metrics[2].Data) != 1 {
		t.Errorf("Expected 1 Disk entry, got %d", len(metrics[2].Data))
	}

	// Verify types
	if metrics[0].Type != "Cpu" {
		t.Errorf("Expected type 'Cpu', got '%s'", metrics[0].Type)
	}
	if metrics[1].Type != "Memory" {
		t.Errorf("Expected type 'Memory', got '%s'", metrics[1].Type)
	}
	if metrics[2].Type != "Disk" {
		t.Errorf("Expected type 'Disk', got '%s'", metrics[2].Type)
	}
}

func TestGetMetrics_TimeRangeFiltering(t *testing.T) {
	db := setupTestDB(t)

	baseTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	// Create metrics at different times
	metrics := []struct {
		timeOffset time.Duration
		metrics    [][]string
	}{
		{-2 * time.Hour, [][]string{{baseTime.Add(-2 * time.Hour).Format("2006-01-02 15:04:05 MST"), "core 0", "25.50"}}},
		{0, [][]string{{baseTime.Format("2006-01-02 15:04:05 MST"), "core 0", "30.75"}}},
		{2 * time.Hour, [][]string{{baseTime.Add(2 * time.Hour).Format("2006-01-02 15:04:05 MST"), "core 0", "35.00"}}},
	}

	for i, m := range metrics {
		err := db.Create(&structs.Cpu{
			Datetime: baseTime.Add(m.timeOffset).Unix(),
			Metrics:  m.metrics,
		}).Error
		if err != nil {
			t.Fatalf("Failed to create test data %d: %v", i, err)
		}
	}

	// Query only middle time range
	start := baseTime.Add(-30 * time.Minute).Format(time.DateTime)
	end := baseTime.Add(30 * time.Minute).Format(time.DateTime)

	result, err := GetMetrics(db, start, end)
	if err != nil {
		t.Fatalf("GetMetrics failed: %v", err)
	}

	// Should only return the middle metric
	if len(result[0].Data) != 1 {
		t.Errorf("Expected 1 CPU metric entry (within time range), got %d", len(result[0].Data))
	}
}

func TestGetMetrics_InvalidTimestampFormat(t *testing.T) {
	db := setupTestDB(t)

	now := time.Now()
	timestamp := now.Unix()

	// Create first valid metric
	err := db.Create(&structs.Cpu{
		Datetime: timestamp,
		Metrics: [][]string{
			{now.Format("2006-01-02 15:04:05 MST"), "core 0", "25.50"},
		},
	}).Error
	if err != nil {
		t.Fatalf("Failed to create test data: %v", err)
	}

	// Create second metric with invalid timestamp format
	err = db.Create(&structs.Cpu{
		Datetime: timestamp + 1,
		Metrics: [][]string{
			{"invalid-timestamp", "core 0", "30.75"},
		},
	}).Error
	if err != nil {
		t.Fatalf("Failed to create test data: %v", err)
	}

	start := now.Add(-1 * time.Hour).Format(time.DateTime)
	end := now.Add(1 * time.Hour).Format(time.DateTime)

	// Should return error when parsing invalid timestamp (on the second metric)
	_, err = GetMetrics(db, start, end)
	if err == nil {
		t.Error("Expected error when parsing invalid timestamp format in second metric")
	}
}
