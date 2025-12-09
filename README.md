[English version](README.md) | [中文版](README_CN.md)

# rotail

## Introduction

rotail is a real-time log monitoring tool that can tail a single file or an entire directory.  
It works like the Unix/Linux `tail -f` command, with added support for directory log rotation monitoring.

## Features

- Real-time monitoring of single log files
- Monitor multiple log files in a directory
- Customizable file extension filtering
- Support for interrupt signal handling (SIGINT, SIGTERM)
- Easy-to-use command-line interface

## Installation

```bash
go install github.com/bynow2code/rotail@latest
```


Or build from source:

```bash
git clone https://github.com/bynow2code/rotail.git
cd rotail
go build -o rotail main.go
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

| Option | Description | Default |
|--------|-------------|---------|
| `-f` | File path to tail | None |
| `-d` | Directory path to tail | None |
| `-ext` | Comma-separated file extensions | `.log` |
| `-h` | Show help | false |

## Examples

```bash
# Monitor a single log file
rotail -f /var/log/nginx/access.log

# Monitor the latest .log files in /var/log directory
rotail -d /var/log

# Monitor .log and .txt files in a specific directory
rotail -d /app/logs -ext .log,.txt
```


## Architecture

The project is developed in Go language with key components including:

- [tail.Tailer](file:///Users/changqianqian/GolandProjects/rotail/internal/tail/tailer.go#L2-L7) interface: Defines core log monitoring functionality
- [tail.FileTailer](file:///Users/changqianqian/GolandProjects/rotail/internal/tail/file.go#L27-L39): Implements single file monitoring
- [tail.DirTailer](file:///Users/changqianqian/GolandProjects/rotail/internal/tail/dir.go#L25-L34): Implements directory monitoring
- Signal handling mechanism: Gracefully handles program exit

## License

rotail is licensed under the MIT License. See [LICENSE](LICENSE) for details.
