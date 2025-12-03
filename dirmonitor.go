package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/fsnotify/fsnotify"
)

type DirMonitor struct {
	mu        sync.Mutex
	Path      string
	Ext       []string
	Watcher   *fsnotify.Watcher
	ErrCh     chan error
	RotateSig chan struct{}
}

// NewDirMonitor 创建目录监控器
func NewDirMonitor(path string, opts ...DMOption) (*DirMonitor, error) {
	dm := &DirMonitor{
		Path:      path,
		ErrCh:     make(chan error),
		RotateSig: make(chan struct{}),
	}

	for _, opt := range opts {
		if err := opt(dm); err != nil {
			return nil, err
		}
	}

	return dm, nil
}

type DMOption func(m *DirMonitor) error

// WithExt 根据设置的文件拓展名筛选日志
func WithExt(ext []string) DMOption {
	return func(m *DirMonitor) error {
		m.Ext = ext
		return nil
	}
}

// Start 启动监控器
func (dm *DirMonitor) Start() error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if dm.Watcher != nil {
		return fmt.Errorf("监控器重复启动")
	}

	var err error
	defer func() {
		if err != nil {
			dm.Cleanup()
		}
	}()

	// 创建监控器
	dm.Watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("创建监控器失败: %w", err)
	}

	// 添加监控
	err = dm.Watcher.Add(dm.Path)
	if err != nil {
		return fmt.Errorf("监控文件错误：%w", err)
	}

	return nil
}

// Cleanup 清理资源
func (dm *DirMonitor) Cleanup() {
	if dm.Watcher != nil {
		_ = dm.Watcher.Close()
		dm.Watcher = nil
	}
}

// Close 安全清理资源
func (dm *DirMonitor) Close() {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.Cleanup()
}

// FindLastMatchEntry 取目录(字典倒叙排列)中复合条件文件的第一个
func (dm *DirMonitor) FindLastMatchEntry() (string, error) {
	dirEntries, err := os.ReadDir(dm.Path)
	if err != nil {
		return "", fmt.Errorf("获取目录下的文件失败: %w", err)
	}

	// 倒序遍历目录下的文件
	for i := len(dirEntries) - 1; i >= 0; i-- {
		entry := dirEntries[i]
		if entry.IsDir() {
			continue
		}

		// 判断文件拓展名
		if !slices.Contains(dm.Ext, filepath.Ext(entry.Name())) {
			continue
		}

		return filepath.Join(dm.Path, entry.Name()), nil
	}

	return "", fmt.Errorf("没有找到指定扩展名的文件")
}

func tailDir(dirPath string) error {
	// 创建目录监控器
	monitor, err := NewDirMonitor(dirPath, WithExt([]string{".log"}))
	if err != nil {
		return err
	}

	// 启动监控器
	if err = monitor.Start(); err != nil {
		return err
	}

	// 获取目录下的文件
	filePath, err := monitor.FindLastMatchEntry()
	if err != nil {
		return err
	}

	go tailFile(filePath, monitor.RotateSig)

	go func() {
		for {
			select {
			case event, ok := <-monitor.Watcher.Events:
				if !ok {
					return
				}

				if event.Has(fsnotify.Create) {
					fileInfo, err := os.Stat(event.Name)
					if err != nil {
						continue
					}

					if fileInfo.IsDir() {
						continue
					}

					fileExt := filepath.Ext(event.Name)
					if !slices.Contains(monitor.Ext, fileExt) {
						continue
					}

					monitor.RotateSig <- struct{}{}

					go tailFile(filePath, monitor.RotateSig)
				}
			case err, ok := <-monitor.Watcher.Errors:
				if !ok {
					return
				}

				monitor.ErrCh <- fmt.Errorf("监控目录错误: %w", err)
			}
		}
	}()

	return <-monitor.ErrCh
}
