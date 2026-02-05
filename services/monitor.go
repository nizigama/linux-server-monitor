package services

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

var lookPath = exec.LookPath
var procStatSleep = time.Sleep
var execCommandContext = exec.CommandContext
var makeLoadCpuMetricsMpstatContext = func() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}

func LoadCpuMetrics() ([][]string, error) {
	if _, err := lookPath("mpstat"); err == nil {
		return loadCpuMetricsMpstat()
	}
	return loadCpuMetricsProcStat()
}

func loadCpuMetricsMpstat() ([][]string, error) {
	awkScript := `
BEGIN {
	time = strftime("%Y-%m-%d %H:%M:%S %Z")
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
		print "ERROR: %idle column not found" > "/dev/stderr"
		exit 1
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
}
`

	ctx, cancel := makeLoadCpuMetricsMpstatContext()
	defer cancel()

	cmd := execCommandContext(ctx, "bash", "-c", "mpstat -P ALL 1 1 | awk '"+awkScript+"'")

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	reader := csv.NewReader(stdout)
	reader.TrimLeadingSpace = true
	res, readErr := reader.ReadAll()
	waitErr := cmd.Wait()

	if stderrBuf.Len() > 0 {
		return nil, fmt.Errorf("mpstat: %s", strings.TrimSpace(stderrBuf.String()))
	}

	if readErr != nil {
		return nil, readErr
	}

	if waitErr != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, errors.New("mpstat timed out")
		}
		return nil, waitErr
	}

	if len(res) == 0 {
		return nil, errors.New("mpstat returned no metrics")
	}

	return res, nil
}

func loadCpuMetricsProcStat() ([][]string, error) {
	return loadCpuMetricsProcStatFromGetter(func() (map[string]cpuStat, error) {
		return readProcStatCores("/proc/stat")
	})
}

func loadCpuMetricsProcStatFromGetter(getCores func() (map[string]cpuStat, error)) ([][]string, error) {
	cores1, err := getCores()
	if err != nil {
		return nil, err
	}

	procStatSleep(1 * time.Second)

	cores2, err := getCores()
	if err != nil {
		return nil, err
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05 MST")
	var results [][]string

	for name, stat1 := range cores1 {
		stat2, ok := cores2[name]
		if !ok {
			continue
		}

		idleDelta := stat2.idle - stat1.idle
		totalDelta := stat2.total - stat1.total

		if totalDelta == 0 {
			continue
		}

		usage := 100.0 * (1.0 - float64(idleDelta)/float64(totalDelta))
		if usage < 0 {
			usage = 0
		}
		if usage > 100 {
			usage = 100
		}

		results = append(results, []string{
			timestamp,
			name,
			fmt.Sprintf("%.2f", usage),
		})
	}

	if len(results) == 0 {
		return nil, errors.New("/proc/stat returned no metrics")
	}

	return results, nil
}

type cpuStat struct {
	idle  uint64
	total uint64
}

func readProcStatCores(statPath string) (map[string]cpuStat, error) {
	if statPath == "" {
		statPath = "/proc/stat"
	}
	f, err := os.Open(statPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cores := make(map[string]cpuStat)
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "cpu") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		name := fields[0]
		if name == "cpu" {
			name = "core all"
		} else {
			// cpu0 -> core 0
			name = "core " + strings.TrimPrefix(name, "cpu")
		}

		// Parse available fields (handles varying kernel versions)
		var values []uint64
		for _, field := range fields[1:] {
			v, err := strconv.ParseUint(field, 10, 64)
			if err != nil {
				break
			}
			values = append(values, v)
		}

		if len(values) < 4 {
			continue
		}

		// user(0) + nice(1) + system(2) + idle(3) + iowait(4) + irq(5) + softirq(6) + steal(7)
		var total uint64
		for _, v := range values {
			total += v
		}

		idle := values[3]
		if len(values) > 4 {
			idle += values[4] // iowait
		}

		cores[name] = cpuStat{idle: idle, total: total}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return cores, nil
}

func LoadMemoryMetrics() ([][]string, error) {

	logger := log.New(os.Stdout, "MEMORY: ", log.LstdFlags)

	cmd := exec.Command("bash", "-c", `free | awk '/Mem:/ {printf("%s,%.2f\n", strftime("%Y-%m-%d %H:%M:%S %Z"), $3/$2 * 100.0)}'`)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.Println(err)
		return nil, errors.New("failed to create stdout pipe")
	}

	if err := cmd.Start(); err != nil {
		logger.Println(err)
		return nil, errors.New("failed to start the command")
	}

	res, err := csv.NewReader(stdout).ReadAll()
	if err != nil {
		logger.Println(err)
		return nil, errors.New("failed to parse csv result")
	}

	defer stdout.Close()

	if err := cmd.Wait(); err != nil {
		logger.Println(err)
		return nil, errors.New("failed to close the command execution")
	}

	return res, nil
}

func LoadDiskMetrics() ([][]string, error) {

	logger := log.New(os.Stdout, "DISK: ", log.LstdFlags)

	cmd := exec.Command("bash", "-c", `df -h --output=source,pcent / | awk -v time="$(date +'%Y-%m-%d %H:%M:%S %Z')" 'NR==2 {print time "," $1 "," $2}'`)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.Println(err)
		return nil, errors.New("failed to create stdout pipe")
	}

	if err := cmd.Start(); err != nil {
		logger.Println(err)
		return nil, errors.New("failed to start the command")
	}

	res, err := csv.NewReader(stdout).ReadAll()
	if err != nil {
		logger.Println(err)
		return nil, errors.New("failed to parse csv result")
	}

	defer stdout.Close()

	if err := cmd.Wait(); err != nil {
		logger.Println(err)
		return nil, errors.New("failed to close the command execution")
	}

	return res, nil
}
