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

	cmd := exec.Command("bash", "-c", `mpstat -P ALL 1 1 | awk -v time="$(date +'%Y-%m-%d %H:%M:%S')" '/^Average/ && $2 ~ /[0-9]/ {printf "%s,core %s,%.2f\n", time, $2, 100 - $NF}'`)

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

	cmd := exec.Command("bash", "-c", `free | awk '/Mem:/ {printf("%s,%.2f\n", strftime("%Y-%m-%d %H:%M:%S"), $3/$2 * 100.0)}'`)

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

	cmd := exec.Command("bash", "-c", `df -h --output=source,pcent / | awk -v time="$(date +'%Y-%m-%d %H:%M:%S')" 'NR==2 {print time "," $1 "," $2}'`)

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
