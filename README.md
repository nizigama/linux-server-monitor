# LINUX SERVER MONITOR

## Description

This is a small program that reads a linux server's stats and store them in a local database, then exposes an endpoint to consume those statistics.
The available statistics, in percentages, are:
  - CPU usage(for each core)
  - Memory usage
  - Disk usage

## Usage
- Build for specific architectures
  - Linux arm
    ```shell
        env GOOS=linux GOARCH=arm go build -v -o bin/linux-arm *.go
    ```
  - Linux amd64
    ```shell
        env GOOS=linux GOARCH=amd64 go build -v -o bin/linux-amd64 *.go
    ```
  - Linux 386
    ```shell
        env GOOS=linux GOARCH=386 go build -v -o bin/linux-386 *.go
    ```