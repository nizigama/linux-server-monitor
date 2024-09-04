# LINUX SERVER MONITOR

## Description

This is a small program that reads a linux server's stats and store them in a local database, then exposes an endpoint to consume those statistics.
The available statistics, in percentages, are:
  - CPU usage(for each core)
  - Memory usage
  - Disk usage