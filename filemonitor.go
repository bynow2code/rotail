package main

import (
	"fmt"
	"os"
	"sync"

	"github.com/fsnotify/fsnotify"
)

type FileMonitor struct {
	Path    string
	File    *os.File
	Watcher *fsnotify.Watcher
	mu      sync.Mutex
}

type Option func(file *FileMonitor) error

// WithSeek 设置文件下次读取或写入操作的偏移量
func WithSeek(offset int64, whence int) Option {
	return func(fm *FileMonitor) error {
		if fm.File == nil {
			return fmt.Errorf("文件未打开")
		}
		_, err := fm.File.Seek(offset, whence)
		if err != nil {
			return fmt.Errorf("文件Seek错误: %w", err)
		}
		return nil
	}
}

// NewFileMonitor 创建文件监控器
func NewFileMonitor(path string) *FileMonitor {
	return &FileMonitor{
		Path: path,
	}
}

// Start 启动监控器
func (fm *FileMonitor) Start() error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if fm.File != nil || fm.Watcher != nil {
		return fmt.Errorf("监控器重复启动！")
	}

	var err error
	defer func() {
		if err != nil {
			fm.Cleanup()
		}
	}()

	// 打开文件
	fm.File, err = os.Open(fm.Path)
	if err != nil {
		return fmt.Errorf("打开文件错误: %w", err)
	}

	// 创建监控器
	fm.Watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("创建监控器失败: %w", err)
	}

	// 添加监控
	err = fm.Watcher.Add(fm.Path)
	if err != nil {
		return fmt.Errorf("监控文件错误：%w", err)
	}

	return nil
}

// Cleanup 清理资源
func (fm *FileMonitor) Cleanup() {
	if fm.File != nil {
		_ = fm.File.Close()
		fm.File = nil
	}

	if fm.Watcher != nil {
		_ = fm.Watcher.Close()
		fm.Watcher = nil
	}
}

// Close 安全清理资源
func (fm *FileMonitor) Close() {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	fm.Cleanup()
}
