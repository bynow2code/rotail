package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/fsnotify/fsnotify"
)

var ErrNoFoundFile = errors.New("no file found")

// DirTailer 是一个目录监听器，用于监控指定目录下特定扩展名文件的变化并读取新增的行内容
// path: 监听的目录路径
// ext: 需要监听的文件扩展名列表
// ft: 文件跟踪器，用于跟踪文件内容变化
// watcher: 文件系统监听器，用于监控目录变化
// LineCh: 用于发送读取到的文件行内容的通道
// ErrCh: 用于发送错误信息的通道
// stopCh: 用于接收停止信号的通道
// wg: 用于等待所有goroutine完成的等待组
type DirTailer struct {
	path    string
	ext     []string
	ft      *FileTailer
	watcher *fsnotify.Watcher
	LineCh  chan string
	ErrCh   chan error
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

// NewDirTailer 创建一个目录尾随器
func NewDirTailer(path string, opts ...DTOption) (*DirTailer, error) {
	t := &DirTailer{
		path:   path,
		LineCh: make(chan string),
		ErrCh:  make(chan error),
		stopCh: make(chan struct{}),
	}

	// 设置参数
	for _, opt := range opts {
		if err := opt(t); err != nil {
			return nil, err
		}
	}

	return t, nil
}

// DTOption 参数
type DTOption func(tailer *DirTailer) error

// WithExt 设置文件后缀
func WithExt(ext []string) DTOption {
	return func(t *DirTailer) error {
		t.ext = ext
		return nil
	}
}

// Start 启动目录尾随器
func (t *DirTailer) Start() error {
	var err error

	defer func() {
		if err != nil {
			// 关闭 watcher
			if t.watcher != nil {
				_ = t.watcher.Close()
				t.watcher = nil
			}
		}
	}()

	// 获取绝对路径
	if !filepath.IsAbs(t.path) {
		var absPath string
		absPath, err = filepath.Abs(t.path)
		if err != nil {
			return err
		}
		t.path = absPath
	}

	// 创建 watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	t.watcher = watcher

	// 添加目录监控
	if err = t.watcher.Add(t.path); err != nil {
		return err
	}

	t.wg.Add(1)
	go t.run()

	return nil
}

// run 运行目录尾随器
func (t *DirTailer) run() {
	defer t.wg.Done()

	defer func() {
		// 停止文件尾随器
		if t.ft != nil {
			t.ft.Stop()
			t.ft = nil
		}

		// 关闭 watcher
		if t.watcher != nil {
			_ = t.watcher.Close()
			t.watcher = nil
		}

		// 关闭 LineCh
		close(t.LineCh)
	}()

	// 寻找并开始监听文件
	t.findAndTailFile()

	for {
		select {

		// 停止信号
		case <-t.stopCh:
			return

		// 监听事件
		case event, ok := <-t.watcher.Events:
			if !ok {
				return
			}

			// 处理目录内创建事件
			if event.Has(fsnotify.Create) {
				t.findAndTailFile()
			}

			// 处理目录更改
			if event.Op&(fsnotify.Rename|fsnotify.Remove) != 0 {
				t.handleDirChangeEvent(event)
			}

		// 监听错误
		case err, ok := <-t.watcher.Errors:
			if !ok {
				return
			}
			t.ErrCh <- err
			return
		}
	}
}

// findAndTailFile 寻找并开始监听文件
func (t *DirTailer) findAndTailFile() {
	// 查找文件
	file, err := t.findFileInDir()
	if err != nil && !errors.Is(err, ErrNoFoundFile) {
		t.ErrCh <- err
		return
	}
	// 文件不存在
	if file == "" {
		return
	}

	// 停止文件尾随器
	if t.ft != nil {
		if t.ft.path == file {
			return
		}
		t.ft.Stop()
		t.ft = nil
	}

	// 创建文件尾随器
	ft, err := NewFileTailer(file)
	if err != nil {
		t.ErrCh <- err
		return
	}
	t.ft = ft

	go t.tailFile(ft)
}

// tailFile 监听文件
func (t *DirTailer) tailFile(ft *FileTailer) {
	// 启动文件尾随器
	if err := t.ft.Start(); err != nil {
		t.ErrCh <- err
		return
	}

	// 监听文件行内容
	go func() {
		for line := range t.ft.LineCh {
			t.LineCh <- line
		}
	}()

	select {

	// 停止信号
	case <-t.stopCh:
		t.ft.Stop()

	// 错误
	case err := <-t.ft.ErrCh:
		t.ErrCh <- err
	}
}

// handleDirChangeEvent 处理目录更改事件
func (t *DirTailer) handleDirChangeEvent(event fsnotify.Event) {
	if event.Name == t.path {
		t.ErrCh <- fmt.Errorf("path %s has been changed", event.Name)
		return
	}
}

// findFileInDir 寻找目录下符合条件的文件
func (t *DirTailer) findFileInDir() (string, error) {
	// 读取目录下的文件
	entries, err := os.ReadDir(t.path)
	if err != nil {
		return "", err
	}

	// 倒序遍历
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]

		// 跳过目录
		if entry.IsDir() {
			continue
		}

		// 匹配文件后缀
		ext := filepath.Ext(entry.Name())
		if slices.Contains(t.ext, ext) {
			return filepath.Join(t.path, entry.Name()), nil
		}
	}

	return "", ErrNoFoundFile
}

// Stop 停止目录尾随器
func (t *DirTailer) Stop() {
	select {
	case <-t.stopCh:
	default:
		close(t.stopCh)
		t.wg.Wait()
	}
}
