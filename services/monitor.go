package services

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"
)

func LoadCpuMetrics() ([][]string, error) {
	logger := log.New(os.Stdout, "CPU: ", log.LstdFlags)

	// Verify mpstat is available
	if _, err := exec.LookPath("mpstat"); err != nil {
		return nil, errors.New("mpstat not found - install sysstat package (apt/dnf/zypper install sysstat)")
	}

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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", "mpstat -P ALL 1 1 | awk '"+awkScript+"'")

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.Println(err)
		return nil, errors.New("failed to create stdout pipe")
	}

	if err := cmd.Start(); err != nil {
		logger.Println(err)
		return nil, errors.New("failed to start cpu metrics command")
	}

	reader := csv.NewReader(stdout)
	reader.TrimLeadingSpace = true
	res, readErr := reader.ReadAll()

	waitErr := cmd.Wait()

	if stderrBuf.Len() > 0 {
		errMsg := stderrBuf.String()
		logger.Println(errMsg)
		return nil, fmt.Errorf("cpu metrics error: %s", errMsg)
	}

	if readErr != nil {
		logger.Println(readErr)
		return nil, errors.New("failed to parse cpu metrics csv")
	}

	if waitErr != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, errors.New("cpu metrics command timed out")
		}
		logger.Println(waitErr)
		return nil, errors.New("cpu metrics command failed")
	}

	if len(res) == 0 {
		return nil, errors.New("no cpu metrics captured")
	}

	return res, nil
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
