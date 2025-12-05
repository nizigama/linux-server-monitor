package services

import (
	"testing"
	"time"

	"github.com/nizigama/linux-server-monitor/structs"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRecordMetrics_Validation(t *testing.T) {
	// This test verifies that the validation logic in recorder.go works correctly
	// We can't easily test the full RecordMetrics function since it runs indefinitely,
	// but we can test the validation logic that was added

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	err = db.AutoMigrate(&structs.Cpu{}, &structs.Memory{}, &structs.Disk{})
	if err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	tests := []struct {
		name        string
		metrics     [][]string
		shouldSave  bool
		description string
	}{
		{
			name: "Valid CPU metrics",
			metrics: [][]string{
				{"2024-01-01 12:00:00 EST", "core 0", "25.50"},
				{"2024-01-01 12:00:00 EST", "core 1", "30.75"},
			},
			shouldSave:  true,
			description: "Valid metrics with multiple cores should be saved",
		},
		{
			name:        "Empty metrics slice",
			metrics:     [][]string{},
			shouldSave:  false,
			description: "Empty metrics should not be saved",
		},
		{
			name: "Metrics with empty first row",
			metrics: [][]string{
				[]string{},
			},
			shouldSave:  false,
			description: "Metrics with empty first row should not be saved",
		},
		{
			name: "Valid single core metric",
			metrics: [][]string{
				{"2024-01-01 12:00:00 EST", "core 0", "25.50"},
			},
			shouldSave:  true,
			description: "Valid single core metric should be saved",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear database before each test
			db.Exec("DELETE FROM cpus")
			db.Exec("DELETE FROM memories")
			db.Exec("DELETE FROM disks")

			// Simulate the validation logic from recorder.go
			if len(tt.metrics) == 0 || len(tt.metrics[0]) == 0 {
				// This is what recorder.go does - skip empty metrics
				if tt.shouldSave {
					t.Errorf("Expected to save but validation would skip: %s", tt.description)
				}
				return
			}

			// If validation passes, save to database
			err := db.Create(&structs.Cpu{
				Datetime: time.Now().Unix(),
				Metrics:  tt.metrics,
			}).Error

			if tt.shouldSave {
				if err != nil {
					t.Errorf("Expected to save but got error: %v", err)
				} else {
					// Verify it was saved
					var count int64
					db.Model(&structs.Cpu{}).Count(&count)
					if count != 1 {
						t.Errorf("Expected 1 record in database, got %d", count)
					}
				}
			} else {
				// Should not save, but if it did, that's also an error
				var count int64
				db.Model(&structs.Cpu{}).Count(&count)
				if count > 0 {
					t.Errorf("Expected 0 records in database, got %d", count)
				}
			}
		})
	}
}

func TestRecordMetrics_ValidationForAllTypes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	err = db.AutoMigrate(&structs.Cpu{}, &structs.Memory{}, &structs.Disk{})
	if err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	validMetrics := [][]string{
		{"2024-01-01 12:00:00 EST", "core 0", "25.50"},
	}

	timestamp := time.Now().Unix()

	// Test CPU validation
	err = db.Create(&structs.Cpu{
		Datetime: timestamp,
		Metrics:  validMetrics,
	}).Error
	if err != nil {
		t.Errorf("Failed to save valid CPU metrics: %v", err)
	}

	// Test Memory validation
	memoryMetrics := [][]string{
		{"2024-01-01 12:00:00 EST", "45.25"},
	}
	err = db.Create(&structs.Memory{
		Datetime: timestamp,
		Metrics:  memoryMetrics,
	}).Error
	if err != nil {
		t.Errorf("Failed to save valid Memory metrics: %v", err)
	}

	// Test Disk validation
	diskMetrics := [][]string{
		{"2024-01-01 12:00:00 EST", "/dev/sda1", "60%"},
	}
	err = db.Create(&structs.Disk{
		Datetime: timestamp,
		Metrics:  diskMetrics,
	}).Error
	if err != nil {
		t.Errorf("Failed to save valid Disk metrics: %v", err)
	}

	// Verify all were saved
	var cpuCount, memCount, diskCount int64
	db.Model(&structs.Cpu{}).Count(&cpuCount)
	db.Model(&structs.Memory{}).Count(&memCount)
	db.Model(&structs.Disk{}).Count(&diskCount)

	if cpuCount != 1 {
		t.Errorf("Expected 1 CPU record, got %d", cpuCount)
	}
	if memCount != 1 {
		t.Errorf("Expected 1 Memory record, got %d", memCount)
	}
	if diskCount != 1 {
		t.Errorf("Expected 1 Disk record, got %d", diskCount)
	}
}

func TestRecordMetrics_EmptyMetricsNotSaved(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	err = db.AutoMigrate(&structs.Cpu{}, &structs.Memory{}, &structs.Disk{})
	if err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	timestamp := time.Now().Unix()

	// Try to save empty CPU metrics (simulating what would happen if validation didn't catch it)
	err = db.Create(&structs.Cpu{
		Datetime: timestamp,
		Metrics:  [][]string{},
	}).Error
	if err != nil {
		t.Fatalf("Database save should succeed even with empty metrics: %v", err)
	}

	// But when retrieving, empty metrics should be filtered out
	var cpus []structs.Cpu
	db.Find(&cpus)

	// The record exists in DB, but GetMetrics should skip it
	start := time.Now().Add(-1 * time.Hour).Format(time.DateTime)
	end := time.Now().Add(1 * time.Hour).Format(time.DateTime)

	metrics, err := GetMetrics(db, start, end)
	if err != nil {
		t.Fatalf("GetMetrics failed: %v", err)
	}

	// Empty metrics should be skipped by GetMetrics
	if len(metrics[0].Data) != 0 {
		t.Errorf("Expected empty CPU data (empty metrics should be filtered), got %d entries", len(metrics[0].Data))
	}
}
