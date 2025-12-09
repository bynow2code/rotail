package main

import (
	"bufio"
	"errors"
	"io"
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FileTailer 文件尾随器结构体，用于实时监控和读取文件新增内容
// path: 要监控的文件路径
// file: 打开的文件句柄
// watcher: 文件系统监控器，用于监听文件变化
// size: 当前文件大小
// lastSize: 上次记录的文件大小
// offset: 当前读取位置的偏移量
// whence: 寻找位置的参考点（SeekStart, SeekCurrent, SeekEnd）
// LineCh: 传输文件新行内容的通道
// ErrCh: 传输错误信息的通道
// stopCh: 控制停止信号的通道
// wg: 用于等待所有goroutine完成的同步组
type FileTailer struct {
	path     string
	file     *os.File
	watcher  *fsnotify.Watcher
	size     int64
	lastSize int64
	offset   int64
	whence   int
	LineCh   chan string
	ErrCh    chan error
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// NewFileTailer 创建一个文件尾随器
func NewFileTailer(path string, opts ...FTOption) (*FileTailer, error) {
	t := &FileTailer{
		path:   path,
		offset: 0,
		whence: io.SeekEnd,
		LineCh: make(chan string),
		ErrCh:  make(chan error),
		stopCh: make(chan struct{}),
	}

	// 配置项
	for _, opt := range opts {
		if err := opt(t); err != nil {
			return nil, err
		}
	}

	return t, nil
}

// FTOption 文件尾随器配置项
type FTOption func(tailer *FileTailer) error

// WithSeek 设置文件读取位置
func WithSeek(offset int64, whence int) FTOption {
	return func(t *FileTailer) error {
		t.offset = offset
		t.whence = whence
		return nil
	}
}

// Start 启动文件尾随器
func (t *FileTailer) Start() error {
	var err error

	defer func() {
		if err != nil {
			if t.watcher != nil {
				_ = t.watcher.Close()
				t.watcher = nil
			}

			if t.file != nil {
				_ = t.file.Close()
				t.file = nil
			}
		}
	}()

	// 打开文件
	file, err := os.Open(t.path)
	if err != nil {
		return err
	}
	t.file = file

	// 获取文件大小
	fi, err := t.file.Stat()
	if err != nil {
		return err
	}
	t.size = fi.Size()
	t.lastSize = t.size

	// 创建文件系统监控器
	t.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	// 添加文件监控
	if err = t.watcher.Add(t.path); err != nil {
		return err
	}

	t.wg.Add(1)
	go t.run()

	return nil
}

// run 运行文件尾随器
func (t *FileTailer) run() {
	defer t.wg.Done()

	defer func() {
		if t.watcher != nil {
			_ = t.watcher.Close()
			t.watcher = nil
		}

		if t.file != nil {
			_ = t.file.Close()
			t.file = nil
		}

		close(t.LineCh)
	}()

	// 改变文件偏移量
	if _, err := t.file.Seek(t.offset, t.whence); err != nil {
		t.ErrCh <- err
		return
	}

	// 读取文件内容
	t.readLines()

	for {
		select {

		// 停止信号
		case <-t.stopCh:
			return

		// 文件事件
		case event, ok := <-t.watcher.Events:
			if !ok {
				return
			}

			// 处理文件写入
			if event.Has(fsnotify.Write) {
				t.handleWriteEvent(event)
			}

			// 处理文件轮转
			if event.Op&(fsnotify.Create|fsnotify.Rename|fsnotify.Remove) != 0 {
				// 等待写入方重建文件
				time.Sleep(100 * time.Millisecond)
				t.handleRotate()
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

// handleWriteEvent 处理文件写入
func (t *FileTailer) handleWriteEvent(event fsnotify.Event) {
	// 处理文件截断
	if err := t.handleFileTruncation(); err != nil {
		t.ErrCh <- err
		return
	}

	// 读取文件内容
	t.readLines()
}

// handleRotate 处理文件轮转
func (t *FileTailer) handleRotate() {
	// 关闭文件句柄
	if err := t.file.Close(); err != nil {
		t.ErrCh <- err
		return
	}

	// 打开文件
	file, err := os.Open(t.path)
	if err != nil {
		t.ErrCh <- err
		return
	}
	t.file = file

	// 获取文件大小
	fi, err := t.file.Stat()
	if err != nil {
		t.ErrCh <- err
		return
	}
	t.size = fi.Size()
	t.lastSize = t.size

	// 添加文件监控
	_ = t.watcher.Remove(t.path)
	if err = t.watcher.Add(t.path); err != nil {
		t.ErrCh <- err
		return
	}

	// 读取文件内容
	t.readLines()

	return
}

// handleFileTruncation 处理文件截断
func (t *FileTailer) handleFileTruncation() error {
	// 获取文件大小
	fi, err := t.file.Stat()
	if err != nil {
		return err
	}

	// 文件被截断
	curSize := fi.Size()
	if curSize < t.lastSize {
		if _, err := t.file.Seek(0, io.SeekStart); err != nil {
			return err
		}
	}

	// 更新文件大小
	t.lastSize = curSize

	return nil
}

// readLines 读取文件内容
func (t *FileTailer) readLines() {
	reader := bufio.NewReader(t.file)

	for {
		// 读取一行
		line, err := reader.ReadString('\n')

		if err != nil {
			if !errors.Is(err, io.EOF) {
				t.ErrCh <- err
				return
			}

			// 遇到 EOF，也要发送最后一行
			t.LineCh <- line
			break
		}

		// 正常发送新行
		t.LineCh <- line
	}
}

// Stop 停止文件尾随器
func (t *FileTailer) Stop() {
	select {
	case <-t.stopCh:
	default:
		close(t.stopCh)
		t.wg.Wait()
	}
}
