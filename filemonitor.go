package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/fsnotify/fsnotify"
)

type FileMonitor struct {
	mu      sync.Mutex
	Path    string
	File    *os.File
	Watcher *fsnotify.Watcher
	ErrCh   chan error
}

// NewFileMonitor 创建文件监控器
func NewFileMonitor(path string) *FileMonitor {
	return &FileMonitor{
		Path:  path,
		ErrCh: make(chan error),
	}
}

// Start 启动监控器
func (fm *FileMonitor) Start() error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if fm.File != nil || fm.Watcher != nil {
		return fmt.Errorf("监控器重复启动")
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

func tailFile(filePath string, stopSig <-chan struct{}) error {
	// 创建文件监控器
	monitor := NewFileMonitor(filePath)

	// 启动监控器
	if err := monitor.Start(); err != nil {
		return err
	}
	defer monitor.Close()

	go func() {
		// 确保文件从末尾开始读取
		_, err := monitor.File.Seek(0, io.SeekEnd)
		if err != nil {
			monitor.ErrCh <- fmt.Errorf("文件 Seek 错误：%w", err)
			return
		}

		// 获取文件信息
		fileInfo, err := monitor.File.Stat()
		if err != nil {
			monitor.ErrCh <- fmt.Errorf("获取文件信息错误: %w", err)
			return
		}

		// 获取文件大小
		lastSize := fileInfo.Size()

		for {
			select {
			case <-stopSig:
				// 关闭监控器
				monitor.Close()
				return
			case event, ok := <-monitor.Watcher.Events:
				if !ok {
					return
				}

				if event.Has(fsnotify.Write) {
					if err := handleWriteEvent(monitor, &lastSize); err != nil {
						monitor.ErrCh <- err
						return
					}
				}

				if event.Has(fsnotify.Remove) {
					monitor.ErrCh <- fmt.Errorf("文件被删除")
					return
				}
			case err, ok := <-monitor.Watcher.Errors:
				if !ok {
					return
				}

				monitor.ErrCh <- fmt.Errorf("监控文件错误: %w", err)
				return
			}
		}
	}()

	return <-monitor.ErrCh
}

// 处理文件写入事件
func handleWriteEvent(fileMonitor *FileMonitor, lastSize *int64) error {
	// 获取文件信息
	fileInfo, err := fileMonitor.File.Stat()
	if err != nil {
		return fmt.Errorf("获取文件信息错误: %w", err)
	}

	// 获取文件大小
	currSize := fileInfo.Size()

	// 大小不变，无需处理
	if currSize == *lastSize {
		return nil
	}

	// 文件被截断
	if currSize < *lastSize {
		if _, err := fileMonitor.File.Seek(0, io.SeekStart); err != nil {
			return err
		}
	}

	// 读取文件
	reader := bufio.NewReader(fileMonitor.File)
	for {
		// 读取一行
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("读取文件错误: %w", err)
		}

		fmt.Println("行：", line)

		// 读到文件末尾
		if errors.Is(err, io.EOF) {
			break
		}
	}

	// 更新文件大小
	*lastSize = currSize

	return nil
}
