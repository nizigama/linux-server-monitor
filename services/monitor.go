package services

import (
	"encoding/csv"
	"errors"
	"log"
	"os"
	"os/exec"
)

func LoadCpuMetrics() ([][]string, error) {

	logger := log.New(os.Stdout, "CPU: ", log.LstdFlags)

	// More robust awk script that:
	// 1. Finds the %idle column dynamically from the header
	// 2. Handles different mpstat versions and column positions
	// 3. Falls back to last numeric column if %idle not found
	// 4. Validates data before output
	awkScript := `BEGIN {
		idle_col = 0
	}
	/^Linux/ || /^$/ { next }
	/^%?[Cc][Pp][Uu]/ || /CPU/ || /%usr/ || /%sys/ {
		# Find the %idle column in the header
		for (i = 1; i <= NF; i++) {
			if ($i ~ /%?[Ii]dle/) {
				idle_col = i
				break
			}
		}
		# If not found, use last column as fallback (most mpstat versions have %idle as last)
		if (idle_col == 0) {
			idle_col = NF
		}
		next
	}
	/^Average/ && $2 ~ /^[0-9]+$/ {
		# Get idle value from the identified column
		idle = $(idle_col)
		# Remove % sign if present
		gsub(/%/, "", idle)
		# Validate idle is a number
		if (idle ~ /^[0-9]+\.?[0-9]*$/) {
			cpu_usage = 100 - idle
			# Ensure CPU usage is within valid range
			if (cpu_usage >= 0 && cpu_usage <= 100) {
				printf "%s,core %s,%.2f\n", time, $2, cpu_usage
			}
		}
	}`

	cmd := exec.Command("bash", "-c", `mpstat -P ALL 1 1 | awk -v time="$(date +'%Y-%m-%d %H:%M:%S %Z')" '`+awkScript+`'`)

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
