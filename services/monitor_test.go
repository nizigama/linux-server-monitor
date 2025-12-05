package services

import (
	"bytes"
	"encoding/csv"
	"os/exec"
	"strings"
	"testing"
)

// Helper function to test CPU metrics parsing with sample mpstat output
func testCpuMetricsParsing(t *testing.T, mpstatOutput string, expectedCores int) {
	// Use a here-document to properly handle multiline input
	testScript := `awk -v time="2024-01-01 12:00:00 EST" 'BEGIN {
		idle_col = 0
	}
	/^Linux/ || /^$/ { next }
	/^%?[Cc][Pp][Uu]/ || /CPU/ || /%usr/ || /%sys/ {
		for (i = 1; i <= NF; i++) {
			if ($i ~ /%?[Ii]dle/) {
				idle_col = i
				break
			}
		}
		if (idle_col == 0) {
			idle_col = NF
		}
		next
	}
	/^Average/ && $2 ~ /^[0-9]+$/ {
		idle = $(idle_col)
		gsub(/%/, "", idle)
		if (idle ~ /^[0-9]+\.?[0-9]*$/) {
			cpu_usage = 100 - idle
			if (cpu_usage >= 0 && cpu_usage <= 100) {
				printf "%s,core %s,%.2f\n", time, $2, cpu_usage
			}
		}
	}' <<'EOF'
` + mpstatOutput + `EOF`

	cmd := exec.Command("bash", "-c", testScript)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	err := cmd.Run()
	if err != nil {
		t.Fatalf("Test script failed: %v", err)
	}

	reader := csv.NewReader(&stdout)
	results, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("Failed to parse CSV: %v", err)
	}

	if len(results) != expectedCores {
		t.Errorf("Expected %d CPU cores, got %d", expectedCores, len(results))
	}

	// Validate format: timestamp, core name, CPU usage
	for i, row := range results {
		if len(row) != 3 {
			t.Errorf("Row %d: Expected 3 columns, got %d: %v", i, len(row), row)
		}
		if !strings.HasPrefix(row[1], "core ") {
			t.Errorf("Row %d: Expected core name to start with 'core ', got '%s'", i, row[1])
		}
	}
}

func TestCpuMetricsParsing_StandardFormat(t *testing.T) {
	mpstatOutput := `Linux 5.4.0 (x86_64)  01/01/24  _x86_64_  (4 CPU)

12:00:00     CPU    %usr   %nice    %sys %iowait    %irq   %soft  %steal  %guest  %gnice   %idle
12:00:01     all    5.25    0.00    2.50    0.25    0.00    0.00    0.00    0.00    0.00   91.75
12:00:01       0    5.00    0.00    2.00    0.00    0.00    0.00    0.00    0.00    0.00   93.00
12:00:01       1    5.50    0.00    3.00    0.50    0.00    0.00    0.00    0.00    0.00   91.25
12:00:01       2    5.25    0.00    2.25    0.25    0.00    0.00    0.00    0.00    0.00   92.25
12:00:01       3    5.00    0.00    2.75    0.25    0.00    0.00    0.00    0.00    0.00   91.75

Average:     CPU    %usr   %nice    %sys %iowait    %irq   %soft  %steal  %guest  %gnice   %idle
Average:     all    5.25    0.00    2.50    0.25    0.00    0.00    0.00    0.00    0.00   91.75
Average:       0    5.00    0.00    2.00    0.00    0.00    0.00    0.00    0.00    0.00   93.00
Average:       1    5.50    0.00    3.00    0.50    0.00    0.00    0.00    0.00    0.00   91.25
Average:       2    5.25    0.00    2.25    0.25    0.00    0.00    0.00    0.00    0.00   92.25
Average:       3    5.00    0.00    2.75    0.25    0.00    0.00    0.00    0.00    0.00   91.75
`

	testCpuMetricsParsing(t, mpstatOutput, 4)
}

func TestCpuMetricsParsing_WithoutGuestColumns(t *testing.T) {
	// Some older mpstat versions don't have %guest and %gnice columns
	mpstatOutput := `Linux 4.15.0 (x86_64)  01/01/24  _x86_64_  (2 CPU)

12:00:00     CPU    %usr   %nice    %sys %iowait    %irq   %soft  %steal   %idle
12:00:01     all    3.50    0.00    1.75    0.25    0.00    0.25    0.00   93.75
12:00:01       0    3.25    0.00    1.50    0.00    0.00    0.00    0.00   94.75
12:00:01       1    3.75    0.00    2.00    0.50    0.00    0.50    0.00   92.75

Average:     CPU    %usr   %nice    %sys %iowait    %irq   %soft  %steal   %idle
Average:     all    3.50    0.00    1.75    0.25    0.00    0.25    0.00   93.75
Average:       0    3.25    0.00    1.50    0.00    0.00    0.00    0.00   94.75
Average:       1    3.75    0.00    2.00    0.50    0.00    0.50    0.00   92.75
`

	testCpuMetricsParsing(t, mpstatOutput, 2)
}

func TestCpuMetricsParsing_AlternativeHeaderFormat(t *testing.T) {
	// Some systems use "CPU" instead of "%CPU"
	mpstatOutput := `Linux 5.10.0 (x86_64)  01/01/24  _x86_64_  (8 CPU)

12:00:00  CPU    %usr   %nice    %sys %iowait    %irq   %soft  %steal  %guest  %gnice   %idle
12:00:01  all    4.12    0.00    2.06    0.12    0.00    0.00    0.00    0.00    0.00   93.70
12:00:01    0    4.00    0.00    2.00    0.00    0.00    0.00    0.00    0.00    0.00   94.00
12:00:01    1    4.25    0.00    2.12    0.25    0.00    0.00    0.00    0.00    0.00   93.38

Average:  CPU    %usr   %nice    %sys %iowait    %irq   %soft  %steal  %guest  %gnice   %idle
Average:  all    4.12    0.00    2.06    0.12    0.00    0.00    0.00    0.00    0.00   93.70
Average:    0    4.00    0.00    2.00    0.00    0.00    0.00    0.00    0.00    0.00   94.00
Average:    1    4.25    0.00    2.12    0.25    0.00    0.00    0.00    0.00    0.00   93.38
`

	testCpuMetricsParsing(t, mpstatOutput, 2)
}

func TestCpuMetricsParsing_EmptyOutput(t *testing.T) {
	// Test with empty or invalid output
	mpstatOutput := `Linux 5.4.0 (x86_64)  01/01/24  _x86_64_  (4 CPU)

12:00:00     CPU    %usr   %nice    %sys %iowait    %irq   %soft  %steal  %guest  %gnice   %idle
`

	testCpuMetricsParsing(t, mpstatOutput, 0)
}

func TestCpuMetricsParsing_InvalidIdleValues(t *testing.T) {
	// Test that invalid idle values are filtered out
	mpstatOutput := `Linux 5.4.0 (x86_64)  01/01/24  _x86_64_  (2 CPU)

12:00:00     CPU    %usr   %nice    %sys %iowait    %irq   %soft  %steal  %guest  %gnice   %idle
12:00:01     all    5.25    0.00    2.50    0.25    0.00    0.00    0.00    0.00    0.00   91.75

Average:     CPU    %usr   %nice    %sys %iowait    %irq   %soft  %steal  %guest  %gnice   %idle
Average:     all    5.25    0.00    2.50    0.25    0.00    0.00    0.00    0.00    0.00   91.75
Average:       0    5.00    0.00    2.00    0.00    0.00    0.00    0.00    0.00    0.00   invalid
Average:       1    5.50    0.00    3.00    0.50    0.00    0.00    0.00    0.00    0.00   91.25
`

	// Should only parse valid cores (0 should be skipped due to invalid idle)
	testCpuMetricsParsing(t, mpstatOutput, 1)
}

func TestMemoryMetricsParsing(t *testing.T) {
	// Test memory metrics parsing with sample free output
	freeOutput := `              total        used        free      shared  buff/cache   available
Mem:        8192000     3686400     2048000       51200     2457600     4096000
Swap:       2097152           0     2097152
`

	testScript := `awk '/Mem:/ {printf("%s,%.2f\n", "2024-01-01 12:00:00 EST", $3/$2 * 100.0)}' <<'EOF'
` + freeOutput + `EOF`

	cmd := exec.Command("bash", "-c", testScript)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	err := cmd.Run()
	if err != nil {
		t.Fatalf("Test script failed: %v", err)
	}

	reader := csv.NewReader(&stdout)
	results, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("Failed to parse CSV: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 memory metric, got %d", len(results))
	}

	if len(results[0]) != 2 {
		t.Errorf("Expected 2 columns (timestamp, usage), got %d: %v", len(results[0]), results[0])
	}

	// Verify timestamp format
	if !strings.Contains(results[0][0], "2024-01-01") {
		t.Errorf("Expected timestamp in first column, got '%s'", results[0][0])
	}
}

func TestDiskMetricsParsing(t *testing.T) {
	// Test disk metrics parsing with sample df output
	dfOutput := `Filesystem      Size  Used Avail Use% Mounted on
/dev/sda1        20G   12G  7.0G  60% /
`

	testScript := `awk -v time="2024-01-01 12:00:00 EST" 'NR==2 {print time "," $1 "," $5}' <<'EOF'
` + dfOutput + `EOF`

	cmd := exec.Command("bash", "-c", testScript)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	err := cmd.Run()
	if err != nil {
		t.Fatalf("Test script failed: %v", err)
	}

	reader := csv.NewReader(&stdout)
	results, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("Failed to parse CSV: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 disk metric, got %d", len(results))
	}

	if len(results[0]) != 3 {
		t.Errorf("Expected 3 columns (timestamp, device, usage), got %d: %v", len(results[0]), results[0])
	}

	// Verify format
	if !strings.Contains(results[0][0], "2024-01-01") {
		t.Errorf("Expected timestamp in first column, got '%s'", results[0][0])
	}
	if !strings.HasPrefix(results[0][1], "/dev/") {
		t.Errorf("Expected device path in second column, got '%s'", results[0][1])
	}
}

func TestCpuMetricsParsing_EdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		mpstatOutput  string
		expectedCores int
		description   string
	}{
		{
			name: "Single core system",
			mpstatOutput: `Linux 5.4.0 (x86_64)  01/01/24  _x86_64_  (1 CPU)

12:00:00     CPU    %usr   %nice    %sys %iowait    %irq   %soft  %steal  %guest  %gnice   %idle
Average:       0    5.00    0.00    2.00    0.00    0.00    0.00    0.00    0.00    0.00   93.00
`,
			expectedCores: 1,
			description:   "Should handle single core systems",
		},
		{
			name: "Many cores system",
			mpstatOutput: `Linux 5.4.0 (x86_64)  01/01/24  _x86_64_  (16 CPU)

12:00:00     CPU    %usr   %nice    %sys %iowait    %irq   %soft  %steal  %guest  %gnice   %idle
Average:       0    5.00    0.00    2.00    0.00    0.00    0.00    0.00    0.00    0.00   93.00
Average:       1    5.00    0.00    2.00    0.00    0.00    0.00    0.00    0.00    0.00   93.00
Average:       2    5.00    0.00    2.00    0.00    0.00    0.00    0.00    0.00    0.00   93.00
Average:       3    5.00    0.00    2.00    0.00    0.00    0.00    0.00    0.00    0.00   93.00
Average:       4    5.00    0.00    2.00    0.00    0.00    0.00    0.00    0.00    0.00   93.00
Average:       5    5.00    0.00    2.00    0.00    0.00    0.00    0.00    0.00    0.00   93.00
Average:       6    5.00    0.00    2.00    0.00    0.00    0.00    0.00    0.00    0.00   93.00
Average:       7    5.00    0.00    2.00    0.00    0.00    0.00    0.00    0.00    0.00   93.00
Average:       8    5.00    0.00    2.00    0.00    0.00    0.00    0.00    0.00    0.00   93.00
Average:       9    5.00    0.00    2.00    0.00    0.00    0.00    0.00    0.00    0.00   93.00
Average:      10    5.00    0.00    2.00    0.00    0.00    0.00    0.00    0.00    0.00   93.00
Average:      11    5.00    0.00    2.00    0.00    0.00    0.00    0.00    0.00    0.00   93.00
Average:      12    5.00    0.00    2.00    0.00    0.00    0.00    0.00    0.00    0.00   93.00
Average:      13    5.00    0.00    2.00    0.00    0.00    0.00    0.00    0.00    0.00   93.00
Average:      14    5.00    0.00    2.00    0.00    0.00    0.00    0.00    0.00    0.00   93.00
Average:      15    5.00    0.00    2.00    0.00    0.00    0.00    0.00    0.00    0.00   93.00
`,
			expectedCores: 16,
			description:   "Should handle many core systems",
		},
		{
			name: "CPU usage at boundaries",
			mpstatOutput: `Linux 5.4.0 (x86_64)  01/01/24  _x86_64_  (2 CPU)

12:00:00     CPU    %usr   %nice    %sys %iowait    %irq   %soft  %steal  %guest  %gnice   %idle
Average:       0    0.00    0.00    0.00    0.00    0.00    0.00    0.00    0.00    0.00  100.00
Average:       1  100.00    0.00    0.00    0.00    0.00    0.00    0.00    0.00    0.00    0.00
`,
			expectedCores: 2,
			description:   "Should handle 0% and 100% CPU usage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCpuMetricsParsing(t, tt.mpstatOutput, tt.expectedCores)
		})
	}
}
