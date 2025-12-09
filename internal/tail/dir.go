package tail

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/fsnotify/fsnotify"
)

var ErrNotFoundFile = errors.New("file not found")

type DirTailer struct {
	path       string
	ext        []string
	fileTailer *FileTailer
	watcher    *fsnotify.Watcher
	lineCh     chan string
	errCh      chan error
	stopCh     chan struct{}
	wg         sync.WaitGroup
	stopOnce   sync.Once
}

// NewDir 创建一个目录跟踪器
func NewDir(path string, opts ...DTOption) (*DirTailer, error) {
	t := &DirTailer{
		path:   path,
		lineCh: make(chan string),
		errCh:  make(chan error),
		stopCh: make(chan struct{}),
	}

	for _, opt := range opts {
		if err := opt(t); err != nil {
			return nil, err
		}
	}

	return t, nil
}

type DTOption func(tailer *DirTailer) error

// WithExt 设置文件后缀
func WithExt(ext []string) DTOption {
	return func(t *DirTailer) error {
		t.ext = ext
		return nil
	}
}

// Start 启动目录跟踪器
func (t *DirTailer) Start() error {
	fmt.Printf("%sStarting dir tailer:%s\n%s", colorGreen, t.path, colorReset)

	fi, err := os.Stat(t.path)
	if err != nil {
		return err
	}

	if !fi.IsDir() {
		return fmt.Errorf("%s is not a directory", t.path)
	}

	// 设置绝对路径
	absPath, err := filepath.Abs(t.path)
	if err != nil {
		return err
	}
	t.path = absPath

	// 创建 watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	t.watcher = watcher

	if err = t.watcher.Add(t.path); err != nil {
		t.cleanup()
		return err
	}

	t.wg.Add(1)
	go t.run()

	return nil
}

// run 运行目录跟踪器
func (t *DirTailer) run() {
	defer t.wg.Done()
	defer t.cleanup()

	// 监听目录内文件
	go t.tailFileInDir()

	for {
		select {
		case <-t.stopCh:
			return
		case event, ok := <-t.watcher.Events:
			if !ok {
				return
			}

			if event.Has(fsnotify.Create) {
				t.handleCreateEvent(event)
			}

			if event.Op&(fsnotify.Rename|fsnotify.Remove) != 0 {
				if err := t.handleChangeEvent(event); err != nil {
					t.errCh <- err
					return
				}
			}
		case err, ok := <-t.watcher.Errors:
			if !ok {
				return
			}
			t.errCh <- err
			return
		}
	}
}

// handleCreateEvent 处理新文件
func (t *DirTailer) handleCreateEvent(event fsnotify.Event) {
	// 检查文件后缀
	ext := filepath.Ext(event.Name)
	if !slices.Contains(t.ext, ext) {
		return
	}

	go t.tailFileInDir()
}

// findLastFileInDir 获取最新文件
func (t *DirTailer) findLastFileInDir() (string, error) {
	entries, err := os.ReadDir(t.path)
	if err != nil {
		return "", err
	}

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

	return "", ErrNotFoundFile
}

// tailFileInDir 启动最新文件跟踪
func (t *DirTailer) tailFileInDir() {
	file, err := t.findLastFileInDir()
	if err != nil && !errors.Is(err, ErrNotFoundFile) {
		t.errCh <- err
		return
	}

	// 目录里暂时没有匹配文件，等下一次创建
	if file == "" {
		return
	}

	var opts []FTOption

	if t.fileTailer != nil {
		t.fileTailer.Stop()
		t.fileTailer = nil
		// 调整文件偏移量
		opts = append(opts, WithSeek(0, io.SeekStart))
	}

	// 创建文件跟踪器
	fileTailer, err := NewFile(file, opts...)
	if err != nil {
		t.errCh <- err
		return
	}
	t.fileTailer = fileTailer

	if err := t.fileTailer.Start(); err != nil {
		t.errCh <- err
		return
	}

	go func() {
		for line := range t.fileTailer.GetLineCh() {
			t.lineCh <- line
		}
	}()

	select {
	case <-t.stopCh:
		t.fileTailer.Stop()
		return
	case err, ok := <-t.fileTailer.errCh:
		if !ok {
			return
		}
		t.errCh <- err
		return
	}
}

// handleChangeEvent 处理目录更改
func (t *DirTailer) handleChangeEvent(event fsnotify.Event) error {
	if event.Name == t.path {
		t.errCh <- fmt.Errorf("the directory has been moved: %s", event.Name)
	}
	return nil
}

// GetLineCh 获取行通道
func (t *DirTailer) GetLineCh() <-chan string {
	return t.lineCh
}

// GetErrCh 获取错误通道
func (t *DirTailer) GetErrCh() <-chan error {
	return t.errCh
}

// Stop 停止目录跟踪器
func (t *DirTailer) Stop() {
	t.stopOnce.Do(func() {
		close(t.stopCh)
		t.wg.Wait()
	})
}

// cleanup 清理资源
func (t *DirTailer) cleanup() {
	if t.fileTailer != nil {
		t.fileTailer.Stop()
		t.fileTailer = nil
	}

	if t.watcher != nil {
		_ = t.watcher.Close()
		t.watcher = nil
	}

	close(t.lineCh)
	close(t.errCh)
}
