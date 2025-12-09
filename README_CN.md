[English version](README.md) | [中文版](README_CN.md)

# rotail

[![Release](https://github.com/bynow2code/rotail/actions/workflows/release.yml/badge.svg)](https://github.com/bynow2code/rotail/actions/workflows/release.yml)

## 简介

rotail 类似于 `tail -f`，但额外支持实时监控目录中最新文件的日志轮转，自动跟踪新生成的日志文件。

## 功能特性

- 实时监控单个日志文件
- 监控目录中最新日志文件
    - 自动处理日志轮转
    - 支持自定义文件扩展名
- 支持中断信号(SIGINT, SIGTERM)
- 易于使用的命令行界面

## 安装

### 方式一：下载预编译二进制（推荐）

从 GitHub Releases 获取 rotail：  
https://github.com/bynow2code/rotail/releases/latest

Linux / macOS：

```bash
curl -sfL https://raw.githubusercontent.com/bynow2code/rotail/main/install.sh | bash
```

Windows：下载 `rotail-windows-amd64.exe` 直接使用。

### 方式二：使用 Go 安装

```bash
go install github.com/bynow/rotail@latest
```

### 验证安装

```bash
rotail -h
```

## 使用方法

### 监控单个文件

```bash
rotail -f /path/to/your/logfile.log
```

### 监控整个目录

```bash
rotail -d /path/to/your/logdir
```

### 指定文件扩展名

```bash
rotail -d /path/to/your/logdir -ext .log,.txt,.out
```

### 查看帮助信息

```bash
rotail -h
```

## 命令行参数

| 参数     | 描述             | 默认值    |
|--------|----------------|--------|
| `-f`   | 要监控的文件路径       | 无      |
| `-d`   | 要监控的目录路径       | 无      |
| `-ext` | 文件扩展名过滤器(逗号分隔) | `.log` |
| `-v`   | 版本信息           |        |
| `-h`   | 显示帮助信息         | false  |

## 示例

```bash
# 监控单个日志文件
rotail -f /var/log/nginx/access.log

# 监控 /var/log 目录下的最新 .log 文件
rotail -d /var/log

# 监控指定目录下的 .log 和 .txt 文件
rotail -d /app/logs -ext .log,.txt
```

## ⭐ 支持 rotail

如果你觉得 rotail 有用，请给我们一个 ⭐ 支持！

## 技术架构

项目采用 Go 语言开发。

## 许可证

rotail is licensed under the MIT License. See [LICENSE](LICENSE) for details.
