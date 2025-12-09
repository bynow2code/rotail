#!/usr/bin/env bash
set -e

# -----------------------
# 自动安装 rotail 最新版本
# -----------------------

# 系统检测
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

# OS 映射
case "$OS" in
    darwin) OS="macos" ;;
    linux)  OS="linux" ;;
    msys*|mingw*|cygwin*) OS="windows" ;;
    *) echo "不支持的系统: $OS"; exit 1 ;;
esac

# ARCH 映射
case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *) echo "不支持的架构: $ARCH"; exit 1 ;;
esac

# 获取最新版本号
LATEST=$(curl -sI https://github.com/bynow2code/rotail/releases/latest \
         | grep -i location \
         | awk -F/ '{print $NF}' | tr -d '\r\n')

if [ -z "$LATEST" ]; then
    echo "获取最新版本失败！"
    exit 1
fi

# 构建下载文件名
FILENAME="rotail-${LATEST}-${OS}-${ARCH}"
# Windows 二进制加 .exe
if [ "$OS" = "windows" ]; then
    FILENAME="${FILENAME}.exe"
fi

URL="https://github.com/bynow2code/rotail/releases/download/${LATEST}/${FILENAME}"

echo "安装 rotail ${LATEST}..."
echo "操作系统: $OS, 架构: $ARCH"
echo "下载链接: $URL"

# 下载临时文件
TMPFILE=$(mktemp)
curl -fL "$URL" -o "$TMPFILE"

# macOS / Linux: chmod + 移动到 /usr/local/bin
if [ "$OS" = "macos" ] || [ "$OS" = "linux" ]; then
    chmod +x "$TMPFILE"
    sudo mv "$TMPFILE" /usr/local/bin/rotail
    echo "安装完成: /usr/local/bin/rotail"
    rotail -h
else
    # Windows: 放到用户 bin 目录
    TARGET="$HOME/bin/rotail.exe"
    mkdir -p "$(dirname "$TARGET")"
    mv "$TMPFILE" "$TARGET"
    echo "安装完成: $TARGET"
    echo "请确保 $HOME/bin 在 PATH 中，然后运行: rotail.exe --version"
fi
