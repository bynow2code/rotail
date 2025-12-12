[English version](README.md) | [中文版](README_CN.md)

# rotail

[![Release](https://github.com/bynow2code/rotail/actions/workflows/release.yml/badge.svg)](https://github.com/bynow2code/rotail/actions/workflows/release.yml)

## 简介

rotail 是一个增强版的日志监控工具，类似于 `tail -f` 命令，但具备更强大的功能。除了可以实时监控单个日志文件外，它还能智能地监控整个目录中的最新日志文件，在发生日志轮转时自动切换到新的日志文件，确保您不会错过任何重要信息。

## 核心功能

- 🔍 **实时日志监控** - 实时追踪单个日志文件的变化
- 📂 **目录级监控** - 自动发现并监控目录中的最新日志文件
- 🔄 **智能轮转处理** - 日志轮转时自动切换到新文件，无缝衔接
- 🎯 **灵活文件过滤** - 支持自定义文件扩展名过滤器
- ⚡ **信号处理** - 优雅处理 SIGINT 和 SIGTERM 中断信号
- 💻 **友好命令行界面** - 简洁直观的命令行操作体验

## 快速安装

### 方法一：一键安装脚本（推荐）

适用于 Linux 和 macOS 系统：

```
curl -sfL https://raw.githubusercontent.com/bynow2code/rotail/main/install.sh | bash
```

### 方法二：手动下载

访问 [GitHub Releases](https://github.com/bynow2code/rotail/releases/latest) 页面下载适合您系统的预编译版本：

- **Windows 用户**：下载 `rotail-windows-amd64.exe` 并重命名为 `rotail.exe`
- **macOS 用户**：下载 `rotail-darwin-amd64` 或 `rotail-darwin-arm64`
- **Linux 用户**：根据架构下载对应的二进制文件

### 验证安装

安装完成后，运行以下命令验证是否安装成功：

rotail -v

## 使用指南

### 监控单个日志文件

```
rotail -f /path/to/your/logfile.log
```

### 监控整个目录

自动监控目录中最新的日志文件：

```
rotail -d /path/to/your/logdir
```

### 指定文件类型

只监控特定扩展名的文件：

```
rotail -d /path/to/your/logdir -ext .log,.txt,.out
```

### 获取帮助信息

查看所有可用选项：

```
rotail -h
```

## 命令行参数详解

| 参数     | 描述                   | 默认值     |
|----------|------------------------|------------|
| `-f`     | 指定要监控的单个文件路径 | 无         |
| `-d`     | 指定要监控的目录路径     | 无         |
| `-ext`   | 文件扩展名过滤器(逗号分隔)| `.log`    |
| `-v`     | 显示版本信息             |            |
| `-h`     | 显示帮助信息             | false     |

## 实用示例

### 监控 Nginx 访问日志

```
rotail -f /var/log/nginx/access.log
```

### 监控系统日志目录下的最新 .log 文件

```
rotail -d /var/log
```

### 监控应用程序日志目录下的 .log 和 .txt 文件

```
rotail -d /app/logs -ext .log,.txt
```

### 监控单个文件并输出到其他程序处理

```
rotail -f app.log | grep ERROR
```

## 支持 rotail

如果 rotail 对您的工作有所帮助，诚挚邀请您支持本项目。您的反馈将直接推动 rotail 的长期维护与功能增强：

1. **点击右上角的 ⭐ Star**，帮助更多开发者发现 rotail
2. **分享给朋友和同事**，让团队一起受益
3. **在社交媒体或技术社区推荐 rotail**，扩大项目影响力

## 开源许可证

本项目采用 MIT 许可证，详情请查看 [LICENSE](LICENSE) 文件。
