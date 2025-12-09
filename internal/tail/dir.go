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

var ErrNoFoundFile = errors.New("no file found")

// DirTailer 是一个目录监听器，用于监控指定目录下特定扩展名文件的变化并读取新增的行内容
// path: 监听的目录路径
// ext: 需要监听的文件扩展名列表
// ft: 文件跟踪器，用于跟踪文件内容变化
// watcher: 文件系统监听器，用于监控目录变化
// lineCh: 用于发送读取到的文件行内容的通道
// errCh: 用于发送错误信息的通道
// stopCh: 用于接收停止信号的通道
// wg: 用于等待所有goroutine完成的等待组
type DirTailer struct {
	path    string
	ext     []string
	ft      *FileTailer
	watcher *fsnotify.Watcher
	lineCh  chan string
	errCh   chan error
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

// NewDir 创建一个目录跟踪器
func NewDir(path string, opts ...DTOption) (*DirTailer, error) {
	t := &DirTailer{
		path:   path,
		lineCh: make(chan string),
		errCh:  make(chan error),
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

// Start 启动目录跟踪器
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

	fmt.Printf("%sStarting dir tailer:%s\n%s", colorGreen, t.path, colorReset)

	// 获取目录信息
	fi, err := os.Stat(t.path)
	if err != nil {
		return err
	}

	if !fi.IsDir() {
		return fmt.Errorf("%s is not a directory", t.path)
	}

	// 设置绝对路径
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

// run 运行目录跟踪器
func (t *DirTailer) run() {
	defer t.wg.Done()

	defer func() {
		// 停止文件跟踪器
		if t.ft != nil {
			t.ft.Stop()
			t.ft = nil
		}

		// 关闭 watcher
		if t.watcher != nil {
			_ = t.watcher.Close()
			t.watcher = nil
		}

		close(t.lineCh)
	}()

	// 监听目录内文件
	err := t.tailFileInDir()
	if err != nil {
		t.errCh <- err
		return
	}

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
				if err := t.handleCreateEvent(event); err != nil {
					t.errCh <- err
					return
				}
			}

			// 处理目录更改
			if event.Op&(fsnotify.Rename|fsnotify.Remove) != 0 {
				if err := t.handleChangeEvent(event); err != nil {
					t.errCh <- err
					return
				}
			}

		// 监听错误
		case err, ok := <-t.watcher.Errors:
			if !ok {
				return
			}
			t.errCh <- err
			return
		}
	}
}

// handleCreateEvent 处理目录内文件创建事件
func (t *DirTailer) handleCreateEvent(event fsnotify.Event) error {
	// 文件后缀不匹配
	ext := filepath.Ext(event.Name)
	if !slices.Contains(t.ext, ext) {
		return nil
	}

	// 监听目录内文件
	if err := t.tailFileInDir(); err != nil {
		return err
	}

	return nil
}

// tailFileInDir 监听目录内文件
func (t *DirTailer) tailFileInDir() error {
	// 查找目录内最新文件
	file, err := t.findLastFileInDir()
	if err != nil {
		if !errors.Is(err, ErrNoFoundFile) {
			return err
		}
	}
	// 目录内没有符合的文件
	if errors.Is(err, ErrNoFoundFile) {
		return nil
	}

	var opts []FTOption

	// 停止上一个文件跟踪器
	if t.ft != nil {
		t.ft.Stop()
		t.ft = nil
		// 调整文件偏移量
		opts = append(opts, WithSeek(0, io.SeekStart))
	}

	// 创建新的文件跟踪器
	ft, err := NewFile(file, opts...)
	if err != nil {
		return err
	}
	t.ft = ft

	go t.tailFile()

	return nil
}

// tailFile 监听文件
func (t *DirTailer) tailFile() {
	defer func() {
		if t.ft != nil {
			t.ft = nil
		}
	}()

	// 启动文件跟踪器
	if err := t.ft.Start(); err != nil {
		t.errCh <- err
		return
	}

	// 监听文件行内容
	go func() {
		for line := range t.ft.lineCh {
			t.lineCh <- line
		}
	}()

	select {

	// 停止信号
	case <-t.stopCh:
		t.ft.Stop()

	// 错误
	case err := <-t.ft.errCh:
		t.errCh <- err
	}
}

// handleChangeEvent 处理目录更改事件
func (t *DirTailer) handleChangeEvent(event fsnotify.Event) error {
	if event.Name == t.path {
		return fmt.Errorf("the directory has been moved:%s", event.Name)
	}
	return nil
}

// findLastFileInDir 查找目录内最新文件
func (t *DirTailer) findLastFileInDir() (string, error) {
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
	select {
	case <-t.stopCh:
	default:
		close(t.stopCh)
		t.wg.Wait()
	}
}
