package tail

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/fsnotify/fsnotify"
)

type DirTailer struct {
	path     string
	ext      []string
	ft       *FileTailer
	watcher  *fsnotify.Watcher
	lineCh   chan string
	errCh    chan error
	stopCh   chan struct{}
	wg       sync.WaitGroup
	stopOnce sync.Once
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
	if err := t.tailFileInDir(); err != nil {
		t.errCh <- err
		return
	}

	for {
		select {
		case <-t.stopCh:
			return
		case event, ok := <-t.watcher.Events:
			if !ok {
				return
			}

			if event.Has(fsnotify.Create) {
				if err := t.handleCreateEvent(event); err != nil {
					t.errCh <- err
					return
				}
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
func (t *DirTailer) handleCreateEvent(event fsnotify.Event) error {
	// 检查文件后缀
	ext := filepath.Ext(event.Name)
	if !slices.Contains(t.ext, ext) {
		return nil
	}

	return t.tailFileInDir()
}

// findLastFileInDir 获取最新文件
func (t *DirTailer) findLastFileInDir() ([]string, error) {
	entries, err := os.ReadDir(t.path)
	if err != nil {
		return nil, err
	}

	var files []string
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
			files = append(files, filepath.Join(t.path, entry.Name()))
			break
		}
	}

	return files, nil
}

// tailFileInDir 启动最新文件跟踪
func (t *DirTailer) tailFileInDir() error {
	files, err := t.findLastFileInDir()
	if err != nil {
		return err
	}

	// 目录里暂时没有匹配文件，等下一次创建
	if len(files) == 0 {
		return nil
	}

	var opts []FTOption

	if t.ft != nil {
		t.ft.Stop()
		t.ft = nil
		// 调整文件偏移量
		opts = append(opts, WithSeek(0, io.SeekStart))
	}

	// 创建新的文件跟踪器
	ft, err := NewFile(files[0], opts...)
	if err != nil {
		return err
	}
	t.ft = ft

	t.wg.Add(1)
	go t.tailFile()

	return nil
}

// tailFile 监听单个文件
func (t *DirTailer) tailFile() {
	defer t.wg.Done()

	if err := t.ft.Start(); err != nil {
		t.errCh <- err
		return
	}

	for {
		select {
		case <-t.stopCh:
			t.ft.Stop()
		case line, ok := <-t.ft.GetLineCh():
			if !ok {
				return
			}
			t.lineCh <- line
		case err, ok := <-t.ft.errCh:
			if !ok {
				return
			}
			t.errCh <- err
		}
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
	if t.ft != nil {
		t.ft.Stop()
		t.ft = nil
	}

	if t.watcher != nil {
		_ = t.watcher.Close()
		t.watcher = nil
	}

	close(t.lineCh)
	close(t.errCh)
}
