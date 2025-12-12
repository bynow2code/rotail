[English version](README.md) | [中文版](README_CN.md)

# rotail

[![Release](https://github.com/bynow2code/rotail/actions/workflows/release.yml/badge.svg)](https://github.com/bynow2code/rotail/actions/workflows/release.yml)

## Introduction

rotail is an enhanced log monitoring tool similar to the `tail -f` command, but with more powerful features. In addition to real-time monitoring of individual log files, it can intelligently monitor the latest log files in an entire directory, automatically switching to new log files when log rotation occurs, ensuring you don't miss any important information.

## Core Features

- 🔍 **Real-time Log Monitoring** - Track changes in individual log files in real-time
- 📂 **Directory-level Monitoring** - Automatically discover and monitor the latest log files in a directory
- 🔄 **Smart Rotation Handling** - Automatically switch to new files during log rotation for seamless transition
- 🎯 **Flexible File Filtering** - Support custom file extension filters
- ⚡ **Signal Handling** - Gracefully handle SIGINT and SIGTERM interrupt signals
- 💻 **User-friendly CLI** - Clean and intuitive command-line experience

## Quick Installation

### Method 1: One-click Installation Script (Recommended)

For Linux and macOS systems:

```
curl -sfL https://raw.githubusercontent.com/bynow2code/rotail/main/install.sh | bash
```

### Method 2: Manual Download

Visit the [GitHub Releases](https://github.com/bynow2code/rotail/releases/latest) page to download the precompiled version suitable for your system:

- **Windows Users**: Download `rotail-windows-amd64.exe` and rename it to `rotail.exe`
- **macOS Users**: Download `rotail-darwin-amd64` or `rotail-darwin-arm64`
- **Linux Users**: Download the corresponding binary file based on your architecture

### Verify Installation

After installation, run the following command to verify successful installation:

```
rotail -v
```

## Usage Guide

### Monitor a Single Log File

```
rotail -f /path/to/your/logfile.log
```

### Monitor an Entire Directory

Automatically monitor the latest log file in a directory:

```
rotail -d /path/to/your/logdir
```

### Specify File Types

Monitor only files with specific extensions:

```
rotail -d /path/to/your/logdir -ext .log,.txt,.out
```

### Get Help Information

View all available options:

```
rotail -h
```

## Command Line Arguments

| Argument | Description                       | Default    |
|----------|-----------------------------------|------------|
| `-f`     | Path to the single file to monitor | None       |
| `-d`     | Path to the directory to monitor   | None       |
| `-ext`   | File extension filter (comma-separated) | `.log`    |
| `-v`     | Display version information        |            |
| `-h`     | Display help information           | false      |

## Practical Examples

### Monitor Nginx Access Logs

```
rotail -f /var/log/nginx/access.log
```

### Monitor Latest .log Files in System Log Directory

```
rotail -d /var/log
```

### Monitor .log and .txt Files in Application Log Directory

```
rotail -d /app/logs -ext .log,.txt
```

### Monitor a Single File and Pipe to Another Program

```
rotail -f app.log | grep ERROR
```

## Support rotail

If rotail has been helpful in your workflow, we sincerely invite you to support the project. Your engagement directly contributes to ongoing improvements and long-term maintenance:

1. **Click the ⭐ Star button** in the top-right corner to help others discover rotail
2. **Share it with your colleagues and friends** who may benefit from it
3. **Recommend rotail on social media or technical communities** to expand visibility

## Open Source License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
