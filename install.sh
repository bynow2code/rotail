#!/usr/bin/env bash
set -e

# -------------------------------
# rotail 安装脚本
# -------------------------------

# 默认版本，可改为指定版本号
VERSION="latest"

# 检测 uname
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

# 映射 OS
if [ "$OS" = "darwin" ]; then OS="macos"; fi
if [ "$OS" = "linux" ]; then OS="linux"; fi

# 映射 ARCH
if [ "$ARCH" = "x86_64" ]; then ARCH="amd64"; fi
if [ "$ARCH" = "arm64" ] || [ "$ARCH" = "aarch64" ]; then ARCH="arm64"; fi

# 设置下载文件名
FILENAME="rotail-${VERSION}-${OS}-${ARCH}"
URL="https://github.com/bynow2code/rotail/releases/download/${VERSION}/${FILENAME}"

echo "安装 rotail ${VERSION} 版本..."
echo "操作系统: $OS, 架构: $ARCH"
echo "下载链接: $URL"

# 下载到临时文件
TMPFILE=$(mktemp)
curl -L "$URL" -o "$TMPFILE"

# macOS/Linux: 添加执行权限并移动到 /usr/local/bin
if [ "$OS" = "macos" ] || [ "$OS" = "linux" ]; then
    chmod +x "$TMPFILE"
    sudo mv "$TMPFILE" /usr/local/bin/rotail
    echo "安装完成: /usr/local/bin/rotail"
    echo "可用命令: rotail --version"
else
    # Windows 或其他系统
    echo "请手动将下载文件放到系统 PATH 中，并重命名为 rotail"
    echo "下载文件: $TMPFILE"
fi
