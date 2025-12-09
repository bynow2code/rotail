[English version](README.md) | [中文版](README_CN.md)

# rotail

## Introduction

rotail works like `tail -f` but adds real-time directory log rotation monitoring, automatically following newly created
log files.

## Features

- Real-time monitoring of single log files
- Monitor the newest log file in a directory
    - Automatically handle log rotation
    - Filter files by custom extensions
- Support for interrupt signal handling (SIGINT, SIGTERM)
- Easy-to-use command-line interface

## Installation

### Method 1: Download prebuilt binaries (recommended)

Get rotail from GitHub Releases:  
https://github.com/bynow/rotail/releases/latest

Linux / macOS:

```bash
curl -sfL https://raw.githubusercontent.com/bynow/rotail/main/install.sh | bash
```

Windows: download `rotail-windows-amd64.exe` and run it.

---

### Method 2: Install via Go

```bash
go install github.com/bynow/rotail@latest
```

---

### Verify installation

```bash
rotail -h
```

## Usage

### Monitor a single file

```bash
rotail -f /path/to/your/logfile.log
```

### Monitor an entire directory

```bash
rotail -d /path/to/your/logdir
```

### Specify file extensions

```bash
rotail -d /path/to/your/logdir -ext .log,.txt,.out
```

### Show help

```bash
rotail -h
```

## Command Line Options

| Option | Description                     | Default |
|--------|---------------------------------|---------|
| `-f`   | File path to tail               | None    |
| `-d`   | Directory path to tail          | None    |
| `-ext` | Comma-separated file extensions | `.log`  |
| `-v`   | Show Version                    |  None   |
| `-h`   | Show help                       | false   |

## Examples

```bash
# Monitor a single log file
rotail -f /var/log/nginx/access.log

# Monitor the latest .log files in /var/log directory
rotail -d /var/log

# Monitor .log and .txt files in a specific directory
rotail -d /app/logs -ext .log,.txt
```

## ⭐ Star this project

If you find rotail useful, please give it a ⭐ on GitHub!

## Architecture

This project is written in Go.

## License

rotail is licensed under the MIT License. See [LICENSE](LICENSE) for details.
