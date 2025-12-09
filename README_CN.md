# rotail

## 简介

rotail 是一款日志文件实时监控工具，支持对单个文件或整个目录进行实时追踪（tail）。  
它类似于 Unix/Linux 的 `tail -f` 命令，但额外提供目录日志轮转监控功能。

## 功能特性

- 实时监控单个日志文件
- 监控整个目录下的多个日志文件
- 支持自定义文件扩展名过滤
- 支持中断信号处理(SIGINT, SIGTERM)
- 易于使用的命令行界面

## 安装

```bash
go install github.com/bynow2code/rotail@latest
```


或者从源码构建：

```bash
git clone https://github.com/bynow2code/rotail.git
cd rotail
go build -o rotail main.go
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

| 参数 | 描述 | 默认值 |
|------|------|--------|
| `-f` | 要监控的文件路径 | 无 |
| `-d` | 要监控的目录路径 | 无 |
| `-ext` | 文件扩展名过滤器(逗号分隔) | `.log` |
| `-h` | 显示帮助信息 | false |

## 示例

```bash
# 监控单个日志文件
rotail -f /var/log/nginx/access.log

# 监控 /var/log 目录下的最新 .log 文件
rotail -d /var/log

# 监控指定目录下的 .log 和 .txt 文件
rotail -d /app/logs -ext .log,.txt
```


## 技术架构

项目采用 Go 语言开发，主要组件包括：

- [tail.Tailer](file:///Users/changqianqian/GolandProjects/rotail/internal/tail/tailer.go#L2-L7) 接口：定义了日志监控的核心功能
- [tail.FileTailer](file:///Users/changqianqian/GolandProjects/rotail/internal/tail/file.go#L27-L39)：实现单文件监控
- [tail.DirTailer](file:///Users/changqianqian/GolandProjects/rotail/internal/tail/dir.go#L25-L34)：实现目录监控
- 信号处理机制：优雅地处理程序退出

## 许可证

rotail is licensed under the MIT License. See [LICENSE](LICENSE) for details.
