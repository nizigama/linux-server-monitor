package services

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
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

func TestLoadCpuMetrics_WhenMpstatNotAvailable(t *testing.T) {
	savedLookPath := lookPath
	savedSleep := procStatSleep
	defer func() {
		lookPath = savedLookPath
		procStatSleep = savedSleep
	}()
	lookPath = func(string) (string, error) { return "", errors.New("mpstat not found") }
	procStatSleep = func(time.Duration) {} // avoid 1s wait in fallback path

	res, err := LoadCpuMetrics()

	if err != nil {
		// On non-Linux we may get "open /proc/stat: no such file or directory"
		if runtime.GOOS != "linux" {
			t.Skipf("skipping: /proc/stat not available on %s", runtime.GOOS)
		}
		// Ensure we took the fallback path: error must not be from mpstat
		if strings.Contains(err.Error(), "mpstat timed out") || strings.Contains(err.Error(), "mpstat returned no metrics") {
			t.Errorf("expected fallback to /proc/stat; got mpstat-style error: %v", err)
		}
		return
	}

	// Linux with /proc/stat: validate result shape
	for i, row := range res {
		if len(row) != 3 {
			t.Errorf("row %d: expected 3 columns (timestamp, core, usage), got %d: %v", i, len(row), row)
		}
		if len(row) >= 2 && !strings.HasPrefix(row[1], "core ") {
			t.Errorf("row %d: core name should start with 'core ', got %q", i, row[1])
		}
	}
}

func TestReadProcStatCores(t *testing.T) {
	// Sample /proc/stat content: cpu + cpu0 + cpu1, and a line with too few fields to skip
	content := `cpu  100 50 80 200 10 0 5 0 0 0
cpu0 40 20 30 80 5 0 2 0 0 0
cpu1 60 30 50 120 5 0 3 0 0 0
ctxt 12345
btime 1234567890
cpu  1 2 3
`
	f, err := os.CreateTemp("", "procstat-*.txt")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		t.Fatalf("WriteString: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	cores, err := readProcStatCores(f.Name())
	if err != nil {
		t.Fatalf("readProcStatCores: %v", err)
	}

	// Expect "core all", "core 0", "core 1". Line "cpu  1 2 3" has only 3 numeric fields after name, so skipped (need at least 4).
	wantKeys := map[string]bool{"core all": true, "core 0": true, "core 1": true}
	for k := range cores {
		if !wantKeys[k] {
			t.Errorf("unexpected key %q", k)
		}
	}
	for _, k := range []string{"core all", "core 0", "core 1"} {
		if !wantKeys[k] {
			continue
		}
		s, ok := cores[k]
		if !ok {
			t.Errorf("missing key %q", k)
			continue
		}
		if s.total == 0 {
			t.Errorf("%s: total is 0", k)
		}
		// core all: user=100 nice=50 sys=80 idle=200 iowait=10 -> total=445, idle=200+10=210
		if k == "core all" {
			if s.idle != 210 || s.total != 445 {
				t.Errorf("core all: expected idle=210 total=445, got idle=%d total=%d", s.idle, s.total)
			}
		}
	}
}

func TestLoadCpuMetricsProcStatFromGetter(t *testing.T) {
	savedSleep := procStatSleep
	defer func() { procStatSleep = savedSleep }()
	procStatSleep = func(time.Duration) {}

	t.Run("computes usage from two snapshots", func(t *testing.T) {
		callCount := 0
		getCores := func() (map[string]cpuStat, error) {
			callCount++
			if callCount == 1 {
				return map[string]cpuStat{"core 0": {idle: 100, total: 200}}, nil
			}
			// idle 100->150 (+50), total 200->300 (+100) -> usage = 100*(1 - 50/100) = 50%
			return map[string]cpuStat{"core 0": {idle: 150, total: 300}}, nil
		}
		res, err := loadCpuMetricsProcStatFromGetter(getCores)
		if err != nil {
			t.Fatalf("loadCpuMetricsProcStatFromGetter: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 row, got %d", len(res))
		}
		if len(res[0]) != 3 {
			t.Fatalf("expected 3 columns, got %v", res[0])
		}
		if res[0][1] != "core 0" {
			t.Errorf("expected core name 'core 0', got %q", res[0][1])
		}
		if res[0][2] != "50.00" {
			t.Errorf("expected usage 50.00, got %q", res[0][2])
		}
	})

	t.Run("skips when totalDelta is zero", func(t *testing.T) {
		same := map[string]cpuStat{"core 0": {idle: 100, total: 200}}
		getCores := func() (map[string]cpuStat, error) {
			return same, nil
		}
		res, err := loadCpuMetricsProcStatFromGetter(getCores)
		if err == nil {
			t.Fatalf("expected error when all deltas are zero (no metrics), got %d rows", len(res))
		}
		if !strings.Contains(err.Error(), "/proc/stat returned no metrics") {
			t.Errorf("expected '/proc/stat returned no metrics', got %q", err.Error())
		}
	})

	t.Run("returns error when getter fails on first call", func(t *testing.T) {
		getCores := func() (map[string]cpuStat, error) {
			return nil, errors.New("read failed")
		}
		_, err := loadCpuMetricsProcStatFromGetter(getCores)
		if err == nil {
			t.Fatal("expected error from getter")
		}
		if err.Error() != "read failed" {
			t.Errorf("expected 'read failed', got %q", err.Error())
		}
	})

	t.Run("returns error when getter fails on second call", func(t *testing.T) {
		callCount := 0
		getCores := func() (map[string]cpuStat, error) {
			callCount++
			if callCount == 1 {
				return map[string]cpuStat{"core 0": {idle: 100, total: 200}}, nil
			}
			return nil, errors.New("second read failed")
		}
		_, err := loadCpuMetricsProcStatFromGetter(getCores)
		if err == nil {
			t.Fatal("expected error on second getter call")
		}
		if err.Error() != "second read failed" {
			t.Errorf("expected 'second read failed', got %q", err.Error())
		}
	})

	t.Run("returns error when no metrics produced", func(t *testing.T) {
		getCores := func() (map[string]cpuStat, error) {
			return map[string]cpuStat{}, nil
		}
		_, err := loadCpuMetricsProcStatFromGetter(getCores)
		if err == nil {
			t.Fatal("expected error when no metrics")
		}
		if !strings.Contains(err.Error(), "/proc/stat returned no metrics") {
			t.Errorf("expected '/proc/stat returned no metrics', got %q", err.Error())
		}
	})
}

func TestLoadCpuMetricsMpstat_ErrorPaths(t *testing.T) {
	savedExec := execCommandContext
	savedCtx := makeLoadCpuMetricsMpstatContext
	defer func() {
		execCommandContext = savedExec
		makeLoadCpuMetricsMpstatContext = savedCtx
	}()

	t.Run("returns error when command writes to stderr", func(t *testing.T) {
		execCommandContext = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
			return exec.CommandContext(ctx, "sh", "-c", "echo 'mpstat: some error' >&2; exit 0")
		}
		_, err := loadCpuMetricsMpstat()
		if err == nil {
			t.Fatal("expected error when stderr is non-empty")
		}
		if !strings.Contains(err.Error(), "mpstat:") || !strings.Contains(err.Error(), "some error") {
			t.Errorf("expected error to contain stderr content, got %q", err.Error())
		}
	})

	t.Run("returns mpstat timed out when context expires during run", func(t *testing.T) {
		makeLoadCpuMetricsMpstatContext = func() (context.Context, context.CancelFunc) {
			return context.WithTimeout(context.Background(), 10*time.Millisecond)
		}
		execCommandContext = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
			return exec.CommandContext(ctx, "sh", "-c", "sleep 10")
		}
		_, err := loadCpuMetricsMpstat()
		if err == nil {
			t.Fatal("expected error when context expires")
		}
		if err.Error() != "mpstat timed out" {
			t.Errorf("expected 'mpstat timed out', got %q", err.Error())
		}
	})

	t.Run("returns error when stdout is empty", func(t *testing.T) {
		makeLoadCpuMetricsMpstatContext = func() (context.Context, context.CancelFunc) {
			return context.WithTimeout(context.Background(), 5*time.Second)
		}
		execCommandContext = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
			return exec.CommandContext(ctx, "sh", "-c", "exit 0")
		}
		_, err := loadCpuMetricsMpstat()
		if err == nil {
			t.Fatal("expected error when stdout is empty")
		}
		if err.Error() != "mpstat returned no metrics" {
			t.Errorf("expected 'mpstat returned no metrics', got %q", err.Error())
		}
	})
}
