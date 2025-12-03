package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/fsnotify/fsnotify"
)

type DMOption func(m *DirMonitor) error

func WithExt(ext []string) DMOption {
	return func(m *DirMonitor) error {
		m.Ext = ext
		return nil
	}
}

type DirMonitor struct {
	mu      sync.Mutex
	Path    string
	Ext     []string
	Watcher *fsnotify.Watcher
}

func NewDirMonitor(path string, opts ...DMOption) (*DirMonitor, error) {
	dm := &DirMonitor{
		Path: path,
	}

	for _, opt := range opts {
		if err := opt(dm); err != nil {
			return nil, err
		}
	}

	return dm, nil
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

func (dm *DirMonitor) FindLastMatchEntry() (os.DirEntry, error) {
	dirEntries, err := os.ReadDir(dm.Path)
	if err != nil {
		return nil, fmt.Errorf("获取目录下的文件失败: %w", err)
	}

	// 倒序遍历目录下的文件
	for i := len(dirEntries) - 1; i >= 0; i-- {
		entry := dirEntries[i]
		if entry.IsDir() {
			continue
		}
		fileExt := filepath.Ext(entry.Name())
		if slices.Contains(dm.Ext, fileExt) {
			return entry, nil
		}
	}

	return nil, fmt.Errorf("没有找到指定扩展名的文件")
}

func tailDir(dirPath string) error {
	dirMonitor, err := NewDirMonitor(dirPath, WithExt([]string{".log"}))
	if err != nil {
		return err
	}

	if err = dirMonitor.Start(); err != nil {
		return err
	}

	errCh := make(chan error)
	stopCh := make(chan struct{})

	entry, err := dirMonitor.FindLastMatchEntry()
	if err != nil {
		return err
	}
	filePath := filepath.Join(dirPath, entry.Name())
	go tailFile(filePath, stopCh)

	go func() {
		for {
			select {
			case event, ok := <-dirMonitor.Watcher.Events:
				if !ok {
					return
				}

				log.Println("event:", event)

				if event.Has(fsnotify.Create) {
					filePath := event.Name
					fileInfo, err := os.Stat(filePath)
					if err != nil {
						continue
					}

					if fileInfo.IsDir() {
						continue
					}

					fileExt := filepath.Ext(filePath)
					if !slices.Contains(dirMonitor.Ext, fileExt) {
						continue
					}

					stopCh <- struct{}{}

					go tailFile(filePath, stopCh)
				}
			case err, ok := <-dirMonitor.Watcher.Errors:
				if !ok {
					return
				}

				log.Println("error:", err)
			}
		}
	}()

	err = <-errCh
	return err
}
